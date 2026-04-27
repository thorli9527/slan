library slan_app.application.device_setup_service;

import '../infra/app_core/api/app_core_api.dart';
import '../infra/app_core/models/bootstrap_models.dart';
import '../infra/app_core/models/control_models.dart';
import '../infra/app_core/models/identity_models.dart';
import '../infra/app_core/models/network_models.dart';

class DeviceRegistrationInput {
  const DeviceRegistrationInput({
    required this.name,
    required this.platform,
    this.deviceVersion,
    required this.machineId,
    required this.publicKey,
  });

  final String name;
  final String platform;
  final String? deviceVersion;
  final String machineId;
  final String publicKey;
}

class NodeRegistrationInput {
  const NodeRegistrationInput({
    required this.deviceId,
    required this.nodeId,
    required this.nodePublicKey,
    this.bootstrapNetworkId,
    this.capabilities = const ['desktop'],
  });

  final String deviceId;
  final String nodeId;
  final String nodePublicKey;
  final String? bootstrapNetworkId;
  final List<String> capabilities;
}

class DeviceRegistrationResult {
  const DeviceRegistrationResult({
    required this.device,
    required this.suggestedNodeId,
    required this.suggestedNodePublicKey,
  });

  final DeviceModel device;
  final String suggestedNodeId;
  final String suggestedNodePublicKey;
}

class NodeRegistrationResult {
  const NodeRegistrationResult({
    required this.node,
    required this.bootstrap,
    required this.controlStatus,
    required this.networks,
  });

  final NodeModel node;
  final BootstrapModel? bootstrap;
  final ControlStatusModel? controlStatus;
  final List<NetworkModel> networks;
}

class JoinedNetworkResult {
  const JoinedNetworkResult({
    required this.networkId,
    required this.networks,
  });

  final String networkId;
  final List<NetworkModel> networks;
}

enum NetworkJoinMethod {
  explicit,
  ownerEmail,
  joinKey,
}

class NetworkJoinIntent {
  const NetworkJoinIntent.explicit(String networkId)
      : method = NetworkJoinMethod.explicit,
        value = networkId;

  const NetworkJoinIntent.ownerEmail(String ownerEmail)
      : method = NetworkJoinMethod.ownerEmail,
        value = ownerEmail;

  const NetworkJoinIntent.joinKey(String joinKey)
      : method = NetworkJoinMethod.joinKey,
        value = joinKey;

  final NetworkJoinMethod method;
  final String value;

  bool get isEmpty => value.trim().isEmpty;
}

class DeviceSetupService {
  const DeviceSetupService({
    required AppCoreApi Function() apiProvider,
  }) : _apiProvider = apiProvider;

  final AppCoreApi Function() _apiProvider;

  AppCoreApi get _api => _apiProvider();

  Future<DeviceRegistrationResult> registerDevice(
    DeviceRegistrationInput input,
  ) async {
    final device = await _api.registerDevice(
      name: input.name,
      platform: input.platform,
      deviceVersion: input.deviceVersion,
      machineId: input.machineId,
      publicKey: input.publicKey,
    );
    return DeviceRegistrationResult(
      device: device,
      suggestedNodeId: _generatedNodeId(device.deviceId),
      suggestedNodePublicKey: _generatedPublicKey('node'),
    );
  }

  Future<NodeRegistrationResult> registerNodeAndBootstrap(
    NodeRegistrationInput input, {
    required List<NetworkModel> currentNetworks,
  }) async {
    var networks = currentNetworks;
    if (networks.isEmpty) {
      networks = await _api.listNetworks();
    }

    final node = await _api.registerNode(
      deviceId: input.deviceId,
      nodeId: input.nodeId,
      nodePublicKey: input.nodePublicKey,
      capabilities: input.capabilities,
    );

    final bootstrapNetworkId = _resolveBootstrapNetworkId(
      requestedNetworkId: input.bootstrapNetworkId,
      node: node,
      networks: networks,
    );
    if (bootstrapNetworkId == null || bootstrapNetworkId.isEmpty) {
      return NodeRegistrationResult(
        node: node,
        bootstrap: null,
        controlStatus: null,
        networks: networks,
      );
    }

    final bootstrap = await _api.bootstrap(
      nodeId: node.nodeId,
      networkId: bootstrapNetworkId,
    );
    final controlStatus = await _api.controlStatus();
    return NodeRegistrationResult(
      node: node,
      bootstrap: bootstrap,
      controlStatus: controlStatus,
      networks: bootstrap.networks,
    );
  }

