library slan_app.application.control_plan_insights;

import '../infra/app_core/models/connection_models.dart';
import '../infra/app_core/models/control_models.dart';
import '../infra/app_core/models/diagnostic_models.dart';

String preferredControlPathLabel(ControlStatusModel? status) {
  final plan =
      status?.connectPlans.isEmpty == false ? status!.connectPlans.first : null;
  final path = plan?.preferredPath;
  if (path == null) {
    return '-';
  }
  return '${path.pathType} ${path.endpoint}';
}

String preferredControlRelayLabel(ControlStatusModel? status) {
  final plan =
      status?.connectPlans.isEmpty == false ? status!.connectPlans.first : null;
  if (plan == null) {
    return '-';
  }
  if (plan.preferredDerpNodeIds.isNotEmpty) {
    return plan.preferredDerpNodeIds.first;
  }
  return plan.derpClusterId ?? '-';
}

ControlConnectPlanModel? connectPlanForPeer(
  ControlStatusModel? status,
  String peerNodeId,
) {
  if (peerNodeId.trim().isEmpty) {
    return null;
  }
  for (final plan
      in status?.connectPlans ?? const <ControlConnectPlanModel>[]) {
    if (plan.peerNodeId == peerNodeId) {
      return plan;
    }
  }
  return null;
}

String describeConnectPlan(ControlConnectPlanModel plan) {
  final preferredPath = plan.preferredPath;
  if (preferredPath != null) {
    return '${plan.preferDirect ? 'direct' : 'guided'} ${preferredPath.pathType} ${preferredPath.endpoint}';
  }
  if (plan.preferredDerpNodeIds.isNotEmpty) {
    return 'relay ${plan.preferredDerpNodeIds.first}';
  }
  if (plan.derpClusterId?.isNotEmpty == true) {
    return 'relay cluster ${plan.derpClusterId}';
  }
  return plan.preferDirect ? 'direct-first plan' : 'relay-guided plan';
}

String connectRecommendationMatchLabel({
  required ControlConnectPlanModel? controlPlan,
  required ConnectionStateModel connectionState,
  required DataPlaneProbeModel? lastProbe,
}) {
  if (controlPlan == null) {
    return 'none';
  }
  if (connectionState.status != 'connected') {
    return connectionState.status == 'failed' ? 'diverged' : 'pending';
  }

  final preferredPath = controlPlan.preferredPath;
  final expectsRelay = preferredPath == null
      ? (!controlPlan.preferDirect &&
          (controlPlan.preferredDerpNodeIds.isNotEmpty ||
              (controlPlan.derpClusterId?.isNotEmpty ?? false)))
      : pathLooksRelay(preferredPath.pathType);
  final actualRelay = connectionState.path == ConnectionPathModel.relay ||
      probeLooksRelay(lastProbe);
  final actualDirect = connectionState.path == ConnectionPathModel.p2p ||
      probeLooksDirect(lastProbe);

  if (expectsRelay) {
    return actualRelay ? 'matched' : 'diverged';
  }
  if (actualDirect) {
    return 'matched';
  }
  if (actualRelay) {
    return 'diverged';
  }
  return 'pending';
}

bool pathLooksRelay(String pathType) {
  final normalized = pathType.toLowerCase();
  return normalized.contains('relay') || normalized.contains('derp');
}

bool probeLooksRelay(DataPlaneProbeModel? probe) {
  if (probe == null) {
    return false;
  }
  final values = probe.activePath.values.map((value) => '$value'.toLowerCase());
  return values
      .any((value) => value.contains('relay') || value.contains('derp'));
}

bool probeLooksDirect(DataPlaneProbeModel? probe) {
  if (probe == null) {
    return false;
  }
  final values = probe.activePath.values.map((value) => '$value'.toLowerCase());
  return values.any(
    (value) =>
        value.contains('p2p') ||
        value.contains('direct') ||
        value.contains('reflexive'),
  );
}
