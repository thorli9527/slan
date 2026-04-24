import '../app_core/models/bootstrap_models.dart';
import '../app_core/models/identity_models.dart';
import '../app_core/models/network_models.dart';
import '../app_core/models/relay_models.dart';
import 'response_dtos.dart';

extension AuthResponseDtoMapper on AuthResponseDto {
  SessionModel toModel() => SessionModel(
        userId: userId,
        accessToken: accessToken,
        refreshToken: refreshToken,
        expiresIn: expiresIn,
      );
}

extension DeviceResponseDtoMapper on DeviceResponseDto {
  DeviceModel toModel({String? virtualIp}) => DeviceModel(
        deviceId: deviceId,
        name: name,
        platform: platform,
        status: status,
        virtualIp: virtualIp ?? currentVirtualIp,
        publicKey: publicKey,
        ownerEmail: ownerEmail,
        linkStatus: linkStatus,
        connectivityProtocol: connectivityProtocol,
        joinedAt: joinedAt,
        membershipStatus: membershipStatus,
        networkRole: networkRole,
        createdAt: createdAt,
        networkIds: networkIds,
      );
}

extension NodeResponseDtoMapper on NodeResponseDto {
  NodeModel toModel() => NodeModel(
        nodeId: nodeId,
        deviceId: deviceId,
        nodePublicKey: nodePublicKey,
        networkIds: networkIds,
        capabilities: capabilities,
      );
}

extension NetworkSummaryResponseDtoMapper on NetworkSummaryResponseDto {
  NetworkModel toModel() => NetworkModel(
        networkId: networkId,
        name: name,
        cidr: defaultSubnetCidr ?? '',
      );
}

extension BootstrapResponseDtoMapper on BootstrapResponseDto {
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

extension DeviceBootstrapResponseDtoMapper on DeviceBootstrapResponseDto {
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

extension NetworkDetailResponseDtoMapper on NetworkDetailResponseDto {
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

extension NetworkMemberResponseDtoMapper on NetworkMemberResponseDto {
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

extension ControlPlaneConfigResponseDtoMapper on ControlPlaneConfigResponseDto {
  ControlPlaneConfigModel toModel() => ControlPlaneConfigModel(
        wsUrl: wsUrl,
        sessionToken: sessionToken,
        heartbeatSeconds: heartbeatSeconds,
      );
}

extension RelayConfigResponseDtoMapper on RelayConfigResponseDto {
  RelayConfigModel toModel() => RelayConfigModel(
        defaultClusterId: defaultClusterId,
        countries: countries
            .map((country) => country.toModel())
            .toList(growable: false),
      );
}

extension RelayCountryResponseDtoMapper on RelayCountryResponseDto {
  RelayCountryModel toModel() => RelayCountryModel(
        countryCode: countryCode,
        countryName: countryName,
        cities: cities.map((city) => city.toModel()).toList(growable: false),
      );
}

extension RelayCityResponseDtoMapper on RelayCityResponseDto {
  RelayCityModel toModel() => RelayCityModel(
        cityCode: cityCode,
        cityName: cityName,
        clusters: clusters
            .map((cluster) => cluster.toModel())
            .toList(growable: false),
      );
}

extension RelayClusterResponseDtoMapper on RelayClusterResponseDto {
  RelayClusterModel toModel() => RelayClusterModel(
        clusterId: clusterId,
        clusterName: clusterName,
        nodes: nodes.map((node) => node.toModel()).toList(growable: false),
      );
}

extension RelayNodeResponseDtoMapper on RelayNodeResponseDto {
  RelayNodeModel toModel() => RelayNodeModel(
        nodeId: nodeId,
        transport: transport,
        address: address,
        priority: priority,
        tags: tags,
      );
}

extension RelayTicketResponseDtoMapper on RelayTicketResponseDto {
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
