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

  testWidgets('devices page surfaces generic probe failures as unknown',
      (tester) async {
    host.failProbe = true;
    host.probeFailureCode = 'probe_failed';
    host.probeFailureMessage = 'probe failed unexpectedly';
    final page = harness(tester);

    await page.pumpAndConnectRelayFallback();
    await page.probePayload('hello', timeoutMs: '7');
    await page.scrollToStateCard();

    page.expectProbeFailure(
      kind: 'unknown',
      detail: 'probe failed unexpectedly',
    );
  });
}
