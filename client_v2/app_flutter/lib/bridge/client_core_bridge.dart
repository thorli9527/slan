// Flutter 客户端桥接层。
//
// 桌面端通过本地独立进程 client-core-service 工作；Android/iOS 通过原生插件
// 内嵌 client-core-service，并把 VPN/PacketTunnel 数据面状态回传给服务。
// 这个文件负责把这些平台差异收敛成 UI 可使用的 ClientCoreBridge。
//
// 架构约束：
// Flutter 不直接访问 /api/app/...，业务状态只从 Rust local/embedded API 读取。

import 'dart:async';
import 'dart:convert';
import 'dart:io';

import 'package:client_core_plugin/client_core_plugin.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter/services.dart';

import 'android_network_authorization.dart';
import 'client_commands.dart';
import 'client_core_local_service.dart';
import 'client_core_bridge_support.dart';
import 'client_core_bridge_toggle.dart';
import 'control_transport_status.dart';
import 'client_ui_diagnostics.dart';
import 'client_view_state.dart';

/// ClientCoreBridge 是 UI 层使用的客户端核心门面。
///
/// host 平台通过本地 client-core-service HTTP API 工作；移动端通过原生插件内嵌
/// service 和平台 VPN/PacketTunnel 能力工作。
abstract interface class ClientCoreBridge {
  /// 首页渲染使用的状态源。
  ValueListenable<ClientViewState> get state;

  /// Android VPN 授权和网络配置状态源。
  ValueListenable<AndroidNetworkAuthorizationState>
      get androidNetworkAuthorization;

  /// 启动 bridge，加载状态并启动后台 watch loop。
  Future<void> start();

  /// 当前控制面 API 地址。
  Future<String> serverBaseUrl();

  /// 更新移动端控制面 API 地址。
  Future<void> updateServerBaseUrl(String serverBaseUrl);

  /// 检查 Android VPN 权限并准备网络配置。
  Future<void> prepareAndroidNetworkAuthorization();

  /// 派发 UI 命令。
  Future<void> dispatch(ClientCommand command);

  /// 查询本地控制通道状态。
  Future<ControlTransportStatus?> localControlStatus();

  /// 停止后台监听和异步任务。
  Future<void> close() async {}
}

/// ClientBridgeRuntimePlatform 用于测试时显式指定 bridge 走 host/android/ios 分支。
@visibleForTesting
enum ClientBridgeRuntimePlatform {
  host,
  android,
  ios,
}

/// MethodChannelClientCoreBridge 负责协调 Flutter UI、本地服务、移动端原生插件
/// 以及业务事件 watch loop。
class MethodChannelClientCoreBridge implements ClientCoreBridge {
  MethodChannelClientCoreBridge({
    String? localServiceHost,
    @visibleForTesting bool? useMobileControlPlane,
    @visibleForTesting ClientBridgeRuntimePlatform runtimePlatform =
        ClientBridgeRuntimePlatform.host,
    @visibleForTesting Future<void> Function(String url)? openExternalUrl,
  })  : _plugin = ClientCorePlugin(),
        _localService = ClientCoreLocalService(host: localServiceHost),
        _useMobileControlPlaneOverride = useMobileControlPlane,
        _runtimePlatform = runtimePlatform,
        _openExternalUrlOverride = openExternalUrl,
        _state = ValueNotifier<ClientViewState>(ClientViewState.initial()),
        _androidNetworkAuthorization =
            ValueNotifier<AndroidNetworkAuthorizationState>(
          AndroidNetworkAuthorizationState.initial,
        );

  /// 原生插件入口，负责移动端 VPN、iOS PacketTunnel、外部浏览器等能力。
  final ClientCorePlugin _plugin;

  /// 桌面端本地 Rust 服务 JSON-line 客户端。
  final ClientCoreLocalService _localService;

  /// 测试用开关：强制移动端走内嵌服务或桌面本地服务。
  final bool? _useMobileControlPlaneOverride;

  /// 测试用平台枚举，生产环境默认根据 Dart `Platform` 判断。
  final ClientBridgeRuntimePlatform _runtimePlatform;

  /// 测试时替代系统浏览器启动命令。
  final Future<void> Function(String url)? _openExternalUrlOverride;

  /// 当前 UI 状态。
  final ValueNotifier<ClientViewState> _state;

  /// Android VPN 授权状态。
  final ValueNotifier<AndroidNetworkAuthorizationState>
      _androidNetworkAuthorization;

  /// 网络切换操作 epoch，用于丢弃过期异步结果。
  int _networkToggleEpoch = 0;

  /// 已处理的最后一个业务事件 revision。
  int _lastBusinessEventRevision = 0;

  /// 是否存在正在执行的网络开关操作。
  bool _networkToggleInFlight = false;

  /// 当前网络开关操作上下文。
  NetworkToggleOperation? _networkToggleOperation;

  /// 用户是否刚刚主动退出，用于抑制旧 session 事件回写 UI。
  bool _localLogoutRequested = false;

  /// 桌面浏览器登录状态监听代次；递增可终止上一轮等待。
  int _desktopBrowserLoginWatchEpoch = 0;

  /// 业务事件 watch loop 是否已启动。
  bool _watchingBusinessEvents = false;

  /// Android VPN 事件 watch loop 是否已启动。
  bool _watchingAndroidNetworkEvents = false;

  /// iOS PacketTunnel 事件 watch loop 是否已启动。
  bool _watchingIosNetworkEvents = false;

  /// Android 数据面运行态轮询是否已启动。
  bool _watchingAndroidRuntimeStats = false;

  /// iOS PacketTunnel 运行态轮询是否已启动。
  bool _watchingIosPacketTunnelStats = false;

  /// 是否正在修复移动端 MQTT 控制通道。
  bool _repairingNativeMobileMqtt = false;

  /// 上一次下发给 Android VPN 的配置指纹，避免重复启动同一配置。
  String? _lastAndroidVpnConfigFingerprint;

  /// 上一次下发给 iOS PacketTunnel 的配置指纹。
  String? _lastIosPacketTunnelConfigFingerprint;

  /// 最近一次处理的控制同步消息类型，用于抑制相同事件导致的重复重配。
  String? _lastHandledControlSyncMessageType;

  /// 最近一次处理的控制同步重配标记。
  bool _lastHandledControlSyncReconfigureRequired = false;

  /// 最近一次处理的控制同步事件键，用于区分不同版本/不同事件的重配请求。
  String? _lastHandledControlSyncEventKey;

  /// 移动端 MQTT 确保连接流程是否在运行。
  bool _mobileMqttEnsureRunning = false;

  /// 移动端 MQTT 确保连接流程的 in-flight future，用于去重。
  Future<void>? _mobileMqttEnsureInFlight;

  /// 最近一次移动端 MQTT 修复时间，避免高频重试。
  DateTime? _lastNativeMobileMqttRepairAt;

  /// 运行期覆盖的控制面地址，主要给移动端服务器设置使用。
  String? _runtimeControlBaseUrl;

  /// bridge 是否已经关闭，关闭后不再接受后台状态回写。
  bool _closed = false;

  /// 集成测试时强制使用的设备 ID。
  static const _testDeviceId = String.fromEnvironment('SLAN_TEST_DEVICE_ID');

  /// 默认生产控制面地址。
  static const _defaultControlBaseUrl = 'http://47.245.40.231:28080';

  @override
  ValueListenable<ClientViewState> get state => _state;

  @override
  ValueListenable<AndroidNetworkAuthorizationState>
      get androidNetworkAuthorization => _androidNetworkAuthorization;

  /// 启动状态同步、业务事件监听和平台运行态监听。
  ///
  /// 此方法只应调用一次；内部各 watch loop 都有 guard，重复调用不会
  /// 创建多条后台循环。
  @override
  Future<void> start() async {
    if (_closed) {
      return;
    }
    await _loadServerBaseUrl();
    ClientUiDiagnostics.unawaitedLog('bridge.start.begin', state: _state.value);
    await _startStateWithFallback();
    _startBusinessEventWatchLoop();
    _startAndroidNetworkEventWatchLoop();
    _startIosNetworkEventWatchLoop();
    _startAndroidRuntimeStatsLoop();
    _startIosPacketTunnelStatsLoop();
    ClientUiDiagnostics.unawaitedLog('bridge.start.end', state: _state.value);
  }

  /// 返回当前控制面 API 地址。
  ///
  /// 移动端会优先读取原生持久化设置，桌面端使用默认生产地址或环境变量。
  @override
  Future<String> serverBaseUrl() async {
    await _loadServerBaseUrl();
    return _effectiveControlBaseUrl;
  }

  /// 更新移动端控制面 API 地址并写入原生持久化存储。
  @override
  Future<void> updateServerBaseUrl(String serverBaseUrl) async {
    final normalized = _normalizeServerBaseUrl(serverBaseUrl);
    _runtimeControlBaseUrl = normalized;
    if (_usesNativeMobileControlPlane) {
      await _plugin.setMobileServerBaseUrl(normalized);
    }
    ClientUiDiagnostics.unawaitedLog(
      'bridge.serverBaseUrl.updated',
      state: _state.value,
      fields: {'serverBaseUrl': normalized},
    );
  }

  /// 检查 Android VPN 授权并准备可启动的网络配置。
  ///
  /// 已授权且已登录时会读取 `localPlatformNetworkConfig`，这份配置随后用于
  /// Android VpnService 启动数据面。
  @override
  Future<void> prepareAndroidNetworkAuthorization() async {
    if (!_isAndroid) {
      return;
    }
    _setAndroidNetworkAuthorizationState(
      checking: true,
      error: null,
    );
    try {
      final permissionStateResult = await _plugin.androidVpnPermissionState();
      final permissionState =
          ClientCoreLocalService.stringResult(permissionStateResult);
      AndroidVpnConsentRequest? consentRequest;
      AndroidVpnSessionConfig? networkConfig;
      if (permissionState == AndroidVpnPermissionState.needsUserConsent) {
        consentRequest = await _plugin.androidRequestVpnPermission();
      }
      if (permissionState == AndroidVpnPermissionState.granted &&
          _state.value.signedIn) {
        networkConfig = await _preparedAndroidAuthorizationNetworkConfig();
      }
      _androidNetworkAuthorization.value = AndroidNetworkAuthorizationState(
        checking: false,
        permissionState: permissionState,
        consentRequest: consentRequest,
        networkConfig: networkConfig,
      );
      ClientUiDiagnostics.unawaitedLog(
        'bridge.android.authorization.prepared',
        state: _state.value,
        fields: {
          'permissionState': permissionState,
          'hasConsentRequest': consentRequest != null,
          'hasNetworkConfig': networkConfig != null,
        },
      );
    } on Object catch (error) {
      _setAndroidNetworkAuthorizationState(
        checking: false,
        error: error.toString(),
        clearNetworkConfig: true,
      );
      ClientUiDiagnostics.unawaitedLog(
        'bridge.android.authorization.failed',
        state: _state.value,
        fields: {'message': error.toString()},
      );
    }
  }

  /// 汇总 Android 直连候选地址，供调试日志快速判断是否拿到打洞候选。
  String _androidDirectCandidateSummary(AndroidVpnSessionConfig? config) {
    final peerPaths = config?.relayDataPlane?.peerPaths;
    if (peerPaths == null || peerPaths.isEmpty) {
      return '';
    }
    return peerPaths.map((path) {
      final candidates = path.candidates
          .where((candidate) => candidate.kind == PathKind.directUdp)
          .map((candidate) => candidate.address ?? '')
          .where((address) => address.isNotEmpty)
          .join('|');
      return '${path.peerNodeId}:${candidates.isEmpty ? '-' : candidates}';
    }).join(',');
  }

  Future<AndroidVpnSessionConfig?>
      _preparedAndroidAuthorizationNetworkConfig() async {
    var networkConfig = await _platformNetworkConfig();
    final existingRelaySessions = _androidNetworkAuthorization
            .value.networkConfig?.relayDataPlane?.sessions.length ??
        0;
    final nextRelaySessions =
        networkConfig?.relayDataPlane?.sessions.length ?? 0;
    if (existingRelaySessions > 0 && nextRelaySessions == 0) {
      networkConfig = _androidNetworkAuthorization.value.networkConfig;
    }
    _debugLogAndroidNetworkConfig(networkConfig);
    return networkConfig;
  }

