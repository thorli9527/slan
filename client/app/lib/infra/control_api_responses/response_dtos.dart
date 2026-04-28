import 'json_readers.dart';

class ErrorResponseDto {
  const ErrorResponseDto({
    required this.code,
    required this.message,
  });

  factory ErrorResponseDto.fromJson(Map<String, dynamic> json) =>
      ErrorResponseDto(
        code: readString(json, 'code'),
        message: readString(json, 'message'),
      );

  final String code;
  final String message;
}

class AuthResponseDto {
  const AuthResponseDto({
    required this.userId,
    this.email,
    required this.accessToken,
    this.refreshToken,
    required this.expiresIn,
  });

  factory AuthResponseDto.fromJson(Map<String, dynamic> json) =>
      AuthResponseDto(
        userId: readString(json, 'userId'),
        email: readNullableString(json, 'email'),
        accessToken: readString(json, 'accessToken'),
        refreshToken: readNullableString(json, 'refreshToken'),
        expiresIn: readInt(json, 'expiresIn'),
      );

  final String userId;
  final String? email;
  final String accessToken;
  final String? refreshToken;
  final int expiresIn;
}

class CompleteAuthCallbackRequestDto {
  const CompleteAuthCallbackRequestDto({
    required this.accessToken,
    required this.userId,
    this.refreshToken,
    required this.expiresIn,
    this.deviceId,
    this.userLabel,
    this.action,
  });

  factory CompleteAuthCallbackRequestDto.fromJson(Map<String, dynamic> json) =>
      CompleteAuthCallbackRequestDto(
        accessToken: readString(json, 'accessToken'),
        userId: readString(json, 'userId'),
        refreshToken: readNullableString(json, 'refreshToken'),
        expiresIn: readInt(json, 'expiresIn'),
        deviceId: readNullableString(json, 'deviceId'),
        userLabel: readNullableString(json, 'userLabel'),
        action: readNullableString(json, 'action'),
      );

  final String accessToken;
  final String userId;
  final String? refreshToken;
  final int expiresIn;
  final String? deviceId;
  final String? userLabel;
  final String? action;
}

class AuthCallbackStatusResponseDto {
  const AuthCallbackStatusResponseDto({
    required this.callbackId,
    required this.ready,
    this.payload,
  });

  factory AuthCallbackStatusResponseDto.fromJson(Map<String, dynamic> json) =>
      AuthCallbackStatusResponseDto(
        callbackId: readString(json, 'callbackId'),
        ready: readBool(json, 'ready'),
        payload: readNullableMap(json, 'payload') == null
            ? null
            : CompleteAuthCallbackRequestDto.fromJson(readMap(json, 'payload')),
      );

  final String callbackId;
  final bool ready;
  final CompleteAuthCallbackRequestDto? payload;
}

class MqttCredentialResponseDto {
  const MqttCredentialResponseDto({
    required this.brokerUrl,
    required this.clientId,
    required this.username,
    required this.password,
    required this.topicPrefix,
    this.expiresAt,
  });

  factory MqttCredentialResponseDto.fromJson(Map<String, dynamic> json) =>
      MqttCredentialResponseDto(
        brokerUrl: readString(json, 'brokerUrl'),
        clientId: readString(json, 'clientId'),
        username: readString(json, 'username'),
        password: readString(json, 'password'),
        topicPrefix: readString(json, 'topicPrefix'),
        expiresAt: readNullableInt(json, 'expiresAt'),
      );

  final String brokerUrl;
  final String clientId;
  final String username;
  final String password;
  final String topicPrefix;
  final int? expiresAt;
}

class DeviceNetworkStateResponseDto {
  const DeviceNetworkStateResponseDto({
    required this.deviceId,
    required this.networkId,
    required this.controlReachable,
    required this.networkOnline,
    required this.tunnelUp,
    required this.lastProbeOk,
    this.virtualIp,
    required this.lastSeenAt,
    required this.updatedAt,
  });