  Future<JoinedNetworkResult> ensureNetworkAvailableAndJoined({
    required DeviceModel device,
    required List<NetworkModel> currentNetworks,
    required String preferredNetworkId,
    required String fallbackNetworkName,
    String fallbackCidr = '10.0.0.0/16',
    NetworkJoinIntent? joinIntent,
  }) async {
    var networks = currentNetworks;
    final intent = joinIntent;
    final isDiscoveryJoin = intent != null &&
        !intent.isEmpty &&
        intent.method != NetworkJoinMethod.explicit;
    if (networks.isEmpty && !isDiscoveryJoin) {
      await _api.createNetwork(
        name: fallbackNetworkName,
        cidr: fallbackCidr,
      );
      networks = await _api.listNetworks();
    }

    var targetNetworkId = preferredNetworkId.trim();
    if (targetNetworkId.isEmpty &&
        intent != null &&
        !intent.isEmpty &&
        intent.method == NetworkJoinMethod.explicit) {
      targetNetworkId = intent.value.trim();
    }
    if (targetNetworkId.isEmpty && networks.isNotEmpty) {
      targetNetworkId = networks.first.networkId;
    }
    if (targetNetworkId.isEmpty && !isDiscoveryJoin) {
      throw StateError('No target network is available.');
    }

    if (intent != null && !intent.isEmpty) {
      switch (intent.method) {
        case NetworkJoinMethod.explicit:
          await _api.joinNetwork(
            networkId: intent.value.trim(),
            deviceId: device.deviceId,
          );
          targetNetworkId = intent.value.trim();
          break;
        case NetworkJoinMethod.ownerEmail:
          await _api.joinNetworkByOwnerEmail(
            ownerEmail: intent.value.trim(),
            deviceId: device.deviceId,
          );
          break;
        case NetworkJoinMethod.joinKey:
          await _api.joinNetworkByKey(
            joinKey: intent.value.trim(),
            deviceId: device.deviceId,
          );
          break;
      }
    } else {
      await _api.joinNetwork(
        networkId: targetNetworkId,
        deviceId: device.deviceId,
      );
    }
    networks = await _api.listNetworks();
    if (intent != null && intent.method != NetworkJoinMethod.explicit) {
      targetNetworkId = _resolveJoinedNetworkId(
        currentNetworks: currentNetworks,
        refreshedNetworks: networks,
        fallbackNetworkId: targetNetworkId,
      );
    }

    return JoinedNetworkResult(
      networkId: targetNetworkId,
      networks: networks,
    );
  }
}

String _resolveJoinedNetworkId({
  required List<NetworkModel> currentNetworks,
  required List<NetworkModel> refreshedNetworks,
  required String fallbackNetworkId,
}) {
  for (final network in refreshedNetworks) {
    final existed = currentNetworks.any(
      (item) => item.networkId == network.networkId,
    );
    if (!existed) {
      return network.networkId;
    }
  }
  return refreshedNetworks.isNotEmpty
      ? refreshedNetworks.first.networkId
      : fallbackNetworkId;
}

String _generatedNodeId(String seed) {
  final normalized = seed.trim().replaceAll(RegExp(r'[^a-zA-Z0-9_-]'), '-');
  return 'node-${normalized.isEmpty ? DateTime.now().millisecondsSinceEpoch : normalized}';
}

String _generatedPublicKey(String prefix) {
  return '$prefix-key-${DateTime.now().microsecondsSinceEpoch}';
}

String? _resolveBootstrapNetworkId({
  required String? requestedNetworkId,
  required NodeModel node,
  required List<NetworkModel> networks,
}) {
  final requested = requestedNetworkId?.trim();
  if (requested != null && requested.isNotEmpty) {
    return requested;
  }
  for (final networkId in node.networkIds) {
    final normalized = networkId.trim();
    if (normalized.isEmpty) {
      continue;
    }
    if (networks.any((network) => network.networkId == normalized)) {
      return normalized;
    }
  }
  return networks.isNotEmpty ? networks.first.networkId : null;
}
