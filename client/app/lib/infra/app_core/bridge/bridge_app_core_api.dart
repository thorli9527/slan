import 'package:flutter/services.dart';
import 'package:slan_app_core_plugin/slan_app_core_plugin.dart';

import '../api/app_core_api.dart';
import '../models/bootstrap_models.dart';
import '../models/connection_models.dart';
import '../models/control_models.dart';
import '../models/diagnostic_models.dart';
import '../models/identity_models.dart';
import '../models/network_models.dart';
import '../models/relay_models.dart';

class BridgeAppCoreApi implements AppCoreApi {
  BridgeAppCoreApi({
    SlanAppCorePluginPlatform? pluginPlatform,
  }) : _pluginPlatform = pluginPlatform ?? SlanAppCorePluginPlatform.instance;
  final SlanAppCorePluginPlatform _pluginPlatform;

  @override
  void restoreSession(SessionModel session) {
    // Bridge mode owns session state on the Rust/native side. The desktop
    // callback path currently targets HTTP mode, so no extra local state is
    // required here.
  }

  // Plugin-platform-backed app-core flows.
  @override
  Future<SessionModel> register({
    required String email,
    required String password,
  }) async {
    final payload = await _pluginPlatform.register(
      email: email,
      password: password,
    );
    return _toSessionModel(payload);
  }

  @override
  Future<SessionModel> login({
    required String email,
    required String password,
  }) async {
    final payload = await _pluginPlatform.login(
      email: email,
      password: password,
    );
    return _toSessionModel(payload);
  }

  @override
  Future<SessionModel> refreshSession({
    required String refreshToken,
    String? deviceId,
  }) async {
    final payload = await _pluginPlatform.refreshSession(
      refreshToken: refreshToken,
      deviceId: deviceId,
    );
    return _toSessionModel(payload);
  }

  @override
  Future<DeviceModel> registerDevice({
    required String name,
    required String platform,
    required String machineId,
    required String publicKey,
  }) async {
    final payload = await _pluginPlatform.registerDevice(
      name: name,
      platform: platform,
      machineId: machineId,
      publicKey: publicKey,
    );
    return _toDeviceModel(payload);
  }

