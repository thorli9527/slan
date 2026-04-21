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
    required this.machineId,
    required this.publicKey,
  });

  final String name;
  final String platform;
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

    final bootstrapNetworkId =
        input.bootstrapNetworkId?.trim().isNotEmpty == true
            ? input.bootstrapNetworkId!.trim()
            : networks.isNotEmpty
                ? networks.first.networkId
                : null;
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
  }) async {
    var networks = currentNetworks;
    if (networks.isEmpty) {
      await _api.createNetwork(
        name: fallbackNetworkName,
        cidr: fallbackCidr,
      );
      networks = await _api.listNetworks();
    }

    var targetNetworkId = preferredNetworkId.trim();
    if (targetNetworkId.isEmpty && networks.isNotEmpty) {
      targetNetworkId = networks.first.networkId;
    }
    if (targetNetworkId.isEmpty) {
      throw StateError('No target network is available.');
    }

    await _api.joinNetwork(
      networkId: targetNetworkId,
      deviceId: device.deviceId,
    );
    networks = await _api.listNetworks();

    return JoinedNetworkResult(
      networkId: targetNetworkId,
      networks: networks,
    );
  }
}

String _generatedNodeId(String seed) {
  final normalized = seed.trim().replaceAll(RegExp(r'[^a-zA-Z0-9_-]'), '-');
  return 'node-${normalized.isEmpty ? DateTime.now().millisecondsSinceEpoch : normalized}';
}

String _generatedPublicKey(String prefix) {
  return '$prefix-key-${DateTime.now().microsecondsSinceEpoch}';
}
