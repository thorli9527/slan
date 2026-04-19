import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:slan_app/features/shared/desktop_client_widgets.dart';

void main() {
  Widget wrapForTest(Widget child) {
    return MaterialApp(
      home: Scaffold(
        body: child,
      ),
    );
  }

  testWidgets('DesktopHeroPanel renders title, description, and footer', (
    WidgetTester tester,
  ) async {
    await tester.pumpWidget(
      wrapForTest(
        const DesktopHeroPanel(
          title: 'Workspace',
          description: 'Desktop shell status',
          footer: Text('Footer actions'),
        ),
      ),
    );

    expect(find.text('Workspace'), findsOneWidget);
    expect(find.text('Desktop shell status'), findsOneWidget);
    expect(find.text('Footer actions'), findsOneWidget);
  });

  testWidgets('DesktopNavigationItem reflects selected state and label', (
    WidgetTester tester,
  ) async {
    await tester.pumpWidget(
      wrapForTest(
        const DesktopNavigationItem(
          icon: Icons.hub_outlined,
          label: 'Networks',
          selected: true,
          onTap: _noop,
        ),
      ),
    );

    expect(find.text('Networks'), findsOneWidget);
    expect(find.byIcon(Icons.hub_outlined), findsOneWidget);
  });

  testWidgets('DesktopWorkspaceFrame renders child content', (
    WidgetTester tester,
  ) async {
    await tester.pumpWidget(
      wrapForTest(
        const DesktopWorkspaceFrame(
          child: Text('Embedded workspace'),
        ),
      ),
    );

    expect(find.text('Embedded workspace'), findsOneWidget);
  });

  testWidgets('DesktopKeyValueList renders key value entries', (
    WidgetTester tester,
  ) async {
    await tester.pumpWidget(
      wrapForTest(
        const DesktopKeyValueList(
          entries: [
            DesktopKeyValueEntry(label: 'User', value: 'thor'),
            DesktopKeyValueEntry(label: 'Device', value: 'mac-1'),
          ],
        ),
      ),
    );

    expect(find.text('User'), findsOneWidget);
    expect(find.text('thor'), findsOneWidget);
    expect(find.text('Device'), findsOneWidget);
    expect(find.text('mac-1'), findsOneWidget);
  });
}

void _noop() {}
