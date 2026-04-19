/// 网络成员模型。
class NetworkMemberModel {
  const NetworkMemberModel({
    required this.deviceId,
    required this.role,
    this.virtualIp,
  });

  final String deviceId;
  final String role;
  final String? virtualIp;
}

/// 网络模型。
class NetworkModel {
  const NetworkModel({
    required this.networkId,
    required this.name,
    required this.cidr,
    this.members = const [],
  });

  final String networkId;
  final String name;
  final String cidr;
  final List<NetworkMemberModel> members;
}