  void _debugLogAndroidNetworkConfig(AndroidVpnSessionConfig? networkConfig) {
    debugPrint(
      'SLAN_ANDROID_NETWORK_CONFIG relaySessions='
      '${networkConfig?.relayDataPlane?.sessions.length ?? 0} '
      'virtualIp=${networkConfig?.virtualIp ?? ''} '
      'relayEnabled=${networkConfig?.relayDataPlane?.enabled ?? false} '
      'relayAddress=${networkConfig?.relayDataPlane?.relayAddress ?? networkConfig?.relayAddress ?? ''} '
      'relayUrls=${networkConfig?.relayDataPlane?.sessions.map((session) => session.ticket.relayUrl).where((url) => url.isNotEmpty).join(",") ?? ''} '
      'relayPeerIps=${networkConfig?.relayDataPlane?.sessions.map((session) => session.peerVirtualIps.join("|")).join(",") ?? ''} '
      'peerPaths=${networkConfig?.relayDataPlane?.peerPaths.length ?? 0} '
      'pathKinds=${_androidPathKindSummary(networkConfig)} '
      'directCandidates=${_androidDirectCandidateSummary(networkConfig)} '
      'routes=${networkConfig?.routes.map((route) => route['destination']).join(",") ?? ''}',
    );
  }

  /// 汇总 Android 路径候选类型和状态。
  String _androidPathKindSummary(AndroidVpnSessionConfig? config) {
    final peerPaths = config?.relayDataPlane?.peerPaths;
    if (peerPaths == null || peerPaths.isEmpty) {
      return '';
    }
    return peerPaths.map((path) {
      final kinds = path.candidates
          .map((candidate) => '${candidate.kind}:${candidate.state}')
          .where((value) => value.isNotEmpty)
          .join('|');
      return '${path.peerNodeId}:${kinds.isEmpty ? '-' : kinds}';
    }).join(',');
  }

  /// 派发 UI 命令。
  ///
  /// 这个方法是页面层唯一命令入口，会根据平台和命令类型决定走本地服务、
  /// 移动端内嵌服务、原生 VPN/PacketTunnel，或打开浏览器。
  @override
  Future<void> dispatch(ClientCommand command) async {
    ClientUiDiagnostics.unawaitedLog(
      'bridge.dispatch.begin',
      state: _state.value,
      fields: _commandLogFields(command),
    );
    if (command.type == ClientCommandType.openClientLogin) {
      _localLogoutRequested = false;
    }
    if (command.type == ClientCommandType.logout) {
      _desktopBrowserLoginWatchEpoch++;
      _clearNetworkToggle();
      _localLogoutRequested = true;
      _setStateIfChanged(ClientViewState.initial());
      ClientUiDiagnostics.unawaitedLog(
        'bridge.logout.localCleared',
        state: _state.value,
      );
      try {
        await _requestLocalService('localLogout');
        ClientUiDiagnostics.unawaitedLog(
          'bridge.logout.serviceCleared',
          state: _state.value,
        );
      } on Object catch (error) {
        ClientUiDiagnostics.unawaitedLog(
          'bridge.logout.serviceClearFailed',
          state: _state.value,
          fields: {'message': error.toString()},
        );
      }
      await _refreshState();
      return;
    }
    if (_isNetworkToggle(command.type)) {
      if (_isAndroid) {
        _startAsyncAndroidNetworkToggle(command);
        return;
      }
      if (_isIos) {
        _startAsyncIosNetworkToggle(command);
        return;
      }
      _startAsyncNetworkToggle(command);
      return;
    }
    if (command.type == ClientCommandType.localNetworkShutdown) {
      await _localNetworkShutdownWithFallback();
      await _refreshState();
      return;
    }
    await _dispatchControlWithFallback(command);
    if (command.type == ClientCommandType.openClientLogin &&
        _isDesktopHostPlatform &&
        !_state.value.signedIn) {
      _startDesktopBrowserLoginStateWatch();
    }
  }

  /// 查询控制通道状态。
  ///
  /// 移动端发现 MQTT 未连接但凭证已就绪时，会异步触发一次修复流程。
  @override
  Future<ControlTransportStatus?> localControlStatus() async {
    if (_closed) {
      return null;
    }
    try {
      final result = await _localControlStatusWithFallback();
      final json = _resultMap(result);
      final status =
          json == null ? null : ControlTransportStatus.fromJson(json);
      ClientUiDiagnostics.unawaitedLog(
        'bridge.localControlStatus.result',
        state: _state.value,
        fields: {
          'ready': status?.ready,
          'mqttConnected': status?.mqttConnected,
          'mqttLastError': status?.mqttLastError,
          'mqttLastMessageTopic': status?.mqttLastMessageTopic,
          'mqttLastMessageType': status?.mqttLastMessageType,
          'lastMqttPublishSummary': status?.lastMqttPublishSummary,
          'mqttNetworkEventTopic': status?.mqttNetworkEventTopic,
          'mqttNetworkEventSubscribed': status?.mqttNetworkEventSubscribed,
          'activeNetworkId': status?.activeNetworkId,
          'deviceId': status?.deviceId,
        },
      );
      if (_usesNativeMobileControlPlane &&
          status?.ready == true &&
          status?.mqttConnected != true) {
        unawaited(_repairNativeMobileMqttIfNeeded(
          'bridge.localControlStatus.mqttRepair',
        ));
      }
      return status;
    } on Object {
      return null;
    }
  }

  @override
  Future<void> close() async {
    _closed = true;
    _desktopBrowserLoginWatchEpoch++;
    _watchingBusinessEvents = false;
    _watchingAndroidNetworkEvents = false;
    _watchingIosNetworkEvents = false;
    _watchingAndroidRuntimeStats = false;
    _watchingIosPacketTunnelStats = false;
    _mobileMqttEnsureRunning = false;
    _mobileMqttEnsureInFlight = null;
    _repairingNativeMobileMqtt = false;
    _clearNetworkToggle();
  }

  /// 浏览器登录期间轮询本地 Rust 状态，作为业务事件长轮询的桌面兜底。
  ///
  /// 请求只访问 loopback `client-core-service`；收到登录态、退出或超时后立即
  /// 停止。这样即使原生托盘和 Flutter 同时持有长轮询，也不会让 UI 停在
  /// “打开浏览器登录”页面。
  void _startDesktopBrowserLoginStateWatch() {
    final epoch = ++_desktopBrowserLoginWatchEpoch;
    unawaited(Future<void>(() async {
      final deadline = DateTime.now().add(const Duration(minutes: 10));
      while (!_closed &&
          epoch == _desktopBrowserLoginWatchEpoch &&
          DateTime.now().isBefore(deadline)) {
        try {
          final next = await _queryCurrentState();
          if (next?.signedIn == true) {
            _setStateIfChanged(next!);
            return;
          }
        } on Object catch (error) {
          ClientUiDiagnostics.unawaitedLog(
            'bridge.desktopBrowserLogin.stateWatchFailed',
            state: _state.value,
            fields: {'message': error.toString()},
          );
        }
        await _pauseIfActive(
          const Duration(milliseconds: 500),
          () => !_closed && epoch == _desktopBrowserLoginWatchEpoch,
        );
      }
    }));
  }

  Future<void> _pauseIfActive(
    Duration duration,
    bool Function() isActive,
  ) async {
    if (!isActive()) {
      return;
    }
    await Future<void>.delayed(duration);
  }

  void _startWatchLoop({
    required bool Function() isAlreadyWatching,
    required void Function() markWatching,
    required bool Function() isActive,
    required Future<void> Function() runOnce,
    required Duration errorBackoff,
    required String errorEvent,
  }) {
    if (isAlreadyWatching()) {
      return;
    }
    markWatching();
    unawaited(Future<void>(() async {
      while (isActive()) {
        try {
          await runOnce();
        } on Object catch (error) {
          ClientUiDiagnostics.unawaitedLog(
            errorEvent,
            state: _state.value,
            fields: {'message': error.toString()},
          );
          await _pauseIfActive(errorBackoff, isActive);
        }
      }
    }));
  }

  void _startPollingLoop({
    required bool Function() isAlreadyWatching,
    required void Function() markWatching,
    required bool Function() isActive,
    required Future<void> Function() pollOnce,
    required Duration interval,
    required Duration errorBackoff,
    required String errorEvent,
  }) {
    if (isAlreadyWatching()) {
      return;
    }
    markWatching();
    unawaited(Future<void>(() async {
      while (isActive()) {
        try {
          await pollOnce();
          await _pauseIfActive(interval, isActive);
        } on MissingPluginException {
          await _pauseIfActive(errorBackoff, isActive);
        } on Object catch (error) {
          ClientUiDiagnostics.unawaitedLog(
            errorEvent,
            state: _state.value,
            fields: {'message': error.toString()},
          );
          await _pauseIfActive(errorBackoff, isActive);
        }
      }
    }));
  }

  /// 判断命令是否属于网络开关。
  bool _isNetworkToggle(ClientCommandType type) {
    return type == ClientCommandType.enableNetwork ||
        type == ClientCommandType.disableNetwork;
  }

  /// 启动时读取当前状态。
  ///
  /// 移动端走内嵌服务 `start`，桌面端走本地进程 `start`。桌面服务不可用时
  /// 不让 UI 崩溃，而是进入“本地服务未连接”的可恢复状态。
  Future<void> _startStateWithFallback() async {
    final state = await _applyStatefulControlPlaneRequest(
      embeddedMethod: 'start',
      embeddedUnavailableMessage: 'embedded start is not available',
      localMethod: 'start',
      fallbackEvent: 'bridge.start.fallback',
      onLocalError: (_) => _setStateIfChanged(_localServiceNotConnectedState()),
    );
    if (state?.signedIn == true) {
      unawaited(_ensureDeviceThenConnectMqtt('bridge.start.mqtt'));
    }
  }

  /// 获取当前平台数据面需要的网络配置。
  ///
  /// 移动端从内嵌服务读取；桌面端从独立本地服务读取。返回值使用插件包内
  /// 的 [AndroidVpnSessionConfig]，iOS 也复用这个结构表达 PacketTunnel 配置。
  Future<AndroidVpnSessionConfig?> _platformNetworkConfig() async {
    if (_usesNativeMobileControlPlane) {
      final embedded = await _embeddedServiceRequest(
        'localPlatformNetworkConfig',
        null,
        true,
      );
      if (embedded == null) {
        throw StateError('embedded platform network config is not available');
      }
      final relayDebug = embedded['relayDebug'];
      if (relayDebug != null) {
        debugPrint(
          'SLAN_EMBEDDED_RELAY_DEBUG='
          '${jsonEncode(relayDebugSummary(relayDebug))}',
        );
      }
      return AndroidVpnSessionConfig.fromJson(embedded);
    }
    return _runLoggedLocalFallback(
      fallbackEvent: 'bridge.platformNetworkConfig.fallback',
      run: _localService.localPlatformNetworkConfig,
    );
  }

  /// 派发普通控制命令。
  ///
  /// 登录、刷新、打开登录等命令先交给服务处理；桌面端还会在需要时打开
  /// Web Console 或浏览器登录页。
  Future<void> _dispatchControlWithFallback(ClientCommand command) async {
    if (_usesNativeMobileControlPlane) {
      final embedded = await _embeddedServiceRequest(
        'dispatch',
        command.toJson(),
        true,
      );
      if (embedded == null) {
        throw StateError('embedded control dispatch is not available');
      }
      final state = _applyStateFromResult(embedded);
      await _afterEmbeddedControlDispatch(command, state);
      return;
    }
    if (_usesDesktopBrowserPlugin(command.type)) {
      ClientUiDiagnostics.unawaitedCriticalLog(
        'bridge.browser.dispatch.begin',
        state: _state.value,
        fields: {'command': command.type.name},
      );
      try {
        final result = await _plugin.dispatch(command.toJson());
        final state = _applyStateFromResult(result);
        ClientUiDiagnostics.unawaitedCriticalLog(
          'bridge.browser.dispatch.completed',
          state: _state.value,
          fields: {'command': command.type.name},
        );
        if (_isMacOS) {
          await _afterLocalControlDispatch(command, state);
        }
        return;
      } on MissingPluginException catch (error) {
        _logDesktopBrowserPluginFailure(
          'bridge.desktopBrowserPlugin.missing',
          command,
          error,
        );
      } on PlatformException catch (error) {
        _logDesktopBrowserPluginFailure(
          'bridge.desktopBrowserPlugin.failed',
          command,
          error,
        );
      }
    }
    try {
      final result = await _requestLocalService('dispatch', command.toJson());
      final state = _applyStateFromResult(result);
      await _afterLocalControlDispatch(command, state);
      return;
    } on Object catch (error) {
      _logCommandFailure(
        'bridge.controlPlane.fallback',
        command,
        error,
      );
      rethrow;
    }
  }

