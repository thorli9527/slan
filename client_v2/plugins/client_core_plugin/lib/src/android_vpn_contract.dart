/// AndroidVpnPermissionState 是 Flutter 侧识别 Android VpnService 授权状态的字符串常量。
class AndroidVpnPermissionState {
  const AndroidVpnPermissionState._();

  static const granted = 'granted';
  static const needsUserConsent = 'needsUserConsent';
}

/// AndroidVpnConsentRequest 表示一次 Android VPN 授权请求。
class AndroidVpnConsentRequest {
  const AndroidVpnConsentRequest({
    required this.requestId,
    this.message,
  });

  final String requestId;
  final String? message;

  factory AndroidVpnConsentRequest.fromJson(Map<String, Object?> json) {
    return AndroidVpnConsentRequest(
      requestId: json['requestId'] as String? ?? '',
      message: json['message'] as String?,
    );
  }
}

const _resolverConfigKey = 'resolver';

/// AndroidVpnSessionConfig 是启动 Android VpnService 所需的完整网络配置。
///
/// 这里仅透传平台协议里的 `resolver` 负载；
/// Flutter 层不再展开或管理 zone / record 级别数据。
class AndroidVpnSessionConfig {
  const AndroidVpnSessionConfig({
    required this.sessionName,
    required this.virtualIp,
    required this.prefixLen,
    this.networkConfigs = const <PlatformDeviceNetworkConfig>[],
    Map<String, Object?>? resolverConfigPayload,
    this.routes = const <Map<String, Object?>>[],
    this.mtu,
    this.relayEndpointId,
    this.relayTransport,
    this.relayAddress,
    this.aclPolicies = const <PlatformAclPolicy>[],
    this.relayDataPlane,
  }) : _resolverConfigPayload = resolverConfigPayload;

  final String sessionName;
  final String virtualIp;
  final int prefixLen;
  final List<PlatformDeviceNetworkConfig> networkConfigs;
  final Map<String, Object?>? _resolverConfigPayload;
  final List<Map<String, Object?>> routes;
  final int? mtu;
  final String? relayEndpointId;
  final String? relayTransport;
  final String? relayAddress;
  final List<PlatformAclPolicy> aclPolicies;
  final RelayDataPlaneConfig? relayDataPlane;

  factory AndroidVpnSessionConfig.fromJson(Map<String, Object?> json) {
    return AndroidVpnSessionConfig(
      sessionName: json['sessionName'] as String? ?? 'SLAN',
      virtualIp: _virtualIp(json['virtualIp']),
      prefixLen: json['prefixLen'] as int? ?? 32,
      networkConfigs: _platformDeviceNetworkConfigs(json['networkConfigs']),
      resolverConfigPayload: _jsonMapPayload(json[_resolverConfigKey]),
      routes: _mapList(json['routes']),
      mtu: json['mtu'] as int?,
      relayEndpointId: json['relayEndpointId'] as String?,
      relayTransport: json['relayTransport'] as String?,
      relayAddress: json['relayAddress'] as String?,
      aclPolicies: _platformAclPolicies(json['aclPolicies']),
      relayDataPlane: _relayDataPlane(json['relayDataPlane']),
    );
  }

  Map<String, Object?> toJson() {
    return {
      'sessionName': sessionName,
      'virtualIp': virtualIp,
      'prefixLen': prefixLen,
      'networkConfigs':
          networkConfigs.map((config) => config.toJson()).toList(),
      _resolverConfigKey: _resolverConfigPayload ?? const <String, Object?>{},
      'routes': routes,
      if (mtu != null) 'mtu': mtu,
      if (relayEndpointId != null) 'relayEndpointId': relayEndpointId,
      if (relayTransport != null) 'relayTransport': relayTransport,
      if (relayAddress != null) 'relayAddress': relayAddress,
      'aclPolicies': aclPolicies.map((policy) => policy.toJson()).toList(),
      if (relayDataPlane != null) 'relayDataPlane': relayDataPlane!.toJson(),
    };
  }
}

