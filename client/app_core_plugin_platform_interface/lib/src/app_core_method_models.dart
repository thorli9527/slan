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
    this.ownerEmail,
    this.linkStatus,
    this.connectivityProtocol,
    this.joinedAt,
    this.membershipStatus,
    this.networkRole,
    this.createdAt,
    this.networkIds = const [],
  });

  factory AppCoreDevicePayload.fromJson(Map<String, dynamic> json) {
    return AppCoreDevicePayload(
      deviceId: json['deviceId'] as String? ?? '',
      name: json['name'] as String? ?? '',
      platform: json['platform'] as String? ?? '',
      status: json['status'] as String? ?? '',
      currentVirtualIp: json['currentVirtualIp'] as String?,
      publicKey: json['publicKey'] as String?,
      ownerEmail: json['ownerEmail'] as String?,
      linkStatus: json['linkStatus'] as String?,
      connectivityProtocol: json['connectivityProtocol'] as String?,
      joinedAt: (json['joinedAt'] as num?)?.toInt(),
      membershipStatus: json['membershipStatus'] as String?,
      networkRole: json['networkRole'] as String?,
      createdAt: (json['createdAt'] as num?)?.toInt(),
      networkIds: _readStringList(json['networkIds']),
    );
  }

  final String deviceId;
  final String name;
  final String platform;
  final String status;
  final String? currentVirtualIp;
  final String? publicKey;
  final String? ownerEmail;
  final String? linkStatus;
  final String? connectivityProtocol;
  final int? joinedAt;
  final String? membershipStatus;
  final String? networkRole;
  final int? createdAt;
  final List<String> networkIds;

  Map<String, dynamic> toJson() => {
        'deviceId': deviceId,
        'name': name,
        'platform': platform,
        'status': status,
        'currentVirtualIp': currentVirtualIp,
        'publicKey': publicKey,
        'ownerEmail': ownerEmail,
        'linkStatus': linkStatus,
        'connectivityProtocol': connectivityProtocol,
        'joinedAt': joinedAt,
        'membershipStatus': membershipStatus,
        'networkRole': networkRole,
        'createdAt': createdAt,
        'networkIds': networkIds,
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
    this.description,
    this.defaultSubnetId,
    this.cidr,
    this.defaultSubnetCidr,
    this.subnets = const [],
    this.members = const [],
  });

  factory AppCoreNetworkPayload.fromJson(Map<String, dynamic> json) {
    return AppCoreNetworkPayload(
      networkId: json['networkId'] as String? ?? '',
      name: json['name'] as String? ?? '',
      description: json['description'] as String?,
      defaultSubnetId: json['defaultSubnetId'] as String?,
      cidr: json['cidr'] as String?,
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
  final String? description;
  final String? defaultSubnetId;
  final String? cidr;
  final String? defaultSubnetCidr;
  final List<AppCoreSubnetPayload> subnets;
  final List<AppCoreNetworkMemberPayload> members;

  Map<String, dynamic> toJson() => {
        'networkId': networkId,
        'name': name,
        'description': description,
        'defaultSubnetId': defaultSubnetId,
        'cidr': cidr,
        'defaultSubnetCidr': defaultSubnetCidr,
        'subnets': subnets.map((item) => item.toJson()).toList(),
        'members': members.map((item) => item.toJson()).toList(),
      };
}

class AppCoreNetworkJoinPayload {
  const AppCoreNetworkJoinPayload({
    required this.networkId,
    required this.deviceId,
    this.memberId,
    this.attachmentId,
    this.virtualIp,
  });

  factory AppCoreNetworkJoinPayload.fromJson(Map<String, dynamic> json) {
    return AppCoreNetworkJoinPayload(
      networkId: json['networkId'] as String? ?? '',
      deviceId: json['deviceId'] as String? ?? '',
      memberId: json['memberId'] as String?,
      attachmentId: json['attachmentId'] as String?,
      virtualIp: json['virtualIp'] as String?,
    );
  }

  final String networkId;
  final String deviceId;
  final String? memberId;
  final String? attachmentId;
  final String? virtualIp;

  Map<String, dynamic> toJson() => {
        'networkId': networkId,
        'deviceId': deviceId,
        'memberId': memberId,
        'attachmentId': attachmentId,
        'virtualIp': virtualIp,
      };
}

class AppCoreNetworkAssignmentPayload {
  const AppCoreNetworkAssignmentPayload({
    required this.attachmentId,
    required this.networkId,
    required this.subnetId,
    required this.deviceId,
    required this.deviceName,
    required this.userId,
    required this.userEmail,
    required this.role,
    this.remark,
    this.virtualIp,
    this.status,
  });

  factory AppCoreNetworkAssignmentPayload.fromJson(Map<String, dynamic> json) {
    return AppCoreNetworkAssignmentPayload(
      attachmentId: json['attachmentId'] as String? ?? '',
      networkId: json['networkId'] as String? ?? '',
      subnetId: json['subnetId'] as String? ?? '',
      deviceId: json['deviceId'] as String? ?? '',
      deviceName: json['deviceName'] as String? ?? '',
      userId: json['userId'] as String? ?? '',
      userEmail: json['userEmail'] as String? ?? '',
      role: json['role'] as String? ?? '',
      remark: json['remark'] as String?,
      virtualIp: json['virtualIp'] as String?,
      status: json['status'] as String?,
    );
  }

  final String attachmentId;
  final String networkId;
  final String subnetId;
  final String deviceId;
  final String deviceName;
  final String userId;
  final String userEmail;
  final String role;
  final String? remark;
  final String? virtualIp;
  final String? status;
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
    this.attachmentId,
    required this.networkId,
    required this.deviceId,
    this.virtualIp,
    this.remark,
  });

  factory AppCoreSubnetAttachmentPayload.fromJson(Map<String, dynamic> json) {
    return AppCoreSubnetAttachmentPayload(
      attachmentId: json['attachmentId'] as String?,
      networkId: json['networkId'] as String? ?? '',
      deviceId: json['deviceId'] as String? ?? '',
      virtualIp: json['virtualIp'] as String?,
      remark: json['remark'] as String?,
    );
  }

  final String? attachmentId;
  final String networkId;
  final String deviceId;
  final String? virtualIp;
  final String? remark;

  Map<String, dynamic> toJson() => {
        'attachmentId': attachmentId,
        'networkId': networkId,
        'deviceId': deviceId,
        'virtualIp': virtualIp,
        'remark': remark,
      };
}

