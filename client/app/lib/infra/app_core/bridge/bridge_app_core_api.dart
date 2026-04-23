import 'package:flutter/services.dart';
import 'package:slan_app_core_plugin/slan_app_core_plugin.dart';

import '../../control_api_responses/response_parsers.dart';
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
  }) : _pluginPlatform =
            pluginPlatform ?? SlanAppCorePluginPlatform.instance;
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
    return parseSessionResponse(payload.toJson());
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
    return parseSessionResponse(payload.toJson());
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
    return parseDeviceResponse(payload.toJson());
  }

  @override
  Future<List<DeviceModel>> listDevices() async {
    final payload = await _pluginPlatform.listDevices();
    return parseDeviceListResponse(
      payload.map((item) => item.toJson()).toList(growable: false),
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
    return parseNodeResponse(payload.toJson());
  }

  @override
  Future<List<NetworkModel>> listNetworks() async {
    final payload = await _pluginPlatform.listNetworks();
    return parseNetworkListResponse(
      payload.map((item) => item.toJson()).toList(growable: false),
    );
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
    return parseNetworkResponse(payload.toJson());
  }

  @override
  Future<void> joinNetwork({
    required String networkId,
    required String deviceId,
  }) async {
    await _pluginPlatform.joinNetwork(
      networkId: networkId,
      deviceId: deviceId,
    );
  }

  @override
  Future<void> activateNetwork({
    required String networkId,
    required String deviceId,
  }) async {
    await _pluginPlatform.activateNetwork(
      networkId: networkId,
      deviceId: deviceId,
    );
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
    return parseBootstrapResponse(payload.toJson());
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
    return parseBootstrapResponse(payload.toJson());
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
  }) async {
    final payload = await _pluginPlatform.issueRelayTicket(
      networkId: networkId,
      srcNodeId: srcNodeId,
      dstNodeId: dstNodeId,
      reason: reason,
    );
    return parseRelayTicketResponse(payload.toJson());
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
    activePath: payload.activePath,
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
