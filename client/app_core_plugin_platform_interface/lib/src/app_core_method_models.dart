class AppCoreSessionPayload {
  const AppCoreSessionPayload({
    required this.userId,
    required this.accessToken,
    this.refreshToken,
    required this.expiresIn,
    this.deviceId,
    this.userLabel,
  });

  factory AppCoreSessionPayload.fromJson(Map<String, dynamic> json) {
    return AppCoreSessionPayload(
      userId: json['userId'] as String? ?? '',
      accessToken: json['accessToken'] as String? ?? '',
      refreshToken: json['refreshToken'] as String?,
      expiresIn: (json['expiresIn'] as num?)?.toInt() ?? 3600,
      deviceId: json['deviceId'] as String?,
      userLabel: json['userLabel'] as String?,
    );
  }

  final String userId;
  final String accessToken;
  final String? refreshToken;
  final int expiresIn;
  final String? deviceId;
  final String? userLabel;

  Map<String, dynamic> toJson() => {
        'userId': userId,
        'accessToken': accessToken,
        'refreshToken': refreshToken,
        'expiresIn': expiresIn,
        'deviceId': deviceId,
        'userLabel': userLabel,
      };
}

class AppCoreDevicePayload {
  const AppCoreDevicePayload({
    required this.deviceId,
    required this.name,
    required this.platform,
    required this.status,
    this.currentVirtualIp,
    this.publicKey,
  });

  factory AppCoreDevicePayload.fromJson(Map<String, dynamic> json) {
    return AppCoreDevicePayload(
      deviceId: json['deviceId'] as String? ?? '',
      name: json['name'] as String? ?? '',
      platform: json['platform'] as String? ?? '',
      status: json['status'] as String? ?? '',
      currentVirtualIp: json['currentVirtualIp'] as String?,
      publicKey: json['publicKey'] as String?,
    );
  }

  final String deviceId;
  final String name;
  final String platform;
  final String status;
  final String? currentVirtualIp;
  final String? publicKey;

  Map<String, dynamic> toJson() => {
        'deviceId': deviceId,
        'name': name,
        'platform': platform,
        'status': status,
        'currentVirtualIp': currentVirtualIp,
        'publicKey': publicKey,
      };
}

class AppCoreNodePayload {
  const AppCoreNodePayload({
    required this.nodeId,
    required this.deviceId,
    required this.nodePublicKey,
    this.networkIds = const [],
    this.capabilities = const [],
  });

  factory AppCoreNodePayload.fromJson(Map<String, dynamic> json) {
    return AppCoreNodePayload(
      nodeId: json['nodeId'] as String? ?? '',
      deviceId: json['deviceId'] as String? ?? '',
      nodePublicKey: json['nodePublicKey'] as String? ?? '',
      networkIds: _readStringList(json['networkIds']),
      capabilities: _readStringList(json['capabilities']),
    );
  }

  final String nodeId;
  final String deviceId;
  final String nodePublicKey;
  final List<String> networkIds;
  final List<String> capabilities;

  Map<String, dynamic> toJson() => {
        'nodeId': nodeId,
        'deviceId': deviceId,
        'nodePublicKey': nodePublicKey,
        'networkIds': networkIds,
        'capabilities': capabilities,
      };
}

class AppCoreNetworkPayload {
  const AppCoreNetworkPayload({
    required this.networkId,
    required this.name,
    this.defaultSubnetCidr,
  });

  factory AppCoreNetworkPayload.fromJson(Map<String, dynamic> json) {
    return AppCoreNetworkPayload(
      networkId: json['networkId'] as String? ?? '',
      name: json['name'] as String? ?? '',
      defaultSubnetCidr: json['defaultSubnetCidr'] as String?,
    );
  }

  final String networkId;
  final String name;
  final String? defaultSubnetCidr;

  Map<String, dynamic> toJson() => {
        'networkId': networkId,
        'name': name,
        'defaultSubnetCidr': defaultSubnetCidr,
      };
}

