enum WireGuardTunnelActionPhase {
  accepted,
  configured,
  started,
  verified,
  failed,
  pendingVerification;

  static WireGuardTunnelActionPhase fromWireValue(String? value) {
    return switch (value) {
      'accepted' => WireGuardTunnelActionPhase.accepted,
      'configured' => WireGuardTunnelActionPhase.configured,
      'started' => WireGuardTunnelActionPhase.started,
      'verified' => WireGuardTunnelActionPhase.verified,
      'failed' => WireGuardTunnelActionPhase.failed,
      'pending_verification' => WireGuardTunnelActionPhase.pendingVerification,
      _ => WireGuardTunnelActionPhase.accepted,
    };
  }
}

class WireGuardTunnelActionResult {
  const WireGuardTunnelActionResult({
    required this.action,
    required this.accepted,
    required this.phase,
    required this.source,
    required this.detail,
    required this.connectionStatus,
    required this.hasConfiguration,
    this.configurationPeerVirtualIp,
    this.runtimeState,
    this.backendName,
    this.backendState,
    this.backendExecutionMode,
    this.backendExecutionBackend,
    this.backendInterfaceName,
    this.backendIsUp,
    this.backendPlannedPeerCount,
    this.backendRecentCommandCount,
    this.runtimeLastError,
  });

  final String action;
  final bool accepted;
  final WireGuardTunnelActionPhase phase;
  final String source;
  final String detail;
  final String connectionStatus;
  final bool hasConfiguration;
  final String? configurationPeerVirtualIp;
  final String? runtimeState;
  final String? backendName;
  final String? backendState;
  final String? backendExecutionMode;
  final String? backendExecutionBackend;
  final String? backendInterfaceName;
  final bool? backendIsUp;
  final int? backendPlannedPeerCount;
  final int? backendRecentCommandCount;
  final String? runtimeLastError;

  factory WireGuardTunnelActionResult.fromJson(Map<Object?, Object?> json) {
    return WireGuardTunnelActionResult(
      action: json['action'] as String? ?? 'unknown',
      accepted: json['accepted'] as bool? ?? false,
      phase: WireGuardTunnelActionPhase.fromWireValue(
        json['phase'] as String?,
      ),
      source: json['source'] as String? ?? 'native-backend',
      detail: json['detail'] as String? ?? 'No action detail returned',
      connectionStatus: json['connectionStatus'] as String? ?? 'unknown',
      hasConfiguration: json['hasConfiguration'] as bool? ?? false,
      configurationPeerVirtualIp: json['configurationPeerVirtualIp'] as String?,
      runtimeState: json['runtimeState'] as String?,
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
      runtimeLastError: json['runtimeLastError'] as String?,
    );
  }
}
