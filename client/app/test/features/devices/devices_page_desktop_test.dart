import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:slan_app/application/mqtt_control_task_service.dart';
import 'package:slan_app/application/network_enable_preflight_service.dart';
import 'package:slan_app/application/tunnel_host_gateway.dart';
import 'package:slan_app/features/devices/devices_page.dart';
import 'package:slan_app/infra/app_core/api/app_core_api.dart';
import 'package:slan_app/infra/app_core/api/mock_app_core_api.dart';
import 'package:slan_app/infra/app_core/models/models.dart';
import 'package:slan_app/infra/app_core/scope/app_core_scope.dart';
import 'package:slan_app/infra/app_core/api/dev_defaults.dart';
import 'package:slan_app/testing/app_test_keys.dart';
import 'package:slan_app_core_plugin/slan_app_core_plugin.dart';
import 'package:shared_preferences/shared_preferences.dart';

void main() {
  testWidgets('DevicesPage renders desktop workbench sections', (
    WidgetTester tester,
  ) async {
    await _pumpDevicesPageWithMockAppCore(tester);

    expect(find.text('Devices Workspace'), findsOneWidget);
    expect(find.text('Identity'), findsOneWidget);
    expect(find.text('Connectivity'), findsOneWidget);
    expect(find.text('Diagnostics'), findsOneWidget);
    expect(find.text('WireGuard Tunnel Debug'), findsOneWidget);
    expect(find.text('Runtime State'), findsOneWidget);
    expect(find.byKey(AppTestKeys.devicesStateCard), findsOneWidget);
  });

  testWidgets('DevicesPage shows platform doctor results', (
    WidgetTester tester,
  ) async {
    await _pumpDevicesPageWithMockAppCore(tester);

    await tester.ensureVisible(
      find.byKey(AppTestKeys.devicesPlatformDoctorButton),
    );
    await tester.tap(find.byKey(AppTestKeys.devicesPlatformDoctorButton));
    await tester.pumpAndSettle();

    expect(find.text('Run Doctor'), findsOneWidget);
    expect(
        find.textContaining('mock / family mock / pkg mock'), findsOneWidget);
    expect(find.textContaining('in-memory / memory / memory'), findsOneWidget);
    expect(find.textContaining('mock_backend:ok'), findsOneWidget);
  });

  testWidgets(
      'DevicesPage shows control plan guidance after quick setup and connect', (
    WidgetTester tester,
  ) async {
    await _pumpDevicesPageWithMockAppCore(tester);

    await AppCoreScope.sessionController.registerDevice(
      name: 'thor-mac',
      platform: 'macos',
      machineId: 'dev-1',
      publicKey: 'device-pub-1',
    );
    final currentDevice = AppCoreScope.sessionStore.device!;
    final networkId =
        await AppCoreScope.sessionController.ensureNetworkAvailableAndJoined(
      currentDevice: currentDevice,
      preferredNetworkId: 'net-1',
      fallbackNetworkName: 'home',
    );
    await AppCoreScope.sessionController.registerNode(
      deviceId: currentDevice.deviceId,
      nodeId: 'node-1',
      nodePublicKey: 'node-pub-1',
      bootstrapNetworkId: networkId,
    );

    await tester.ensureVisible(find.byKey(AppTestKeys.devicesPeerNodeIdField));
    await tester.enterText(
      find.byKey(AppTestKeys.devicesPeerNodeIdField),
      'peer-1',
    );
    await tester.ensureVisible(find.byKey(AppTestKeys.devicesConnectButton));
    await tester.tap(find.byKey(AppTestKeys.devicesConnectButton));
    await tester.pumpAndSettle();

    expect(find.text('Connection Guidance'), findsOneWidget);
    expect(
      find.text('direct direct_udp $kDevPeerEndpointAddress'),
      findsOneWidget,
    );
    expect(find.text('matched'), findsWidgets);
    expect(find.textContaining('Control plane suggested direct direct_udp'),
        findsOneWidget);
  });

  testWidgets(
      'DevicesPage renders runtime snapshot from injected tunnel gateway', (
    WidgetTester tester,
  ) async {
    final gateway = _FakeTunnelHostGateway.healthy();
    await _pumpDevicesPageWithGateway(tester, gateway);
    await _inspectTunnelRuntime(tester);

    expect(gateway.lastRuntimePeerVirtualIp, '10.0.0.2');
    expect(find.text('Tunnel Runtime'), findsOneWidget);
    expect(find.text('state up'), findsWidgets);
    expect(find.text('backend started'), findsWidgets);
    expect(find.text('engine loopback'), findsWidgets);
    expect(find.text('100.64.0.2'), findsWidgets);
    expect(find.text('wireguardkit / started'), findsWidgets);
    expect(find.text('203.0.113.10:51820'), findsWidgets);
  });

  testWidgets(
      'DevicesPage derives healthy guidance from injected runtime snapshot', (
    WidgetTester tester,
  ) async {
    final gateway = _FakeTunnelHostGateway.healthy();
    await _pumpDevicesPageWithGateway(tester, gateway);
    await _inspectTunnelRuntime(tester);

    expect(find.text('Health Guidance'), findsWidgets);
    expect(find.text('Recent checks'), findsWidgets);
    expect(
      find.textContaining(
        'The tunnel backend is running, an endpoint is selected, and traffic has been observed.',
      ),
      findsWidgets,
    );
    expect(
      find.textContaining(
        'Recommended action: The session looks healthy.',
      ),
      findsWidgets,
    );
    expect(find.text('Run Bring Down'), findsOneWidget);
  });

  testWidgets(
      'DevicesPage derives degraded guidance when runtime has no endpoint', (
    WidgetTester tester,
  ) async {
    final gateway = _FakeTunnelHostGateway.missingEndpoint();
    await _pumpDevicesPageWithGateway(tester, gateway);
    await _inspectTunnelRuntime(tester);

    expect(
      find.textContaining(
        'The session is running, but runtime has not selected an endpoint yet.',
      ),
      findsWidgets,
    );
    expect(
      find.textContaining(
        'Refresh runtime now to confirm whether endpoint selection is still missing.',
      ),
      findsWidgets,
    );
    expect(find.text('View Runtime'), findsWidgets);
  });

  testWidgets(
      'DevicesPage derives failed guidance when backend reports failure', (
    WidgetTester tester,
  ) async {
    final gateway = _FakeTunnelHostGateway.failedBackend();
    await _pumpDevicesPageWithGateway(tester, gateway);
    await _inspectTunnelRuntime(tester);

    expect(
      find.textContaining('The tunnel backend reports a failed state.'),
      findsWidgets,
    );
    expect(
      find.textContaining(
        'Inspect runtime first, then re-apply configuration if the backend still looks failed.',
      ),
      findsWidgets,
    );
    expect(find.text('Apply Tunnel Again'), findsWidgets);
  });

  testWidgets(
      'DevicesPage derives staged guidance when backend is not started yet', (
    WidgetTester tester,
  ) async {
    final gateway = _FakeTunnelHostGateway.notStartedYet();
    await _pumpDevicesPageWithGateway(tester, gateway);
    await _inspectTunnelRuntime(tester);

    expect(
      find.textContaining(
        'Configuration has been staged, but the backend does not look fully started yet.',
      ),
      findsWidgets,
    );
    expect(
      find.textContaining(
        'Bring the tunnel up again, then inspect runtime.',
      ),
      findsWidgets,
    );
    expect(find.text('Run Bring Up'), findsOneWidget);
  });

  testWidgets(
      'DevicesPage derives no-traffic guidance when runtime is up but idle', (
    WidgetTester tester,
  ) async {
    final gateway = _FakeTunnelHostGateway.noTraffic();
    await _pumpDevicesPageWithGateway(tester, gateway);
    await _inspectTunnelRuntime(tester);

    expect(
      find.textContaining(
        'The session is up and an endpoint is selected, but no traffic has been observed yet.',
      ),
      findsWidgets,
    );
    expect(
      find.textContaining(
        'Inspect runtime first. Reconfigure only if traffic stays idle for multiple checks.',
      ),
      findsWidgets,
    );
    expect(find.text('View Runtime'), findsWidgets);
  });

  testWidgets('DevicesPage uses service runtime actions in bridge mode', (
    WidgetTester tester,
  ) async {
    SharedPreferences.setMockInitialValues({});
    final appCore = _BridgeRuntimeMockAppCoreApi();
    final gateway = _ThrowingTunnelHostGateway();
    final mqttTasks = _RecordingMqttControlTaskService();
    await _pumpDevicesPageWithScope(
      tester,
      appCoreApi: appCore,
      tunnelHostGateway: gateway,
      mqttControlTaskService: mqttTasks,
      networkEnablePreflightService: _StaticNetworkEnablePreflightService(
        appCore,
      ),
      mode: 'bridge',
    );

    AppCoreScope.sessionStore.session = const SessionModel(
      userId: 'user-1',
      accessToken: 'token-1',
      expiresIn: 3600,
      deviceId: 'dev-1',
    );
    AppCoreScope.sessionStore.device = appCore.device;
    AppCoreScope.sessionStore.node = appCore.node;
    AppCoreScope.sessionStore.networks = appCore.networks;
    AppCoreScope.sessionStore.selectedNetworkId = 'net-1';
    AppCoreScope.sessionStore.emit();
    await tester.pumpAndSettle();

    expect(find.text('Service Network Runtime'), findsOneWidget);
    expect(find.text('Enable Network'), findsOneWidget);
    expect(find.text('Sync State'), findsOneWidget);
    expect(find.text('Disable Network'), findsOneWidget);
    expect(find.text('Local Virtual IP'), findsNothing);
    expect(find.text('Interface Private Key'), findsNothing);
    expect(find.text('Peer Endpoint'), findsNothing);

    await tester
        .ensureVisible(find.byKey(AppTestKeys.devicesTunnelApplyButton));
    await tester.tap(find.byKey(AppTestKeys.devicesTunnelApplyButton));
    await tester.pumpAndSettle();
    await tester.pump(const Duration(milliseconds: 100));
    await tester.pumpAndSettle();

    expect(appCore.enableLocalNetworkCalls, 0);
    expect(mqttTasks.taskTypes, contains('enable_network'));
    expect(gateway.tunnelActionCalls, 0);
    expect(find.text('Enable network'), findsWidgets);
    var usageState = await AppCoreScope.readNetworkUsageState('user-1');
    expect(usageState, isNotNull);
    expect(usageState!.enabled, isTrue);
    expect(usageState.networkId, 'net-1');

    final syncButton = tester.widget<OutlinedButton>(
      find.widgetWithText(OutlinedButton, 'Sync State'),
    );
    expect(syncButton.onPressed, isNotNull);
    syncButton.onPressed!();
    await tester.pumpAndSettle();

    expect(appCore.controlSyncCalls, 1);
    expect(gateway.tunnelActionCalls, 0);
    expect(find.text('Sync service state'), findsWidgets);

    final disableButton = tester.widget<OutlinedButton>(
      find.widgetWithText(OutlinedButton, 'Disable Network'),
    );
    expect(disableButton.onPressed, isNotNull);
    disableButton.onPressed!();
    await tester.pumpAndSettle();

    expect(appCore.disableLocalNetworkCalls, 0);
    expect(mqttTasks.taskTypes, contains('disable_network'));
    expect(gateway.tunnelActionCalls, 0);
    expect(find.text('Disable network'), findsWidgets);
    usageState = await AppCoreScope.readNetworkUsageState('user-1');
    expect(usageState, isNotNull);
    expect(usageState!.enabled, isFalse);
    expect(usageState.networkId, 'net-1');
  });

  testWidgets('DevicesPage quick setup uses service runtime in bridge mode', (
    WidgetTester tester,
  ) async {
    SharedPreferences.setMockInitialValues({});
    final appCore = _BridgeRuntimeMockAppCoreApi();
    final gateway = _ThrowingTunnelHostGateway();
    final mqttTasks = _RecordingMqttControlTaskService();
    await _pumpDevicesPageWithScope(
      tester,
      appCoreApi: appCore,
      tunnelHostGateway: gateway,
      mqttControlTaskService: mqttTasks,
      networkEnablePreflightService: _StaticNetworkEnablePreflightService(
        appCore,
      ),
      mode: 'bridge',
    );

    AppCoreScope.sessionStore.session = const SessionModel(
      userId: 'user-1',
      accessToken: 'token-1',
      expiresIn: 3600,
    );
    AppCoreScope.sessionStore.emit();
    await tester.pumpAndSettle();

    await tester.ensureVisible(find.text('Quick Setup Client'));
    await tester.tap(find.text('Quick Setup Client'));
    await tester.pumpAndSettle();

    expect(appCore.enableLocalNetworkCalls, 0);
    expect(appCore.controlSyncCalls, 0);
    expect(mqttTasks.taskTypes, contains('enable_network'));
    expect(gateway.tunnelActionCalls, 0);
    expect(find.text('Quick setup client'), findsWidgets);
    final usageState = await AppCoreScope.readNetworkUsageState('user-1');
    expect(usageState, isNotNull);
    expect(usageState!.enabled, isTrue);
    expect(usageState.networkId, 'net-1');
  });
}

