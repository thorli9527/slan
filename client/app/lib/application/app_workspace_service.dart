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

    final activeNetwork = networks.first;
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
  }) async {
    final device = currentDevice;
    final activeNetwork =
        currentNetworks.isNotEmpty ? currentNetworks.first : null;
    if (device != null && activeNetwork != null) {
      await _api.deactivateNetwork(
        networkId: activeNetwork.networkId,
        deviceId: device.deviceId,
      );
    }

    final refreshedNetworks =
        session == null ? currentNetworks : await _api.listNetworks();
    return DeactivatedNetworkResult(
      device: device,
      networks: refreshedNetworks,
    );
  }
}
