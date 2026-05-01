import 'dart:convert';
import 'dart:io';

import 'package:flutter/services.dart';

class ClientCorePlugin {
  ClientCorePlugin({MethodChannel? channel})
      : _channel = channel ?? const MethodChannel('dev.slan/client_core_v2');

  final MethodChannel _channel;

  static void registerWith() {}

  Future<Object?> start() {
    return _invoke('start');
  }

  Future<Object?> state() {
    return _invoke('state');
  }

  Future<Object?> refresh() {
    return _invoke('refresh');
  }

  Future<Object?> watchState(int lastRevision) {
    return _invoke('watchState', {
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

  Future<Object?> shutdownNetwork() {
    return _invoke('shutdownNetwork');
  }

  Future<Object?> controlTransportStatus() {
    return _invoke('controlTransportStatus');
  }

  Future<Object?> controlTransportPlan() {
    return _invoke('controlTransportPlan');
  }

  Future<Object?> controlTransportCadence() {
    return _invoke('controlTransportCadence');
  }

  Future<Object?> controlTransportTickPlan(Map<String, Object?> tick) {
    return _invoke('controlTransportTickPlan', tick);
  }

  Future<Object?> controlTransportOutbox([Map<String, Object?>? options]) {
    return _invoke('controlTransportOutbox', options);
  }

  Future<Object?> pendingControlAcks() {
    return _invoke('pendingControlAcks');
  }

  Future<Object?> markControlAcked(Map<String, Object?> ack) {
    return _invoke('markControlAcked', ack);
  }

  Future<Object?> markTransportPublished(Map<String, Object?> message) {
    return _invoke('markTransportPublished', message);
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

  Future<Object?> _invokeLocalService(String method, Object? arguments) async {
    final host = Platform.environment['SLAN_CLIENT_CORE_SERVICE_HOST'] ??
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
      final callbackId = type == 'loginWithBrowser'
          ? _stringField(result, 'authCallbackId')
          : '';
      final deviceId =
          type == 'loginWithBrowser' ? _stringField(result, 'deviceId') : '';
      await _openConsole(callbackId: callbackId, deviceId: deviceId);
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
  }) async {
    final baseUrl =
        Platform.environment['SLAN_WEB_CONSOLE_URL'] ?? 'http://127.0.0.1:24200';
    final uri = Uri.parse(baseUrl).replace(queryParameters: {
      ...Uri.parse(baseUrl).queryParameters,
      if (callbackId.isNotEmpty) 'auth': 'login',
      if (callbackId.isNotEmpty) 'callbackId': callbackId,
      if (deviceId.isNotEmpty) 'deviceId': deviceId,
    });
    final url = uri.toString();
    if (Platform.isLinux) {
      await Process.start('xdg-open', [url], mode: ProcessStartMode.detached);
    } else if (Platform.isMacOS) {
      await Process.start('open', [url], mode: ProcessStartMode.detached);
    } else if (Platform.isWindows) {
      await Process.start(
        'powershell.exe',
        ['-NoProfile', '-Command', 'Start-Process', url],
        mode: ProcessStartMode.detached,
      );
    }
  }
}
