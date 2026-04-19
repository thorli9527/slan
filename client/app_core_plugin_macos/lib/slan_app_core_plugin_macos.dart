library slan_app_core_plugin_macos;

import 'package:slan_app_core_plugin_platform_interface/slan_app_core_plugin_platform_interface.dart';

export 'package:slan_app_core_plugin_platform_interface/slan_app_core_plugin_platform_interface.dart'
    show
        SlanAppCorePluginPlatform,
        WireGuardTunnelActionResult,
        WireGuardTunnelConfiguration,
        WireGuardTunnelInterfaceConfiguration,
        WireGuardTunnelKeyPair,
        WireGuardTunnelPeerConfiguration,
        WireGuardTunnelRuntimeView;

class SlanAppCorePluginMacos {
  const SlanAppCorePluginMacos();

  SlanAppCorePluginPlatform get _platform => SlanAppCorePluginPlatform.instance;

  Future<WireGuardTunnelActionResult> applyTunnelConfiguration(
    WireGuardTunnelConfiguration configuration,
  ) {
    return _platform.applyTunnelConfiguration(configuration);
  }

  Future<WireGuardTunnelActionResult> removeTunnelPeer(String peerVirtualIp) {
    return _platform.removeTunnelPeer(peerVirtualIp);
  }

  Future<WireGuardTunnelActionResult> bringTunnelUp() {
    return _platform.bringTunnelUp();
  }

  Future<WireGuardTunnelActionResult> bringTunnelDown() {
    return _platform.bringTunnelDown();
  }

  Future<WireGuardTunnelRuntimeView?> tunnelRuntimeView(String peerVirtualIp) {
    return _platform.tunnelRuntimeView(peerVirtualIp);
  }
}