void _configureDesktopViewport(WidgetTester tester) {
  tester.view.physicalSize = const Size(1440, 1400);
  tester.view.devicePixelRatio = 1.0;
  addTearDown(tester.view.reset);
}

Future<void> _pumpDevicesPage(
  WidgetTester tester, {
  bool configureViewport = true,
}) async {
  if (configureViewport) {
    _configureDesktopViewport(tester);
  }
  await tester.pumpWidget(
    const MaterialApp(
      home: DevicesPage(),
    ),
  );
  await tester.pumpAndSettle();
}

Future<void> _pumpDevicesPageWithMockAppCore(WidgetTester tester) async {
  await _pumpDevicesPageWithScope(
    tester,
    appCoreApi: MockAppCoreApi(),
    mode: 'mock',
  );
}

Future<void> _pumpDevicesPageWithGateway(
  WidgetTester tester,
  TunnelHostGateway gateway,
) async {
  await _pumpDevicesPageWithScope(
    tester,
    appCoreApi: MockAppCoreApi(),
    tunnelHostGateway: gateway,
    mode: 'mock',
  );
}

Future<void> _pumpDevicesPageWithScope(
  WidgetTester tester, {
  required AppCoreApi appCoreApi,
  TunnelHostGateway? tunnelHostGateway,
  MqttControlTaskService? mqttControlTaskService,
  NetworkEnablePreflightService? networkEnablePreflightService,
  String? mode,
}) async {
  _configureDesktopViewport(tester);
  AppCoreScope.configureForTest(
    appCoreApi: appCoreApi,
    tunnelHostGateway: tunnelHostGateway,
    mqttControlTaskService: mqttControlTaskService,
    networkEnablePreflightService: networkEnablePreflightService,
    mode: mode,
  );
  addTearDown(AppCoreScope.resetForTest);
  await _pumpDevicesPage(tester, configureViewport: false);
}

