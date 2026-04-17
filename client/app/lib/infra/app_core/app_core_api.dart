/// AppCore 的 Flutter 侧抽象接口。
///
/// UI 层只依赖这一层，不直接依赖具体的 Rust FFI 或 HTTP 实现。
import 'models.dart';

abstract class AppCoreApi {
  /// 注册用户并返回会话信息。
  Future<SessionModel> register({
    required String email,
    required String password,
  });

  /// 登录用户并返回会话信息。
  Future<SessionModel> login({
    required String email,
    required String password,
  });

  /// 注册当前设备。
  Future<DeviceModel> registerDevice({
    required String name,
    required String platform,
    required String machineId,
    required String publicKey,
  });

  /// 注册当前节点身份。
  Future<NodeModel> registerNode({
    required String deviceId,
    required String nodeId,
    required String nodePublicKey,
    List<String> capabilities = const [],
  });

  /// 获取当前用户可见的网络列表。
  Future<List<NetworkModel>> listNetworks();

  /// 创建网络，默认会由服务端一并创建默认子网。
  Future<NetworkModel> createNetwork({
    required String name,
    String cidr = '100.64.0.0/24',
  });

  /// 获取指定节点的启动配置。
  Future<BootstrapModel> bootstrap({
    required String nodeId,
    required String networkId,
  });

  /// 向控制面申请 relay 回退票据。
  Future<RelayTicketModel> issueRelayTicket({
    required String networkId,
    required String srcNodeId,
    required String dstNodeId,
    required String reason,
  });

  /// 发起到指定对端节点的连接。
  Future<ConnectionStateModel> connect({
    required String networkId,
    required String peerNodeId,
  });

  /// 断开当前连接。
  Future<void> disconnect();
}