String _virtualIp(Object? value) {
  final text = value is String ? value.trim() : '';
  if (text.isEmpty) {
    return '';
  }
  return text.split('/').first.trim();
}

typedef PlatformNetworkConfig = AndroidVpnSessionConfig;

/// PlatformDeviceNetworkConfig 是单个虚拟网络配置摘要。
class PlatformDeviceNetworkConfig {
  const PlatformDeviceNetworkConfig({
    required this.networkId,
    required this.deviceId,
    this.networkName,
    this.intraGroupPolicy,
    this.configVersion,
    this.globalIp,
    this.globalName,
    this.peerCount = 0,
    this.securityRuleCount = 0,
    this.relayCandidateCount = 0,
  });

  final String networkId;
  final String deviceId;
  final String? networkName;
  final String? intraGroupPolicy;
  final int? configVersion;
  final String? globalIp;
  final String? globalName;
  final int peerCount;
  final int securityRuleCount;
  final int relayCandidateCount;

  factory PlatformDeviceNetworkConfig.fromJson(Map<String, Object?> json) {
    return PlatformDeviceNetworkConfig(
      networkId: json['networkId'] as String? ?? '',
      deviceId: json['deviceId'] as String? ?? '',
      networkName: json['networkName'] as String?,
      intraGroupPolicy: json['intraGroupPolicy'] as String?,
      configVersion: json['configVersion'] as int?,
      globalIp: json['globalIp'] as String?,
      globalName: json['globalName'] as String?,
      peerCount: json['peerCount'] as int? ?? 0,
      securityRuleCount: json['securityRuleCount'] as int? ?? 0,
      relayCandidateCount: json['relayCandidateCount'] as int? ?? 0,
    );
  }

  Map<String, Object?> toJson() {
    return {
      'networkId': networkId,
      'deviceId': deviceId,
      if (networkName != null) 'networkName': networkName,
      if (intraGroupPolicy != null) 'intraGroupPolicy': intraGroupPolicy,
      if (configVersion != null) 'configVersion': configVersion,
      if (globalIp != null) 'globalIp': globalIp,
      if (globalName != null) 'globalName': globalName,
      'peerCount': peerCount,
      'securityRuleCount': securityRuleCount,
      'relayCandidateCount': relayCandidateCount,
    };
  }
}

/// PlatformAclPolicy 是下发到客户端数据面的安全组策略集合。
class PlatformAclPolicy {
  const PlatformAclPolicy({
    required this.networkId,
    this.rules = const <PlatformAclRule>[],
  });

  final String networkId;
  final List<PlatformAclRule> rules;

  factory PlatformAclPolicy.fromJson(Map<String, Object?> json) {
    return PlatformAclPolicy(
      networkId: json['networkId'] as String? ?? '',
      rules: _platformAclRules(json['rules']),
    );
  }

  Map<String, Object?> toJson() {
    return {
      'networkId': networkId,
      'rules': rules.map((rule) => rule.toJson()).toList(),
    };
  }
}

/// PlatformAclRule 是客户端数据面实际执行的安全组规则。
class PlatformAclRule {
  const PlatformAclRule({
    required this.ruleId,
    required this.securityGroupId,
    this.direction = '',
    this.priority = 0,
    this.action = '',
    this.protocol = '',
    this.portFrom = 0,
    this.portTo = 0,
    this.peerType = '',
    this.peerValue = '',
    this.sourceType = '',
    this.sourceValue = '',
    this.enabled = false,
    this.resolvedPeerNodeId,
    this.resolvedPeerVirtualIps = const <String>[],
  });

  final String ruleId;
  final String securityGroupId;
  final String direction;
  final int priority;
  final String action;
  final String protocol;
  final int portFrom;
  final int portTo;
  final String peerType;
  final String peerValue;
  final String sourceType;
  final String sourceValue;
  final bool enabled;
  final String? resolvedPeerNodeId;
  final List<String> resolvedPeerVirtualIps;

