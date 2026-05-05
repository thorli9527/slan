import 'dart:async';
import 'dart:convert';
import 'dart:io';

import 'package:client_core_plugin/client_core_plugin.dart';
import 'package:flutter/services.dart';

class ClientCoreLocalService {
  ClientCoreLocalService({String? host}) : _host = host;

  final String? _host;

  Future<Map<String, Object?>?> state() async {
    return requestJson('state');
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

  Future<Map<String, Object?>?> logout() async {
    return requestJson('logout');
  }

  Future<Map<String, Object?>?> activateNetwork() async {
    return requestJson('activateNetwork');
  }

  Future<Map<String, Object?>?> deactivateNetwork() async {
    return requestJson('deactivateNetwork');
  }

  Future<Map<String, Object?>?> watchBusinessEvent({
    required int lastRevision,
    int timeoutMs = 30000,
  }) async {
    return requestJson(
      'watchBusinessEvent',
      arguments: {
        'lastRevision': lastRevision,
        'timeoutMs': timeoutMs,
      },
      allowEmptyResponse: true,
    );
  }

  Future<AndroidVpnSessionConfig?> androidNetworkConfig() async {
    final json = await requestJson('androidNetworkConfig');
    return json == null ? null : AndroidVpnSessionConfig.fromJson(json);
  }

  Future<Map<String, Object?>?> exportDiagnostics() async {
    return requestJson('exportDiagnostics');
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
