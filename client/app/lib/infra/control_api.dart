/// 面向控制面的更薄一层接口抽象。
///
/// 这层更接近业务动作语义，便于页面或控制器直接调用。
abstract class ControlApi {
  Future<void> register(String email, String password);
  Future<void> login(String email, String password);
  Future<void> registerDevice({
    required String name,
    required String platform,
    required String machineId,
    required String publicKey,
  });
  Future<void> createNetwork(String name, {String cidr = '100.64.0.0/24'});
  Future<void> loadBootstrap({
    required String nodeId,
    required String networkId,
  });
  Future<void> issueRelayTicket({
    required String networkId,
    required String srcNodeId,
    required String dstNodeId,
    required String reason,
  });
}