  factory PlatformAclRule.fromJson(Map<String, Object?> json) {
    final peerType = json['peerType'] as String? ?? '';
    final peerValue = json['peerValue'] as String? ?? '';
    return PlatformAclRule(
      ruleId: json['ruleId'] as String? ?? '',
      securityGroupId: json['securityGroupId'] as String? ?? '',
      direction: json['direction'] as String? ?? '',
      priority: _intValue(json['priority']),
      action: json['action'] as String? ?? '',
      protocol: json['protocol'] as String? ?? '',
      portFrom: _intValue(json['portFrom']),
      portTo: _intValue(json['portTo']),
      peerType: peerType,
      peerValue: peerValue,
      sourceType: json['sourceType'] as String? ?? peerType,
      sourceValue: json['sourceValue'] as String? ?? peerValue,
      enabled: json['enabled'] == true,
      resolvedPeerNodeId: json['resolvedPeerNodeId'] as String?,
      resolvedPeerVirtualIps: _stringList(json['resolvedPeerVirtualIps']),
    );
  }

  Map<String, Object?> toJson() {
    return {
      'ruleId': ruleId,
      'securityGroupId': securityGroupId,
      'direction': direction,
      'priority': priority,
      'action': action,
      'protocol': protocol,
      'portFrom': portFrom,
      'portTo': portTo,
      'peerType': peerType,
      'peerValue': peerValue,
      'sourceType': sourceType.isEmpty ? peerType : sourceType,
      'sourceValue': sourceValue.isEmpty ? peerValue : sourceValue,
      'enabled': enabled,
      if (resolvedPeerNodeId != null) 'resolvedPeerNodeId': resolvedPeerNodeId,
      'resolvedPeerVirtualIps': resolvedPeerVirtualIps,
    };
  }
}

/// RelayDataPlaneConfig 是 Android 原生 TUN runtime 使用的 relay/direct UDP 数据面配置。
class RelayDataPlaneConfig {
  const RelayDataPlaneConfig({
    required this.enabled,
    required this.transport,
    required this.relayAddress,
    required this.localNodeId,
    required this.networkId,
    this.nodeConfigs = const <NodeConfig>[],
    this.pathPolicy,
    this.peerPaths = const <PeerPathConfig>[],
    this.relayMtu,
    this.maxFramePayload,
    this.aclPolicies = const <PlatformAclPolicy>[],
    this.sessions = const <RelayPeerSession>[],
  });

  final bool enabled;
  final String transport;
  final String relayAddress;
  final String localNodeId;
  final String networkId;
  final List<NodeConfig> nodeConfigs;
  final PathPolicy? pathPolicy;
  final List<PeerPathConfig> peerPaths;
  final int? relayMtu;
  final int? maxFramePayload;
  final List<PlatformAclPolicy> aclPolicies;
  final List<RelayPeerSession> sessions;

  factory RelayDataPlaneConfig.fromJson(Map<String, Object?> json) {
    return RelayDataPlaneConfig(
      enabled: json['enabled'] == true,
      transport: json['transport'] as String? ?? '',
      relayAddress: json['relayAddress'] as String? ?? '',
      localNodeId: json['localNodeId'] as String? ?? '',
      networkId: json['networkId'] as String? ?? '',
      nodeConfigs: _nodeConfigs(json['nodeConfigs']),
      pathPolicy: _pathPolicy(json['pathPolicy']),
      peerPaths: _peerPathConfigs(json['peerPaths']),
      relayMtu: json['relayMtu'] as int?,
      maxFramePayload: json['maxFramePayload'] as int?,
      aclPolicies: _platformAclPolicies(json['aclPolicies']),
      sessions: _relayPeerSessions(json['sessions']),
    );
  }