  @override
  Future<List<DeviceModel>> listDevices() async {
    final payload = await _pluginPlatform.listDevices();
    return payload.map(_toDeviceModel).toList(growable: false);
  }

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
    await _pluginPlatform.setDeviceNetworkState(
      deviceId: deviceId,
      networkId: networkId,
      controlReachable: controlReachable,
      networkOnline: networkOnline,
      tunnelUp: tunnelUp,
      lastProbeOk: lastProbeOk,
      virtualIp: virtualIp,
      reportedAt: reportedAt,
    );
  }

  @override
  Future<NodeModel> registerNode({
    required String deviceId,
    required String nodeId,
    required String nodePublicKey,
    List<String> capabilities = const [],
  }) async {
    final payload = await _pluginPlatform.registerNode(
      deviceId: deviceId,
      nodeId: nodeId,
      nodePublicKey: nodePublicKey,
      capabilities: capabilities,
    );
    return _toNodeModel(payload);
  }

  @override
  Future<List<NetworkModel>> listNetworks() async {
    final payload = await _pluginPlatform.listNetworks();
    return payload.map(_toNetworkSummaryModel).toList(growable: false);
  }

  @override
  Future<NetworkModel> createNetwork({
    required String name,
    String cidr = '10.0.0.0/16',
    String? bindDeviceId,
  }) async {
    final payload = await _pluginPlatform.createNetwork(
      name: name,
      cidr: cidr,
      bindDeviceId: bindDeviceId,
    );
    return _toNetworkSummaryModel(payload);
  }

  @override
  Future<NetworkJoinModel> joinNetwork({
    required String networkId,
    required String deviceId,
  }) async {
    final payload = await _pluginPlatform.joinNetwork(
      networkId: networkId,
      deviceId: deviceId,
    );
    return _toNetworkJoinModel(payload, fallbackNetworkId: networkId);
  }

  @override
  Future<NetworkJoinModel> joinNetworkByOwnerEmail({
    required String ownerEmail,
    required String deviceId,
  }) async {
    final payload = await _pluginPlatform.joinNetworkByOwnerEmail(
      ownerEmail: ownerEmail,
      deviceId: deviceId,
    );
    return _toNetworkJoinModel(payload);
  }

  @override
  Future<NetworkJoinModel> joinNetworkByKey({
    required String joinKey,
    required String deviceId,
  }) async {
    final payload = await _pluginPlatform.joinNetworkByKey(
      joinKey: joinKey,
      deviceId: deviceId,
    );
    return _toNetworkJoinModel(payload);
  }

  @override
  Future<NetworkAssignmentModel> updateAttachmentRemark({
    required String networkId,
    required String attachmentId,
    required String remark,
  }) async {
    final payload = await _pluginPlatform.updateAttachmentRemark(
      networkId: networkId,
      attachmentId: attachmentId,
      remark: remark,
    );
    return _toNetworkAssignmentModel(payload);
  }

  @override
  Future<NetworkJoinModel> activateNetwork({
    required String networkId,
    required String deviceId,
  }) async {
    final payload = await _pluginPlatform.activateNetwork(
      networkId: networkId,
      deviceId: deviceId,
    );
    return _toNetworkJoinModel(payload);
  }

  @override
  Future<NetworkJoinModel> switchNetwork({
    required String networkId,
    required String deviceId,
  }) async {
    final payload = await _pluginPlatform.switchNetwork(
      networkId: networkId,
      deviceId: deviceId,
    );
    return _toNetworkJoinModel(payload);
  }

  @override
  Future<void> deactivateNetwork({
    required String networkId,
    required String deviceId,
  }) async {
    await _pluginPlatform.deactivateNetwork(
      networkId: networkId,
      deviceId: deviceId,
    );
  }

  @override
  Future<BootstrapModel> bootstrap({
    required String nodeId,
    required String networkId,
  }) async {
    final payload = await _pluginPlatform.bootstrap(
      nodeId: nodeId,
      networkId: networkId,
    );
    return _toBootstrapModel(payload);
  }

  @override
  Future<BootstrapModel> controlSync({
    required String nodeId,
    required String networkId,
  }) async {
    final payload = await _pluginPlatform.controlSync(
      nodeId: nodeId,
      networkId: networkId,
    );
    return _toBootstrapModel(payload);
  }

  @override
  Future<ControlStatusModel> controlStatus() async {
    final payload = await _pluginPlatform.controlStatus();
    return _parseControlStatus(payload);
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
    final payload = await _pluginPlatform.issueRelayTicket(
      networkId: networkId,
      srcNodeId: srcNodeId,
      dstNodeId: dstNodeId,
      reason: reason,
      derpClusterId: derpClusterId,
      preferredDerpNodeIds: preferredDerpNodeIds,
      relayRegionId: relayRegionId,
    );
    return _toRelayTicketModel(payload);
  }

  @override
  Future<ConnectionStateModel> connect({
    required String networkId,
    required String peerNodeId,
  }) async {
    final payload = await _pluginPlatform.connect(
      networkId: networkId,
      peerNodeId: peerNodeId,
    );
    return _parseConnectionState(payload);
  }

  @override
  Future<DataPlaneProbeModel> probe({
    required String payload,
    int? probeTimeoutMs,
  }) async {
    try {
      final probe = await _pluginPlatform.probe(
        payload: payload,
        probeTimeoutMs: probeTimeoutMs,
      );
      return _parseProbe(probe);
    } on PlatformException catch (err) {
      throw ProbeException(_classifyProbeFailure(err));
    }
  }

  @override
  Future<PlatformDoctorModel> platformDoctor() async {
    final payload = await _pluginPlatform.platformDoctor();
    return _toPlatformDoctorModel(payload);
  }

  @override
  Future<PlatformInstallPlanModel> platformInstallPlan() async {
    final payload = await _pluginPlatform.platformInstallPlan();
    return _toPlatformInstallPlanModel(payload);
  }

  @override
  Future<int> send({
    required String payload,
  }) async {
    try {
      return await _pluginPlatform.send(payload: payload);
    } on PlatformException catch (err) {
      throw SendException(_classifySendFailure(err));
    }
  }

  @override
  Future<void> disconnect() async {
    await _pluginPlatform.disconnect();
  }
}