  /// embedded control dispatch 完成后的补动作。
  ///
  /// 当前仅在移动端密码登录成功后补做 ensureDevice + MQTT 连接。
  Future<void> _afterEmbeddedControlDispatch(
    ClientCommand command,
    ClientViewState? state,
  ) async {
    if (command.type == ClientCommandType.loginWithPassword &&
        state?.signedIn == true) {
      await _ensureDeviceThenConnectMqtt('bridge.login.mqtt');
    }
  }

  /// 桌面端本地 dispatch 完成后的补动作。
  ///
  /// 当前仅负责按命令类型打开浏览器；单独抽出便于后续继续追加桌面后处理。
  Future<void> _afterLocalControlDispatch(
    ClientCommand command,
    ClientViewState? state,
  ) async {
    await _openDesktopBrowserForCommand(command, state);
  }

  /// 统一记录桌面浏览器插件失败日志。
  void _logDesktopBrowserPluginFailure(
    String event,
    ClientCommand command,
    Object error,
  ) {
    _logCommandFailure(event, command, error);
  }

  void _logCommandFailure(
    String event,
    ClientCommand command,
    Object error,
  ) {
    ClientUiDiagnostics.unawaitedLog(
      event,
      state: _state.value,
      fields: {
        ..._commandLogFields(command),
        'message': error.toString(),
      },
    );
  }

  Map<String, Object?> _commandLogFields(ClientCommand command) {
    return {'command': command.type.name};
  }

  /// 桌面端登录/控制台命令需要原生插件帮忙自动启动 service 并打开浏览器。
  bool _usesDesktopBrowserPlugin(ClientCommandType type) {
    return (_isMacOS || _isWindows || _isLinux) &&
        (type == ClientCommandType.openClientLogin ||
            type == ClientCommandType.openWebConsole);
  }

  /// 关闭本地网络数据面。
  ///
  /// 该方法用于清理场景，不等价于用户退出；它只要求平台网络停止。
  Future<void> _localNetworkShutdownWithFallback() async {
    if (_usesNativeMobileControlPlane) {
      final embedded = await _embeddedServiceRequest('localNetworkShutdown');
      if (embedded == null) {
        throw StateError('embedded local network shutdown is not available');
      }
      return;
    }
    await _runLoggedLocalFallback(
      fallbackEvent: 'bridge.localNetworkShutdown.fallback',
      run: () => _requestLocalService('localNetworkShutdown'),
    );
  }

  /// 根据命令决定是否打开桌面浏览器。
  ///
  /// 移动端没有桌面浏览器联动，直接返回。
  Future<void> _openDesktopBrowserForCommand(
    ClientCommand command,
    ClientViewState? state,
  ) async {
    if (_usesNativeMobileControlPlane) {
      return;
    }
    if (command.type == ClientCommandType.openClientLogin) {
      await _openWebConsoleUrl(
        deviceId: state?.deviceId,
        browserLogin: true,
      );
      return;
    }
    if (command.type == ClientCommandType.openWebConsole) {
      await _openWebConsoleWithLoginKey(state);
    }
  }

  /// 为 Web Console 生成一次性登录 key 并打开控制台。
  Future<void> _openWebConsoleWithLoginKey(ClientViewState? state) async {
    String? consoleLoginKey;
    String? deviceId = state?.deviceId ?? _state.value.deviceId;
    try {
      final response = await _requestLocalService('consoleLoginKey');
      final json = _resultMap(response);
      consoleLoginKey = json?['loginKey'] as String?;
      deviceId = (json?['deviceId'] as String?) ?? deviceId;
    } on Object catch (error) {
      ClientUiDiagnostics.unawaitedLog(
        'bridge.openConsole.loginKeyFailed',
        state: _state.value,
        fields: {'message': error.toString()},
      );
    }
    if ((consoleLoginKey ?? '').trim().isEmpty) {
      throw StateError('failed to create Web Console login key');
    }
    await _openWebConsoleUrl(
      consoleLoginKey: consoleLoginKey,
      deviceId: deviceId,
    );
  }

  /// 构造并打开 Web Console URL。
  ///
  /// `browserLogin=true` 表示从客户端触发登录流程；带 `consoleLoginKey`
  /// 表示浏览器可直接完成控制台登录。
  Future<void> _openWebConsoleUrl({
    String? consoleLoginKey,
    String? deviceId,
    bool browserLogin = false,
  }) async {
    final query = <String, String>{};
    if (browserLogin) {
      query['auth'] = 'login';
      query['source'] = 'client';
    }
    final loginKey = consoleLoginKey?.trim() ?? '';
    if (loginKey.isNotEmpty) {
      query['consoleLoginKey'] = loginKey;
    }
    final safeDeviceId = _usableClientDeviceId(deviceId);
    if (safeDeviceId.isNotEmpty) {
      query['deviceId'] = safeDeviceId;
    }
    final base = Uri.parse(_resolveWebConsoleUrl());
    final url = base.replace(queryParameters: {
      ...base.queryParameters,
      ...query,
    }).toString();
    ClientUiDiagnostics.unawaitedCriticalLog(
      'bridge.browser.url.prepared',
      state: _state.value,
      fields: {
        'host': base.host,
        'port': base.hasPort ? base.port : null,
        'browserLogin': browserLogin,
        'hasConsoleLoginKey': loginKey.isNotEmpty,
        'hasDeviceId': safeDeviceId.isNotEmpty,
      },
    );
    await _openExternalUrl(url);
  }

  /// 用当前桌面平台的系统命令打开外部 URL。
  Future<void> _openExternalUrl(String url) async {
    final override = _openExternalUrlOverride;
    if (override != null) {
      await override(url);
      return;
    }
    if (!_isDesktopHostPlatform) {
      throw UnsupportedError('open browser is not supported on this platform');
    }
    if (_isMacOS) {
      ClientUiDiagnostics.unawaitedCriticalLog(
        'bridge.browser.native.begin',
        state: _state.value,
      );
      try {
        final opened = await _plugin.openExternalUrl(url);
        ClientUiDiagnostics.unawaitedCriticalLog(
          'bridge.browser.native.completed',
          state: _state.value,
          fields: {'opened': opened},
        );
        if (!opened) {
          throw StateError('macOS did not open the browser');
        }
      } on Object catch (error) {
        ClientUiDiagnostics.unawaitedCriticalLog(
          'bridge.browser.native.failed',
          state: _state.value,
          fields: {
            'errorType': error.runtimeType.toString(),
            'message': error.toString(),
          },
        );
        rethrow;
      }
      return;
    }
    if (_isWindows) {
      await _startDetachedProcess('cmd', ['/c', 'start', '', url]);
      return;
    }
    if (_isLinux) {
      await _startDetachedProcess('xdg-open', [url]);
      return;
    }
    throw UnsupportedError('open browser is not supported on this platform');
  }

  Future<void> _startDetachedProcess(
    String executable,
    List<String> arguments,
  ) async {
    await Process.start(
      executable,
      arguments,
      mode: ProcessStartMode.detached,
    );
  }

  /// 根据当前控制面地址推导 Web Console 地址。
  String _resolveWebConsoleUrl() {
    final controlBaseUrl = _effectiveControlBaseUrl;
    final controlUri = Uri.tryParse(controlBaseUrl);
    final controlHost = controlUri?.host.toLowerCase() ?? '';
    final controlScheme =
        (controlUri?.scheme ?? '').isEmpty ? 'http' : controlUri!.scheme;
    if (controlHost == 'api.dev.staticlss.com') {
      return Uri(
        scheme: controlScheme,
        host: 'web.dev.staticlss.com',
      ).toString();
    }
    if (controlHost == 'api.slan.localhost' ||
        controlHost == 'slan.localhost') {
      return Uri(
        scheme: controlScheme,
        host: 'web.slan.localhost',
      ).toString();
    }
    if (controlHost == '127.0.0.1' ||
        controlHost == 'localhost' ||
        controlHost == '::1' ||
        controlUri?.port == 28080) {
      return Uri(
        scheme: controlScheme,
        host: controlUri?.host ?? '127.0.0.1',
        port: 24200,
      ).toString();
    }
    return 'http://47.245.40.231:24200';
  }

  /// 清理并返回可放进 Web 登录 URL 的设备 ID。
  String _usableClientDeviceId(String? deviceId) {
    return deviceId?.trim() ?? '';
  }

  /// 查询控制通道状态，按平台选择本地服务或内嵌服务。
  Future<Object?> _localControlStatusWithFallback() async {
    if (_usesNativeMobileControlPlane) {
      final embedded = await _embeddedServiceRequest('localControlStatus');
      if (embedded == null) {
        throw StateError('embedded local control status is not available');
      }
      return embedded;
    }
    try {
      return await _localService.localControlStatus();
    } on Object catch (error) {
      ClientUiDiagnostics.unawaitedLog(
        'bridge.localControlStatus.fallback',
        state: _state.value,
        fields: {'message': error.toString()},
      );
      rethrow;
    }
  }

  /// 调用移动端内嵌 client-core-service。
  ///
  /// Android/iOS 不启动独立本地进程，而是通过原生插件把 JSON 请求交给
  /// Rust FFI 内嵌服务。此方法统一做 JSON 编码、错误规约和诊断日志。
  Future<Map<String, Object?>?> _embeddedServiceRequest(
    String method, [
    Object? arguments,
    bool rethrowErrors = false,
  ]) async {
    if (!_usesNativeMobileControlPlane) {
      return null;
    }
    final embeddedArgs = _embeddedArguments(arguments);
    try {
      final response = await _plugin.embeddedServiceRequest(jsonEncode({
        'method': method,
        'args': embeddedArgs,
      }));
      final error = response?['error'];
      if (error is String && error.trim().isNotEmpty) {
        throw PlatformException(
          code: 'embedded_service_error',
          message: error,
        );
      }
      return response;
    } on Object catch (error) {
      ClientUiDiagnostics.unawaitedLog(
        'bridge.embeddedService.failed',
        state: _state.value,
        fields: {
          'method': method,
          'message': error.toString(),
        },
      );
      if (rethrowErrors) {
        rethrow;
      }
      return null;
    }
  }

  /// 为移动端内嵌服务补齐公共参数。
  ///
  /// 每个内嵌服务请求都需要知道控制面地址。设备身份 override 必须显式传
  /// `deviceIdOverride`，避免普通业务字段 `deviceId` 被误解释成全局身份切换。
  Object _embeddedArguments(Object? arguments) {
    final controlBaseUrl = _effectiveControlBaseUrl;
    final testDeviceId = _testDeviceId.trim();
    if (controlBaseUrl.isEmpty) {
      if (arguments is Map) {
        return {
          ...arguments.cast<String, Object?>(),
          if (testDeviceId.isNotEmpty) 'deviceIdOverride': testDeviceId,
        };
      }
      return {
        if (arguments != null) 'value': arguments,
        if (testDeviceId.isNotEmpty) 'deviceIdOverride': testDeviceId,
      };
    }
    if (arguments is Map) {
      return {
        ...arguments.cast<String, Object?>(),
        'controlBaseUrl': controlBaseUrl,
        if (testDeviceId.isNotEmpty) 'deviceIdOverride': testDeviceId,
      };
    }
    return {
      'value': arguments,
      'controlBaseUrl': controlBaseUrl,
      if (testDeviceId.isNotEmpty) 'deviceIdOverride': testDeviceId,
    };
  }

  /// 加载运行期控制面地址。
  ///
  /// 移动端优先读取原生持久化地址；没有配置时退回 dart-define 或生产默认值。
  Future<void> _loadServerBaseUrl() async {
    if (_runtimeControlBaseUrl != null) {
      return;
    }
    var value = '';
    if (_usesNativeMobileControlPlane) {
      value = (await _plugin.mobileServerBaseUrl()) ?? '';
    }
    _runtimeControlBaseUrl = _normalizeServerBaseUrl(
      value.isNotEmpty ? value : _defaultEmbeddedControlBaseUrl,
    );
  }

  /// 当前实际用于控制面请求的 API 地址。
  String get _effectiveControlBaseUrl =>
      _runtimeControlBaseUrl ??
      _normalizeServerBaseUrl(_defaultEmbeddedControlBaseUrl);

  static int? _intValue(Object? value) {
    if (value is int) {
      return value;
    }
    if (value is num) {
      return value.toInt();
    }
    if (value is String) {
      return int.tryParse(value);
    }
    return null;
  }

  static bool? _boolValue(Object? value) {
    if (value is bool) {
      return value;
    }
    if (value is String) {
      final normalized = value.trim().toLowerCase();
      if (normalized == 'true') {
        return true;
      }
      if (normalized == 'false') {
        return false;
      }
    }
    return null;
  }

