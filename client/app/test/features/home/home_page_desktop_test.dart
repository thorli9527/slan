import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:slan_app/features/home/home_page.dart';
import 'package:slan_app/infra/app_core/api/mock_app_core_api.dart';
import 'package:slan_app/infra/app_core/models/identity_models.dart';
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
    expect(find.text('Server Config'), findsOneWidget);
    expect(find.byKey(AppTestKeys.homeLoginButton), findsOneWidget);
    expect(find.byKey(AppTestKeys.homeSettingsButton), findsOneWidget);
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

    expect(find.text('Current Session'), findsOneWidget);
    expect(find.text('Thor'), findsOneWidget);
    expect(find.byKey(AppTestKeys.homeEnableNetworkButton), findsOneWidget);
    expect(find.byKey(AppTestKeys.homeDisableNetworkButton), findsOneWidget);
    expect(find.byKey(AppTestKeys.homeDetailsButton), findsOneWidget);
    expect(find.byKey(AppTestKeys.homeLogoutButton), findsOneWidget);
  });
}