SessionModel _toSessionModel(AppCoreSessionPayload payload) {
  return SessionModel(
    userId: payload.userId,
    accessToken: payload.accessToken,
    refreshToken: payload.refreshToken,
    expiresIn: payload.expiresIn,
    deviceId: payload.deviceId,
    userLabel: payload.userLabel,
  );
}

DeviceModel _toDeviceModel(
  AppCoreDevicePayload payload, {
  String? virtualIp,
}) {
  return DeviceModel(
    deviceId: payload.deviceId,
    name: payload.name,
    platform: payload.platform,
    status: payload.status,
    virtualIp: virtualIp ?? payload.currentVirtualIp,
    publicKey: payload.publicKey,
    ownerEmail: payload.ownerEmail,
    linkStatus: payload.linkStatus,
    connectivityProtocol: payload.connectivityProtocol,
    joinedAt: payload.joinedAt,
    membershipStatus: payload.membershipStatus,
    networkRole: payload.networkRole,
    createdAt: payload.createdAt,
    networkIds: payload.networkIds,
  );
}

NodeModel _toNodeModel(AppCoreNodePayload payload) {
  return NodeModel(
    nodeId: payload.nodeId,
    deviceId: payload.deviceId,
    nodePublicKey: payload.nodePublicKey,
    networkIds: payload.networkIds,
    capabilities: payload.capabilities,
  );
}

NetworkModel _toNetworkSummaryModel(AppCoreNetworkPayload payload) {
  return NetworkModel(
    networkId: payload.networkId,
    name: payload.name,
    cidr: payload.cidr ?? payload.defaultSubnetCidr ?? '',
    members: payload.members
        .map(
          (member) => _toNetworkMemberModel(
            member,
            networkId: payload.networkId,
            selfDeviceId: '',
            selfAttachments: const [],
          ),
        )
        .toList(growable: false),
  );
}

NetworkJoinModel _toNetworkJoinModel(
  AppCoreNetworkJoinPayload payload, {
  String? fallbackNetworkId,
}) {
  return NetworkJoinModel(
    networkId:
        payload.networkId.isEmpty ? fallbackNetworkId ?? '' : payload.networkId,
    deviceId: payload.deviceId,
    memberId: payload.memberId,
    attachmentId: payload.attachmentId,
    virtualIp: payload.virtualIp,
  );
}

NetworkAssignmentModel _toNetworkAssignmentModel(
  AppCoreNetworkAssignmentPayload payload,
) {
  return NetworkAssignmentModel(
    attachmentId: payload.attachmentId,
    networkId: payload.networkId,
    subnetId: payload.subnetId,
    deviceId: payload.deviceId,
    deviceName: payload.deviceName,
    userId: payload.userId,
    userEmail: payload.userEmail,
    role: payload.role,
    remark: payload.remark,
    virtualIp: payload.virtualIp,
    status: payload.status,
  );
}

BootstrapModel _toBootstrapModel(AppCoreBootstrapPayload payload) {
  final activeNetworkId = payload.networkMap?.networkId;
  return BootstrapModel(
    device: _toBootstrapDeviceModel(
      payload.device,
      preferredNetworkId: activeNetworkId,
    ),
    networks: payload.networks
        .map(
          (network) => _toNetworkDetailModel(
            network,
            selfDeviceId: payload.device.device.deviceId,
            selfAttachments: payload.device.attachments,
          ),
        )
        .toList(growable: false),
    controlPlane: _toControlPlaneModel(payload.controlPlane),
    stunServers: payload.stunServers,
    relay: _toRelayConfigModel(payload.relay),
  );
}