  /// 移动端内嵌服务的默认控制面地址。
  String get _defaultEmbeddedControlBaseUrl => _defaultControlBaseUrl;

  /// 归一化服务地址，保证协议存在并去掉末尾 `/`。
  static String _normalizeServerBaseUrl(String value) {
    var normalized = value.trim();
    if (normalized.isEmpty) {
      normalized = _defaultControlBaseUrl;
    }
    if (!normalized.contains('://')) {
      normalized = 'http://$normalized';
    }
    return normalized.replaceFirst(RegExp(r'/+$'), '');
  }

  /// 确保移动端设备注册完成并建立 MQTT 控制连接。
  ///
  /// 登录、启动和修复流程都可能触发该动作；这里通过 in-flight future 去重，
  /// 避免同一时间重复注册设备或重复连接 MQTT。
  Future<void> _ensureDeviceThenConnectMqtt(String event) async {
    if (!_usesNativeMobileControlPlane) {
      return;
    }
    if (_mobileMqttEnsureRunning) {
      await _awaitMobileMqttEnsureInFlight();
      return;
    }
    if (_mobileMqttEnsureInFlight != null) {
      await _awaitMobileMqttEnsureInFlight();
      return;
    }
    _mobileMqttEnsureRunning = true;
    final task = _ensureDeviceThenConnectMqttOnce(event);
    _mobileMqttEnsureInFlight = task;
    try {
      await task;
    } finally {
      _mobileMqttEnsureRunning = false;
      if (identical(_mobileMqttEnsureInFlight, task)) {
        _mobileMqttEnsureInFlight = null;
      }
    }
  }

  Future<void> _awaitMobileMqttEnsureInFlight() async {
    final inFlight = _mobileMqttEnsureInFlight;
    if (inFlight != null) {
      await inFlight;
    }
  }

  /// 执行一次移动端设备注册 + MQTT 连接。
  Future<void> _ensureDeviceThenConnectMqttOnce(String event) async {
    final device =
        await _embeddedServiceRequest('localEnsureDevice', null, true);
    ClientUiDiagnostics.unawaitedLog(
      '$event.ensureDevice',
      state: _state.value,
      fields: device ?? {'accepted': false},
    );
    if (device == null) {
      return;
    }
    final mqtt = await _embeddedServiceRequest('localConnectControlMqtt');
    ClientUiDiagnostics.unawaitedLog(
      '$event.connectMqtt',
      state: _state.value,
      fields: mqtt ?? {'connected': false},
    );
  }

  /// 移动端 MQTT 断开时的轻量修复入口。
  ///
  /// 该方法有 15 秒冷却时间，避免 watch loop 和状态查询同时失败时产生重试风暴。
  Future<void> _repairNativeMobileMqttIfNeeded(String event) async {
    if (!_usesNativeMobileControlPlane || _repairingNativeMobileMqtt) {
      return;
    }
    final now = DateTime.now();
    final last = _lastNativeMobileMqttRepairAt;
    if (last != null && now.difference(last) < const Duration(seconds: 15)) {
      return;
    }
    _lastNativeMobileMqttRepairAt = now;
    _repairingNativeMobileMqtt = true;
    try {
      final status = await localControlStatus();
      if (status?.mqttConnected == true) {
        return;
      }
      await _ensureDeviceThenConnectMqtt(event);
    } on Object catch (error) {
      ClientUiDiagnostics.unawaitedLog(
        '$event.failed',
        state: _state.value,
        fields: {'message': error.toString()},
      );
    } finally {
      _repairingNativeMobileMqtt = false;
    }
  }

  /// 清除当前网络切换上下文，并让旧异步回调失效。
  void _clearNetworkToggle() {
    _networkToggleEpoch++;
    _networkToggleInFlight = false;
    _networkToggleOperation = null;
  }

  /// 桌面端网络开关流程。
  ///
  /// 先做 UI 乐观更新，然后调用本地服务；最终状态以业务事件或服务返回错误为准。
  void _startAsyncNetworkToggle(ClientCommand command) {
    final operation = _beginNetworkToggle(
      command,
      enableMethod: 'localNetworkActivate',
      disableMethod: 'localNetworkDeactivate',
      ignoredEvent: 'bridge.switch.ignoredInFlight',
      clearVirtualIpWhenDisabling: true,
    );
    if (operation == null) {
      return;
    }
    ClientUiDiagnostics.unawaitedLog(
      'bridge.switch.serviceApi',
      state: _state.value,
      fields: {'method': operation.method, 'command': operation.command.name},
    );
    ClientUiDiagnostics.unawaitedLog(
      'bridge.switch.pending',
      state: _state.value,
      fields: {
        'command': operation.command.name,
        'epoch': operation.epoch,
        'optimisticNetworkEnabled': operation.targetEnabled,
      },
    );
    unawaited(
      Future<void>(() async {
        await Future<void>.delayed(_networkToggleTimeout);
        if (!_isCurrentNetworkToggle(operation)) {
          return;
        }
        _finishNetworkToggle(operation);
        _setStateIfChanged(_networkToggleFailureState(
          operation,
          'network switch timed out',
        ));
        ClientUiDiagnostics.unawaitedLog(
          'bridge.switch.eventTimeout',
          state: _state.value,
          fields: {
            'command': operation.command.name,
            'epoch': operation.epoch,
          },
        );
      }),
    );
    unawaited(
      _runNetworkToggle(
        operation,
        execute: (operation) async {
          final result = await _requestLocalService(
            operation.method,
          ).timeout(_networkToggleTimeout);
          ClientUiDiagnostics.unawaitedLog(
            'bridge.switch.serviceResult',
            state: _state.value,
            fields: {
              'command': operation.command.name,
              'epoch': operation.epoch,
              'stale': operation.epoch != _networkToggleEpoch,
              'hasResult': result != null,
            },
          );
          if (!_isCurrentNetworkToggle(operation)) {
            return _state.value;
          }
          _throwStateErrorText(_stateFromResult(result));
          ClientUiDiagnostics.unawaitedLog(
            'bridge.switch.waitingBusinessEvent',
            state: _state.value,
            fields: {
              'command': operation.command.name,
              'epoch': operation.epoch,
            },
          );
          return _state.value;
        },
        onError: (error) async {
          if (error is MissingPluginException) {
            _setStateIfChanged(_networkToggleFailureState(
              operation,
              'local service not connected',
              notice: 'localServiceNotConnected',
            ));
            ClientUiDiagnostics.unawaitedLog(
              'bridge.switch.missingPlugin',
              state: _state.value,
              fields: {
                'command': operation.command.name,
                'epoch': operation.epoch,
              },
            );
            return;
          }
          if (error is TimeoutException) {
            ClientUiDiagnostics.unawaitedLog(
              'bridge.switch.timeout',
              state: _state.value,
              fields: {
                'command': operation.command.name,
                'epoch': operation.epoch,
                'message': error.toString(),
              },
            );
            return;
          }
          if (error is PlatformException) {
            ClientUiDiagnostics.unawaitedLog(
              'bridge.switch.platformError',
              state: _state.value,
              fields: {
                'command': operation.command.name,
                'epoch': operation.epoch,
                'code': error.code,
                'message': error.message,
              },
            );
            return;
          }
          ClientUiDiagnostics.unawaitedLog(
            'bridge.switch.error',
            state: _state.value,
            fields: {
              'command': operation.command.name,
              'epoch': operation.epoch,
              'message': error.toString(),
            },
          );
        },
        finishedEvent: 'bridge.switch.waitingBusinessEvent',
        failedEvent: 'bridge.switch.failed',
      ),
    );
  }

  NetworkToggleOperation? _beginNetworkToggle(
    ClientCommand command, {
    required String enableMethod,
    required String disableMethod,
    required String ignoredEvent,
    required bool clearVirtualIpWhenDisabling,
  }) {
    if (_networkToggleInFlight) {
      ClientUiDiagnostics.unawaitedLog(
        ignoredEvent,
        state: _state.value,
        fields: _commandLogFields(command),
      );
      return null;
    }
    final previousState = _state.value;
    final epoch = ++_networkToggleEpoch;
    _networkToggleInFlight = true;
    final targetEnabled = command.type == ClientCommandType.enableNetwork;
    final operation = NetworkToggleOperation(
      epoch: epoch,
      command: command.type,
      method: targetEnabled ? enableMethod : disableMethod,
      targetEnabled: targetEnabled,
      previousState: previousState,
    );
    _networkToggleOperation = operation;
    _setStateIfChanged(_networkTogglePendingState(
      command: command,
      targetEnabled: targetEnabled,
      clearVirtualIpWhenDisabling: clearVirtualIpWhenDisabling,
    ));
    return operation;
  }

  NetworkToggleOperation? _beginMobileNetworkToggle(
    ClientCommand command, {
    required String enableMethod,
    required String disableMethod,
    required String ignoredEvent,
  }) {
    return _beginNetworkToggle(
      command,
      enableMethod: enableMethod,
      disableMethod: disableMethod,
      ignoredEvent: ignoredEvent,
      clearVirtualIpWhenDisabling: true,
    );
  }

  Future<void> _runNetworkToggle(
    NetworkToggleOperation operation, {
    required Future<ClientViewState> Function(NetworkToggleOperation operation)
        execute,
    required Future<void> Function(Object error) onError,
    required String finishedEvent,
    required String failedEvent,
  }) async {
    try {
      final nextState = await execute(operation);
      if (!_isCurrentNetworkToggle(operation)) {
        return;
      }
      _finishNetworkToggle(operation);
      _setStateIfChanged(nextState);
      ClientUiDiagnostics.unawaitedLog(
        finishedEvent,
        state: _state.value,
        fields: {
          'command': operation.command.name,
          'epoch': operation.epoch,
        },
      );
    } on Object catch (error) {
      if (!_isCurrentNetworkToggle(operation)) {
        return;
      }
      _finishNetworkToggle(operation);
      _setStateIfChanged(_networkToggleFailureState(
        operation,
        error.toString(),
      ));
      await onError(error);
      ClientUiDiagnostics.unawaitedLog(
        failedEvent,
        state: _state.value,
        fields: {
          'command': operation.command.name,
          'epoch': operation.epoch,
          'message': error.toString(),
        },
      );
    }
  }

  ClientViewState _networkTogglePendingState({
    required ClientCommand command,
    required bool targetEnabled,
    required bool clearVirtualIpWhenDisabling,
  }) {
    return _pendingNetworkUiState(
      _state.value,
      networkEnabled: targetEnabled,
      syncReason: command.type.name,
      error: null,
      notice: null,
      clearVirtualIp: clearVirtualIpWhenDisabling && !targetEnabled,
    );
  }

  Future<void> _runMobileNetworkToggle(
    NetworkToggleOperation operation, {
    required Future<ClientViewState> Function(NetworkToggleOperation operation)
        execute,
    required Future<void> Function(Object error) onError,
    required String finishedEvent,
    required String failedEvent,
  }) {
    return _runNetworkToggle(
      operation,
      execute: execute,
      onError: onError,
      finishedEvent: finishedEvent,
      failedEvent: failedEvent,
    );
  }

  /// 启动平台网络开关异步流程。
  ///
  /// Android / iOS 的主要差异通过参数传入，骨架负责复用“创建操作、
  /// 可选前置日志、启动异步执行”的一致时序。
  void _startAsyncPlatformNetworkToggle(
    ClientCommand command, {
    required String enableMethod,
    required String disableMethod,
    required String ignoredEvent,
    required Future<ClientViewState> Function(NetworkToggleOperation operation)
        execute,
    required Future<void> Function(Object error) onError,
    required String finishedEvent,
    required String failedEvent,
    Future<void> Function()? beforeRun,
  }) {
    final operation = _beginMobileNetworkToggle(
      command,
      enableMethod: enableMethod,
      disableMethod: disableMethod,
      ignoredEvent: ignoredEvent,
    );
    if (operation == null) {
      return;
    }
    if (beforeRun != null) {
      unawaited(beforeRun());
    }
    unawaited(
      _runMobileNetworkToggle(
        operation,
        execute: execute,
        onError: onError,
        finishedEvent: finishedEvent,
        failedEvent: failedEvent,
      ),
    );
  }

  /// Android 网络开关流程。
  ///
  /// Android 需要先确认 VPN 授权和网络配置，再通过原生插件启动 VpnService。
  void _startAsyncAndroidNetworkToggle(ClientCommand command) {
    _startAsyncPlatformNetworkToggle(
      command,
      enableMethod: 'androidStartVpn',
      disableMethod: 'androidStopVpn',
      ignoredEvent: 'bridge.android.switch.ignoredInFlight',
      execute: _executeAndroidNetworkToggle,
      onError: (error) async {
        _setAndroidNetworkAuthorizationState(
          checking: false,
          error: error.toString(),
        );
      },
      finishedEvent: 'bridge.android.switch.finished',
      failedEvent: 'bridge.android.switch.failed',
      beforeRun: () => _logAndroidRuntimeState('bridge.android.switch.runtime'),
    );
  }