class AppCoreRelayTicketPayload {
  const AppCoreRelayTicketPayload({
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

  factory AppCoreRelayTicketPayload.fromJson(Map<String, dynamic> json) {
    return AppCoreRelayTicketPayload(
      ticketId: json['ticketId'] as String? ?? '',
      networkId: json['networkId'] as String? ?? '',
      sessionId: json['sessionId'] as String? ?? '',
      srcNodeId: json['srcNodeId'] as String? ?? '',
      dstNodeId: json['dstNodeId'] as String? ?? '',
      derpClusterId: json['derpClusterId'] as String?,
      countryCode: json['countryCode'] as String?,
      cityCode: json['cityCode'] as String?,
      allowedDerpNodeIds: _readStringList(json['allowedDerpNodeIds']),
      relayUrl: json['relayUrl'] as String? ?? '',
      expiresAt: json['expiresAt'] as String? ?? '',
      sessionKey: json['sessionKey'] as String?,
      signature: json['signature'] as String? ?? '',
    );
  }

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

  Map<String, dynamic> toJson() => {
        'ticketId': ticketId,
        'networkId': networkId,
        'sessionId': sessionId,
        'srcNodeId': srcNodeId,
        'dstNodeId': dstNodeId,
        'derpClusterId': derpClusterId,
        'countryCode': countryCode,
        'cityCode': cityCode,
        'allowedDerpNodeIds': allowedDerpNodeIds,
        'relayUrl': relayUrl,
        'expiresAt': expiresAt,
        'sessionKey': sessionKey,
        'signature': signature,
      };
}

class AppCoreConnectionStatusPayload {
  const AppCoreConnectionStatusPayload({
    required this.status,
    this.path,
    this.reason,
  });

  factory AppCoreConnectionStatusPayload.fromJson(Map<String, dynamic> json) {
    return AppCoreConnectionStatusPayload(
      status: json['status'] as String? ?? '',
      path: json['path'] as String?,
      reason: json['reason'] as String?,
    );
  }

  final String status;
  final String? path;
  final String? reason;

  Map<String, dynamic> toJson() => {
        'status': status,
        'path': path,
        'reason': reason,
      };
}

class AppCoreBootstrapPayload {
  const AppCoreBootstrapPayload({
    required this.device,
    this.networks = const [],
    required this.controlPlane,
    this.stunServers = const [],
    required this.relay,
    this.networkMap,
  });

  factory AppCoreBootstrapPayload.fromJson(Map<String, dynamic> json) {
    return AppCoreBootstrapPayload(
      device: AppCoreBootstrapDevicePayload.fromJson(
        _readMap(json['device']),
      ),
      networks: _readMapList(json['networks'])
          .map(AppCoreNetworkDetailPayload.fromJson)
          .toList(growable: false),
      controlPlane: AppCoreControlPlanePayload.fromJson(
        _readMap(json['controlPlane']),
      ),
      stunServers: _readStringList(json['stunServers']),
      relay: AppCoreRelayConfigPayload.fromJson(_readMap(json['relay'])),
      networkMap: json['networkMap'] == null
          ? null
          : AppCoreNetworkMapPayload.fromJson(_readMap(json['networkMap'])),
    );
  }

  final AppCoreBootstrapDevicePayload device;
  final List<AppCoreNetworkDetailPayload> networks;
  final AppCoreControlPlanePayload controlPlane;
  final List<String> stunServers;
  final AppCoreRelayConfigPayload relay;
  final AppCoreNetworkMapPayload? networkMap;

  Map<String, dynamic> toJson() => {
        'device': device.toJson(),
        'networks': networks.map((item) => item.toJson()).toList(),
        'controlPlane': controlPlane.toJson(),
        'stunServers': stunServers,
        'relay': relay.toJson(),
        'networkMap': networkMap?.toJson(),
      };
}

class AppCoreBootstrapDevicePayload {
  const AppCoreBootstrapDevicePayload({
    required this.device,
    this.attachments = const [],
  });

  factory AppCoreBootstrapDevicePayload.fromJson(Map<String, dynamic> json) {
    return AppCoreBootstrapDevicePayload(
      device: AppCoreDevicePayload.fromJson(_readMap(json['device'])),
      attachments: _readMapList(json['attachments'])
          .map(AppCoreSubnetAttachmentPayload.fromJson)
          .toList(growable: false),
    );
  }

  final AppCoreDevicePayload device;
  final List<AppCoreSubnetAttachmentPayload> attachments;

  Map<String, dynamic> toJson() => {
        'device': device.toJson(),
        'attachments': attachments.map((item) => item.toJson()).toList(),
      };
}

class AppCoreSubnetAttachmentPayload {
  const AppCoreSubnetAttachmentPayload({
    required this.networkId,
    required this.deviceId,
    this.virtualIp,
  });

  factory AppCoreSubnetAttachmentPayload.fromJson(Map<String, dynamic> json) {
    return AppCoreSubnetAttachmentPayload(
      networkId: json['networkId'] as String? ?? '',
      deviceId: json['deviceId'] as String? ?? '',
      virtualIp: json['virtualIp'] as String?,
    );
  }

  final String networkId;
  final String deviceId;
  final String? virtualIp;

  Map<String, dynamic> toJson() => {
        'networkId': networkId,
        'deviceId': deviceId,
        'virtualIp': virtualIp,
      };
}

class AppCoreNetworkDetailPayload {
  const AppCoreNetworkDetailPayload({
    required this.networkId,
    required this.name,
    this.defaultSubnetCidr,
    this.subnets = const [],
    this.members = const [],
  });

  factory AppCoreNetworkDetailPayload.fromJson(Map<String, dynamic> json) {
    return AppCoreNetworkDetailPayload(
      networkId: json['networkId'] as String? ?? '',
      name: json['name'] as String? ?? '',
      defaultSubnetCidr: json['defaultSubnetCidr'] as String?,
      subnets: _readMapList(json['subnets'])
          .map(AppCoreSubnetPayload.fromJson)
          .toList(growable: false),
      members: _readMapList(json['members'])
          .map(AppCoreNetworkMemberPayload.fromJson)
          .toList(growable: false),
    );
  }

  final String networkId;
  final String name;
  final String? defaultSubnetCidr;
  final List<AppCoreSubnetPayload> subnets;
  final List<AppCoreNetworkMemberPayload> members;

  Map<String, dynamic> toJson() => {
        'networkId': networkId,
        'name': name,
        'defaultSubnetCidr': defaultSubnetCidr,
        'subnets': subnets.map((item) => item.toJson()).toList(),
        'members': members.map((item) => item.toJson()).toList(),
      };
}

class AppCoreSubnetPayload {
  const AppCoreSubnetPayload({
    required this.networkId,
    required this.cidr,
    required this.isDefault,
  });

  factory AppCoreSubnetPayload.fromJson(Map<String, dynamic> json) {
    return AppCoreSubnetPayload(
      networkId: json['networkId'] as String? ?? '',
      cidr: json['cidr'] as String? ?? '',
      isDefault: json['isDefault'] as bool? ?? false,
    );
  }

  final String networkId;
  final String cidr;
  final bool isDefault;

  Map<String, dynamic> toJson() => {
        'networkId': networkId,
        'cidr': cidr,
        'isDefault': isDefault,
      };
}

class AppCoreNetworkMemberPayload {
  const AppCoreNetworkMemberPayload({
    required this.deviceId,
    required this.role,
  });

  factory AppCoreNetworkMemberPayload.fromJson(Map<String, dynamic> json) {
    return AppCoreNetworkMemberPayload(
      deviceId: json['deviceId'] as String? ?? '',
      role: json['role'] as String? ?? '',
    );
  }

  final String deviceId;
  final String role;

  Map<String, dynamic> toJson() => {
        'deviceId': deviceId,
        'role': role,
      };
}

class AppCoreControlPlanePayload {
  const AppCoreControlPlanePayload({
    required this.wsUrl,
    this.sessionToken,
    required this.heartbeatSeconds,
  });

  factory AppCoreControlPlanePayload.fromJson(Map<String, dynamic> json) {
    return AppCoreControlPlanePayload(
      wsUrl: json['wsUrl'] as String? ?? '',
      sessionToken: json['sessionToken'] as String?,
      heartbeatSeconds: (json['heartbeatSeconds'] as num?)?.toInt() ?? 0,
    );
  }

  final String wsUrl;
  final String? sessionToken;
  final int heartbeatSeconds;

  Map<String, dynamic> toJson() => {
        'wsUrl': wsUrl,
        'sessionToken': sessionToken,
        'heartbeatSeconds': heartbeatSeconds,
      };
}

class AppCoreRelayConfigPayload {
  const AppCoreRelayConfigPayload({
    required this.defaultClusterId,
    this.countries = const [],
  });

  factory AppCoreRelayConfigPayload.fromJson(Map<String, dynamic> json) {
    return AppCoreRelayConfigPayload(
      defaultClusterId: json['defaultClusterId'] as String? ?? '',
      countries: _readMapList(json['countries'])
          .map(AppCoreRelayCountryPayload.fromJson)
          .toList(growable: false),
    );
  }

  final String defaultClusterId;
  final List<AppCoreRelayCountryPayload> countries;

  Map<String, dynamic> toJson() => {
        'defaultClusterId': defaultClusterId,
        'countries': countries.map((item) => item.toJson()).toList(),
      };
}

class AppCoreRelayCountryPayload {
  const AppCoreRelayCountryPayload({
    required this.countryCode,
    required this.countryName,
    this.cities = const [],
  });

  factory AppCoreRelayCountryPayload.fromJson(Map<String, dynamic> json) {
    return AppCoreRelayCountryPayload(
      countryCode: json['countryCode'] as String? ?? '',
      countryName: json['countryName'] as String? ?? '',
      cities: _readMapList(json['cities'])
          .map(AppCoreRelayCityPayload.fromJson)
          .toList(growable: false),
    );
  }

  final String countryCode;
  final String countryName;
  final List<AppCoreRelayCityPayload> cities;

  Map<String, dynamic> toJson() => {
        'countryCode': countryCode,
        'countryName': countryName,
        'cities': cities.map((item) => item.toJson()).toList(),
      };
}

class AppCoreRelayCityPayload {
  const AppCoreRelayCityPayload({
    required this.cityCode,
    required this.cityName,
    this.clusters = const [],
  });

  factory AppCoreRelayCityPayload.fromJson(Map<String, dynamic> json) {
    return AppCoreRelayCityPayload(
      cityCode: json['cityCode'] as String? ?? '',
      cityName: json['cityName'] as String? ?? '',
      clusters: _readMapList(json['clusters'])
          .map(AppCoreRelayClusterPayload.fromJson)
          .toList(growable: false),
    );
  }

  final String cityCode;
  final String cityName;
  final List<AppCoreRelayClusterPayload> clusters;

  Map<String, dynamic> toJson() => {
        'cityCode': cityCode,
        'cityName': cityName,
        'clusters': clusters.map((item) => item.toJson()).toList(),
      };
}

class AppCoreRelayClusterPayload {
  const AppCoreRelayClusterPayload({
    required this.clusterId,
    required this.clusterName,
    this.nodes = const [],
  });

  factory AppCoreRelayClusterPayload.fromJson(Map<String, dynamic> json) {
    return AppCoreRelayClusterPayload(
      clusterId: json['clusterId'] as String? ?? '',
      clusterName: json['clusterName'] as String? ?? '',
      nodes: _readMapList(json['nodes'])
          .map(AppCoreRelayNodePayload.fromJson)
          .toList(growable: false),
    );
  }

  final String clusterId;
  final String clusterName;
  final List<AppCoreRelayNodePayload> nodes;

  Map<String, dynamic> toJson() => {
        'clusterId': clusterId,
        'clusterName': clusterName,
        'nodes': nodes.map((item) => item.toJson()).toList(),
      };
}

class AppCoreRelayNodePayload {
  const AppCoreRelayNodePayload({
    required this.nodeId,
    required this.transport,
    required this.address,
    required this.priority,
    this.tags = const [],
  });

  factory AppCoreRelayNodePayload.fromJson(Map<String, dynamic> json) {
    return AppCoreRelayNodePayload(
      nodeId: json['nodeId'] as String? ?? '',
      transport: json['transport'] as String? ?? '',
      address: json['address'] as String? ?? '',
      priority: (json['priority'] as num?)?.toInt() ?? 0,
      tags: _readStringList(json['tags']),
    );
  }

  final String nodeId;
  final String transport;
  final String address;
  final int priority;
  final List<String> tags;

  Map<String, dynamic> toJson() => {
        'nodeId': nodeId,
        'transport': transport,
        'address': address,
        'priority': priority,
        'tags': tags,
      };
}

class AppCoreNetworkMapPayload {
  const AppCoreNetworkMapPayload({
    required this.networkId,
  });

  factory AppCoreNetworkMapPayload.fromJson(Map<String, dynamic> json) {
    return AppCoreNetworkMapPayload(
      networkId: json['networkId'] as String? ?? '',
    );
  }

  final String networkId;

  Map<String, dynamic> toJson() => {
        'networkId': networkId,
      };
}

class AppCoreControlStatusPayload {
  const AppCoreControlStatusPayload({
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

  factory AppCoreControlStatusPayload.fromJson(Map<String, dynamic> json) {
    return AppCoreControlStatusPayload(
      status: json['status'] as String? ?? '',
      wsUrl: json['wsUrl'] as String?,
      heartbeatSeconds: (json['heartbeatSeconds'] as num?)?.toInt(),
      sessionTokenPresent: json['sessionTokenPresent'] as bool? ?? false,
      networkMapPresent: json['networkMapPresent'] as bool? ?? false,
      networkId: json['networkId'] as String?,
      nodeId: json['nodeId'] as String?,
      deviceId: json['deviceId'] as String?,
      peerCount: (json['peerCount'] as num?)?.toInt() ?? 0,
      connectPlanCount: (json['connectPlanCount'] as num?)?.toInt() ?? 0,
      connectPlans: _readMapList(json['connectPlans'])
          .map(AppCoreControlConnectPlanPayload.fromJson)
          .toList(growable: false),
    );
  }

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
  final List<AppCoreControlConnectPlanPayload> connectPlans;

  Map<String, dynamic> toJson() => {
        'status': status,
        'wsUrl': wsUrl,
        'heartbeatSeconds': heartbeatSeconds,
        'sessionTokenPresent': sessionTokenPresent,
        'networkMapPresent': networkMapPresent,
        'networkId': networkId,
        'nodeId': nodeId,
        'deviceId': deviceId,
        'peerCount': peerCount,
        'connectPlanCount': connectPlanCount,
        'connectPlans': connectPlans.map((item) => item.toJson()).toList(),
      };
}

class AppCoreControlConnectPlanPayload {
  const AppCoreControlConnectPlanPayload({
    required this.peerNodeId,
    required this.preferDirect,
    required this.pathCount,
    this.preferredPath,
    this.derpClusterId,
    this.preferredDerpNodeIds = const [],
    this.relayTicketId,
  });

  factory AppCoreControlConnectPlanPayload.fromJson(Map<String, dynamic> json) {
    return AppCoreControlConnectPlanPayload(
      peerNodeId: json['peerNodeId'] as String? ?? '',
      preferDirect: json['preferDirect'] as bool? ?? false,
      pathCount: (json['pathCount'] as num?)?.toInt() ?? 0,
      preferredPath: json['preferredPath'] == null
          ? null
          : AppCoreControlPathOptionPayload.fromJson(
              _readMap(json['preferredPath']),
            ),
      derpClusterId: json['derpClusterId'] as String?,
      preferredDerpNodeIds: _readStringList(json['preferredDerpNodeIds']),
      relayTicketId: json['relayTicketId'] as String?,
    );
  }

  final String peerNodeId;
  final bool preferDirect;
  final int pathCount;
  final AppCoreControlPathOptionPayload? preferredPath;
  final String? derpClusterId;
  final List<String> preferredDerpNodeIds;
  final String? relayTicketId;

  Map<String, dynamic> toJson() => {
        'peerNodeId': peerNodeId,
        'preferDirect': preferDirect,
        'pathCount': pathCount,
        'preferredPath': preferredPath?.toJson(),
        'derpClusterId': derpClusterId,
        'preferredDerpNodeIds': preferredDerpNodeIds,
        'relayTicketId': relayTicketId,
      };
}

class AppCoreControlPathOptionPayload {
  const AppCoreControlPathOptionPayload({
    required this.pathType,
    required this.endpoint,
    required this.priority,
  });

  factory AppCoreControlPathOptionPayload.fromJson(Map<String, dynamic> json) {
    return AppCoreControlPathOptionPayload(
      pathType: json['pathType'] as String? ?? '',
      endpoint: json['endpoint'] as String? ?? '',
      priority: (json['priority'] as num?)?.toInt() ?? 0,
    );
  }

  final String pathType;
  final String endpoint;
  final int priority;

  Map<String, dynamic> toJson() => {
        'pathType': pathType,
        'endpoint': endpoint,
        'priority': priority,
      };
}

class AppCoreDataPlaneProbePayload {
  const AppCoreDataPlaneProbePayload({
    required this.probeId,
    required this.sampledAtMs,
    required this.activePath,
    required this.bytesSent,
    required this.replyObserved,
    this.replyBytesReceived,
    this.replySampledAtMs,
    this.replyRttMs,
    this.tunnelPeerVirtualIp,
    this.observedRttMs,
    this.packetLossPpm,
    this.pathScore,
    this.derpClusterId,
    this.derpNodeId,
  });

  factory AppCoreDataPlaneProbePayload.fromJson(Map<String, dynamic> json) {
    return AppCoreDataPlaneProbePayload(
      probeId: json['probeId'] as String? ?? '',
      sampledAtMs: (json['sampledAtMs'] as num?)?.toInt() ?? 0,
      activePath: AppCoreDataPlanePathPayload.fromJson(
        _readMap(json['activePath']),
      ),
      bytesSent: (json['bytesSent'] as num?)?.toInt() ?? 0,
      replyObserved: json['replyObserved'] as bool? ?? false,
      replyBytesReceived: (json['replyBytesReceived'] as num?)?.toInt(),
      replySampledAtMs: (json['replySampledAtMs'] as num?)?.toInt(),
      replyRttMs: (json['replyRttMs'] as num?)?.toInt(),
      tunnelPeerVirtualIp: json['tunnelPeerVirtualIp'] as String?,
      observedRttMs: (json['observedRttMs'] as num?)?.toInt(),
      packetLossPpm: (json['packetLossPpm'] as num?)?.toInt(),
      pathScore: (json['pathScore'] as num?)?.toInt(),
      derpClusterId: json['derpClusterId'] as String?,
      derpNodeId: json['derpNodeId'] as String?,
    );
  }

  final String probeId;
  final int sampledAtMs;
  final AppCoreDataPlanePathPayload activePath;
  final int bytesSent;
  final bool replyObserved;
  final int? replyBytesReceived;
  final int? replySampledAtMs;
  final int? replyRttMs;
  final String? tunnelPeerVirtualIp;
  final int? observedRttMs;
  final int? packetLossPpm;
  final int? pathScore;
  final String? derpClusterId;
  final String? derpNodeId;

  Map<String, dynamic> toJson() => {
        'probeId': probeId,
        'sampledAtMs': sampledAtMs,
        'activePath': activePath.toJson(),
        'bytesSent': bytesSent,
        'replyObserved': replyObserved,
        'replyBytesReceived': replyBytesReceived,
        'replySampledAtMs': replySampledAtMs,
        'replyRttMs': replyRttMs,
        'tunnelPeerVirtualIp': tunnelPeerVirtualIp,
        'observedRttMs': observedRttMs,
        'packetLossPpm': packetLossPpm,
        'pathScore': pathScore,
        'derpClusterId': derpClusterId,
        'derpNodeId': derpNodeId,
      };
}

class AppCoreDataPlanePathPayload {
  const AppCoreDataPlanePathPayload({
    required this.kind,
    this.details = const {},
  });

  factory AppCoreDataPlanePathPayload.fromJson(Map<String, dynamic> json) {
    if (json.isEmpty) {
      return const AppCoreDataPlanePathPayload(kind: '');
    }
    final entry = json.entries.first;
    return AppCoreDataPlanePathPayload(
      kind: entry.key,
      details: entry.value is Map ? _readMap(entry.value) : const {},
    );
  }

  final String kind;
  final Map<String, dynamic> details;

  bool get isRelay {
    final normalized = kind.toLowerCase();
    return normalized.contains('relay') || normalized.contains('derp');
  }

  bool get isDirect {
    final normalized = kind.toLowerCase();
    return normalized.contains('p2p') ||
        normalized.contains('direct') ||
        normalized.contains('reflexive');
  }

  Map<String, dynamic> toJson() => {
        if (kind.isNotEmpty) kind: details,
      };
}

List<String> _readStringList(Object? value) {
  if (value is List) {
    return value.whereType<String>().toList(growable: false);
  }
  return const [];
}

Map<String, dynamic> _readMap(Object? value) {
  if (value is Map<String, dynamic>) {
    return value;
  }
  if (value is Map) {
    return value.map((key, item) => MapEntry(key.toString(), item));
  }
  return const {};
}

List<Map<String, dynamic>> _readMapList(Object? value) {
  if (value is List) {
    return value.whereType<Map>().map(_readMap).toList(growable: false);
  }
  return const [];
}
