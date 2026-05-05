import 'dart:convert';
import 'dart:io';

import 'package:flutter/services.dart';

import 'android_vpn_contract.dart';

class ClientCorePlugin {
  ClientCorePlugin({MethodChannel? channel})
      : _channel = channel ?? const MethodChannel('dev.slan/client_core_v2');

  static const _definedServiceHost =
      String.fromEnvironment('SLAN_CLIENT_CORE_SERVICE_HOST');

  final MethodChannel _channel;

  static void registerWith() {}

  Future<Object?> start() {
    return _invoke('start');
  }

  Future<Object?> localState() {
    return _invoke('localState');
  }

  Future<Object?> refresh() {
    return _invoke('refresh');
  }

  Future<Object?> localStateWatch(int lastRevision) {
    return _invoke('localStateWatch', {
      'lastRevision': lastRevision,
      'timeoutMs': 30000,
    });
  }

  Future<Object?> localBusinessEventWatch(int lastRevision) {
    return _invoke('localBusinessEventWatch', {
      'lastRevision': lastRevision,
      'timeoutMs': 30000,
    });
  }

  Future<Object?> dispatch(Map<String, Object?> command) {
    return _invoke('dispatch', command);
  }

  Future<Object?> enqueueControlTask(Map<String, Object?> task) {
    return _invoke('enqueueControlTask', task);
  }

  Future<Object?> enqueueDownstreamControlTask(Map<String, Object?> task) {
    return _invoke('enqueueDownstreamControlTask', task);
  }

  Future<Object?> ingestDownstreamControlMessage(Map<String, Object?> message) {
    return _invoke('ingestDownstreamControlMessage', message);
  }

  Future<Object?> localNetworkShutdown() {
    return _invoke('localNetworkShutdown');
  }

  Future<Object?> localControlStatus() {
    return _invoke('localControlStatus');
  }

  Future<Object?> localRelayCandidates() {
    return _invoke('localRelayCandidates');
  }

  Future<Object?> localRefreshRelayCandidates() {
    return _invoke('localRefreshRelayCandidates');
  }

  Future<Object?> localControlPlan() {
    return _invoke('localControlPlan');
  }

  Future<Object?> localControlCadence() {
    return _invoke('localControlCadence');
  }

  Future<Object?> localControlTickPlan(Map<String, Object?> tick) {
    return _invoke('localControlTickPlan', tick);
  }

  Future<Object?> localControlOutbox([Map<String, Object?>? options]) {
    return _invoke('localControlOutbox', options);
  }

  Future<Object?> localPendingControlAcks() {
    return _invoke('localPendingControlAcks');
  }

  Future<Object?> localMarkControlAcked(Map<String, Object?> ack) {
    return _invoke('localMarkControlAcked', ack);
  }

  Future<Object?> localMarkTransportPublished(Map<String, Object?> message) {
    return _invoke('localMarkTransportPublished', message);
  }

  Future<Object?> androidVpnPermissionState() {
    return _invokeNativeOnly('androidVpnPermissionState');
  }

  Future<AndroidVpnConsentRequest?> androidRequestVpnPermission() async {
    final result = await _invokeNativeOnly('androidRequestVpnPermission');
    final json = _jsonMap(result);
    return json == null ? null : AndroidVpnConsentRequest.fromJson(json);
  }

  Future<Object?> androidStartVpn(AndroidVpnSessionConfig config) {
    return _invokeNativeOnly('androidStartVpn', config.toJson());
  }

  Future<Object?> androidStopVpn() {
    return _invokeNativeOnly('androidStopVpn');
  }

  Future<Object?> androidProtectSocket(AndroidSocketProtectionRequest request) {
    return _invokeNativeOnly('androidProtectSocket', request.toJson());
  }

  Future<Object?> androidRuntimeState() {
    return _invokeNativeOnly('androidRuntimeState');
  }

  Future<AndroidNetworkEvent?> androidPollNetworkEvent() async {
    final result = await _invokeNativeOnly('androidPollNetworkEvent');
    final json = _jsonMap(result);
    return json == null ? null : AndroidNetworkEvent.fromJson(json);
  }

  Future<Object?> _invoke(String method, [Object? arguments]) async {
    try {
      return await _channel.invokeMethod<Object?>(method, arguments);
    } on MissingPluginException {
      final result = await _invokeLocalService(method, arguments);
      await _afterFallbackInvoke(method, arguments, result);
      return result;
    }
  }

  Future<Object?> _invokeNativeOnly(String method, [Object? arguments]) {
    return _channel.invokeMethod<Object?>(method, arguments);
  }

  Map<String, Object?>? _jsonMap(Object? result) {
    if (result is Map) {
      return result.cast<String, Object?>();
    }
    if (result is String && result.trim().isNotEmpty) {
      final decoded = jsonDecode(result);
      return decoded is Map ? decoded.cast<String, Object?>() : null;
    }
    return null;
  }

