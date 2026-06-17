/// 本地控制通道状态。
///
/// 控制通道负责客户端和服务端之间的 MQTT 下行/上行消息同步。UI 使用这个
/// 状态判断是否已经具备 MQTT 凭证、控制会话和活跃网络等必要条件。
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

  /// MQTT 用户名、密码、broker 地址等凭证是否已就绪。
  final bool mqttCredentialReady;

  /// 设备控制会话是否已在服务端建立。
  final bool controlSessionReady;

  /// 控制通道整体是否满足工作条件。
  final bool ready;

  /// 未满足的条件列表，用于诊断。
  final List<String> missing;

  /// MQTT 当前是否已连接。
  final bool mqttConnected;

  /// MQTT 凭证过期时间，Unix 秒。
  final int? mqttExpiresAt;

  /// 最近一次 MQTT 连接或订阅错误。
  final String? mqttLastError;

  /// 最近收到的下行控制消息类型。
  final String? mqttLastMessageType;

  /// 当前活跃网络 ID。
  final String? activeNetworkId;

  /// 当前设备 ID。
  final String? deviceId;

  /// 从本地服务 JSON 响应解析控制通道状态。
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
