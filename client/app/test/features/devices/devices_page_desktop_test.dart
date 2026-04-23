import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:slan_app/application/tunnel_host_gateway.dart';
import 'package:slan_app/features/devices/devices_page.dart';
import 'package:slan_app/infra/app_core/api/mock_app_core_api.dart';
import 'package:slan_app/infra/app_core/scope/app_core_scope.dart';
import 'package:slan_app/infra/app_core/api/dev_defaults.dart';
import 'package:slan_app/testing/app_test_keys.dart';
import 'package:slan_app_core_plugin/slan_app_core_plugin.dart';

void main() {
  testWidgets('DevicesPage renders desktop workbench sections', (
    WidgetTester tester,
  ) async {
    tester.view.physicalSize = const Size(1440, 1400);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.reset);

    await tester.pumpWidget(
      const MaterialApp(
        home: DevicesPage(),
      ),
    );

    expect(find.text('Devices Workspace'), findsOneWidget);
    expect(find.text('Identity'), findsOneWidget);
    expect(find.text('Connectivity'), findsOneWidget);
    expect(find.text('Diagnostics'), findsOneWidget);
    expect(find.text('WireGuard Tunnel Debug'), findsOneWidget);
    expect(find.text('Runtime State'), findsOneWidget);
    expect(find.byKey(AppTestKeys.devicesStateCard), findsOneWidget);
  });

  testWidgets('DevicesPage shows control plan guidance after quick setup and connect', (
    WidgetTester tester,
  ) async {
    tester.view.physicalSize = const Size(1440, 1400);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.reset);

    AppCoreScope.configureForTest(appCoreApi: MockAppCoreApi());
    addTearDown(AppCoreScope.resetForTest);

    await AppCoreScope.sessionController.registerDevice(
      name: 'thor-mac',
      platform: 'macos',
      machineId: 'dev-1',
      publicKey: 'device-pub-1',
    );
    final currentDevice = AppCoreScope.sessionStore.device!;
    final networkId = await AppCoreScope.sessionController
        .ensureNetworkAvailableAndJoined(
      currentDevice: currentDevice,
      preferredNetworkId: 'net-1',
      fallbackNetworkName: 'home',
    );
    await AppCoreScope.sessionController.registerNode(
      deviceId: currentDevice.deviceId,
      nodeId: 'node-1',
      nodePublicKey: 'node-pub-1',
      bootstrapNetworkId: networkId,
    );

    await tester.pumpWidget(
      const MaterialApp(
        home: DevicesPage(),
      ),
    );
    await tester.pumpAndSettle();

    await tester.ensureVisible(find.byKey(AppTestKeys.devicesPeerNodeIdField));
    await tester.enterText(
      find.byKey(AppTestKeys.devicesPeerNodeIdField),
      'peer-1',
    );
    await tester.ensureVisible(find.byKey(AppTestKeys.devicesConnectButton));
    await tester.tap(find.byKey(AppTestKeys.devicesConnectButton));
    await tester.pumpAndSettle();

    expect(find.text('Connection Guidance'), findsOneWidget);
    expect(
      find.text('direct direct_udp $kDevPeerEndpointAddress'),
      findsOneWidget,
    );
    expect(find.text('matched'), findsWidgets);
    expect(find.textContaining('Control plane suggested direct direct_udp'), findsOneWidget);
  });

  testWidgets('DevicesPage renders runtime snapshot from injected tunnel gateway', (
    WidgetTester tester,
  ) async {
    tester.view.physicalSize = const Size(1440, 1400);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.reset);

    final gateway = _FakeTunnelHostGateway();
    AppCoreScope.configureForTest(
      appCoreApi: MockAppCoreApi(),
      tunnelHostGateway: gateway,
    );
    addTearDown(AppCoreScope.resetForTest);

    await tester.pumpWidget(
      const MaterialApp(
        home: DevicesPage(),
      ),
    );
    await tester.pumpAndSettle();

    await tester.ensureVisible(find.byKey(AppTestKeys.devicesTunnelViewButton));
    await tester.tap(find.byKey(AppTestKeys.devicesTunnelViewButton));
    await tester.pumpAndSettle();

    expect(gateway.lastRuntimePeerVirtualIp, '10.0.0.2');
    expect(find.text('Tunnel Runtime'), findsOneWidget);
    expect(find.text('state configured'), findsWidgets);
    expect(find.text('backend started'), findsWidgets);
    expect(find.text('engine loopback'), findsWidgets);
    expect(find.text('100.64.0.2'), findsWidgets);
    expect(find.text('wireguardkit / started'), findsWidgets);
    expect(find.text('203.0.113.10:51820'), findsWidgets);
  });
}

class _FakeTunnelHostGateway extends TunnelHostGateway {
  String? lastRuntimePeerVirtualIp;

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
  Future<WireGuardTunnelRuntimeView?> tunnelRuntimeView(String peerVirtualIp) async {
    lastRuntimePeerVirtualIp = peerVirtualIp;
    return WireGuardTunnelRuntimeView.fromJson({
      'state': 'configured',
      'transport': 'relay',
      'debugEngineMode': 'loopback',
      'backendName': 'wireguardkit',
      'backendState': 'started',
      'backendPeerVirtualIp': '100.64.0.2',
      'backendSelectedEndpoint': '203.0.113.10:51820',
      'peerVirtualIp': '100.64.0.2',
      'peerPublicKey': 'peer-debug-public-key',
      'selectedEndpoint': '203.0.113.10:51820',
      'interfaceName': 'utun9',
      'localVirtualIp': '100.64.0.10',
      'remoteAddress': '203.0.113.10:51820',
      'packetRxCount': 3,
      'packetRxBytes': 192,
      'packetTxCount': 3,
      'packetTxBytes': 192,
      'lastAppliedAtMs': 1712345678000,
    });
  }
}
