// Mobile control-plane state is routed through embedded client-core-service.

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
  static const sessionChanged = 'session.changed';
  static const networkSwitchFinished = 'network.switch.finished';
  static const networkRuntimeChanged = 'network.runtime.changed';
  static const networkSwitchFailed = 'network.switch.failed';
  static const controlSyncChanged = 'control.sync.changed';
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

  final int epoch;
  final ClientCommandType command;
  final String method;
  final bool targetEnabled;
  final ClientViewState previousState;
}

/// ClientCoreBridge 是 UI 层使用的客户端核心门面。
///
/// host 平台通过本地 client-core-service HTTP API 工作；移动端通过原生插件内嵌
/// service 和平台 VPN/PacketTunnel 能力工作。
abstract interface class ClientCoreBridge {
  ValueListenable<ClientViewState> get state;
  ValueListenable<AndroidNetworkAuthorizationState>
      get androidNetworkAuthorization;

  Future<void> start();
  Future<String> serverBaseUrl();
  Future<void> updateServerBaseUrl(String serverBaseUrl);
  Future<void> prepareAndroidNetworkAuthorization();
  Future<void> dispatch(ClientCommand command);
  Future<ControlTransportStatus?> localControlStatus();
}

@visibleForTesting

/// ClientBridgeRuntimePlatform 用于测试时显式指定 bridge 走 host/android/ios 分支。
enum ClientBridgeRuntimePlatform {
  host,
  android,
  ios,
}

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

  final ClientCorePlugin _plugin;
  final ClientCoreLocalService _localService;
  final bool? _useMobileControlPlaneOverride;
  final ClientBridgeRuntimePlatform _runtimePlatform;
  final ValueNotifier<ClientViewState> _state;
  final ValueNotifier<AndroidNetworkAuthorizationState>
      _androidNetworkAuthorization;
  int _networkToggleEpoch = 0;
  int _lastBusinessEventRevision = 0;
  bool _networkToggleInFlight = false;
  _NetworkToggleOperation? _networkToggleOperation;
  bool _localLogoutRequested = false;
  bool _watchingBusinessEvents = false;
  bool _watchingAndroidNetworkEvents = false;
  bool _watchingIosNetworkEvents = false;
  bool _watchingAndroidRuntimeStats = false;
  bool _watchingIosPacketTunnelStats = false;
  bool _repairingNativeMobileMqtt = false;
  String? _lastAndroidVpnConfigFingerprint;
  String? _lastIosPacketTunnelConfigFingerprint;
  bool _mobileMqttEnsureRunning = false;
  Future<void>? _mobileMqttEnsureInFlight;
  DateTime? _lastNativeMobileMqttRepairAt;
  String? _runtimeControlBaseUrl;

  static const _embeddedControlBaseUrl =
      String.fromEnvironment('SLAN_EMBEDDED_CONTROL_BASE_URL');
  static const _testDeviceId = String.fromEnvironment('SLAN_TEST_DEVICE_ID');
  static const _defaultControlBaseUrl = String.fromEnvironment(
      'SLAN_CONTROL_BASE_URL',
      defaultValue: 'http://47.245.40.231:28080');

  @override
  ValueListenable<ClientViewState> get state => _state;

  @override
  ValueListenable<AndroidNetworkAuthorizationState>
      get androidNetworkAuthorization => _androidNetworkAuthorization;

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

  @override
  Future<String> serverBaseUrl() async {
    await _loadServerBaseUrl();
    return _effectiveControlBaseUrl;
  }

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

  bool _isNetworkToggle(ClientCommandType type) {
    return type == ClientCommandType.enableNetwork ||
        type == ClientCommandType.disableNetwork;
  }

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

  String _usableClientDeviceId(String? deviceId) {
    return deviceId?.trim() ?? '';
  }

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

  String get _effectiveControlBaseUrl =>
      _runtimeControlBaseUrl ??
      _normalizeServerBaseUrl(_defaultEmbeddedControlBaseUrl);

  String get _defaultEmbeddedControlBaseUrl =>
      _embeddedControlBaseUrl.isNotEmpty
          ? _embeddedControlBaseUrl
          : _defaultControlBaseUrl;

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

  void _clearNetworkToggle() {
    _networkToggleEpoch++;
    _networkToggleInFlight = false;
    _networkToggleOperation = null;
  }

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

  static const Duration _networkToggleTimeout = Duration(seconds: 45);

  bool _isCurrentNetworkToggle(_NetworkToggleOperation operation) {
    return _networkToggleOperation == operation &&
        operation.epoch == _networkToggleEpoch;
  }

  void _finishNetworkToggle(_NetworkToggleOperation operation) {
    if (!_isCurrentNetworkToggle(operation)) {
      return;
    }
    _networkToggleInFlight = false;
    _networkToggleOperation = null;
    _networkToggleEpoch++;
  }

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

  bool get _usesNativeMobileControlPlane =>
      _useMobileControlPlaneOverride ?? (_isAndroid || _isIos);

  bool get _isAndroid =>
      _runtimePlatform == ClientBridgeRuntimePlatform.android ||
      (_runtimePlatform == ClientBridgeRuntimePlatform.host &&
          Platform.isAndroid);

  bool get _isIos =>
      _runtimePlatform == ClientBridgeRuntimePlatform.ios ||
      (_runtimePlatform == ClientBridgeRuntimePlatform.host && Platform.isIOS);

  bool get _isMacOS =>
      _runtimePlatform == ClientBridgeRuntimePlatform.host && Platform.isMacOS;

  bool get _isWindows =>
      _runtimePlatform == ClientBridgeRuntimePlatform.host &&
      Platform.isWindows;

  bool get _isLinux =>
      _runtimePlatform == ClientBridgeRuntimePlatform.host && Platform.isLinux;

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
        fields: {
          'platform': platform,
          'message': reportError.toString(),
        },
      );
    }
  }

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

  String? _businessEventType(Map<String, Object?> event) {
    return event['businessType'] as String? ?? event['type'] as String?;
  }

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

  void _settleNetworkToggleFromEvent() {
    if (!_networkToggleInFlight && _networkToggleOperation == null) {
      return;
    }
    _networkToggleInFlight = false;
    _networkToggleOperation = null;
    _networkToggleEpoch++;
  }

  void _setStateIfChanged(ClientViewState state) {
    final next = _localLogoutRequested
        ? ClientViewState.initial()
        : _preserveMobilePlatformNetworkState(state);
    if (_state.value == next) {
      return;
    }
    _state.value = next;
  }

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

  Map<String, Object?>? _jsonMap(Object? result) {
    if (result is Map) {
      return result.cast<String, Object?>();
    }
    return ClientCoreLocalService.jsonMapFromResult(result);
  }
}

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

String _androidVpnConfigFingerprint(AndroidVpnSessionConfig config) {
  return jsonEncode(config.toJson());
}
