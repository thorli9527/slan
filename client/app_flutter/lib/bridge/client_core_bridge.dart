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

  /// 使用 Opt 签发的授权 Key 激活当前设备。
  Future<void> activateDevice(String key);

  /// 检查 Android VPN 权限并准备网络配置。
  Future<void> prepareAndroidNetworkAuthorization();

  /// 派发 UI 命令。
  Future<void> dispatch(ClientCommand command);

  /// 查询本地控制通道状态。
  Future<ControlTransportStatus?> localControlStatus();

  /// 通知核心服务前台生命周期已恢复，以便立即重建失效连接。
  Future<void> notifyAppResumed();

  /// 停止后台监听和异步任务。
  Future<void> close() async {}
}

/// ClientBridgeRuntimePlatform 用于测试时显式指定 bridge 走 host/android/ios 分支。
@visibleForTesting
enum ClientBridgeRuntimePlatform { host, android, ios }

/// MethodChannelClientCoreBridge 负责协调 Flutter UI、本地服务、移动端原生插件
/// 以及业务事件 watch loop。
class MethodChannelClientCoreBridge implements ClientCoreBridge {
  MethodChannelClientCoreBridge({
    String? localServiceHost,
    @visibleForTesting bool? useMobileControlPlane,
    @visibleForTesting ClientBridgeRuntimePlatform runtimePlatform =
        ClientBridgeRuntimePlatform.host,
  })  : _plugin = ClientCorePlugin(),
        _localService = ClientCoreLocalService(host: localServiceHost),
        _useMobileControlPlaneOverride = useMobileControlPlane,
        _runtimePlatform = runtimePlatform,
        _state = ValueNotifier<ClientViewState>(ClientViewState.initial()),
        _androidNetworkAuthorization =
            ValueNotifier<AndroidNetworkAuthorizationState>(
          AndroidNetworkAuthorizationState.initial,
        );

  /// 原生插件入口，负责移动端 VPN 和 iOS PacketTunnel 等能力。
  final ClientCorePlugin _plugin;

  /// 桌面端本地 Rust 服务 JSON-line 客户端。
  final ClientCoreLocalService _localService;

  /// 测试用开关：强制移动端走内嵌服务或桌面本地服务。
  final bool? _useMobileControlPlaneOverride;

  /// 测试用平台枚举，生产环境默认根据 Dart `Platform` 判断。
  final ClientBridgeRuntimePlatform _runtimePlatform;

  /// 当前 UI 状态。
  final ValueNotifier<ClientViewState> _state;

  /// Android VPN 授权状态。
  final ValueNotifier<AndroidNetworkAuthorizationState>
      _androidNetworkAuthorization;

  /// 网络切换操作 epoch，用于丢弃过期异步结果。
  int _networkToggleEpoch = 0;

  /// 已处理的最后一个业务事件 revision。
  int _lastBusinessEventRevision = 0;

  /// Rust 事件流实例 ID；服务重启后变化，用于重置 revision 游标。
  String? _businessEventStreamId;

  /// 首次订阅从当前最新事件开始，避免应用启动时重放历史 UI 状态。
  bool _businessEventCursorInitialized = false;

  /// 当前网络开关操作上下文。
  NetworkToggleOperation? _networkToggleOperation;

  /// 移动端前台恢复任务，合并短时间内重复的 lifecycle 回调。
  Future<void>? _mobileResumeInFlight;

  /// 网络开关是否正在执行。操作上下文本身是唯一状态源。
  bool get _networkToggleInFlight => _networkToggleOperation != null;

  /// 当前桌面浏览器命令，用于合并连续点击。

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
  /// 移动端读取原生持久化设置，桌面端读取 Rust 统一配置文件。
  @override
  Future<String> serverBaseUrl() async {
    await _loadServerBaseUrl();
    return _effectiveControlBaseUrl;
  }

