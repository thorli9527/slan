library slan_app.infra.app_core.api;

import '../models/bootstrap_models.dart';
import '../models/connection_models.dart';
import '../models/control_models.dart';
import '../models/diagnostic_models.dart';
import '../models/identity_models.dart';
import '../models/network_models.dart';
import '../models/relay_models.dart';

abstract class AppCoreApi {
  void restoreSession(SessionModel session);

  Future<SessionModel> register({
    required String email,
    required String password,
  });

  Future<SessionModel> login({
    required String email,
    required String password,
  });

  Future<SessionModel> refreshSession({
    required String refreshToken,
    String? deviceId,
  });

  Future<DeviceModel> registerDevice({
    required String name,
    required String platform,
    String? deviceVersion,
    required String machineId,
    required String publicKey,
  });

  Future<List<DeviceModel>> listDevices();

  Future<void> setDeviceNetworkState({
    required String deviceId,
    required String networkId,
    required bool controlReachable,
    required bool networkOnline,
    required bool tunnelUp,
    required bool lastProbeOk,
    String? virtualIp,
    int? reportedAt,
  });

  Future<NodeModel> registerNode({
    required String deviceId,
    required String nodeId,
    required String nodePublicKey,
    List<String> capabilities = const [],
  });

  Future<List<NetworkModel>> listNetworks();

  Future<NetworkModel> createNetwork({
    required String name,
    String? cidr,
    String? allocationStartIp,
    String? allocationEndIp,
    String? bindDeviceId,
  });

  Future<NetworkJoinModel> joinNetwork({
    required String networkId,
    required String deviceId,
  });

  Future<NetworkJoinModel> joinNetworkByOwnerEmail({
    required String ownerEmail,
    required String deviceId,
  });

  Future<NetworkJoinModel> joinNetworkByKey({
    required String joinKey,
    required String deviceId,
  });

  Future<NetworkAssignmentModel> updateAttachmentRemark({
    required String networkId,
    required String attachmentId,
    required String remark,
  });

  Future<NetworkJoinModel> activateNetwork({
    required String networkId,
    required String deviceId,
  });

  Future<NetworkJoinModel> switchNetwork({
    required String networkId,
    required String deviceId,
  });

  Future<void> deactivateNetwork({
    required String networkId,
    required String deviceId,
  });

  Future<BootstrapModel> bootstrap({
    required String nodeId,
    required String networkId,
  });

  Future<BootstrapModel> controlSync({
    required String nodeId,
    required String networkId,
  });

  Future<ControlStatusModel> controlStatus();

  Future<RelayTicketModel> issueRelayTicket({
    required String networkId,
    required String srcNodeId,
    required String dstNodeId,
    required String reason,
    String? derpClusterId,
    List<String> preferredDerpNodeIds = const [],
    String? relayRegionId,
  });

  Future<ConnectionStateModel> connect({
    required String networkId,
    required String peerNodeId,
  });

  Future<DataPlaneProbeModel> probe({
    required String payload,
    int? probeTimeoutMs,
  });

  Future<PlatformDoctorModel> platformDoctor();

  Future<PlatformInstallPlanModel> platformInstallPlan();

  Future<int> send({
    required String payload,
  });

  Future<void> disconnect();
}