  factory DeviceNetworkStateResponseDto.fromJson(Map<String, dynamic> json) =>
      DeviceNetworkStateResponseDto(
        deviceId: readString(json, 'deviceId'),
        networkId: readString(json, 'networkId'),
        controlReachable: readBool(json, 'controlReachable'),
        networkOnline: readBool(json, 'networkOnline'),
        tunnelUp: readBool(json, 'tunnelUp'),
        lastProbeOk: readBool(json, 'lastProbeOk'),
        virtualIp: readNullableString(json, 'virtualIp'),
        lastSeenAt: readInt(json, 'lastSeenAt'),
        updatedAt: readInt(json, 'updatedAt'),
      );

  final String deviceId;
  final String networkId;
  final bool controlReachable;
  final bool networkOnline;
  final bool tunnelUp;
  final bool lastProbeOk;
  final String? virtualIp;
  final int lastSeenAt;
  final int updatedAt;
}

class DeviceResponseDto {
  const DeviceResponseDto({
    required this.deviceId,
    required this.name,
    required this.platform,
    this.deviceVersion,
    required this.status,
    this.ownerEmail,
    this.machineId,
    this.currentVirtualIp,
    this.linkStatus,
    this.connectivityProtocol,
    this.joinedAt,
    this.membershipStatus,
    this.networkRole,
    this.createdAt,
    this.publicKey,
    this.networkIds = const [],
    this.mqtt,
    this.networkState,
  });

  factory DeviceResponseDto.fromJson(Map<String, dynamic> json) =>
      DeviceResponseDto(
        deviceId: readString(json, 'deviceId'),
        name: readString(json, 'name'),
        platform: readString(json, 'platform'),
        deviceVersion: readNullableString(json, 'deviceVersion'),
        status: readString(json, 'status'),
        ownerEmail: readNullableString(json, 'ownerEmail'),
        machineId: readNullableString(json, 'machineId'),
        currentVirtualIp: readNullableString(json, 'currentVirtualIp'),
        linkStatus: readNullableString(json, 'linkStatus'),
        connectivityProtocol: readNullableString(json, 'connectivityProtocol'),
        joinedAt: readNullableInt(json, 'joinedAt'),
        membershipStatus: readNullableString(json, 'membershipStatus'),
        networkRole: readNullableString(json, 'networkRole'),
        createdAt: readNullableInt(json, 'createdAt'),
        publicKey: readNullableString(json, 'publicKey'),
        networkIds: readStringList(json, 'networkIds'),
        mqtt: readNullableMap(json, 'mqtt') == null
            ? null
            : MqttCredentialResponseDto.fromJson(readMap(json, 'mqtt')),
        networkState: readNullableMap(json, 'networkState') == null
            ? null
            : DeviceNetworkStateResponseDto.fromJson(
                readMap(json, 'networkState'),
              ),
      );

  final String deviceId;
  final String name;
  final String platform;
  final String? deviceVersion;
  final String status;
  final String? ownerEmail;
  final String? machineId;
  final String? currentVirtualIp;
  final String? linkStatus;
  final String? connectivityProtocol;
  final int? joinedAt;
  final String? membershipStatus;
  final String? networkRole;
  final int? createdAt;
  final String? publicKey;
  final List<String> networkIds;
  final MqttCredentialResponseDto? mqtt;
  final DeviceNetworkStateResponseDto? networkState;
}

class NodeResponseDto {
  const NodeResponseDto({
    required this.nodeId,
    required this.deviceId,
    required this.nodePublicKey,
    this.networkIds = const [],
    this.capabilities = const [],
  });

  factory NodeResponseDto.fromJson(Map<String, dynamic> json) =>
      NodeResponseDto(
        nodeId: readString(json, 'nodeId'),
        deviceId: readString(json, 'deviceId'),
        nodePublicKey: readString(json, 'nodePublicKey'),
        networkIds: readStringList(json, 'networkIds'),
        capabilities: readStringList(json, 'capabilities'),
      );

  final String nodeId;
  final String deviceId;
  final String nodePublicKey;
  final List<String> networkIds;
  final List<String> capabilities;
}

class NetworkSummaryResponseDto {
  const NetworkSummaryResponseDto({
    required this.networkId,
    required this.name,
    this.description,
    this.defaultSubnetId,
    this.defaultSubnetCidr,
    this.joinKeyConfigured,
    this.subnets = const [],
  });

