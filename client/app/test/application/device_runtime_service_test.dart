import 'package:flutter_test/flutter_test.dart';
import 'package:slan_app/application/device_runtime_service.dart';
import 'package:slan_app/infra/app_core/api/app_core_api.dart';
import 'package:slan_app/infra/app_core/models/bootstrap_models.dart';
import 'package:slan_app/infra/app_core/models/connection_models.dart';
import 'package:slan_app/infra/app_core/models/control_models.dart';
import 'package:slan_app/infra/app_core/models/diagnostic_models.dart';
import 'package:slan_app/infra/app_core/models/identity_models.dart';
import 'package:slan_app/infra/app_core/models/network_models.dart';
import 'package:slan_app/infra/app_core/models/relay_models.dart';

void main() {
  group('DeviceRuntimeService.connectWithFallback', () {
    test('returns direct success hint when planned direct path connects',
        () async {
      final api = _FakeAppCoreApi(
        connectResponses: [
          const ConnectionStateModel.connected(ConnectionPathModel.p2p),
        ],
      );
      final service = DeviceRuntimeService(apiProvider: () => api);

      final result = await service.connectWithFallback(
        networkId: 'net-1',
        peerNodeId: 'peer-1',
        reason: 'timeout',
        currentNode: const NodeModel(
          nodeId: 'node-1',
          deviceId: 'dev-1',
          nodePublicKey: 'node-pub-1',
        ),
        controlStatus: const ControlStatusModel(
          status: 'connected',
          sessionTokenPresent: true,
          networkMapPresent: true,
          peerCount: 1,
          connectPlanCount: 1,
          connectPlans: [
            ControlConnectPlanModel(
              peerNodeId: 'peer-1',
              preferDirect: true,
              pathCount: 1,
              preferredPath: ControlPathOptionModel(
                pathType: 'direct_udp',
                endpoint: '198.51.100.10:51820',
                priority: 10,
              ),
            ),
          ],
        ),
        lastProbe: null,
      );

      expect(result.relayTicket, isNull);
      expect(result.connectionState.path, ConnectionPathModel.p2p);
      expect(result.preflightHint, contains('direct_udp'));
      expect(result.resultHint, contains('Recommendation matched'));
      expect(api.connectCalls, [
        const _ConnectCall(networkId: 'net-1', peerNodeId: 'peer-1'),
      ]);
    });

    test('falls back to relay ticket when first connect fails', () async {
      final api = _FakeAppCoreApi(
        connectResponses: [
          const ConnectionStateModel.failed('p2p handshake timeout'),
          const ConnectionStateModel.connected(ConnectionPathModel.relay),
        ],
        relayTicket: const RelayTicketModel(
          ticketId: 'ticket-1',
          networkId: 'net-1',
          sessionId: 'session-1',
          srcNodeId: 'node-1',
          dstNodeId: 'peer-1',
          derpClusterId: 'cn-local-a',
          allowedDerpNodeIds: ['relay-cn-local-udp'],
          relayUrl: 'udp://127.0.0.1:19000',
          expiresAt: '2026-04-23T10:00:00Z',
          signature: 'signed',
        ),
      );
      final service = DeviceRuntimeService(apiProvider: () => api);

      final result = await service.connectWithFallback(
        networkId: 'net-1',
        peerNodeId: 'peer-1',
        reason: 'p2p_failed',
        currentNode: const NodeModel(
          nodeId: 'node-1',
          deviceId: 'dev-1',
          nodePublicKey: 'node-pub-1',
        ),
        controlStatus: const ControlStatusModel(
          status: 'connected',
          sessionTokenPresent: true,
          networkMapPresent: true,
          peerCount: 1,
          connectPlanCount: 1,
          connectPlans: [
            ControlConnectPlanModel(
              peerNodeId: 'peer-1',
              preferDirect: false,
              pathCount: 1,
              derpClusterId: 'cn-local-a',
              preferredDerpNodeIds: ['relay-cn-local-udp'],
            ),
          ],
        ),
        lastProbe: null,
      );

      expect(result.relayTicket?.ticketId, 'ticket-1');
      expect(result.connectionState.path, ConnectionPathModel.relay);
      expect(result.resultHint, contains('Recommendation matched'));
      expect(api.connectCalls, [
        const _ConnectCall(networkId: 'net-1', peerNodeId: 'peer-1'),
        const _ConnectCall(networkId: 'net-1', peerNodeId: 'relay-peer-1'),
      ]);
      expect(api.relayTicketRequests, [
        const _RelayTicketRequest(
          networkId: 'net-1',
          srcNodeId: 'node-1',
          dstNodeId: 'peer-1',
          reason: 'p2p_failed',
        ),
      ]);
    });
  });
}

class _FakeAppCoreApi implements AppCoreApi {
  _FakeAppCoreApi({
    required List<ConnectionStateModel> connectResponses,
    RelayTicketModel? relayTicket,
  })  : _connectResponses = List.of(connectResponses),
        _relayTicket = relayTicket;

  final List<ConnectionStateModel> _connectResponses;
  final RelayTicketModel? _relayTicket;
  final List<_ConnectCall> connectCalls = [];
  final List<_RelayTicketRequest> relayTicketRequests = [];

