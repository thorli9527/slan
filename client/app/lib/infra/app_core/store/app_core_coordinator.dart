import 'dart:async';

import 'package:flutter/foundation.dart' show debugPrint;
import 'package:slan_app_core_plugin/slan_app_core_plugin.dart';

import '../../../application/app_workspace_service.dart';
import '../../../application/auth_session_service.dart';
import '../../../application/device_setup_service.dart';
import '../../../application/device_runtime_service.dart';
import '../../../application/local_dns_service.dart';
import '../../../application/tunnel_configuration_service.dart';
import '../../../application/tunnel_host_gateway.dart';
import '../../../application/tunnel_runtime_service.dart';
import '../../mqtt/device_mqtt_service.dart';
import '../scope/app_core_scope.dart';
import '../models/diagnostic_models.dart';
import '../models/identity_models.dart';
import '../models/tunnel_action_models.dart';
import 'app_core_coordinator_async.dart';
import 'app_session_store.dart';
import 'app_tunnel_store.dart';

class AppCoreCoordinator with AppCoreCoordinatorAsync {
  static const defaultAutoNetworkCidr = '10.0.0.0/16';
  AppCoreCoordinator({
    TunnelHostGateway hostGateway = const PluginTunnelHostGateway(),
  }) : _tunnelRuntimeService = TunnelRuntimeService(hostGateway: hostGateway);

  final AppSessionStore sessionStore = AppSessionStore();
  final AppTunnelStore tunnelStore = AppTunnelStore();
  final AppWorkspaceService _workspaceService = AppWorkspaceService(
    apiProvider: () => AppCoreScope.instance,
  );
  final AuthSessionService _authSessionService = AuthSessionService(
    apiProvider: () => AppCoreScope.instance,
  );
  final DeviceSetupService _deviceSetupService = DeviceSetupService(
    apiProvider: () => AppCoreScope.instance,
  );
  final DeviceRuntimeService _deviceRuntimeService = DeviceRuntimeService(
    apiProvider: () => AppCoreScope.instance,
  );
  final TunnelRuntimeService _tunnelRuntimeService;
  final TunnelConfigurationService _tunnelConfigurationService =
      const TunnelConfigurationService();
  Timer? _networkStateTimer;
  Timer? _controlSyncTimer;
  bool _controlSyncInFlight = false;

  @override
  bool get busy => sessionStore.busy;
  @override
  set busy(bool value) => sessionStore.busy = value;

  @override
  String? get error => sessionStore.error;
  @override
  set error(String? value) => sessionStore.error = value;

  @override
  DataPlaneProbeModel? get lastProbe => tunnelStore.lastProbe;
  @override
  set lastProbe(DataPlaneProbeModel? value) => tunnelStore.lastProbe = value;

  @override
  ProbeFailure? get lastProbeFailure => tunnelStore.lastProbeFailure;
  @override
  set lastProbeFailure(ProbeFailure? value) =>
      tunnelStore.lastProbeFailure = value;

  @override
  int? get lastSendBytes => tunnelStore.lastSendBytes;
  @override
  set lastSendBytes(int? value) => tunnelStore.lastSendBytes = value;

  @override
  SendFailure? get lastSendFailure => tunnelStore.lastSendFailure;
  @override
  set lastSendFailure(SendFailure? value) =>
      tunnelStore.lastSendFailure = value;

  @override
  String? get tunnelDebugError => tunnelStore.tunnelDebugError;
  @override
  set tunnelDebugError(String? value) => tunnelStore.tunnelDebugError = value;

  @override
  void emitStateChanged() {
    sessionStore.emit();
    tunnelStore.emit();
  }

  void resetState() {
    _networkStateTimer?.cancel();
    _networkStateTimer = null;
    _controlSyncTimer?.cancel();
    _controlSyncTimer = null;
    _controlSyncInFlight = false;
    unawaited(DeviceMqttService.instance.close());
    sessionStore.resetAll();
    tunnelStore.resetAll();
  }

  void resetForServerSwitch() {
    resetState();
    emitStateChanged();
  }