  /// iOS 网络开关流程。
  ///
  /// iOS 复用平台网络配置结构，但实际启动的是 PacketTunnel 扩展。
  void _startAsyncIosNetworkToggle(ClientCommand command) {
    _startAsyncPlatformNetworkToggle(
      command,
      enableMethod: 'iosEnableNetwork',
      disableMethod: 'iosDisableNetwork',
      ignoredEvent: 'bridge.ios.switch.ignoredInFlight',
      execute: _executeIosNetworkToggle,
      onError: (_) async {},
      finishedEvent: 'bridge.ios.switch.finished',
      failedEvent: 'bridge.ios.switch.failed',
    );
  }

  /// 网络切换最长等待时间，覆盖服务调用和平台数据面启动。
  static const Duration _networkToggleTimeout = Duration(seconds: 45);

  /// 判断异步回调是否仍属于当前网络开关操作。
  bool _isCurrentNetworkToggle(NetworkToggleOperation operation) {
    return _networkToggleOperation == operation &&
        operation.epoch == _networkToggleEpoch;
  }

  /// 结束当前网络开关操作，并让后续旧回调失效。
  void _finishNetworkToggle(NetworkToggleOperation operation) {
    if (!_isCurrentNetworkToggle(operation)) {
      return;
    }
    _networkToggleInFlight = false;
    _networkToggleOperation = null;
    _networkToggleEpoch++;
  }

  /// 构造网络开关失败后的 UI 状态。
  ///
  /// 失败时回滚到操作前状态，同时保留错误来源，页面层可据此弹出友好提示。
  ClientViewState _networkToggleFailureState(
    NetworkToggleOperation operation,
    String error, {
    String? notice,
  }) {
    return _settledNetworkUiState(
      operation.previousState,
      notice: notice,
      error: error,
      errorSource: ClientErrorSource.networkSwitch,
      clearVirtualIp: !operation.previousState.networkEnabled,
    );
  }

  void _setAndroidNetworkAuthorizationState({
    bool? checking,
    String? error,
    bool clearError = false,
    bool clearNetworkConfig = false,
  }) {
    _androidNetworkAuthorization.value =
        _androidNetworkAuthorization.value.copyWith(
      checking: checking,
      error: clearError ? null : error,
      clearNetworkConfig: clearNetworkConfig,
    );
  }

  Future<ClientViewState> _executeAndroidNetworkToggle(
    NetworkToggleOperation operation,
  ) async {
    if (operation.targetEnabled) {
      await prepareAndroidNetworkAuthorization();
      if (!_isCurrentNetworkToggle(operation)) {
        return _state.value;
      }
      final config = _requireAndroidGrantedNetworkConfig();
      await _plugin.androidStartVpn(config).timeout(_networkToggleTimeout);
      _rememberAndroidVpnConfigFingerprint(config);
    } else {
      await _plugin.androidStopVpn().timeout(_networkToggleTimeout);
    }
    return _androidNetworkToggleSuccessState(operation);
  }

  Future<ClientViewState> _executeIosNetworkToggle(
    NetworkToggleOperation operation,
  ) async {
    Object? result;
    AndroidVpnSessionConfig? config;
    if (operation.targetEnabled) {
      config = await _requireIosPacketTunnelConfig();
      result = await _plugin
          .iosStartPacketTunnel(config)
          .timeout(_networkToggleTimeout);
      _rememberIosPacketTunnelConfigFingerprint(config);
    } else {
      result =
          await _plugin.iosStopPacketTunnel().timeout(_networkToggleTimeout);
      _clearIosPacketTunnelConfigFingerprint();
    }
    final next = _stateOrThrowError(result);
    final current = _state.value;
    unawaited(_logIosSharedStoreDiagnostics(
      'bridge.ios.switch.sharedStore',
    ));
    unawaited(_logIosPacketTunnelStats('bridge.ios.switch.stats'));
    return _iosNetworkToggleSuccessState(
      operation,
      current: current,
      next: next,
      config: config,
    );
  }

  ClientViewState _mobileNetworkToggleSuccessState(
    ClientViewState current, {
    required bool networkEnabled,
    required bool clearVirtualIp,
    String? virtualIp,
    String? notice,
    String? error,
  }) {
    return _settledNetworkUiState(
      current,
      networkEnabled: networkEnabled,
      virtualIp: virtualIp,
      notice: notice,
      error: error,
      clearVirtualIp: clearVirtualIp,
    );
  }

  ClientViewState _androidNetworkToggleSuccessState(
    NetworkToggleOperation operation,
  ) {
    return _mobileNetworkToggleSuccessState(
      _state.value,
      networkEnabled: operation.targetEnabled,
      notice: operation.targetEnabled ? 'networkEnabled' : 'networkDisabled',
      virtualIp: operation.targetEnabled
          ? _androidNetworkAuthorization.value.networkConfig?.virtualIp
          : null,
      clearVirtualIp: !operation.targetEnabled,
    );
  }

  ClientViewState _iosNetworkToggleSuccessState(
    NetworkToggleOperation operation, {
    required ClientViewState current,
    required ClientViewState? next,
    required AndroidVpnSessionConfig? config,
  }) {
    return _mobileNetworkToggleSuccessState(
      current,
      networkEnabled: next?.networkEnabled ?? operation.targetEnabled,
      virtualIp: operation.targetEnabled
          ? (next?.virtualIp ?? config?.virtualIp)
          : null,
      notice: next?.notice,
      error: next?.error,
      clearVirtualIp: !(next?.networkEnabled ?? operation.targetEnabled),
    );
  }

  void _rememberAndroidVpnConfigFingerprint(AndroidVpnSessionConfig config) {
    _lastAndroidVpnConfigFingerprint = androidVpnConfigFingerprint(config);
  }

  AndroidVpnSessionConfig _requireAndroidGrantedNetworkConfig() {
    final authorization = _androidNetworkAuthorization.value;
    if (authorization.needsUserConsent) {
      throw StateError('Android 网络需要授权后才能启用');
    }
    final config = authorization.networkConfig;
    if (!authorization.granted || config == null) {
      throw StateError('Android 网络配置未就绪');
    }
    return config;
  }

  Future<AndroidVpnSessionConfig> _requireIosPacketTunnelConfig() async {
    final config = await _platformNetworkConfig();
    if (config == null) {
      throw StateError('iOS 网络配置未就绪');
    }
    return config;
  }

  void _rememberIosPacketTunnelConfigFingerprint(
      AndroidVpnSessionConfig config) {
    _lastIosPacketTunnelConfigFingerprint = androidVpnConfigFingerprint(config);
  }

  void _clearIosPacketTunnelConfigFingerprint() {
    _lastIosPacketTunnelConfigFingerprint = null;
  }

  /// 主动刷新当前 UI 状态。
  Future<void> _refreshState() async {
    await _applyStatefulControlPlaneRequest(
      embeddedMethod: 'refresh',
      embeddedUnavailableMessage: 'embedded refresh is not available',
      localMethod: 'refresh',
      fallbackEvent: 'bridge.refresh.fallback',
      onLocalError: (error) => throw error,
    );
  }

  /// 当前平台是否使用移动端原生内嵌控制面。
  bool get _usesNativeMobileControlPlane =>
      _useMobileControlPlaneOverride ?? (_isAndroid || _isIos);

  bool get _isNativeMobileTunnelPlatform => _isIos || _isAndroid;

  bool get _isDesktopHostPlatform => _isMacOS || _isWindows || _isLinux;

  /// 当前运行环境是否按 Android 处理。
  bool get _isAndroid =>
      _runtimePlatform == ClientBridgeRuntimePlatform.android ||
      (_runtimePlatform == ClientBridgeRuntimePlatform.host &&
          Platform.isAndroid);

  /// 当前运行环境是否按 iOS 处理。
  bool get _isIos =>
      _runtimePlatform == ClientBridgeRuntimePlatform.ios ||
      (_runtimePlatform == ClientBridgeRuntimePlatform.host && Platform.isIOS);

  /// 当前运行环境是否按 macOS 桌面处理。
  bool get _isMacOS =>
      _runtimePlatform == ClientBridgeRuntimePlatform.host && Platform.isMacOS;

  /// 当前运行环境是否按 Windows 桌面处理。
  bool get _isWindows =>
      _runtimePlatform == ClientBridgeRuntimePlatform.host &&
      Platform.isWindows;

  /// 当前运行环境是否按 Linux 桌面处理。
  bool get _isLinux =>
      _runtimePlatform == ClientBridgeRuntimePlatform.host && Platform.isLinux;

  /// 启动业务事件监听循环。
  ///
  /// 该循环负责接收登录、设备绑定、安全组/网络配置、网络开关结果、消息收发等
  /// 业务事件，是 Flutter UI 与后端状态保持一致的主路径。
  void _startBusinessEventWatchLoop() {
    _startWatchLoop(
      isAlreadyWatching: () => _watchingBusinessEvents,
      markWatching: () => _watchingBusinessEvents = true,
      isActive: () => _watchingBusinessEvents && !_closed,
      errorBackoff: const Duration(seconds: 2),
      errorEvent: 'bridge.businessEvent.watchError',
      runOnce: () async {
        if (_state.value.signedIn) {
          await _repairNativeMobileMqttIfNeeded(
            'bridge.businessEvent.mqttRepair',
          );
        }
        final json = await _watchBusinessEvents(_lastBusinessEventRevision);
        if (json == null) {
          if (_usesNativeMobileControlPlane) {
            await _pauseIfActive(
              const Duration(seconds: 1),
              () => _watchingBusinessEvents && !_closed,
            );
          }
          return;
        }
        final revision = json['revision'];
        if (revision is! int) {
          if (_usesNativeMobileControlPlane) {
            await _pauseIfActive(
              const Duration(seconds: 1),
              () => _watchingBusinessEvents && !_closed,
            );
          }
          return;
        }
        if (revision <= _lastBusinessEventRevision) {
          if (revision < _lastBusinessEventRevision) {
            _lastBusinessEventRevision = revision;
          }
          final next = await _stateAfterBusinessEvent(json);
          if (next != null && _staleBusinessSnapshotShouldUpdate(next)) {
            _setStateIfChanged(next);
          }
          if (_usesNativeMobileControlPlane) {
            await _pauseIfActive(
              const Duration(seconds: 1),
              () => _watchingBusinessEvents && !_closed,
            );
          }
          return;
        }
        _lastBusinessEventRevision = revision;
        final next = await _stateAfterBusinessEvent(json);
        if (next != null) {
          _setStateIfChanged(next);
        }
      },
    );
  }

  /// 判断旧 revision 快照是否仍值得合并到当前 UI。
  ///
  /// 某些服务重启或 revision 回退场景会带来旧快照，只合并明确变化的字段，
  /// 避免把当前用户操作中的状态覆盖掉。
  bool _staleBusinessSnapshotShouldUpdate(ClientViewState next) {
    final current = _state.value;
    if (next.signedIn != current.signedIn ||
        next.networkEnabled != current.networkEnabled ||
        next.syncing != current.syncing ||
        next.switchEnabled != current.switchEnabled) {
      return true;
    }
    if (next.userLabel != null && next.userLabel != current.userLabel) {
      return true;
    }
    if (next.deviceId != null && next.deviceId != current.deviceId) {
      return true;
    }
    if (next.virtualIp != null && next.virtualIp != current.virtualIp) {
      return true;
    }
    if (next.notice != null && next.notice != current.notice) {
      return true;
    }
    if (next.error != null && next.error != current.error) {
      return true;
    }
    return false;
  }

  void _applyPlatformNetworkEvent(
    AndroidNetworkEvent event, {
    required String runtimeLogEvent,
    required Future<void> Function(
      String event, {
      Map<String, Object?>? runtimeState,
    }) logRuntimeState,
  }) {
    final runtimeState = event.runtimeState;
    if (runtimeState != null) {
      unawaited(logRuntimeState(
        runtimeLogEvent,
        runtimeState: runtimeState,
      ));
      _setStateIfChanged(_platformNetworkEventState(event, runtimeState));
    }
    if (_platformNetworkEventShouldSettleToggle(event)) {
      _settleNetworkToggleFromEvent();
    }
  }

