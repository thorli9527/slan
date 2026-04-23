import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:integration_test/integration_test.dart';
import 'package:slan_app/infra/app_core/bridge/bridge_app_core_api.dart';
import 'package:slan_app/infra/app_core/scope/app_core_scope.dart';

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

  testWidgets('devices page surfaces external tunnel backend runtime',
      (tester) async {
    final page = harness(tester);

    await page.pumpAndConnectRelayFallback();
    await page.applyTunnelConfiguration(
      localVirtualIp: '100.64.0.10',
      peerVirtualIp: '100.64.0.2',
      endpoint: '203.0.113.10:51820',
      debugEngineMode: 'external',
    );
    await page.bringTunnelUp();
    await page.viewTunnelRuntime();
    await page.scrollToStateCard();

    page.expectTunnelRuntime(
      state: 'configured',
      transport: 'relay',
      peerVirtualIp: '100.64.0.2',
      endpoint: '203.0.113.10:51820',
      interfaceName: 'utun9',
      debugEngineMode: 'external',
      backendName: 'wireguardkit',
      backendState: 'failed',
      backendLastError: 'WireGuardKit backend is not integrated yet',
      backendLastStartedAtMs: 1712345677000,
      backendPeerVirtualIp: '100.64.0.2',
      backendSelectedEndpoint: '203.0.113.10:51820',
    );

    expect(find.text('0 pkt / 0 B'), findsWidgets);

    host.expectTunnelApply(
      localVirtualIp: '100.64.0.10',
      peerVirtualIp: '100.64.0.2',
      endpoint: '203.0.113.10:51820',
      debugEngineMode: 'external',
    );
    host.expectTunnelLifecycle();
  });
}