  Future<void> applyExternalSession(SessionModel nextSession) async {
    await applyExternalSessionInternal(nextSession, persistSession: true);
  }

  Future<void> applyExternalSessionInternal(
    SessionModel nextSession, {
    required bool persistSession,
  }) async {
    await runAction(() async {
      debugPrint(
        '[auth-callback] store applyExternalSessionInternal start userId=${nextSession.userId} deviceId=${nextSession.deviceId}',
      );
      resetState();
      sessionStore.session = nextSession;
      sessionStore.notice = '已收到浏览器登录回调，正在恢复客户端会话。';
      emitStateChanged();
      debugPrint(
          '[auth-callback] store session assigned and listeners notified');
      final hydrated =
          await _authSessionService.hydrateExternalSession(nextSession);
      await applyHydratedSession(hydrated, persistSession: persistSession);
    });
  }

  Future<void> refreshPersistedSession(SessionModel session) async {
    await runAction(() async {
      debugPrint(
        '[auth-refresh] store refreshPersistedSession start userId=${session.userId} deviceId=${session.deviceId}',
      );
      resetState();
      sessionStore.session = session;
      sessionStore.notice = 'refreshing login session';
      emitStateChanged();
      final hydrated =
          await _authSessionService.refreshAndHydrateSession(session);
      await applyHydratedSession(hydrated, persistSession: true);
    });
  }

  Future<void> applyHydratedSession(
    AuthSessionHydrationResult hydrated, {
    required bool persistSession,
  }) async {
    sessionStore.session = hydrated.session;
    if (persistSession) {
      await AppCoreScope.persistSession(hydrated.session);
    }
    if (hydrated.device != null) {
      sessionStore.devices = hydrated.devices;
      sessionStore.syncDevice(hydrated.device);
      sessionStore.networks = hydrated.networks;
      sessionStore.syncSelectedNetworkId();
      await _connectMqttAndMarkOnline(hydrated.device!);
      emitStateChanged();
      debugPrint(
        '[auth-callback] store matched device=${hydrated.device!.deviceId}',
      );
    } else {
      sessionStore.devices = hydrated.devices;
      sessionStore.networks = hydrated.networks;
      sessionStore.syncSelectedNetworkId();
    }
    debugPrint(
        '[auth-callback] store loaded networks count=${sessionStore.networks.length}');
    sessionStore.notice = hydrated.notice;
    debugPrint('[auth-callback] store final notice=${sessionStore.notice}');
  }

  Future<void> signOut() async {
    await runAction(() async {
      try {
        await bringTunnelDown();
      } catch (_) {
        // Best effort. Local state still needs to be cleared even if the
        // tunnel runtime is already gone or not initialized.
      }
      await LocalDnsService.instance.stop();
      await _reportDeviceNetworkState(networkOnline: false, tunnelUp: false);
      await AppCoreScope.clearPersistedSession();
      resetState();
    });
  }

  Future<void> _connectMqttAndMarkOnline(DeviceModel device) async {
    final connected = await DeviceMqttService.instance.connectForDevice(device);
    if (!connected) {
      return;
    }
    try {
      await _reportDeviceNetworkState(
        deviceId: device.deviceId,
        controlReachable: true,
        networkOnline: false,
        tunnelUp: false,
      );
    } catch (_) {
      // The MQTT auth provider also marks the control channel reachable.
    }
    _startDeviceNetworkHeartbeat(networkOnline: false, tunnelUp: false);
  }

