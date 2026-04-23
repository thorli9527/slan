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

  setUp(() {
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
    await cleanupMacOsIntegrationApp();
  });

  DevicesPageHarness harness(WidgetTester tester) => DevicesPageHarness(tester);

  testWidgets(
      'devices page drives bootstrap then relay fallback through bridge channel',
      (tester) async {
    final page = harness(tester);

    await page.pumpAndConnectRelayFallback();

    expect(host.calls.any((call) => call.method == 'createNetwork'), isTrue);
    expect(host.calls.any((call) => call.method == 'registerDevice'), isTrue);

    await page.sendPayload('hello');
    await page.probePayload('hello', timeoutMs: '7');
    await page.scrollToStateCard();

    page.expectSendSuccess(bytes: 5);
    page.expectProbeSuccess(
      probeId: 'probe-1',
      bytes: 5,
      replyObserved: true,
      replyRttMs: 2,
    );

    host.expectRelayFallbackFlow();
    host.expectSendPayload('hello');
    host.expectProbePayload('hello', probeTimeoutMs: 7);
  });
}
