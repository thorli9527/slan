import 'dart:async';

import 'package:flutter/foundation.dart' show debugPrint, listEquals;
import 'package:slan_app_core_plugin/slan_app_core_plugin.dart';

import '../../../application/app_workspace_service.dart';
import '../../../application/auth_session_service.dart';
import '../../../application/device_setup_service.dart';
import '../../../application/device_runtime_service.dart';
import '../../../application/local_dns_service.dart';
import '../../../application/tunnel_configuration_service.dart';
import '../../../application/tunnel_host_gateway.dart';
import '../../../application/tunnel_runtime_service.dart';
import '../scope/app_core_scope.dart';
import '../models/diagnostic_models.dart';
import '../models/identity_models.dart';
import '../models/tunnel_action_models.dart';
import 'app_core_coordinator_async.dart';
import 'app_session_store.dart';
import 'app_tunnel_store.dart';

class AppCoreCoordinator with AppCoreCoordinatorAsync {
  static const defaultAutoNetworkCidr = '10.0.0.0/16';
  static const _networkStateHeartbeatInterval = Duration(seconds: 15);

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
  Timer? _networkStateHeartbeatTimer;
  bool _networkStateHeartbeatInFlight = false;
  bool get _serviceOwnsLocalNetwork => AppCoreScope.mode == 'bridge';
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
    _stopNetworkStateHeartbeat();
    sessionStore.resetAll();
    tunnelStore.resetAll();
  }

  void resetForServerSwitch() {
    resetState();
    emitStateChanged();
  }

  Future<void> applyExternalSession(SessionModel nextSession) async {
    await applyExternalSessionInternal(
      nextSession,
      persistSession: true,
      autoEnableLastNetwork: true,
    );
  }

  Future<void> applyExternalSessionInternal(
    SessionModel nextSession, {
    required bool persistSession,
    bool autoEnableLastNetwork = false,
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
      if (autoEnableLastNetwork) {
        await _autoEnableLastNetworkIfRequested();
      }
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
      _startNetworkStateHeartbeat();
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
      _stopNetworkStateHeartbeat();
      final peerVirtualIp = tunnelStore.tunnelRuntimeView?.peerVirtualIp.trim();
      await _persistNetworkUsageState(enabled: false);
      if (_serviceOwnsLocalNetwork) {
        try {
          await AppCoreScope.instance.disableLocalNetwork(
            networkId: sessionStore.selectedNetworkId,
          );
        } catch (error) {
          debugPrint('[logout] service disable network skipped: $error');
        }
      } else {
        try {
          await bringTunnelDown();
        } catch (_) {
          // Best effort. Local state still needs to be cleared even if the
          // tunnel runtime is already gone or not initialized.
        }
        if (peerVirtualIp != null && peerVirtualIp.isNotEmpty) {
          try {
            await removeTunnelPeer(peerVirtualIp: peerVirtualIp);
          } catch (_) {
            // Best effort. The app-core disconnect below also attempts to close
            // the current tunnel peer.
          }
        }
        try {
          await LocalDnsService.instance.stop();
        } catch (error) {
          debugPrint('[logout] stop dns skipped: $error');
        }
        try {
          await _reportDeviceNetworkState(
            networkOnline: false,
            tunnelUp: false,
          );
        } catch (error) {
          debugPrint('[logout] offline report skipped: $error');
        }
      }
      try {
        await AppCoreScope.instance.disconnect();
      } catch (error) {
        debugPrint('[logout] app-core disconnect skipped: $error');
        // Best effort. Signing out must clear the Flutter session even if the
        // embedded core has already torn down its runtime.
      }
      try {
        await AppCoreScope.clearPersistedSession();
      } catch (error) {
        debugPrint('[logout] clear persisted session skipped: $error');
      }
      resetState();
    });
  }

  Future<void> _connectMqttAndMarkOnline(DeviceModel device) async {
    final networkOnline = _hasActiveTunnelRuntime();
    try {
      await _reportDeviceNetworkState(
        deviceId: device.deviceId,
        controlReachable: true,
        networkOnline: networkOnline,
        tunnelUp: networkOnline,
        lastProbeOk: networkOnline ? true : null,
      );
    } catch (_) {
      // The MQTT auth provider also marks the control channel reachable.
    }
  }

  void _startNetworkStateHeartbeat() {
    _networkStateHeartbeatTimer?.cancel();
    unawaited(_sendNetworkStateHeartbeat());
    _networkStateHeartbeatTimer = Timer.periodic(
      _networkStateHeartbeatInterval,
      (_) => unawaited(_sendNetworkStateHeartbeat()),
    );
  }

  void _stopNetworkStateHeartbeat() {
    _networkStateHeartbeatTimer?.cancel();
    _networkStateHeartbeatTimer = null;
    _networkStateHeartbeatInFlight = false;
  }

  Future<void> _sendNetworkStateHeartbeat() async {
    if (_networkStateHeartbeatInFlight || sessionStore.session == null) {
      return;
    }
    _networkStateHeartbeatInFlight = true;
    try {
      if (_serviceOwnsLocalNetwork) {
        try {
          await AppCoreScope.instance.reportDeviceNetworkState();
        } catch (error) {
          debugPrint('[network-heartbeat] service state report skipped: $error');
        }
        if (!sessionStore.busy && !_hasActiveTunnelRuntime()) {
          await _refreshNetworksForRemoteControl();
        }
        return;
      }
      if (!sessionStore.busy && _hasActiveTunnelRuntime()) {
        final runtimeStillActive = await _refreshActiveTunnelRuntimeFromNative();
        if (!runtimeStillActive) {
          await _markLocalTunnelRuntimeOffline(
            reason: '本地网络适配器或虚拟 IP 已不存在，本地网络状态已刷新为停用。',
          );
        }
      }
      if (!sessionStore.busy && _hasActiveTunnelRuntime()) {
        await _syncActiveNetworkBeforeHeartbeat();
      } else if (!sessionStore.busy) {
        await _refreshNetworksForRemoteControl();
      }
      final networkOnline = _hasActiveTunnelRuntime();
      await _reportDeviceNetworkState(
        networkOnline: networkOnline,
        tunnelUp: networkOnline,
        lastProbeOk: networkOnline
            ? (tunnelStore.lastProbe?.replyObserved ?? true)
            : false,
      );
    } catch (error) {
      debugPrint('[network-heartbeat] skipped: $error');
    } finally {
      _networkStateHeartbeatInFlight = false;
    }
  }

  Future<void> _syncActiveNetworkBeforeHeartbeat() async {
    if (sessionStore.node == null ||
        sessionStore.bootstrap == null ||
        sessionStore.selectedNetworkId == null) {
      return;
    }
    final previousLocalVirtualIp =
        tunnelStore.tunnelRuntimeView?.localVirtualIp;
    late final String detail;
    try {
      detail = await _refreshBootstrapOrControlSyncState();
    } catch (error) {
      if (_isRemoteAttachmentDisabledError(error)) {
        await _forceDisableLocalNetwork(
          reason: '网络管理员已停用当前设备绑定，本地网络已自动禁用。',
        );
        return;
      }
      rethrow;
    }
    await _configureLocalDnsForActiveTunnel();
    await _reapplyActiveNetworkTunnelIfVirtualIpChanged(
      previousLocalVirtualIp,
    );
    debugPrint('[network-heartbeat] control sync before state report: $detail');
  }

  Future<bool> _refreshActiveTunnelRuntimeFromNative() async {
    final runtime = tunnelStore.tunnelRuntimeView;
    if (runtime == null) {
      return false;
    }
    final peerVirtualIp = runtime.peerVirtualIp.trim();
    if (peerVirtualIp.isEmpty) {
      return false;
    }
    final result = await _tunnelRuntimeService.refreshTunnelRuntime(
      peerVirtualIp: peerVirtualIp,
    );
    tunnelStore.tunnelRuntimeView = result.runtimeView;
    tunnelStore.lastTunnelActionReport = result.report;
    final refreshed = result.runtimeView;
    if (refreshed == null || !result.report.succeeded) {
      emitStateChanged();
      return false;
    }
    final active = _tunnelRuntimeViewIsActive(refreshed);
    emitStateChanged();
    return active;
  }

  Future<void> _markLocalTunnelRuntimeOffline({required String reason}) async {
    try {
      await LocalDnsService.instance.stop();
    } catch (error) {
      debugPrint('[network-heartbeat] stop dns after stale runtime skipped: $error');
    }
    try {
      await _reportDeviceNetworkState(
        networkOnline: false,
        tunnelUp: false,
        lastProbeOk: false,
      );
    } catch (error) {
      debugPrint('[network-heartbeat] stale runtime offline report skipped: $error');
    }
    sessionStore.clearConnection();
    tunnelStore.clearConnection();
    tunnelStore.lastTunnelActionReport = TunnelActionReport(
      succeeded: true,
      detail: reason,
      source: TunnelActionReportSource.runtimeSnapshot,
      phase: TunnelActionPhase.verified,
    );
    sessionStore.notice = reason;
    emitStateChanged();
  }

  Future<void> _refreshNetworksForRemoteControl() async {
    final session = sessionStore.session;
    final device = sessionStore.device;
    if (session == null || device == null || sessionStore.busy) {
      return;
    }
    final previousNetworkId = sessionStore.selectedNetworkId;
    final usageState = await AppCoreScope.readNetworkUsageState(session.userId);
    final networks = await AppCoreScope.instance.listNetworks();
    sessionStore.networks = networks;
    sessionStore.syncSelectedNetworkId(
      preferredNetworkId: previousNetworkId ?? usageState?.networkId,
    );
    if (sessionStore.selectedNetwork == null || _hasActiveTunnelRuntime()) {
      return;
    }
    final memberStatus = _currentDeviceMemberStatus(device.deviceId);
    if (memberStatus == 'disabled' || memberStatus == 'suspended') {
      return;
    }
    if (memberStatus != 'active' && usageState?.enabled != true) {
      return;
    }
    await enableActiveNetwork();
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

  bool _hasActiveTunnelRuntime() {
    if (sessionStore.selectedNetworkId == null ||
        tunnelStore.tunnelRuntimeView == null) {
      return false;
    }
    return _tunnelRuntimeViewIsActive(tunnelStore.tunnelRuntimeView!);
  }

  bool _tunnelRuntimeViewIsActive(WireGuardTunnelRuntimeView runtime) {
    final state = runtime.state.toLowerCase().trim();
    final backendState = (runtime.backendState ?? '').toLowerCase().trim();
    final stateActive =
        state == 'up' || state == 'running' || state == 'configured';
    final backendActive = runtime.backendIsUp == true ||
        backendState == 'started' ||
        backendState == 'running' ||
        backendState == 'up';
    return stateActive && backendActive;
  }

  void _setServiceTunnelRuntimeSnapshot() {
    final network = sessionStore.selectedNetwork;
    final device = sessionStore.device;
    if (network == null || device == null) {
      return;
    }
    final config = _tunnelConfigurationService.buildActiveNetworkConfiguration(
      network: network,
      deviceId: device.deviceId,
      devicePublicKey: device.publicKey,
    );
    final nowMs = DateTime.now().millisecondsSinceEpoch;
    tunnelStore.tunnelRuntimeView = WireGuardTunnelRuntimeView(
      state: 'running',
      transport: config.transport,
      backendName: 'app-core-service',
      backendState: 'running',
      backendIsUp: true,
      backendLastStartedAtMs: nowMs,
      peerVirtualIp: config.peerVirtualIp,
      peerPublicKey: config.peer.publicKey,
      selectedEndpoint: config.peer.endpoint,
      interfaceName: config.interface.interfaceName,
      dnsServers: config.interface.dnsServers,
      allowedIps: config.peer.allowedIps,
      localVirtualIp: config.localVirtualIp,
      remoteAddress: config.peer.endpoint ?? '',
      mtu: config.interface.mtu,
      interfaceAddresses: config.interface.addresses,
      lastAppliedAtMs: nowMs,
    );
    tunnelStore.lastTunnelActionReport = TunnelActionReport(
      succeeded: true,
      detail: 'app-core-service 已启用当前网络。',
      source: TunnelActionReportSource.runtimeSnapshot,
      phase: TunnelActionPhase.verified,
      runtimeSnapshot: tunnelStore.tunnelRuntimeView,
    );
    emitStateChanged();
  }

  String? _currentDeviceMemberStatus(String deviceId) {
    final network = sessionStore.selectedNetwork;
    if (network == null) {
      return null;
    }
    for (final member in network.members) {
      if (member.deviceId == deviceId) {
        return member.status?.trim().toLowerCase();
      }
    }
    return null;
  }

  bool _isRemoteAttachmentDisabledError(Object error) {
    final message = error.toString().toLowerCase();
    return message.contains('forbidden') &&
        (message.contains('no active network attachment') ||
            message.contains('has no active network attachment') ||
            message.contains('device unavailable'));
  }

  Future<void> _persistNetworkUsageState({required bool enabled}) async {
    final userId = sessionStore.session?.userId.trim();
    if (userId == null || userId.isEmpty) {
      return;
    }
    await AppCoreScope.persistNetworkUsageState(
      userId: userId,
      enabled: enabled,
      networkId: sessionStore.selectedNetworkId,
    );
  }

  Future<void> _autoEnableLastNetworkIfRequested() async {
    final session = sessionStore.session;
    if (session == null) {
      return;
    }
    final usageState = await AppCoreScope.readNetworkUsageState(session.userId);
    if (usageState?.enabled != true) {
      return;
    }
    final preferredNetworkId = usageState?.networkId?.trim();
    if (preferredNetworkId != null && preferredNetworkId.isNotEmpty) {
      sessionStore.syncSelectedNetworkId(
        preferredNetworkId: preferredNetworkId,
      );
    } else {
      sessionStore.syncSelectedNetworkId();
    }
    if (sessionStore.selectedNetworkId == null ||
        sessionStore.networks.isEmpty) {
      debugPrint('[startup] skip auto enable: no selected network');
      return;
    }
    if (_hasActiveTunnelRuntime()) {
      await _reportDeviceNetworkState(
        networkOnline: true,
        tunnelUp: true,
        lastProbeOk: true,
      );
      return;
    }
    debugPrint(
      '[startup] last network state enabled; auto enabling ${sessionStore.selectedNetworkId}',
    );
    sessionStore.notice = 'Restoring last enabled network';
    emitStateChanged();
    await _enableActiveNetworkCore();
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
      sessionStore.syncDevice(result.device);
      _startNetworkStateHeartbeat();
      await _connectMqttAndMarkOnline(result.device);
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
        _startNetworkStateHeartbeat();
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
    String? joinKey,
  }) async {
    await runAction(() async {
      final result = await _workspaceService.joinNetwork(
        session: sessionStore.session,
        currentDevice: sessionStore.device,
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
      await _enableActiveNetworkCore();
      sessionStore.notice = 'Network created and enabled: ${network.networkId}';
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
    await runAction(_enableActiveNetworkCore);
  }

  Future<void> _enableActiveNetworkCore() async {
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
    final requestedNetworkId = sessionStore.selectedNetworkId;
    if (_serviceOwnsLocalNetwork) {
      final serviceBootstrap = await AppCoreScope.instance.enableLocalNetwork(
        networkId: requestedNetworkId,
      );
      if (serviceBootstrap == null) {
        throw StateError('app-core-service failed to enable local network');
      }
      sessionStore.bootstrap = serviceBootstrap;
      sessionStore.controlStatus = await AppCoreScope.instance.controlStatus();
      sessionStore.syncDevice(serviceBootstrap.device);
      sessionStore.networks = serviceBootstrap.networks;
      sessionStore.syncSelectedNetworkId(
        preferredNetworkId: requestedNetworkId,
      );
      _setServiceTunnelRuntimeSnapshot();
      await _persistNetworkUsageState(enabled: true);
      unawaited(_sendNetworkStateHeartbeat());
      sessionStore.notice = 'Network enabled';
      debugPrint(
        '[tunnel-enable] service handled network=${sessionStore.selectedNetworkId}',
      );
      return;
    }
    final runtime = await _workspaceService.prepareActiveNetworkRuntime(
      session: sessionStore.session,
      currentDevice: sessionStore.device,
      currentNode: sessionStore.node,
      currentNetworks: sessionStore.networks,
      targetNetworkId: sessionStore.selectedNetworkId,
    );
    debugPrint(
      '[tunnel-enable] runtime ready activeNetwork=${runtime.activeNetwork.networkId} node=${runtime.node.nodeId} device=${runtime.device.deviceId}',
    );
    sessionStore.node = runtime.node;
    sessionStore.bootstrap = runtime.bootstrap;
    sessionStore.controlStatus = runtime.controlStatus;
    sessionStore.syncDevice(runtime.device);
    sessionStore.networks = runtime.networks;
    sessionStore.syncSelectedNetworkId(
      preferredNetworkId: runtime.activeNetwork.networkId,
    );
    await LocalDnsService.instance.configureFromNetwork(runtime.activeNetwork);

    final config = _tunnelConfigurationService.buildActiveNetworkConfiguration(
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
    final bringUpReport = await bringTunnelUp(
      verifyPeerVirtualIp: peerVirtualIp,
    );
    debugPrint(
      '[tunnel-enable] bring-up report succeeded=${bringUpReport.succeeded} phase=${bringUpReport.phase} error=${bringUpReport.errorMessage}',
    );
    if (!bringUpReport.succeeded) {
      await _reportDeviceNetworkState(
        networkOnline: false,
        tunnelUp: false,
        lastProbeOk: false,
      );
      await _persistNetworkUsageState(enabled: false);
      return;
    }
    await LocalDnsService.instance.configureFromNetwork(runtime.activeNetwork);
    await _reportDeviceNetworkState(
      networkOnline: true,
      tunnelUp: true,
      lastProbeOk: true,
    );
    await _persistNetworkUsageState(enabled: true);
    unawaited(_sendNetworkStateHeartbeat());
    debugPrint('[tunnel-enable] bring-up done peerVirtualIp=$peerVirtualIp');
  }

  Future<void> disableActiveNetwork() async {
    await runAction(() async {
      if (sessionStore.networks.isEmpty) {
        sessionStore.notice = '当前没有已接入的活动网络。';
        return;
      }
      late final DeactivatedNetworkResult result;
      if (_serviceOwnsLocalNetwork) {
        final disabledByService =
            await AppCoreScope.instance.disableLocalNetwork(
          networkId: sessionStore.selectedNetworkId,
        );
        if (!disabledByService) {
          throw StateError('app-core-service failed to disable local network');
        }
        await _persistNetworkUsageState(enabled: false);
        result = DeactivatedNetworkResult(
          device: sessionStore.device,
          networks: sessionStore.session == null
              ? sessionStore.networks
              : await AppCoreScope.instance.listNetworks(),
        );
      } else {
        await bringTunnelDown();
        await LocalDnsService.instance.stop();
        await _reportDeviceNetworkState(networkOnline: false, tunnelUp: false);
        await _persistNetworkUsageState(enabled: false);
        result = DeactivatedNetworkResult(
          device: sessionStore.device,
          networks: sessionStore.session == null
              ? sessionStore.networks
              : await AppCoreScope.instance.listNetworks(),
        );
      }
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
    if (_serviceOwnsLocalNetwork) {
      await runAction(() async {
        detail = await _refreshBootstrapOrControlSyncState();
      });
      return detail;
    }
    final previousLocalVirtualIp =
        tunnelStore.tunnelRuntimeView?.localVirtualIp;
    await runAction(() async {
      detail = await _refreshBootstrapOrControlSyncState();
      await _configureLocalDnsForActiveTunnel();
    });
    await _reapplyActiveNetworkTunnelIfVirtualIpChanged(previousLocalVirtualIp);
    return detail;
  }

  Future<String> _refreshBootstrapOrControlSyncState() async {
    if (_serviceOwnsLocalNetwork) {
      final targetNodeId =
          sessionStore.node?.nodeId ?? sessionStore.controlStatus?.nodeId;
      final targetNetworkId = sessionStore.selectedNetworkId ??
          sessionStore.controlStatus?.networkId;
      if (targetNodeId == null ||
          targetNodeId.trim().isEmpty ||
          targetNetworkId == null ||
          targetNetworkId.trim().isEmpty) {
        throw StateError('nodeId and networkId are required');
      }
      final bootstrap = await AppCoreScope.instance.controlSync(
        nodeId: targetNodeId,
        networkId: targetNetworkId,
      );
      final controlStatus = await AppCoreScope.instance.controlStatus();
      if (!controlStatus.networkMapPresent) {
        await _forceDisableLocalNetwork(
          reason: '当前设备已被服务端移出网络，本地网络已自动禁用。',
        );
        return '当前设备已被服务端移出网络，本地网络已自动禁用。';
      }
      sessionStore.bootstrap = bootstrap;
      sessionStore.controlStatus = controlStatus;
      sessionStore.syncDevice(bootstrap.device);
      sessionStore.networks = bootstrap.networks;
      sessionStore.syncSelectedNetworkId(
        preferredNetworkId: controlStatus.networkId ?? targetNetworkId,
      );
      return 'Control session synchronized by app-core-service.';
    }
    final result = await _deviceRuntimeService.refreshBootstrap(
      node: sessionStore.node,
      currentBootstrap: sessionStore.bootstrap,
      currentNetworks: sessionStore.networks,
    );
    if (result.usedControlSync &&
        result.controlStatus.status != 'none' &&
        !result.controlStatus.networkMapPresent) {
      await _forceDisableLocalNetwork(
        reason: '当前设备已被服务端移出网络，本地网络已自动禁用。',
      );
      return '当前设备已被服务端移出网络，本地网络已自动禁用。';
    }
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

  Future<void> _forceDisableLocalNetwork({required String reason}) async {
    if (_serviceOwnsLocalNetwork) {
      try {
        await AppCoreScope.instance.disableLocalNetwork(
          networkId: sessionStore.selectedNetworkId,
        );
      } catch (error) {
        debugPrint('[control-sync] service force disable skipped: $error');
      }
      sessionStore.bootstrap = null;
      sessionStore.controlStatus = null;
      sessionStore.networks = const [];
      sessionStore.selectedNetworkId = null;
      sessionStore.clearConnection();
      tunnelStore.clearConnection();
      tunnelStore.lastTunnelActionReport = TunnelActionReport(
        succeeded: true,
        detail: reason,
        source: TunnelActionReportSource.runtimeSnapshot,
        phase: TunnelActionPhase.verified,
      );
      sessionStore.notice = reason;
      return;
    }
    final peerVirtualIp = tunnelStore.tunnelRuntimeView?.peerVirtualIp.trim();
    try {
      await bringTunnelDown();
    } catch (error) {
      debugPrint('[control-sync] force bring down skipped: $error');
    }
    if (peerVirtualIp != null && peerVirtualIp.isNotEmpty) {
      try {
        await removeTunnelPeer(peerVirtualIp: peerVirtualIp);
      } catch (error) {
        debugPrint('[control-sync] force remove peer skipped: $error');
      }
    }
    try {
      await LocalDnsService.instance.stop();
    } catch (error) {
      debugPrint('[control-sync] force stop dns skipped: $error');
    }
    try {
      await AppCoreScope.instance.disconnect();
    } catch (error) {
      debugPrint('[control-sync] force app-core disconnect skipped: $error');
    }
    try {
      await _reportDeviceNetworkState(networkOnline: false, tunnelUp: false);
    } catch (error) {
      debugPrint('[control-sync] force offline report skipped: $error');
    }
    sessionStore.bootstrap = null;
    sessionStore.controlStatus = null;
    sessionStore.networks = const [];
    sessionStore.selectedNetworkId = null;
    sessionStore.clearConnection();
    tunnelStore.clearConnection();
    tunnelStore.lastTunnelActionReport = TunnelActionReport(
      succeeded: true,
      detail: reason,
      source: TunnelActionReportSource.runtimeSnapshot,
      phase: TunnelActionPhase.verified,
    );
    sessionStore.notice = reason;
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
    final dnsUnchanged = listEquals(
      runtime.dnsServers,
      config.interface.dnsServers,
    );
    if (previousLocalVirtualIp == config.localVirtualIp &&
        runtime.localVirtualIp == config.localVirtualIp &&
        dnsUnchanged) {
      return;
    }
    final peerVirtualIp = config.peer.allowedIps.first.split('/').first;
    debugPrint(
      '[tunnel-refresh] runtime config changed '
      'localVirtualIp=${previousLocalVirtualIp ?? runtime.localVirtualIp}'
      '->${config.localVirtualIp} '
      'dnsServers=${config.interface.dnsServers}; reapplying tunnel',
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
  }

  Future<void> _configureLocalDnsForActiveTunnel() async {
    if (tunnelStore.tunnelRuntimeView == null) {
      return;
    }
    final network = sessionStore.selectedNetwork;
    if (network == null) {
      await LocalDnsService.instance.stop();
      return;
    }
    await LocalDnsService.instance.configureFromNetwork(network);
  }

  Future<ConnectAttemptResult> connectUsingControlPlan({
    required String networkId,
    required String peerNodeId,
    required String reason,
  }) async {
    late ConnectAttemptResult result;
    await runAction(() async {
      if (_serviceOwnsLocalNetwork) {
        final connectionState = await AppCoreScope.instance.connect(
          networkId: networkId,
          peerNodeId: peerNodeId,
        );
        result = ConnectAttemptResult(
          connectionState: connectionState,
          relayTicket: null,
          preflightHint: 'Connection planning is handled by app-core-service.',
          resultHint:
              'Connection state is ${connectionState.status}; app-core-service applied the control-plane path and fallback policy.',
        );
      } else {
        result = await _deviceRuntimeService.connectWithFallback(
          networkId: networkId,
          peerNodeId: peerNodeId,
          reason: reason,
          currentNode: sessionStore.node,
          controlStatus: sessionStore.controlStatus,
          lastProbe: lastProbe,
        );
      }
      sessionStore.relayTicket = result.relayTicket;
      sessionStore.connectionState = result.connectionState;
    });
    return result;
  }

  Future<void> disconnect() async {
    await runAction(() async {
      await AppCoreScope.instance.disconnect();
      if (!_serviceOwnsLocalNetwork) {
        try {
          await LocalDnsService.instance.stop();
        } catch (error) {
          debugPrint('[disconnect] stop dns skipped: $error');
        }
      }
      sessionStore.clearConnection();
      tunnelStore.clearConnection();
    });
  }

  Future<TunnelActionReport> applyTunnelConfiguration({
    required WireGuardTunnelConfiguration configuration,
    String? verifyPeerVirtualIp,
  }) async {
    if (_serviceOwnsLocalNetwork) {
      return _serviceOwnedTunnelActionReport('applyTunnelConfiguration');
    }
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
    if (_serviceOwnsLocalNetwork) {
      return _serviceOwnedTunnelActionReport('bringTunnelUp');
    }
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
    if (_serviceOwnsLocalNetwork) {
      return _serviceOwnedTunnelActionReport('bringTunnelDown');
    }
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
    if (_serviceOwnsLocalNetwork) {
      return _serviceOwnedTunnelActionReport('removeTunnelPeer');
    }
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

  TunnelActionReport _serviceOwnedTunnelActionReport(String action) {
    final report = TunnelActionReport(
      succeeded: true,
      detail:
          '$action skipped because app-core-service owns local network runtime.',
      source: TunnelActionReportSource.runtimeSnapshot,
      phase: TunnelActionPhase.verified,
      runtimeSnapshot: tunnelStore.tunnelRuntimeView,
    );
    tunnelStore.lastTunnelActionReport = report;
    emitStateChanged();
    return report;
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
