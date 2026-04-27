library slan_app.application.app_workspace_service;

import '../infra/app_core/api/app_core_api.dart';
import '../infra/app_core/models/bootstrap_models.dart';
import '../infra/app_core/models/control_models.dart';
import '../infra/app_core/models/identity_models.dart';
import '../infra/app_core/models/network_models.dart';

class WorkspacePreparationResult {
  const WorkspacePreparationResult({
    required this.device,
    required this.networks,
    required this.statusMessage,
    required this.shouldOpenNetworkConsole,
  });

  final DeviceModel? device;
  final List<NetworkModel> networks;
  final String? statusMessage;
  final bool shouldOpenNetworkConsole;
}

class ActiveNetworkRuntimeResult {
  const ActiveNetworkRuntimeResult({
    required this.device,
    required this.node,
    required this.networks,
    required this.bootstrap,
    required this.controlStatus,
    required this.activeNetwork,
  });

  final DeviceModel device;
  final NodeModel node;
  final List<NetworkModel> networks;
  final BootstrapModel bootstrap;
  final ControlStatusModel controlStatus;
  final NetworkModel activeNetwork;
}

class DeactivatedNetworkResult {
  const DeactivatedNetworkResult({
    required this.device,
    required this.networks,
  });

  final DeviceModel? device;
  final List<NetworkModel> networks;
}

class NetworkJoinResult {
  const NetworkJoinResult({
    required this.networks,
    required this.networkId,
  });

  final List<NetworkModel> networks;
  final String networkId;
}

class AppWorkspaceService {
  const AppWorkspaceService({
    required AppCoreApi Function() apiProvider,
  }) : _apiProvider = apiProvider;

  final AppCoreApi Function() _apiProvider;

  AppCoreApi get _api => _apiProvider();

  Future<WorkspacePreparationResult> ensureWorkspaceReady({
    required SessionModel? session,
    required DeviceModel? currentDevice,
    required String deviceName,
    required String platform,
    String? deviceVersion,
    required String machineId,
    required String devicePublicKey,
  }) async {
    if (session == null) {
      return const WorkspacePreparationResult(
        device: null,
        networks: [],
        statusMessage: null,
        shouldOpenNetworkConsole: false,
      );
    }

    var device = currentDevice;
    device ??= await _api.registerDevice(
      name: deviceName,
      platform: platform,
      deviceVersion: deviceVersion,
      machineId: machineId,
      publicKey: devicePublicKey,
    );
    final ensuredDevice = device;

    var networks = await _api.listNetworks();
    final activeNetwork = networks.isNotEmpty ? networks.first : null;
    if (activeNetwork == null) {
      return WorkspacePreparationResult(
        device: ensuredDevice,
        networks: networks,
        statusMessage: null,
        shouldOpenNetworkConsole: true,
      );
    }

    final alreadyJoined = activeNetwork.members.any(
      (member) => member.deviceId == ensuredDevice.deviceId,
    );
    if (!alreadyJoined) {
      await _api.joinNetwork(
        networkId: activeNetwork.networkId,
        deviceId: ensuredDevice.deviceId,
      );
      networks = await _api.listNetworks();
    }

    return WorkspacePreparationResult(
      device: ensuredDevice,
      networks: networks,
      statusMessage: '已发现网络 ${activeNetwork.name}。只有点击“启用网络”后才会建立接入并分配 IP。',
      shouldOpenNetworkConsole: false,
    );
  }

  Future<ActiveNetworkRuntimeResult> prepareActiveNetworkRuntime({
    required SessionModel? session,
    required DeviceModel? currentDevice,
    required NodeModel? currentNode,
    required List<NetworkModel> currentNetworks,
    String? targetNetworkId,
  }) async {
    if (session == null) {
      throw StateError('login required');
    }

    final networks = await _api.listNetworks();

    final device = currentDevice;
    if (device == null || networks.isEmpty) {
      throw StateError('device and network must be ready first');
    }

    var node = currentNode;
    node ??= await _api.registerNode(
      deviceId: device.deviceId,
      nodeId: 'node-${DateTime.now().millisecondsSinceEpoch}',
      nodePublicKey: 'node-key-${DateTime.now().microsecondsSinceEpoch}',
      capabilities: const ['desktop'],
    );

    final activeNetwork = _resolveNetwork(networks, targetNetworkId);
    await _api.activateNetwork(
      networkId: activeNetwork.networkId,
      deviceId: device.deviceId,
    );

    final bootstrap = await _api.bootstrap(
      nodeId: node.nodeId,
      networkId: activeNetwork.networkId,
    );
    final controlStatus = await _api.controlStatus();
    final refreshedNetworks = bootstrap.networks;

    final refreshedActiveNetwork = refreshedNetworks.firstWhere(
      (network) => network.networkId == activeNetwork.networkId,
      orElse: () => activeNetwork,
    );

    return ActiveNetworkRuntimeResult(
      device: bootstrap.device,
      node: node,
      networks: refreshedNetworks,
      bootstrap: bootstrap,
      controlStatus: controlStatus,
      activeNetwork: refreshedActiveNetwork,
    );
  }

