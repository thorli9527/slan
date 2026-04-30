import '../infra/app_core/models/diagnostic_models.dart';

abstract class ServiceRuntimeSyncPolicy {
  ServiceRuntimeSyncDecision decide(ServiceRuntimeSyncInput input);
}

class DefaultServiceRuntimeSyncPolicy implements ServiceRuntimeSyncPolicy {
  const DefaultServiceRuntimeSyncPolicy();

  @override
  ServiceRuntimeSyncDecision decide(ServiceRuntimeSyncInput input) {
    final status = input.status;
    final helperNetworkId = status.currentNetworkId?.trim();
    final runtimeActive = status.helperReachable &&
        status.tunnelBackendRunning &&
        helperNetworkId != null &&
        helperNetworkId.isNotEmpty;
    final uiRefreshRequired = status.mqttControlUiRefreshRequired;
    final uiRefreshReason =
        status.mqttControlUiRefreshReason?.trim().toLowerCase();

    if (runtimeActive) {
      if (uiRefreshRequired &&
          (uiRefreshReason == 'network_disabled' ||
              uiRefreshReason == 'ip_reassigned')) {
        return const ServiceRuntimeSyncDecision(
          action: ServiceRuntimeSyncAction.waitForPendingDisable,
        );
      }
      final shouldRefreshInventory = uiRefreshRequired ||
          !input.hasAssignedDeviceIp ||
          input.selectedNetworkId != helperNetworkId;
      final shouldConsumeUiRefresh = uiRefreshRequired &&
          uiRefreshReason != 'network_disabled' &&
          uiRefreshReason != 'ip_reassigned';
      return ServiceRuntimeSyncDecision(
        action: shouldRefreshInventory
            ? ServiceRuntimeSyncAction.refreshInventoryAndActivate
            : ServiceRuntimeSyncAction.selectNetworkAndActivate,
        helperNetworkId: helperNetworkId,
        consumeUiRefresh: shouldConsumeUiRefresh,
        notice: shouldConsumeUiRefresh ? '本地网络已启用，界面状态已同步。' : null,
      );
    }

    if (!uiRefreshRequired &&
        !input.hadRuntime &&
        !input.hadDeviceIp &&
        !input.hasControlStatus) {
      return const ServiceRuntimeSyncDecision(
        action: ServiceRuntimeSyncAction.none,
      );
    }
    return ServiceRuntimeSyncDecision(
      action: ServiceRuntimeSyncAction.clearInactiveRuntime,
      consumeUiRefresh: uiRefreshRequired,
      notice: '本地网络已停用，界面状态已同步。',
    );
  }
}

class ServiceRuntimeSyncInput {
  const ServiceRuntimeSyncInput({
    required this.status,
    required this.hadRuntime,
    required this.hadDeviceIp,
    required this.hasAssignedDeviceIp,
    required this.hasControlStatus,
    required this.selectedNetworkId,
  });

  final AppCoreHelperStatusModel status;
  final bool hadRuntime;
  final bool hadDeviceIp;
  final bool hasAssignedDeviceIp;
  final bool hasControlStatus;
  final String? selectedNetworkId;
}

class ServiceRuntimeSyncDecision {
  const ServiceRuntimeSyncDecision({
    required this.action,
    this.helperNetworkId,
    this.consumeUiRefresh = false,
    this.notice,
  });

  final ServiceRuntimeSyncAction action;
  final String? helperNetworkId;
  final bool consumeUiRefresh;
  final String? notice;
}

enum ServiceRuntimeSyncAction {
  none,
  waitForPendingDisable,
  refreshInventoryAndActivate,
  selectNetworkAndActivate,
  clearInactiveRuntime,
}