  Future<void> _reportDeviceNetworkState({
    String? deviceId,
    bool controlReachable = true,
    required bool networkOnline,
    required bool tunnelUp,
    bool? lastProbeOk,
  }) async {
    final targetDevice = deviceId ?? sessionStore.device?.deviceId;
    final targetNetwork = sessionStore.selectedNetworkId;
    if (targetDevice == null ||
        targetDevice.isEmpty ||
        targetNetwork == null ||
        targetNetwork.isEmpty) {
      return;
    }
    final virtualIp = sessionStore.device?.virtualIp;
    final probeOk =
        lastProbeOk ?? tunnelStore.lastProbe?.replyObserved ?? false;
    final reportedAt = DateTime.now().millisecondsSinceEpoch ~/ 1000;
    try {
      await DeviceMqttService.instance.publishNetworkState(
        device: sessionStore.device,
        networkId: targetNetwork,
        controlReachable: controlReachable,
        networkOnline: networkOnline,
        tunnelUp: tunnelUp,
        lastProbeOk: probeOk,
        virtualIp: virtualIp,
        reportedAt: reportedAt,
      );
    } catch (error) {
      debugPrint('[device-mqtt] network state publish failed: $error');
    }
    await AppCoreScope.instance.setDeviceNetworkState(
      deviceId: targetDevice,
      networkId: targetNetwork,
      controlReachable: controlReachable,
      networkOnline: networkOnline,
      tunnelUp: tunnelUp,
      lastProbeOk: probeOk,
      virtualIp: virtualIp,
      reportedAt: reportedAt,
    );
  }

  void _startDeviceNetworkHeartbeat({
    required bool networkOnline,
    required bool tunnelUp,
  }) {
    _networkStateTimer?.cancel();
    _networkStateTimer = Timer.periodic(const Duration(seconds: 15), (_) {
      unawaited(_reportDeviceNetworkState(
        networkOnline: networkOnline,
        tunnelUp: tunnelUp,
      ));
    });
  }

  Future<void> registerDevice({
    required String name,
    required String platform,
    String? deviceVersion,
    required String machineId,
    required String publicKey,
  }) async {
    await runAction(() async {
      final result = await _deviceSetupService.registerDevice(
        DeviceRegistrationInput(
          name: name,
          platform: platform,
          deviceVersion: deviceVersion,
          machineId: machineId,
          publicKey: publicKey,
        ),
      );
      if (result.device != null) {
        sessionStore.syncDevice(result.device);
        await _connectMqttAndMarkOnline(result.device!);
      }
    });
  }

  Future<void> refreshDeviceInventory() async {
    await runAction(() async {
      if (sessionStore.session == null) {
        throw StateError('login required');
      }
      final devices = await AppCoreScope.instance.listDevices();
      sessionStore.devices = devices;
      final currentDeviceId =
          sessionStore.device?.deviceId ?? sessionStore.session?.deviceId;
      if (currentDeviceId != null && currentDeviceId.isNotEmpty) {
        for (final device in devices) {
          if (device.deviceId == currentDeviceId) {
            sessionStore.syncDevice(device);
            break;
          }
        }
      } else if (devices.isNotEmpty) {
        sessionStore.syncDevice(devices.first);
      }
    });
  }

  Future<String?> ensureHomeWorkspaceReady({
    required String deviceName,
    required String platform,
    String? deviceVersion,
    required String machineId,
    required String devicePublicKey,
  }) async {
    String? statusMessage;
    await runAction(() async {
      final result = await _workspaceService.ensureWorkspaceReady(
        session: sessionStore.session,
        currentDevice: sessionStore.device,
        deviceName: deviceName,
        platform: platform,
        deviceVersion: deviceVersion,
        machineId: machineId,
        devicePublicKey: devicePublicKey,
      );
      if (result.device != null) {
        sessionStore.syncDevice(result.device);
        await _connectMqttAndMarkOnline(result.device!);
      }
      sessionStore.networks = result.networks;
      sessionStore.syncSelectedNetworkId();
      statusMessage = result.statusMessage;
    });
    return statusMessage;
  }

  Future<void> registerNode({
    required String deviceId,
    required String nodeId,
    required String nodePublicKey,
    List<String> capabilities = const [],
    String? bootstrapNetworkId,
  }) async {
    await runAction(() async {
      final result = await _deviceSetupService.registerNodeAndBootstrap(
        NodeRegistrationInput(
          deviceId: deviceId,
          nodeId: nodeId,
          nodePublicKey: nodePublicKey,
          bootstrapNetworkId: bootstrapNetworkId,
          capabilities: capabilities.isEmpty ? const ['desktop'] : capabilities,
        ),
        currentNetworks: sessionStore.networks,
      );
      sessionStore.node = result.node;
      if (result.bootstrap != null) {
        sessionStore.bootstrap = result.bootstrap;
        sessionStore.controlStatus = result.controlStatus;
        sessionStore.syncDevice(result.bootstrap!.device);
      }
      sessionStore.networks = result.networks;
      sessionStore.syncSelectedNetworkId();
    });
  }

