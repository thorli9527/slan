import 'package:flutter/services.dart';
import 'dart:convert';

import 'slan_app_core_plugin_platform.dart';

class MethodChannelSlanAppCorePlugin extends SlanAppCorePluginPlatform {
  static const MethodChannel _channel = MethodChannel('slan/app_core');

  @override
  Future<Object?> invoke(
    String method, [
    Map<String, Object?> args = const {},
  ]) {
    return _channel
        .invokeMethod<Object?>(method, args)
        .then(_decodeHelperEnvelopeIfNeeded);
  }
}

Object? _decodeHelperEnvelopeIfNeeded(Object? payload) {
  if (payload is! String) {
    return payload;
  }
  final dynamic decoded;
  try {
    decoded = jsonDecode(payload);
  } on FormatException {
    return payload;
  }
  if (decoded is! Map<String, dynamic>) {
    return decoded;
  }
  final ok = decoded['ok'];
  if (ok is bool) {
    if (ok) {
      return decoded['result'];
    }
    throw PlatformException(
      code: decoded['errorCode'] as String? ?? 'app_core_helper_error',
      message: decoded['errorMessage'] as String? ??
          decoded['error'] as String? ??
          'unknown app-core helper error',
    );
  }
  return decoded;
}