  factory NetworkSummaryResponseDto.fromJson(Map<String, dynamic> json) =>
      NetworkSummaryResponseDto(
        networkId: readString(json, 'networkId'),
        name: readString(json, 'name'),
        description: readNullableString(json, 'description'),
        defaultSubnetId: readNullableString(json, 'defaultSubnetId'),
        defaultSubnetCidr: readNullableString(json, 'defaultSubnetCidr'),
        joinKeyConfigured: readNullableBool(json, 'joinKeyConfigured'),
        subnets: readMapList(json, 'subnets')
            .map(SubnetResponseDto.fromJson)
            .toList(growable: false),
      );

  final String networkId;
  final String name;
  final String? description;
  final String? defaultSubnetId;
  final String? defaultSubnetCidr;
  final bool? joinKeyConfigured;
  final List<SubnetResponseDto> subnets;
}

class BootstrapResponseDto {
  const BootstrapResponseDto({
    this.controlSessionId,
    this.sessionToken,
    required this.device,
    this.networks = const [],
    required this.controlPlane,
    this.stunServers = const [],
    required this.relay,
    required this.derpMap,
    this.networkMap,
  });

  factory BootstrapResponseDto.fromJson(Map<String, dynamic> json) =>
      BootstrapResponseDto(
        controlSessionId: readNullableString(json, 'controlSessionId'),
        sessionToken: readNullableString(json, 'sessionToken'),
        device: DeviceBootstrapResponseDto.fromJson(readMap(json, 'device')),
        networks: readMapList(json, 'networks')
            .map(NetworkDetailResponseDto.fromJson)
            .toList(growable: false),
        controlPlane: ControlPlaneConfigResponseDto.fromJson(
            readMap(json, 'controlPlane')),
        stunServers: readStringList(json, 'stunServers'),
        relay: RelayConfigResponseDto.fromJson(readMap(json, 'relay')),
        derpMap: DerpMapResponseDto.fromJson(readMap(json, 'derpMap')),
        networkMap: readNullableMap(json, 'networkMap') == null
            ? null
            : NetworkMapResponseDto.fromJson(readMap(json, 'networkMap')),
      );

  final String? controlSessionId;
  final String? sessionToken;
  final DeviceBootstrapResponseDto device;
  final List<NetworkDetailResponseDto> networks;
  final ControlPlaneConfigResponseDto controlPlane;
  final List<String> stunServers;
  final RelayConfigResponseDto relay;
  final DerpMapResponseDto derpMap;
  final NetworkMapResponseDto? networkMap;
}

class DeviceBootstrapResponseDto {
  const DeviceBootstrapResponseDto({
    required this.device,
    this.attachments = const [],
  });

  factory DeviceBootstrapResponseDto.fromJson(Map<String, dynamic> json) =>
      DeviceBootstrapResponseDto(
        device: DeviceResponseDto.fromJson(readMap(json, 'device')),
        attachments: readMapList(json, 'attachments')
            .map(SubnetAttachmentResponseDto.fromJson)
            .toList(growable: false),
      );

  final DeviceResponseDto device;
  final List<SubnetAttachmentResponseDto> attachments;
}

class SubnetAttachmentResponseDto {
  const SubnetAttachmentResponseDto({
    this.attachmentId,
    required this.networkId,
    this.subnetId,
    required this.deviceId,
    this.virtualIp,
    this.remark,
    this.status,
  });

  factory SubnetAttachmentResponseDto.fromJson(Map<String, dynamic> json) =>
      SubnetAttachmentResponseDto(
        attachmentId: readNullableString(json, 'attachmentId'),
        networkId: readString(json, 'networkId'),
        subnetId: readNullableString(json, 'subnetId'),
        deviceId: readString(json, 'deviceId'),
        virtualIp: readNullableString(json, 'virtualIp'),
        remark: readNullableString(json, 'remark'),
        status: readNullableString(json, 'status'),
      );