  Future<String> ensureNetworkAvailableAndJoined({
    required DeviceModel currentDevice,
    required String preferredNetworkId,
    required String fallbackNetworkName,
    String fallbackCidr = defaultAutoNetworkCidr,
    NetworkJoinIntent? joinIntent,
  }) async {
    late String targetNetworkId;
    await runAction(() async {
      final result = await _deviceSetupService.ensureNetworkAvailableAndJoined(
        device: currentDevice,
        currentNetworks: sessionStore.networks,
        preferredNetworkId: preferredNetworkId,
        fallbackNetworkName: fallbackNetworkName,
        fallbackCidr: fallbackCidr,
        joinIntent: joinIntent,
      );
      sessionStore.networks = result.networks;
      sessionStore.syncSelectedNetworkId(preferredNetworkId: result.networkId);
      targetNetworkId = result.networkId;
    });
    return targetNetworkId;
  }

  Future<void> selectNetwork(String networkId) async {
    await runAction(() async {
      final target = networkId.trim();
      if (target.isEmpty) {
        throw StateError('networkId is required');
      }
      if (sessionStore.networks
          .every((network) => network.networkId != target)) {
        throw StateError('network not found: $target');
      }
      final result = await _workspaceService.switchNetwork(
        session: sessionStore.session,
        currentDevice: sessionStore.device,
        currentNetworks: sessionStore.networks,
        networkId: target,
      );
      sessionStore.networks = result.networks;
      sessionStore.syncSelectedNetworkId(preferredNetworkId: result.networkId);
    });
  }

  Future<void> joinNetwork({
    String? ownerEmail,
    String? joinKey,
  }) async {
    await runAction(() async {
      final result = await _workspaceService.joinNetwork(
        session: sessionStore.session,
        currentDevice: sessionStore.device,
        ownerEmail: ownerEmail,
        joinKey: joinKey,
      );
      sessionStore.networks = result.networks;
      sessionStore.syncSelectedNetworkId(preferredNetworkId: result.networkId);
      sessionStore.notice = 'Network joined: ${result.networkId}';
    });
  }

  Future<void> createNetwork({
    required String name,
    String? cidr,
    String? allocationStartIp,
    String? allocationEndIp,
  }) async {
    await runAction(() async {
      if (sessionStore.session == null) {
        throw StateError('login required');
      }
      final device = sessionStore.device;
      if (device == null) {
        throw StateError('device must be ready first');
      }
      final network = await AppCoreScope.instance.createNetwork(
        name: name,
        cidr: cidr,
        allocationStartIp: allocationStartIp,
        allocationEndIp: allocationEndIp,
        bindDeviceId: device.deviceId,
      );
      sessionStore.networks = await AppCoreScope.instance.listNetworks();
      sessionStore.syncSelectedNetworkId(preferredNetworkId: network.networkId);
      sessionStore.notice = 'Network created: ${network.networkId}';
    });
  }

  Future<void> refreshNetworks() async {
    await runAction(() async {
      if (sessionStore.session == null) {
        throw StateError('login required');
      }
      sessionStore.networks = await AppCoreScope.instance.listNetworks();
      sessionStore.syncSelectedNetworkId();
    });
  }

