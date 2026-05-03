class AndroidVpnPermissionState {
  const AndroidVpnPermissionState._();

  static const granted = 'granted';
  static const needsUserConsent = 'needsUserConsent';
}

class AndroidVpnConsentRequest {
  const AndroidVpnConsentRequest({
    required this.callbackId,
    this.message,
  });

  final String callbackId;
  final String? message;

  factory AndroidVpnConsentRequest.fromJson(Map<String, Object?> json) {
    return AndroidVpnConsentRequest(
      callbackId: json['callbackId'] as String? ?? '',
      message: json['message'] as String?,
    );
  }
}

class AndroidVpnSessionConfig {
  const AndroidVpnSessionConfig({
    required this.sessionName,
    required this.virtualIp,
    required this.prefixLen,
    this.dnsServers = const <String>[],
    this.routes = const <Map<String, Object?>>[],
    this.mtu,
    this.relayEndpointId,
    this.relayTransport,
    this.relayAddress,
    this.relayDataPlane,
  });

  final String sessionName;
  final String virtualIp;
  final int prefixLen;
  final List<String> dnsServers;
  final List<Map<String, Object?>> routes;
  final int? mtu;
  final String? relayEndpointId;
  final String? relayTransport;
  final String? relayAddress;
  final RelayDataPlaneConfig? relayDataPlane;

  factory AndroidVpnSessionConfig.fromJson(Map<String, Object?> json) {
    return AndroidVpnSessionConfig(
      sessionName: json['sessionName'] as String? ?? 'SLAN',
      virtualIp: json['virtualIp'] as String? ?? '',
      prefixLen: json['prefixLen'] as int? ?? 32,
      dnsServers: _stringList(json['dnsServers']),
      routes: _mapList(json['routes']),
      mtu: json['mtu'] as int?,
      relayEndpointId: json['relayEndpointId'] as String?,
      relayTransport: json['relayTransport'] as String?,
      relayAddress: json['relayAddress'] as String?,
      relayDataPlane: _relayDataPlane(json['relayDataPlane']),
    );
  }

  Map<String, Object?> toJson() {
    return {
      'sessionName': sessionName,
      'virtualIp': virtualIp,
      'prefixLen': prefixLen,
      'dnsServers': dnsServers,
      'routes': routes,
      if (mtu != null) 'mtu': mtu,
      if (relayEndpointId != null) 'relayEndpointId': relayEndpointId,
      if (relayTransport != null) 'relayTransport': relayTransport,
      if (relayAddress != null) 'relayAddress': relayAddress,
      if (relayDataPlane != null) 'relayDataPlane': relayDataPlane!.toJson(),
    };
  }
}

class RelayDataPlaneConfig {
  const RelayDataPlaneConfig({
    required this.enabled,
    required this.transport,
    required this.relayAddress,
    required this.localNodeId,
    required this.networkId,
    this.pathPolicy,
    this.peerPaths = const <PeerPathConfig>[],
    this.relayMtu,
    this.maxFramePayload,
    this.sessions = const <RelayPeerSession>[],
  });

  final bool enabled;
  final String transport;
  final String relayAddress;
  final String localNodeId;
  final String networkId;
  final PathPolicy? pathPolicy;
  final List<PeerPathConfig> peerPaths;
  final int? relayMtu;
  final int? maxFramePayload;
  final List<RelayPeerSession> sessions;

  factory RelayDataPlaneConfig.fromJson(Map<String, Object?> json) {
    return RelayDataPlaneConfig(
      enabled: json['enabled'] == true,
      transport: json['transport'] as String? ?? '',
      relayAddress: json['relayAddress'] as String? ?? '',
      localNodeId: json['localNodeId'] as String? ?? '',
      networkId: json['networkId'] as String? ?? '',
      pathPolicy: _pathPolicy(json['pathPolicy']),
      peerPaths: _peerPathConfigs(json['peerPaths']),
      relayMtu: json['relayMtu'] as int?,
      maxFramePayload: json['maxFramePayload'] as int?,
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
      if (pathPolicy != null) 'pathPolicy': pathPolicy!.toJson(),
      'peerPaths': peerPaths.map((path) => path.toJson()).toList(),
      if (relayMtu != null) 'relayMtu': relayMtu,
      if (maxFramePayload != null) 'maxFramePayload': maxFramePayload,
      'sessions': sessions.map((session) => session.toJson()).toList(),
    };
  }
}

class PathKind {
  const PathKind._();

  static const directUdp = 'direct_udp';
  static const relayUdp = 'relay_udp';
  static const relayTcp = 'relay_tcp';
  static const relayHttp3 = 'relay_http3';
  static const relayTls = 'relay_tls';
}

class PathState {
  const PathState._();

  static const disabled = 'disabled';
  static const probing = 'probing';
  static const ready = 'ready';
  static const standby = 'standby';
  static const degraded = 'degraded';
  static const failed = 'failed';
}

class PathPolicy {
  const PathPolicy({
    this.preferred = const <String>[
      PathKind.directUdp,
      PathKind.relayUdp,
      PathKind.relayTcp,
      PathKind.relayHttp3,
      PathKind.relayTls,
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

List<PeerPathConfig> _peerPathConfigs(Object? value) {
  if (value is! List) {
    return const <PeerPathConfig>[];
  }
  return value
      .whereType<Map>()
      .map((item) => PeerPathConfig.fromJson(item.cast<String, Object?>()))
      .toList(growable: false);
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
