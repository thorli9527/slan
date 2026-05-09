class ControlTransportStatus {
  const ControlTransportStatus({
    required this.mqttCredentialReady,
    required this.controlSessionReady,
    required this.ready,
    required this.missing,
    required this.mqttConnected,
    this.mqttExpiresAt,
    this.mqttLastError,
    this.mqttLastMessageType,
    this.activeNetworkId,
    this.deviceId,
  });

  final bool mqttCredentialReady;
  final bool controlSessionReady;
  final bool ready;
  final List<String> missing;
  final bool mqttConnected;
  final int? mqttExpiresAt;
  final String? mqttLastError;
  final String? mqttLastMessageType;
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
      mqttConnected: json['mqttConnected'] == true,
      mqttExpiresAt: json['mqttExpiresAt'] as int?,
      mqttLastError: json['mqttLastError'] as String?,
      mqttLastMessageType: json['mqttLastMessageType'] as String?,
      activeNetworkId: json['activeNetworkId'] as String?,
      deviceId: json['deviceId'] as String?,
    );
  }
}
