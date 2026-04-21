import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:slan_app/features/networks/networks_page.dart';

void main() {
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
}
