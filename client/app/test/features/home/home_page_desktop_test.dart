import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:slan_app/features/home/home_page.dart';
import 'package:slan_app/testing/app_test_keys.dart';

void main() {
  testWidgets('HomePage renders desktop shell on wide layouts', (
    WidgetTester tester,
  ) async {
    tester.view.physicalSize = const Size(1440, 1400);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.reset);

    await tester.pumpWidget(
      const MaterialApp(
        home: HomePage(),
      ),
    );

    expect(find.byKey(AppTestKeys.authTab), findsOneWidget);
    expect(find.byKey(AppTestKeys.networksTab), findsOneWidget);
    expect(find.byKey(AppTestKeys.devicesTab), findsOneWidget);
    expect(find.text('Mac desktop control surface'), findsOneWidget);
    expect(find.text('Control Plane'), findsOneWidget);
    expect(find.text('Overlay'), findsOneWidget);
    expect(find.text('Data Plane'), findsOneWidget);
    expect(find.text('Desktop Shell'), findsOneWidget);
  });
}