  final String? attachmentId;
  final String networkId;
  final String? subnetId;
  final String deviceId;
  final String? virtualIp;
  final String? remark;
  final String? status;
}

class NetworkAssignmentResponseDto {
  const NetworkAssignmentResponseDto({
    required this.attachmentId,
    required this.networkId,
    required this.subnetId,
    required this.deviceId,
    required this.deviceName,
    this.devicePlatform,
    this.deviceVersion,
    this.connectionType,
    required this.userId,
    required this.userEmail,
    required this.role,
    this.remark,
    this.virtualIp,
    this.status,
  });

  factory NetworkAssignmentResponseDto.fromJson(Map<String, dynamic> json) =>
      NetworkAssignmentResponseDto(
        attachmentId: readString(json, 'attachmentId'),
        networkId: readString(json, 'networkId'),
        subnetId: readString(json, 'subnetId'),
        deviceId: readString(json, 'deviceId'),
        deviceName: readString(json, 'deviceName'),
        devicePlatform: readNullableString(json, 'devicePlatform'),
        deviceVersion: readNullableString(json, 'deviceVersion'),
        connectionType: readNullableString(json, 'connectionType'),
        userId: readString(json, 'userId'),
        userEmail: readString(json, 'userEmail'),
        role: readString(json, 'role'),
        remark: readNullableString(json, 'remark'),
        virtualIp: readNullableString(json, 'virtualIp'),
        status: readNullableString(json, 'status'),
      );

  final String attachmentId;
  final String networkId;
  final String subnetId;
  final String deviceId;
  final String deviceName;
  final String? devicePlatform;
  final String? deviceVersion;
  final String? connectionType;
  final String userId;
  final String userEmail;
  final String role;
  final String? remark;
  final String? virtualIp;
  final String? status;
}

class NetworkJoinResultResponseDto {
  const NetworkJoinResultResponseDto({
    required this.member,
    required this.attachment,
  });

  factory NetworkJoinResultResponseDto.fromJson(Map<String, dynamic> json) =>
      NetworkJoinResultResponseDto(
        member: NetworkMemberResponseDto.fromJson(readMap(json, 'member')),
        attachment:
            SubnetAttachmentResponseDto.fromJson(readMap(json, 'attachment')),
      );

  final NetworkMemberResponseDto member;
  final SubnetAttachmentResponseDto attachment;
}

class NetworkDetailResponseDto {
  const NetworkDetailResponseDto({
    required this.networkId,
    required this.name,
    this.description,
    this.defaultSubnetId,
    this.defaultSubnetCidr,
    this.joinKeyConfigured,
    this.ownedByCurrentUser,
    this.dns,
    this.joinKey,
    this.subnets = const [],
    this.members = const [],
  });

  factory NetworkDetailResponseDto.fromJson(Map<String, dynamic> json) =>
      NetworkDetailResponseDto(
        networkId: readString(json, 'networkId'),
        name: readString(json, 'name'),
        description: readNullableString(json, 'description'),
        defaultSubnetId: readNullableString(json, 'defaultSubnetId'),
        defaultSubnetCidr: readNullableString(json, 'defaultSubnetCidr'),
        joinKeyConfigured: readNullableBool(json, 'joinKeyConfigured'),
        ownedByCurrentUser: readNullableBool(json, 'ownedByCurrentUser'),
        dns: readNullableMap(json, 'dns') == null
            ? null
            : DNSConfigResponseDto.fromJson(readMap(json, 'dns')),
        joinKey: readNullableString(json, 'joinKey'),
        subnets: readMapList(json, 'subnets')
            .map(SubnetResponseDto.fromJson)
            .toList(growable: false),
        members: readMapList(json, 'members')
            .map(NetworkMemberResponseDto.fromJson)
            .toList(growable: false),
      );

  final String networkId;
  final String name;
  final String? description;
  final String? defaultSubnetId;
  final String? defaultSubnetCidr;
  final bool? joinKeyConfigured;
  final bool? ownedByCurrentUser;
  final DNSConfigResponseDto? dns;
  final String? joinKey;
  final List<SubnetResponseDto> subnets;
  final List<NetworkMemberResponseDto> members;
}