  /// 更新控制面 API 地址并写入对应平台的持久化配置。
  @override
  Future<void> updateServerBaseUrl(String serverBaseUrl) async {
    final normalized = _normalizeServerBaseUrl(serverBaseUrl);
    if (_usesNativeMobileControlPlane) {
      await _plugin.setMobileServerBaseUrl(normalized);
      _runtimeControlBaseUrl = normalized;
    } else {
      final response =
          await _localService.localUpdateServerApiSettings(normalized);
      final persisted = response?['serverApiBaseUrl'] as String?;
      _runtimeControlBaseUrl = _normalizeServerBaseUrl(
        persisted == null || persisted.trim().isEmpty ? normalized : persisted,
      );
    }
    ClientUiDiagnostics.unawaitedLog(
      'bridge.serverBaseUrl.updated',
      state: _state.value,
      fields: {'serverBaseUrl': normalized},
    );
  }

  @override
  Future<void> activateDevice(String key) async {
    final value = key.trim();
    if (value.isEmpty) {
      throw ArgumentError('授权 Key 不能为空');
    }
    final Object? result;
    if (_usesNativeMobileControlPlane) {
      result = await _embeddedServiceRequest(
        'localActivateDevice',
        {'key': value},
        true,
      );
      if (result == null) {
        throw StateError('embedded device activation is not available');
      }
    } else {
      result = await _requestLocalService('localActivateDevice', {
        'key': value,
      });
    }
    await _ensureDeviceThenConnectMqtt('bridge.deviceActivation.mqtt');
    _applyStateFromResult(result);
    _startBusinessEventWatchLoop();
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
    _setAndroidNetworkAuthorizationState(checking: true, error: null);
    try {
      final permissionStateResult = await _plugin.androidVpnPermissionState();
      final permissionState = ClientCoreLocalService.stringResult(
        permissionStateResult,
      );
      AndroidVpnConsentRequest? consentRequest;
      AndroidVpnSessionConfig? networkConfig;
      if (permissionState == AndroidVpnPermissionState.needsUserConsent) {
        consentRequest = await _plugin.androidRequestVpnPermission();
      }
      if (permissionState == AndroidVpnPermissionState.granted &&
          _state.value.activated) {
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
  /// 移动端内嵌服务或原生 VPN/PacketTunnel。
  @override
  Future<void> dispatch(ClientCommand command) async {
    ClientUiDiagnostics.unawaitedLog(
      'bridge.dispatch.begin',
      state: _state.value,
      fields: _commandLogFields(command),
    );
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
          'mqttNetworkEventTopics': status?.mqttNetworkEventTopics,
          'mqttNetworkEventSubscribed': status?.mqttNetworkEventSubscribed,
          'activeNetworkId': status?.activeNetworkId,
          'deviceId': status?.deviceId,
        },
      );
      if (_usesNativeMobileControlPlane &&
          status?.ready == true &&
          status?.mqttConnected != true) {
        unawaited(
          _repairNativeMobileMqttIfNeeded(
            'bridge.localControlStatus.mqttRepair',
          ),
        );
      }
      return status;
    } on Object {
      return null;
    }
  }

  @override
  Future<void> close() async {
    _closed = true;
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
    unawaited(
      Future<void>(() async {
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
      }),
    );
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
    unawaited(
      Future<void>(() async {
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
      }),
    );
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
    if (state?.activated == true) {
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
  /// 命令统一交给本地或内嵌服务处理。
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
      _applyStateFromResult(embedded);
      return;
    }
    try {
      final result = await _requestLocalService('dispatch', command.toJson());
      _applyStateFromResult(result);
      return;
    } on Object catch (error) {
      _logCommandFailure('bridge.controlPlane.fallback', command, error);
      rethrow;
    }
  }

  void _logCommandFailure(String event, ClientCommand command, Object error) {
    ClientUiDiagnostics.unawaitedLog(
      event,
      state: _state.value,
      fields: {..._commandLogFields(command), 'message': error.toString()},
    );
  }

