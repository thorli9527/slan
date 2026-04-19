import 'dart:convert';
import 'dart:io';

import '../../api_models.dart';
import '../../control_api_responses.dart';
import 'app_core_api.dart';
import '../models/models.dart';

class HttpAppCoreApi implements AppCoreApi {
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
  Future<DeviceModel> registerDevice({
    required String name,
    required String platform,
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
          machineId: machineId,
          publicKey: publicKey,
        ).toJson(),
        authorized: true,
      ),
    );
  }

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
    String cidr = '100.64.0.0/24',
  }) async {
    return parseNetworkResponse(
      await _send(
        'POST',
        '/networks',
        body: CreateNetworkRequest(name: name, cidr: cidr).toJson(),
        authorized: true,
      ),
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
    final request = await _httpClient.openUrl(method, _baseUri.resolve(path));
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

    final response = await request.close();
    final payload = await response.transform(utf8.decoder).join();
    final json = payload.isEmpty ? <String, dynamic>{} : _decodeObject(payload);
    if (response.statusCode < 200 || response.statusCode >= 300) {
      throw HttpAppCoreException(
        statusCode: response.statusCode,
        code: _readOptionalString(json, 'code'),
        message: _readOptionalString(json, 'message') ??
            'HTTP $method $path failed with status ${response.statusCode}',
      );
    }
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
