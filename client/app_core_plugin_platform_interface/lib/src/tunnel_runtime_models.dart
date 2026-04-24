class WireGuardTunnelRuntimeView {
  const WireGuardTunnelRuntimeView({
    required this.state,
    required this.transport,
    this.debugEngineMode,
    this.backendName,
    this.backendState,
    this.backendExecutionMode,
    this.backendExecutionBackend,
    this.backendInterfaceName,
    this.backendIsUp,
    this.backendPlannedPeerCount,
    this.backendRecentCommandCount,
    this.backendLastError,
    this.backendLastStartedAtMs,
    this.backendPeerVirtualIp,
    this.backendSelectedEndpoint,
    required this.peerVirtualIp,
    required this.peerPublicKey,
    this.selectedEndpoint,
    this.interfaceName,
    this.dnsServers = const [],
    this.allowedIps = const [],
    required this.localVirtualIp,
    required this.remoteAddress,
    this.mtu,
    this.interfaceAddresses = const [],
    this.includedRoutes = const [],
    this.packetRxCount = 0,
    this.packetRxBytes = 0,
    this.packetTxCount = 0,
    this.packetTxBytes = 0,
    this.lastPacketAtMs,
    this.lastAppliedAtMs,
    this.lastError,
  });

  final String state;
  final String transport;
  final String? debugEngineMode;
  final String? backendName;
  final String? backendState;
  final String? backendExecutionMode;
  final String? backendExecutionBackend;
  final String? backendInterfaceName;
  final bool? backendIsUp;
  final int? backendPlannedPeerCount;
  final int? backendRecentCommandCount;
  final String? backendLastError;
  final int? backendLastStartedAtMs;
  final String? backendPeerVirtualIp;
  final String? backendSelectedEndpoint;
  final String peerVirtualIp;
  final String peerPublicKey;
  final String? selectedEndpoint;
  final String? interfaceName;
  final List<String> dnsServers;
  final List<String> allowedIps;
  final String localVirtualIp;
  final String remoteAddress;
  final int? mtu;
  final List<String> interfaceAddresses;
  final List<String> includedRoutes;
  final int packetRxCount;
  final int packetRxBytes;
  final int packetTxCount;
  final int packetTxBytes;
  final int? lastPacketAtMs;
  final int? lastAppliedAtMs;
  final String? lastError;

  factory WireGuardTunnelRuntimeView.fromJson(Map<Object?, Object?> json) {
    return WireGuardTunnelRuntimeView(
      state: json['state'] as String,
      transport: json['transport'] as String,
      debugEngineMode: json['debugEngineMode'] as String?,
      backendName: json['backendName'] as String?,
      backendState: json['backendState'] as String?,
      backendExecutionMode: json['backendExecutionMode'] as String?,
      backendExecutionBackend: json['backendExecutionBackend'] as String?,
      backendInterfaceName: json['backendInterfaceName'] as String?,
      backendIsUp: json['backendIsUp'] as bool?,
      backendPlannedPeerCount:
          (json['backendPlannedPeerCount'] as num?)?.toInt(),
      backendRecentCommandCount:
          (json['backendRecentCommandCount'] as num?)?.toInt(),
      backendLastError: json['backendLastError'] as String?,
      backendLastStartedAtMs:
          (json['backendLastStartedAtMs'] as num?)?.toInt(),
      backendPeerVirtualIp: json['backendPeerVirtualIp'] as String?,
      backendSelectedEndpoint: json['backendSelectedEndpoint'] as String?,
      peerVirtualIp: json['peerVirtualIp'] as String,
      peerPublicKey: json['peerPublicKey'] as String,
      selectedEndpoint: json['selectedEndpoint'] as String?,
      interfaceName: json['interfaceName'] as String?,
      dnsServers: (json['dnsServers'] as List<Object?>? ?? const [])
          .whereType<String>()
          .toList(growable: false),
      allowedIps: (json['allowedIps'] as List<Object?>? ?? const [])
          .whereType<String>()
          .toList(growable: false),
      localVirtualIp: json['localVirtualIp'] as String? ?? '',
      remoteAddress: json['remoteAddress'] as String? ?? '',
      mtu: json['mtu'] as int?,
      interfaceAddresses:
          (json['interfaceAddresses'] as List<Object?>? ?? const [])
              .whereType<String>()
              .toList(growable: false),
      includedRoutes: (json['includedRoutes'] as List<Object?>? ?? const [])
          .whereType<String>()
          .toList(growable: false),
      packetRxCount: (json['packetRxCount'] as num?)?.toInt() ?? 0,
      packetRxBytes: (json['packetRxBytes'] as num?)?.toInt() ?? 0,
      packetTxCount: (json['packetTxCount'] as num?)?.toInt() ?? 0,
      packetTxBytes: (json['packetTxBytes'] as num?)?.toInt() ?? 0,
      lastPacketAtMs: (json['lastPacketAtMs'] as num?)?.toInt(),
      lastAppliedAtMs: (json['lastAppliedAtMs'] as num?)?.toInt(),
      lastError: json['lastError'] as String?,
    );
  }
}
