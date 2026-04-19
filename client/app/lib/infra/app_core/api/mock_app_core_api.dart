/// AppCore 的本地 mock 实现。
///
/// 用于前端页面在后端与 Rust 核心尚未完全接通前进行联调和占位展示。
library slan_app.infra.app_core.mock_api;

import 'app_core_api.dart';
import '../models/models.dart';

class MockAppCoreApi implements AppCoreApi {
  SessionModel? _session;
  DeviceModel? _device;
  NodeModel? _node;
  final List<NetworkModel> _networks = [];

  @override
  Future<SessionModel> register({
    required String email,
    required String password,
  }) async {
    // 这里直接把邮箱当作 userId 返回，方便前端联调时观察状态变化。
    _session = SessionModel(
      userId: email,
      accessToken: 'mock-access-token',
      refreshToken: 'mock-refresh-token',
      expiresIn: 3600,
    );
    return _session!;
  }

  @override
  Future<SessionModel> login({
    required String email,
    required String password,
  }) async {
    // mock 登录会复用当前已注册设备，以模拟“登录后恢复本机设备状态”。
    _session = SessionModel(
      userId: email,
      accessToken: 'mock-access-token',
      refreshToken: 'mock-refresh-token',
      expiresIn: 3600,
      deviceId: _device?.deviceId,
    );
    return _session!;
  }

  @override
  Future<DeviceModel> registerDevice({
    required String name,
    required String platform,
    required String machineId,
    required String publicKey,
  }) async {
    // mock 里用 machineId 直接充当 deviceId，简化页面联调。
    _device = DeviceModel(
      deviceId: machineId,
      name: name,
      platform: platform,
      status: 'online',
      virtualIp: '100.64.0.10',
      publicKey: publicKey,
    );
    return _device!;
  }

  @override
  Future<NodeModel> registerNode({
    required String deviceId,
    required String nodeId,
    required String nodePublicKey,
    List<String> capabilities = const [],
  }) async {
    _node = NodeModel(
      nodeId: nodeId,
      deviceId: deviceId,
      nodePublicKey: nodePublicKey,
      networkIds: _networks.map((network) => network.networkId).toList(),
      capabilities: List.unmodifiable(capabilities),
    );
    return _node!;
  }

  @override
  Future<List<NetworkModel>> listNetworks() async =>
      List.unmodifiable(_networks);

  @override
  Future<NetworkModel> createNetwork({
    required String name,
    String cidr = '100.64.0.0/24',
  }) async {
    // mock 创建网络时，如果当前设备已存在，则自动把它放进成员列表。
    final network = NetworkModel(
      networkId: 'net-${_networks.length + 1}',
      name: name,
      cidr: cidr,
      members: _device == null
          ? const []
          : [
              NetworkMemberModel(
                deviceId: _device!.deviceId,
                role: 'owner',
                virtualIp: _device!.virtualIp,
              ),
            ],
    );
    _networks.add(network);
    return network;
  }

  @override
  Future<BootstrapModel> bootstrap({
    required String nodeId,
    required String networkId,
  }) async {
    // 如果前面没有显式注册设备，则按当前节点构造一个默认设备视图。
    final device = _device ??
        DeviceModel(
          deviceId: _node?.deviceId ?? 'mock-device',
          name: 'mock-device',
          platform: 'macos',
          status: 'online',
          virtualIp: '100.64.0.10',
        );
    return BootstrapModel(
      device: device,
      networks: List.unmodifiable(_networks),
      controlPlane: const ControlPlaneConfigModel(
        wsUrl: 'ws://127.0.0.1:8080/control/ws',
        sessionToken: 'mock-control-session-token',
        heartbeatSeconds: 15,
      ),
      stunServers: const ['stun:stun.l.google.com:19302'],
      relay: const RelayConfigModel(
        defaultClusterId: 'cn-local-a',
        countries: [
          RelayCountryModel(
            countryCode: 'CN',
            countryName: 'China',
            cities: [
              RelayCityModel(
                cityCode: 'local',
                cityName: 'Local',
                clusters: [
                  RelayClusterModel(
                    clusterId: 'cn-local-a',
                    clusterName: 'CN Local A',
                    nodes: [
                      RelayNodeModel(
                        nodeId: 'relay-cn-local-udp',
                        transport: 'udp',
                        address: '127.0.0.1:9000',
                        priority: 10,
                      ),
                      RelayNodeModel(
                        nodeId: 'relay-cn-local-tcp',
                        transport: 'tcp',
                        address: '127.0.0.1:9001',
                        priority: 20,
                      ),
                    ],
                  ),
                ],
              ),
            ],
          ),
        ],
      ),
    );
  }

