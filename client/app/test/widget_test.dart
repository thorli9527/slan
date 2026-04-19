import 'package:flutter_test/flutter_test.dart';
import 'package:slan_app/app/app.dart';

void main() {
  testWidgets('SlanApp renders primary home tabs on compact layout', (
    WidgetTester tester,
  ) async {
    await tester.pumpWidget(const SlanApp());

    expect(find.text('Auth'), findsOneWidget);
    expect(find.text('Networks'), findsOneWidget);
    expect(find.text('Devices'), findsOneWidget);
  });
}
