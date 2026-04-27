import 'package:plugin_platform_interface/plugin_platform_interface.dart';

import 'app_core_method_models.dart';
import 'method_channel_slan_app_core_plugin.dart';
import 'tunnel_backend_models.dart';

abstract class SlanAppCorePluginPlatform extends PlatformInterface {
  SlanAppCorePluginPlatform() : super(token: _token);

  static final Object _token = Object();

  static SlanAppCorePluginPlatform _instance = MethodChannelSlanAppCorePlugin();

  static SlanAppCorePluginPlatform get instance => _instance;

  static set instance(SlanAppCorePluginPlatform instance) {
    PlatformInterface.verifyToken(instance, _token);
    _instance = instance;
  }

  Future<Object?> invoke(
    String method, [
    Map<String, Object?> args = const {},
  ]) {
    throw UnimplementedError('invoke() has not been implemented.');
  }

  Future<AppCoreSessionPayload> register({
    required String email,
    required String password,
  }) async {
    final payload = await invoke('register', {
      'email': email,
      'password': password,
    });
    return AppCoreSessionPayload.fromJson(
      _expectMap(payload, method: 'register'),
    );
  }

  Future<AppCoreSessionPayload> login({
    required String email,
    required String password,
  }) async {
    final payload = await invoke('login', {
      'email': email,
      'password': password,
    });
    return AppCoreSessionPayload.fromJson(
      _expectMap(payload, method: 'login'),
    );
  }

  Future<AppCoreSessionPayload> refreshSession({
    required String refreshToken,
    String? deviceId,
  }) async {
    final payload = await invoke('refreshSession', {
      'refreshToken': refreshToken,
      if (deviceId != null && deviceId.isNotEmpty) 'deviceId': deviceId,
    });
    return AppCoreSessionPayload.fromJson(
      _expectMap(payload, method: 'refreshSession'),
    );
  }

  Future<AppCoreDevicePayload> registerDevice({
    required String name,
    required String platform,
    required String machineId,
    required String publicKey,
  }) async {
    final payload = await invoke('registerDevice', {
      'name': name,
      'platform': platform,
      'machineId': machineId,
      'publicKey': publicKey,
    });
    return AppCoreDevicePayload.fromJson(
      _expectMap(payload, method: 'registerDevice'),
    );
  }

  Future<List<AppCoreDevicePayload>> listDevices() async {
    final payload = await invoke('listDevices');
    return _expectList(
      _expectMap(payload, method: 'listDevices'),
      method: 'listDevices',
    ).map(AppCoreDevicePayload.fromJson).toList(growable: false);
  }

  Future<AppCoreNodePayload> registerNode({
    required String deviceId,
    required String nodeId,
    required String nodePublicKey,
    List<String> capabilities = const [],
  }) async {
    final payload = await invoke('registerNode', {
      'deviceId': deviceId,
      'nodeId': nodeId,
      'nodePublicKey': nodePublicKey,
      'capabilities': capabilities,
    });
    return AppCoreNodePayload.fromJson(
      _expectMap(payload, method: 'registerNode'),
    );
  }

  Future<List<AppCoreNetworkPayload>> listNetworks() async {
    final payload = await invoke('listNetworks');
    return _expectList(
      _expectMap(payload, method: 'listNetworks'),
      method: 'listNetworks',
    ).map(AppCoreNetworkPayload.fromJson).toList(growable: false);
  }

  Future<AppCoreNetworkPayload> createNetwork({
    required String name,
    String? cidr,
    int? expectedDevices,
    String? gatewayIp,
    String? allocationStartIp,
    String? allocationEndIp,
    String? bindDeviceId,
  }) async {
    final payload = await invoke('createNetwork', {
      'name': name,
      if (cidr != null && cidr.isNotEmpty) 'cidr': cidr,
      if (expectedDevices != null && expectedDevices > 0)
        'expectedDevices': expectedDevices,
      if (gatewayIp != null && gatewayIp.isNotEmpty) 'gatewayIp': gatewayIp,
      if (allocationStartIp != null && allocationStartIp.isNotEmpty)
        'allocationStartIp': allocationStartIp,
      if (allocationEndIp != null && allocationEndIp.isNotEmpty)
        'allocationEndIp': allocationEndIp,
      if (bindDeviceId != null && bindDeviceId.isNotEmpty)
        'bindDeviceId': bindDeviceId,
    });
    return AppCoreNetworkPayload.fromJson(
      _expectMap(payload, method: 'createNetwork'),
    );
  }

  Future<AppCoreNetworkJoinPayload> joinNetwork({
    required String networkId,
    required String deviceId,
  }) async {
    final payload = await invoke('joinNetwork', {
      'networkId': networkId,
      'deviceId': deviceId,
    });
    return AppCoreNetworkJoinPayload.fromJson(
      _expectMap(payload, method: 'joinNetwork'),
    );
  }

