class ControlTransportStatus {
  const ControlTransportStatus({
    required this.mqttCredentialReady,
    required this.controlSessionReady,
    required this.ready,
    required this.missing,
    this.mqttExpiresAt,
    this.activeNetworkId,
    this.deviceId,
  });

  final bool mqttCredentialReady;
  final bool controlSessionReady;
  final bool ready;
  final List<String> missing;
  final int? mqttExpiresAt;
  final String? activeNetworkId;
  final String? deviceId;

  factory ControlTransportStatus.fromJson(Map<String, Object?> json) {
    return ControlTransportStatus(
      mqttCredentialReady: json['mqttCredentialReady'] == true,
      controlSessionReady: json['controlSessionReady'] == true,
      ready: json['ready'] == true,
      missing: (json['missing'] as List? ?? const [])
          .whereType<String>()
          .toList(growable: false),
      mqttExpiresAt: json['mqttExpiresAt'] as int?,
      activeNetworkId: json['activeNetworkId'] as String?,
      deviceId: json['deviceId'] as String?,
    );
  }
}
