/// Flutter 侧控制面请求模型定义。

/// 注册请求体。
class RegisterRequest {
  const RegisterRequest({required this.email, required this.password});

  final String email;
  final String password;

  Map<String, dynamic> toJson() => {
        'email': email,
        'password': password,
      };
}

/// 登录请求体。
class LoginRequest {
  const LoginRequest({required this.email, required this.password});

  final String email;
  final String password;

  Map<String, dynamic> toJson() => {
        'email': email,
        'password': password,
      };
}

/// 创建网络请求体。
class CreateNetworkRequest {
  const CreateNetworkRequest({required this.name, this.cidr = '100.64.0.0/24'});

  final String name;
  final String cidr;

  Map<String, dynamic> toJson() => {
        'name': name,
        'cidr': cidr,
      };
}

/// 注册设备请求体。
class RegisterDeviceRequest {
  const RegisterDeviceRequest({
    required this.name,
    required this.platform,
    required this.machineId,
    required this.publicKey,
  });

  final String name;
  final String platform;
  final String machineId;
  final String publicKey;

  Map<String, dynamic> toJson() => {
        'name': name,
        'platform': platform,
        'machineId': machineId,
        'publicKey': publicKey,
      };
}

/// 申请 relay 票据请求体。
class RelayTicketRequest {
  const RelayTicketRequest({
    required this.networkId,
    required this.srcNodeId,
    required this.dstNodeId,
    required this.reason,
  });

  final String networkId;
  final String srcNodeId;
  final String dstNodeId;
  final String reason;

  Map<String, dynamic> toJson() => {
        'networkId': networkId,
        'srcNodeId': srcNodeId,
        'dstNodeId': dstNodeId,
        'reason': reason,
      };
}
