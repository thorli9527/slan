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

    expect(find.text('Desktop Access Gateway'), findsOneWidget);
    expect(find.text('Sign In'), findsOneWidget);
    expect(find.byKey(AppTestKeys.authServerConfigButton), findsOneWidget);
    await tester.tap(find.byKey(AppTestKeys.authServerConfigButton));
    await tester.pumpAndSettle();

    expect(find.text('Server Config'), findsOneWidget);
    expect(find.text('Server Host'), findsOneWidget);
    expect(find.byKey(AppTestKeys.authHostField), findsOneWidget);
    expect(find.byKey(AppTestKeys.authApplyHostButton), findsOneWidget);
    expect(find.byKey(AppTestKeys.authOpenLoginButton), findsOneWidget);
    expect(find.byKey(AppTestKeys.authOpenConsoleButton), findsOneWidget);
  });
}