  Map<String, Object?> toJson() {
    return {
      'enabled': enabled,
      'transport': transport,
      'relayAddress': relayAddress,
      'localNodeId': localNodeId,
      'networkId': networkId,
      'nodeConfigs': nodeConfigs.map((node) => node.toJson()).toList(),
      if (pathPolicy != null) 'pathPolicy': pathPolicy!.toJson(),
      'peerPaths': peerPaths.map((path) => path.toJson()).toList(),
      if (relayMtu != null) 'relayMtu': relayMtu,
      if (maxFramePayload != null) 'maxFramePayload': maxFramePayload,
      'aclPolicies': aclPolicies.map((policy) => policy.toJson()).toList(),
      'sessions': sessions.map((session) => session.toJson()).toList(),
    };
  }
}

/// NodeConfig is an opaque server-managed direct-discovery or relay node.
class NodeConfig {
  const NodeConfig({
    required this.address,
    required this.connectionType,
    required this.transport,
    required this.pathKind,
    this.nodeId = '',
    this.priority = 0,
  });

  final String nodeId;
  final String connectionType;
  final String transport;
  final String pathKind;
  final String address;
  final int priority;

  bool get isValid {
    if (nodeId.trim().isEmpty || address.trim().isEmpty) {
      return false;
    }
    return switch ((connectionType, transport, pathKind)) {
      ('direct', 'udp', 'direct_udp') => true,
      ('relay', 'udp', 'relay_udp') => true,
      ('relay', 'tcp', 'relay_tcp') => true,
      _ => false,
    };
  }

  int get pathRank => switch (pathKind) {
        'direct_udp' => 0,
        'relay_udp' => 1,
        'relay_tcp' => 2,
        _ => 0x7fffffff,
      };

  factory NodeConfig.fromJson(Map<String, Object?> json) {
    return NodeConfig(
      nodeId: json['nodeId'] as String? ?? '',
      connectionType: json['connectionType'] as String? ?? '',
      transport: json['transport'] as String? ?? '',
      pathKind: json['pathKind'] as String? ?? '',
      address: json['address'] as String? ?? '',
      priority: _intValue(json['priority']),
    );
  }

  Map<String, Object?> toJson() {
    return {
      'nodeId': nodeId,
      'connectionType': connectionType,
      'transport': transport,
      'pathKind': pathKind,
      'address': address,
      'priority': priority,
    };
  }
}

/// PathKind 是 Android/Dart 侧与 Rust 控制面一致的路径类型常量。
class PathKind {
  const PathKind._();

  static const lanUdp = 'lan_udp';
  static const ipv6Udp = 'ipv6_udp';
  static const directUdp = 'direct_udp';
  static const relayUdp = 'relay_udp';
  static const derpTcpTls443 = 'derp_tcp_tls_443';
}

/// PathState 是 Android/Dart 侧与 Rust 控制面一致的路径状态常量。
class PathState {
  const PathState._();

  static const disabled = 'disabled';
  static const probing = 'probing';
  static const ready = 'ready';
  static const standby = 'standby';
  static const degraded = 'degraded';
  static const failed = 'failed';
}

/// PathPolicy 是 Android 数据面本地路径切换策略。
class PathPolicy {
  const PathPolicy({
    this.preferred = const <String>[
      PathKind.lanUdp,
      PathKind.ipv6Udp,
      PathKind.directUdp,
      PathKind.relayUdp,
      PathKind.derpTcpTls443,
    ],
    this.fallbackEnabled = true,
    this.probeIntervalMs = 15000,
    this.failoverAfterMs = 30000,
  });

  final List<String> preferred;
  final bool fallbackEnabled;
  final int probeIntervalMs;
  final int failoverAfterMs;

  factory PathPolicy.fromJson(Map<String, Object?> json) {
    return PathPolicy(
      preferred: _stringList(json['preferred']),
      fallbackEnabled: json['fallbackEnabled'] as bool? ?? true,
      probeIntervalMs: json['probeIntervalMs'] as int? ?? 15000,
      failoverAfterMs: json['failoverAfterMs'] as int? ?? 30000,
    );
  }

