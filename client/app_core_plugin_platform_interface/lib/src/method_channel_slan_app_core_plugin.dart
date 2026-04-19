import 'package:flutter/services.dart';

import 'slan_app_core_plugin_platform.dart';

class MethodChannelSlanAppCorePlugin extends SlanAppCorePluginPlatform {
  static const MethodChannel _channel = MethodChannel('slan/app_core');

  @override
  Future<Object?> invoke(
    String method, [
    Map<String, Object?> args = const {},
  ]) {
    return _channel.invokeMethod<Object?>(method, args);
  }
}
