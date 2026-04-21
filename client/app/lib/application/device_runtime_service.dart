library slan_app.application.device_runtime_service;

import '../infra/app_core/api/app_core_api.dart';
import '../infra/app_core/models/bootstrap_models.dart';
import '../infra/app_core/models/connection_models.dart';
import '../infra/app_core/models/control_models.dart';
import '../infra/app_core/models/diagnostic_models.dart';
import '../infra/app_core/models/identity_models.dart';
import '../infra/app_core/models/network_models.dart';
import '../infra/app_core/models/relay_models.dart';

class BootstrapRefreshResult {
  const BootstrapRefreshResult({
    required this.bootstrap,
    required this.controlStatus,
    required this.detail,
    required this.usedControlSync,
  });

  final BootstrapModel bootstrap;
  final ControlStatusModel controlStatus;
  final String detail;
  final bool usedControlSync;
}

class ConnectAttemptResult {
  const ConnectAttemptResult({
    required this.connectionState,
    required this.relayTicket,
    required this.preflightHint,
    required this.resultHint,
  });

  final ConnectionStateModel connectionState;
  final RelayTicketModel? relayTicket;
  final String preflightHint;
  final String resultHint;
}

class DeviceRuntimeService {
  const DeviceRuntimeService({
    required AppCoreApi Function() apiProvider,
  }) : _apiProvider = apiProvider;

  final AppCoreApi Function() _apiProvider;

  AppCoreApi get _api => _apiProvider();

  Future<BootstrapRefreshResult> refreshBootstrap({
    required NodeModel? node,
    required BootstrapModel? currentBootstrap,
    required List<NetworkModel> currentNetworks,
  }) async {
    final targetNodeId = node?.nodeId.trim();
    final targetNetworkId = currentBootstrap?.networks.isNotEmpty == true
        ? currentBootstrap!.networks.first.networkId
        : currentNetworks.isNotEmpty
            ? currentNetworks.first.networkId
            : null;
    if (targetNodeId == null ||
        targetNodeId.isEmpty ||
        targetNetworkId == null ||
        targetNetworkId.isEmpty) {
      throw StateError('nodeId and networkId are required');
    }

    final canRunControlSync =
        (currentBootstrap?.controlPlane.sessionToken?.isNotEmpty ?? false) &&
            (currentBootstrap?.controlPlane.wsUrl.trim().isNotEmpty ?? false);

    final bootstrap = canRunControlSync
        ? await _api.controlSync(
            nodeId: targetNodeId,
            networkId: targetNetworkId,
          )
        : await _api.bootstrap(
            nodeId: targetNodeId,
            networkId: targetNetworkId,
          );
    final controlStatus = await _api.controlStatus();
    final detail = canRunControlSync
        ? 'Control session synchronized and bootstrap metadata were refreshed from the control plane.'
        : 'Bootstrap and relay metadata were refreshed from the control plane.';
    return BootstrapRefreshResult(
      bootstrap: bootstrap,
      controlStatus: controlStatus,
      detail: detail,
      usedControlSync: canRunControlSync,
    );
  }

  Future<ConnectAttemptResult> connectWithFallback({
    required String networkId,
    required String peerNodeId,
    required String reason,
    required NodeModel? currentNode,
    required ControlStatusModel? controlStatus,
    required DataPlaneProbeModel? lastProbe,
  }) async {
    final localNodeId = currentNode?.nodeId.trim();
    if (localNodeId == null || localNodeId.isEmpty) {
      throw StateError('register a node first');
    }

    final plan = _connectPlanForPeer(controlStatus, peerNodeId);
    final preflightHint = plan == null
        ? 'No control-plane connect plan for $peerNodeId. Trying local direct path first, then relay fallback if needed.'
        : 'Trying ${_describeConnectPlan(plan)} for $peerNodeId before relay fallback.';

    RelayTicketModel? relayTicket;
    var connectionState = await _api.connect(
      networkId: networkId,
      peerNodeId: peerNodeId,
    );
    if (connectionState.status == 'failed') {
      relayTicket = await _api.issueRelayTicket(
        networkId: networkId,
        srcNodeId: localNodeId,
        dstNodeId: peerNodeId,
        reason: reason,
      );
      connectionState = await _api.connect(
        networkId: networkId,
        peerNodeId: 'relay-$peerNodeId',
      );
    }

    final recommendationMatch = _connectRecommendationMatchLabel(
      controlPlan: plan,
      connectionState: connectionState,
      lastProbe: lastProbe,
    );
    final resultHint = switch (connectionState.status) {
      'connected' =>
        'Connected over ${connectionState.path?.name ?? 'unknown'} path. Recommendation $recommendationMatch.${plan == null ? '' : ' Control plane suggested ${_describeConnectPlan(plan)}.'}',
      'failed' =>
        'Connection failed after trying the planned path. Recommendation $recommendationMatch.${plan == null ? '' : ' Last control suggestion was ${_describeConnectPlan(plan)}.'}',
      'connecting' =>
        'Connection is still in progress. Recommendation $recommendationMatch.${plan == null ? '' : ' Following ${_describeConnectPlan(plan)}.'}',
      _ =>
        'Connection state is ${connectionState.status}. Recommendation $recommendationMatch.${plan == null ? '' : ' Control suggestion remains ${_describeConnectPlan(plan)}.'}',
    };

    return ConnectAttemptResult(
      connectionState: connectionState,
      relayTicket: relayTicket,
      preflightHint: preflightHint,
      resultHint: resultHint,
    );
  }
}

ControlConnectPlanModel? _connectPlanForPeer(
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

String _describeConnectPlan(ControlConnectPlanModel plan) {
  final preferredPath = plan.preferredPath;
  if (preferredPath != null) {
    return '${plan.preferDirect ? 'direct' : 'guided'} ${preferredPath.pathType} ${preferredPath.endpoint}';
  }
  if (plan.preferredDerpNodeIds.isNotEmpty) {
    return 'relay ${plan.preferredDerpNodeIds.first}';
  }
  if (plan.derpClusterIDOrNull?.isNotEmpty == true) {
    return 'relay cluster ${plan.derpClusterIDOrNull}';
  }
  return plan.preferDirect ? 'direct-first plan' : 'relay-guided plan';
}

String _connectRecommendationMatchLabel({
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
              (controlPlan.derpClusterIDOrNull?.isNotEmpty ?? false)))
      : _pathLooksRelay(preferredPath.pathType);
  final actualRelay = connectionState.path == ConnectionPathModel.relay ||
      _probeLooksRelay(lastProbe);
  final actualDirect = connectionState.path == ConnectionPathModel.p2p ||
      _probeLooksDirect(lastProbe);

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

bool _pathLooksRelay(String pathType) {
  final normalized = pathType.toLowerCase();
  return normalized.contains('relay') || normalized.contains('derp');
}

bool _probeLooksRelay(DataPlaneProbeModel? probe) {
  if (probe == null) {
    return false;
  }
  final values = probe.activePath.values.map((value) => '$value'.toLowerCase());
  return values
      .any((value) => value.contains('relay') || value.contains('derp'));
}

bool _probeLooksDirect(DataPlaneProbeModel? probe) {
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

extension on ControlConnectPlanModel {
  String? get derpClusterIDOrNull => derpClusterId;
}