  Map<String, Object?> toJson() {
    return {
      'preferred': preferred,
      'fallbackEnabled': fallbackEnabled,
      'probeIntervalMs': probeIntervalMs,
      'failoverAfterMs': failoverAfterMs,
    };
  }
}

/// PeerPathConfig 是单个 peer 的路径候选配置。
class PeerPathConfig {
  const PeerPathConfig({
    required this.peerNodeId,
    this.peerVirtualIps = const <String>[],
    this.candidates = const <PathCandidate>[],
  });

  final String peerNodeId;
  final List<String> peerVirtualIps;
  final List<PathCandidate> candidates;

  factory PeerPathConfig.fromJson(Map<String, Object?> json) {
    return PeerPathConfig(
      peerNodeId: json['peerNodeId'] as String? ?? '',
      peerVirtualIps: _stringList(json['peerVirtualIps']),
      candidates: _pathCandidates(json['candidates']),
    );
  }

  Map<String, Object?> toJson() {
    return {
      'peerNodeId': peerNodeId,
      'peerVirtualIps': peerVirtualIps,
      'candidates': candidates.map((candidate) => candidate.toJson()).toList(),
    };
  }
}

/// PathCandidate 是一条直连或转发路径候选。
class PathCandidate {
  const PathCandidate({
    required this.kind,
    this.state = PathState.standby,
    this.endpointId,
    this.address,
    this.sessionId,
    this.transport,
    this.rttMs,
    this.pathScore,
    this.lastOkAtMs,
    this.lastError,
  });

  final String kind;
  final String state;
  final String? endpointId;
  final String? address;
  final String? sessionId;
  final String? transport;
  final int? rttMs;
  final int? pathScore;
  final int? lastOkAtMs;
  final String? lastError;

  factory PathCandidate.fromJson(Map<String, Object?> json) {
    return PathCandidate(
      kind: json['kind'] as String? ?? '',
      state: json['state'] as String? ?? PathState.standby,
      endpointId: json['endpointId'] as String?,
      address: json['address'] as String?,
      sessionId: json['sessionId'] as String?,
      transport: json['transport'] as String?,
      rttMs: json['rttMs'] as int?,
      pathScore: json['pathScore'] as int?,
      lastOkAtMs: json['lastOkAtMs'] as int?,
      lastError: json['lastError'] as String?,
    );
  }

  Map<String, Object?> toJson() {
    return {
      'kind': kind,
      'state': state,
      if (endpointId != null) 'endpointId': endpointId,
      if (address != null) 'address': address,
      if (sessionId != null) 'sessionId': sessionId,
      if (transport != null) 'transport': transport,
      if (rttMs != null) 'rttMs': rttMs,
      if (pathScore != null) 'pathScore': pathScore,
      if (lastOkAtMs != null) 'lastOkAtMs': lastOkAtMs,
      if (lastError != null) 'lastError': lastError,
    };
  }
}

/// RelayPeerSession 是本机到某个 peer 的 relay 会话。
class RelayPeerSession {
  const RelayPeerSession({
    required this.sessionId,
    required this.peerNodeId,
    required this.ticket,
    this.peerVirtualIps = const <String>[],
  });

  final String sessionId;
  final String peerNodeId;
  final List<String> peerVirtualIps;
  final RelayTicket ticket;

  factory RelayPeerSession.fromJson(Map<String, Object?> json) {
    return RelayPeerSession(
      sessionId: json['sessionId'] as String? ?? '',
      peerNodeId: json['peerNodeId'] as String? ?? '',
      peerVirtualIps: _stringList(json['peerVirtualIps']),
      ticket: RelayTicket.fromJson(
        (json['ticket'] as Map?)?.cast<String, Object?>() ??
            const <String, Object?>{},
      ),
    );
  }

