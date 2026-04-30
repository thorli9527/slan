import 'dart:async';
import 'dart:io';

import 'package:flutter/foundation.dart' show debugPrint, listEquals;
import 'package:slan_app_core_plugin/slan_app_core_plugin.dart';

import '../../../application/app_workspace_service.dart';
import '../../../application/auth_session_service.dart';
import '../../../application/device_setup_service.dart';
import '../../../application/device_runtime_service.dart';
import '../../../application/device_network_runtime_service.dart';
import '../../../application/local_dns_service.dart';
import '../../../application/mqtt_control_task_service.dart';
import '../../../application/network_enable_preflight_service.dart';
import '../../../application/service_runtime_sync_policy.dart';
import '../../../application/tunnel_configuration_service.dart';
import '../../../application/tunnel_host_gateway.dart';
import '../../../application/tunnel_runtime_service.dart';
import '../api/app_core_api.dart';
import '../api/http_app_core_api.dart';
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
    AppCoreApi Function()? apiProvider,
    TunnelHostGateway hostGateway = const PluginTunnelHostGateway(),
    DeviceNetworkRuntimeService deviceNetworkRuntimeService =
        const DefaultDeviceNetworkRuntimeService(),
    MqttControlTaskService mqttControlTaskService =
        const XmlMqttControlTaskService(),
    NetworkEnablePreflightService networkEnablePreflightService =
        const DefaultNetworkEnablePreflightService(),
    ServiceRuntimeSyncPolicy serviceRuntimeSyncPolicy =
        const DefaultServiceRuntimeSyncPolicy(),
    LocalDnsServiceContract? localDnsService,
    AppWorkspaceServiceContract? workspaceService,
    AuthSessionServiceContract? authSessionService,
    DeviceSetupServiceContract? deviceSetupService,
    DeviceRuntimeServiceContract? deviceRuntimeService,
    TunnelConfigurationServiceContract? tunnelConfigurationService,
    TunnelRuntimeServiceContract? tunnelRuntimeService,
  })  : _deviceNetworkRuntimeService = deviceNetworkRuntimeService,
        _mqttControlTaskService = mqttControlTaskService,
        _networkEnablePreflightService = networkEnablePreflightService,
        _serviceRuntimeSyncPolicy = serviceRuntimeSyncPolicy,
        _localDnsService = localDnsService ?? LocalDnsService.instance,
        _workspaceService = workspaceService ??
            AppWorkspaceService(
              apiProvider: apiProvider ?? (() => AppCoreScope.instance),
            ),
        _authSessionService = authSessionService ??
            AuthSessionService(
              apiProvider: apiProvider ?? (() => AppCoreScope.instance),
            ),
        _deviceSetupService = deviceSetupService ??
            DeviceSetupService(
              apiProvider: apiProvider ?? (() => AppCoreScope.instance),
            ),
        _deviceRuntimeService = deviceRuntimeService ??
            DeviceRuntimeService(
              apiProvider: apiProvider ?? (() => AppCoreScope.instance),
            ),
        _apiProvider = apiProvider ?? (() => AppCoreScope.instance),
        _tunnelConfigurationService =
            tunnelConfigurationService ?? const TunnelConfigurationService(),
        _tunnelRuntimeService = tunnelRuntimeService ??
            TunnelRuntimeService(hostGateway: hostGateway);

  final AppSessionStore sessionStore = AppSessionStore();
  final AppTunnelStore tunnelStore = AppTunnelStore();
  final AppCoreApi Function() _apiProvider;
  final AppWorkspaceServiceContract _workspaceService;
  final AuthSessionServiceContract _authSessionService;
  final DeviceSetupServiceContract _deviceSetupService;
  final DeviceRuntimeServiceContract _deviceRuntimeService;
  final TunnelRuntimeServiceContract _tunnelRuntimeService;
  final DeviceNetworkRuntimeService _deviceNetworkRuntimeService;
  final MqttControlTaskService _mqttControlTaskService;
  final NetworkEnablePreflightService _networkEnablePreflightService;
  final ServiceRuntimeSyncPolicy _serviceRuntimeSyncPolicy;
  final LocalDnsServiceContract _localDnsService;
  final TunnelConfigurationServiceContract _tunnelConfigurationService;
  Timer? _networkStateHeartbeatTimer;
  bool _networkStateHeartbeatInFlight = false;
  AppCoreApi get _api => _apiProvider();
  bool get _serviceOwnsLocalNetwork =>
      AppCoreScope.mode == 'bridge' && Platform.isWindows;
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
          await _api.disableLocalNetwork(
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
          // The local mesh runtime is already gone or not initialized.
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
          await _localDnsService.stop();
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
        await _api.disconnect();
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
        await _syncServiceOwnedRuntimeFromHelper();
        try {
          await _api.reportDeviceNetworkState();
        } catch (error) {
          debugPrint(
              '[network-heartbeat] service state report skipped: $error');
        }
        if (!sessionStore.busy && !_hasActiveTunnelRuntime()) {
          await _refreshNetworksForRemoteControl();
        }
        return;
      }
      if (!sessionStore.busy && _hasActiveTunnelRuntime()) {
        final runtimeStillActive =
            await _refreshActiveTunnelRuntimeFromNative();
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

  Future<void> _syncServiceOwnedRuntimeFromHelper() async {
    final hadRuntime = _hasActiveTunnelRuntime();
    final hadDeviceIp =
        sessionStore.device?.virtualIp?.trim().isNotEmpty == true;
    final status = await _readHelperStatus();
    await _applyServiceOwnedRuntimeFromHelperStatus(
      status,
      hadRuntime: hadRuntime,
      hadDeviceIp: hadDeviceIp,
    );
  }

  Future<void> _applyServiceOwnedRuntimeFromHelperStatus(
    AppCoreHelperStatusModel status, {
    required bool hadRuntime,
    required bool hadDeviceIp,
  }) async {
    final decision = _serviceRuntimeSyncPolicy.decide(
      ServiceRuntimeSyncInput(
        status: status,
        hadRuntime: hadRuntime,
        hadDeviceIp: hadDeviceIp,
        hasAssignedDeviceIp: _currentDeviceHasAssignedVirtualIp(),
        hasControlStatus: sessionStore.controlStatus != null,
        selectedNetworkId: sessionStore.selectedNetworkId,
      ),
    );
    switch (decision.action) {
      case ServiceRuntimeSyncAction.none:
      case ServiceRuntimeSyncAction.waitForPendingDisable:
        return;
      case ServiceRuntimeSyncAction.refreshInventoryAndActivate:
        await _refreshServiceNetworkInventory(
          preferredNetworkId: decision.helperNetworkId,
        );
        _syncServiceDeviceVirtualIpFromHelperStatus(status);
        if (!_hasActiveTunnelRuntime() || decision.consumeUiRefresh) {
          _setServiceTunnelRuntimeSnapshot();
        }
        if (decision.notice != null) {
          sessionStore.notice = decision.notice;
        }
        if (decision.consumeUiRefresh) {
          _mqttControlTaskService.markUiRefreshConsumed(
            status.mqttControlUiRefreshTaskId,
          );
        }
        emitStateChanged();
        return;
      case ServiceRuntimeSyncAction.selectNetworkAndActivate:
        sessionStore.syncSelectedNetworkId(
          preferredNetworkId: decision.helperNetworkId,
        );
        _syncServiceDeviceVirtualIpFromHelperStatus(status);
        if (!_hasActiveTunnelRuntime() || decision.consumeUiRefresh) {
          _setServiceTunnelRuntimeSnapshot();
        }
        if (decision.notice != null) {
          sessionStore.notice = decision.notice;
        }
        if (decision.consumeUiRefresh) {
          _mqttControlTaskService.markUiRefreshConsumed(
            status.mqttControlUiRefreshTaskId,
          );
        }
        emitStateChanged();
        return;
      case ServiceRuntimeSyncAction.clearInactiveRuntime:
        break;
    }
    if (sessionStore.device != null && hadDeviceIp) {
      sessionStore.syncDevice(
        _deviceNetworkRuntimeService.copyDeviceWithVirtualIp(
          sessionStore.device!,
          null,
        ),
      );
    }
    sessionStore.controlStatus = null;
    sessionStore.clearConnection();
    tunnelStore.clearConnection();
    tunnelStore.lastTunnelActionReport = const TunnelActionReport(
      succeeded: true,
      detail: 'app-core-service 当前没有启用的本地网络，界面状态已同步为停用。',
      source: TunnelActionReportSource.runtimeSnapshot,
      phase: TunnelActionPhase.verified,
    );
    sessionStore.notice = decision.notice;
    if (decision.consumeUiRefresh) {
      _mqttControlTaskService.markUiRefreshConsumed(
        status.mqttControlUiRefreshTaskId,
      );
    }
    emitStateChanged();
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
      await _localDnsService.stop();
    } catch (error) {
      debugPrint(
          '[network-heartbeat] stop dns after stale runtime skipped: $error');
    }
    try {
      await _reportDeviceNetworkState(
        networkOnline: false,
        tunnelUp: false,
        lastProbeOk: false,
      );
    } catch (error) {
      debugPrint(
          '[network-heartbeat] stale runtime offline report skipped: $error');
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
    final networks = await _api.listNetworks();
    sessionStore.networks = networks;
    sessionStore.syncSelectedNetworkId(
      preferredNetworkId: previousNetworkId ?? usageState?.networkId,
    );
    _syncCurrentDeviceVirtualIpFromSelectedNetwork();
    if (sessionStore.selectedNetwork == null || _hasActiveTunnelRuntime()) {
      return;
    }
    if (await _deferNetworkInitializationUntilAssignedIp(
      notice: '登录成功，正在等待服务器分配 IP。',
      debugMessage:
          '[remote-control] skip enable: current device has no virtual IP',
    )) {
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

  Future<void> _refreshServiceNetworkInventory({
    String? preferredNetworkId,
  }) async {
    final session = sessionStore.session;
    if (session == null) {
      return;
    }
    try {
      final previousNetworkId = sessionStore.selectedNetworkId;
      sessionStore.networks = await _api.listNetworks();
      sessionStore.syncSelectedNetworkId(
        preferredNetworkId: preferredNetworkId ?? previousNetworkId,
      );
      _syncCurrentDeviceVirtualIpFromSelectedNetwork();
      emitStateChanged();
    } catch (error) {
      debugPrint('[service-runtime-sync] refresh networks skipped: $error');
    }
  }

  void _syncServiceDeviceVirtualIpFromHelperStatus(
    AppCoreHelperStatusModel status,
  ) {
    final helperDeviceId = status.deviceId?.trim();
    final helperVirtualIp = status.currentDeviceVirtualIp?.trim();
    final currentDevice = sessionStore.device;
    if (currentDevice == null ||
        helperDeviceId == null ||
        helperDeviceId.isEmpty ||
        helperVirtualIp == null ||
        helperVirtualIp.isEmpty ||
        currentDevice.deviceId != helperDeviceId ||
        currentDevice.virtualIp?.trim() == helperVirtualIp) {
      return;
    }
    sessionStore.syncDevice(
      _deviceNetworkRuntimeService.copyDeviceWithVirtualIp(
        currentDevice,
        helperVirtualIp,
      ),
    );
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
    await _api.setDeviceNetworkState(
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

  bool _currentDeviceHasAssignedVirtualIp() {
    final device = sessionStore.device;
    if (device == null) {
      return false;
    }
    return _assignedVirtualIpForDevice(device.deviceId) != null;
  }

  String? _assignedVirtualIpForDevice(String deviceId) {
    return _deviceNetworkRuntimeService.assignedVirtualIpForDevice(
      device: sessionStore.device,
      network: sessionStore.selectedNetwork,
      deviceId: deviceId,
    );
  }

  void _syncCurrentDeviceVirtualIpFromSelectedNetwork() {
    final syncedDevice =
        _deviceNetworkRuntimeService.syncDeviceVirtualIpFromNetwork(
      device: sessionStore.device,
      network: sessionStore.selectedNetwork,
    );
    if (syncedDevice != null) {
      sessionStore.syncDevice(syncedDevice);
    }
  }

  Future<void> _refreshCurrentDeviceNetworkConfigFromServer({
    String? preferredNetworkId,
  }) async {
    final session = sessionStore.session;
    final controlBaseUrl = AppCoreScope.controlBaseUrl;
    if (session == null ||
        controlBaseUrl == null ||
        controlBaseUrl.trim().isEmpty) {
      throw StateError('网络错误：无法从服务器拉取最新设备 IP 和 DNS 信息');
    }
    final currentDeviceId = sessionStore.device?.deviceId ?? session.deviceId;
    if (currentDeviceId == null || currentDeviceId.trim().isEmpty) {
      return;
    }
    final httpApi = HttpAppCoreApi(baseUrl: controlBaseUrl);
    final result = await _networkEnablePreflightService.verifyRemoteNetwork(
      remoteApi: httpApi,
      session: session,
      currentDevice: sessionStore.device,
      preferredNetworkId: preferredNetworkId,
    );
    sessionStore.devices = result.devices;
    sessionStore.networks = result.networks;
    sessionStore.syncSelectedNetworkId(
      preferredNetworkId: result.selectedNetworkId ?? preferredNetworkId,
    );
    sessionStore.syncDevice(result.device);
    final network = sessionStore.selectedNetwork;
    if (network != null) {
      debugPrint(
        '[device-config-refresh] network=${network.networkId} '
        'dnsWildcards=${network.dns.wildcards.length} '
        'dnsServers=${network.dns.servers.length} '
        'searchDomains=${network.dns.searchDomains.length}',
      );
    }
  }

  Future<bool> _deferNetworkInitializationUntilAssignedIp({
    required String notice,
    required String debugMessage,
  }) async {
    _syncCurrentDeviceVirtualIpFromSelectedNetwork();
    if (_currentDeviceHasAssignedVirtualIp()) {
      return false;
    }
    sessionStore.notice = notice;
    await _reportDeviceNetworkState(
      networkOnline: false,
      tunnelUp: false,
      lastProbeOk: false,
    );
    emitStateChanged();
    debugPrint(debugMessage);
    return true;
  }

  void _setServiceTunnelRuntimeSnapshot() {
    final network = sessionStore.selectedNetwork;
    final device = sessionStore.device;
    if (network == null || device == null) {
      return;
    }
    _syncCurrentDeviceVirtualIpFromSelectedNetwork();
    if (!_currentDeviceHasAssignedVirtualIp()) {
      sessionStore.notice = '网络已请求启用，正在等待服务器返回 IP。';
      emitStateChanged();
      return;
    }
    final config = _tunnelConfigurationService.buildActiveNetworkConfiguration(
      network: network,
      deviceId: device.deviceId,
      devicePublicKey: device.publicKey,
      deviceVirtualIp: device.virtualIp,
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

  Future<void> _enqueueServiceNetworkTask({
    required String taskType,
    required String? networkId,
    required String reason,
  }) async {
    final deviceId = sessionStore.device?.deviceId.trim();
    final targetNetworkId = networkId?.trim();
    if (deviceId == null || deviceId.isEmpty) {
      throw StateError('当前设备尚未注册完成。');
    }
    if (targetNetworkId == null || targetNetworkId.isEmpty) {
      throw StateError('当前没有可操作的网络。');
    }
    await _mqttControlTaskService.enqueueNetworkTask(
      taskType: taskType,
      networkId: targetNetworkId,
      deviceId: deviceId,
      uiRefreshReason: reason,
    );
  }

  void _clearServiceOwnedNetworkUiState({required String notice}) {
    final currentDevice = sessionStore.device;
    if (currentDevice != null) {
      sessionStore.syncDevice(
        _deviceNetworkRuntimeService.copyDeviceWithVirtualIp(
          currentDevice,
          null,
        ),
      );
    }
    sessionStore.controlStatus = null;
    sessionStore.clearConnection();
    tunnelStore.clearConnection();
    tunnelStore.lastTunnelActionReport = TunnelActionReport(
      succeeded: true,
      detail: notice,
      source: TunnelActionReportSource.runtimeSnapshot,
      phase: TunnelActionPhase.verified,
    );
    sessionStore.notice = notice;
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

  bool _currentDeviceMemberIsDisabled() {
    final deviceId = sessionStore.device?.deviceId;
    if (deviceId == null || deviceId.trim().isEmpty) {
      return false;
    }
    final status = _currentDeviceMemberStatus(deviceId)?.trim().toLowerCase();
    return status == 'disabled' ||
        status == 'suspended' ||
        status == 'rejected';
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
    if (await _deferNetworkInitializationUntilAssignedIp(
      notice: '登录成功，正在等待服务器分配 IP。',
      debugMessage:
          '[startup] skip auto enable: current device has no virtual IP',
    )) {
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
      if (sessionStore.selectedNetworkId != null) {
        _startNetworkStateHeartbeat();
      }
      await _connectMqttAndMarkOnline(result.device);
    });
  }

  Future<void> refreshDeviceInventory() async {
    await runAction(() async {
      if (sessionStore.session == null) {
        throw StateError('login required');
      }
      final devices = await _api.listDevices();
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
      final network = await _api.createNetwork(
        name: name,
        cidr: cidr,
        allocationStartIp: allocationStartIp,
        allocationEndIp: allocationEndIp,
        bindDeviceId: device.deviceId,
      );
      sessionStore.networks = await _api.listNetworks();
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
      sessionStore.networks = await _api.listNetworks();
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
      await _preflightServiceNetworkEnable(
        preferredNetworkId: requestedNetworkId,
      );
      await _enqueueServiceNetworkTask(
        taskType: 'enable_network',
        networkId: requestedNetworkId,
        reason: 'network_enabled',
      );
      sessionStore.notice = '网络启用任务已提交，正在后台应用。';
      await _persistNetworkUsageState(enabled: true);
      unawaited(_sendNetworkStateHeartbeat());
      debugPrint(
        '[tunnel-enable] queued service task network=${sessionStore.selectedNetworkId}',
      );
      return;
    }
    await _refreshCurrentDeviceNetworkConfigFromServer(
      preferredNetworkId: requestedNetworkId,
    );
    if (await _deferNetworkInitializationUntilAssignedIp(
      notice: '网络已请求启用，正在等待服务器返回 IP。',
      debugMessage:
          '[tunnel-enable] skip enable: current device has no virtual IP after HTTP refresh',
    )) {
      await _persistNetworkUsageState(enabled: false);
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
    _syncCurrentDeviceVirtualIpFromSelectedNetwork();
    if (await _deferNetworkInitializationUntilAssignedIp(
      notice: '网络已准备就绪，正在等待服务器返回 IP。',
      debugMessage:
          '[tunnel-enable] runtime ready but current device has no virtual IP',
    )) {
      await _persistNetworkUsageState(enabled: false);
      return;
    }

    final config = _tunnelConfigurationService.buildActiveNetworkConfiguration(
      network: runtime.activeNetwork,
      deviceId: sessionStore.device!.deviceId,
      devicePublicKey: sessionStore.device!.publicKey,
      deviceVirtualIp: sessionStore.device!.virtualIp,
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
    await _localDnsService.configureFromNetwork(runtime.activeNetwork);
    await _reportDeviceNetworkState(
      networkOnline: true,
      tunnelUp: true,
      lastProbeOk: true,
    );
    await _persistNetworkUsageState(enabled: true);
    unawaited(_sendNetworkStateHeartbeat());
    debugPrint('[tunnel-enable] bring-up done peerVirtualIp=$peerVirtualIp');
  }

  Future<void> _preflightServiceNetworkEnable({
    required String? preferredNetworkId,
  }) async {
    await _refreshCurrentDeviceNetworkConfigFromServer(
      preferredNetworkId: preferredNetworkId,
    );
    final device = sessionStore.device;
    if (device == null) {
      throw StateError('device unavailable: 当前设备尚未注册完成，请联系管理员。');
    }
    final assignedIp = _assignedVirtualIpForDevice(device.deviceId);
    if (assignedIp == null || assignedIp.trim().isEmpty) {
      throw StateError('device unavailable: 当前设备没有分配远程 IP，请联系管理员。');
    }
    debugPrint(
      '[tunnel-enable] service preflight ok network=$preferredNetworkId '
      'device=${device.deviceId} virtualIp=$assignedIp',
    );
  }

  Future<void> disableActiveNetwork() async {
    await runAction(() async {
      if (sessionStore.networks.isEmpty) {
        sessionStore.notice = '当前没有已接入的活动网络。';
        return;
      }
      late final DeactivatedNetworkResult result;
      if (_serviceOwnsLocalNetwork) {
        await _enqueueServiceNetworkTask(
          taskType: 'disable_network',
          networkId: sessionStore.selectedNetworkId,
          reason: 'network_disabled',
        );
        await _persistNetworkUsageState(enabled: false);
        _clearServiceOwnedNetworkUiState(
          notice: '网络停用任务已提交，本地界面已切换为停用状态。',
        );
        result = DeactivatedNetworkResult(
          device: sessionStore.device,
          networks: sessionStore.networks,
        );
      } else {
        await bringTunnelDown();
        await _localDnsService.stop();
        await _reportDeviceNetworkState(networkOnline: false, tunnelUp: false);
        await _persistNetworkUsageState(enabled: false);
        result = DeactivatedNetworkResult(
          device: sessionStore.device,
          networks: sessionStore.session == null
              ? sessionStore.networks
              : await _api.listNetworks(),
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
      sessionStore.bootstrap = await _api.bootstrap(
        nodeId: targetNodeId,
        networkId: targetNetworkId,
      );
      sessionStore.controlStatus = await _api.controlStatus();
      sessionStore.syncDevice(sessionStore.bootstrap!.device);
      sessionStore.networks = sessionStore.bootstrap!.networks;
      sessionStore.syncSelectedNetworkId(preferredNetworkId: targetNetworkId);
    });
  }

  Future<String> refreshBootstrapOrControlSync() async {
    var detail = '';
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
      final bootstrap = await _api.controlSync(
        nodeId: targetNodeId,
        networkId: targetNetworkId,
      );
      final controlStatus = await _api.controlStatus();
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
      if (_currentDeviceMemberIsDisabled()) {
        await _forceDisableLocalNetwork(
          reason: '网络管理员已停用当前设备绑定，本地网络已自动禁用。',
        );
        return '网络管理员已停用当前设备绑定，本地网络已自动禁用。';
      }
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
    if (_currentDeviceMemberIsDisabled()) {
      await _forceDisableLocalNetwork(
        reason: '网络管理员已停用当前设备绑定，本地网络已自动禁用。',
      );
      return '网络管理员已停用当前设备绑定，本地网络已自动禁用。';
    }
    return result.detail;
  }

  Future<void> _forceDisableLocalNetwork({required String reason}) async {
    final currentDevice = sessionStore.device;
    if (currentDevice != null) {
      sessionStore.syncDevice(
        _deviceNetworkRuntimeService.copyDeviceWithVirtualIp(
          currentDevice,
          null,
        ),
      );
    }
    if (_serviceOwnsLocalNetwork) {
      try {
        await _api.disableLocalNetwork(
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
      await _localDnsService.stop();
    } catch (error) {
      debugPrint('[control-sync] force stop dns skipped: $error');
    }
    try {
      await _api.disconnect();
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
      deviceVirtualIp: device.virtualIp,
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
    await _localDnsService.configureFromNetwork(network);
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
      await _localDnsService.stop();
      return;
    }
    await _localDnsService.configureFromNetwork(network);
  }

  Future<ConnectAttemptResult> connectUsingControlPlan({
    required String networkId,
    required String peerNodeId,
    required String reason,
  }) async {
    late ConnectAttemptResult result;
    await runAction(() async {
      if (_serviceOwnsLocalNetwork) {
        final connectionState = await _api.connect(
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
      await _api.disconnect();
      if (!_serviceOwnsLocalNetwork) {
        try {
          await _localDnsService.stop();
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
      return _api.platformDoctor();
    });
    tunnelStore.platformDoctor = report ?? _failedPlatformDoctor();
    emitStateChanged();
    return tunnelStore.platformDoctor!;
  }

  Future<AppCoreHelperStatusModel> refreshHelperStatus() async {
    final hadRuntime = _hasActiveTunnelRuntime();
    final hadDeviceIp =
        sessionStore.device?.virtualIp?.trim().isNotEmpty == true;
    final status = await _readHelperStatus();
    if (_serviceOwnsLocalNetwork) {
      await _applyServiceOwnedRuntimeFromHelperStatus(
        status,
        hadRuntime: hadRuntime,
        hadDeviceIp: hadDeviceIp,
      );
    }
    return status;
  }

  Future<AppCoreHelperStatusModel> _readHelperStatus() async {
    try {
      sessionStore.helperStatus = await _api.helperStatus();
    } catch (err) {
      sessionStore.helperStatus = AppCoreHelperStatusModel(
        source: AppCoreScope.mode,
        helperReachable: false,
        configuredControlBaseUrl: AppCoreScope.controlBaseUrl,
        sessionPresent: sessionStore.session != null,
        refreshTokenPresent:
            sessionStore.session?.refreshToken?.trim().isNotEmpty == true,
        mqttControlUiRefreshRequired: false,
        deviceId: sessionStore.device?.deviceId,
        nodeId: sessionStore.node?.nodeId,
        currentNetworkId: sessionStore.selectedNetworkId,
        bootstrapPresent: sessionStore.bootstrap != null,
        networkMapPresent: false,
        tunnelRuntimePresent: tunnelStore.tunnelRuntimeView != null,
        tunnelBackendRunning: false,
        tunnelLastError: err.toString(),
      );
    }
    emitStateChanged();
    return sessionStore.helperStatus!;
  }

  Future<PlatformInstallPlanModel> refreshPlatformInstallPlan() async {
    final plan = await runTunnelAction<PlatformInstallPlanModel>(() {
      return _api.platformInstallPlan();
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
      return _api.probe(
        payload: payload,
        probeTimeoutMs: probeTimeoutMs,
      );
    });
  }

  Future<void> send({
    required String payload,
  }) async {
    await runSendAction(() => _api.send(payload: payload));
  }
}