class _BridgeRuntimeMockAppCoreApi extends MockAppCoreApi {
  _BridgeRuntimeMockAppCoreApi();

  int enableLocalNetworkCalls = 0;
  int controlSyncCalls = 0;
  int disableLocalNetworkCalls = 0;

  final DeviceModel device = const DeviceModel(
    deviceId: 'dev-1',
    name: 'desktop',
    platform: 'windows',
    status: 'reachable',
    virtualIp: '10.0.0.2',
    publicKey: 'device-pub-1',
    networkIds: ['net-1'],
  );

  final NodeModel node = const NodeModel(
    nodeId: 'node-1',
    deviceId: 'dev-1',
    nodePublicKey: 'node-pub-1',
    networkIds: ['net-1'],
  );

  final List<NetworkModel> networks = const [
    NetworkModel(
      networkId: 'net-1',
      name: 'default',
      cidr: '10.0.0.0/16',
      members: [
        NetworkMemberModel(
          deviceId: 'dev-1',
          role: 'owner',
          status: 'enabled',
          virtualIp: '10.0.0.2',
        ),
      ],
    ),
  ];

  @override
  Future<BootstrapModel?> enableLocalNetwork({String? networkId}) async {
    enableLocalNetworkCalls += 1;
    return _bootstrap();
  }

