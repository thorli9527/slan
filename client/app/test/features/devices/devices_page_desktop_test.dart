import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:slan_app/features/devices/devices_page.dart';
import 'package:slan_app/infra/app_core/api/mock_app_core_api.dart';
import 'package:slan_app/infra/app_core/scope/app_core_scope.dart';
import 'package:slan_app/infra/app_core/api/dev_defaults.dart';
import 'package:slan_app/testing/app_test_keys.dart';

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
}