  Future<void> enableActiveNetwork() async {
    await runAction(() async {
      if (sessionStore.session == null) {
        throw StateError('login required');
      }
      if (sessionStore.device == null) {
        throw StateError('当前设备尚未注册完成。');
      }
      if (sessionStore.networks.isEmpty) {
        sessionStore.notice = '当前账号还没有可启用的网络，请先在网页端创建或接入网络。';
        return;
      }
      sessionStore.syncSelectedNetworkId();
      final runtime = await _workspaceService.prepareActiveNetworkRuntime(
        session: sessionStore.session,
        currentDevice: sessionStore.device,
        currentNode: sessionStore.node,
        currentNetworks: sessionStore.networks,
        targetNetworkId: sessionStore.selectedNetworkId,
      );
      debugPrint(
        '[tunnel-enable] runtime ready activeNetwork=${runtime.activeNetwork.networkId} node=${runtime.node?.nodeId} device=${runtime.device?.deviceId}',
      );
      sessionStore.node = runtime.node;
      sessionStore.bootstrap = runtime.bootstrap;
      sessionStore.controlStatus = runtime.controlStatus;
      sessionStore.syncDevice(runtime.device);
      sessionStore.networks = runtime.networks;
      sessionStore.syncSelectedNetworkId(
        preferredNetworkId: runtime.activeNetwork.networkId,
      );

      final config =
          _tunnelConfigurationService.buildActiveNetworkConfiguration(
        network: runtime.activeNetwork,
        deviceId: sessionStore.device!.deviceId,
        devicePublicKey: sessionStore.device!.publicKey,
      );
      final peerVirtualIp = config.peer.allowedIps.first.split('/').first;
      debugPrint(
        '[tunnel-enable] apply start peerVirtualIp=$peerVirtualIp listenPort=${config.interface.listenPort}',
      );
      final applyReport = await applyTunnelConfiguration(
        configuration: config,
        verifyPeerVirtualIp: peerVirtualIp,
      );
      debugPrint(
        '[tunnel-enable] apply done succeeded=${applyReport.succeeded} phase=${applyReport.phase} error=${applyReport.errorMessage}',
      );
      if (!applyReport.succeeded) {
        return;
      }
      debugPrint('[tunnel-enable] bring-up start peerVirtualIp=$peerVirtualIp');
      await bringTunnelUp(verifyPeerVirtualIp: peerVirtualIp);
      await LocalDnsService.instance.configureFromNetwork(runtime.activeNetwork);
      await _reportDeviceNetworkState(
        networkOnline: true,
        tunnelUp: true,
        lastProbeOk: true,
      );
      _startDeviceNetworkHeartbeat(networkOnline: true, tunnelUp: true);
      _syncControlSyncPollingState();
      debugPrint('[tunnel-enable] bring-up done peerVirtualIp=$peerVirtualIp');
    });
  }

  Future<void> disableActiveNetwork() async {
    await runAction(() async {
      if (sessionStore.networks.isEmpty) {
        sessionStore.notice = '当前没有已接入的活动网络。';
        return;
      }
      await bringTunnelDown();
      await LocalDnsService.instance.stop();
      _networkStateTimer?.cancel();
      _networkStateTimer = null;
      _stopControlSyncPolling();
      await _reportDeviceNetworkState(networkOnline: false, tunnelUp: false);
      _startDeviceNetworkHeartbeat(networkOnline: false, tunnelUp: false);
      final result = await _workspaceService.deactivateActiveNetwork(
        session: sessionStore.session,
        currentDevice: sessionStore.device,
        currentNetworks: sessionStore.networks,
        targetNetworkId: sessionStore.selectedNetworkId,
      );
      final currentDevice = result.device;
      sessionStore.networks = result.networks;
      sessionStore.syncSelectedNetworkId();
      if (currentDevice != null) {
        sessionStore.syncDevice(DeviceModel(
          deviceId: currentDevice.deviceId,
          name: currentDevice.name,
          platform: currentDevice.platform,
          deviceVersion: currentDevice.deviceVersion,
          status: currentDevice.status,
          publicKey: currentDevice.publicKey,
          ownerEmail: currentDevice.ownerEmail,
          linkStatus: currentDevice.linkStatus,
          connectivityProtocol: currentDevice.connectivityProtocol,
          joinedAt: currentDevice.joinedAt,
          membershipStatus: currentDevice.membershipStatus,
          networkRole: currentDevice.networkRole,
          createdAt: currentDevice.createdAt,
          networkIds: currentDevice.networkIds,
          mqtt: currentDevice.mqtt,
        ));
      }
      sessionStore.notice = '当前网络已停用，本地隧道和虚拟 IP 已释放。';
      sessionStore.bootstrap = null;
      sessionStore.controlStatus = null;
      sessionStore.clearConnection();
      tunnelStore.clearConnection();
      tunnelStore.lastTunnelActionReport = const TunnelActionReport(
        succeeded: true,
        detail: '当前网络已停用，本地隧道运行态已清空。',
        source: TunnelActionReportSource.runtimeSnapshot,
        phase: TunnelActionPhase.verified,
      );
    });
  }

