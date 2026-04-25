/// 登录或注册后得到的会话模型。
class SessionModel {
  const SessionModel({
    required this.userId,
    required this.accessToken,
    this.refreshToken,
    required this.expiresIn,
    this.deviceId,
    this.userLabel,
    this.authenticatedAtMs,
  });

  final String userId;
  final String accessToken;
  final String? refreshToken;
  final int expiresIn;
  final String? deviceId;
  final String? userLabel;
  final int? authenticatedAtMs;

  SessionModel copyWith({
    String? userId,
    String? accessToken,
    String? refreshToken,
    int? expiresIn,
    String? deviceId,
    String? userLabel,
    int? authenticatedAtMs,
  }) {
    return SessionModel(
      userId: userId ?? this.userId,
      accessToken: accessToken ?? this.accessToken,
      refreshToken: refreshToken ?? this.refreshToken,
      expiresIn: expiresIn ?? this.expiresIn,
      deviceId: deviceId ?? this.deviceId,
      userLabel: userLabel ?? this.userLabel,
      authenticatedAtMs: authenticatedAtMs ?? this.authenticatedAtMs,
    );
  }

  Map<String, dynamic> toJson() => {
        'userId': userId,
        'accessToken': accessToken,
        'refreshToken': refreshToken,
        'expiresIn': expiresIn,
        'deviceId': deviceId,
        'userLabel': userLabel,
        'authenticatedAtMs': authenticatedAtMs,
      };

  factory SessionModel.fromJson(Map<String, dynamic> json) {
    return SessionModel(
      userId: json['userId'] as String? ?? '',
      accessToken: json['accessToken'] as String? ?? '',
      refreshToken: json['refreshToken'] as String?,
      expiresIn: (json['expiresIn'] as num?)?.toInt() ?? 3600,
      deviceId: json['deviceId'] as String?,
      userLabel: json['userLabel'] as String?,
      authenticatedAtMs: (json['authenticatedAtMs'] as num?)?.toInt(),
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
    this.ownerEmail,
    this.machineId,
    this.linkStatus,
    this.connectivityProtocol,
    this.joinedAt,
    this.membershipStatus,
    this.networkRole,
    this.createdAt,
    this.networkIds = const [],
  });

  final String deviceId;
  final String name;
  final String platform;
  final String status;
  final String? virtualIp;
  final String? publicKey;
  final String? ownerEmail;
  final String? machineId;
  final String? linkStatus;
  final String? connectivityProtocol;
  final int? joinedAt;
  final String? membershipStatus;
  final String? networkRole;
  final int? createdAt;
  final List<String> networkIds;
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
