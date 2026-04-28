import 'dart:convert';
import 'dart:io';

import '../../api_contracts/request_models.dart';
import '../../control_api_responses/response_parsers.dart';
import '../../logging/startup_log.dart';
import 'app_core_api.dart';
import '../models/bootstrap_models.dart';
import '../models/connection_models.dart';
import '../models/control_models.dart';
import '../models/diagnostic_models.dart';
import '../models/identity_models.dart';
import '../models/network_models.dart';
import '../models/relay_models.dart';

class HttpAppCoreApi implements AppCoreApi {
  static const Duration _requestTimeout = Duration(seconds: 8);

  HttpAppCoreApi({
    required String baseUrl,
    HttpClient? httpClient,
  })  : _baseUri = Uri.parse(baseUrl),
        _httpClient = httpClient ?? HttpClient();

  final Uri _baseUri;
  final HttpClient _httpClient;

  String? _accessToken;
  ConnectionStateModel _connectionState =
      const ConnectionStateModel.disconnected();

  @override
  void restoreSession(SessionModel session) {
    _accessToken = session.accessToken;
  }

  @override
  Future<SessionModel> register({
    required String email,
    required String password,
  }) async {
    final session = parseSessionResponse(
      await _send(
        'POST',
        '/auth/register',
        body: RegisterRequest(email: email, password: password).toJson(),
      ),
    );
    _accessToken = session.accessToken;
    return session;
  }

  @override
  Future<SessionModel> login({
    required String email,
    required String password,
  }) async {
    final session = parseSessionResponse(
      await _send(
        'POST',
        '/auth/login',
        body: LoginRequest(email: email, password: password).toJson(),
      ),
    );
    _accessToken = session.accessToken;
    return session;
  }

  @override
  Future<SessionModel> refreshSession({
    required String refreshToken,
    String? deviceId,
  }) async {
    final session = parseSessionResponse(
      await _send(
        'POST',
        '/auth/refresh',
        body: RefreshTokenRequest(
          refreshToken: refreshToken,
          deviceId: deviceId,
        ).toJson(),
      ),
    );
    _accessToken = session.accessToken;
    return session;
  }

  @override
  Future<DeviceModel> registerDevice({
    required String name,
    required String platform,
    String? deviceVersion,
    required String machineId,
    required String publicKey,
  }) async {
    return parseDeviceResponse(
      await _send(
        'POST',
        '/devices/register',
        body: RegisterDeviceRequest(
          name: name,
          platform: platform,
          deviceVersion: deviceVersion,
          machineId: machineId,
          publicKey: publicKey,
        ).toJson(),
        authorized: true,
      ),
    );
  }

  @override
  Future<List<DeviceModel>> listDevices() async {
    final json = await _send('GET', '/devices', authorized: true);
    return parseDeviceListResponse(_readItems(json));
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
    await _send(
      'PUT',
      '/devices/$deviceId/networks/$networkId/state',
      body: {
        'controlReachable': controlReachable,
        'networkOnline': networkOnline,
        'tunnelUp': tunnelUp,
        'lastProbeOk': lastProbeOk,
        if (virtualIp != null) 'virtualIp': virtualIp,
        if (reportedAt != null) 'reportedAt': reportedAt,
      },
      authorized: true,
    );
  }

  @override
  Future<bool> reportDeviceNetworkState() async => false;

  @override
  Future<BootstrapModel?> enableLocalNetwork({String? networkId}) async => null;

  @override
  Future<bool> disableLocalNetwork({String? networkId}) async => false;

  @override
  Future<NodeModel> registerNode({
    required String deviceId,
    required String nodeId,
    required String nodePublicKey,
    List<String> capabilities = const [],
  }) async {
    return parseNodeResponse(
      await _send(
        'POST',
        '/nodes/register',
        body: RegisterNodeRequest(
          deviceId: deviceId,
          nodeId: nodeId,
          nodePublicKey: nodePublicKey,
          capabilities: capabilities,
        ).toJson(),
        authorized: true,
      ),
    );
  }

  @override
  Future<List<NetworkModel>> listNetworks() async {
    final json = await _send('GET', '/networks', authorized: true);
    return parseNetworkListResponse(_readItems(json));
  }

