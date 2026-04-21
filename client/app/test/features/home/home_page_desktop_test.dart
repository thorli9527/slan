import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:slan_app/features/home/home_page.dart';
import 'package:slan_app/infra/app_core/api/mock_app_core_api.dart';
import 'package:slan_app/infra/app_core/scope/app_core_scope.dart';
import 'package:slan_app/testing/app_test_keys.dart';

void main() {
  testWidgets('HomePage renders desktop shell on wide layouts', (
    WidgetTester tester,
  ) async {
    tester.view.physicalSize = const Size(1440, 1400);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.reset);

    AppCoreScope.configureForTest(appCoreApi: MockAppCoreApi());
    addTearDown(AppCoreScope.resetForTest);

    await tester.pumpWidget(
      const MaterialApp(
        home: HomePage(),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('登录'), findsOneWidget);
    expect(find.text('设置'), findsOneWidget);
    expect(find.byKey(AppTestKeys.homeLoginButton), findsOneWidget);
    expect(find.byKey(AppTestKeys.homeSettingsButton), findsOneWidget);
  });
}