  Future<void> loadBootstrap({
    String? nodeId,
    String? networkId,
  }) async {
    final targetNodeId = nodeId?.trim().isNotEmpty == true
        ? nodeId!.trim()
        : sessionStore.node?.nodeId;
    final targetNetworkId = networkId?.trim().isNotEmpty == true
        ? networkId!.trim()
        : sessionStore.networks.isNotEmpty
            ? sessionStore.selectedNetwork?.networkId
            : null;
    if (targetNodeId == null ||
        targetNodeId.isEmpty ||
        targetNetworkId == null) {
      error = 'nodeId and networkId are required';
      emitStateChanged();
      return;
    }
    await runAction(() async {
      sessionStore.bootstrap = await AppCoreScope.instance.bootstrap(
        nodeId: targetNodeId,
        networkId: targetNetworkId,
      );
      sessionStore.controlStatus = await AppCoreScope.instance.controlStatus();
      sessionStore.syncDevice(sessionStore.bootstrap!.device);
      sessionStore.networks = sessionStore.bootstrap!.networks;
      sessionStore.syncSelectedNetworkId(preferredNetworkId: targetNetworkId);
    });
  }

  Future<String> refreshBootstrapOrControlSync() async {
    late String detail;
    final previousLocalVirtualIp = tunnelStore.tunnelRuntimeView?.localVirtualIp;
    await runAction(() async {
      detail = await _refreshBootstrapOrControlSyncState();
    });
    await _reapplyActiveNetworkTunnelIfVirtualIpChanged(previousLocalVirtualIp);
    _syncControlSyncPollingState();
    return detail;
  }

  Future<String> _refreshBootstrapOrControlSyncState() async {
    final result = await _deviceRuntimeService.refreshBootstrap(
      node: sessionStore.node,
      currentBootstrap: sessionStore.bootstrap,
      currentNetworks: sessionStore.networks,
    );
    sessionStore.bootstrap = result.bootstrap;
    sessionStore.controlStatus = result.controlStatus;
    sessionStore.syncDevice(result.bootstrap.device);
    sessionStore.networks = result.bootstrap.networks;
    sessionStore.syncSelectedNetworkId(
      preferredNetworkId: result.bootstrap.networks.isEmpty
          ? null
          : result.bootstrap.networks.first.networkId,
    );
    return result.detail;
  }

  void _startControlSyncPolling() {
    _controlSyncTimer?.cancel();
    _controlSyncTimer = Timer.periodic(const Duration(seconds: 10), (_) {
      unawaited(_backgroundControlSyncOnce());
    });
  }

  void _syncControlSyncPollingState() {
    if (_canRunControlSync() && tunnelStore.tunnelRuntimeView != null) {
      _startControlSyncPolling();
      return;
    }
    _stopControlSyncPolling();
  }

  void _stopControlSyncPolling() {
    _controlSyncTimer?.cancel();
    _controlSyncTimer = null;
    _controlSyncInFlight = false;
  }

  Future<void> _backgroundControlSyncOnce() async {
    if (_controlSyncInFlight ||
        sessionStore.session == null ||
        sessionStore.device == null ||
        sessionStore.selectedNetworkId == null ||
        !_canRunControlSync()) {
      return;
    }
    _controlSyncInFlight = true;
    final previousLocalVirtualIp = tunnelStore.tunnelRuntimeView?.localVirtualIp;
    try {
      await _refreshBootstrapOrControlSyncState();
      emitStateChanged();
      await _reapplyActiveNetworkTunnelIfVirtualIpChanged(previousLocalVirtualIp);
    } catch (error) {
      debugPrint('[control-sync] background sync skipped: $error');
    } finally {
      _controlSyncInFlight = false;
    }
  }

