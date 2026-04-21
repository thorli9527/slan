import 'package:slan_app_core_plugin/slan_app_core_plugin.dart';

import '../models/tunnel_action_models.dart';
import 'app_core_coordinator.dart';

class AppTunnelController {
  const AppTunnelController(this._coordinator);

  final AppCoreCoordinator _coordinator;

  Future<TunnelActionReport> applyTunnelConfiguration({
    required WireGuardTunnelConfiguration configuration,
    String? verifyPeerVirtualIp,
  }) =>
      _coordinator.applyTunnelConfiguration(
        configuration: configuration,
        verifyPeerVirtualIp: verifyPeerVirtualIp,
      );

  Future<TunnelActionReport> bringTunnelUp({
    String? verifyPeerVirtualIp,
  }) =>
      _coordinator.bringTunnelUp(
        verifyPeerVirtualIp: verifyPeerVirtualIp,
      );

  Future<TunnelActionReport> bringTunnelDown() =>
      _coordinator.bringTunnelDown();

  Future<TunnelActionReport> removeTunnelPeer({
    required String peerVirtualIp,
  }) =>
      _coordinator.removeTunnelPeer(peerVirtualIp: peerVirtualIp);

  Future<TunnelActionReport> refreshTunnelRuntime({
    required String peerVirtualIp,
  }) =>
      _coordinator.refreshTunnelRuntime(peerVirtualIp: peerVirtualIp);

  Future<void> probe({
    required String payload,
    int? probeTimeoutMs,
  }) =>
      _coordinator.probe(
        payload: payload,
        probeTimeoutMs: probeTimeoutMs,
      );

  Future<void> send({
    required String payload,
  }) =>
      _coordinator.send(payload: payload);
}
