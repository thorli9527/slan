import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:slan_app/application/mqtt_control_task_service.dart';
import 'package:slan_app/application/network_enable_preflight_service.dart';
import 'package:slan_app/application/tunnel_host_gateway.dart';
import 'package:slan_app/infra/app_core/api/app_core_api.dart';
import 'package:slan_app/infra/app_core/api/mock_app_core_api.dart';
import 'package:slan_app/infra/app_core/api/dev_defaults.dart';
import 'package:slan_app/infra/app_core/models/models.dart';
import 'package:slan_app/infra/app_core/scope/app_core_scope.dart';
import 'package:slan_app_core_plugin/slan_app_core_plugin.dart';

void main() {
  test('configureForTest can override tunnel host gateway', () {
    final gateway = _FakeTunnelHostGateway();

    AppCoreScope.configureForTest(
      appCoreApi: MockAppCoreApi(),
      tunnelHostGateway: gateway,
    );
    addTearDown(AppCoreScope.resetForTest);

    expect(AppCoreScope.tunnelHostGateway, same(gateway));
  });

  test('resetForTest restores default tunnel host gateway', () {
    final gateway = _FakeTunnelHostGateway();

    AppCoreScope.configureForTest(
      appCoreApi: MockAppCoreApi(),
      tunnelHostGateway: gateway,
    );
    AppCoreScope.resetForTest();

    expect(AppCoreScope.tunnelHostGateway, isNot(same(gateway)));
    expect(AppCoreScope.tunnelHostGateway, isA<TunnelHostGateway>());
  });

  test('configureForTest can override app core mode', () {
    final originalMode = AppCoreScope.mode;

    AppCoreScope.configureForTest(
      appCoreApi: MockAppCoreApi(),
      mode: 'bridge',
    );
    addTearDown(AppCoreScope.resetForTest);

    expect(AppCoreScope.mode, 'bridge');

    AppCoreScope.resetForTest();
    expect(AppCoreScope.mode, originalMode);
  });

  test('windows default host config falls back to local server', () {
    AppCoreScope.resetForTest();

    expect(AppCoreScope.hostConfig, isNotNull);
    expect(AppCoreScope.hostConfig!.controlBaseUrl, 'http://127.0.0.1:28080');
    expect(AppCoreScope.webConsoleUrl, 'http://127.0.0.1:24200');
  });

  test('network usage state is persisted per user', () async {
    SharedPreferences.setMockInitialValues({});

    await AppCoreScope.persistNetworkUsageState(
      userId: 'user-1',
      enabled: true,
      networkId: 'net-1',
    );

    final state = await AppCoreScope.readNetworkUsageState('user-1');
    expect(state, isNotNull);
    expect(state!.enabled, isTrue);
    expect(state.networkId, 'net-1');
    expect(state.updatedAtMs, isNotNull);
    expect(await AppCoreScope.readNetworkUsageState('user-2'), isNull);
  });

  test('network usage disabled state is persisted', () async {
    SharedPreferences.setMockInitialValues({});

    await AppCoreScope.persistNetworkUsageState(
      userId: 'user-1',
      enabled: false,
      networkId: 'net-1',
    );

    final state = await AppCoreScope.readNetworkUsageState('user-1');
    expect(state, isNotNull);
    expect(state!.enabled, isFalse);
    expect(state.networkId, 'net-1');
  });

  test('external session restore auto-enables last enabled network', () async {
    SharedPreferences.setMockInitialValues({});
    final appCore = _AutoEnableMockAppCoreApi();
    AppCoreScope.configureForTest(
      appCoreApi: appCore,
      mqttControlTaskService: appCore.mqttTasks,
      networkEnablePreflightService: _AutoEnablePreflightService(appCore),
      mode: 'bridge',
    );
    addTearDown(AppCoreScope.resetForTest);
    await AppCoreScope.persistNetworkUsageState(
      userId: 'user-1',
      enabled: true,
      networkId: 'net-1',
    );

    await AppCoreScope.sessionController.applyExternalSession(
      const SessionModel(
        userId: 'user-1',
        accessToken: 'token-1',
        expiresIn: 3600,
        deviceId: 'dev-1',
      ),
    );

    expect(appCore.enableLocalNetworkCalls, 0);
    expect(appCore.mqttTasks.taskTypes, contains('enable_network'));
    expect(appCore.mqttTasks.networkIds, contains('net-1'));
    expect(AppCoreScope.sessionStore.selectedNetworkId, 'net-1');
  });

  test('external session restore keeps disabled network disabled', () async {
    SharedPreferences.setMockInitialValues({});
    final appCore = _AutoEnableMockAppCoreApi();
    AppCoreScope.configureForTest(
      appCoreApi: appCore,
      mqttControlTaskService: appCore.mqttTasks,
      networkEnablePreflightService: _AutoEnablePreflightService(appCore),
      mode: 'bridge',
    );
    addTearDown(AppCoreScope.resetForTest);
    await AppCoreScope.persistNetworkUsageState(
      userId: 'user-1',
      enabled: false,
      networkId: 'net-1',
    );

    await AppCoreScope.sessionController.applyExternalSession(
      const SessionModel(
        userId: 'user-1',
        accessToken: 'token-1',
        expiresIn: 3600,
        deviceId: 'dev-1',
      ),
    );

    expect(appCore.enableLocalNetworkCalls, 0);
    expect(appCore.lastEnabledNetworkId, isNull);
    expect(AppCoreScope.sessionStore.selectedNetworkId, 'net-1');
  });
}

class _AutoEnableMockAppCoreApi extends MockAppCoreApi {
  int enableLocalNetworkCalls = 0;
  String? lastEnabledNetworkId;
  final _RecordingMqttControlTaskService mqttTasks =
      _RecordingMqttControlTaskService();

  final DeviceModel device = const DeviceModel(
    deviceId: 'dev-1',
    name: 'desktop',
    platform: 'windows',
    status: 'reachable',
    virtualIp: '10.0.0.2',
    publicKey: 'device-pub-1',
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
  Future<List<DeviceModel>> listDevices() async => [device];

  @override
  Future<List<NetworkModel>> listNetworks() async => networks;

  @override
  Future<BootstrapModel?> enableLocalNetwork({String? networkId}) async {
    enableLocalNetworkCalls += 1;
    lastEnabledNetworkId = networkId;
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
  final List<String> networkIds = [];

  @override
  Future<void> enqueueNetworkTask({
    required String taskType,
    required String networkId,
    required String deviceId,
    required String uiRefreshReason,
  }) async {
    taskTypes.add(taskType);
    networkIds.add(networkId);
  }

  @override
  void markUiRefreshConsumed(String? taskId) {}
}

class _AutoEnablePreflightService implements NetworkEnablePreflightService {
  const _AutoEnablePreflightService(this.appCore);

  final _AutoEnableMockAppCoreApi appCore;

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

class _FakeTunnelHostGateway extends TunnelHostGateway {
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
  Future<WireGuardTunnelRuntimeView?> tunnelRuntimeView(String peerVirtualIp) {
    throw UnimplementedError();
  }
}