class SubnetResponseDto {
  const SubnetResponseDto({
    this.subnetId,
    required this.networkId,
    this.name,
    required this.cidr,
    this.remark,
    this.gatewayIp,
    this.allocationStartIp,
    this.allocationEndIp,
    required this.isDefault,
    this.status,
  });

  factory SubnetResponseDto.fromJson(Map<String, dynamic> json) =>
      SubnetResponseDto(
        subnetId: readNullableString(json, 'subnetId'),
        networkId: readString(json, 'networkId'),
        name: readNullableString(json, 'name'),
        cidr: readString(json, 'cidr'),
        remark: readNullableString(json, 'remark'),
        gatewayIp: readNullableString(json, 'gatewayIp'),
        allocationStartIp: readNullableString(json, 'allocationStartIp'),
        allocationEndIp: readNullableString(json, 'allocationEndIp'),
        isDefault: readBool(json, 'isDefault'),
        status: readNullableString(json, 'status'),
      );

  final String? subnetId;
  final String networkId;
  final String? name;
  final String cidr;
  final String? remark;
  final String? gatewayIp;
  final String? allocationStartIp;
  final String? allocationEndIp;
  final bool isDefault;
  final String? status;
}

class NetworkMemberResponseDto {
  const NetworkMemberResponseDto({
    this.memberId,
    this.networkId,
    this.attachmentId,
    required this.deviceId,
    required this.role,
    this.createdAt,
    this.status,
    this.remark,
    this.virtualIp,
  });

  factory NetworkMemberResponseDto.fromJson(Map<String, dynamic> json) =>
      NetworkMemberResponseDto(
        memberId: readNullableString(json, 'memberId'),
        networkId: readNullableString(json, 'networkId'),
        attachmentId: readNullableString(json, 'attachmentId'),
        deviceId: readString(json, 'deviceId'),
        role: readString(json, 'role'),
        createdAt: readNullableInt(json, 'createdAt'),
        status: readNullableString(json, 'status'),
        remark: readNullableString(json, 'remark'),
        virtualIp: readNullableString(json, 'virtualIp'),
      );

  final String? memberId;
  final String? networkId;
  final String? attachmentId;
  final String deviceId;
  final String role;
  final int? createdAt;
  final String? status;
  final String? remark;
  final String? virtualIp;
}

class ControlPlaneConfigResponseDto {
  const ControlPlaneConfigResponseDto({
    required this.wsUrl,
    this.sessionToken,
    required this.heartbeatSeconds,
  });

  factory ControlPlaneConfigResponseDto.fromJson(Map<String, dynamic> json) =>
      ControlPlaneConfigResponseDto(
        wsUrl: readString(json, 'wsUrl'),
        sessionToken: readNullableString(json, 'sessionToken'),
        heartbeatSeconds: readInt(json, 'heartbeatSeconds'),
      );

  final String wsUrl;
  final String? sessionToken;
  final int heartbeatSeconds;
}

class RelayConfigResponseDto {
  const RelayConfigResponseDto({
    required this.defaultClusterId,
    this.countries = const [],
  });

  factory RelayConfigResponseDto.fromJson(Map<String, dynamic> json) =>
      RelayConfigResponseDto(
        defaultClusterId: readString(json, 'defaultClusterId'),
        countries: readMapList(json, 'countries')
            .map(RelayCountryResponseDto.fromJson)
            .toList(growable: false),
      );

  final String defaultClusterId;
  final List<RelayCountryResponseDto> countries;
}

class DerpNodeResponseDto {
  const DerpNodeResponseDto({
    required this.nodeId,
    required this.host,
    required this.port,
    required this.transport,
    required this.priority,
    this.tags = const [],
  });

  factory DerpNodeResponseDto.fromJson(Map<String, dynamic> json) =>
      DerpNodeResponseDto(
        nodeId: readString(json, 'nodeId'),
        host: readString(json, 'host'),
        port: readInt(json, 'port'),
        transport: readString(json, 'transport'),
        priority: readInt(json, 'priority'),
        tags: readStringList(json, 'tags'),
      );

