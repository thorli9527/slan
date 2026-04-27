/// 网络成员模型。
class DNSConfigModel {
  const DNSConfigModel({
    this.servers = const [],
    this.searchDomains = const [],
    this.wildcards = const [],
  });

  final List<String> servers;
  final List<String> searchDomains;
  final List<String> wildcards;

  bool get enabled =>
      servers.isNotEmpty || searchDomains.isNotEmpty || wildcards.isNotEmpty;
}

class NetworkMemberModel {
  const NetworkMemberModel({
    this.memberId,
    this.networkId,
    this.attachmentId,
    required this.deviceId,
    required this.role,
    this.createdAt,
    this.status,
    this.virtualIp,
    this.remark,
  });

  final String? memberId;
  final String? networkId;
  final String? attachmentId;
  final String deviceId;
  final String role;
  final int? createdAt;
  final String? status;
  final String? virtualIp;
  final String? remark;
}

/// 网络模型。
class NetworkModel {
  const NetworkModel({
    required this.networkId,
    required this.name,
    required this.cidr,
    this.description,
    this.defaultSubnetId,
    this.joinKeyConfigured,
    this.dns = const DNSConfigModel(),
    this.subnets = const [],
    this.members = const [],
  });

  final String networkId;
  final String name;
  final String cidr;
  final String? description;
  final String? defaultSubnetId;
  final bool? joinKeyConfigured;
  final DNSConfigModel dns;
  final List<SubnetModel> subnets;
  final List<NetworkMemberModel> members;
}

class SubnetModel {
  const SubnetModel({
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

class NetworkJoinModel {
  const NetworkJoinModel({
    required this.networkId,
    required this.deviceId,
    this.memberId,
    this.attachmentId,
    this.virtualIp,
  });

  final String networkId;
  final String deviceId;
  final String? memberId;
  final String? attachmentId;
  final String? virtualIp;
}

class NetworkAssignmentModel {
  const NetworkAssignmentModel({
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