  Future<Object?> _invokeLocalService(String method, Object? arguments) async {
    final host = (_definedServiceHost.isEmpty ? null : _definedServiceHost) ??
        Platform.environment['SLAN_CLIENT_CORE_SERVICE_HOST'] ??
        '127.0.0.1:46392';
    final separator = host.lastIndexOf(':');
    if (separator <= 0 || separator == host.length - 1) {
      throw PlatformException(
        code: 'invalid_service_host',
        message: 'invalid SLAN_CLIENT_CORE_SERVICE_HOST: $host',
      );
    }
    final hostname = host.substring(0, separator);
    final port = int.tryParse(host.substring(separator + 1));
    if (port == null || port <= 0 || port > 65535) {
      throw PlatformException(
        code: 'invalid_service_port',
        message: 'invalid SLAN client service port: $host',
      );
    }

    final socket = await Socket.connect(
      hostname,
      port,
      timeout: const Duration(seconds: 2),
    );
    try {
      final payload = jsonEncode({
        'method': method,
        'args': arguments ?? <String, Object?>{},
      });
      socket.write('$payload\n');
      await socket.flush();
      final line = await socket
          .cast<List<int>>()
          .transform(utf8.decoder)
          .transform(const LineSplitter())
          .first
          .timeout(const Duration(seconds: 5));
      return line;
    } finally {
      await socket.close();
    }
  }

  Future<void> _afterFallbackInvoke(
    String method,
    Object? arguments,
    Object? result,
  ) async {
    if (method != 'dispatch' || arguments is! Map) {
      return;
    }
    final type = arguments['type'];
    if (type != 'loginWithBrowser' && type != 'openWebConsole') {
      return;
    }
    try {
      if (type == 'loginWithBrowser') {
        await _openConsole(
          callbackId: _stringField(result, 'authCallbackId'),
          deviceId: _stringField(result, 'deviceId'),
        );
        return;
      }
      final loginKeyResult = await _invokeLocalService('consoleLoginKey', null);
      await _openConsole(
        consoleLoginKey: _stringField(loginKeyResult, 'loginKey'),
        deviceId: _stringField(loginKeyResult, 'deviceId'),
      );
    } on Object {
      // Browser opening is best-effort for platforms without a native plugin.
    }
  }

  String _stringField(Object? result, String key) {
    if (result is! String || result.trim().isEmpty) {
      return '';
    }
    final decoded = jsonDecode(result);
    if (decoded is! Map) {
      return '';
    }
    final value = decoded[key];
    return value is String ? value : '';
  }

  Future<void> _openConsole({
    String callbackId = '',
    String deviceId = '',
    String consoleLoginKey = '',
  }) async {
    final safeDeviceId = _usableClientDeviceId(deviceId);
    final baseUrl = Platform.environment['SLAN_WEB_CONSOLE_URL'] ??
        'http://127.0.0.1:24200';
    final uri = Uri.parse(baseUrl).replace(
      queryParameters: {
        ...Uri.parse(baseUrl).queryParameters,
        if (callbackId.isNotEmpty) 'auth': 'login',
        if (callbackId.isNotEmpty) 'callbackId': callbackId,
        if (consoleLoginKey.isNotEmpty) 'consoleLoginKey': consoleLoginKey,
        if (safeDeviceId.isNotEmpty) 'deviceId': safeDeviceId,
        'clientPlatform': _clientPlatform(),
        'clientName': 'SLAN Client V2',
      },
    );
    final url = uri.toString();
    if (Platform.isLinux) {
      await Process.start('xdg-open', [url], mode: ProcessStartMode.detached);
    } else if (Platform.isMacOS) {
      await Process.start('open', [url], mode: ProcessStartMode.detached);
    } else if (Platform.isWindows) {
      await Process.start(
          'powershell.exe',
          [
            '-NoProfile',
            '-Command',
            'Start-Process',
            url,
          ],
          mode: ProcessStartMode.detached);
    }
  }

  String _usableClientDeviceId(String deviceId) {
    final value = deviceId.trim();
    if (value.isEmpty) {
      return '';
    }
    final lower = value.toLowerCase();
    if (lower == 'authcallbackid' ||
        lower == 'windows-plugin-login' ||
        lower == 'macos-plugin-login' ||
        lower.startsWith('cb-')) {
      return '';
    }
    return value;
  }

  String _clientPlatform() {
    if (Platform.isWindows) {
      return 'windows';
    }
    if (Platform.isMacOS) {
      return 'macos';
    }
    if (Platform.isLinux) {
      return 'linux';
    }
    if (Platform.isAndroid) {
      return 'android';
    }
    if (Platform.isIOS) {
      return 'ios';
    }
    return 'unknown';
  }
}
