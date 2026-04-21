/// 登录或注册后得到的会话模型。
class SessionModel {
  const SessionModel({
    required this.userId,
    required this.accessToken,
    this.refreshToken,
    required this.expiresIn,
    this.deviceId,
    this.userLabel,
  });

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

  factory SessionModel.fromJson(Map<String, dynamic> json) {
    return SessionModel(
      userId: json['userId'] as String? ?? '',
      accessToken: json['accessToken'] as String? ?? '',
      refreshToken: json['refreshToken'] as String?,
      expiresIn: (json['expiresIn'] as num?)?.toInt() ?? 3600,
      deviceId: json['deviceId'] as String?,
      userLabel: json['userLabel'] as String?,
    );
  }
}

/// 设备模型。
class DeviceModel {
  const DeviceModel({
    required this.deviceId,
    required this.name,
    required this.platform,
    required this.status,
    this.virtualIp,
    this.publicKey,
  });

  final String deviceId;
  final String name;
  final String platform;
  final String status;
  final String? virtualIp;
  final String? publicKey;
}

/// 节点模型。
class NodeModel {
  const NodeModel({
    required this.nodeId,
    required this.deviceId,
    required this.nodePublicKey,
    this.networkIds = const [],
    this.capabilities = const [],
  });

  final String nodeId;
  final String deviceId;
  final String nodePublicKey;
  final List<String> networkIds;
  final List<String> capabilities;
}