  final String nodeId;
  final String host;
  final int port;
  final String transport;
  final int priority;
  final List<String> tags;
}

class DerpClusterResponseDto {
  const DerpClusterResponseDto({
    required this.clusterId,
    this.clusterName,
    required this.regionId,
    required this.regionName,
    this.countryCode,
    this.countryName,
    this.cityCode,
    this.cityName,
    required this.recommendedFanout,
    this.nodes = const [],
  });

  factory DerpClusterResponseDto.fromJson(Map<String, dynamic> json) =>
      DerpClusterResponseDto(
        clusterId: readString(json, 'clusterId'),
        clusterName: readNullableString(json, 'clusterName'),
        regionId: readString(json, 'regionId'),
        regionName: readString(json, 'regionName'),
        countryCode: readNullableString(json, 'countryCode'),
        countryName: readNullableString(json, 'countryName'),
        cityCode: readNullableString(json, 'cityCode'),
        cityName: readNullableString(json, 'cityName'),
        recommendedFanout: readInt(json, 'recommendedFanout'),
        nodes: readMapList(json, 'nodes')
            .map(DerpNodeResponseDto.fromJson)
            .toList(growable: false),
      );

  final String clusterId;
  final String? clusterName;
  final String regionId;
  final String regionName;
  final String? countryCode;
  final String? countryName;
  final String? cityCode;
  final String? cityName;
  final int recommendedFanout;
  final List<DerpNodeResponseDto> nodes;
}

class DerpMapResponseDto {
  const DerpMapResponseDto({
    required this.probeIntervalSeconds,
    this.clusters = const [],
  });

  factory DerpMapResponseDto.fromJson(Map<String, dynamic> json) =>
      DerpMapResponseDto(
        probeIntervalSeconds: readInt(json, 'probeIntervalSeconds'),
        clusters: readMapList(json, 'clusters')
            .map(DerpClusterResponseDto.fromJson)
            .toList(growable: false),
      );

  final int probeIntervalSeconds;
  final List<DerpClusterResponseDto> clusters;
}

class RelayCountryResponseDto {
  const RelayCountryResponseDto({
    required this.countryCode,
    required this.countryName,
    this.cities = const [],
  });

  factory RelayCountryResponseDto.fromJson(Map<String, dynamic> json) =>
      RelayCountryResponseDto(
        countryCode: readString(json, 'countryCode'),
        countryName: readString(json, 'countryName'),
        cities: readMapList(json, 'cities')
            .map(RelayCityResponseDto.fromJson)
            .toList(growable: false),
      );

  final String countryCode;
  final String countryName;
  final List<RelayCityResponseDto> cities;
}

class RelayCityResponseDto {
  const RelayCityResponseDto({
    required this.cityCode,
    required this.cityName,
    this.clusters = const [],
  });

  factory RelayCityResponseDto.fromJson(Map<String, dynamic> json) =>
      RelayCityResponseDto(
        cityCode: readString(json, 'cityCode'),
        cityName: readString(json, 'cityName'),
        clusters: readMapList(json, 'clusters')
            .map(RelayClusterResponseDto.fromJson)
            .toList(growable: false),
      );

  final String cityCode;
  final String cityName;
  final List<RelayClusterResponseDto> clusters;
}

class RelayClusterResponseDto {
  const RelayClusterResponseDto({
    required this.clusterId,
    required this.clusterName,
    this.nodes = const [],
  });

  factory RelayClusterResponseDto.fromJson(Map<String, dynamic> json) =>
      RelayClusterResponseDto(
        clusterId: readString(json, 'clusterId'),
        clusterName: readString(json, 'clusterName'),
        nodes: readMapList(json, 'nodes')
            .map(RelayNodeResponseDto.fromJson)
            .toList(growable: false),
      );

  final String clusterId;
  final String clusterName;
  final List<RelayNodeResponseDto> nodes;
}

class RelayNodeResponseDto {
  const RelayNodeResponseDto({
    required this.nodeId,
    required this.transport,
    required this.address,
    required this.priority,
    this.tags = const [],
  });