  Map<String, Object?> toJson() {
    return {
      'sessionId': sessionId,
      'peerNodeId': peerNodeId,
      'peerVirtualIps': peerVirtualIps,
      'ticket': ticket.toJson(),
    };
  }
}

/// RelayTicket 是 Android 原生 relay 数据面 attach 时使用的授权票据。
class RelayTicket {
  const RelayTicket({
    required this.ticketId,
    required this.networkId,
    required this.sessionId,
    required this.srcNodeId,
    required this.dstNodeId,
    required this.relayUrl,
    required this.expiresAt,
    required this.signature,
    this.derpClusterId,
    this.countryCode,
    this.cityCode,
    this.allowedDerpNodeIds = const <String>[],
    this.sessionKey = '',
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
  final String sessionKey;
  final String signature;

  factory RelayTicket.fromJson(Map<String, Object?> json) {
    return RelayTicket(
      ticketId: json['ticketId'] as String? ?? '',
      networkId: json['networkId'] as String? ?? '',
      sessionId: json['sessionId'] as String? ?? '',
      srcNodeId: json['srcNodeId'] as String? ?? '',
      dstNodeId: json['dstNodeId'] as String? ?? '',
      derpClusterId: json['derpClusterId'] as String?,
      countryCode: json['countryCode'] as String?,
      cityCode: json['cityCode'] as String?,
      allowedDerpNodeIds: _stringList(json['allowedDerpNodeIds']),
      relayUrl: json['relayUrl'] as String? ?? '',
      expiresAt: json['expiresAt'] as String? ?? '',
      sessionKey: json['sessionKey'] as String? ?? '',
      signature: json['signature'] as String? ?? '',
    );
  }

  Map<String, Object?> toJson() {
    return {
      'ticketId': ticketId,
      'networkId': networkId,
      'sessionId': sessionId,
      'srcNodeId': srcNodeId,
      'dstNodeId': dstNodeId,
      if (derpClusterId != null) 'derpClusterId': derpClusterId,
      if (countryCode != null) 'countryCode': countryCode,
      if (cityCode != null) 'cityCode': cityCode,
      'allowedDerpNodeIds': allowedDerpNodeIds,
      'relayUrl': relayUrl,
      'expiresAt': expiresAt,
      'sessionKey': sessionKey,
      'signature': signature,
    };
  }
}

RelayDataPlaneConfig? _relayDataPlane(Object? value) {
  if (value is! Map) {
    return null;
  }
  return RelayDataPlaneConfig.fromJson(value.cast<String, Object?>());
}

PathPolicy? _pathPolicy(Object? value) {
  if (value is! Map) {
    return null;
  }
  return PathPolicy.fromJson(value.cast<String, Object?>());
}

List<PlatformDeviceNetworkConfig> _platformDeviceNetworkConfigs(Object? value) {
  final items = switch (value) {
    List() => value,
    Map() when value['items'] is List => value['items'] as List,
    _ => const <Object?>[],
  };
  return items
      .whereType<Map>()
      .map((item) => PlatformDeviceNetworkConfig.fromJson(
            item.cast<String, Object?>(),
          ))
      .toList();
}

Map<String, Object?>? _jsonMapPayload(Object? value) {
  if (value is Map<String, Object?>) {
    return value;
  }
  if (value is Map) {
    return value.map((key, item) => MapEntry('$key', item));
  }
  return null;
}

List<PlatformAclPolicy> _platformAclPolicies(Object? value) {
  if (value is! List) {
    return const <PlatformAclPolicy>[];
  }
  return value
      .whereType<Map>()
      .map((item) => PlatformAclPolicy.fromJson(item.cast<String, Object?>()))
      .toList(growable: false);
}

List<PlatformAclRule> _platformAclRules(Object? value) {
  if (value is! List) {
    return const <PlatformAclRule>[];
  }
  return value
      .whereType<Map>()
      .map((item) => PlatformAclRule.fromJson(item.cast<String, Object?>()))
      .toList(growable: false);
}

List<PeerPathConfig> _peerPathConfigs(Object? value) {
  if (value is! List) {
    return const <PeerPathConfig>[];
  }
  return value
      .whereType<Map>()
      .map((item) => PeerPathConfig.fromJson(item.cast<String, Object?>()))
      .toList(growable: false);
}

List<NodeConfig> _nodeConfigs(Object? value) {
  if (value is! List) {
    return const <NodeConfig>[];
  }
  final nodes = value
      .whereType<Map>()
      .map((item) => NodeConfig.fromJson(item.cast<String, Object?>()))
      .where((node) => node.isValid)
      .toList();
  nodes.sort((left, right) {
    final pathComparison = left.pathRank.compareTo(right.pathRank);
    if (pathComparison != 0) {
      return pathComparison;
    }
    final priorityComparison = left.priority.compareTo(right.priority);
    if (priorityComparison != 0) {
      return priorityComparison;
    }
    return left.nodeId.compareTo(right.nodeId);
  });
  return List<NodeConfig>.unmodifiable(nodes);
}

List<PathCandidate> _pathCandidates(Object? value) {
  if (value is! List) {
    return const <PathCandidate>[];
  }
  return value
      .whereType<Map>()
      .map((item) => PathCandidate.fromJson(item.cast<String, Object?>()))
      .toList(growable: false);
}

List<RelayPeerSession> _relayPeerSessions(Object? value) {
  if (value is! List) {
    return const <RelayPeerSession>[];
  }
  return value
      .whereType<Map>()
      .map((item) => RelayPeerSession.fromJson(item.cast<String, Object?>()))
      .toList(growable: false);
}

List<String> _stringList(Object? value) {
  if (value is! List) {
    return const <String>[];
  }
  return value.whereType<String>().toList(growable: false);
}

int _intValue(Object? value) {
  if (value is int) {
    return value;
  }
  if (value is num) {
    return value.toInt();
  }
  if (value is String) {
    return int.tryParse(value) ?? 0;
  }
  return 0;
}

List<Map<String, Object?>> _mapList(Object? value) {
  if (value is! List) {
    return const <Map<String, Object?>>[];
  }
  return value
      .whereType<Map>()
      .map((item) => item.cast<String, Object?>())
      .toList(growable: false);
}

class AndroidSocketProtectionReason {
  const AndroidSocketProtectionReason._();

  static const controlPlane = 'controlPlane';
  static const mqtt = 'mqtt';
  static const relayProbe = 'relayProbe';
  static const relayTransport = 'relayTransport';
}

class AndroidSocketProtectionRequest {
  const AndroidSocketProtectionRequest({
    required this.socketFd,
    required this.reason,
  });

  final int socketFd;
  final String reason;

  Map<String, Object?> toJson() {
    return {
      'socketFd': socketFd,
      'reason': reason,
    };
  }
}

class AndroidNetworkEventType {
  const AndroidNetworkEventType._();

  static const permissionRequired = 'permissionRequired';
  static const permissionGranted = 'permissionGranted';
  static const vpnStarted = 'vpnStarted';
  static const vpnStopped = 'vpnStopped';
  static const vpnRevoked = 'vpnRevoked';
  static const connectivityChanged = 'connectivityChanged';
  static const relayChanged = 'relayChanged';
  static const error = 'error';
}

class AndroidNetworkEvent {
  const AndroidNetworkEvent({
    required this.eventType,
    this.message,
    this.runtimeState,
  });

  final String eventType;
  final String? message;
  final Map<String, Object?>? runtimeState;

  factory AndroidNetworkEvent.fromJson(Map<String, Object?> json) {
    final runtimeState = json['runtimeState'];
    return AndroidNetworkEvent(
      eventType: json['eventType'] as String? ?? '',
      message: json['message'] as String?,
      runtimeState:
          runtimeState is Map ? runtimeState.cast<String, Object?>() : null,
    );
  }
}
