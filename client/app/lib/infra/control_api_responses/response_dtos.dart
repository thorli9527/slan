import '../app_core/models/models.dart';
import 'json_readers.dart';

class AuthResponseDto {
  const AuthResponseDto({
    required this.userId,
    required this.accessToken,
    this.refreshToken,
    required this.expiresIn,
  });

  factory AuthResponseDto.fromJson(Map<String, dynamic> json) =>
      AuthResponseDto(
        userId: readString(json, 'userId'),
        accessToken: readString(json, 'accessToken'),
        refreshToken: readNullableString(json, 'refreshToken'),
        expiresIn: readInt(json, 'expiresIn'),
      );

  final String userId;
  final String accessToken;
  final String? refreshToken;
  final int expiresIn;

  SessionModel toModel() => SessionModel(
        userId: userId,
        accessToken: accessToken,
        refreshToken: refreshToken,
        expiresIn: expiresIn,
      );
}

class DeviceResponseDto {
  const DeviceResponseDto({
    required this.deviceId,
    required this.name,
    required this.platform,
    required this.status,
    this.publicKey,
    this.networkIds = const [],
  });

  factory DeviceResponseDto.fromJson(Map<String, dynamic> json) =>
      DeviceResponseDto(
        deviceId: readString(json, 'deviceId'),
        name: readString(json, 'name'),
        platform: readString(json, 'platform'),
        status: readString(json, 'status'),
        publicKey: readNullableString(json, 'publicKey'),
        networkIds: readStringList(json, 'networkIds'),
      );

  final String deviceId;
  final String name;
  final String platform;
  final String status;
  final String? publicKey;
  final List<String> networkIds;

  DeviceModel toModel({String? virtualIp}) => DeviceModel(
        deviceId: deviceId,
        name: name,
        platform: platform,
        status: status,
        virtualIp: virtualIp,
        publicKey: publicKey,
      );
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

  NodeModel toModel() => NodeModel(
        nodeId: nodeId,
        deviceId: deviceId,
        nodePublicKey: nodePublicKey,
        networkIds: networkIds,
        capabilities: capabilities,
      );
}

class NetworkSummaryResponseDto {
  const NetworkSummaryResponseDto({
    required this.networkId,
    required this.name,
    this.defaultSubnetCidr,
  });

  factory NetworkSummaryResponseDto.fromJson(Map<String, dynamic> json) =>
      NetworkSummaryResponseDto(
        networkId: readString(json, 'networkId'),
        name: readString(json, 'name'),
        defaultSubnetCidr: readNullableString(json, 'defaultSubnetCidr'),
      );

  final String networkId;
  final String name;
  final String? defaultSubnetCidr;

  NetworkModel toModel() => NetworkModel(
        networkId: networkId,
        name: name,
        cidr: defaultSubnetCidr ?? '',
      );
}

class BootstrapResponseDto {
  const BootstrapResponseDto({
    required this.device,
    this.networks = const [],
    required this.controlPlane,
    this.stunServers = const [],
    required this.relay,
    this.networkMap,
  });

  factory BootstrapResponseDto.fromJson(Map<String, dynamic> json) =>
      BootstrapResponseDto(
        device: DeviceBootstrapResponseDto.fromJson(readMap(json, 'device')),
        networks: readMapList(json, 'networks')
            .map(NetworkDetailResponseDto.fromJson)
            .toList(growable: false),
        controlPlane: ControlPlaneConfigResponseDto.fromJson(
            readMap(json, 'controlPlane')),
        stunServers: readStringList(json, 'stunServers'),
        relay: RelayConfigResponseDto.fromJson(readMap(json, 'relay')),
        networkMap: readNullableMap(json, 'networkMap') == null
            ? null
            : NetworkMapResponseDto.fromJson(readMap(json, 'networkMap')),
      );

  final DeviceBootstrapResponseDto device;
  final List<NetworkDetailResponseDto> networks;
  final ControlPlaneConfigResponseDto controlPlane;
  final List<String> stunServers;
  final RelayConfigResponseDto relay;
  final NetworkMapResponseDto? networkMap;

  BootstrapModel toModel() {
    final activeNetworkId = networkMap?.networkId;
    return BootstrapModel(
      device: device.toModel(preferredNetworkId: activeNetworkId),
      networks: networks
          .map(
            (network) => network.toModel(
              selfDeviceId: device.device.deviceId,
              selfAttachments: device.attachments,
            ),
          )
          .toList(growable: false),
      controlPlane: controlPlane.toModel(),
      stunServers: stunServers,
      relay: relay.toModel(),
    );
  }
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

  String? resolveVirtualIp({String? preferredNetworkId}) {
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

  DeviceModel toModel({String? preferredNetworkId}) => device.toModel(
        virtualIp: resolveVirtualIp(preferredNetworkId: preferredNetworkId),
      );
}

class SubnetAttachmentResponseDto {
  const SubnetAttachmentResponseDto({
    required this.networkId,
    required this.deviceId,
    this.virtualIp,
  });

  factory SubnetAttachmentResponseDto.fromJson(Map<String, dynamic> json) =>
      SubnetAttachmentResponseDto(
        networkId: readString(json, 'networkId'),
        deviceId: readString(json, 'deviceId'),
        virtualIp: readNullableString(json, 'virtualIp'),
      );