  ClientViewState _localServiceNotConnectedState() {
    return _settledNetworkUiState(
      _state.value,
      notice: 'localServiceNotConnected',
    );
  }

  ClientViewState _platformNetworkEventState(
    AndroidNetworkEvent event,
    Map<String, Object?> runtimeState,
  ) {
    final networkEnabled = runtimeState['networkEnabled'] == true;
    final isError = event.eventType == AndroidNetworkEventType.error;
    return _settledNetworkUiState(
      _state.value,
      networkEnabled: networkEnabled,
      virtualIp: runtimeState['virtualIp'] as String?,
      notice: event.eventType,
      error: isError ? event.message : null,
      errorSource: isError ? ClientErrorSource.networkSwitch : null,
      clearVirtualIp: !networkEnabled,
    );
  }

  ClientViewState _settledNetworkUiState(
    ClientViewState current, {
    bool? networkEnabled,
    String? virtualIp,
    String? notice,
    String? error,
    String? errorSource,
    bool clearVirtualIp = false,
  }) {
    return current.copyWith(
      networkEnabled: networkEnabled,
      virtualIp: virtualIp,
      syncing: false,
      clearSyncReason: true,
      switchEnabled: true,
      notice: notice,
      error: error,
      errorSource: errorSource,
      clearVirtualIp: clearVirtualIp,
    );
  }

  ClientViewState _pendingNetworkUiState(
    ClientViewState current, {
    bool? networkEnabled,
    String? syncReason,
    String? notice,
    String? error,
    bool clearVirtualIp = false,
  }) {
    return current.copyWith(
      networkEnabled: networkEnabled,
      syncing: true,
      syncReason: syncReason,
      switchEnabled: false,
      notice: notice,
      error: error,
      clearVirtualIp: clearVirtualIp,
    );
  }

  /// 平台网络事件是否应该结算当前网络开关操作。
  bool _platformNetworkEventShouldSettleToggle(AndroidNetworkEvent event) {
    return event.eventType == AndroidNetworkEventType.vpnStarted ||
        event.eventType == AndroidNetworkEventType.vpnStopped ||
        event.eventType == AndroidNetworkEventType.error;
  }

  /// 启动平台网络事件监听骨架。
  ///
  /// Android / iOS 共用超时、重试和事件落状态流程，仅保留平台 watch 方法、
  /// runtime 日志函数和附加事件副作用作为参数差异。
  void _startPlatformNetworkEventWatchLoop({
    required bool enabled,
    required bool Function() isAlreadyWatching,
    required void Function() markWatching,
    required bool Function() isActive,
    required String errorEvent,
    required Future<AndroidNetworkEvent?> Function() watchEvent,
    required String runtimeLogEvent,
    required Future<void> Function(
      String event, {
      Map<String, Object?>? runtimeState,
    }) logRuntimeState,
    void Function(AndroidNetworkEvent event)? onEvent,
  }) {
    if (!enabled) {
      return;
    }
    _startWatchLoop(
      isAlreadyWatching: isAlreadyWatching,
      markWatching: markWatching,
      isActive: isActive,
      errorBackoff: const Duration(seconds: 2),
      errorEvent: errorEvent,
      runOnce: () async {
        final event = await watchEvent().timeout(const Duration(seconds: 35));
        if (event == null) {
          return;
        }
        onEvent?.call(event);
        _applyPlatformNetworkEvent(
          event,
          runtimeLogEvent: runtimeLogEvent,
          logRuntimeState: logRuntimeState,
        );
      },
    );
  }

  /// 启动 Android VpnService 事件监听循环。
  void _startAndroidNetworkEventWatchLoop() {
    _startPlatformNetworkEventWatchLoop(
      enabled: _isAndroid,
      isAlreadyWatching: () => _watchingAndroidNetworkEvents,
      markWatching: () => _watchingAndroidNetworkEvents = true,
      isActive: () => _watchingAndroidNetworkEvents && !_closed,
      errorEvent: 'bridge.android.event.watchError',
      watchEvent: _plugin.androidWatchNetworkEvent,
      runtimeLogEvent: 'bridge.android.event.runtime',
      logRuntimeState: _logAndroidRuntimeState,
      onEvent: (event) {
        _androidNetworkAuthorization.value =
            _androidNetworkAuthorization.value.applyEvent(event);
      },
    );
  }

  /// 启动 iOS PacketTunnel 事件监听循环。
  void _startIosNetworkEventWatchLoop() {
    _startPlatformNetworkEventWatchLoop(
      enabled: _isIos,
      isAlreadyWatching: () => _watchingIosNetworkEvents,
      markWatching: () => _watchingIosNetworkEvents = true,
      isActive: () => _watchingIosNetworkEvents && !_closed,
      errorEvent: 'bridge.ios.event.watchError',
      watchEvent: _plugin.iosWatchNetworkEvent,
      runtimeLogEvent: 'bridge.ios.event.runtime',
      logRuntimeState: _logIosRuntimeState,
    );
  }

  /// 启动 Android 数据面运行态统计循环。
  void _startAndroidRuntimeStatsLoop() {
    if (!_isAndroid) {
      return;
    }
    _startPollingLoop(
      isAlreadyWatching: () => _watchingAndroidRuntimeStats,
      markWatching: () => _watchingAndroidRuntimeStats = true,
      isActive: () => _watchingAndroidRuntimeStats && !_closed,
      interval: const Duration(seconds: 5),
      errorBackoff: const Duration(seconds: 5),
      errorEvent: 'bridge.android.runtime.statsError',
      pollOnce: () async {
        if (_state.value.networkEnabled) {
          await _logAndroidRuntimeState('bridge.android.runtime.stats');
        }
      },
    );
  }

  /// 启动 iOS PacketTunnel 运行态统计循环。
  void _startIosPacketTunnelStatsLoop() {
    if (!_isIos) {
      return;
    }
    _startPollingLoop(
      isAlreadyWatching: () => _watchingIosPacketTunnelStats,
      markWatching: () => _watchingIosPacketTunnelStats = true,
      isActive: () => _watchingIosPacketTunnelStats && !_closed,
      interval: const Duration(seconds: 5),
      errorBackoff: const Duration(seconds: 5),
      errorEvent: 'bridge.ios.packetTunnel.statsError',
      pollOnce: () async {
        if (_state.value.networkEnabled) {
          await _logIosPacketTunnelStats('bridge.ios.packetTunnel.stats');
        }
      },
    );
  }

  /// 读取并记录 iOS PacketTunnel 统计，同时上报给本地/内嵌服务。
  Future<void> _logIosPacketTunnelStats(String event) async {
    if (!_isIos || !_state.value.networkEnabled) {
      return;
    }
    await _logIosSharedStoreDiagnostics('$event.sharedStore');
    final stats = await _plugin.iosPacketTunnelStats();
    if (stats == null || stats.isEmpty) {
      return;
    }
    ClientUiDiagnostics.unawaitedLog(
      event,
      state: _state.value,
      fields: iosPacketTunnelDiagnosticsFields(stats),
    );
    unawaited(_reportPlatformRuntimeState(
      platform: 'ios',
      runtimeState: {
        'adapterPresent': true,
        'networkEnabled': _state.value.networkEnabled,
        if (_state.value.virtualIp != null) 'virtualIp': _state.value.virtualIp,
      },
      traffic: iosPacketTunnelDiagnosticsFields(stats),
    ));
  }

  /// 读取 iOS App Group shared store 诊断信息。
  Future<void> _logIosSharedStoreDiagnostics(String event) async {
    if (!_isIos) {
      return;
    }
    try {
      final diagnostics = await _plugin.iosSharedStoreDiagnostics();
      if (diagnostics == null || diagnostics.isEmpty) {
        return;
      }
      ClientUiDiagnostics.unawaitedLog(
        event,
        state: _state.value,
        fields: diagnostics,
      );
    } on MissingPluginException {
      return;
    } on Object catch (error) {
      ClientUiDiagnostics.unawaitedLog(
        'bridge.ios.sharedStore.diagnosticsFailed',
        state: _state.value,
        fields: {'message': error.toString()},
      );
    }
  }

  /// 读取并记录 Android VpnService 运行态，同时上报给服务端诊断入口。
  Future<void> _logAndroidRuntimeState(
    String event, {
    Map<String, Object?>? runtimeState,
  }) async {
    if (!_isAndroid) {
      return;
    }
    await _logPlatformRuntimeState(
      event,
      platform: 'android',
      runtimeState:
          runtimeState ?? _jsonMap(await _plugin.androidRuntimeState()),
    );
  }

  /// 读取并记录 iOS PacketTunnel 原生运行态。
  Future<void> _logIosRuntimeState(
    String event, {
    Map<String, Object?>? runtimeState,
  }) async {
    if (!_isIos) {
      return;
    }
    await _logPlatformRuntimeState(
      event,
      platform: 'ios',
      runtimeState: runtimeState ?? _jsonMap(await _plugin.iosRuntimeState()),
    );
  }

  Future<void> _logPlatformRuntimeState(
    String event, {
    required String platform,
    required Map<String, Object?>? runtimeState,
  }) async {
    final state = runtimeState;
    if (state == null || state.isEmpty) {
      return;
    }
    final traffic = androidRuntimeDiagnosticsFields(state);
    ClientUiDiagnostics.unawaitedLog(
      event,
      state: _state.value,
      fields: traffic,
    );
    unawaited(_reportPlatformRuntimeState(
      platform: platform,
      runtimeState: state,
      traffic: traffic,
    ));
  }

  /// 把平台数据面运行态写回本地/内嵌服务，供 Web Console 和诊断接口查看。
  Future<void> _reportPlatformRuntimeState({
    required String platform,
    required Map<String, Object?> runtimeState,
    Map<String, Object?>? traffic,
    String? error,
  }) async {
    try {
      final payload = {
        'platform': platform,
        'runtimeState': runtimeState,
        if (traffic != null) 'traffic': traffic,
        if (error != null) 'error': error,
      };
      if (_usesNativeMobileControlPlane) {
        await _embeddedServiceRequest('ingestPlatformRuntimeState', payload);
      } else {
        await _localService.ingestPlatformRuntimeState(
          platform: platform,
          runtimeState: runtimeState,
          traffic: traffic,
          error: error,
        );
      }
      await _reportRuntimeToControlPlane(
        platform: platform,
        runtimeState: runtimeState,
        traffic: traffic,
      );
    } on Object catch (reportError) {
      ClientUiDiagnostics.unawaitedLog(
        'bridge.platformRuntimeState.reportFailed',
        state: _state.value,
        fields: {
          'platform': platform,
          'message': reportError.toString(),
        },
      );
    }
  }

