import 'package:plugin_platform_interface/plugin_platform_interface.dart';

import 'method_channel_slan_app_core_plugin.dart';
import 'tunnel_backend_models.dart';

abstract class SlanAppCorePluginPlatform extends PlatformInterface {
  SlanAppCorePluginPlatform() : super(token: _token);

  static final Object _token = Object();

  static SlanAppCorePluginPlatform _instance = MethodChannelSlanAppCorePlugin();

  static SlanAppCorePluginPlatform get instance => _instance;

  static set instance(SlanAppCorePluginPlatform instance) {
    PlatformInterface.verifyToken(instance, _token);
    _instance = instance;
  }

  Future<Object?> invoke(
    String method, [
    Map<String, Object?> args = const {},
  ]) {
    throw UnimplementedError('invoke() has not been implemented.');
  }

  Future<WireGuardTunnelActionResult> applyTunnelConfiguration(
    WireGuardTunnelConfiguration configuration,
  ) async {
    final payload =
        await invoke('applyTunnelConfiguration', configuration.toJson());
    if (payload is Map<Object?, Object?>) {
      return WireGuardTunnelActionResult.fromJson(payload);
    }
    throw ArgumentError.value(
      payload,
      'payload',
      'Expected applyTunnelConfiguration payload to be a map',
    );
  }

  Future<WireGuardTunnelActionResult> removeTunnelPeer(
      String peerVirtualIp) async {
    final payload = await invoke('removeTunnelPeer', {
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

  Future<WireGuardTunnelActionResult> bringTunnelUp() async {
    final payload = await invoke('bringTunnelUp');
    if (payload is Map<Object?, Object?>) {
      return WireGuardTunnelActionResult.fromJson(payload);
    }
    throw ArgumentError.value(
      payload,
      'payload',
      'Expected bringTunnelUp payload to be a map',
    );
  }

  Future<WireGuardTunnelActionResult> bringTunnelDown() async {
    final payload = await invoke('bringTunnelDown');
    if (payload is Map<Object?, Object?>) {
      return WireGuardTunnelActionResult.fromJson(payload);
    }
    throw ArgumentError.value(
      payload,
      'payload',
      'Expected bringTunnelDown payload to be a map',
    );
  }

  Future<WireGuardTunnelRuntimeView?> tunnelRuntimeView(
    String peerVirtualIp,
  ) async {
    final payload = await invoke('tunnelRuntimeView', {
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
}
