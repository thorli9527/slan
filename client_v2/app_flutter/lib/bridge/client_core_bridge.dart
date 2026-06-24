// Flutter 客户端桥接层。
//
// 桌面端通过本地独立进程 client-core-service 工作；Android/iOS 通过原生插件
// 内嵌 client-core-service，并把 VPN/PacketTunnel 数据面状态回传给服务。
// 这个文件负责把这些平台差异收敛成 UI 可使用的 ClientCoreBridge。

import 'dart:async';
import 'dart:convert';
import 'dart:io';

import 'package:client_core_plugin/client_core_plugin.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter/services.dart';

import 'android_network_authorization.dart';
import 'client_commands.dart';
import 'client_core_local_service.dart';
import 'control_transport_status.dart';
import 'client_ui_diagnostics.dart';
import 'client_view_state.dart';

abstract final class ClientBusinessEventType {
  /// 用户 session 变化，例如登录、退出、设备绑定完成。
  static const sessionChanged = 'session.changed';

  /// 网络开关操作成功完成。
  static const networkSwitchFinished = 'network.switch.finished';

  /// 平台数据面运行态变化，例如 VPN 已启动、流量统计更新。
  static const networkRuntimeChanged = 'network.runtime.changed';

  /// 网络开关操作失败。
  static const networkSwitchFailed = 'network.switch.failed';

  /// 控制通道同步状态变化，例如 MQTT 下行消息处理完成。
  static const controlSyncChanged = 'control.sync.changed';

  /// 通用 UI 状态变化。
  static const stateChanged = 'state.changed';
}

/// _NetworkToggleOperation 记录一次网络开关操作的上下文，用于异步回调回来时
/// 判断结果是否仍属于当前最新操作。
class _NetworkToggleOperation {
  const _NetworkToggleOperation({
    required this.epoch,
    required this.command,
    required this.method,
    required this.targetEnabled,
    required this.previousState,
  });

  /// 操作序号。新操作会递增 epoch，旧异步回调不能覆盖新状态。
  final int epoch;

  /// 用户触发的原始命令。
  final ClientCommandType command;

  /// 本次操作对应的底层 service method，便于日志和诊断。
  final String method;

  /// 本次操作期望的网络状态。
  final bool targetEnabled;

  /// 操作开始前的 UI 状态，失败时用于回滚可见状态。
  final ClientViewState previousState;
}

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
}

/// ClientBridgeRuntimePlatform 用于测试时显式指定 bridge 走 host/android/ios 分支。
@visibleForTesting
enum ClientBridgeRuntimePlatform {
  host,
  android,
  ios,
}

/// 提取 iOS PacketTunnel 诊断字段。
///
/// 测试和日志只关心关键计数，避免把完整 stats 原样写入导致日志过大。
@visibleForTesting
Map<String, Object?> iosPacketTunnelDiagnosticsFields(
  Map<String, Object?> stats,
) {
  return {
    'relaySessionCount': stats['relaySessionCount'],
    'relayAttachedSessionCount': stats['relayAttachedSessionCount'],
    'relayAttachFailures': stats['relayAttachFailures'],
    'lastRelayAttachError': stats['lastRelayAttachError'],
    'packetsRead': stats['packetsRead'],
    'bytesRead': stats['bytesRead'],
    'bytesWritten': stats['bytesWritten'],
    'routedPackets': stats['routedPackets'],
    'unroutablePackets': stats['unroutablePackets'],
    'nonIpv4Packets': stats['nonIpv4Packets'],
    'relayFramesSent': stats['relayFramesSent'],
    'relayFramesReceived': stats['relayFramesReceived'],
    'relayPacketsWritten': stats['relayPacketsWritten'],
    'relayDetachSent': stats['relayDetachSent'],
    'relayNoPeerPackets': stats['relayNoPeerPackets'],
    'directUdpAttachedPeerCount': stats['directUdpAttachedPeerCount'],
    'directUdpReadyPeerCount': stats['directUdpReadyPeerCount'],
    'directUdpProbesSent': stats['directUdpProbesSent'],
    'directUdpProbesReceived': stats['directUdpProbesReceived'],
    'directUdpPongsSent': stats['directUdpPongsSent'],
    'directUdpPongsReceived': stats['directUdpPongsReceived'],
    'directUdpFramesSent': stats['directUdpFramesSent'],
    'directUdpFramesReceived': stats['directUdpFramesReceived'],
    'lastDestination': stats['lastDestination'],
    'lastRoute': stats['lastRoute'],
    'lastRoutedAtMs': stats['lastRoutedAtMs'],
    'updatedAtMs': stats['updatedAtMs'],
  };
}

