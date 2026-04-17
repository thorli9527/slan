/// AppCore 暴露给 Flutter 页面层的模型定义。

/// 登录或注册后得到的会话模型。
class SessionModel {
  const SessionModel({
    required this.userId,
    required this.accessToken,
    this.refreshToken,
    required this.expiresIn,
    this.deviceId,
  });

  final String userId;
  final String accessToken;
  final String? refreshToken;
  final int expiresIn;
  final String? deviceId;
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

/// 网络成员模型。
class NetworkMemberModel {
  const NetworkMemberModel({
    required this.deviceId,
    required this.role,
    this.virtualIp,
  });

  final String deviceId;
  final String role;
  final String? virtualIp;
}

/// 网络模型。
class NetworkModel {
  const NetworkModel({
    required this.networkId,
    required this.name,
    required this.cidr,
    this.members = const [],
  });

  final String networkId;
  final String name;
  final String cidr;
  final List<NetworkMemberModel> members;
}

/// 控制面连接配置。
class ControlPlaneConfigModel {
  const ControlPlaneConfigModel({
    required this.wsUrl,
    required this.heartbeatSeconds,
  });

  final String wsUrl;
  final int heartbeatSeconds;
}

/// Relay 配置模型。
class RelayConfigModel {
  const RelayConfigModel({
    required this.region,
    required this.udpEndpoint,
    this.tcpEndpoint,
  });

  final String region;
  final String udpEndpoint;
  final String? tcpEndpoint;
}

/// Relay 票据模型。
class RelayTicketModel {
  const RelayTicketModel({
    required this.ticketId,
    required this.networkId,
    required this.sessionId,
    required this.srcNodeId,
    required this.dstNodeId,
    required this.relayUrl,
    required this.expiresAt,
    this.sessionKey,
    required this.signature,
  });

  final String ticketId;
  final String networkId;
  final String sessionId;
  final String srcNodeId;
  final String dstNodeId;
  final String relayUrl;
  final String expiresAt;
  final String? sessionKey;
  final String signature;
}

/// 启动配置模型。
class BootstrapModel {
  const BootstrapModel({
    required this.device,
    required this.networks,
    required this.controlPlane,
    required this.stunServers,
    required this.relay,
  });

  final DeviceModel device;
  final List<NetworkModel> networks;
  final ControlPlaneConfigModel controlPlane;
  final List<String> stunServers;
  final RelayConfigModel relay;
}

/// 连接路径枚举，表示当前走 P2P 还是 Relay。
enum ConnectionPathModel {
  p2p,
  relay,
}

/// 连接状态模型。
class ConnectionStateModel {
  const ConnectionStateModel.disconnected()
      : status = 'disconnected',
        path = null,
        reason = null;

  const ConnectionStateModel.connecting()
      : status = 'connecting',
        path = null,
        reason = null;

  const ConnectionStateModel.connected(this.path)
      : status = 'connected',
        reason = null;

  const ConnectionStateModel.failed(this.reason)
      : status = 'failed',
        path = null;

  final String status;
  final ConnectionPathModel? path;
  final String? reason;
}