  Future<DeactivatedNetworkResult> deactivateActiveNetwork({
    required SessionModel? session,
    required DeviceModel? currentDevice,
    required List<NetworkModel> currentNetworks,
    String? targetNetworkId,
  }) async {
    final device = currentDevice;
    final networks =
        session == null ? currentNetworks : await _api.listNetworks();
    final activeNetwork =
        networks.isEmpty ? null : _resolveNetwork(networks, targetNetworkId);
    if (device != null && activeNetwork != null) {
      await _api.deactivateNetwork(
        networkId: activeNetwork.networkId,
        deviceId: device.deviceId,
      );
    }

    final refreshedNetworks =
        session == null ? networks : await _api.listNetworks();
    return DeactivatedNetworkResult(
      device: device,
      networks: refreshedNetworks,
    );
  }

  Future<NetworkJoinResult> joinNetwork({
    required SessionModel? session,
    required DeviceModel? currentDevice,
    String? ownerEmail,
    String? joinKey,
  }) async {
    if (session == null) {
      throw StateError('login required');
    }
    final device = currentDevice;
    if (device == null) {
      throw StateError('device must be ready first');
    }
    final trimmedKey = joinKey?.trim() ?? '';
    final trimmedOwnerEmail = ownerEmail?.trim() ?? '';
    if (trimmedKey.isEmpty && trimmedOwnerEmail.isEmpty) {
      throw StateError('invite code is required');
    }

    final joinResult = trimmedKey.isNotEmpty
        ? await _api.joinNetworkByKey(
            joinKey: trimmedKey,
            deviceId: device.deviceId,
          )
        : await _api.joinNetworkByOwnerEmail(
            ownerEmail: trimmedOwnerEmail,
            deviceId: device.deviceId,
          );

    var networks = await _api.listNetworks();
    final joinedNetworkId = joinResult.networkId.trim().isEmpty
        ? _findJoinedNetwork(
            networks: networks,
            deviceId: device.deviceId,
          ).networkId
        : joinResult.networkId;

    return NetworkJoinResult(
      networks: networks,
      networkId: joinedNetworkId,
    );
  }

  Future<NetworkJoinResult> switchNetwork({
    required SessionModel? session,
    required DeviceModel? currentDevice,
    required List<NetworkModel> currentNetworks,
    required String networkId,
  }) async {
    if (session == null) {
      throw StateError('login required');
    }
    final device = currentDevice;
    if (device == null) {
      throw StateError('device must be ready first');
    }
    final target = networkId.trim();
    if (target.isEmpty) {
      throw StateError('networkId is required');
    }
    if (currentNetworks.every((network) => network.networkId != target)) {
      throw StateError('network not found: $target');
    }
    final switched = await _api.switchNetwork(
      networkId: target,
      deviceId: device.deviceId,
    );
    final refreshedNetworks = await _api.listNetworks();
    final networks =
        refreshedNetworks.any((network) => network.networkId == target)
            ? refreshedNetworks
            : currentNetworks;
    return NetworkJoinResult(
      networks: networks,
      networkId: switched.networkId.isNotEmpty ? switched.networkId : target,
    );
  }

  NetworkModel _resolveNetwork(
    List<NetworkModel> networks,
    String? targetNetworkId,
  ) {
    final target = targetNetworkId?.trim();
    if (target != null && target.isNotEmpty) {
      for (final network in networks) {
        if (network.networkId == target) {
          return network;
        }
      }
    }
    return networks.first;
  }

  NetworkModel _findJoinedNetwork({
    required List<NetworkModel> networks,
    required String deviceId,
  }) {
    for (final network in networks) {
      if (network.members.any((member) => member.deviceId == deviceId)) {
        return network;
      }
    }
    if (networks.isNotEmpty) {
      return networks.first;
    }
    throw StateError('joined network was not returned by the server');
  }

}
