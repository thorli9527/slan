import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:slan_app/features/home/home_page.dart';
import 'package:slan_app/infra/app_core/api/mock_app_core_api.dart';
import 'package:slan_app/infra/app_core/models/identity_models.dart';
import 'package:slan_app/infra/app_core/models/network_models.dart';
import 'package:slan_app/infra/app_core/scope/app_core_scope.dart';
import 'package:slan_app/testing/app_test_keys.dart';

void main() {
  testWidgets('HomePage renders minimal logged-out actions on wide layouts', (
    WidgetTester tester,
  ) async {
    tester.view.physicalSize = const Size(1440, 1400);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.reset);

    AppCoreScope.configureForTest(appCoreApi: MockAppCoreApi());
    addTearDown(AppCoreScope.resetForTest);

    await tester.pumpWidget(
      const MaterialApp(
        home: HomePage(enableAutoSetup: false),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('Login'), findsOneWidget);
    expect(find.byKey(AppTestKeys.homeLoginButton), findsOneWidget);
    expect(find.byKey(AppTestKeys.homeSettingsButton), findsOneWidget);

    await tester.tap(find.byKey(AppTestKeys.homeSettingsButton));
    await tester.pumpAndSettle();

    expect(find.text('Server Host'), findsWidgets);
    expect(find.text('Save'), findsOneWidget);
  });

  testWidgets('HomePage renders minimal logged-in summary actions', (
    WidgetTester tester,
  ) async {
    tester.view.physicalSize = const Size(1440, 1400);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.reset);

    AppCoreScope.configureForTest(appCoreApi: MockAppCoreApi());
    addTearDown(AppCoreScope.resetForTest);

    AppCoreScope.sessionStore.session = SessionModel(
      userId: 'user-1',
      accessToken: 'token-1',
      expiresIn: 3600,
      userLabel: 'Thor',
      authenticatedAtMs: DateTime(2026, 4, 23, 16, 50).millisecondsSinceEpoch,
    );
    AppCoreScope.sessionStore.emit();

    await tester.pumpWidget(
      const MaterialApp(
        home: HomePage(enableAutoSetup: false),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('Thor'), findsOneWidget);
    expect(find.byKey(AppTestKeys.homeNetworkSwitchButton), findsOneWidget);
    expect(find.byKey(AppTestKeys.homeDetailsButton), findsOneWidget);
    expect(find.byKey(AppTestKeys.homeLogoutButton), findsOneWidget);
  });

  testWidgets('HomePage blocks enable when current device is disabled', (
    WidgetTester tester,
  ) async {
    tester.view.physicalSize = const Size(1440, 1400);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.reset);

    AppCoreScope.configureForTest(appCoreApi: MockAppCoreApi());
    addTearDown(AppCoreScope.resetForTest);

    AppCoreScope.sessionStore.session = SessionModel(
      userId: 'user-1',
      accessToken: 'token-1',
      expiresIn: 3600,
      userLabel: 'Thor',
      authenticatedAtMs: DateTime(2026, 4, 23, 16, 50).millisecondsSinceEpoch,
      deviceId: 'dev-1',
    );
    AppCoreScope.sessionStore.device = const DeviceModel(
      deviceId: 'dev-1',
      name: 'desktop',
      platform: 'windows',
      status: 'offline',
    );
    AppCoreScope.sessionStore.networks = const [
      NetworkModel(
        networkId: 'net-1',
        name: 'office',
        cidr: '10.0.0.0/24',
        members: [
          NetworkMemberModel(
            deviceId: 'dev-1',
            role: 'member',
            status: 'disabled',
          ),
        ],
      ),
    ];
    AppCoreScope.sessionStore.selectedNetworkId = 'net-1';
    AppCoreScope.sessionStore.emit();

    await tester.pumpWidget(
      const MaterialApp(
        home: HomePage(enableAutoSetup: false),
      ),
    );
    await tester.pumpAndSettle();

    await tester.tap(find.byKey(AppTestKeys.homeNetworkSwitchButton));
    await tester.pumpAndSettle();

    expect(find.text('设备不可用'), findsOneWidget);
    expect(
      find.text('当前设备已被网络管理员停用，请联系管理员重新启用后再连接。'),
      findsOneWidget,
    );
  });
}
