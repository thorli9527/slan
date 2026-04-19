/// Relay 配置模型。
class RelayConfigModel {
  const RelayConfigModel({
    required this.defaultClusterId,
    this.countries = const [],
  });

  final String defaultClusterId;
  final List<RelayCountryModel> countries;
}

class RelayCountryModel {
  const RelayCountryModel({
    required this.countryCode,
    required this.countryName,
    this.cities = const [],
  });

  final String countryCode;
  final String countryName;
  final List<RelayCityModel> cities;
}

class RelayCityModel {
  const RelayCityModel({
    required this.cityCode,
    required this.cityName,
    this.clusters = const [],
  });

  final String cityCode;
  final String cityName;
  final List<RelayClusterModel> clusters;
}

class RelayClusterModel {
  const RelayClusterModel({
    required this.clusterId,
    required this.clusterName,
    this.nodes = const [],
  });

  final String clusterId;
  final String clusterName;
  final List<RelayNodeModel> nodes;
}

class RelayNodeModel {
  const RelayNodeModel({
    required this.nodeId,
    required this.transport,
    required this.address,
    required this.priority,
    this.tags = const [],
  });

  final String nodeId;
  final String transport;
  final String address;
  final int priority;
  final List<String> tags;
}

/// Relay 票据模型。
class RelayTicketModel {
  const RelayTicketModel({
    required this.ticketId,
    required this.networkId,
    required this.sessionId,
    required this.srcNodeId,
    required this.dstNodeId,
    this.derpClusterId,
    this.countryCode,
    this.cityCode,
    this.allowedDerpNodeIds = const [],
    required this.relayUrl,
    required this.expiresAt,
    this.sessionKey,
    required this.signature,
  });

  final String ticketId;
  final String networkId;
  final String sessionId;
  final String srcNodeId;
  final String dstNodeId;
  final String? derpClusterId;
  final String? countryCode;
  final String? cityCode;
  final List<String> allowedDerpNodeIds;
  final String relayUrl;
  final String expiresAt;
  final String? sessionKey;
  final String signature;
}
