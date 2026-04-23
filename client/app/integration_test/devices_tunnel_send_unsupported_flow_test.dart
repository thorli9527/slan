import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:integration_test/integration_test.dart';
import 'package:slan_app/infra/app_core/scope/app_core_scope.dart';
import 'package:slan_app/infra/app_core/bridge/bridge_app_core_api.dart';

import 'devices_flow_test_support.dart';

void main() {
  IntegrationTestWidgetsFlutterBinding.ensureInitialized();

  const channel = MethodChannel('slan/app_core');

  late FakeHost host;

  setUp(() async {
    await cleanupDesktopIntegrationApp();
    host = FakeHost();
    TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
        .setMockMethodCallHandler(channel, host.handle);
    AppCoreScope.configureForTest(
      appCoreApi: BridgeAppCoreApi(),
    );
  });

  tearDown(() async {
    TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
        .setMockMethodCallHandler(channel, null);
    AppCoreScope.resetForTest();
    await cleanupDesktopIntegrationApp();
  });

  DevicesPageHarness harness(WidgetTester tester) => DevicesPageHarness(tester);

  testWidgets('devices page surfaces send unsupported failures',
      (tester) async {
    host.failSend = true;
    host.sendFailureCode = 'send_unsupported_path';
    host.sendFailureMessage = 'active path does not support send';
    final page = harness(tester);

    await page.pumpAndConnectRelayFallback();
    await page.sendPayload('hello');
    await page.scrollToStateCard();

    page.expectSendFailure(
      kind: 'unsupported',
      detail: 'active path does not support send',
    );
  });
}
