import 'package:client_core_plugin/client_core_plugin.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:integration_test/integration_test.dart';

void main() {
  IntegrationTestWidgetsFlutterBinding.ensureInitialized();

  testWidgets('macOS plugin creates a visible browser window', (_) async {
    final opened = await ClientCorePlugin().openExternalUrl(
      'http://47.245.40.231:24200?auth=login&source=client',
    );

    expect(opened, isTrue);
  });
}
