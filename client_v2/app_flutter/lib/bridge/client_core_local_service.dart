import 'dart:async';
import 'dart:convert';
import 'dart:io';

import 'package:client_core_plugin/client_core_plugin.dart';
import 'package:flutter/services.dart';

class ClientCoreLocalService {
  ClientCoreLocalService({String? host}) : _host = host;

  static const _definedServiceHost =
      String.fromEnvironment('SLAN_CLIENT_CORE_SERVICE_HOST');

  final String? _host;

  Future<Map<String, Object?>?> localState() async {
    return requestJson('localState');
  }

  Future<Map<String, Object?>?> localStateWatch({
    required int lastRevision,
    int timeoutMs = 30000,
  }) async {
    return requestJson(
      'localStateWatch',
      arguments: {
        'lastRevision': lastRevision,
        'timeoutMs': timeoutMs,
      },
      allowEmptyResponse: true,
    );
  }

  Future<Map<String, Object?>?> localStatus() async {
    return requestJson('localStatus');
  }

  Future<Map<String, Object?>?> localSession() async {
    return requestJson('localSession');
  }

  Future<Map<String, Object?>?> localPeers() async {
    return requestJson('localPeers');
  }

  Future<Map<String, Object?>?> localPathPlan() async {
    return requestJson('localPathPlan');
  }

  Future<Map<String, Object?>?> localPathDiagnose() async {
    return requestJson('localPathDiagnose');
  }

  Future<Map<String, Object?>?> localRelayCandidates(
      {bool refresh = false}) async {
    return requestJson(
      refresh ? 'localRefreshRelayCandidates' : 'localRelayCandidates',
    );
  }

  Future<Map<String, Object?>?> localControlStatus() async {
    return requestJson('localControlStatus');
  }

  Future<Map<String, Object?>?> localControlPlan() async {
    return requestJson('localControlPlan');
  }

  Future<Map<String, Object?>?> localControlCadence() async {
    return requestJson('localControlCadence');
  }

  Future<Map<String, Object?>?> localControlTickPlan({
    Object? arguments,
  }) async {
    return requestJson('localControlTickPlan', arguments: arguments);
  }

  Future<Map<String, Object?>?> localControlOutbox({
    Object? arguments,
  }) async {
    return requestJson('localControlOutbox', arguments: arguments);
  }

  Future<Map<String, Object?>?> localPendingControlAcks() async {
    return requestJson('localPendingControlAcks');
  }

  Future<Map<String, Object?>?> localMarkControlAcked({
    required String taskId,
  }) async {
    return requestJson(
      'localMarkControlAcked',
      arguments: {'taskId': taskId},
    );
  }

  Future<Map<String, Object?>?> localMarkTransportPublished({
    Object? arguments,
  }) async {
    return requestJson('localMarkTransportPublished', arguments: arguments);
  }

  Future<Map<String, Object?>?> localRelayPrepare() async {
    return requestJson('localRelayPrepare');
  }

  Future<Map<String, Object?>?> logout() async {
    return requestJson('localLogout');
  }

  Future<Map<String, Object?>?> localNetworkActivate() async {
    return requestJson('localNetworkActivate');
  }

  Future<Map<String, Object?>?> localNetworkDeactivate() async {
    return requestJson('localNetworkDeactivate');
  }

  Future<Map<String, Object?>?> localNetworkShutdown() async {
    return requestJson('localNetworkShutdown');
  }

  Future<Map<String, Object?>?> localSendClientMessage({
    required String targetDeviceId,
    required String body,
    Map<String, Object?>? metadata,
  }) async {
    return requestJson(
      'localSendClientMessage',
      arguments: {
        'targetDeviceId': targetDeviceId,
        'body': body,
        if (metadata != null) 'metadata': metadata,
      },
    );
  }

  Future<Map<String, Object?>?> localBusinessEventWatch({
    required int lastRevision,
    int timeoutMs = 30000,
  }) async {
    return requestJson(
      'localBusinessEventWatch',
      arguments: {
        'lastRevision': lastRevision,
        'timeoutMs': timeoutMs,
      },
      allowEmptyResponse: true,
    );
  }

  Future<AndroidVpnSessionConfig?> localPlatformNetworkConfig() async {
    final json = await requestJson('localPlatformNetworkConfig');
    return json == null ? null : AndroidVpnSessionConfig.fromJson(json);
  }

  Future<Map<String, Object?>?> ingestPlatformRuntimeState({
    required Map<String, Object?> runtimeState,
    String? platform,
    Map<String, Object?>? traffic,
    String? error,
  }) {
    return requestJson(
      'ingestPlatformRuntimeState',
      arguments: {
        if (platform != null) 'platform': platform,
        'runtimeState': runtimeState,
        if (traffic != null) 'traffic': traffic,
        if (error != null) 'error': error,
        'reportedAtMs': DateTime.now().millisecondsSinceEpoch,
      },
    );
  }

  Future<Map<String, Object?>?> localDiagnosticsExport() async {
    return requestJson('localDiagnosticsExport');
  }

  Future<Map<String, Object?>?> requestJson(
    String method, {
    Object? arguments,
    bool allowEmptyResponse = false,
  }) async {
    final result = await request(
      method,
      arguments: arguments,
      allowEmptyResponse: allowEmptyResponse,
    );
    return jsonMapFromResult(result);
  }

  Future<Object?> request(
    String method, {
    Object? arguments,
    bool allowEmptyResponse = false,
  }) async {
    final host = _serviceHost();
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
      try {
        return await socket
            .cast<List<int>>()
            .transform(utf8.decoder)
            .transform(const LineSplitter())
            .first
            .timeout(const Duration(seconds: 90));
      } on StateError {
        if (allowEmptyResponse) {
          return '';
        }
        rethrow;
      }
    } finally {
      await socket.close();
    }
  }

  String _serviceHost() {
    return _host ??
        (_definedServiceHost.isEmpty ? null : _definedServiceHost) ??
        Platform.environment['SLAN_CLIENT_CORE_SERVICE_HOST'] ??
        '127.0.0.1:46392';
  }

  static Map<String, Object?>? jsonMapFromResult(Object? result) {
    if (result is Map) {
      return result.cast<String, Object?>();
    }
    if (result is String && result.trim().isNotEmpty) {
      return jsonDecode(result) as Map<String, Object?>;
    }
    return null;
  }

  static String stringResult(Object? result) {
    if (result is String) {
      final value = result.trim();
      if (value.startsWith('{')) {
        final decoded = jsonDecode(value);
        if (decoded is Map && decoded['permissionState'] is String) {
          return decoded['permissionState'] as String;
        }
      }
      return value;
    }
    if (result is Map && result['permissionState'] is String) {
      return result['permissionState'] as String;
    }
    return '';
  }
}