  Future<void> _reportRuntimeToControlPlane({
    required String platform,
    required Map<String, Object?> runtimeState,
    Map<String, Object?>? traffic,
  }) async {
    final deviceId = _runtimeReportDeviceId(runtimeState);
    if (deviceId == null || deviceId.isEmpty) {
      return;
    }
    final base = _runtimeReportBaseFields(runtimeState, traffic);
    final reportedAtMs = base.reportedAtMs;
    final lastSeenAt = base.lastSeenAt;
    final rxBytesTotal = base.rxBytesTotal;
    final txBytesTotal = base.txBytesTotal;
    final networkEnabled = base.networkEnabled;
    final deviceVersion = runtimeState['deviceVersion'] is String
        ? (runtimeState['deviceVersion'] as String).trim()
        : '';
    final runtimePath = runtimeState['runtimePath'] is Map
        ? (runtimeState['runtimePath'] as Map).cast<String, Object?>()
        : const <String, Object?>{};
    final networkId = _firstNonEmptyString([
      runtimeState['networkId'],
      runtimePath['networkId'],
    ]);
    final natType = _firstNonEmptyString([
      runtimeState['natType'],
      runtimePath['natType'],
    ]);
    final activePath = _firstNonEmptyString([
      runtimeState['activePath'],
      runtimeState['pathType'],
      runtimePath['activePath'],
    ]);
    final relayTransport = _firstNonEmptyString([
      runtimeState['relayTransport'],
      runtimePath['relayTransport'],
    ]);
    final relayEndpoint = _firstNonEmptyString([
      runtimeState['relayEndpoint'],
      runtimeState['relayAddress'],
      runtimeState['endpoint'],
      runtimePath['relayEndpoint'],
      runtimePath['relayAddress'],
    ]);
    final derpNodeId = _firstNonEmptyString([
      runtimeState['derpNodeId'],
      runtimeState['relayEndpointId'],
      runtimeState['endpointId'],
      runtimePath['derpNodeId'],
      runtimePath['relayEndpointId'],
      runtimePath['endpointId'],
    ]);
    final peerNodeId = _firstNonEmptyString([
      runtimeState['peerNodeId'],
      runtimePath['peerNodeId'],
    ]);
    final ticketExpiresAt = _firstNonEmptyString([
      runtimeState['ticketExpiresAt'],
      runtimePath['ticketExpiresAt'],
    ]);
    final lastPathChange = _firstNonEmptyString([
      runtimeState['lastPathChange'],
      runtimePath['lastPathChange'],
    ]);
    final path = _runtimeReportPathFields(runtimeState, runtimePath);
    final pathObservedAt = path.pathObservedAt;
    final pathScore = path.pathScore;
    final observedRttMs = path.observedRttMs;
    final packetLossPpm = path.packetLossPpm;
    final relayMtu = path.relayMtu;
    final maxFramePayload = path.maxFramePayload;
    final ticketRenewDue = path.ticketRenewDue;
    final pathDowngrades = path.pathDowngrades;
    final pathUpgrades = path.pathUpgrades;
    final body = _runtimeReportBody(
      deviceId: deviceId,
      platform: platform,
      deviceVersion: deviceVersion,
      reportedAtMs: reportedAtMs,
      lastSeenAt: lastSeenAt,
      networkEnabled: networkEnabled,
      rxBytesTotal: rxBytesTotal,
      txBytesTotal: txBytesTotal,
      networkId: networkId,
      natType: natType,
      activePath: activePath,
      pathObservedAt: pathObservedAt,
      relayTransport: relayTransport,
      relayEndpoint: relayEndpoint,
      derpNodeId: derpNodeId,
      peerNodeId: peerNodeId,
      pathScore: pathScore,
      observedRttMs: observedRttMs,
      packetLossPpm: packetLossPpm,
      relayMtu: relayMtu,
      maxFramePayload: maxFramePayload,
      ticketExpiresAt: ticketExpiresAt,
      ticketRenewDue: ticketRenewDue,
      pathDowngrades: pathDowngrades,
      pathUpgrades: pathUpgrades,
      lastPathChange: lastPathChange,
    );

    try {
      if (_usesNativeMobileControlPlane) {
        await _embeddedServiceRequest('localReportDeviceRuntime', {
          'deviceId': deviceId,
          'body': body,
        });
      } else {
        await _localService.localReportDeviceRuntime(
          deviceId: deviceId,
          body: body,
        );
      }
    } on Object catch (error) {
      ClientUiDiagnostics.unawaitedLog(
        'bridge.platformRuntimeState.controlPlaneReportFailed',
        state: _state.value,
        fields: {
          'platform': platform,
          'deviceId': deviceId,
          'message': error.toString(),
        },
      );
    }
  }

  String? _runtimeReportDeviceId(Map<String, Object?> runtimeState) {
    final stateDeviceId = _state.value.deviceId?.trim();
    final testDeviceId = _testDeviceId.trim();
    final runtimeDeviceId = runtimeState['deviceId'] is String
        ? runtimeState['deviceId'] as String
        : null;
    final trimmedStateDeviceId =
        stateDeviceId?.isNotEmpty == true ? stateDeviceId : null;
    final trimmedRuntimeDeviceId = runtimeDeviceId?.trim().isNotEmpty == true
        ? runtimeDeviceId!.trim()
        : null;
    if (trimmedStateDeviceId != null) {
      return trimmedStateDeviceId;
    }
    if (trimmedRuntimeDeviceId != null) {
      return trimmedRuntimeDeviceId;
    }
    if (testDeviceId.isNotEmpty) {
      return testDeviceId;
    }
    return null;
  }

  _RuntimeReportBaseFields _runtimeReportBaseFields(
    Map<String, Object?> runtimeState,
    Map<String, Object?>? traffic,
  ) {
    final reportedAtMs = _intValue(runtimeState['reportedAtMs']) ??
        _intValue(traffic?['updatedAtMs']) ??
        DateTime.now().millisecondsSinceEpoch;
    return _RuntimeReportBaseFields(
      reportedAtMs: reportedAtMs,
      lastSeenAt:
          _intValue(runtimeState['lastSeenAt']) ?? (reportedAtMs ~/ 1000),
      rxBytesTotal: _intValue(runtimeState['rxBytesTotal']) ??
          _intValue(traffic?['bytesRead']) ??
          _state.value.trafficRxBytes,
      txBytesTotal: _intValue(runtimeState['txBytesTotal']) ??
          _intValue(traffic?['bytesWritten']) ??
          _state.value.trafficTxBytes,
      networkEnabled: _boolValue(runtimeState['networkEnabled']) ??
          _state.value.networkEnabled,
    );
  }

  _RuntimeReportPathFields _runtimeReportPathFields(
    Map<String, Object?> runtimeState,
    Map<String, Object?> runtimePath,
  ) {
    return _RuntimeReportPathFields(
      pathObservedAt: _intValue(runtimeState['pathObservedAt']) ??
          _intValue(runtimeState['observedAt']) ??
          _intValue(runtimePath['observedAt']) ??
          _intValue(runtimePath['pathObservedAt']),
      pathScore: _intValue(runtimeState['pathScore']) ??
          _intValue(runtimePath['pathScore']),
      observedRttMs: _intValue(runtimeState['observedRttMs']) ??
          _intValue(runtimeState['rttMs']) ??
          _intValue(runtimePath['observedRttMs']),
      packetLossPpm: _intValue(runtimeState['packetLossPpm']) ??
          _intValue(runtimePath['packetLossPpm']),
      relayMtu: _intValue(runtimeState['relayMtu']) ??
          _intValue(runtimePath['relayMtu']),
      maxFramePayload: _intValue(runtimeState['maxFramePayload']) ??
          _intValue(runtimePath['maxFramePayload']),
      ticketRenewDue: _boolValue(runtimeState['ticketRenewDue']) ??
          _boolValue(runtimePath['ticketRenewDue']),
      pathDowngrades: _intValue(runtimeState['pathDowngrades']) ??
          _intValue(runtimePath['pathDowngrades']),
      pathUpgrades: _intValue(runtimeState['pathUpgrades']) ??
          _intValue(runtimePath['pathUpgrades']),
    );
  }

  Map<String, Object?> _runtimeReportBody({
    required String deviceId,
    required String platform,
    required String deviceVersion,
    required int reportedAtMs,
    required int lastSeenAt,
    required bool networkEnabled,
    required int? rxBytesTotal,
    required int? txBytesTotal,
    required String? networkId,
    required String? natType,
    required String? activePath,
    required int? pathObservedAt,
    required String? relayTransport,
    required String? relayEndpoint,
    required String? derpNodeId,
    required String? peerNodeId,
    required int? pathScore,
    required int? observedRttMs,
    required int? packetLossPpm,
    required int? relayMtu,
    required int? maxFramePayload,
    required String? ticketExpiresAt,
    required bool? ticketRenewDue,
    required int? pathDowngrades,
    required int? pathUpgrades,
    required String? lastPathChange,
  }) {
    return <String, Object?>{
      'deviceId': deviceId,
      'reportedAtMs': reportedAtMs,
      'lastSeenAt': lastSeenAt,
      'status': networkEnabled ? 'active' : 'inactive',
      if (rxBytesTotal != null) 'rxBytesTotal': rxBytesTotal,
      if (txBytesTotal != null) 'txBytesTotal': txBytesTotal,
      if (deviceVersion.isNotEmpty) 'deviceVersion': deviceVersion,
      'platform': platform,
      if (networkId != null) 'networkId': networkId,
      if (natType != null) 'natType': natType,
      if (activePath != null) 'activePath': activePath,
      if (pathObservedAt != null) 'pathObservedAt': pathObservedAt,
      if (relayTransport != null) 'relayTransport': relayTransport,
      if (relayEndpoint != null) 'relayEndpoint': relayEndpoint,
      if (derpNodeId != null) 'derpNodeId': derpNodeId,
      if (peerNodeId != null) 'peerNodeId': peerNodeId,
      if (pathScore != null) 'pathScore': pathScore,
      if (observedRttMs != null) 'observedRttMs': observedRttMs,
      if (packetLossPpm != null) 'packetLossPpm': packetLossPpm,
      if (relayMtu != null) 'relayMtu': relayMtu,
      if (maxFramePayload != null) 'maxFramePayload': maxFramePayload,
      if (ticketExpiresAt != null) 'ticketExpiresAt': ticketExpiresAt,
      if (ticketRenewDue != null) 'ticketRenewDue': ticketRenewDue,
      if (pathDowngrades != null) 'pathDowngrades': pathDowngrades,
      if (pathUpgrades != null) 'pathUpgrades': pathUpgrades,
      if (lastPathChange != null) 'lastPathChange': lastPathChange,
    };
  }

  String? _firstNonEmptyString(List<Object?> values) {
    for (final value in values) {
      if (value is! String) {
        continue;
      }
      final text = value.trim();
      if (text.isNotEmpty) {
        return text;
      }
    }
    return null;
  }

  /// 将业务事件转换为下一份 UI 状态。
  ///
  /// 对可能影响网络配置的事件，会先刷新移动端 peer 配置；对网络开关完成/失败
  /// 等关键事件，会主动查询当前状态，减少只靠事件 payload 导致的状态漂移。
  Future<ClientViewState?> _stateAfterBusinessEvent(
    Map<String, Object?> event,
  ) async {
    final type = businessEventType(event);
    final businessDataMap = _eventPayloadMap(event, 'businessData');
    final snapshotMap = _eventPayloadMap(event, 'snapshot');
    ClientUiDiagnostics.unawaitedLog(
      'bridge.businessEvent.received',
      state: _state.value,
      fields: businessEventReceivedLogFields(
        type,
        businessDataMap: businessDataMap,
        snapshotMap: snapshotMap,
      ),
    );
    if (_nativeMobilePeerRefreshRequired(event)) {
      await _refreshNativeMobilePeersFromControlSync();
    }
    if (!businessEventRequiresStateQuery(
      type,
      networkToggleInFlight: _networkToggleInFlight,
    )) {
      return _reduceBusinessEvent(event);
    }
    final queriedState = await _queriedBusinessEventState(event, type: type);
    if (queriedState != null) {
      return queriedState;
    }
    return _reduceBusinessEvent(event);
  }

  Map<String, Object?>? _eventPayloadMap(
    Map<String, Object?> event,
    String key,
  ) {
    final payload = event[key];
    return payload is Map ? payload.cast<String, Object?>() : null;
  }

  Future<ClientViewState?> _queriedBusinessEventState(
    Map<String, Object?> event, {
    required String? type,
  }) async {
    try {
      final state = await _queryCurrentState();
      if (state != null) {
        ClientUiDiagnostics.unawaitedLog(
          'bridge.businessEvent.stateQueried',
          state: _state.value,
          fields: businessEventStateQueriedLogFields(type, state),
        );
        return _reduceBusinessEvent(event, queriedState: state);
      }
    } on Object catch (error) {
      ClientUiDiagnostics.unawaitedLog(
        'bridge.businessEvent.stateQueryFailed',
        state: _state.value,
        fields: {
          'businessType': type,
          'message': error.toString(),
        },
      );
    }
    return null;
  }

  /// 判断业务事件是否要求移动端刷新 peer/relay 配置。
  bool _nativeMobilePeerRefreshRequired(Map<String, Object?> event) {
    if (!_usesNativeMobileControlPlane ||
        _networkToggleInFlight ||
        !_state.value.networkEnabled) {
      return false;
    }
    final type = businessEventType(event);
    if (type != ClientBusinessEventType.controlSyncChanged &&
        type != ClientBusinessEventType.networkRuntimeChanged) {
      return false;
    }
    final data = event['businessData'];
    if (data is! Map) {
      return false;
    }
    final businessData = data.cast<String, Object?>();
    final reconfigureRequired = boolField(businessData, 'reconfigureRequired');
    if (!reconfigureRequired) {
      return false;
    }
    final eventKey = controlSyncEventKey(event, businessData);
    final messageType = stringField(businessData, 'messageType');
    if (_lastHandledControlSyncEventKey == eventKey &&
        _lastHandledControlSyncMessageType == messageType &&
        _lastHandledControlSyncReconfigureRequired == reconfigureRequired) {
      return false;
    }
    _lastHandledControlSyncEventKey = eventKey;
    _lastHandledControlSyncMessageType = messageType;
    _lastHandledControlSyncReconfigureRequired = reconfigureRequired;
    return true;
  }

