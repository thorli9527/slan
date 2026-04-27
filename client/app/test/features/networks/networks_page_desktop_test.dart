import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:slan_app/features/networks/networks_page.dart';
import 'package:slan_app/infra/app_core/api/mock_app_core_api.dart';
import 'package:slan_app/infra/app_core/models/identity_models.dart';
import 'package:slan_app/infra/app_core/models/network_models.dart';
import 'package:slan_app/infra/app_core/scope/app_core_scope.dart';
import 'package:slan_app/testing/app_test_keys.dart';

void main() {
  tearDown(AppCoreScope.resetForTest);

  testWidgets('NetworksPage renders desktop workspace on wide layouts', (
    WidgetTester tester,
  ) async {
    tester.view.physicalSize = const Size(1440, 1400);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.reset);

    await tester.pumpWidget(
      const MaterialApp(
        home: NetworksPage(),
      ),
    );
    await tester.pump();

    expect(find.text('Network Details'), findsOneWidget);
    expect(find.text('Current Network'), findsOneWidget);
    expect(
        find.text('No active network has been prepared yet.'), findsOneWidget);
  });

  testWidgets('NetworksPage joins by invite code', (
    WidgetTester tester,
  ) async {
    AppCoreScope.configureForTest(appCoreApi: MockAppCoreApi());
    _seedLoggedInDevice();

    await _pumpNetworksPage(tester);
    await tester.tap(find.byKey(AppTestKeys.networksOpenJoinDialogButton));
    await tester.pumpAndSettle();

    await tester.enterText(
      find.byKey(AppTestKeys.networksJoinKeyField),
      'join-key-1',
    );
    await tester.tap(find.byKey(AppTestKeys.networksJoinButton));
    await tester.pumpAndSettle();

    expect(find.text('Joined network'), findsWidgets);
    expect(AppCoreScope.sessionStore.selectedNetworkId, 'mock-key-network');
  });

  testWidgets('NetworksPage creates network from dialog', (
    WidgetTester tester,
  ) async {
    AppCoreScope.configureForTest(appCoreApi: MockAppCoreApi());
    _seedLoggedInDevice();

    await _pumpNetworksPage(tester);
    await tester.tap(find.byKey(AppTestKeys.networksOpenCreateDialogButton));
    await tester.pumpAndSettle();

    await tester.enterText(
        find.byKey(AppTestKeys.networksIpAddressField), '10.9.0.0');
    await tester.enterText(
        find.byKey(AppTestKeys.networksSubnetMaskField), '255.255.252.0');
    await tester.tap(find.byKey(AppTestKeys.networksCreateButton));
    await tester.pumpAndSettle();

    expect(find.text('My Network'), findsWidgets);
    expect(AppCoreScope.sessionStore.selectedNetworkId, 'net-1');
  });

  testWidgets('NetworksPage switches selected network', (
    WidgetTester tester,
  ) async {
    AppCoreScope.configureForTest(appCoreApi: MockAppCoreApi());
    _seedLoggedInDevice(
      networks: const [
        NetworkModel(networkId: 'net-1', name: 'Home', cidr: '10.0.0.0/16'),
        NetworkModel(networkId: 'net-2', name: 'Lab', cidr: '10.1.0.0/16'),
      ],
    );

    await _pumpNetworksPage(tester);

    await tester.tap(find.byKey(AppTestKeys.networksSwitchButton('net-2')));
    await tester.pumpAndSettle();

    expect(AppCoreScope.sessionStore.selectedNetworkId, 'net-2');
    expect(find.text('selected net-2'), findsOneWidget);
    expect(find.text('Lab'), findsWidgets);
  });
}

Future<void> _pumpNetworksPage(WidgetTester tester) async {
  tester.view.physicalSize = const Size(1440, 1400);
  tester.view.devicePixelRatio = 1.0;
  addTearDown(tester.view.reset);

  await tester.pumpWidget(
    const MaterialApp(
      home: NetworksPage(),
    ),
  );
  await tester.pump();
}

void _seedLoggedInDevice({List<NetworkModel> networks = const []}) {
  final store = AppCoreScope.sessionStore;
  store.session = const SessionModel(
    userId: 'user-1',
    accessToken: 'token-1',
    expiresIn: 3600,
  );
  store.device = const DeviceModel(
    deviceId: 'dev-1',
    name: 'thor-laptop',
    platform: 'windows',
    status: 'online',
  );
  store.networks = networks;
  store.syncSelectedNetworkId();
  store.emit();
}