  Map<String, Object?> _commandLogFields(ClientCommand command) {
    return {'command': command.type.name};
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
    final requestId = _localService.createRequestId();
    final embeddedArgs = _embeddedArguments(arguments, requestId);
    ClientUiDiagnostics.unawaitedLog(
      'bridge.embeddedService.request',
      state: _state.value,
      fields: {'method': method, 'requestId': requestId},
    );
    try {
      final response = await _plugin
          .embeddedServiceRequest(
            jsonEncode({'method': method, 'args': embeddedArgs}),
          )
          .timeout(
            _localService.responseTimeoutFor(method, embeddedArgs),
            onTimeout: () => throw PlatformException(
              code: 'embedded_service_timeout',
              message: 'SLAN embedded service request timed out: $method',
              details: {'method': method, 'requestId': requestId},
            ),
          );
      final error = response?['error'];
      if (error is String && error.trim().isNotEmpty) {
        throw PlatformException(code: 'embedded_service_error', message: error);
      }
      ClientUiDiagnostics.unawaitedLog(
        'bridge.embeddedService.response',
        state: _state.value,
        fields: {'method': method, 'requestId': requestId},
      );
      return response;
    } on Object catch (error) {
      ClientUiDiagnostics.unawaitedLog(
        'bridge.embeddedService.failed',
        state: _state.value,
        fields: {
          'method': method,
          'requestId': requestId,
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
  Object _embeddedArguments(Object? arguments, String requestId) {
    final controlBaseUrl = _effectiveControlBaseUrl;
    final testDeviceId = _testDeviceId.trim();
    if (controlBaseUrl.isEmpty) {
      if (arguments is Map) {
        return {
          ...arguments.cast<String, Object?>(),
          'requestId': arguments['requestId'] ?? requestId,
          if (testDeviceId.isNotEmpty) 'deviceIdOverride': testDeviceId,
        };
      }
      return {
        if (arguments != null) 'value': arguments,
        'requestId': requestId,
        if (testDeviceId.isNotEmpty) 'deviceIdOverride': testDeviceId,
      };
    }
    if (arguments is Map) {
      return {
        ...arguments.cast<String, Object?>(),
        'requestId': arguments['requestId'] ?? requestId,
        'controlBaseUrl': controlBaseUrl,
        if (testDeviceId.isNotEmpty) 'deviceIdOverride': testDeviceId,
      };
    }
    return {
      'value': arguments,
      'requestId': requestId,
      'controlBaseUrl': controlBaseUrl,
      if (testDeviceId.isNotEmpty) 'deviceIdOverride': testDeviceId,
    };
  }

  /// 加载运行期控制面地址。
  ///
  /// 移动端读取原生持久化地址，桌面端读取 Rust 统一配置。
  Future<void> _loadServerBaseUrl() async {
    if (_runtimeControlBaseUrl != null) {
      return;
    }
    var value = '';
    if (_usesNativeMobileControlPlane) {
      value = (await _plugin.mobileServerBaseUrl()) ?? '';
    } else {
      final settings = await _localService.localServerApiSettings();
      value = settings?['serverApiBaseUrl'] as String? ?? '';
    }
    _runtimeControlBaseUrl = _normalizeServerBaseUrl(
      value.isNotEmpty ? value : _defaultEmbeddedControlBaseUrl,
    );
  }

  /// 当前实际用于控制面请求的 API 地址。
  String get _effectiveControlBaseUrl =>
      _runtimeControlBaseUrl ??
      _normalizeServerBaseUrl(_defaultEmbeddedControlBaseUrl);

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
    final device = await _embeddedServiceRequest(
      'localEnsureDevice',
      null,
      true,
    );
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
      clearVirtualIpWhenDisabling: false,
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
        _setStateIfChanged(
          _networkToggleFailureState(operation, 'network switch timed out'),
        );
        ClientUiDiagnostics.unawaitedLog(
          'bridge.switch.eventTimeout',
          state: _state.value,
          fields: {'command': operation.command.name, 'epoch': operation.epoch},
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
            _setStateIfChanged(
              _networkToggleFailureState(
                operation,
                'local service not connected',
                notice: 'localServiceNotConnected',
              ),
            );
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
    final targetEnabled = command.type == ClientCommandType.enableNetwork;
    final operation = NetworkToggleOperation(
      epoch: epoch,
      command: command.type,
      method: targetEnabled ? enableMethod : disableMethod,
      targetEnabled: targetEnabled,
      previousState: previousState,
    );
    _networkToggleOperation = operation;
    _setStateIfChanged(
      _networkTogglePendingState(
        command: command,
        targetEnabled: targetEnabled,
        clearVirtualIpWhenDisabling: clearVirtualIpWhenDisabling,
      ),
    );
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
      clearVirtualIpWhenDisabling: false,
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
        fields: {'command': operation.command.name, 'epoch': operation.epoch},
      );
    } on Object catch (error) {
      if (!_isCurrentNetworkToggle(operation)) {
        return;
      }
      _finishNetworkToggle(operation);
      _setStateIfChanged(
        _networkToggleFailureState(operation, error.toString()),
      );
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
      clearVirtualIp: !operation.previousState.activated,
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
      result = await _plugin.iosStopPacketTunnel().timeout(
            _networkToggleTimeout,
          );
      _clearIosPacketTunnelConfigFingerprint();
    }
    final next = _stateOrThrowError(result);
    final current = _state.value;
    unawaited(_logIosSharedStoreDiagnostics('bridge.ios.switch.sharedStore'));
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
      clearVirtualIp: false,
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
      clearVirtualIp: false,
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
    AndroidVpnSessionConfig config,
  ) {
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

  @override
  Future<void> notifyAppResumed() async {
    if (_usesNativeMobileControlPlane) {
      final inFlight = _mobileResumeInFlight;
      if (inFlight != null) {
        return inFlight;
      }
      final operation = _recoverNativeMobileAfterResume();
      _mobileResumeInFlight = operation;
      try {
        await operation;
      } finally {
        if (identical(_mobileResumeInFlight, operation)) {
          _mobileResumeInFlight = null;
        }
      }
      return;
    }
    if (!_isDesktopHostPlatform) {
      return;
    }
    try {
      await _localService.localConnectivityChanged();
    } catch (error) {
      ClientUiDiagnostics.unawaitedLog(
        'bridge.connectivity.resume.failed',
        state: _state.value,
        fields: {'error': error.toString()},
      );
    }
  }

  Future<void> _recoverNativeMobileAfterResume() async {
    try {
      await _embeddedServiceRequest('localConnectivityChanged', null, true);
    } catch (error) {
      ClientUiDiagnostics.unawaitedLog(
        'bridge.mobile.resume.controlRecoveryFailed',
        state: _state.value,
        fields: {'error': error.toString()},
      );
    }
    if (!_state.value.networkEnabled) {
      return;
    }
    await _refreshNativeMobilePeersFromControlSync(forceReconnect: true);
  }

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
        if (_state.value.activated) {
          await _repairNativeMobileMqttIfNeeded(
            'bridge.businessEvent.mqttRepair',
          );
        }
        final followLatest = !_businessEventCursorInitialized;
        final json = await _watchBusinessEvents(
          _lastBusinessEventRevision,
          followLatest: followLatest,
        );
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
        _businessEventCursorInitialized = true;
        final responseStreamId = json['streamId'];
        final streamChanged = responseStreamId is String &&
            responseStreamId.isNotEmpty &&
            _businessEventStreamId != null &&
            responseStreamId != _businessEventStreamId;
        if (responseStreamId is String && responseStreamId.isNotEmpty) {
          _businessEventStreamId = responseStreamId;
        }
        if (json['streamReset'] == true ||
            streamChanged ||
            json['replayGap'] == true) {
          await _recoverBusinessEventStream(json, revision);
          return;
        }
        if (revision <= _lastBusinessEventRevision) {
          if (revision == _lastBusinessEventRevision) {
            final next = await _stateAfterBusinessEvent(json);
            if (next != null && _staleBusinessSnapshotShouldUpdate(next)) {
              _setStateIfChanged(next);
            }
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

  Future<void> _recoverBusinessEventStream(
    Map<String, Object?> event,
    int eventRevision,
  ) async {
    final latestRevision = event['latestRevision'];
    _lastBusinessEventRevision =
        latestRevision is int ? latestRevision : eventRevision;
    ClientUiDiagnostics.unawaitedLog(
      'bridge.businessEvent.streamRecovered',
      state: _state.value,
      fields: {
        'oldestAvailableRevision': event['oldestAvailableRevision'],
        'latestRevision': latestRevision,
        'streamId': event['streamId'],
        'streamReset': event['streamReset'],
        'replayGap': event['replayGap'],
      },
    );
    if (_usesNativeMobileControlPlane && _state.value.activated) {
      await _refreshNativeMobilePeersFromControlSync();
    }
    final snapshot = _eventPayloadState(event, 'snapshot');
    final next = snapshot ?? await _queryCurrentState();
    if (next != null) {
      _setStateIfChanged(next);
    }
  }

  /// 判断旧 revision 快照是否仍值得合并到当前 UI。
  ///
  /// 某些服务重启或 revision 回退场景会带来旧快照，只合并明确变化的字段，
  /// 避免把当前用户操作中的状态覆盖掉。
  bool _staleBusinessSnapshotShouldUpdate(ClientViewState next) {
    final current = _state.value;
    if (next.activated != current.activated ||
        next.networkEnabled != current.networkEnabled ||
        next.syncing != current.syncing ||
        next.switchEnabled != current.switchEnabled) {
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

  Future<void> _forwardPlatformNetworkEvent(
    AndroidNetworkEvent event, {
    required String runtimeLogEvent,
    required Future<void> Function(
      String event, {
      Map<String, Object?>? runtimeState,
      String? error,
    }) logRuntimeState,
  }) async {
    final isError = event.eventType == AndroidNetworkEventType.error;
    final runtimeState = event.runtimeState ??
        (isError
            ? <String, Object?>{
                'adapterPresent': _state.value.networkEnabled,
                'networkEnabled': _state.value.networkEnabled,
                if (_state.value.virtualIp != null)
                  'virtualIp': _state.value.virtualIp,
              }
            : null);
    await logRuntimeState(
      runtimeLogEvent,
      runtimeState: runtimeState,
      error: isError ? event.message : null,
    );
  }

  ClientViewState _localServiceNotConnectedState() {
    return _settledNetworkUiState(
      _state.value,
      notice: 'localServiceNotConnected',
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
      String? error,
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
        await _forwardPlatformNetworkEvent(
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
    unawaited(
      _reportPlatformRuntimeState(
        platform: 'ios',
        runtimeState: {
          'adapterPresent': true,
          'networkEnabled': _state.value.networkEnabled,
          if (_state.value.virtualIp != null)
            'virtualIp': _state.value.virtualIp,
        },
        traffic: iosPacketTunnelDiagnosticsFields(stats),
      ),
    );
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
    String? error,
  }) async {
    if (!_isAndroid) {
      return;
    }
    await _logPlatformRuntimeState(
      event,
      platform: 'android',
      runtimeState:
          runtimeState ?? _jsonMap(await _plugin.androidRuntimeState()),
      error: error,
    );
  }

  /// 读取并记录 iOS PacketTunnel 原生运行态。
  Future<void> _logIosRuntimeState(
    String event, {
    Map<String, Object?>? runtimeState,
    String? error,
  }) async {
    if (!_isIos) {
      return;
    }
    await _logPlatformRuntimeState(
      event,
      platform: 'ios',
      runtimeState: runtimeState ?? _jsonMap(await _plugin.iosRuntimeState()),
      error: error,
    );
  }

  Future<void> _logPlatformRuntimeState(
    String event, {
    required String platform,
    required Map<String, Object?>? runtimeState,
    String? error,
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
    await _reportPlatformRuntimeState(
      platform: platform,
      runtimeState: state,
      traffic: traffic,
      error: error,
    );
  }

  /// 把平台数据面运行态写回本地/内嵌服务，供控制面和诊断接口查看。
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
    } on Object catch (reportError) {
      ClientUiDiagnostics.unawaitedLog(
        'bridge.platformRuntimeState.reportFailed',
        state: _state.value,
        fields: {'platform': platform, 'message': reportError.toString()},
      );
    }
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
        fields: {'businessType': type, 'message': error.toString()},
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
  Future<void> _refreshNativeMobilePeersFromControlSync({
    bool forceReconnect = false,
  }) async {
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
          fields: {'existingRelaySessions': existingRelaySessions},
        );
        return;
      }
      if (_isAndroid) {
        final refreshed = await _refreshAndroidMobilePeers(
          config,
          forceReconnect: forceReconnect,
        );
        if (!refreshed) {
          return;
        }
      } else if (_isIos) {
        final refreshed = await _refreshIosMobilePeers(
          config,
          forceReconnect: forceReconnect,
        );
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
    AndroidVpnSessionConfig config, {
    bool forceReconnect = false,
  }) async {
    final fingerprint = androidVpnConfigFingerprint(config);
    final runtimeState = await _plugin.androidRuntimeState();
    final runtimeRunning = runtimeState is Map &&
        runtimeState['networkEnabled'] == true &&
        runtimeState['adapterPresent'] == true;
    if (!forceReconnect &&
        runtimeRunning &&
        _lastAndroidVpnConfigFingerprint == fingerprint) {
      _logMobilePeersRefreshSkipped(reason: 'unchangedAndroidVpnConfig');
      return false;
    }
    await _plugin.androidStartVpn(config).timeout(_networkToggleTimeout);
    _rememberAndroidVpnConfigFingerprint(config);
    return true;
  }

  Future<bool> _refreshIosMobilePeers(
    AndroidVpnSessionConfig config, {
    bool forceReconnect = false,
  }) async {
    final fingerprint = androidVpnConfigFingerprint(config);
    final runtimeState = await _plugin.iosRuntimeState();
    final runtimeRunning = runtimeState is Map &&
        runtimeState['networkEnabled'] == true &&
        runtimeState['adapterPresent'] == true;
    if (!forceReconnect &&
        runtimeRunning &&
        _lastIosPacketTunnelConfigFingerprint == fingerprint) {
      _logMobilePeersRefreshSkipped(reason: 'unchangedIosPacketTunnelConfig');
      return false;
    }
    if (forceReconnect && runtimeRunning) {
      await _plugin
          .iosRefreshPacketTunnel(config)
          .timeout(_networkToggleTimeout);
    } else {
      await _plugin.iosStartPacketTunnel(config).timeout(_networkToggleTimeout);
    }
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
      fields: {'reason': reason, ...fields},
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
    if (businessEventSettlesNetworkToggle(type)) {
      _settleNetworkToggleFromEvent();
    }

    return reduceBusinessEvent(
      _state.value,
      event,
      queriedState: queriedState,
      dataState: dataState,
      snapshotState: snapshotState,
      networkToggleInFlight: _networkToggleInFlight,
    );
  }

  ClientViewState? _eventPayloadState(Map<String, Object?> event, String key) {
    final payload = _eventPayloadMap(event, key);
    return payload == null ? null : _stateFromResult(payload);
  }

  /// 长轮询读取下一条业务事件。
  Future<Map<String, Object?>?> _watchBusinessEvents(
    int lastRevision, {
    required bool followLatest,
  }) async {
    if (_usesNativeMobileControlPlane) {
      return _embeddedServiceRequest('localBusinessEventWatch', {
        'lastRevision': lastRevision,
        if (followLatest) 'followLatest': true,
        if (_businessEventStreamId != null) 'streamId': _businessEventStreamId,
        'timeoutMs': 30000,
      });
    }
    try {
      return await _localService.localBusinessEventWatch(
        lastRevision: lastRevision,
        followLatest: followLatest,
        streamId: _businessEventStreamId,
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
    if (_networkToggleOperation == null) {
      return;
    }
    _networkToggleOperation = null;
    _networkToggleEpoch++;
  }

  /// 仅在状态变化时写入 [ValueNotifier]，降低 Flutter 重建成本。
  void _setStateIfChanged(ClientViewState state) {
    if (_closed) {
      return;
    }
    final next = _preserveMobilePlatformNetworkState(state);
    if (_state.value == next) {
      return;
    }
    _state.value = next;
  }

  /// 移动端保护数据面状态，避免控制面短暂旧快照把已启用网络误置为关闭。
  ClientViewState _preserveMobilePlatformNetworkState(
    ClientViewState incoming,
  ) {
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
    if (!incoming.activated) {
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
  Future<Object?> _requestLocalService(
    String method, [
    Object? arguments,
  ]) async {
    final requestId = _localService.createRequestId();
    ClientUiDiagnostics.unawaitedLog(
      'bridge.localService.request',
      state: _state.value,
      fields: {'method': method, 'requestId': requestId},
    );
    final response = await _localService.request(
      method,
      arguments: arguments,
      requestId: requestId,
    );
    ClientUiDiagnostics.unawaitedLog(
      'bridge.localService.response',
      state: _state.value,
      fields: {
        'method': method,
        'requestId': requestId,
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
    if (!json.containsKey('networkEnabled') && !json.containsKey('activated')) {
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