DeviceModel _toBootstrapDeviceModel(
  AppCoreBootstrapDevicePayload payload, {
  String? preferredNetworkId,
}) {
  return _toDeviceModel(
    payload.device,
    virtualIp: _resolveVirtualIp(
      payload.attachments,
      preferredNetworkId: preferredNetworkId,
    ),
  );
}

String? _resolveVirtualIp(
  List<AppCoreSubnetAttachmentPayload> attachments, {
  String? preferredNetworkId,
}) {
  for (final attachment in attachments) {
    if (preferredNetworkId != null &&
        attachment.networkId == preferredNetworkId &&
        attachment.virtualIp != null &&
        attachment.virtualIp!.isNotEmpty) {
      return attachment.virtualIp;
    }
  }
  for (final attachment in attachments) {
    if (attachment.virtualIp != null && attachment.virtualIp!.isNotEmpty) {
      return attachment.virtualIp;
    }
  }
  return null;
}

NetworkModel _toNetworkDetailModel(
  AppCoreNetworkDetailPayload payload, {
  required String selfDeviceId,
  required List<AppCoreSubnetAttachmentPayload> selfAttachments,
}) {
  final cidr = payload.defaultSubnetCidr ??
      payload.subnets
          .where((subnet) => subnet.isDefault)
          .map((subnet) => subnet.cidr)
          .cast<String?>()
          .firstWhere(
            (subnet) => subnet != null,
            orElse: () =>
                payload.subnets.isNotEmpty ? payload.subnets.first.cidr : '',
          ) ??
      '';
  return NetworkModel(
    networkId: payload.networkId,
    name: payload.name,
    cidr: cidr,
    members: payload.members
        .map(
          (member) => _toNetworkMemberModel(
            member,
            networkId: payload.networkId,
            selfDeviceId: selfDeviceId,
            selfAttachments: selfAttachments,
          ),
        )
        .toList(growable: false),
  );
}

NetworkMemberModel _toNetworkMemberModel(
  AppCoreNetworkMemberPayload payload, {
  required String networkId,
  required String selfDeviceId,
  required List<AppCoreSubnetAttachmentPayload> selfAttachments,
}) {
  String? virtualIp;
  if (payload.deviceId == selfDeviceId) {
    for (final attachment in selfAttachments) {
      if (attachment.networkId == networkId &&
          attachment.virtualIp != null &&
          attachment.virtualIp!.isNotEmpty) {
        virtualIp = attachment.virtualIp;
        break;
      }
    }
  }
  return NetworkMemberModel(
    memberId: payload.memberId,
    networkId: payload.networkId ?? networkId,
    attachmentId: payload.attachmentId,
    deviceId: payload.deviceId,
    role: payload.role,
    createdAt: payload.createdAt,
    status: payload.status,
    virtualIp: virtualIp ?? payload.virtualIp,
    remark: payload.remark,
  );
}

ControlPlaneConfigModel _toControlPlaneModel(
    AppCoreControlPlanePayload payload) {
  return ControlPlaneConfigModel(
    wsUrl: payload.wsUrl,
    sessionToken: payload.sessionToken,
    heartbeatSeconds: payload.heartbeatSeconds,
  );
}

RelayConfigModel _toRelayConfigModel(AppCoreRelayConfigPayload payload) {
  return RelayConfigModel(
    defaultClusterId: payload.defaultClusterId,
    countries: payload.countries
        .map(
          (country) => RelayCountryModel(
            countryCode: country.countryCode,
            countryName: country.countryName,
            cities: country.cities
                .map(
                  (city) => RelayCityModel(
                    cityCode: city.cityCode,
                    cityName: city.cityName,
                    clusters: city.clusters
                        .map(
                          (cluster) => RelayClusterModel(
                            clusterId: cluster.clusterId,
                            clusterName: cluster.clusterName,
                            nodes: cluster.nodes
                                .map(
                                  (node) => RelayNodeModel(
                                    nodeId: node.nodeId,
                                    transport: node.transport,
                                    address: node.address,
                                    priority: node.priority,
                                    tags: node.tags,
                                  ),
                                )
                                .toList(growable: false),
                          ),
                        )
                        .toList(growable: false),
                  ),
                )
                .toList(growable: false),
          ),
        )
        .toList(growable: false),
  );
}