  bool _canRunControlSync() {
    final controlPlane = sessionStore.bootstrap?.controlPlane;
    return controlPlane?.sessionToken?.isNotEmpty == true &&
        controlPlane?.wsUrl.trim().isNotEmpty == true;
  }

  Future<void> _reapplyActiveNetworkTunnelIfVirtualIpChanged(
    String? previousLocalVirtualIp,
  ) async {
    final runtime = tunnelStore.tunnelRuntimeView;
    final device = sessionStore.device;
    final network = sessionStore.selectedNetwork;
    if (runtime == null || device == null || network == null) {
      return;
    }
    final config = _tunnelConfigurationService.buildActiveNetworkConfiguration(
      network: network,
      deviceId: device.deviceId,
      devicePublicKey: device.publicKey,
    );
    if (previousLocalVirtualIp == config.localVirtualIp &&
        runtime.localVirtualIp == config.localVirtualIp) {
      return;
    }
    final peerVirtualIp = config.peer.allowedIps.first.split('/').first;
    debugPrint(
      '[tunnel-refresh] virtual ip changed from ${previousLocalVirtualIp ?? runtime.localVirtualIp} to ${config.localVirtualIp}; reapplying tunnel',
    );
    final applyReport = await applyTunnelConfiguration(
      configuration: config,
      verifyPeerVirtualIp: peerVirtualIp,
    );
    if (!applyReport.succeeded) {
      await _reportDeviceNetworkState(networkOnline: false, tunnelUp: false);
      return;
    }
    await bringTunnelUp(verifyPeerVirtualIp: peerVirtualIp);
    await LocalDnsService.instance.configureFromNetwork(network);
    await _reportDeviceNetworkState(
      networkOnline: true,
      tunnelUp: true,
      lastProbeOk: true,
    );
    _startDeviceNetworkHeartbeat(networkOnline: true, tunnelUp: true);
  }

  Future<ConnectAttemptResult> connectUsingControlPlan({
    required String networkId,
    required String peerNodeId,
    required String reason,
  }) async {
    late ConnectAttemptResult result;
    await runAction(() async {
      result = await _deviceRuntimeService.connectWithFallback(
        networkId: networkId,
        peerNodeId: peerNodeId,
        reason: reason,
        currentNode: sessionStore.node,
        controlStatus: sessionStore.controlStatus,
        lastProbe: lastProbe,
      );
      sessionStore.relayTicket = result.relayTicket;
      sessionStore.connectionState = result.connectionState;
    });
    return result;
  }

  Future<void> disconnect() async {
    await runAction(() async {
      await AppCoreScope.instance.disconnect();
      sessionStore.clearConnection();
      tunnelStore.clearConnection();
    });
  }

  Future<TunnelActionReport> applyTunnelConfiguration({
    required WireGuardTunnelConfiguration configuration,
    String? verifyPeerVirtualIp,
  }) async {
    final report = await runTunnelAction<TunnelActionReport>(() async {
      final result = await _tunnelRuntimeService.applyTunnelConfiguration(
        configuration: configuration,
        verifyPeerVirtualIp: verifyPeerVirtualIp,
      );
      tunnelStore.tunnelRuntimeView = result.runtimeView;
      return result.report;
    });
    tunnelStore.lastTunnelActionReport = report ?? _failedTunnelActionReport();
    return tunnelStore.lastTunnelActionReport!;
  }

  Future<TunnelActionReport> bringTunnelUp({
    String? verifyPeerVirtualIp,
  }) async {
    final report = await runTunnelAction<TunnelActionReport>(() async {
      final result = await _tunnelRuntimeService.bringTunnelUp(
        verifyPeerVirtualIp: verifyPeerVirtualIp,
      );
      tunnelStore.tunnelRuntimeView = result.runtimeView;
      return result.report;
    });
    tunnelStore.lastTunnelActionReport = report ?? _failedTunnelActionReport();
    return tunnelStore.lastTunnelActionReport!;
  }

