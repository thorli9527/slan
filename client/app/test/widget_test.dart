import 'package:flutter_test/flutter_test.dart';
import 'package:slan_app/app/app.dart';
import 'package:slan_app/testing/app_test_keys.dart';

void main() {
  testWidgets('SlanApp defaults to the login page before a session exists', (
    WidgetTester tester,
  ) async {
    await tester.pumpWidget(
      SlanApp(initialization: Future<void>.value()),
    );
    await tester.pumpAndSettle();

    expect(find.byKey(AppTestKeys.homeLoginButton), findsOneWidget);
    expect(find.byKey(AppTestKeys.homeSettingsButton), findsOneWidget);
  });
}