RelayTicketModel _toRelayTicketModel(AppCoreRelayTicketPayload payload) {
  return RelayTicketModel(
    ticketId: payload.ticketId,
    networkId: payload.networkId,
    sessionId: payload.sessionId,
    srcNodeId: payload.srcNodeId,
    dstNodeId: payload.dstNodeId,
    derpClusterId: payload.derpClusterId,
    countryCode: payload.countryCode,
    cityCode: payload.cityCode,
    allowedDerpNodeIds: payload.allowedDerpNodeIds,
    relayUrl: payload.relayUrl,
    expiresAt: payload.expiresAt,
    sessionKey: payload.sessionKey,
    signature: payload.signature,
  );
}

ConnectionStateModel _parseConnectionState(
  AppCoreConnectionStatusPayload payload,
) {
  final status = payload.status;
  switch (status) {
    case 'disconnected':
      return const ConnectionStateModel.disconnected();
    case 'connecting':
      return const ConnectionStateModel.connecting();
    case 'connected':
      final path = payload.path;
      if (path == 'relay' || path == 'derp') {
        return const ConnectionStateModel.connected(ConnectionPathModel.relay);
      }
      return const ConnectionStateModel.connected(ConnectionPathModel.p2p);
    case 'failed':
      return ConnectionStateModel.failed(payload.reason ?? 'unknown');
    default:
      throw FormatException('Unsupported connection status: $status');
  }
}

DataPlaneProbeModel _parseProbe(AppCoreDataPlaneProbePayload payload) {
  return DataPlaneProbeModel(
    probeId: payload.probeId,
    sampledAtMs: payload.sampledAtMs,
    activePath: DataPlanePathModel(
      kind: payload.activePath.kind,
      details: payload.activePath.details,
    ),
    bytesSent: payload.bytesSent,
    replyObserved: payload.replyObserved,
    replyBytesReceived: payload.replyBytesReceived,
    replySampledAtMs: payload.replySampledAtMs,
    replyRttMs: payload.replyRttMs,
    tunnelPeerVirtualIp: payload.tunnelPeerVirtualIp,
    observedRttMs: payload.observedRttMs,
    packetLossPpm: payload.packetLossPpm,
    pathScore: payload.pathScore,
    derpClusterId: payload.derpClusterId,
    derpNodeId: payload.derpNodeId,
  );
}

PlatformDoctorModel _toPlatformDoctorModel(
  AppCorePlatformDoctorPayload payload,
) {
  return PlatformDoctorModel(
    platform: _toPlatformInfoModel(payload.platform),
    tunnelBackend: _toTunnelBackendDiagnosticsModel(payload.tunnelBackend),
    checks: payload.checks
        .map(
          (check) => PlatformCheckModel(
            name: check.name,
            status: check.status,
            detail: check.detail,
          ),
        )
        .toList(growable: false),
  );
}

PlatformInstallPlanModel _toPlatformInstallPlanModel(
  AppCorePlatformInstallPlanPayload payload,
) {
  return PlatformInstallPlanModel(
    platform: _toPlatformInfoModel(payload.platform),
    packages: payload.packages,
    supportedDriverModes: payload.supportedDriverModes,
    warnings: payload.warnings,
  );
}

PlatformInfoModel _toPlatformInfoModel(AppCorePlatformPayload payload) {
  return PlatformInfoModel(
    os: payload.os,
    distroId: payload.distroId,
    versionId: payload.versionId,
    idLike: payload.idLike,
    family: payload.family,
    kernelRelease: payload.kernelRelease,
    packageManager: payload.packageManager,
  );
}

TunnelBackendDiagnosticsModel _toTunnelBackendDiagnosticsModel(
  AppCoreTunnelBackendDiagnosticsPayload payload,
) {
  return TunnelBackendDiagnosticsModel(
    name: payload.name,
    executionMode: payload.executionMode,
    executionBackend: payload.executionBackend,
    interfaceName: payload.interfaceName,
    isUp: payload.isUp,
    plannedPeerCount: payload.plannedPeerCount,
    recentCommandCount: payload.recentCommandCount,
  );
}