  final String networkId;
  final String deviceId;
  final String? virtualIp;
}

class NetworkDetailResponseDto {
  const NetworkDetailResponseDto({
    required this.networkId,
    required this.name,
    this.defaultSubnetCidr,
    this.subnets = const [],
    this.members = const [],
  });

  factory NetworkDetailResponseDto.fromJson(Map<String, dynamic> json) =>
      NetworkDetailResponseDto(
        networkId: readString(json, 'networkId'),
        name: readString(json, 'name'),
        defaultSubnetCidr: readNullableString(json, 'defaultSubnetCidr'),
        subnets: readMapList(json, 'subnets')
            .map(SubnetResponseDto.fromJson)
            .toList(growable: false),
        members: readMapList(json, 'members')
            .map(NetworkMemberResponseDto.fromJson)
            .toList(growable: false),
      );

  final String networkId;
  final String name;
  final String? defaultSubnetCidr;
  final List<SubnetResponseDto> subnets;
  final List<NetworkMemberResponseDto> members;

  NetworkModel toModel({
    required String selfDeviceId,
    required List<SubnetAttachmentResponseDto> selfAttachments,
  }) {
    final cidr = defaultSubnetCidr ??
        subnets
            .firstWhere(
              (subnet) => subnet.isDefault,
              orElse: () => subnets.isNotEmpty
                  ? subnets.first
                  : const SubnetResponseDto(
                      networkId: '',
                      cidr: '',
                      isDefault: false,
                    ),
            )
            .cidr;
    return NetworkModel(
      networkId: networkId,
      name: name,
      cidr: cidr,
      members: members
          .map(
            (member) => member.toModel(
              networkId: networkId,
              selfDeviceId: selfDeviceId,
              selfAttachments: selfAttachments,
            ),
          )
          .toList(growable: false),
    );
  }
}

class SubnetResponseDto {
  const SubnetResponseDto({
    required this.networkId,
    required this.cidr,
    required this.isDefault,
  });

  factory SubnetResponseDto.fromJson(Map<String, dynamic> json) =>
      SubnetResponseDto(
        networkId: readString(json, 'networkId'),
        cidr: readString(json, 'cidr'),
        isDefault: readBool(json, 'isDefault'),
      );

  final String networkId;
  final String cidr;
  final bool isDefault;
}

class NetworkMemberResponseDto {
  const NetworkMemberResponseDto({
    required this.deviceId,
    required this.role,
  });

  factory NetworkMemberResponseDto.fromJson(Map<String, dynamic> json) =>
      NetworkMemberResponseDto(
        deviceId: readString(json, 'deviceId'),
        role: readString(json, 'role'),
      );

  final String deviceId;
  final String role;

  NetworkMemberModel toModel({
    required String networkId,
    required String selfDeviceId,
    required List<SubnetAttachmentResponseDto> selfAttachments,
  }) {
    String? virtualIp;
    if (deviceId == selfDeviceId) {
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
      deviceId: deviceId,
      role: role,
      virtualIp: virtualIp,
    );
  }
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

  ControlPlaneConfigModel toModel() => ControlPlaneConfigModel(
        wsUrl: wsUrl,
        sessionToken: sessionToken,
        heartbeatSeconds: heartbeatSeconds,
      );
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

  RelayConfigModel toModel() => RelayConfigModel(
        defaultClusterId: defaultClusterId,
        countries: countries
            .map((country) => country.toModel())
            .toList(growable: false),
      );
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

  RelayCountryModel toModel() => RelayCountryModel(
        countryCode: countryCode,
        countryName: countryName,
        cities: cities.map((city) => city.toModel()).toList(growable: false),
      );
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

  RelayCityModel toModel() => RelayCityModel(
        cityCode: cityCode,
        cityName: cityName,
        clusters: clusters
            .map((cluster) => cluster.toModel())
            .toList(growable: false),
      );
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

  RelayClusterModel toModel() => RelayClusterModel(
        clusterId: clusterId,
        clusterName: clusterName,
        nodes: nodes.map((node) => node.toModel()).toList(growable: false),
      );
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

  RelayNodeModel toModel() => RelayNodeModel(
        nodeId: nodeId,
        transport: transport,
        address: address,
        priority: priority,
        tags: tags,
      );
}

class NetworkMapResponseDto {
  const NetworkMapResponseDto({required this.networkId});

  factory NetworkMapResponseDto.fromJson(Map<String, dynamic> json) =>
      NetworkMapResponseDto(networkId: readString(json, 'networkId'));

  final String networkId;
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

  RelayTicketModel toModel() => RelayTicketModel(
        ticketId: ticketId,
        networkId: networkId,
        sessionId: sessionId,
        srcNodeId: srcNodeId,
        dstNodeId: dstNodeId,
        derpClusterId: derpClusterId,
        countryCode: countryCode,
        cityCode: cityCode,
        allowedDerpNodeIds: allowedDerpNodeIds,
        relayUrl: relayUrl,
        expiresAt: expiresAt,
        sessionKey: sessionKey,
        signature: signature,
      );
}
