/// AppCore 的本地 mock 实现。
///
/// 用于前端页面在后端与 Rust 核心尚未完全接通前进行联调和占位展示。
library slan_app.infra.app_core.mock_api;

import 'app_core_api.dart';
import 'dev_defaults.dart';
import '../models/bootstrap_models.dart';
import '../models/connection_models.dart';
import '../models/control_models.dart';
import '../models/diagnostic_models.dart';
import '../models/identity_models.dart';
import '../models/network_models.dart';
import '../models/relay_models.dart';

class MockAppCoreApi implements AppCoreApi {
  SessionModel? _session;
  DeviceModel? _device;
  NodeModel? _node;
  final List<NetworkModel> _networks = [];

  @override
  void restoreSession(SessionModel session) {
    _session = session;
  }

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
  Future<SessionModel> refreshSession({
    required String refreshToken,
    String? deviceId,
  }) async {
    final current = _session;
    _session = SessionModel(
      userId: current?.userId ?? 'mock-user',
      accessToken: 'mock-access-token-refreshed',
      refreshToken: 'mock-refresh-token-refreshed',
      expiresIn: 3600,
      deviceId: deviceId ?? current?.deviceId ?? _device?.deviceId,
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
  Future<List<DeviceModel>> listDevices() async =>
      _device == null ? const [] : [_device!];

  @override
  Future<void> setDeviceNetworkState({
    required String deviceId,
    required String networkId,
    required bool controlReachable,
    required bool networkOnline,
    required bool tunnelUp,
    required bool lastProbeOk,
    String? virtualIp,
    int? reportedAt,
  }) async {
    final current = _device;
    if (current == null || current.deviceId != deviceId) {
      throw StateError('device not found: $deviceId');
    }
    _device = DeviceModel(
      deviceId: current.deviceId,
      name: current.name,
      platform: current.platform,
      status: controlReachable ? 'reachable' : 'offline',
      virtualIp: virtualIp ?? current.virtualIp,
      publicKey: current.publicKey,
      ownerEmail: current.ownerEmail,
      linkStatus: networkOnline ? 'online' : 'offline',
      connectivityProtocol: current.connectivityProtocol,
      joinedAt: current.joinedAt,
      membershipStatus: current.membershipStatus,
      networkRole: current.networkRole,
      createdAt: current.createdAt,
      networkIds: current.networkIds,
      mqtt: current.mqtt,
    );
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
    String cidr = '10.0.0.0/16',
    String? bindDeviceId,
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
                attachmentId:
                    'attach-${_device!.deviceId}-${_networks.length + 1}',
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
  Future<NetworkJoinModel> joinNetwork({
    required String networkId,
    required String deviceId,
  }) async {
    final index =
        _networks.indexWhere((network) => network.networkId == networkId);
    if (index < 0) {
      throw StateError('network not found: $networkId');
    }
    final current = _networks[index];
    final alreadyJoined =
        current.members.any((member) => member.deviceId == deviceId);
    if (alreadyJoined) {
      final member = current.members.firstWhere(
        (member) => member.deviceId == deviceId,
      );
      return NetworkJoinModel(
        networkId: current.networkId,
        deviceId: member.deviceId,
        memberId: member.memberId,
        attachmentId: member.attachmentId,
        virtualIp: member.virtualIp,
      );
    }
    final attachmentId = 'attach-$networkId-$deviceId';
    _networks[index] = NetworkModel(
      networkId: current.networkId,
      name: current.name,
      cidr: current.cidr,
      members: [
        ...current.members,
        NetworkMemberModel(
          attachmentId: attachmentId,
          networkId: networkId,
          deviceId: deviceId,
          role: current.members.isEmpty ? 'owner' : 'member',
        ),
      ],
    );
    return NetworkJoinModel(
      networkId: networkId,
      deviceId: deviceId,
      attachmentId: attachmentId,
    );
  }

  @override
  Future<NetworkJoinModel> joinNetworkByOwnerEmail({
    required String ownerEmail,
    required String deviceId,
  }) async {
    final network = _networks.isNotEmpty
        ? _networks.first
        : NetworkModel(
            networkId: 'mock-owner-network',
            name: 'Owner network',
            cidr: '10.0.0.0/16',
          );
    if (_networks.every((item) => item.networkId != network.networkId)) {
      _networks.add(network);
    }
    return joinNetwork(networkId: network.networkId, deviceId: deviceId);
  }

  @override
  Future<NetworkJoinModel> joinNetworkByKey({
    required String joinKey,
    required String deviceId,
  }) async {
    final network = _networks.isNotEmpty
        ? _networks.first
        : NetworkModel(
            networkId: 'mock-key-network',
            name: 'Joined network',
            cidr: '10.0.0.0/16',
          );
    if (_networks.every((item) => item.networkId != network.networkId)) {
      _networks.add(network);
    }
    return joinNetwork(networkId: network.networkId, deviceId: deviceId);
  }

  @override
  Future<NetworkAssignmentModel> updateAttachmentRemark({
    required String networkId,
    required String attachmentId,
    required String remark,
  }) async {
    final index =
        _networks.indexWhere((network) => network.networkId == networkId);
    if (index < 0) {
      return NetworkAssignmentModel(
        attachmentId: attachmentId,
        networkId: networkId,
        subnetId: 'mock-subnet',
        deviceId: '',
        deviceName: '',
        userId: '',
        userEmail: '',
        role: 'member',
        remark: remark,
      );
    }
    final current = _networks[index];
    _networks[index] = NetworkModel(
      networkId: current.networkId,
      name: current.name,
      cidr: current.cidr,
      description: current.description,
      defaultSubnetId: current.defaultSubnetId,
      joinKeyConfigured: current.joinKeyConfigured,
      members: current.members
          .map(
            (member) => member.attachmentId == attachmentId
                ? NetworkMemberModel(
                    memberId: member.memberId,
                    networkId: member.networkId,
                    attachmentId: member.attachmentId,
                    deviceId: member.deviceId,
                    role: member.role,
                    createdAt: member.createdAt,
                    status: member.status,
                    virtualIp: member.virtualIp,
                    remark: remark,
                  )
                : member,
          )
          .toList(growable: false),
    );
    final member = _networks[index].members.firstWhere(
      (member) => member.attachmentId == attachmentId,
      orElse: () => NetworkMemberModel(
        networkId: networkId,
        attachmentId: attachmentId,
        deviceId: '',
        role: 'member',
        remark: remark,
      ),
    );
    return NetworkAssignmentModel(
      attachmentId: attachmentId,
      networkId: member.networkId ?? networkId,
      subnetId: 'mock-subnet',
      deviceId: member.deviceId,
      deviceName: member.deviceId,
      userId: '',
      userEmail: '',
      role: member.role,
      remark: member.remark,
      virtualIp: member.virtualIp,
      status: member.status,
    );
  }

  @override
  Future<NetworkJoinModel> activateNetwork({
    required String networkId,
    required String deviceId,
  }) async {
    final index =
        _networks.indexWhere((network) => network.networkId == networkId);
    if (index < 0) {
      return NetworkJoinModel(
        networkId: networkId,
        deviceId: deviceId,
        attachmentId: 'mock-attachment-$deviceId',
        virtualIp: '100.64.0.10',
      );
    }
    final current = _networks[index];
    final virtualIp =
        _device?.virtualIp ?? '100.64.0.${current.members.length + 10}';
    _networks[index] = NetworkModel(
      networkId: current.networkId,
      name: current.name,
      cidr: current.cidr,
      description: current.description,
      defaultSubnetId: current.defaultSubnetId,
      joinKeyConfigured: current.joinKeyConfigured,
      members: current.members
          .map(
            (member) => member.deviceId == deviceId
                ? NetworkMemberModel(
                    memberId: member.memberId,
                    networkId: member.networkId,
                    attachmentId: member.attachmentId,
                    deviceId: member.deviceId,
                    role: member.role,
                    createdAt: member.createdAt,
                    status: member.status,
                    virtualIp: virtualIp,
                    remark: member.remark,
                  )
                : member,
          )
          .toList(growable: false),
    );
    if (_device?.deviceId == deviceId) {
      _device = DeviceModel(
        deviceId: _device!.deviceId,
        name: _device!.name,
        platform: _device!.platform,
        status: _device!.status,
        publicKey: _device!.publicKey,
        virtualIp: virtualIp,
      );
    }
    final activated = _networks[index].members.firstWhere(
      (member) => member.deviceId == deviceId,
      orElse: () => NetworkMemberModel(
        networkId: networkId,
        attachmentId: 'mock-attachment-$deviceId',
        deviceId: deviceId,
        role: 'member',
        virtualIp: virtualIp,
      ),
    );
    return NetworkJoinModel(
      networkId: activated.networkId ?? networkId,
      deviceId: activated.deviceId,
      memberId: activated.memberId,
      attachmentId: activated.attachmentId,
      virtualIp: activated.virtualIp,
    );
  }

  @override
  Future<NetworkJoinModel> switchNetwork({
    required String networkId,
    required String deviceId,
  }) async {
    await activateNetwork(networkId: networkId, deviceId: deviceId);
    final network = _networks.firstWhere(
      (network) => network.networkId == networkId,
      orElse: () => NetworkModel(
        networkId: networkId,
        name: networkId,
        cidr: '100.64.0.0/24',
      ),
    );
    final member = network.members.firstWhere(
      (member) => member.deviceId == deviceId,
      orElse: () => NetworkMemberModel(
        networkId: networkId,
        attachmentId: 'mock-attachment-$deviceId',
        deviceId: deviceId,
        role: 'member',
        status: 'active',
        virtualIp: _device?.virtualIp,
      ),
    );
    return NetworkJoinModel(
      networkId: networkId,
      deviceId: deviceId,
      memberId: member.memberId,
      attachmentId: member.attachmentId,
      virtualIp: member.virtualIp,
    );
  }

  @override
  Future<void> deactivateNetwork({
    required String networkId,
    required String deviceId,
  }) async {
    final index =
        _networks.indexWhere((network) => network.networkId == networkId);
    if (index < 0) {
      return;
    }
    final current = _networks[index];
    _networks[index] = NetworkModel(
      networkId: current.networkId,
      name: current.name,
      cidr: current.cidr,
      members: current.members
          .map(
            (member) => member.deviceId == deviceId
                ? NetworkMemberModel(
                    memberId: member.memberId,
                    networkId: member.networkId,
                    attachmentId: member.attachmentId,
                    deviceId: member.deviceId,
                    role: member.role,
                    createdAt: member.createdAt,
                    status: member.status,
                    remark: member.remark,
                  )
                : member,
          )
          .toList(growable: false),
    );
    if (_device?.deviceId == deviceId) {
      _device = DeviceModel(
        deviceId: _device!.deviceId,
        name: _device!.name,
        platform: _device!.platform,
        status: _device!.status,
        publicKey: _device!.publicKey,
      );
    }
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
        wsUrl: kDevControlWsUrl,
        sessionToken: 'mock-control-session-token',
        heartbeatSeconds: 15,
      ),
      stunServers: const [kDevStunServer],
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
                        address: kDevRelayUdpAddress,
                        priority: 10,
                      ),
                      RelayNodeModel(
                        nodeId: 'relay-cn-local-tcp',
                        transport: 'tcp',
                        address: kDevRelayTcpAddress,
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
      wsUrl: kDevControlWsUrl,
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
            endpoint: kDevPeerEndpointAddress,
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
    String? derpClusterId,
    List<String> preferredDerpNodeIds = const [],
    String? relayRegionId,
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
      relayUrl: kDevRelayUdpUrl,
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
      activePath: const DataPlanePathModel(
        kind: 'relay',
        details: {'peer_node_id': 'relay-peer-node-1'},
      ),
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
  Future<PlatformDoctorModel> platformDoctor() async {
    return const PlatformDoctorModel(
      platform: PlatformInfoModel(
        os: 'mock',
        family: 'mock',
        packageManager: 'mock',
      ),
      tunnelBackend: TunnelBackendDiagnosticsModel(
        name: 'in-memory',
        executionMode: 'memory',
        executionBackend: 'memory',
        isUp: true,
        plannedPeerCount: 1,
        recentCommandCount: 0,
      ),
      checks: [
        PlatformCheckModel(
          name: 'mock_backend',
          status: 'ok',
          detail: 'mock app-core backend is available',
        ),
      ],
    );
  }

  @override
  Future<PlatformInstallPlanModel> platformInstallPlan() async {
    return const PlatformInstallPlanModel(
      platform: PlatformInfoModel(os: 'mock', family: 'mock'),
      supportedDriverModes: ['in-memory'],
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
