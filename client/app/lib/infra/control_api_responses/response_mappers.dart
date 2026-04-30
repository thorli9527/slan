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
        userLabel: email,
      );
}

extension DeviceResponseDtoMapper on DeviceResponseDto {
  String? resolvedRuntimeVirtualIp(String? overrideVirtualIp) {
    if (overrideVirtualIp != null) {
      return overrideVirtualIp;
    }
    final state = networkState;
    if (state != null && (!state.networkOnline || !state.tunnelUp)) {
      return null;
    }
    return currentVirtualIp;
  }

  DeviceModel toModel({String? virtualIp}) => DeviceModel(
        deviceId: deviceId,
        name: name,
        platform: platform,
        deviceVersion: deviceVersion,
        status: status,
        virtualIp: resolvedRuntimeVirtualIp(virtualIp),
        publicKey: publicKey,
        ownerEmail: ownerEmail,
        machineId: machineId,
        linkStatus: linkStatus,
        connectivityProtocol: connectivityProtocol,
        joinedAt: joinedAt,
        membershipStatus: membershipStatus,
        networkRole: networkRole,
        createdAt: createdAt,
        networkIds: networkIds,
        mqtt: mqtt?.toModel(),
        networkState: networkState?.toModel(),
      );
}

extension MqttCredentialResponseDtoMapper on MqttCredentialResponseDto {
  MqttCredentialModel toModel() => MqttCredentialModel(
        brokerUrl: brokerUrl,
        clientId: clientId,
        username: username,
        password: password,
        topicPrefix: topicPrefix,
        expiresAt: expiresAt,
      );
}

extension DeviceNetworkStateResponseDtoMapper on DeviceNetworkStateResponseDto {
  DeviceNetworkStateModel toModel() => DeviceNetworkStateModel(
        deviceId: deviceId,
        networkId: networkId,
        controlReachable: controlReachable,
        networkOnline: networkOnline,
        tunnelUp: tunnelUp,
        lastProbeOk: lastProbeOk,
        virtualIp: virtualIp,
        lastSeenAt: lastSeenAt,
        updatedAt: updatedAt,
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
        description: description,
        defaultSubnetId: defaultSubnetId,
        joinKeyConfigured: joinKeyConfigured,
        dns: const DNSConfigModel(),
        subnets:
            subnets.map((subnet) => subnet.toModel()).toList(growable: false),
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
      controlPlane: controlPlane.toModel(sessionToken: sessionToken),
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
      description: description,
      defaultSubnetId: defaultSubnetId,
      joinKeyConfigured: joinKeyConfigured,
      dns: dns?.toModel() ?? const DNSConfigModel(),
      subnets:
          subnets.map((subnet) => subnet.toModel()).toList(growable: false),
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

extension DNSConfigResponseDtoMapper on DNSConfigResponseDto {
  DNSConfigModel toModel() => DNSConfigModel(
        servers: servers,
        searchDomains: searchDomains,
        wildcards: wildcards,
      );
}

extension NetworkMemberResponseDtoMapper on NetworkMemberResponseDto {
  NetworkMemberModel toModel({
    required String networkId,
    required String selfDeviceId,
    required List<SubnetAttachmentResponseDto> selfAttachments,
  }) {
    String? attachmentId;
    String? remark = this.remark;
    String? virtualIp = this.virtualIp;
    String? effectiveStatus = status;
    if (deviceId == selfDeviceId) {
      for (final attachment in selfAttachments) {
        if (attachment.networkId == networkId) {
          attachmentId = attachment.attachmentId;
          remark ??= attachment.remark;
          final attachmentStatus = attachment.status?.trim().toLowerCase();
          if (attachmentStatus == 'disabled' ||
              attachmentStatus == 'suspended') {
            effectiveStatus = attachment.status;
          }
          if (attachment.virtualIp != null &&
              attachment.virtualIp!.isNotEmpty) {
            virtualIp = attachment.virtualIp;
          }
          break;
        }
      }
    }
    return NetworkMemberModel(
      memberId: memberId,
      networkId: this.networkId ?? networkId,
      attachmentId: attachmentId ?? this.attachmentId,
      deviceId: deviceId,
      role: role,
      createdAt: createdAt,
      status: effectiveStatus,
      virtualIp: virtualIp,
      remark: remark,
    );
  }
}

extension SubnetResponseDtoMapper on SubnetResponseDto {
  SubnetModel toModel() => SubnetModel(
        subnetId: subnetId,
        networkId: networkId,
        name: name,
        cidr: cidr,
        remark: remark,
        gatewayIp: gatewayIp,
        allocationStartIp: allocationStartIp,
        allocationEndIp: allocationEndIp,
        isDefault: isDefault,
        status: status,
      );
}

extension NetworkAssignmentResponseDtoMapper on NetworkAssignmentResponseDto {
  NetworkAssignmentModel toModel() => NetworkAssignmentModel(
        attachmentId: attachmentId,
        networkId: networkId,
        subnetId: subnetId,
        deviceId: deviceId,
        deviceName: deviceName,
        devicePlatform: devicePlatform,
        deviceVersion: deviceVersion,
        connectionType: connectionType,
        userId: userId,
        userEmail: userEmail,
        role: role,
        remark: remark,
        virtualIp: virtualIp,
        status: status,
      );
}

extension NetworkJoinResultResponseDtoMapper on NetworkJoinResultResponseDto {
  NetworkJoinModel toModel() => NetworkJoinModel(
        networkId: attachment.networkId,
        deviceId: attachment.deviceId,
        memberId: member.memberId,
        attachmentId: attachment.attachmentId,
        virtualIp: attachment.virtualIp,
      );
}

extension ControlPlaneConfigResponseDtoMapper on ControlPlaneConfigResponseDto {
  ControlPlaneConfigModel toModel({String? sessionToken}) =>
      ControlPlaneConfigModel(
        wsUrl: wsUrl,
        sessionToken: sessionToken ?? this.sessionToken,
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
