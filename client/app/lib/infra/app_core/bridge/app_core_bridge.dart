import 'package:flutter/services.dart';

abstract class AppCoreBridge {
  Future<Object?> invoke(String method, [Map<String, Object?> args = const {}]);
}

class MethodChannelAppCoreBridge implements AppCoreBridge {
  MethodChannelAppCoreBridge({
    MethodChannel? channel,
  }) : _channel = channel ?? const MethodChannel('slan/app_core');

  final MethodChannel _channel;

  @override
  Future<Object?> invoke(
    String method, [
    Map<String, Object?> args = const {},
  ]) {
    return _channel.invokeMethod<Object?>(method, args);
  }
}