  @override
  Future<bool> disableLocalNetwork({String? networkId}) async {
    disableLocalNetworkCalls += 1;
    return true;
  }

  @override
  Future<BootstrapModel> controlSync({
    required String nodeId,
    required String networkId,
  }) async {
    controlSyncCalls += 1;
    return _bootstrap();
  }

  BootstrapModel _bootstrap() {
    return BootstrapModel(
      device: device,
      networks: networks,
      controlPlane: const ControlPlaneConfigModel(
        wsUrl: kDevControlMqttUrl,
        heartbeatSeconds: 15,
        sessionToken: 'session-token',
      ),
      stunServers: const [kDevStunServer],
      relay: const RelayConfigModel(
        defaultClusterId: 'local',
        countries: [],
      ),
    );
  }

  @override
  Future<ControlStatusModel> controlStatus() async => const ControlStatusModel(
        status: 'configured',
        wsUrl: kDevControlMqttUrl,
        heartbeatSeconds: 15,
        sessionTokenPresent: true,
        networkMapPresent: true,
        networkId: 'net-1',
        nodeId: 'node-1',
        deviceId: 'dev-1',
        peerCount: 0,
        connectPlanCount: 0,
        connectPlans: [],
      );
}

class _RecordingMqttControlTaskService implements MqttControlTaskService {
  final List<String> taskTypes = [];
  final List<String> consumedTaskIds = [];