/// 提取 Android VPN 运行态诊断字段。
///
/// Android runtime state 来自原生插件，字段较多；这里统一筛选出数据面
/// 启动、relay 连接、包计数和错误计数。
@visibleForTesting
Map<String, Object?> androidRuntimeDiagnosticsFields(
  Map<String, Object?> state,
) {
  return {
    'adapterPresent': state['adapterPresent'],
    'networkEnabled': state['networkEnabled'],
    'virtualIp': state['virtualIp'],
    'mtu': state['mtu'],
    'relayAddress': state['relayAddress'],
    'relaySessionCount': state['relaySessionCount'],
    'requestedRelaySessionCount': state['requestedRelaySessionCount'],
    'attachedRelaySessionCount': state['attachedRelaySessionCount'],
    'relayAttachFailures': state['relayAttachFailures'],
    'lastRelayAttachError': state['lastRelayAttachError'],
    'packetsRead': state['packetsRead'],
    'bytesRead': state['bytesRead'],
    'bytesWritten': state['bytesWritten'],
    'packetsTooLarge': state['packetsTooLarge'],
    'relayFramesSent': state['relayFramesSent'],
    'relayFramesReceived': state['relayFramesReceived'],
    'relayDetachSent': state['relayDetachSent'],
    'relayNoPeerPackets': state['relayNoPeerPackets'],
    'relayWriteFailures': state['relayWriteFailures'],
    'tunWriteFailures': state['tunWriteFailures'],
  };
}

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

  /// 原生插件入口，负责移动端 VPN、iOS PacketTunnel、外部浏览器等能力。
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

  /// 是否存在正在执行的网络开关操作。
  bool _networkToggleInFlight = false;

  /// 当前网络开关操作上下文。
  _NetworkToggleOperation? _networkToggleOperation;

  /// 用户是否刚刚主动退出，用于抑制旧 session 事件回写 UI。
  bool _localLogoutRequested = false;

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

  /// 移动端 MQTT 确保连接流程是否在运行。
  bool _mobileMqttEnsureRunning = false;

  /// 移动端 MQTT 确保连接流程的 in-flight future，用于去重。
  Future<void>? _mobileMqttEnsureInFlight;

  /// 最近一次移动端 MQTT 修复时间，避免高频重试。
  DateTime? _lastNativeMobileMqttRepairAt;

  /// 运行期覆盖的控制面地址，主要给移动端服务器设置使用。
  String? _runtimeControlBaseUrl;

  /// 移动端内嵌服务默认控制面地址，可由 dart-define 覆盖。
  static const _embeddedControlBaseUrl =
      String.fromEnvironment('SLAN_EMBEDDED_CONTROL_BASE_URL');

  /// 集成测试时强制使用的设备 ID。
  static const _testDeviceId = String.fromEnvironment('SLAN_TEST_DEVICE_ID');

  /// 默认生产控制面地址。
  static const _defaultControlBaseUrl = String.fromEnvironment(
      'SLAN_CONTROL_BASE_URL',
      defaultValue: 'http://47.245.40.231:28080');

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
    _androidNetworkAuthorization.value =
        _androidNetworkAuthorization.value.copyWith(
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
        networkConfig = await _platformNetworkConfig();
        final existingRelaySessions = _androidNetworkAuthorization
                .value.networkConfig?.relayDataPlane?.sessions.length ??
            0;
        final nextRelaySessions =
            networkConfig?.relayDataPlane?.sessions.length ?? 0;
        if (existingRelaySessions > 0 && nextRelaySessions == 0) {
          networkConfig = _androidNetworkAuthorization.value.networkConfig;
        }
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
      _androidNetworkAuthorization.value =
          _androidNetworkAuthorization.value.copyWith(
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
      fields: {'command': command.type.name},
    );
    if (command.type == ClientCommandType.openClientLogin) {
      _localLogoutRequested = false;
    }
    if (command.type == ClientCommandType.logout) {
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
    if (command.type == ClientCommandType.sendClientMessage) {
      await _sendClientMessageWithFallback(command.payload);
      return;
    }
    await _dispatchControlWithFallback(command);
  }

  /// 查询控制通道状态。
  ///
  /// 移动端发现 MQTT 未连接但凭证已就绪时，会异步触发一次修复流程。
  @override
  Future<ControlTransportStatus?> localControlStatus() async {
    try {
      final result = await _localControlStatusWithFallback();
      final json = ClientCoreLocalService.jsonMapFromResult(result);
      final status =
          json == null ? null : ControlTransportStatus.fromJson(json);
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
    if (_usesNativeMobileControlPlane) {
      final embedded = await _embeddedServiceRequest('start');
      if (embedded == null) {
        throw StateError('embedded start is not available');
      }
      final state = _stateFromResult(embedded);
      if (state != null) {
        _setStateIfChanged(state);
        if (state.signedIn) {
          unawaited(_ensureDeviceThenConnectMqtt('bridge.start.mqtt'));
        }
      }
      return;
    }
    try {
      final result = await _requestLocalService('start');
      final state = _stateFromResult(result);
      if (state != null) {
        _setStateIfChanged(state);
      }
      return;
    } on Object catch (error) {
      ClientUiDiagnostics.unawaitedLog(
        'bridge.start.fallback',
        state: _state.value,
        fields: {'message': error.toString()},
      );
      _setStateIfChanged(_state.value.copyWith(
        syncing: false,
        clearSyncReason: true,
        switchEnabled: true,
        notice: 'localServiceNotConnected',
      ));
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
          '${jsonEncode(_relayDebugSummary(relayDebug))}',
        );
      }
      return AndroidVpnSessionConfig.fromJson(embedded);
    }
    try {
      return await _localService.localPlatformNetworkConfig();
    } on Object catch (error) {
      ClientUiDiagnostics.unawaitedLog(
        'bridge.platformNetworkConfig.fallback',
        state: _state.value,
        fields: {'message': error.toString()},
      );
      rethrow;
    }
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
      final state = _stateFromResult(embedded);
      if (state != null) {
        _setStateIfChanged(state);
        if (command.type == ClientCommandType.loginWithPassword &&
            state.signedIn) {
          await _ensureDeviceThenConnectMqtt('bridge.login.mqtt');
        }
      }
      return;
    }
    if (_usesDesktopBrowserPlugin(command.type)) {
      try {
        final result = await _plugin.dispatch(command.toJson());
        final state = _stateFromResult(result);
        if (state != null) {
          _setStateIfChanged(state);
        }
        return;
      } on MissingPluginException catch (error) {
        ClientUiDiagnostics.unawaitedLog(
          'bridge.desktopBrowserPlugin.missing',
          state: _state.value,
          fields: {
            'command': command.type.name,
            'message': error.toString(),
          },
        );
      } on PlatformException catch (error) {
        ClientUiDiagnostics.unawaitedLog(
          'bridge.desktopBrowserPlugin.failed',
          state: _state.value,
          fields: {
            'command': command.type.name,
            'message': error.toString(),
          },
        );
      }
    }
    try {
      final result = await _requestLocalService('dispatch', command.toJson());
      final state = _stateFromResult(result);
      if (state != null) {
        _setStateIfChanged(state);
      }
      await _openDesktopBrowserForCommand(command, state);
      return;
    } on Object catch (error) {
      ClientUiDiagnostics.unawaitedLog(
        'bridge.controlPlane.fallback',
        state: _state.value,
        fields: {
          'command': command.type.name,
          'message': error.toString(),
        },
      );
      rethrow;
    }
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
    try {
      await _requestLocalService('localNetworkShutdown');
      return;
    } on Object catch (error) {
      ClientUiDiagnostics.unawaitedLog(
        'bridge.localNetworkShutdown.fallback',
        state: _state.value,
        fields: {'message': error.toString()},
      );
      rethrow;
    }
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
      final json = ClientCoreLocalService.jsonMapFromResult(response);
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
    await _openExternalUrl(url);
  }

  /// 用当前桌面平台的系统命令打开外部 URL。
  Future<void> _openExternalUrl(String url) async {
    if (_isMacOS) {
      await Process.start('open', [url], mode: ProcessStartMode.detached);
      return;
    }
    if (_isWindows) {
      await Process.start(
        'cmd',
        ['/c', 'start', '', url],
        mode: ProcessStartMode.detached,
      );
      return;
    }
    if (_isLinux) {
      await Process.start('xdg-open', [url], mode: ProcessStartMode.detached);
      return;
    }
    throw UnsupportedError('open browser is not supported on this platform');
  }

  /// 推导 Web Console 地址。
  ///
  /// 优先级：显式 `SLAN_WEB_CONSOLE_URL` > 从 `SLAN_CONTROL_BASE_URL`
  /// 推导 > 默认生产 Web 地址。
  String _resolveWebConsoleUrl() {
    final webConsoleUrl =
        Platform.environment['SLAN_WEB_CONSOLE_URL']?.trim() ?? '';
    if (webConsoleUrl.isNotEmpty) {
      return webConsoleUrl;
    }
    final controlBaseUrl =
        Platform.environment['SLAN_CONTROL_BASE_URL']?.trim() ?? '';
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

  /// 发送客户端到客户端消息。
  ///
  /// 桌面端走本地服务；移动端走内嵌服务。页面层的 Ping 工具也复用这条
  /// 通道，只是在 metadata/body 中带上 ping 标记。
  Future<void> _sendClientMessageWithFallback(
    Map<String, Object?>? payload,
  ) async {
    final targetDeviceId = (payload?['targetDeviceId'] as String?)?.trim();
    final body = (payload?['body'] as String?)?.trim();
    final metadata = payload?['metadata'];
    if (targetDeviceId == null ||
        targetDeviceId.isEmpty ||
        body == null ||
        body.isEmpty) {
      throw StateError('targetDeviceId and body are required');
    }
    if (_usesNativeMobileControlPlane) {
      final embedded = await _embeddedServiceRequest(
        'localSendClientMessage',
        {
          'targetDeviceId': targetDeviceId,
          'body': body,
          if (metadata is Map) 'metadata': metadata.cast<String, Object?>(),
        },
        true,
      );
      if (embedded == null) {
        throw StateError('embedded send client message is not available');
      }
      return;
    }
    try {
      await _localService.localSendClientMessage(
        targetDeviceId: targetDeviceId,
        body: body,
        metadata: metadata is Map ? metadata.cast<String, Object?>() : null,
      );
      return;
    } on Object catch (error) {
      ClientUiDiagnostics.unawaitedLog(
        'bridge.sendClientMessage.fallback',
        state: _state.value,
        fields: {'message': error.toString()},
      );
      rethrow;
    }
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
    try {
      final response = await _plugin.embeddedServiceRequest(jsonEncode({
        'method': method,
        'args': _embeddedArguments(arguments),
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
  /// 每个内嵌服务请求都需要知道控制面地址；设备 ID 在测试或初次注册场景
  /// 可能不在业务参数里，因此统一在这里附加，避免各调用点重复拼装。
  Object _embeddedArguments(Object? arguments) {
    final controlBaseUrl = _effectiveControlBaseUrl;
    final deviceId = _testDeviceId.trim().isNotEmpty
        ? _testDeviceId.trim()
        : _state.value.deviceId?.trim();
    if (controlBaseUrl.isEmpty) {
      if (arguments is Map) {
        return {
          ...arguments.cast<String, Object?>(),
          if (deviceId?.isNotEmpty == true) 'deviceId': deviceId,
        };
      }
      return {
        if (arguments != null) 'value': arguments,
        if (deviceId?.isNotEmpty == true) 'deviceId': deviceId,
      };
    }
    if (arguments is Map) {
      return {
        ...arguments.cast<String, Object?>(),
        'controlBaseUrl': controlBaseUrl,
        if (deviceId?.isNotEmpty == true) 'deviceId': deviceId,
      };
    }
    return {
      'value': arguments,
      'controlBaseUrl': controlBaseUrl,
      if (deviceId?.isNotEmpty == true) 'deviceId': deviceId,
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
  String get _defaultEmbeddedControlBaseUrl =>
      _embeddedControlBaseUrl.isNotEmpty
          ? _embeddedControlBaseUrl
          : _defaultControlBaseUrl;

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
      final inFlight = _mobileMqttEnsureInFlight;
      if (inFlight != null) {
        await inFlight;
      }
      return;
    }
    final inFlight = _mobileMqttEnsureInFlight;
    if (inFlight != null) {
      await inFlight;
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
    if (_networkToggleInFlight) {
      ClientUiDiagnostics.unawaitedLog(
        'bridge.switch.ignoredInFlight',
        state: _state.value,
        fields: {'command': command.type.name},
      );
      return;
    }
    final previousState = _state.value;
    final epoch = ++_networkToggleEpoch;
    _networkToggleInFlight = true;
    final targetEnabled = command.type == ClientCommandType.enableNetwork;
    final method =
        targetEnabled ? 'localNetworkActivate' : 'localNetworkDeactivate';
    final operation = _NetworkToggleOperation(
      epoch: epoch,
      command: command.type,
      method: method,
      targetEnabled: targetEnabled,
      previousState: previousState,
    );
    _networkToggleOperation = operation;
    _setStateIfChanged(_state.value.copyWith(
      networkEnabled: targetEnabled,
      syncing: true,
      syncReason: command.type.name,
      switchEnabled: false,
      error: null,
      notice: null,
      clearVirtualIp: command.type == ClientCommandType.disableNetwork,
    ));
    ClientUiDiagnostics.unawaitedLog(
      'bridge.switch.serviceApi',
      state: _state.value,
      fields: {'method': method, 'command': command.type.name},
    );
    ClientUiDiagnostics.unawaitedLog(
      'bridge.switch.pending',
      state: _state.value,
      fields: {
        'command': command.type.name,
        'epoch': epoch,
        'optimisticNetworkEnabled': targetEnabled,
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
          fields: {'command': command.type.name, 'epoch': epoch},
        );
      }),
    );
    unawaited(
      Future<void>(() async {
        try {
          final result =
              await _requestLocalService(method).timeout(_networkToggleTimeout);
          ClientUiDiagnostics.unawaitedLog(
            'bridge.switch.serviceResult',
            state: _state.value,
            fields: {
              'command': command.type.name,
              'epoch': epoch,
              'stale': epoch != _networkToggleEpoch,
              'hasResult': result != null,
            },
          );
          if (!_isCurrentNetworkToggle(operation)) {
            return;
          }
          final state = _stateFromResult(result);
          if (state != null &&
              state.error != null &&
              state.error!.trim().isNotEmpty) {
            _finishNetworkToggle(operation);
            _setStateIfChanged(_networkToggleFailureState(
              operation,
              state.error!,
            ));
            ClientUiDiagnostics.unawaitedLog(
              'bridge.switch.serviceReturnedError',
              state: _state.value,
              fields: {
                'command': command.type.name,
                'epoch': epoch,
                'message': state.error,
              },
            );
            return;
          }
          ClientUiDiagnostics.unawaitedLog(
            'bridge.switch.waitingBusinessEvent',
            state: _state.value,
            fields: {'command': command.type.name, 'epoch': epoch},
          );
        } on MissingPluginException {
          if (!_isCurrentNetworkToggle(operation)) {
            return;
          }
          _finishNetworkToggle(operation);
          _setStateIfChanged(_networkToggleFailureState(
            operation,
            'local service not connected',
            notice: 'localServiceNotConnected',
          ));
          ClientUiDiagnostics.unawaitedLog(
            'bridge.switch.missingPlugin',
            state: _state.value,
            fields: {'command': command.type.name, 'epoch': epoch},
          );
        } on TimeoutException catch (error) {
          if (!_isCurrentNetworkToggle(operation)) {
            return;
          }
          _finishNetworkToggle(operation);
          _setStateIfChanged(_networkToggleFailureState(
            operation,
            'network switch timed out',
          ));
          ClientUiDiagnostics.unawaitedLog(
            'bridge.switch.timeout',
            state: _state.value,
            fields: {
              'command': command.type.name,
              'epoch': epoch,
              'message': error.toString(),
            },
          );
        } on PlatformException catch (error) {
          if (!_isCurrentNetworkToggle(operation)) {
            return;
          }
          _finishNetworkToggle(operation);
          _setStateIfChanged(_networkToggleFailureState(
            operation,
            error.message ?? error.code,
          ));
          ClientUiDiagnostics.unawaitedLog(
            'bridge.switch.platformError',
            state: _state.value,
            fields: {
              'command': command.type.name,
              'epoch': epoch,
              'code': error.code,
              'message': error.message,
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
          ClientUiDiagnostics.unawaitedLog(
            'bridge.switch.error',
            state: _state.value,
            fields: {
              'command': command.type.name,
              'epoch': epoch,
              'message': error.toString(),
            },
          );
        }
      }),
    );
  }

  /// Android 网络开关流程。
  ///
  /// Android 需要先确认 VPN 授权和网络配置，再通过原生插件启动 VpnService。
  void _startAsyncAndroidNetworkToggle(ClientCommand command) {
    if (_networkToggleInFlight) {
      ClientUiDiagnostics.unawaitedLog(
        'bridge.android.switch.ignoredInFlight',
        state: _state.value,
        fields: {'command': command.type.name},
      );
      return;
    }
    final previousState = _state.value;
    final epoch = ++_networkToggleEpoch;
    _networkToggleInFlight = true;
    final targetEnabled = command.type == ClientCommandType.enableNetwork;
    final operation = _NetworkToggleOperation(
      epoch: epoch,
      command: command.type,
      method: targetEnabled ? 'androidStartVpn' : 'androidStopVpn',
      targetEnabled: targetEnabled,
      previousState: previousState,
    );
    _networkToggleOperation = operation;
    _setStateIfChanged(_state.value.copyWith(
      networkEnabled: targetEnabled,
      syncing: true,
      syncReason: command.type.name,
      switchEnabled: false,
      error: null,
      notice: null,
      clearVirtualIp: !targetEnabled,
    ));
    unawaited(
      Future<void>(() async {
        try {
          if (targetEnabled) {
            await prepareAndroidNetworkAuthorization();
            if (!_isCurrentNetworkToggle(operation)) {
              return;
            }
            final authorization = _androidNetworkAuthorization.value;
            if (authorization.needsUserConsent) {
              throw StateError('Android 网络需要授权后才能启用');
            }
            final config = authorization.networkConfig;
            if (!authorization.granted || config == null) {
              throw StateError('Android 网络配置未就绪');
            }
            await _plugin
                .androidStartVpn(config)
                .timeout(_networkToggleTimeout);
            _lastAndroidVpnConfigFingerprint =
                _androidVpnConfigFingerprint(config);
          } else {
            await _plugin.androidStopVpn().timeout(_networkToggleTimeout);
          }
          if (!_isCurrentNetworkToggle(operation)) {
            return;
          }
          _finishNetworkToggle(operation);
          _setStateIfChanged(_state.value.copyWith(
            networkEnabled: targetEnabled,
            syncing: false,
            clearSyncReason: true,
            switchEnabled: true,
            notice: targetEnabled ? 'networkEnabled' : 'networkDisabled',
            virtualIp: targetEnabled
                ? _androidNetworkAuthorization.value.networkConfig?.virtualIp
                : null,
            clearVirtualIp: !targetEnabled,
          ));
          ClientUiDiagnostics.unawaitedLog(
            'bridge.android.switch.finished',
            state: _state.value,
            fields: {'command': command.type.name, 'epoch': epoch},
          );
          unawaited(_logAndroidRuntimeState('bridge.android.switch.runtime'));
        } on Object catch (error) {
          if (!_isCurrentNetworkToggle(operation)) {
            return;
          }
          _finishNetworkToggle(operation);
          _setStateIfChanged(_networkToggleFailureState(
            operation,
            error.toString(),
          ));
          _androidNetworkAuthorization.value =
              _androidNetworkAuthorization.value.copyWith(
            checking: false,
            error: error.toString(),
          );
          ClientUiDiagnostics.unawaitedLog(
            'bridge.android.switch.failed',
            state: _state.value,
            fields: {
              'command': command.type.name,
              'epoch': epoch,
              'message': error.toString(),
            },
          );
        }
      }),
    );
  }

  /// iOS 网络开关流程。
  ///
  /// iOS 复用平台网络配置结构，但实际启动的是 PacketTunnel 扩展。
  void _startAsyncIosNetworkToggle(ClientCommand command) {
    if (_networkToggleInFlight) {
      ClientUiDiagnostics.unawaitedLog(
        'bridge.ios.switch.ignoredInFlight',
        state: _state.value,
        fields: {'command': command.type.name},
      );
      return;
    }
    final previousState = _state.value;
    final epoch = ++_networkToggleEpoch;
    _networkToggleInFlight = true;
    final targetEnabled = command.type == ClientCommandType.enableNetwork;
    final operation = _NetworkToggleOperation(
      epoch: epoch,
      command: command.type,
      method: targetEnabled ? 'iosEnableNetwork' : 'iosDisableNetwork',
      targetEnabled: targetEnabled,
      previousState: previousState,
    );
    _networkToggleOperation = operation;
    _setStateIfChanged(_state.value.copyWith(
      networkEnabled: targetEnabled,
      syncing: true,
      syncReason: command.type.name,
      switchEnabled: false,
      error: null,
      notice: null,
      clearVirtualIp: !targetEnabled,
    ));
    unawaited(
      Future<void>(() async {
        try {
          Object? result;
          AndroidVpnSessionConfig? config;
          if (targetEnabled) {
            config = await _platformNetworkConfig();
            if (config == null) {
              throw StateError('iOS 网络配置未就绪');
            }
            result = await _plugin
                .iosStartPacketTunnel(config)
                .timeout(_networkToggleTimeout);
            _lastIosPacketTunnelConfigFingerprint =
                _androidVpnConfigFingerprint(config);
          } else {
            result = await _plugin
                .iosStopPacketTunnel()
                .timeout(_networkToggleTimeout);
            _lastIosPacketTunnelConfigFingerprint = null;
          }
          if (!_isCurrentNetworkToggle(operation)) {
            return;
          }
          final next = _stateFromResult(result);
          _finishNetworkToggle(operation);
          if (next != null &&
              next.error != null &&
              next.error!.trim().isNotEmpty) {
            _setStateIfChanged(_networkToggleFailureState(
              operation,
              next.error!,
              notice: next.notice,
            ));
            return;
          }
          final current = _state.value;
          _setStateIfChanged(current.copyWith(
            networkEnabled: next?.networkEnabled ?? targetEnabled,
            virtualIp:
                targetEnabled ? (next?.virtualIp ?? config?.virtualIp) : null,
            syncing: false,
            clearSyncReason: true,
            switchEnabled: true,
            notice: next?.notice,
            error: next?.error,
            clearVirtualIp: !(next?.networkEnabled ?? targetEnabled),
          ));
          ClientUiDiagnostics.unawaitedLog(
            'bridge.ios.switch.finished',
            state: _state.value,
            fields: {'command': command.type.name, 'epoch': epoch},
          );
          unawaited(_logIosSharedStoreDiagnostics(
            'bridge.ios.switch.sharedStore',
          ));
          unawaited(_logIosPacketTunnelStats('bridge.ios.switch.stats'));
        } on Object catch (error) {
          if (!_isCurrentNetworkToggle(operation)) {
            return;
          }
          _finishNetworkToggle(operation);
          _setStateIfChanged(_networkToggleFailureState(
            operation,
            error.toString(),
          ));
          ClientUiDiagnostics.unawaitedLog(
            'bridge.ios.switch.failed',
            state: _state.value,
            fields: {
              'command': command.type.name,
              'epoch': epoch,
              'message': error.toString(),
            },
          );
        }
      }),
    );
  }

  /// 网络切换最长等待时间，覆盖服务调用和平台数据面启动。
  static const Duration _networkToggleTimeout = Duration(seconds: 45);

  /// 判断异步回调是否仍属于当前网络开关操作。
  bool _isCurrentNetworkToggle(_NetworkToggleOperation operation) {
    return _networkToggleOperation == operation &&
        operation.epoch == _networkToggleEpoch;
  }

  /// 结束当前网络开关操作，并让后续旧回调失效。
  void _finishNetworkToggle(_NetworkToggleOperation operation) {
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
    _NetworkToggleOperation operation,
    String error, {
    String? notice,
  }) {
    return operation.previousState.copyWith(
      syncing: false,
      clearSyncReason: true,
      switchEnabled: true,
      notice: notice,
      error: error,
      errorSource: ClientErrorSource.networkSwitch,
      clearVirtualIp: !operation.previousState.networkEnabled,
    );
  }

  /// 主动刷新当前 UI 状态。
  Future<void> _refreshState() async {
    if (_usesNativeMobileControlPlane) {
      final embedded = await _embeddedServiceRequest('refresh');
      if (embedded == null) {
        throw StateError('embedded refresh is not available');
      }
      final state = _stateFromResult(embedded);
      if (state != null) {
        _setStateIfChanged(state);
      }
      return;
    }
    try {
      final result = await _requestLocalService('refresh');
      final state = _stateFromResult(result);
      if (state != null) {
        _setStateIfChanged(state);
      }
      return;
    } on Object catch (error) {
      ClientUiDiagnostics.unawaitedLog(
        'bridge.refresh.fallback',
        state: _state.value,
        fields: {'message': error.toString()},
      );
      rethrow;
    }
  }

  /// 当前平台是否使用移动端原生内嵌控制面。
  bool get _usesNativeMobileControlPlane =>
      _useMobileControlPlaneOverride ?? (_isAndroid || _isIos);

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
    if (_watchingBusinessEvents) {
      return;
    }
    _watchingBusinessEvents = true;
    unawaited(
      Future<void>(() async {
        while (_watchingBusinessEvents) {
          try {
            if (_state.value.signedIn) {
              await _repairNativeMobileMqttIfNeeded(
                'bridge.businessEvent.mqttRepair',
              );
            }
            final json = await _watchBusinessEvents(_lastBusinessEventRevision);
            if (json == null) {
              if (_usesNativeMobileControlPlane) {
                await Future<void>.delayed(const Duration(seconds: 1));
              }
              continue;
            }
            final revision = json['revision'];
            if (revision is! int) {
              if (_usesNativeMobileControlPlane) {
                await Future<void>.delayed(const Duration(seconds: 1));
              }
              continue;
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
                await Future<void>.delayed(const Duration(seconds: 1));
              }
              continue;
            }
            _lastBusinessEventRevision = revision;
            final next = await _stateAfterBusinessEvent(json);
            if (next != null) {
              _setStateIfChanged(next);
            }
          } on Object catch (error) {
            ClientUiDiagnostics.unawaitedLog(
              'bridge.businessEvent.watchError',
              state: _state.value,
              fields: {'message': error.toString()},
            );
            await Future<void>.delayed(const Duration(seconds: 2));
          }
        }
      }),
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

  /// 启动 Android VpnService 事件监听循环。
  void _startAndroidNetworkEventWatchLoop() {
    if (!_isAndroid || _watchingAndroidNetworkEvents) {
      return;
    }
    _watchingAndroidNetworkEvents = true;
    unawaited(
      Future<void>(() async {
        while (_watchingAndroidNetworkEvents) {
          try {
            final event = await _plugin
                .androidWatchNetworkEvent()
                .timeout(const Duration(seconds: 35));
            if (event != null) {
              _androidNetworkAuthorization.value =
                  _androidNetworkAuthorization.value.applyEvent(event);
              final runtimeState = event.runtimeState;
              if (runtimeState != null) {
                unawaited(_logAndroidRuntimeState(
                  'bridge.android.event.runtime',
                  runtimeState: runtimeState,
                ));
                final networkEnabled = runtimeState['networkEnabled'] == true;
                _setStateIfChanged(_state.value.copyWith(
                  networkEnabled: networkEnabled,
                  virtualIp: runtimeState['virtualIp'] as String?,
                  syncing: false,
                  clearSyncReason: true,
                  switchEnabled: true,
                  notice: event.eventType,
                  error: event.eventType == AndroidNetworkEventType.error
                      ? event.message
                      : null,
                  errorSource: event.eventType == AndroidNetworkEventType.error
                      ? ClientErrorSource.networkSwitch
                      : null,
                  clearVirtualIp: !networkEnabled,
                ));
              }
              if (event.eventType == AndroidNetworkEventType.vpnStarted ||
                  event.eventType == AndroidNetworkEventType.vpnStopped ||
                  event.eventType == AndroidNetworkEventType.error) {
                _settleNetworkToggleFromEvent();
              }
            }
          } on MissingPluginException {
            await Future<void>.delayed(const Duration(seconds: 5));
          } on TimeoutException {
            // Watch methods may hold the request open; a timeout simply starts the next cycle.
          } on Object catch (error) {
            ClientUiDiagnostics.unawaitedLog(
              'bridge.android.event.watchError',
              state: _state.value,
              fields: {'message': error.toString()},
            );
            await Future<void>.delayed(const Duration(seconds: 2));
          }
        }
      }),
    );
  }

  /// 启动 iOS PacketTunnel 事件监听循环。
  void _startIosNetworkEventWatchLoop() {
    if (!_isIos || _watchingIosNetworkEvents) {
      return;
    }
    _watchingIosNetworkEvents = true;
    unawaited(
      Future<void>(() async {
        while (_watchingIosNetworkEvents) {
          try {
            final event = await _plugin
                .iosWatchNetworkEvent()
                .timeout(const Duration(seconds: 35));
            if (event != null) {
              final runtimeState = event.runtimeState;
              if (runtimeState != null) {
                unawaited(_logIosRuntimeState(
                  'bridge.ios.event.runtime',
                  runtimeState: runtimeState,
                ));
                final networkEnabled = runtimeState['networkEnabled'] == true;
                _setStateIfChanged(_state.value.copyWith(
                  networkEnabled: networkEnabled,
                  virtualIp: runtimeState['virtualIp'] as String?,
                  syncing: false,
                  clearSyncReason: true,
                  switchEnabled: true,
                  notice: event.eventType,
                  error: event.eventType == AndroidNetworkEventType.error
                      ? event.message
                      : null,
                  errorSource: event.eventType == AndroidNetworkEventType.error
                      ? ClientErrorSource.networkSwitch
                      : null,
                  clearVirtualIp: !networkEnabled,
                ));
              }
              if (event.eventType == AndroidNetworkEventType.vpnStarted ||
                  event.eventType == AndroidNetworkEventType.vpnStopped ||
                  event.eventType == AndroidNetworkEventType.error) {
                _settleNetworkToggleFromEvent();
              }
            }
          } on MissingPluginException {
            await Future<void>.delayed(const Duration(seconds: 5));
          } on TimeoutException {
            // Watch methods may hold the request open; a timeout simply starts the next cycle.
          } on Object catch (error) {
            ClientUiDiagnostics.unawaitedLog(
              'bridge.ios.event.watchError',
              state: _state.value,
              fields: {'message': error.toString()},
            );
            await Future<void>.delayed(const Duration(seconds: 2));
          }
        }
      }),
    );
  }

  /// 启动 Android 数据面运行态统计循环。
  void _startAndroidRuntimeStatsLoop() {
    if (!_isAndroid || _watchingAndroidRuntimeStats) {
      return;
    }
    _watchingAndroidRuntimeStats = true;
    unawaited(
      Future<void>(() async {
        while (_watchingAndroidRuntimeStats) {
          try {
            if (_state.value.networkEnabled) {
              await _logAndroidRuntimeState('bridge.android.runtime.stats');
            }
            await Future<void>.delayed(const Duration(seconds: 5));
          } on MissingPluginException {
            await Future<void>.delayed(const Duration(seconds: 5));
          } on Object catch (error) {
            ClientUiDiagnostics.unawaitedLog(
              'bridge.android.runtime.statsError',
              state: _state.value,
              fields: {'message': error.toString()},
            );
            await Future<void>.delayed(const Duration(seconds: 5));
          }
        }
      }),
    );
  }

  /// 启动 iOS PacketTunnel 运行态统计循环。
  void _startIosPacketTunnelStatsLoop() {
    if (!_isIos || _watchingIosPacketTunnelStats) {
      return;
    }
    _watchingIosPacketTunnelStats = true;
    unawaited(
      Future<void>(() async {
        while (_watchingIosPacketTunnelStats) {
          try {
            if (_state.value.networkEnabled) {
              await _logIosPacketTunnelStats('bridge.ios.packetTunnel.stats');
            }
            await Future<void>.delayed(const Duration(seconds: 5));
          } on MissingPluginException {
            await Future<void>.delayed(const Duration(seconds: 5));
          } on Object catch (error) {
            ClientUiDiagnostics.unawaitedLog(
              'bridge.ios.packetTunnel.statsError',
              state: _state.value,
              fields: {'message': error.toString()},
            );
            await Future<void>.delayed(const Duration(seconds: 5));
          }
        }
      }),
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
    final state = runtimeState ?? _jsonMap(await _plugin.androidRuntimeState());
    if (state == null || state.isEmpty) {
      return;
    }
    ClientUiDiagnostics.unawaitedLog(
      event,
      state: _state.value,
      fields: androidRuntimeDiagnosticsFields(state),
    );
    unawaited(_reportPlatformRuntimeState(
      platform: 'android',
      runtimeState: state,
      traffic: androidRuntimeDiagnosticsFields(state),
    ));
  }

  /// 读取并记录 iOS PacketTunnel 原生运行态。
  Future<void> _logIosRuntimeState(
    String event, {
    Map<String, Object?>? runtimeState,
  }) async {
    if (!_isIos) {
      return;
    }
    final state = runtimeState ?? _jsonMap(await _plugin.iosRuntimeState());
    if (state == null || state.isEmpty) {
      return;
    }
    ClientUiDiagnostics.unawaitedLog(
      event,
      state: _state.value,
      fields: androidRuntimeDiagnosticsFields(state),
    );
    unawaited(_reportPlatformRuntimeState(
      platform: 'ios',
      runtimeState: state,
      traffic: androidRuntimeDiagnosticsFields(state),
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
    final fallbackDeviceId = _testDeviceId.trim().isNotEmpty
        ? _testDeviceId.trim()
        : _state.value.deviceId?.trim();
    final runtimeDeviceId = runtimeState['deviceId'] is String
        ? runtimeState['deviceId'] as String
        : null;
    final deviceId = (runtimeDeviceId?.trim().isNotEmpty == true
            ? runtimeDeviceId!.trim()
            : fallbackDeviceId)
        ?.trim();
    if (deviceId == null || deviceId.isEmpty) {
      return;
    }

    final reportedAtMs =
        _intValue(runtimeState['reportedAtMs']) ??
        _intValue(traffic?['updatedAtMs']) ??
        DateTime.now().millisecondsSinceEpoch;
    final lastSeenAt =
        _intValue(runtimeState['lastSeenAt']) ?? (reportedAtMs ~/ 1000);
    final rxBytesTotal =
        _intValue(runtimeState['rxBytesTotal']) ??
        _intValue(traffic?['bytesRead']) ??
        _state.value.trafficRxBytes;
    final txBytesTotal =
        _intValue(runtimeState['txBytesTotal']) ??
        _intValue(traffic?['bytesWritten']) ??
        _state.value.trafficTxBytes;
    final networkEnabled =
        _boolValue(runtimeState['networkEnabled']) ?? _state.value.networkEnabled;
    final deviceVersion = runtimeState['deviceVersion'] is String
        ? (runtimeState['deviceVersion'] as String).trim()
        : '';
    final runtimePath =
        runtimeState['runtimePath'] is Map
            ? (runtimeState['runtimePath'] as Map).cast<String, Object?>()
            : const <String, Object?>{};
    String? stringField(List<Object?> values) {
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

    final networkId = stringField([
      runtimeState['networkId'],
      runtimePath['networkId'],
    ]);
    final natType = stringField([
      runtimeState['natType'],
      runtimePath['natType'],
    ]);
    final activePath = stringField([
      runtimeState['activePath'],
      runtimeState['pathType'],
      runtimePath['activePath'],
    ]);
    final relayTransport = stringField([
      runtimeState['relayTransport'],
      runtimePath['relayTransport'],
    ]);
    final relayEndpoint = stringField([
      runtimeState['relayEndpoint'],
      runtimeState['relayAddress'],
      runtimeState['endpoint'],
      runtimePath['relayEndpoint'],
      runtimePath['relayAddress'],
    ]);
    final derpNodeId = stringField([
      runtimeState['derpNodeId'],
      runtimeState['relayEndpointId'],
      runtimeState['endpointId'],
      runtimePath['derpNodeId'],
      runtimePath['relayEndpointId'],
      runtimePath['endpointId'],
    ]);
    final peerNodeId = stringField([
      runtimeState['peerNodeId'],
      runtimePath['peerNodeId'],
    ]);
    final ticketExpiresAt = stringField([
      runtimeState['ticketExpiresAt'],
      runtimePath['ticketExpiresAt'],
    ]);
    final lastPathChange = stringField([
      runtimeState['lastPathChange'],
      runtimePath['lastPathChange'],
    ]);
    final pathObservedAt =
        _intValue(runtimeState['pathObservedAt']) ??
        _intValue(runtimeState['observedAt']) ??
        _intValue(runtimePath['observedAt']) ??
        _intValue(runtimePath['pathObservedAt']);
    final pathScore =
        _intValue(runtimeState['pathScore']) ??
        _intValue(runtimePath['pathScore']);
    final observedRttMs =
        _intValue(runtimeState['observedRttMs']) ??
        _intValue(runtimeState['rttMs']) ??
        _intValue(runtimePath['observedRttMs']);
    final packetLossPpm =
        _intValue(runtimeState['packetLossPpm']) ??
        _intValue(runtimePath['packetLossPpm']);
    final relayMtu =
        _intValue(runtimeState['relayMtu']) ?? _intValue(runtimePath['relayMtu']);
    final maxFramePayload =
        _intValue(runtimeState['maxFramePayload']) ??
        _intValue(runtimePath['maxFramePayload']);
    final ticketRenewDue =
        _boolValue(runtimeState['ticketRenewDue']) ??
        _boolValue(runtimePath['ticketRenewDue']);
    final pathDowngrades =
        _intValue(runtimeState['pathDowngrades']) ??
        _intValue(runtimePath['pathDowngrades']);
    final pathUpgrades =
        _intValue(runtimeState['pathUpgrades']) ??
        _intValue(runtimePath['pathUpgrades']);

    final body = <String, Object?>{
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

    final uri = Uri.parse(
      '${_effectiveControlBaseUrl}/api/app/devices/$deviceId/runtime',
    );
    HttpClient? client;
    try {
      client = HttpClient();
      final request = await client.postUrl(uri);
      request.headers.contentType = ContentType.json;
      request.add(utf8.encode(jsonEncode(body)));
      final response = await request.close();
      if (response.statusCode >= 400) {
        final message = await response.transform(utf8.decoder).join();
        throw HttpException(
          'runtime report failed: ${response.statusCode} $message',
          uri: uri,
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
    } finally {
      client?.close(force: true);
    }
  }

  /// 将业务事件转换为下一份 UI 状态。
  ///
  /// 对可能影响网络配置的事件，会先刷新移动端 peer 配置；对网络开关完成/失败
  /// 等关键事件，会主动查询当前状态，减少只靠事件 payload 导致的状态漂移。
  Future<ClientViewState?> _stateAfterBusinessEvent(
    Map<String, Object?> event,
  ) async {
    final type = _businessEventType(event);
    if (_nativeMobilePeerRefreshRequired(event)) {
      await _refreshNativeMobilePeersFromControlSync();
    }
    if (!_businessEventRequiresStateQuery(type)) {
      return _reduceBusinessEvent(event);
    }
    try {
      final state = await _queryCurrentState();
      if (state != null) {
        ClientUiDiagnostics.unawaitedLog(
          'bridge.businessEvent.stateQueried',
          state: _state.value,
          fields: {'businessType': type},
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
    return _reduceBusinessEvent(event);
  }

  /// 判断业务事件是否要求移动端刷新 peer/relay 配置。
  bool _nativeMobilePeerRefreshRequired(Map<String, Object?> event) {
    if (!_usesNativeMobileControlPlane ||
        _networkToggleInFlight ||
        !_state.value.networkEnabled) {
      return false;
    }
    final type = _businessEventType(event);
    if (type != ClientBusinessEventType.controlSyncChanged &&
        type != ClientBusinessEventType.networkRuntimeChanged) {
      return false;
    }
    final data = event['businessData'];
    if (data is! Map) {
      return false;
    }
    final messageType = data['messageType']?.toString();
    return data['reconfigureRequired'] == true &&
        (messageType == 'device_network_enabled' ||
            messageType == 'device_network_disabled' ||
            messageType == 'network_config_changed');
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
        ClientUiDiagnostics.unawaitedLog(
          'bridge.mobile.peersRefreshSkipped',
          state: _state.value,
          fields: {
            'reason': 'emptyRelaySessions',
            'existingRelaySessions': existingRelaySessions,
          },
        );
        return;
      }
      if (_isAndroid) {
        final fingerprint = _androidVpnConfigFingerprint(config);
        final runtimeState = await _plugin.androidRuntimeState();
        final runtimeRunning = runtimeState is Map &&
            runtimeState['networkEnabled'] == true &&
            runtimeState['adapterPresent'] == true;
        if (runtimeRunning && _lastAndroidVpnConfigFingerprint == fingerprint) {
          ClientUiDiagnostics.unawaitedLog(
            'bridge.mobile.peersRefreshSkipped',
            state: _state.value,
            fields: {'reason': 'unchangedAndroidVpnConfig'},
          );
          return;
        }
        await _plugin.androidStartVpn(config).timeout(_networkToggleTimeout);
        _lastAndroidVpnConfigFingerprint = fingerprint;
      } else if (_isIos) {
        final fingerprint = _androidVpnConfigFingerprint(config);
        final runtimeState = await _plugin.iosRuntimeState();
        final runtimeRunning = runtimeState is Map &&
            runtimeState['networkEnabled'] == true &&
            runtimeState['adapterPresent'] == true;
        if (runtimeRunning &&
            _lastIosPacketTunnelConfigFingerprint == fingerprint) {
          ClientUiDiagnostics.unawaitedLog(
            'bridge.mobile.peersRefreshSkipped',
            state: _state.value,
            fields: {'reason': 'unchangedIosPacketTunnelConfig'},
          );
          return;
        }
        await _plugin
            .iosStartPacketTunnel(config)
            .timeout(_networkToggleTimeout);
        _lastIosPacketTunnelConfigFingerprint = fingerprint;
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

  /// 判断某类业务事件是否必须再查询当前状态。
  bool _businessEventRequiresStateQuery(String? type) {
    if (_usesNativeMobileControlPlane &&
        type == ClientBusinessEventType.sessionChanged) {
      return true;
    }
    if (type == ClientBusinessEventType.networkRuntimeChanged) {
      return _networkToggleInFlight;
    }
    return type == ClientBusinessEventType.networkSwitchFinished ||
        type == ClientBusinessEventType.networkSwitchFailed;
  }

  /// 根据业务事件 payload 合并 UI 状态。
  ClientViewState? _reduceBusinessEvent(
    Map<String, Object?> event, {
    ClientViewState? queriedState,
  }) {
    final type = _businessEventType(event);
    final businessData = event['businessData'];
    final snapshot = event['snapshot'];
    final dataState = businessData is Map
        ? _stateFromResult(businessData.cast<String, Object?>())
        : null;
    final snapshotState = snapshot is Map
        ? _stateFromResult(snapshot.cast<String, Object?>())
        : null;
    final incoming = queriedState ?? dataState ?? snapshotState;
    if (incoming == null) {
      return null;
    }

    switch (type) {
      case ClientBusinessEventType.sessionChanged:
        return _state.value.copyWith(
          signedIn: incoming.signedIn,
          userLabel: incoming.userLabel,
          deviceId: incoming.deviceId,
          networkEnabled: incoming.networkEnabled,
          virtualIp: incoming.virtualIp,
          syncing: false,
          clearSyncReason: true,
          switchEnabled: incoming.switchEnabled,
          notice: incoming.notice,
          error: incoming.error,
          clearVirtualIp: !incoming.networkEnabled,
        );
      case ClientBusinessEventType.networkSwitchFinished:
      case ClientBusinessEventType.networkRuntimeChanged:
        _settleNetworkToggleFromEvent();
        return incoming.copyWith(
          syncing: false,
          clearSyncReason: true,
          switchEnabled: true,
          clearVirtualIp: !incoming.networkEnabled,
        );
      case ClientBusinessEventType.networkSwitchFailed:
        _settleNetworkToggleFromEvent();
        final error = incoming.error ??
            dataState?.error ??
            snapshotState?.error ??
            'network switch failed';
        return _state.value.copyWith(
          networkEnabled: incoming.networkEnabled,
          virtualIp: incoming.virtualIp,
          syncing: false,
          clearSyncReason: true,
          switchEnabled: true,
          error: error,
          errorSource: ClientErrorSource.networkSwitch,
          clearVirtualIp: !incoming.networkEnabled,
        );
      case ClientBusinessEventType.controlSyncChanged:
      case ClientBusinessEventType.stateChanged:
      default:
        return _mergeBusinessState(incoming);
    }
  }

  /// 默认业务状态合并逻辑，覆盖服务端明确返回的字段。
  ClientViewState _mergeBusinessState(ClientViewState incoming) {
    return _state.value.copyWith(
      signedIn: incoming.signedIn,
      userLabel: incoming.userLabel,
      deviceId: incoming.deviceId,
      networkEnabled: incoming.networkEnabled,
      virtualIp: incoming.virtualIp,
      syncing: incoming.syncing,
      syncReason: incoming.syncReason,
      switchEnabled: incoming.switchEnabled,
      notice: incoming.notice,
      error: incoming.error,
      errorSource: incoming.errorSource,
      lastClientMessageId: incoming.lastClientMessageId,
      lastClientMessageFromDeviceId: incoming.lastClientMessageFromDeviceId,
      lastClientMessageBody: incoming.lastClientMessageBody,
      trafficTxBytes: incoming.trafficTxBytes,
      trafficRxBytes: incoming.trafficRxBytes,
      trafficTxBytesPerMinute: incoming.trafficTxBytesPerMinute,
      trafficRxBytesPerMinute: incoming.trafficRxBytesPerMinute,
      trafficUpdatedAtMs: incoming.trafficUpdatedAtMs,
      clearSyncReason: incoming.syncReason == null,
      clearVirtualIp: !incoming.networkEnabled,
    );
  }

  /// 兼容新旧事件字段名，提取业务事件类型。
  String? _businessEventType(Map<String, Object?> event) {
    return event['businessType'] as String? ?? event['type'] as String?;
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
      return embedded == null ? null : _stateFromResult(embedded);
    }
    try {
      return _stateFromResult(await _localService.localState());
    } on Object catch (error) {
      ClientUiDiagnostics.unawaitedLog(
        'bridge.localState.fallback',
        state: _state.value,
        fields: {'message': error.toString()},
      );
      rethrow;
    }
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
    final nativeMobileTunnel = _isIos || _isAndroid;
    if (!nativeMobileTunnel ||
        !_state.value.networkEnabled ||
        incoming.networkEnabled) {
      return incoming;
    }
    if (!incoming.signedIn) {
      return incoming;
    }
    if (incoming.error != null && incoming.error!.trim().isNotEmpty) {
      return incoming;
    }
    if (_networkToggleOperation?.targetEnabled == false) {
      return incoming;
    }
    return incoming.copyWith(
      networkEnabled: true,
      virtualIp: _state.value.virtualIp ?? incoming.virtualIp,
      clearVirtualIp: false,
    );
  }

  /// 调用桌面端本地服务，并记录请求/响应诊断。
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
        'bytes': response is String ? response.length : 0,
      },
    );
    return response;
  }

  /// 将本地/内嵌服务返回值转换为 UI 状态。
  ClientViewState? _stateFromResult(Object? result) {
    final json = ClientCoreLocalService.jsonMapFromResult(result);
    if (json == null) {
      return null;
    }
    if (!json.containsKey('networkEnabled') && !json.containsKey('signedIn')) {
      return null;
    }
    return ClientViewState.fromJson(json);
  }

  /// 宽松解析 JSON map，兼容插件直接返回 Map 或本地服务返回 JSON 字符串。
  Map<String, Object?>? _jsonMap(Object? result) {
    if (result is Map) {
      return result.cast<String, Object?>();
    }
    return ClientCoreLocalService.jsonMapFromResult(result);
  }
}

/// 压缩 relay 调试信息，避免日志写入完整大对象。
Map<String, Object?> _relayDebugSummary(Object? relayDebug) {
  if (relayDebug is! Map) {
    return {'present': relayDebug != null};
  }
  final requested = relayDebug['requestedRelaySessionCount'] ??
      relayDebug['requestedSessionCount'] ??
      relayDebug['relaySessionCount'];
  final attached = relayDebug['attachedRelaySessionCount'] ??
      relayDebug['attachedSessionCount'];
  return {
    'present': true,
    if (relayDebug.containsKey('enabled')) 'enabled': relayDebug['enabled'],
    if (relayDebug.containsKey('relayAddress'))
      'relayAddress': relayDebug['relayAddress'],
    if (requested != null) 'requestedRelaySessionCount': requested,
    if (attached != null) 'attachedRelaySessionCount': attached,
    if (relayDebug.containsKey('lastRelayAttachError'))
      'lastRelayAttachError': relayDebug['lastRelayAttachError'],
  };
}

/// 生成平台网络配置指纹，用于判断是否需要重复启动数据面。
String _androidVpnConfigFingerprint(AndroidVpnSessionConfig config) {
  return jsonEncode(config.toJson());
}