  factory RelayNodeResponseDto.fromJson(Map<String, dynamic> json) =>
      RelayNodeResponseDto(
        nodeId: readString(json, 'nodeId'),
        transport: readString(json, 'transport'),
        address: readString(json, 'address'),
        priority: readInt(json, 'priority'),
        tags: readStringList(json, 'tags'),
      );

  final String nodeId;
  final String transport;
  final String address;
  final int priority;
  final List<String> tags;
}

class NetworkMapResponseDto {
  const NetworkMapResponseDto({
    required this.selfUserId,
    required this.selfDeviceId,
    required this.selfNodeId,
    required this.networkId,
    required this.revision,
    required this.heartbeatSeconds,
    this.stunServers = const [],
    this.peers = const [],
    this.routes = const [],
    this.relayRegions = const [],
    required this.dns,
    this.mtu,
  });

  factory NetworkMapResponseDto.fromJson(Map<String, dynamic> json) =>
      NetworkMapResponseDto(
        selfUserId: readString(json, 'selfUserId'),
        selfDeviceId: readString(json, 'selfDeviceId'),
        selfNodeId: readString(json, 'selfNodeId'),
        networkId: readString(json, 'networkId'),
        revision: readInt(json, 'revision'),
        heartbeatSeconds: readInt(json, 'heartbeatSeconds'),
        stunServers: readStringList(json, 'stunServers'),
        peers: readMapList(json, 'peers')
            .map(PeerResponseDto.fromJson)
            .toList(growable: false),
        routes: readMapList(json, 'routes')
            .map(RouteResponseDto.fromJson)
            .toList(growable: false),
        relayRegions: readMapList(json, 'relayRegions')
            .map(RelayRegionResponseDto.fromJson)
            .toList(growable: false),
        dns: DNSConfigResponseDto.fromJson(readMap(json, 'dns')),
        mtu: readNullableInt(json, 'mtu'),
      );

  final String selfUserId;
  final String selfDeviceId;
  final String selfNodeId;
  final String networkId;
  final int revision;
  final int heartbeatSeconds;
  final List<String> stunServers;
  final List<PeerResponseDto> peers;
  final List<RouteResponseDto> routes;
  final List<RelayRegionResponseDto> relayRegions;
  final DNSConfigResponseDto dns;
  final int? mtu;
}

class PeerResponseDto {
  const PeerResponseDto({
    required this.nodeId,
    required this.deviceId,
    required this.publicKey,
    required this.status,
    required this.relayAllowed,
    this.virtualIps = const [],
    this.endpoints = const [],
    this.allowedRoutes = const [],
  });

  factory PeerResponseDto.fromJson(Map<String, dynamic> json) =>
      PeerResponseDto(
        nodeId: readString(json, 'nodeId'),
        deviceId: readString(json, 'deviceId'),
        publicKey: readString(json, 'publicKey'),
        status: readString(json, 'status'),
        relayAllowed: readBool(json, 'relayAllowed'),
        virtualIps: readStringList(json, 'virtualIps'),
        endpoints: readMapList(json, 'endpoints')
            .map(EndpointResponseDto.fromJson)
            .toList(growable: false),
        allowedRoutes: readStringList(json, 'allowedRoutes'),
      );

  final String nodeId;
  final String deviceId;
  final String publicKey;
  final String status;
  final bool relayAllowed;
  final List<String> virtualIps;
  final List<EndpointResponseDto> endpoints;
  final List<String> allowedRoutes;
}

class EndpointResponseDto {
  const EndpointResponseDto({
    required this.type,
    required this.address,
    required this.updatedAt,
  });

  factory EndpointResponseDto.fromJson(Map<String, dynamic> json) =>
      EndpointResponseDto(
        type: readString(json, 'type'),
        address: readString(json, 'address'),
        updatedAt: readInt(json, 'updatedAt'),
      );

  final String type;
  final String address;
  final int updatedAt;
}

class RouteResponseDto {
  const RouteResponseDto({
    required this.cidr,
    required this.viaNodeId,
    this.metric,
  });

  factory RouteResponseDto.fromJson(Map<String, dynamic> json) =>
      RouteResponseDto(
        cidr: readString(json, 'cidr'),
        viaNodeId: readString(json, 'viaNodeId'),
        metric: readNullableString(json, 'metric'),
      );

