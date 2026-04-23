import 'package:flutter_test/flutter_test.dart';
import 'package:slan_app/application/tunnel_host_gateway.dart';
import 'package:slan_app/infra/app_core/api/mock_app_core_api.dart';
import 'package:slan_app/infra/app_core/scope/app_core_scope.dart';
import 'package:slan_app_core_plugin/slan_app_core_plugin.dart';

void main() {
  test('configureForTest can override tunnel host gateway', () {
    final gateway = _FakeTunnelHostGateway();

    AppCoreScope.configureForTest(
      appCoreApi: MockAppCoreApi(),
      tunnelHostGateway: gateway,
    );
    addTearDown(AppCoreScope.resetForTest);

    expect(AppCoreScope.tunnelHostGateway, same(gateway));
  });

  test('resetForTest restores default tunnel host gateway', () {
    final gateway = _FakeTunnelHostGateway();

    AppCoreScope.configureForTest(
      appCoreApi: MockAppCoreApi(),
      tunnelHostGateway: gateway,
    );
    AppCoreScope.resetForTest();

    expect(AppCoreScope.tunnelHostGateway, isNot(same(gateway)));
    expect(AppCoreScope.tunnelHostGateway, isA<TunnelHostGateway>());
  });
}

class _FakeTunnelHostGateway extends TunnelHostGateway {
  @override
  Future<WireGuardTunnelActionResult> applyTunnelConfiguration(
    WireGuardTunnelConfiguration configuration,
  ) {
    throw UnimplementedError();
  }

  @override
  Future<WireGuardTunnelActionResult> bringTunnelDown() {
    throw UnimplementedError();
  }

  @override
  Future<WireGuardTunnelActionResult> bringTunnelUp() {
    throw UnimplementedError();
  }

  @override
  Future<WireGuardTunnelActionResult> removeTunnelPeer(String peerVirtualIp) {
    throw UnimplementedError();
  }

  @override
  Future<WireGuardTunnelRuntimeView?> tunnelRuntimeView(String peerVirtualIp) {
    throw UnimplementedError();
  }
}
