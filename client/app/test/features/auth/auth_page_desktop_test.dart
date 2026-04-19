import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:slan_app/features/auth/auth_page.dart';
import 'package:slan_app/testing/app_test_keys.dart';

void main() {
  testWidgets('AuthPage renders desktop workspace on wide layouts', (
    WidgetTester tester,
  ) async {
    tester.view.physicalSize = const Size(1440, 1400);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.reset);

    await tester.pumpWidget(
      const MaterialApp(
        home: AuthPage(),
      ),
    );

    expect(find.text('Authentication'), findsOneWidget);
    expect(find.text('Session Controls'), findsOneWidget);
    expect(find.text('No session'), findsOneWidget);
    expect(find.byKey(AppTestKeys.authEmailField), findsOneWidget);
    expect(find.byKey(AppTestKeys.authPasswordField), findsOneWidget);
    expect(find.byKey(AppTestKeys.authRegisterButton), findsOneWidget);
    expect(find.byKey(AppTestKeys.authLoginButton), findsOneWidget);
  });
}
