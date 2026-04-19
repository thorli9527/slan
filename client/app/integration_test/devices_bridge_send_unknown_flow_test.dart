import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:integration_test/integration_test.dart';
import 'package:slan_app/infra/app_core/bridge/app_core_bridge.dart';
import 'package:slan_app/infra/app_core/scope/app_core_scope.dart';
import 'package:slan_app/infra/app_core/bridge/bridge_app_core_api.dart';

import 'devices_flow_test_support.dart';

void main() {
  IntegrationTestWidgetsFlutterBinding.ensureInitialized();

  const channel = MethodChannel('slan/app_core');

  late FakeHost host;

  setUp(() {
    host = FakeHost();
    TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
        .setMockMethodCallHandler(channel, host.handle);
    AppCoreScope.configureForTest(
      appCoreApi: BridgeAppCoreApi(bridge: MethodChannelAppCoreBridge()),
    );
  });

  tearDown(() async {
    TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
        .setMockMethodCallHandler(channel, null);
    AppCoreScope.resetForTest();
    await cleanupMacOsIntegrationApp();
  });

  DevicesPageHarness harness(WidgetTester tester) => DevicesPageHarness(tester);

  testWidgets('devices page surfaces generic send failures as unknown',
      (tester) async {
    host.failSend = true;
    host.sendFailureCode = 'send_failed';
    host.sendFailureMessage = 'send failed unexpectedly';
    final page = harness(tester);

    await page.pumpAndConnectRelayFallback();
    await page.sendPayload('hello');
    await page.scrollToStateCard();

    page.expectSendFailure(kind: 'unknown', detail: 'send failed unexpectedly');
  });
}