class AppCoreNetworkDetailPayload {
  const AppCoreNetworkDetailPayload({
    required this.networkId,
    required this.name,
    this.description,
    this.defaultSubnetId,
    this.defaultSubnetCidr,
    this.subnets = const [],
    this.members = const [],
  });

  factory AppCoreNetworkDetailPayload.fromJson(Map<String, dynamic> json) {
    return AppCoreNetworkDetailPayload(
      networkId: json['networkId'] as String? ?? '',
      name: json['name'] as String? ?? '',
      description: json['description'] as String?,
      defaultSubnetId: json['defaultSubnetId'] as String?,
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
  final String? description;
  final String? defaultSubnetId;
  final String? defaultSubnetCidr;
  final List<AppCoreSubnetPayload> subnets;
  final List<AppCoreNetworkMemberPayload> members;

  Map<String, dynamic> toJson() => {
        'networkId': networkId,
        'name': name,
        'description': description,
        'defaultSubnetId': defaultSubnetId,
        'defaultSubnetCidr': defaultSubnetCidr,
        'subnets': subnets.map((item) => item.toJson()).toList(),
        'members': members.map((item) => item.toJson()).toList(),
      };
}

class AppCoreSubnetPayload {
  const AppCoreSubnetPayload({
    this.subnetId,
    required this.networkId,
    this.name,
    required this.cidr,
    this.remark,
    this.gatewayIp,
    this.allocationStartIp,
    this.allocationEndIp,
    required this.isDefault,
    this.status,
  });

  factory AppCoreSubnetPayload.fromJson(Map<String, dynamic> json) {
    return AppCoreSubnetPayload(
      subnetId: json['subnetId'] as String?,
      networkId: json['networkId'] as String? ?? '',
      name: json['name'] as String?,
      cidr: json['cidr'] as String? ?? '',
      remark: json['remark'] as String?,
      gatewayIp: json['gatewayIp'] as String?,
      allocationStartIp: json['allocationStartIp'] as String?,
      allocationEndIp: json['allocationEndIp'] as String?,
      isDefault: json['isDefault'] as bool? ?? false,
      status: json['status'] as String?,
    );
  }

  final String? subnetId;
  final String networkId;
  final String? name;
  final String cidr;
  final String? remark;
  final String? gatewayIp;
  final String? allocationStartIp;
  final String? allocationEndIp;
  final bool isDefault;
  final String? status;

  Map<String, dynamic> toJson() => {
        'subnetId': subnetId,
        'networkId': networkId,
        'name': name,
        'cidr': cidr,
        'remark': remark,
        'gatewayIp': gatewayIp,
        'allocationStartIp': allocationStartIp,
        'allocationEndIp': allocationEndIp,
        'isDefault': isDefault,
        'status': status,
      };
}

class AppCoreNetworkMemberPayload {
  const AppCoreNetworkMemberPayload({
    this.memberId,
    this.networkId,
    this.attachmentId,
    required this.deviceId,
    required this.role,
    this.createdAt,
    this.status,
    this.virtualIp,
    this.remark,
  });

  factory AppCoreNetworkMemberPayload.fromJson(Map<String, dynamic> json) {
    return AppCoreNetworkMemberPayload(
      memberId: json['memberId'] as String?,
      networkId: json['networkId'] as String?,
      attachmentId: json['attachmentId'] as String?,
      deviceId: json['deviceId'] as String? ?? '',
      role: json['role'] as String? ?? '',
      createdAt: (json['createdAt'] as num?)?.toInt(),
      status: json['status'] as String?,
      virtualIp: json['virtualIp'] as String?,
      remark: json['remark'] as String?,
    );
  }

  final String? memberId;
  final String? networkId;
  final String? attachmentId;
  final String deviceId;
  final String role;
  final int? createdAt;
  final String? status;
  final String? virtualIp;
  final String? remark;

  Map<String, dynamic> toJson() => {
        'memberId': memberId,
        'networkId': networkId,
        'attachmentId': attachmentId,
        'deviceId': deviceId,
        'role': role,
        'createdAt': createdAt,
        'status': status,
        'virtualIp': virtualIp,
        'remark': remark,
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

class AppCorePlatformDoctorPayload {
  const AppCorePlatformDoctorPayload({
    required this.platform,
    required this.tunnelBackend,
    this.checks = const [],
  });

  factory AppCorePlatformDoctorPayload.fromJson(Map<String, dynamic> json) {
    return AppCorePlatformDoctorPayload(
      platform: AppCorePlatformPayload.fromJson(_readMap(json['platform'])),
      tunnelBackend: AppCoreTunnelBackendDiagnosticsPayload.fromJson(
        _readMap(json['tunnelBackend']),
      ),
      checks: _readMapList(json['checks'])
          .map(AppCorePlatformCheckPayload.fromJson)
          .toList(growable: false),
    );
  }

  final AppCorePlatformPayload platform;
  final AppCoreTunnelBackendDiagnosticsPayload tunnelBackend;
  final List<AppCorePlatformCheckPayload> checks;
}

class AppCorePlatformInstallPlanPayload {
  const AppCorePlatformInstallPlanPayload({
    required this.platform,
    this.packages = const [],
    this.supportedDriverModes = const [],
    this.warnings = const [],
  });

  factory AppCorePlatformInstallPlanPayload.fromJson(
    Map<String, dynamic> json,
  ) {
    return AppCorePlatformInstallPlanPayload(
      platform: AppCorePlatformPayload.fromJson(_readMap(json['platform'])),
      packages: _readStringList(json['packages']),
      supportedDriverModes: _readStringList(json['supportedDriverModes']),
      warnings: _readStringList(json['warnings']),
    );
  }

  final AppCorePlatformPayload platform;
  final List<String> packages;
  final List<String> supportedDriverModes;
  final List<String> warnings;
}

class AppCorePlatformPayload {
  const AppCorePlatformPayload({
    required this.os,
    this.distroId,
    this.versionId,
    this.idLike = const [],
    this.family,
    this.kernelRelease,
    this.packageManager,
  });

  factory AppCorePlatformPayload.fromJson(Map<String, dynamic> json) {
    return AppCorePlatformPayload(
      os: json['os'] as String? ?? '',
      distroId: json['distroId'] as String?,
      versionId: json['versionId'] as String?,
      idLike: _readStringList(json['idLike']),
      family: json['family'] as String?,
      kernelRelease: json['kernelRelease'] as String?,
      packageManager: json['packageManager'] as String?,
    );
  }

  final String os;
  final String? distroId;
  final String? versionId;
  final List<String> idLike;
  final String? family;
  final String? kernelRelease;
  final String? packageManager;
}

class AppCoreTunnelBackendDiagnosticsPayload {
  const AppCoreTunnelBackendDiagnosticsPayload({
    required this.name,
    this.executionMode,
    this.executionBackend,
    this.interfaceName,
    required this.isUp,
    required this.plannedPeerCount,
    required this.recentCommandCount,
  });

  factory AppCoreTunnelBackendDiagnosticsPayload.fromJson(
    Map<String, dynamic> json,
  ) {
    return AppCoreTunnelBackendDiagnosticsPayload(
      name: json['name'] as String? ?? '',
      executionMode: json['executionMode'] as String?,
      executionBackend: json['executionBackend'] as String?,
      interfaceName: json['interfaceName'] as String?,
      isUp: json['isUp'] as bool? ?? false,
      plannedPeerCount: (json['plannedPeerCount'] as num?)?.toInt() ?? 0,
      recentCommandCount: (json['recentCommandCount'] as num?)?.toInt() ?? 0,
    );
  }

  final String name;
  final String? executionMode;
  final String? executionBackend;
  final String? interfaceName;
  final bool isUp;
  final int plannedPeerCount;
  final int recentCommandCount;
}

class AppCorePlatformCheckPayload {
  const AppCorePlatformCheckPayload({
    required this.name,
    required this.status,
    required this.detail,
  });

  factory AppCorePlatformCheckPayload.fromJson(Map<String, dynamic> json) {
    return AppCorePlatformCheckPayload(
      name: json['name'] as String? ?? '',
      status: json['status'] as String? ?? '',
      detail: json['detail'] as String? ?? '',
    );
  }

  final String name;
  final String status;
  final String detail;
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
