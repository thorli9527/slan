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

  testWidgets('devices page drives tunnel debug actions through bridge channel',
      (tester) async {
    final page = harness(tester);

    await page.pumpAndConnectRelayFallback();
    await page.applyTunnelConfiguration(
      localVirtualIp: '100.64.0.10',
      peerVirtualIp: '100.64.0.2',
      endpoint: '203.0.113.10:51820',
      debugEngineMode: 'loopback',
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
    );

    host.expectTunnelApply(
      localVirtualIp: '100.64.0.10',
      peerVirtualIp: '100.64.0.2',
      endpoint: '203.0.113.10:51820',
      debugEngineMode: 'loopback',
    );
    host.expectTunnelLifecycle();
    expect(find.text('tunnel packets tx: 3'), findsOneWidget);
    expect(find.text('tunnel bytes tx: 192'), findsOneWidget);
  });
}
