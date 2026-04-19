class ControlStatusModel {
  const ControlStatusModel({
    required this.status,
    this.wsUrl,
    this.heartbeatSeconds,
    required this.sessionTokenPresent,
    required this.networkMapPresent,
    this.networkId,
    this.nodeId,
    this.deviceId,
    required this.peerCount,
    required this.connectPlanCount,
    this.connectPlans = const [],
  });

  final String status;
  final String? wsUrl;
  final int? heartbeatSeconds;
  final bool sessionTokenPresent;
  final bool networkMapPresent;
  final String? networkId;
  final String? nodeId;
  final String? deviceId;
  final int peerCount;
  final int connectPlanCount;
  final List<ControlConnectPlanModel> connectPlans;
}

class ControlConnectPlanModel {
  const ControlConnectPlanModel({
    required this.peerNodeId,
    required this.preferDirect,
    required this.pathCount,
    this.preferredPath,
    this.derpClusterId,
    this.preferredDerpNodeIds = const [],
    this.relayTicketId,
  });

  final String peerNodeId;
  final bool preferDirect;
  final int pathCount;
  final ControlPathOptionModel? preferredPath;
  final String? derpClusterId;
  final List<String> preferredDerpNodeIds;
  final String? relayTicketId;
}

class ControlPathOptionModel {
  const ControlPathOptionModel({
    required this.pathType,
    required this.endpoint,
    required this.priority,
  });

  final String pathType;
  final String endpoint;
  final int priority;
}