  final String cidr;
  final String viaNodeId;
  final String? metric;
}

class DNSConfigResponseDto {
  const DNSConfigResponseDto({
    this.servers = const [],
    this.searchDomains = const [],
    this.wildcards = const [],
  });

  factory DNSConfigResponseDto.fromJson(Map<String, dynamic> json) =>
      DNSConfigResponseDto(
        servers: readStringList(json, 'servers'),
        searchDomains: readStringList(json, 'searchDomains'),
        wildcards: readStringList(json, 'wildcards'),
      );

  final List<String> servers;
  final List<String> searchDomains;
  final List<String> wildcards;
}

class RelayRegionResponseDto {
  const RelayRegionResponseDto({
    required this.regionId,
    required this.regionName,
    this.countryCode,
    this.countryName,
    this.cityCode,
    this.cityName,
    this.clusterId,
    this.clusterName,
    this.endpoints = const [],
  });

  factory RelayRegionResponseDto.fromJson(Map<String, dynamic> json) =>
      RelayRegionResponseDto(
        regionId: readString(json, 'regionId'),
        regionName: readString(json, 'regionName'),
        countryCode: readNullableString(json, 'countryCode'),
        countryName: readNullableString(json, 'countryName'),
        cityCode: readNullableString(json, 'cityCode'),
        cityName: readNullableString(json, 'cityName'),
        clusterId: readNullableString(json, 'clusterId'),
        clusterName: readNullableString(json, 'clusterName'),
        endpoints: readMapList(json, 'endpoints')
            .map(RelayEndpointResponseDto.fromJson)
            .toList(growable: false),
      );

  final String regionId;
  final String regionName;
  final String? countryCode;
  final String? countryName;
  final String? cityCode;
  final String? cityName;
  final String? clusterId;
  final String? clusterName;
  final List<RelayEndpointResponseDto> endpoints;
}

class RelayEndpointResponseDto {
  const RelayEndpointResponseDto({
    required this.endpointId,
    required this.transport,
    required this.address,
  });

  factory RelayEndpointResponseDto.fromJson(Map<String, dynamic> json) =>
      RelayEndpointResponseDto(
        endpointId: readString(json, 'endpointId'),
        transport: readString(json, 'transport'),
        address: readString(json, 'address'),
      );

  final String endpointId;
  final String transport;
  final String address;
}

class RelayTicketResponseDto {
  const RelayTicketResponseDto({
    required this.ticketId,
    required this.networkId,
    required this.sessionId,
    required this.srcNodeId,
    required this.dstNodeId,
    this.derpClusterId,
    this.countryCode,
    this.cityCode,
    this.allowedDerpNodeIds = const [],
    required this.relayUrl,
    required this.expiresAt,
    this.sessionKey,
    required this.signature,
  });

  factory RelayTicketResponseDto.fromJson(Map<String, dynamic> json) =>
      RelayTicketResponseDto(
        ticketId: readString(json, 'ticketId'),
        networkId: readString(json, 'networkId'),
        sessionId: readString(json, 'sessionId'),
        srcNodeId: readString(json, 'srcNodeId'),
        dstNodeId: readString(json, 'dstNodeId'),
        derpClusterId: readNullableString(json, 'derpClusterId'),
        countryCode: readNullableString(json, 'countryCode'),
        cityCode: readNullableString(json, 'cityCode'),
        allowedDerpNodeIds: readStringList(json, 'allowedDerpNodeIds'),
        relayUrl: readString(json, 'relayUrl'),
        expiresAt: readString(json, 'expiresAt'),
        sessionKey: readNullableString(json, 'sessionKey'),
        signature: readString(json, 'signature'),
      );

  final String ticketId;
  final String networkId;
  final String sessionId;
  final String srcNodeId;
  final String dstNodeId;
  final String? derpClusterId;
  final String? countryCode;
  final String? cityCode;
  final List<String> allowedDerpNodeIds;
  final String relayUrl;
  final String expiresAt;
  final String? sessionKey;
  final String signature;
}
