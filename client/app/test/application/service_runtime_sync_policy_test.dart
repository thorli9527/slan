import 'package:flutter_test/flutter_test.dart';
import 'package:slan_app/application/service_runtime_sync_policy.dart';
import 'package:slan_app/infra/app_core/models/diagnostic_models.dart';

void main() {
  const policy = DefaultServiceRuntimeSyncPolicy();

  test('decide returns none when there is no runtime and no UI refresh', () {
    final decision = policy.decide(
      _input(
        status: _status(helperReachable: true),
        hadRuntime: false,
        hadDeviceIp: false,
        hasAssignedDeviceIp: false,
        hasControlStatus: false,
      ),
    );

    expect(decision.action, ServiceRuntimeSyncAction.none);
  });

  test('decide waits when disable task needs UI refresh but runtime still active', () {
    final decision = policy.decide(
      _input(
        status: _status(
          helperReachable: true,
          currentNetworkId: 'net-1',
          tunnelBackendRunning: true,
          mqttControlUiRefreshRequired: true,
          mqttControlUiRefreshReason: 'network_disabled',
        ),
      ),
    );

    expect(decision.action, ServiceRuntimeSyncAction.waitForPendingDisable);
  });

  test('decide refreshes inventory and consumes UI refresh after enable', () {
    final decision = policy.decide(
      _input(
        status: _status(
          helperReachable: true,
          currentNetworkId: 'net-1',
          tunnelBackendRunning: true,
          mqttControlUiRefreshRequired: true,
          mqttControlUiRefreshReason: 'network_enabled',
        ),
        selectedNetworkId: 'net-1',
      ),
    );

    expect(decision.action, ServiceRuntimeSyncAction.refreshInventoryAndActivate);
    expect(decision.helperNetworkId, 'net-1');
    expect(decision.consumeUiRefresh, isTrue);
  });

  test('decide clears inactive runtime and consumes pending UI refresh', () {
    final decision = policy.decide(
      _input(
        status: _status(
          helperReachable: true,
          mqttControlUiRefreshRequired: true,
          mqttControlUiRefreshReason: 'network_disabled',
        ),
        hadRuntime: true,
        hadDeviceIp: true,
      ),
    );

    expect(decision.action, ServiceRuntimeSyncAction.clearInactiveRuntime);
    expect(decision.consumeUiRefresh, isTrue);
  });
}

ServiceRuntimeSyncInput _input({
  required AppCoreHelperStatusModel status,
  bool hadRuntime = true,
  bool hadDeviceIp = true,
  bool hasAssignedDeviceIp = true,
  bool hasControlStatus = true,
  String? selectedNetworkId,
}) {
  return ServiceRuntimeSyncInput(
    status: status,
    hadRuntime: hadRuntime,
    hadDeviceIp: hadDeviceIp,
    hasAssignedDeviceIp: hasAssignedDeviceIp,
    hasControlStatus: hasControlStatus,
    selectedNetworkId: selectedNetworkId,
  );
}

AppCoreHelperStatusModel _status({
  required bool helperReachable,
  String? currentNetworkId,
  bool tunnelBackendRunning = false,
  bool mqttControlUiRefreshRequired = false,
  String? mqttControlUiRefreshReason,
}) {
  return AppCoreHelperStatusModel(
    source: 'test',
    helperReachable: helperReachable,
    sessionPresent: true,
    refreshTokenPresent: true,
    bootstrapPresent: false,
    networkMapPresent: false,
    tunnelRuntimePresent: tunnelBackendRunning,
    tunnelBackendRunning: tunnelBackendRunning,
    currentNetworkId: currentNetworkId,
    mqttControlUiRefreshRequired: mqttControlUiRefreshRequired,
    mqttControlUiRefreshReason: mqttControlUiRefreshReason,
  );
}