  @override
  Future<NetworkModel> createNetwork({
    required String name,
    String? cidr,
    String? allocationStartIp,
    String? allocationEndIp,
    String? bindDeviceId,
  }) async {
    return parseNetworkResponse(
      await _send(
        'POST',
        '/networks',
        body: CreateNetworkRequest(
          name: name,
          cidr: cidr,
          allocationStartIp: allocationStartIp,
          allocationEndIp: allocationEndIp,
          bindDeviceId: bindDeviceId,
        ).toJson(),
        authorized: true,
      ),
    );
  }

  @override
  Future<NetworkJoinModel> joinNetwork({
    required String networkId,
    required String deviceId,
  }) async {
    final json = await _send(
      'POST',
      '/networks/$networkId/join',
      body: JoinNetworkRequest(deviceId: deviceId).toJson(),
      authorized: true,
    );
    return parseNetworkJoinResponse(json);
  }

  @override
  Future<NetworkJoinModel> joinNetworkByKey({
    required String joinKey,
    required String deviceId,
  }) async {
    final json = await _send(
      'POST',
      '/networks/join-by-key',
      body: JoinNetworkByKeyRequest(
        joinKey: joinKey,
        deviceId: deviceId,
      ).toJson(),
      authorized: true,
    );
    return parseNetworkJoinResponse(json);
  }

  @override
  Future<NetworkAssignmentModel> updateAttachmentRemark({
    required String networkId,
    required String attachmentId,
    required String remark,
  }) async {
    final json = await _send(
      'PUT',
      '/networks/$networkId/attachments/$attachmentId/remark',
      authorized: true,
      body: UpdateAttachmentRemarkRequest(remark: remark).toJson(),
    );
    return parseNetworkAssignmentResponse(json);
  }

  @override
  Future<NetworkJoinModel> activateNetwork({
    required String networkId,
    required String deviceId,
  }) async {
    final json = await _send(
      'POST',
      '/networks/$networkId/activate',
      body: JoinNetworkRequest(deviceId: deviceId).toJson(),
      authorized: true,
    );
    return parseNetworkJoinResponse(json);
  }

  @override
  Future<NetworkJoinModel> switchNetwork({
    required String networkId,
    required String deviceId,
  }) async {
    final json = await _send(
      'POST',
      '/networks/$networkId/switch',
      body: SwitchNetworkRequest(deviceId: deviceId).toJson(),
      authorized: true,
    );
    return parseNetworkJoinResponse(json);
  }

  @override
  Future<void> deactivateNetwork({
    required String networkId,
    required String deviceId,
  }) async {
    await _send(
      'POST',
      '/networks/$networkId/deactivate',
      body: DeactivateNetworkRequest(deviceId: deviceId).toJson(),
      authorized: true,
    );
  }

