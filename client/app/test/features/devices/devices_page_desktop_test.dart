import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:slan_app/features/devices/devices_page.dart';
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
}
