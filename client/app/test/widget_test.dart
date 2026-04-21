import 'package:flutter_test/flutter_test.dart';
import 'package:slan_app/app/app.dart';

void main() {
  testWidgets('SlanApp defaults to the login page before a session exists', (
    WidgetTester tester,
  ) async {
    await tester.pumpWidget(const SlanApp());

    expect(find.text('登录'), findsOneWidget);
    expect(find.text('设置'), findsOneWidget);
  });
}