  @override
  Future<void> enqueueNetworkTask({
    required String taskType,
    required String networkId,
    required String deviceId,
    required String uiRefreshReason,
  }) async {
    taskTypes.add(taskType);
  }

  @override
  void markUiRefreshConsumed(String? taskId) {
    if (taskId != null) {
      consumedTaskIds.add(taskId);
    }
  }
}

class _StaticNetworkEnablePreflightService
    implements NetworkEnablePreflightService {
  const _StaticNetworkEnablePreflightService(this.appCore);

  final _BridgeRuntimeMockAppCoreApi appCore;

  @override
  Future<NetworkEnablePreflightResult> verifyRemoteNetwork({
    required AppCoreApi remoteApi,
    required SessionModel? session,
    required DeviceModel? currentDevice,
    required String? preferredNetworkId,
  }) async {
    return NetworkEnablePreflightResult(
      devices: [appCore.device],
      networks: appCore.networks,
      device: appCore.device,
      selectedNetworkId: preferredNetworkId ?? 'net-1',
      assignedVirtualIp: appCore.device.virtualIp ?? '10.0.0.2',
    );
  }
}

class _ThrowingTunnelHostGateway extends TunnelHostGateway {
  int tunnelActionCalls = 0;

  Never _unexpected() {
    tunnelActionCalls += 1;
    throw StateError(
        'Flutter tunnel gateway should not be used in bridge mode');
  }

  @override
  Future<WireGuardTunnelActionResult> applyTunnelConfiguration(
    WireGuardTunnelConfiguration configuration,
  ) async =>
      _unexpected();

  @override
  Future<WireGuardTunnelActionResult> bringTunnelDown() async => _unexpected();

  @override
  Future<WireGuardTunnelActionResult> bringTunnelUp() async => _unexpected();

  @override
  Future<WireGuardTunnelActionResult> removeTunnelPeer(
    String peerVirtualIp,
  ) async =>
      _unexpected();

  @override
  Future<WireGuardTunnelRuntimeView?> tunnelRuntimeView(
    String peerVirtualIp,
  ) async =>
      _unexpected();
}

