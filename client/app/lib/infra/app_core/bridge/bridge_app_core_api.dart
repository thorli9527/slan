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
    return _parseControlStatus(payload.toJson());
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
    return _parseConnectionState(payload.toJson());
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
      return _parseProbe(probe.toJson());
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

ConnectionStateModel _parseConnectionState(Map<String, dynamic> json) {
  final status = json['status'];
  if (status is! String) {
    throw const FormatException('Expected connection status string');
  }
  switch (status) {
    case 'disconnected':
      return const ConnectionStateModel.disconnected();
    case 'connecting':
      return const ConnectionStateModel.connecting();
    case 'connected':
      final path = json['path'];
      if (path == 'relay' || path == 'derp') {
        return const ConnectionStateModel.connected(ConnectionPathModel.relay);
      }
      return const ConnectionStateModel.connected(ConnectionPathModel.p2p);
    case 'failed':
      final reason = json['reason'];
      return ConnectionStateModel.failed(reason is String ? reason : 'unknown');
    default:
      throw FormatException('Unsupported connection status: $status');
  }
}

DataPlaneProbeModel _parseProbe(Map<String, dynamic> json) {
  return DataPlaneProbeModel(
    probeId: _readString(json, 'probeId'),
    sampledAtMs: _readInt(json, 'sampledAtMs'),
    activePath: _readMap(json, 'activePath'),
    bytesSent: _readInt(json, 'bytesSent'),
    replyObserved: _readBool(json, 'replyObserved'),
    replyBytesReceived: _readNullableInt(json, 'replyBytesReceived'),
    replySampledAtMs: _readNullableInt(json, 'replySampledAtMs'),
    replyRttMs: _readNullableInt(json, 'replyRttMs'),
    tunnelPeerVirtualIp: _readNullableString(json, 'tunnelPeerVirtualIp'),
    observedRttMs: _readNullableInt(json, 'observedRttMs'),
    packetLossPpm: _readNullableInt(json, 'packetLossPpm'),
    pathScore: _readNullableInt(json, 'pathScore'),
    derpClusterId: _readNullableString(json, 'derpClusterId'),
    derpNodeId: _readNullableString(json, 'derpNodeId'),
  );
}

ControlStatusModel _parseControlStatus(Map<String, dynamic> json) {
  final plans = _readList(json, 'connectPlans')
      .whereType<Map>()
      .map((entry) => entry.map(
            (key, value) => MapEntry(key.toString(), value),
          ))
      .map(
        (entry) => ControlConnectPlanModel(
          peerNodeId: _readString(entry, 'peerNodeId'),
          preferDirect: _readBool(entry, 'preferDirect'),
          pathCount: _readInt(entry, 'pathCount'),
          preferredPath: _readNullableMap(entry, 'preferredPath') == null
              ? null
              : ControlPathOptionModel(
                  pathType:
                      _readString(_readMap(entry, 'preferredPath'), 'pathType'),
                  endpoint:
                      _readString(_readMap(entry, 'preferredPath'), 'endpoint'),
                  priority:
                      _readInt(_readMap(entry, 'preferredPath'), 'priority'),
                ),
          derpClusterId: _readNullableString(entry, 'derpClusterId'),
          preferredDerpNodeIds: _readStringList(entry, 'preferredDerpNodeIds'),
          relayTicketId: _readNullableString(entry, 'relayTicketId'),
        ),
      )
      .toList(growable: false);
  return ControlStatusModel(
    status: _readString(json, 'status'),
    wsUrl: _readNullableString(json, 'wsUrl'),
    heartbeatSeconds: _readNullableInt(json, 'heartbeatSeconds'),
    sessionTokenPresent: _readBool(json, 'sessionTokenPresent'),
    networkMapPresent: _readBool(json, 'networkMapPresent'),
    networkId: _readNullableString(json, 'networkId'),
    nodeId: _readNullableString(json, 'nodeId'),
    deviceId: _readNullableString(json, 'deviceId'),
    peerCount: _readInt(json, 'peerCount'),
    connectPlanCount: _readInt(json, 'connectPlanCount'),
    connectPlans: plans,
  );
}

List<dynamic> _readList(Map<String, dynamic> json, String key) {
  final value = json[key];
  if (value is List) {
    return value;
  }
  throw FormatException('Expected list for "$key"');
}

Map<String, dynamic> _readMap(Map<String, dynamic> json, String key) {
  final value = json[key];
  if (value is Map<String, dynamic>) {
    return value;
  }
  if (value is Map) {
    return value
        .map((mapKey, mapValue) => MapEntry(mapKey.toString(), mapValue));
  }
  throw FormatException('Expected object for "$key"');
}

String _readString(Map<String, dynamic> json, String key) {
  final value = json[key];
  if (value is String) {
    return value;
  }
  throw FormatException('Expected string for "$key"');
}

String? _readNullableString(Map<String, dynamic> json, String key) {
  final value = json[key];
  return value is String ? value : null;
}

Map<String, dynamic>? _readNullableMap(Map<String, dynamic> json, String key) {
  final value = json[key];
  if (value == null) {
    return null;
  }
  if (value is Map<String, dynamic>) {
    return value;
  }
  if (value is Map) {
    return value
        .map((mapKey, mapValue) => MapEntry(mapKey.toString(), mapValue));
  }
  return null;
}

List<String> _readStringList(Map<String, dynamic> json, String key) {
  final value = json[key];
  if (value is List) {
    return value.whereType<String>().toList(growable: false);
  }
  return const [];
}

int _readInt(Map<String, dynamic> json, String key) {
  final value = json[key];
  if (value is int) {
    return value;
  }
  throw FormatException('Expected int for "$key"');
}

int? _readNullableInt(Map<String, dynamic> json, String key) {
  final value = json[key];
  return value is int ? value : null;
}

bool _readBool(Map<String, dynamic> json, String key) {
  final value = json[key];
  if (value is bool) {
    return value;
  }
  throw FormatException('Expected bool for "$key"');
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