  @override
  Future<ConnectionStateModel> connect({
    required String networkId,
    required String peerNodeId,
  }) async {
    connectCalls.add(
      _ConnectCall(networkId: networkId, peerNodeId: peerNodeId),
    );
    if (_connectResponses.isEmpty) {
      throw StateError('missing fake connect response');
    }
    return _connectResponses.removeAt(0);
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
    relayTicketRequests.add(
      _RelayTicketRequest(
        networkId: networkId,
        srcNodeId: srcNodeId,
        dstNodeId: dstNodeId,
        reason: reason,
      ),
    );
    if (_relayTicket == null) {
      throw StateError('missing fake relay ticket');
    }
    return _relayTicket;
  }

  @override
  void restoreSession(SessionModel session) {}

  @override
  Future<NetworkJoinModel> activateNetwork({
    required String networkId,
    required String deviceId,
  }) async =>
      NetworkJoinModel(networkId: networkId, deviceId: deviceId);

  @override
  Future<NetworkJoinModel> switchNetwork({
    required String networkId,
    required String deviceId,
  }) async =>
      NetworkJoinModel(networkId: networkId, deviceId: deviceId);

  @override
  Future<BootstrapModel> bootstrap({
    required String nodeId,
    required String networkId,
  }) {
    throw UnimplementedError();
  }

  @override
  Future<BootstrapModel> controlSync({
    required String nodeId,
    required String networkId,
  }) {
    throw UnimplementedError();
  }

  @override
  Future<ControlStatusModel> controlStatus() {
    throw UnimplementedError();
  }

  @override
  Future<NetworkModel> createNetwork({
    required String name,
    String cidr = '10.0.0.0/16',
    String? bindDeviceId,
  }) {
    throw UnimplementedError();
  }

  @override
  Future<void> deactivateNetwork({
    required String networkId,
    required String deviceId,
  }) async {}

  @override
  Future<void> disconnect() async {}

  @override
  Future<NetworkJoinModel> joinNetwork({
    required String networkId,
    required String deviceId,
  }) async =>
      NetworkJoinModel(networkId: networkId, deviceId: deviceId);

  @override
  Future<NetworkJoinModel> joinNetworkByOwnerEmail({
    required String ownerEmail,
    required String deviceId,
  }) async =>
      NetworkJoinModel(networkId: 'net-1', deviceId: deviceId);

  @override
  Future<NetworkJoinModel> joinNetworkByKey({
    required String joinKey,
    required String deviceId,
  }) async =>
      NetworkJoinModel(networkId: 'net-1', deviceId: deviceId);

  @override
  Future<NetworkAssignmentModel> updateAttachmentRemark({
    required String networkId,
    required String attachmentId,
    required String remark,
  }) async =>
      NetworkAssignmentModel(
        attachmentId: attachmentId,
        networkId: networkId,
        subnetId: 'subnet-1',
        deviceId: 'dev-1',
        deviceName: 'dev-1',
        userId: 'user-1',
        userEmail: 'user@example.com',
        role: 'member',
        remark: remark,
      );

  @override
  Future<List<DeviceModel>> listDevices() {
    throw UnimplementedError();
  }

  @override
  Future<List<NetworkModel>> listNetworks() {
    throw UnimplementedError();
  }

  @override
  Future<SessionModel> login({
    required String email,
    required String password,
  }) {
    throw UnimplementedError();
  }

  @override
  Future<SessionModel> refreshSession({
    required String refreshToken,
    String? deviceId,
  }) {
    throw UnimplementedError();
  }

  @override
  Future<DataPlaneProbeModel> probe({
    required String payload,
    int? probeTimeoutMs,
  }) {
    throw UnimplementedError();
  }

  @override
  Future<PlatformDoctorModel> platformDoctor() {
    throw UnimplementedError();
  }

  @override
  Future<PlatformInstallPlanModel> platformInstallPlan() {
    throw UnimplementedError();
  }

  @override
  Future<SessionModel> register({
    required String email,
    required String password,
  }) {
    throw UnimplementedError();
  }

  @override
  Future<DeviceModel> registerDevice({
    required String name,
    required String platform,
    required String machineId,
    required String publicKey,
  }) {
    throw UnimplementedError();
  }

  @override
  Future<NodeModel> registerNode({
    required String deviceId,
    required String nodeId,
    required String nodePublicKey,
    List<String> capabilities = const [],
  }) {
    throw UnimplementedError();
  }

  @override
  Future<int> send({required String payload}) {
    throw UnimplementedError();
  }
}

class _ConnectCall {
  const _ConnectCall({
    required this.networkId,
    required this.peerNodeId,
  });

  final String networkId;
  final String peerNodeId;

  @override
  bool operator ==(Object other) {
    return other is _ConnectCall &&
        other.networkId == networkId &&
        other.peerNodeId == peerNodeId;
  }

  @override
  int get hashCode => Object.hash(networkId, peerNodeId);
}

class _RelayTicketRequest {
  const _RelayTicketRequest({
    required this.networkId,
    required this.srcNodeId,
    required this.dstNodeId,
    required this.reason,
  });

  final String networkId;
  final String srcNodeId;
  final String dstNodeId;
  final String reason;

  @override
  bool operator ==(Object other) {
    return other is _RelayTicketRequest &&
        other.networkId == networkId &&
        other.srcNodeId == srcNodeId &&
        other.dstNodeId == dstNodeId &&
        other.reason == reason;
  }

  @override
  int get hashCode => Object.hash(networkId, srcNodeId, dstNodeId, reason);
}