  @override
  Future<BootstrapModel> controlSync({
    required String nodeId,
    required String networkId,
  }) {
    return bootstrap(nodeId: nodeId, networkId: networkId);
  }

  @override
  Future<ControlStatusModel> controlStatus() async {
    return const ControlStatusModel(
      status: 'configured',
      wsUrl: 'ws://127.0.0.1:8080/control/ws',
      heartbeatSeconds: 15,
      sessionTokenPresent: true,
      networkMapPresent: true,
      networkId: 'net-1',
      nodeId: 'node-1',
      deviceId: 'mock-device',
      peerCount: 1,
      connectPlanCount: 1,
      connectPlans: [
        ControlConnectPlanModel(
          peerNodeId: 'peer-1',
          preferDirect: true,
          pathCount: 1,
          preferredPath: ControlPathOptionModel(
            pathType: 'direct_udp',
            endpoint: '127.0.0.1:40000',
            priority: 10,
          ),
          derpClusterId: 'cn-local-a',
          preferredDerpNodeIds: ['relay-cn-local-udp'],
          relayTicketId: 'ticket-1',
        ),
      ],
    );
  }

  @override
  Future<RelayTicketModel> issueRelayTicket({
    required String networkId,
    required String srcNodeId,
    required String dstNodeId,
    required String reason,
  }) async {
    // mock 票据只保证字段齐全，便于前端验证 relay 回退链路。
    return RelayTicketModel(
      ticketId: 'ticket-$networkId-$dstNodeId',
      networkId: networkId,
      sessionId: 'session-$srcNodeId-$dstNodeId',
      srcNodeId: srcNodeId,
      dstNodeId: dstNodeId,
      derpClusterId: 'cn-local-a',
      countryCode: 'CN',
      cityCode: 'local',
      allowedDerpNodeIds: const ['relay-cn-local-udp', 'relay-cn-local-tcp'],
      relayUrl: 'udp://127.0.0.1:9000',
      expiresAt: DateTime.now()
          .toUtc()
          .add(const Duration(minutes: 10))
          .toIso8601String(),
      sessionKey: '$srcNodeId:$dstNodeId:$networkId',
      signature: 'mock-signature',
    );
  }

  @override
  Future<ConnectionStateModel> connect({
    required String networkId,
    required String peerNodeId,
  }) async {
    // 约定：
    // 1. 以 fail- 开头时，模拟直连失败，供页面演示 fallback；
    // 2. 以 relay- 开头时，模拟 relay 连接成功；
    // 3. 其他情况默认模拟 P2P 直连成功。
    if (peerNodeId.startsWith('fail-')) {
      return const ConnectionStateModel.failed('p2p handshake timeout');
    }
    if (peerNodeId.startsWith('relay-')) {
      return const ConnectionStateModel.connected(ConnectionPathModel.relay);
    }
    return const ConnectionStateModel.connected(ConnectionPathModel.p2p);
  }

  @override
  Future<DataPlaneProbeModel> probe({
    required String payload,
    int? probeTimeoutMs,
  }) async {
    final now = DateTime.now().millisecondsSinceEpoch;
    return DataPlaneProbeModel(
      probeId: 'mock-probe-$now',
      sampledAtMs: now,
      activePath: const {
        'relay': {'peer_node_id': 'relay-peer-node-1'}
      },
      bytesSent: payload.length,
      replyObserved: true,
      replyBytesReceived: payload.length,
      replySampledAtMs: now + 1,
      replyRttMs: 1,
      tunnelPeerVirtualIp: '100.64.0.2',
      observedRttMs: 12,
      packetLossPpm: 0,
      pathScore: 100,
      derpClusterId: null,
      derpNodeId: null,
    );
  }

  @override
  Future<int> send({
    required String payload,
  }) async {
    return payload.length;
  }

  @override
  Future<void> disconnect() async {}
}