  @override
  Future<BootstrapModel> bootstrap({
    required String nodeId,
    required String networkId,
  }) async {
    return parseBootstrapResponse(
      await _send(
        'POST',
        '/bootstrap',
        body: BootstrapRequest(nodeId: nodeId, networkId: networkId).toJson(),
        authorized: true,
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
      status: 'none',
      sessionTokenPresent: false,
      networkMapPresent: false,
      peerCount: 0,
      connectPlanCount: 0,
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
    return parseRelayTicketResponse(
      await _send(
        'POST',
        '/relay/tickets',
        body: RelayTicketRequest(
          networkId: networkId,
          srcNodeId: srcNodeId,
          dstNodeId: dstNodeId,
          reason: reason,
          derpClusterId: derpClusterId,
          preferredDerpNodeIds: preferredDerpNodeIds,
          relayRegionId: relayRegionId,
        ).toJson(),
        authorized: true,
      ),
    );
  }

  @override
  Future<ConnectionStateModel> connect({
    required String networkId,
    required String peerNodeId,
  }) async {
    if (peerNodeId.startsWith('fail-')) {
      _connectionState =
          const ConnectionStateModel.failed('p2p handshake timeout');
      return _connectionState;
    }
    if (peerNodeId.startsWith('relay-')) {
      _connectionState =
          const ConnectionStateModel.connected(ConnectionPathModel.relay);
      return _connectionState;
    }
    _connectionState = const ConnectionStateModel.connected(
      ConnectionPathModel.p2p,
    );
    return _connectionState;
  }

  @override
  Future<DataPlaneProbeModel> probe({
    required String payload,
    int? probeTimeoutMs,
  }) async {
    throw UnsupportedError(
      'HttpAppCoreApi does not expose data-plane probe; use bridge mode',
    );
  }

  @override
  Future<PlatformDoctorModel> platformDoctor() async {
    return const PlatformDoctorModel(
      platform: PlatformInfoModel(os: 'http'),
      tunnelBackend: TunnelBackendDiagnosticsModel(
        name: 'unavailable',
        isUp: false,
        plannedPeerCount: 0,
        recentCommandCount: 0,
      ),
      checks: [
        PlatformCheckModel(
          name: 'app_core_bridge',
          status: 'warn',
          detail: 'HTTP mode does not expose local tunnel platform diagnostics',
        ),
      ],
    );
  }

  @override
  Future<PlatformInstallPlanModel> platformInstallPlan() async {
    return const PlatformInstallPlanModel(
      platform: PlatformInfoModel(os: 'http'),
      supportedDriverModes: ['bridge'],
      warnings: [
        'HTTP mode does not expose local tunnel installation planning',
      ],
    );
  }

  @override
  Future<int> send({
    required String payload,
  }) async {
    throw UnsupportedError(
      'HttpAppCoreApi does not expose data-plane send; use bridge mode',
    );
  }

  @override
  Future<void> disconnect() async {
    _connectionState = const ConnectionStateModel.disconnected();
  }

  Future<Map<String, dynamic>> _send(
    String method,
    String path, {
    Map<String, dynamic>? body,
    bool authorized = false,
  }) async {
    final target = _baseUri.resolve(path);
    await StartupLog.write('http $method $target start authorized=$authorized');
    final request =
        await _httpClient.openUrl(method, target).timeout(_requestTimeout);
    request.headers.set(HttpHeaders.acceptHeader, 'application/json');
    if (authorized) {
      final token = _accessToken;
      if (token == null || token.isEmpty) {
        throw StateError('missing session, login first');
      }
      request.headers.set(HttpHeaders.authorizationHeader, 'Bearer $token');
    }
    if (body != null) {
      request.headers.contentType = ContentType.json;
      request.write(jsonEncode(body));
    }

    final response = await request.close().timeout(_requestTimeout);
    final payload =
        await response.transform(utf8.decoder).join().timeout(_requestTimeout);
    final json = payload.isEmpty ? <String, dynamic>{} : _decodeObject(payload);
    if (response.statusCode < 200 || response.statusCode >= 300) {
      await StartupLog.write(
        'http $method $target failed status=${response.statusCode} code=${_readOptionalString(json, 'code') ?? '-'} message=${_readOptionalString(json, 'message') ?? '-'}',
      );
      throw HttpAppCoreException(
        statusCode: response.statusCode,
        code: _readOptionalString(json, 'code'),
        message: _readOptionalString(json, 'message') ??
            'HTTP $method $path failed with status ${response.statusCode}',
      );
    }
    await StartupLog.write(
        'http $method $target ok status=${response.statusCode}');
    return json;
  }
}

class HttpAppCoreException implements Exception {
  const HttpAppCoreException({
    required this.statusCode,
    this.code,
    required this.message,
  });

  final int statusCode;
  final String? code;
  final String message;

  @override
  String toString() => code == null
      ? 'HttpAppCoreException($statusCode): $message'
      : 'HttpAppCoreException($statusCode, $code): $message';
}

Map<String, dynamic> _decodeObject(String payload) {
  final decoded = jsonDecode(payload);
  if (decoded is Map<String, dynamic>) {
    return decoded;
  }
  if (decoded is Map) {
    return decoded.map(
      (key, value) => MapEntry(key.toString(), value),
    );
  }
  throw const FormatException('Expected JSON object response');
}

List<dynamic> _readItems(Map<String, dynamic> json) {
  final items = json['items'];
  if (items is List) {
    return items;
  }
  throw const FormatException('Expected "items" list in response');
}

String? _readOptionalString(Map<String, dynamic> json, String key) {
  final value = json[key];
  return value is String ? value : null;
}