  Future<AppCoreNetworkJoinPayload> joinNetworkByOwnerEmail({
    required String ownerEmail,
    required String deviceId,
  }) async {
    final payload = await invoke('joinNetworkByOwnerEmail', {
      'ownerEmail': ownerEmail,
      'deviceId': deviceId,
    });
    return AppCoreNetworkJoinPayload.fromJson(
      _expectMap(payload, method: 'joinNetworkByOwnerEmail'),
    );
  }

  Future<AppCoreNetworkJoinPayload> joinNetworkByKey({
    required String joinKey,
    required String deviceId,
  }) async {
    final payload = await invoke('joinNetworkByKey', {
      'joinKey': joinKey,
      'deviceId': deviceId,
    });
    return AppCoreNetworkJoinPayload.fromJson(
      _expectMap(payload, method: 'joinNetworkByKey'),
    );
  }

  Future<AppCoreNetworkAssignmentPayload> updateAttachmentRemark({
    required String networkId,
    required String attachmentId,
    String? remark,
  }) async {
    final payload = await invoke('updateAttachmentRemark', {
      'networkId': networkId,
      'attachmentId': attachmentId,
      if (remark != null && remark.isNotEmpty) 'remark': remark,
    });
    return AppCoreNetworkAssignmentPayload.fromJson(
      _expectMap(payload, method: 'updateAttachmentRemark'),
    );
  }

  Future<AppCoreNetworkJoinPayload> activateNetwork({
    required String networkId,
    required String deviceId,
  }) async {
    final payload = await invoke('activateNetwork', {
      'networkId': networkId,
      'deviceId': deviceId,
    });
    return AppCoreNetworkJoinPayload.fromJson(
      _expectMap(payload, method: 'activateNetwork'),
    );
  }

  Future<AppCoreNetworkJoinPayload> switchNetwork({
    required String networkId,
    required String deviceId,
  }) async {
    final payload = await invoke('switchNetwork', {
      'networkId': networkId,
      'deviceId': deviceId,
    });
    return AppCoreNetworkJoinPayload.fromJson(
      _expectMap(payload, method: 'switchNetwork'),
    );
  }

