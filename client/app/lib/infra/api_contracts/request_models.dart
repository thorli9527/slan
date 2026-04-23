/// Flutter 侧控制面请求合同定义。
library slan_app.infra.api_contracts.request_models;

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
  const CreateNetworkRequest({
    required this.name,
    this.description,
    this.cidr = '10.0.0.0/16',
    this.bindDeviceId,
  });

  final String name;
  final String? description;
  final String cidr;
  final String? bindDeviceId;

  Map<String, dynamic> toJson() => {
        'name': name,
        if (description != null && description!.isNotEmpty)
          'description': description,
        'cidr': cidr,
        if (bindDeviceId != null && bindDeviceId!.isNotEmpty)
          'bindDeviceId': bindDeviceId,
      };
}

/// 设备加入网络请求体。
class JoinNetworkRequest {
  const JoinNetworkRequest({required this.deviceId});

  final String deviceId;

  Map<String, dynamic> toJson() => {
        'deviceId': deviceId,
      };
}

/// 显式切换活动网络请求体。
class SwitchNetworkRequest {
  const SwitchNetworkRequest({required this.deviceId});

  final String deviceId;

  Map<String, dynamic> toJson() => {
        'deviceId': deviceId,
      };
}

/// 释放设备当前网络接入请求体。
class DeactivateNetworkRequest {
  const DeactivateNetworkRequest({required this.deviceId});

  final String deviceId;

  Map<String, dynamic> toJson() => {
        'deviceId': deviceId,
      };
}

/// 按 owner 邮箱加入网络请求体。
class JoinNetworkByOwnerEmailRequest {
  const JoinNetworkByOwnerEmailRequest({
    required this.ownerEmail,
    required this.deviceId,
  });

  final String ownerEmail;
  final String deviceId;

  Map<String, dynamic> toJson() => {
        'ownerEmail': ownerEmail,
        'deviceId': deviceId,
      };
}

/// 按 join key 加入网络请求体。
class JoinNetworkByKeyRequest {
  const JoinNetworkByKeyRequest({
    required this.joinKey,
    required this.deviceId,
  });

  final String joinKey;
  final String deviceId;

  Map<String, dynamic> toJson() => {
        'joinKey': joinKey,
        'deviceId': deviceId,
      };
}

/// 修改网络基础配置请求体。
class UpdateNetworkRequest {
  const UpdateNetworkRequest({
    this.name,
    this.description,
    required this.cidr,
  });

  final String? name;
  final String? description;
  final String cidr;

  Map<String, dynamic> toJson() => {
        if (name != null && name!.isNotEmpty) 'name': name,
        if (description != null && description!.isNotEmpty)
          'description': description,
        'cidr': cidr,
      };
}

/// 创建子网请求体。
class CreateSubnetRequest {
  const CreateSubnetRequest({
    required this.name,
    required this.cidr,
    this.gatewayIp,
    this.allocationStartIp,
    this.allocationEndIp,
  });

  final String name;
  final String cidr;
  final String? gatewayIp;
  final String? allocationStartIp;
  final String? allocationEndIp;

  Map<String, dynamic> toJson() => {
        'name': name,
        'cidr': cidr,
        if (gatewayIp != null && gatewayIp!.isNotEmpty) 'gatewayIp': gatewayIp,
        if (allocationStartIp != null && allocationStartIp!.isNotEmpty)
          'allocationStartIp': allocationStartIp,
        if (allocationEndIp != null && allocationEndIp!.isNotEmpty)
          'allocationEndIp': allocationEndIp,
      };
}

/// 挂载设备到子网请求体。
class AttachDeviceRequest {
  const AttachDeviceRequest({required this.deviceId});

  final String deviceId;

  Map<String, dynamic> toJson() => {
        'deviceId': deviceId,
      };
}

/// 修改 attachment 虚拟 IP 请求体。
class UpdateAttachmentIPRequest {
  const UpdateAttachmentIPRequest({required this.virtualIp});

  final String virtualIp;

  Map<String, dynamic> toJson() => {
        'virtualIp': virtualIp,
      };
}

/// 修改 attachment 备注请求体。
class UpdateAttachmentRemarkRequest {
  const UpdateAttachmentRemarkRequest({this.remark});

  final String? remark;

  Map<String, dynamic> toJson() => {
        if (remark != null) 'remark': remark,
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

/// 注册节点请求体。
class RegisterNodeRequest {
  const RegisterNodeRequest({
    required this.deviceId,
    required this.nodeId,
    required this.nodePublicKey,
    this.capabilities = const [],
  });

  final String deviceId;
  final String nodeId;
  final String nodePublicKey;
  final List<String> capabilities;

  Map<String, dynamic> toJson() => {
        'deviceId': deviceId,
        'nodeId': nodeId,
        'nodePublicKey': nodePublicKey,
        'capabilities': capabilities,
      };
}

/// 获取启动配置请求体。
class BootstrapRequest {
  const BootstrapRequest({
    required this.nodeId,
    required this.networkId,
  });

  final String nodeId;
  final String networkId;

  Map<String, dynamic> toJson() => {
        'nodeId': nodeId,
        'networkId': networkId,
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