  Future<TunnelActionReport> bringTunnelDown() async {
    final report = await runTunnelAction<TunnelActionReport>(() async {
      final result = await _tunnelRuntimeService.bringTunnelDown();
      tunnelStore.tunnelRuntimeView = result.runtimeView;
      return result.report;
    });
    tunnelStore.lastTunnelActionReport = report ?? _failedTunnelActionReport();
    return tunnelStore.lastTunnelActionReport!;
  }

  Future<TunnelActionReport> removeTunnelPeer({
    required String peerVirtualIp,
  }) async {
    final report = await runTunnelAction<TunnelActionReport>(() async {
      final result = await _tunnelRuntimeService.removeTunnelPeer(
        peerVirtualIp: peerVirtualIp,
      );
      tunnelStore.tunnelRuntimeView = result.runtimeView;
      return result.report;
    });
    tunnelStore.lastTunnelActionReport = report ?? _failedTunnelActionReport();
    return tunnelStore.lastTunnelActionReport!;
  }

  Future<TunnelActionReport> refreshTunnelRuntime({
    required String peerVirtualIp,
  }) async {
    final report = await runTunnelAction<TunnelActionReport>(() async {
      final result = await _tunnelRuntimeService.refreshTunnelRuntime(
        peerVirtualIp: peerVirtualIp,
      );
      tunnelStore.tunnelRuntimeView = result.runtimeView;
      return result.report;
    });
    tunnelStore.lastTunnelActionReport = report ?? _failedTunnelActionReport();
    return tunnelStore.lastTunnelActionReport!;
  }

  Future<PlatformDoctorModel> refreshPlatformDoctor() async {
    final report = await runTunnelAction<PlatformDoctorModel>(() {
      return AppCoreScope.instance.platformDoctor();
    });
    tunnelStore.platformDoctor = report ?? _failedPlatformDoctor();
    emitStateChanged();
    return tunnelStore.platformDoctor!;
  }

  Future<PlatformInstallPlanModel> refreshPlatformInstallPlan() async {
    final plan = await runTunnelAction<PlatformInstallPlanModel>(() {
      return AppCoreScope.instance.platformInstallPlan();
    });
    tunnelStore.platformInstallPlan = plan ?? _failedPlatformInstallPlan();
    emitStateChanged();
    return tunnelStore.platformInstallPlan!;
  }

  PlatformDoctorModel _failedPlatformDoctor() {
    return PlatformDoctorModel(
      platform: const PlatformInfoModel(os: 'unknown'),
      tunnelBackend: const TunnelBackendDiagnosticsModel(
        name: 'unavailable',
        isUp: false,
        plannedPeerCount: 0,
        recentCommandCount: 0,
      ),
      checks: [
        PlatformCheckModel(
          name: 'platform_doctor',
          status: 'fail',
          detail: tunnelDebugError ?? error ?? 'platform doctor failed',
        ),
      ],
    );
  }

  PlatformInstallPlanModel _failedPlatformInstallPlan() {
    return PlatformInstallPlanModel(
      platform: const PlatformInfoModel(os: 'unknown'),
      warnings: [
        tunnelDebugError ?? error ?? 'platform install plan failed',
      ],
    );
  }

  TunnelActionReport _failedTunnelActionReport() {
    return TunnelActionReport(
      succeeded: false,
      detail:
          'Native tunnel action failed before a verified backend signal arrived.',
      source: TunnelActionReportSource.pluginHost,
      phase: TunnelActionPhase.failed,
      errorMessage:
          tunnelDebugError ?? error ?? 'unknown tunnel action failure',
      runtimeSnapshot: tunnelStore.tunnelRuntimeView,
    );
  }

  Future<void> probe({
    required String payload,
    int? probeTimeoutMs,
  }) async {
    await runProbeAction(() {
      return AppCoreScope.instance.probe(
        payload: payload,
        probeTimeoutMs: probeTimeoutMs,
      );
    });
  }

  Future<void> send({
    required String payload,
  }) async {
    await runSendAction(() => AppCoreScope.instance.send(payload: payload));
  }
}