  /// 从控制面重新拉取移动端数据面配置并热更新 VPN/PacketTunnel。
  ///
  /// Android/iOS 在网络启用期间收到安全组、设备、relay 或直连候选变化时，
  /// 需要重新下发配置，否则新规则不会进入数据面。
  Future<void> _refreshNativeMobilePeersFromControlSync() async {
    try {
      final config = await _platformNetworkConfig();
      if (config == null) {
        return;
      }
      final existingRelaySessions = _androidNetworkAuthorization
              .value.networkConfig?.relayDataPlane?.sessions.length ??
          0;
      final nextRelaySessions = config.relayDataPlane?.sessions.length ?? 0;
      if (_isAndroid && existingRelaySessions > 0 && nextRelaySessions == 0) {
        _logMobilePeersRefreshSkipped(
          reason: 'emptyRelaySessions',
          fields: {
            'existingRelaySessions': existingRelaySessions,
          },
        );
        return;
      }
      if (_isAndroid) {
        final refreshed = await _refreshAndroidMobilePeers(config);
        if (!refreshed) {
          return;
        }
      } else if (_isIos) {
        final refreshed = await _refreshIosMobilePeers(config);
        if (!refreshed) {
          return;
        }
      } else {
        return;
      }
      ClientUiDiagnostics.unawaitedLog(
        'bridge.mobile.peersRefreshed',
        state: _state.value,
        fields: {
          'virtualIp': config.virtualIp,
          'relaySessions': config.relayDataPlane?.sessions.length,
        },
      );
    } on Object catch (error) {
      ClientUiDiagnostics.unawaitedLog(
        'bridge.mobile.peersRefreshFailed',
        state: _state.value,
        fields: {'message': error.toString()},
      );
    }
  }

  Future<bool> _refreshAndroidMobilePeers(
    AndroidVpnSessionConfig config,
  ) async {
    final fingerprint = androidVpnConfigFingerprint(config);
    final runtimeState = await _plugin.androidRuntimeState();
    final runtimeRunning = runtimeState is Map &&
        runtimeState['networkEnabled'] == true &&
        runtimeState['adapterPresent'] == true;
    if (runtimeRunning && _lastAndroidVpnConfigFingerprint == fingerprint) {
      _logMobilePeersRefreshSkipped(reason: 'unchangedAndroidVpnConfig');
      return false;
    }
    await _plugin.androidStartVpn(config).timeout(_networkToggleTimeout);
    _rememberAndroidVpnConfigFingerprint(config);
    return true;
  }

  Future<bool> _refreshIosMobilePeers(
    AndroidVpnSessionConfig config,
  ) async {
    final fingerprint = androidVpnConfigFingerprint(config);
    final runtimeState = await _plugin.iosRuntimeState();
    final runtimeRunning = runtimeState is Map &&
        runtimeState['networkEnabled'] == true &&
        runtimeState['adapterPresent'] == true;
    if (runtimeRunning &&
        _lastIosPacketTunnelConfigFingerprint == fingerprint) {
      _logMobilePeersRefreshSkipped(reason: 'unchangedIosPacketTunnelConfig');
      return false;
    }
    await _plugin.iosStartPacketTunnel(config).timeout(_networkToggleTimeout);
    _rememberIosPacketTunnelConfigFingerprint(config);
    return true;
  }

  void _logMobilePeersRefreshSkipped({
    required String reason,
    Map<String, Object?> fields = const {},
  }) {
    ClientUiDiagnostics.unawaitedLog(
      'bridge.mobile.peersRefreshSkipped',
      state: _state.value,
      fields: {
        'reason': reason,
        ...fields,
      },
    );
  }

  /// 根据业务事件 payload 合并 UI 状态。
  ClientViewState? _reduceBusinessEvent(
    Map<String, Object?> event, {
    ClientViewState? queriedState,
  }) {
    final dataState = _eventPayloadState(event, 'businessData');
    final snapshotState = _eventPayloadState(event, 'snapshot');
    final type = businessEventType(event);
    if (type == ClientBusinessEventType.networkSwitchFinished ||
        type == ClientBusinessEventType.networkRuntimeChanged ||
        type == ClientBusinessEventType.networkSwitchFailed) {
      _settleNetworkToggleFromEvent();
    }

    return reduceBusinessEvent(
      _state.value,
      event,
      queriedState: queriedState,
      dataState: dataState,
      snapshotState: snapshotState,
    );
  }

  ClientViewState? _eventPayloadState(
    Map<String, Object?> event,
    String key,
  ) {
    final payload = _eventPayloadMap(event, key);
    return payload == null ? null : _stateFromResult(payload);
  }

  /// 长轮询读取下一条业务事件。
  Future<Map<String, Object?>?> _watchBusinessEvents(int lastRevision) async {
    if (_usesNativeMobileControlPlane) {
      return _embeddedServiceRequest(
        'localBusinessEventWatch',
        {
          'lastRevision': lastRevision,
          'timeoutMs': 30000,
        },
      );
    }
    try {
      return await _localService.localBusinessEventWatch(
        lastRevision: lastRevision,
      );
    } on Object catch (error) {
      ClientUiDiagnostics.unawaitedLog(
        'bridge.businessEvent.watchFallback',
        state: _state.value,
        fields: {'message': error.toString()},
      );
      rethrow;
    }
  }

  /// 查询当前服务状态并转换为 UI 状态。
  Future<ClientViewState?> _queryCurrentState() async {
    if (_usesNativeMobileControlPlane) {
      final embedded = await _embeddedServiceRequest('localState');
      ClientUiDiagnostics.unawaitedLog(
        'bridge.localState.embedded',
        state: _state.value,
        fields: {
          'notice': embedded?['notice'],
          'lastDownstreamSummary': embedded?['lastDownstreamSummary'],
          'hasBusinessState': embedded != null,
        },
      );
      return embedded == null ? null : _stateFromResult(embedded);
    }
    return _runLoggedLocalFallback(
      fallbackEvent: 'bridge.localState.fallback',
      run: () async => _stateFromResult(await _localService.localState()),
    );
  }

  /// 收到平台或业务事件后，结算当前网络开关操作。
  void _settleNetworkToggleFromEvent() {
    if (!_networkToggleInFlight && _networkToggleOperation == null) {
      return;
    }
    _networkToggleInFlight = false;
    _networkToggleOperation = null;
    _networkToggleEpoch++;
  }

  /// 仅在状态变化时写入 [ValueNotifier]，降低 Flutter 重建成本。
  void _setStateIfChanged(ClientViewState state) {
    if (_closed) {
      return;
    }
    final next = _localLogoutRequested
        ? ClientViewState.initial()
        : _preserveMobilePlatformNetworkState(state);
    if (_state.value == next) {
      return;
    }
    _state.value = next;
  }

  /// 移动端保护数据面状态，避免控制面短暂旧快照把已启用网络误置为关闭。
  ClientViewState _preserveMobilePlatformNetworkState(
      ClientViewState incoming) {
    if (!_shouldPreserveMobilePlatformNetworkState(incoming)) {
      return incoming;
    }
    return incoming.copyWith(
      networkEnabled: true,
      virtualIp: _state.value.virtualIp ?? incoming.virtualIp,
      clearVirtualIp: false,
    );
  }

  bool _shouldPreserveMobilePlatformNetworkState(ClientViewState incoming) {
    if (!_isNativeMobileTunnelPlatform ||
        !_state.value.networkEnabled ||
        incoming.networkEnabled) {
      return false;
    }
    if (!incoming.signedIn) {
      return false;
    }
    if (incoming.error != null && incoming.error!.trim().isNotEmpty) {
      return false;
    }
    if (_networkToggleOperation?.targetEnabled == false) {
      return false;
    }
    return true;
  }

  /// 调用桌面端本地服务，并记录请求/响应诊断。
  ///
  /// 这是桌面端业务状态和业务命令的唯一入口；不要在 Flutter 新增 biz HTTP。
  Future<Object?> _requestLocalService(String method,
      [Object? arguments]) async {
    ClientUiDiagnostics.unawaitedLog(
      'bridge.localService.request',
      state: _state.value,
      fields: {'method': method},
    );
    final response = await _localService.request(method, arguments: arguments);
    ClientUiDiagnostics.unawaitedLog(
      'bridge.localService.response',
      state: _state.value,
      fields: {
        'method': method,
        'bytes': _responseByteCount(response),
      },
    );
    return response;
  }

  /// 统一记录桌面本地 service 请求失败日志并保留原异常。
  Future<T> _runLoggedLocalFallback<T>({
    required String fallbackEvent,
    required Future<T> Function() run,
  }) async {
    try {
      return await run();
    } on Object catch (error) {
      ClientUiDiagnostics.unawaitedLog(
        fallbackEvent,
        state: _state.value,
        fields: {'message': error.toString()},
      );
      rethrow;
    }
  }

  /// 执行“控制面返回状态并应用到 UI”的双路径请求。
  ///
  /// 移动端走 embedded service，桌面端走本地 service；该 helper 只覆盖
  /// `start/refresh` 这类直接返回状态快照的简单路径。
  Future<ClientViewState?> _applyStatefulControlPlaneRequest({
    required String embeddedMethod,
    required String embeddedUnavailableMessage,
    required String localMethod,
    required String fallbackEvent,
    required void Function(Object error) onLocalError,
  }) async {
    if (_usesNativeMobileControlPlane) {
      final embedded = await _embeddedServiceRequest(embeddedMethod);
      if (embedded == null) {
        throw StateError(embeddedUnavailableMessage);
      }
      return _applyStateFromResult(embedded);
    }
    try {
      final result = await _requestLocalService(localMethod);
      return _applyStateFromResult(result);
    } on Object catch (error) {
      ClientUiDiagnostics.unawaitedLog(
        fallbackEvent,
        state: _state.value,
        fields: {'message': error.toString()},
      );
      onLocalError(error);
      return null;
    }
  }

  @visibleForTesting
  Future<Map<String, Object?>?> requestLocalApi(
    String method, [
    Object? arguments,
  ]) async {
    if (_usesNativeMobileControlPlane) {
      return _embeddedServiceRequest(method, arguments);
    }
    return _resultMap(await _requestLocalService(method, arguments));
  }

  /// 将本地/内嵌服务返回值转换为 UI 状态。
  ClientViewState? _stateFromResult(Object? result) {
    final json = _resultMap(result);
    if (json == null) {
      return null;
    }
    if (!json.containsKey('networkEnabled') && !json.containsKey('signedIn')) {
      return null;
    }
    return ClientViewState.fromJson(json);
  }

  ClientViewState? _applyStateFromResult(Object? result) {
    final state = _stateFromResult(result);
    if (state != null) {
      _setStateIfChanged(state);
    }
    return state;
  }

  void _throwStateErrorText(ClientViewState? state) {
    final error = _stateErrorText(state);
    if (error != null && error.isNotEmpty) {
      throw error;
    }
  }

  void _throwStateError(ClientViewState? state) {
    final error = _stateErrorText(state);
    if (error != null && error.isNotEmpty) {
      throw StateError(error);
    }
  }

  ClientViewState? _stateOrThrowError(Object? result) {
    final state = _stateFromResult(result);
    _throwStateError(state);
    return state;
  }

  /// 宽松解析 JSON map，兼容插件直接返回 Map 或本地服务返回 JSON 字符串。
  Map<String, Object?>? _jsonMap(Object? result) {
    if (result is Map) {
      return result.cast<String, Object?>();
    }
    return _resultMap(result);
  }

  Map<String, Object?>? _resultMap(Object? result) {
    return ClientCoreLocalService.jsonMapFromResult(result);
  }

  String? _stateErrorText(ClientViewState? state) {
    return state?.error?.trim();
  }

  int _responseByteCount(Object? response) {
    return response is String ? response.length : 0;
  }
}

final class _RuntimeReportBaseFields {
  const _RuntimeReportBaseFields({
    required this.reportedAtMs,
    required this.lastSeenAt,
    required this.rxBytesTotal,
    required this.txBytesTotal,
    required this.networkEnabled,
  });

  final int reportedAtMs;
  final int lastSeenAt;
  final int? rxBytesTotal;
  final int? txBytesTotal;
  final bool networkEnabled;
}

final class _RuntimeReportPathFields {
  const _RuntimeReportPathFields({
    required this.pathObservedAt,
    required this.pathScore,
    required this.observedRttMs,
    required this.packetLossPpm,
    required this.relayMtu,
    required this.maxFramePayload,
    required this.ticketRenewDue,
    required this.pathDowngrades,
    required this.pathUpgrades,
  });

  final int? pathObservedAt;
  final int? pathScore;
  final int? observedRttMs;
  final int? packetLossPpm;
  final int? relayMtu;
  final int? maxFramePayload;
  final bool? ticketRenewDue;
  final int? pathDowngrades;
  final int? pathUpgrades;
}