  Future<void> deactivateNetwork({
    required String networkId,
    required String deviceId,
  }) async {
    await invoke('deactivateNetwork', {
      'networkId': networkId,
      'deviceId': deviceId,
    });
  }

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
    await invoke('setDeviceNetworkState', {
      'deviceId': deviceId,
      'networkId': networkId,
      'controlReachable': controlReachable,
      'networkOnline': networkOnline,
      'tunnelUp': tunnelUp,
      'lastProbeOk': lastProbeOk,
      if (virtualIp != null && virtualIp.isNotEmpty) 'virtualIp': virtualIp,
      if (reportedAt != null) 'reportedAt': reportedAt,
    });
  }

  Future<AppCoreBootstrapPayload> bootstrap({
    required String nodeId,
    required String networkId,
  }) async {
    final payload = await invoke('bootstrap', {
      'nodeId': nodeId,
      'networkId': networkId,
    });
    return AppCoreBootstrapPayload.fromJson(
      _expectMap(payload, method: 'bootstrap'),
    );
  }

  Future<AppCoreBootstrapPayload> controlSync({
    required String nodeId,
    required String networkId,
  }) async {
    final payload = await invoke('controlSync', {
      'nodeId': nodeId,
      'networkId': networkId,
    });
    return AppCoreBootstrapPayload.fromJson(
      _expectMap(payload, method: 'controlSync'),
    );
  }

  Future<AppCoreControlStatusPayload> controlStatus() async {
    final payload = await invoke('controlStatus');
    return AppCoreControlStatusPayload.fromJson(
      _expectMap(payload, method: 'controlStatus'),
    );
  }

  Future<AppCoreRelayTicketPayload> issueRelayTicket({
    required String networkId,
    required String srcNodeId,
    required String dstNodeId,
    required String reason,
    String? derpClusterId,
    List<String> preferredDerpNodeIds = const [],
    String? relayRegionId,
  }) async {
    final payload = await invoke('issueRelayTicket', {
      'networkId': networkId,
      'srcNodeId': srcNodeId,
      'dstNodeId': dstNodeId,
      if (derpClusterId != null && derpClusterId.isNotEmpty)
        'derpClusterId': derpClusterId,
      if (preferredDerpNodeIds.isNotEmpty)
        'preferredDerpNodeIds': preferredDerpNodeIds,
      'reason': reason,
      if (relayRegionId != null && relayRegionId.isNotEmpty)
        'relayRegionId': relayRegionId,
    });
    return AppCoreRelayTicketPayload.fromJson(
      _expectMap(payload, method: 'issueRelayTicket'),
    );
  }

  Future<AppCoreConnectionStatusPayload> connect({
    required String networkId,
    required String peerNodeId,
  }) async {
    final payload = await invoke('connect', {
      'networkId': networkId,
      'peerNodeId': peerNodeId,
    });
    return AppCoreConnectionStatusPayload.fromJson(
      _expectMap(payload, method: 'connect'),
    );
  }

  Future<void> disconnect() async {
    await invoke('disconnect');
  }

  Future<AppCoreDataPlaneProbePayload> probe({
    required String payload,
    int? probeTimeoutMs,
  }) async {
    final response = await invoke('probe', {
      'payload': payload,
      if (probeTimeoutMs != null) 'probeTimeoutMs': probeTimeoutMs,
    });
    return AppCoreDataPlaneProbePayload.fromJson(
      _expectMap(response, method: 'probe'),
    );
  }

  Future<int> send({
    required String payload,
  }) async {
    final response = await invoke('send', {
      'payload': payload,
    });
    return _expectInt(
      _expectMap(response, method: 'send'),
      method: 'send',
      key: 'bytesSent',
    );
  }

  Future<WireGuardTunnelActionResult> applyTunnelConfiguration(
    WireGuardTunnelConfiguration configuration,
  ) async {
    final payload =
        await invoke('applyTunnelConfiguration', configuration.toJson());
    return WireGuardTunnelActionResult.fromJson(
      _expectEncodableMap(payload, method: 'applyTunnelConfiguration'),
    );
  }

  Future<WireGuardTunnelActionResult> removeTunnelPeer(
      String peerVirtualIp) async {
    final payload = await invoke('removeTunnelPeer', {
      'peerVirtualIp': peerVirtualIp,
    });
    return WireGuardTunnelActionResult.fromJson(
      _expectEncodableMap(payload, method: 'removeTunnelPeer'),
    );
  }

  Future<WireGuardTunnelActionResult> bringTunnelUp() async {
    final payload = await invoke('bringTunnelUp');
    return WireGuardTunnelActionResult.fromJson(
      _expectEncodableMap(payload, method: 'bringTunnelUp'),
    );
  }

  Future<WireGuardTunnelActionResult> bringTunnelDown() async {
    final payload = await invoke('bringTunnelDown');
    return WireGuardTunnelActionResult.fromJson(
      _expectEncodableMap(payload, method: 'bringTunnelDown'),
    );
  }

  Future<WireGuardTunnelRuntimeView?> tunnelRuntimeView(
    String peerVirtualIp,
  ) async {
    final payload = await invoke('tunnelRuntimeView', {
      'peerVirtualIp': peerVirtualIp,
    });
    if (payload == null) {
      return null;
    }
    return WireGuardTunnelRuntimeView.fromJson(
      _expectEncodableMap(payload, method: 'tunnelRuntimeView'),
    );
  }

  Future<AppCorePlatformDoctorPayload> platformDoctor() async {
    final payload = await invoke('platformDoctor');
    return AppCorePlatformDoctorPayload.fromJson(
      _expectMap(payload, method: 'platformDoctor'),
    );
  }

  Future<AppCorePlatformInstallPlanPayload> platformInstallPlan() async {
    final payload = await invoke('platformInstallPlan');
    return AppCorePlatformInstallPlanPayload.fromJson(
      _expectMap(payload, method: 'platformInstallPlan'),
    );
  }
}

Map<String, dynamic> _expectMap(
  Object? payload, {
  required String method,
}) {
  if (payload is Map<String, dynamic>) {
    return payload;
  }
  if (payload is Map) {
    return payload.map((key, value) => MapEntry(key.toString(), value));
  }
  throw ArgumentError.value(
    payload,
    'payload',
    'Expected $method payload to be a map',
  );
}

Map<Object?, Object?> _expectEncodableMap(
  Object? payload, {
  required String method,
}) {
  if (payload is Map<Object?, Object?>) {
    return payload;
  }
  if (payload is Map) {
    return payload.map((key, value) => MapEntry(key, value));
  }
  throw ArgumentError.value(
    payload,
    'payload',
    'Expected $method payload to be a map',
  );
}

List<Map<String, dynamic>> _expectList(
  Map<String, dynamic> payload, {
  required String method,
}) {
  final items = payload['items'];
  if (items is List) {
    return items.whereType<Map>().map((entry) {
      return entry.map((key, value) => MapEntry(key.toString(), value));
    }).toList(growable: false);
  }
  throw ArgumentError.value(
    payload,
    'payload',
    'Expected $method payload to contain an items list',
  );
}

int _expectInt(
  Map<String, dynamic> payload, {
  required String method,
  required String key,
}) {
  final value = payload[key];
  if (value is int) {
    return value;
  }
  throw ArgumentError.value(
    payload,
    'payload',
    'Expected $method payload to contain an int at $key',
  );
}