Future<void> _inspectTunnelRuntime(WidgetTester tester) async {
  await tester.ensureVisible(find.byKey(AppTestKeys.devicesTunnelViewButton));
  await tester.tap(find.byKey(AppTestKeys.devicesTunnelViewButton));
  await tester.pumpAndSettle();
}

class _FakeTunnelHostGateway extends TunnelHostGateway {
  _FakeTunnelHostGateway(this._runtimePayload);

  factory _FakeTunnelHostGateway.healthy() =>
      _FakeTunnelHostGateway(_healthyRuntimePayload);

  factory _FakeTunnelHostGateway.missingEndpoint() => _FakeTunnelHostGateway({
        ..._healthyRuntimePayload,
        'selectedEndpoint': null,
        'backendSelectedEndpoint': null,
        'remoteAddress': '',
      });

  factory _FakeTunnelHostGateway.failedBackend() => _FakeTunnelHostGateway({
        ..._healthyRuntimePayload,
        'state': 'configured',
        'backendState': 'failed',
        'backendLastError': 'WireGuard backend failed to start',
        'lastPacketAtMs': null,
        'packetRxCount': 0,
        'packetRxBytes': 0,
        'packetTxCount': 0,
        'packetTxBytes': 0,
      });

  factory _FakeTunnelHostGateway.notStartedYet() => _FakeTunnelHostGateway({
        ..._healthyRuntimePayload,
        'state': 'configured',
        'backendState': 'idle',
        'backendLastStartedAtMs': null,
        'lastPacketAtMs': null,
        'packetRxCount': 0,
        'packetRxBytes': 0,
        'packetTxCount': 0,
        'packetTxBytes': 0,
      });

  factory _FakeTunnelHostGateway.noTraffic() => _FakeTunnelHostGateway({
        ..._healthyRuntimePayload,
        'packetRxCount': 0,
        'packetRxBytes': 0,
        'packetTxCount': 0,
        'packetTxBytes': 0,
        'lastPacketAtMs': null,
      });

  final Map<Object?, Object?> _runtimePayload;
  String? lastRuntimePeerVirtualIp;

  @override
  Future<WireGuardTunnelActionResult> applyTunnelConfiguration(
    WireGuardTunnelConfiguration configuration,
  ) {
    throw UnimplementedError();
  }

  @override
  Future<WireGuardTunnelActionResult> bringTunnelDown() {
    throw UnimplementedError();
  }

  @override
  Future<WireGuardTunnelActionResult> bringTunnelUp() {
    throw UnimplementedError();
  }

  @override
  Future<WireGuardTunnelActionResult> removeTunnelPeer(String peerVirtualIp) {
    throw UnimplementedError();
  }

  @override
  Future<WireGuardTunnelRuntimeView?> tunnelRuntimeView(
      String peerVirtualIp) async {
    lastRuntimePeerVirtualIp = peerVirtualIp;
    final payload = Map<Object?, Object?>.from(_runtimePayload)
      ..['peerVirtualIp'] = peerVirtualIp;
    return WireGuardTunnelRuntimeView.fromJson(payload);
  }
}

const Map<Object?, Object?> _healthyRuntimePayload = {
  'state': 'up',
  'transport': 'relay',
  'debugEngineMode': 'loopback',
  'backendName': 'wireguardkit',
  'backendState': 'started',
  'backendLastStartedAtMs': 1712345677000,
  'backendPeerVirtualIp': '100.64.0.2',
  'backendSelectedEndpoint': '203.0.113.10:51820',
  'peerVirtualIp': '100.64.0.2',
  'peerPublicKey': 'peer-debug-public-key',
  'selectedEndpoint': '203.0.113.10:51820',
  'interfaceName': 'utun9',
  'localVirtualIp': '100.64.0.10',
  'remoteAddress': '203.0.113.10:51820',
  'packetRxCount': 3,
  'packetRxBytes': 192,
  'packetTxCount': 3,
  'packetTxBytes': 192,
  'lastPacketAtMs': 1712345678123,
  'lastAppliedAtMs': 1712345678000,
};
