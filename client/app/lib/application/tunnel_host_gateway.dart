library slan_app.application.tunnel_host_gateway;

import 'dart:async';
import 'dart:convert';
import 'dart:io';

import 'package:flutter/services.dart';
import 'package:slan_app_core_plugin/slan_app_core_plugin.dart';

abstract class TunnelHostGateway {
  const TunnelHostGateway();

  Future<WireGuardTunnelActionResult> applyTunnelConfiguration(
    WireGuardTunnelConfiguration configuration,
  );

  Future<WireGuardTunnelActionResult> bringTunnelUp();

  Future<WireGuardTunnelActionResult> bringTunnelDown();

  Future<WireGuardTunnelActionResult> removeTunnelPeer(String peerVirtualIp);

  Future<WireGuardTunnelRuntimeView?> tunnelRuntimeView(String peerVirtualIp);
}

class HelperServiceTunnelHostGateway extends TunnelHostGateway {
  const HelperServiceTunnelHostGateway({
    required this.address,
    this.timeout = const Duration(seconds: 5),
  });

  final String address;
  final Duration timeout;

  @override
  Future<WireGuardTunnelActionResult> applyTunnelConfiguration(
    WireGuardTunnelConfiguration configuration,
  ) async {
    final payload = await _invoke(
      'applyTunnelConfiguration',
      configuration.toJson(),
    );
    if (payload is Map<Object?, Object?>) {
      return WireGuardTunnelActionResult.fromJson(payload);
    }
    throw ArgumentError.value(
      payload,
      'payload',
      'Expected applyTunnelConfiguration payload to be a map',
    );
  }

  @override
  Future<WireGuardTunnelActionResult> bringTunnelUp() async {
    final payload = await _invoke('bringTunnelUp');
    if (payload is Map<Object?, Object?>) {
      return WireGuardTunnelActionResult.fromJson(payload);
    }
    throw ArgumentError.value(
      payload,
      'payload',
      'Expected bringTunnelUp payload to be a map',
    );
  }

  @override
  Future<WireGuardTunnelActionResult> bringTunnelDown() async {
    final payload = await _invoke('bringTunnelDown');
    if (payload is Map<Object?, Object?>) {
      return WireGuardTunnelActionResult.fromJson(payload);
    }
    throw ArgumentError.value(
      payload,
      'payload',
      'Expected bringTunnelDown payload to be a map',
    );
  }

  @override
  Future<WireGuardTunnelActionResult> removeTunnelPeer(
    String peerVirtualIp,
  ) async {
    final payload = await _invoke('removeTunnelPeer', {
      'peerVirtualIp': peerVirtualIp,
    });
    if (payload is Map<Object?, Object?>) {
      return WireGuardTunnelActionResult.fromJson(payload);
    }
    throw ArgumentError.value(
      payload,
      'payload',
      'Expected removeTunnelPeer payload to be a map',
    );
  }

  @override
  Future<WireGuardTunnelRuntimeView?> tunnelRuntimeView(
    String peerVirtualIp,
  ) async {
    final payload = await _invoke('tunnelRuntimeView', {
      'peerVirtualIp': peerVirtualIp,
    });
    if (payload == null) {
      return null;
    }
    if (payload is Map<Object?, Object?>) {
      return WireGuardTunnelRuntimeView.fromJson(payload);
    }
    throw ArgumentError.value(
      payload,
      'payload',
      'Expected tunnelRuntimeView payload to be a map',
    );
  }

  Future<Object?> _invoke(
    String method, [
    Map<String, Object?> args = const {},
  ]) async {
    final endpoint = _parseTcpAddress(address);
    final socket =
        await Socket.connect(endpoint.host, endpoint.port, timeout: timeout);
    try {
      socket.writeln(jsonEncode({
        'method': method,
        'args': args,
      }));
      await socket.flush();
      final responseLine = await socket
          .cast<List<int>>()
          .transform(utf8.decoder)
          .transform(const LineSplitter())
          .first
          .timeout(timeout);
      return _decodeHelperResponse(responseLine);
    } on TimeoutException {
      throw PlatformException(
        code: 'app_core_helper_timeout',
        message: 'Timed out waiting for helper host response from $address.',
      );
    } on SocketException catch (error) {
      throw PlatformException(
        code: 'app_core_helper_unavailable',
        message: 'Failed to reach helper host $address: $error',
      );
    } finally {
      await socket.close();
    }
  }
}

class PluginTunnelHostGateway extends TunnelHostGateway {
  const PluginTunnelHostGateway({
    this.pluginPlatform,
  });

  final SlanAppCorePluginPlatform? pluginPlatform;

  SlanAppCorePluginPlatform get _pluginPlatform =>
      pluginPlatform ?? SlanAppCorePluginPlatform.instance;

  @override
  Future<WireGuardTunnelActionResult> applyTunnelConfiguration(
    WireGuardTunnelConfiguration configuration,
  ) {
    return _pluginPlatform.applyTunnelConfiguration(configuration);
  }

  @override
  Future<WireGuardTunnelActionResult> bringTunnelUp() {
    return _pluginPlatform.bringTunnelUp();
  }

  @override
  Future<WireGuardTunnelActionResult> bringTunnelDown() {
    return _pluginPlatform.bringTunnelDown();
  }

  @override
  Future<WireGuardTunnelActionResult> removeTunnelPeer(String peerVirtualIp) {
    return _pluginPlatform.removeTunnelPeer(peerVirtualIp);
  }

  @override
  Future<WireGuardTunnelRuntimeView?> tunnelRuntimeView(String peerVirtualIp) {
    return _pluginPlatform.tunnelRuntimeView(peerVirtualIp);
  }
}

Object? _decodeHelperResponse(String payload) {
  final dynamic decoded;
  try {
    decoded = jsonDecode(payload);
  } on FormatException {
    throw const FormatException('Expected JSON payload from helper host');
  }
  if (decoded is! Map) {
    throw const FormatException('Expected object payload from helper host');
  }
  final response = decoded.map(
    (key, value) => MapEntry(key.toString(), value),
  );
  final ok = response['ok'];
  if (ok is! bool) {
    throw const FormatException('Expected ok flag from helper host');
  }
  if (ok) {
    return response['result'];
  }
  throw PlatformException(
    code: response['errorCode'] as String? ?? 'app_core_helper_error',
    message: response['errorMessage'] as String? ??
        response['error'] as String? ??
        'unknown app-core helper error',
  );
}

Uri _parseTcpAddress(String rawAddress) {
  final normalized = rawAddress.trim();
  if (normalized.isEmpty) {
    throw const FormatException('Helper host address must not be empty');
  }
  final withScheme =
      normalized.contains('://') ? normalized : 'tcp://$normalized';
  final uri = Uri.tryParse(withScheme);
  if (uri == null || uri.host.isEmpty || !uri.hasPort) {
    throw FormatException('Invalid helper host address: $rawAddress');
  }
  return uri;
}
