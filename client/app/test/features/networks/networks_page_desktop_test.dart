import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:slan_app/features/networks/networks_page.dart';
import 'package:slan_app/testing/app_test_keys.dart';

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

    expect(find.text('Overlay Networks'), findsOneWidget);
    expect(find.text('Network Inventory'), findsOneWidget);
    expect(find.byKey(AppTestKeys.networksNameField), findsOneWidget);
    expect(find.byKey(AppTestKeys.networksCidrField), findsOneWidget);
    expect(find.byKey(AppTestKeys.networksCreateButton), findsOneWidget);
    expect(find.byKey(AppTestKeys.networksRefreshButton), findsOneWidget);
  });
}