ControlStatusModel _parseControlStatus(AppCoreControlStatusPayload payload) {
  final plans = payload.connectPlans
      .map(
        (entry) => ControlConnectPlanModel(
          peerNodeId: entry.peerNodeId,
          preferDirect: entry.preferDirect,
          pathCount: entry.pathCount,
          preferredPath: entry.preferredPath == null
              ? null
              : ControlPathOptionModel(
                  pathType: entry.preferredPath!.pathType,
                  endpoint: entry.preferredPath!.endpoint,
                  priority: entry.preferredPath!.priority,
                ),
          derpClusterId: entry.derpClusterId,
          preferredDerpNodeIds: entry.preferredDerpNodeIds,
          relayTicketId: entry.relayTicketId,
        ),
      )
      .toList(growable: false);
  return ControlStatusModel(
    status: payload.status,
    wsUrl: payload.wsUrl,
    heartbeatSeconds: payload.heartbeatSeconds,
    sessionTokenPresent: payload.sessionTokenPresent,
    networkMapPresent: payload.networkMapPresent,
    networkId: payload.networkId,
    nodeId: payload.nodeId,
    deviceId: payload.deviceId,
    peerCount: payload.peerCount,
    connectPlanCount: payload.connectPlanCount,
    connectPlans: plans,
  );
}

ProbeFailure _classifyProbeFailure(PlatformException err) {
  return ProbeFailure.fromDataPlane(
    _classifyDataPlaneBridgeFailure(err, fallbackMessage: 'probe failed'),
  );
}

SendFailure _classifySendFailure(PlatformException err) {
  return SendFailure.fromDataPlane(
    _classifyDataPlaneBridgeFailure(err, fallbackMessage: 'send failed'),
  );
}

DataPlaneFailureDetails _classifyDataPlaneBridgeFailure(
  PlatformException err, {
  required String fallbackMessage,
}) {
  final code = err.code;
  final message = err.message ?? fallbackMessage;
  final normalizedCode = code.toLowerCase();
  final lower = '$code $message'.toLowerCase();
  if (normalizedCode == 'probe_timeout' ||
      normalizedCode == 'send_timeout' ||
      lower.contains('timeout') ||
      lower.contains('timed out')) {
    return DataPlaneFailureDetails(
      kind: DataPlaneFailureKind.timeout,
      code: code,
      message: message,
    );
  }
  if (normalizedCode == 'probe_unsupported_path' ||
      normalizedCode == 'send_unsupported_path' ||
      lower.contains('unsupported')) {
    return DataPlaneFailureDetails(
      kind: DataPlaneFailureKind.unsupported,
      code: code,
      message: message,
    );
  }
  if (normalizedCode == 'probe_transport_error' ||
      normalizedCode == 'send_transport_error' ||
      lower.contains('transport') ||
      lower.contains('socket') ||
      lower.contains('connection')) {
    return DataPlaneFailureDetails(
      kind: DataPlaneFailureKind.transport,
      code: code,
      message: message,
    );
  }
  if (normalizedCode == 'probe_relay_auth_error' ||
      normalizedCode == 'send_relay_auth_error') {
    return DataPlaneFailureDetails(
      kind: DataPlaneFailureKind.relayAuth,
      code: code,
      message: message,
    );
  }
  if (normalizedCode == 'probe_relay_session_error' ||
      normalizedCode == 'send_relay_session_error') {
    return DataPlaneFailureDetails(
      kind: DataPlaneFailureKind.relaySession,
      code: code,
      message: message,
    );
  }
  if (normalizedCode == 'probe_relay_protocol_error' ||
      normalizedCode == 'send_relay_protocol_error') {
    return DataPlaneFailureDetails(
      kind: DataPlaneFailureKind.relayProtocol,
      code: code,
      message: message,
    );
  }
  return DataPlaneFailureDetails(
    kind: DataPlaneFailureKind.unknown,
    code: code,
    message: message,
  );
}
