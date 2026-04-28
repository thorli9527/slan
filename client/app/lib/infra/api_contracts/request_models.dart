/// Flutter-side control-plane request contracts.
library slan_app.infra.api_contracts.request_models;

class RegisterRequest {
  const RegisterRequest({required this.email, required this.password});

  final String email;
  final String password;

  Map<String, dynamic> toJson() => {
        'email': email,
        'password': password,
      };
}

class LoginRequest {
  const LoginRequest({
    required this.email,
    required this.password,
    this.deviceId,
  });

  final String email;
  final String password;
  final String? deviceId;

  Map<String, dynamic> toJson() => {
        'email': email,
        'password': password,
        if (deviceId != null && deviceId!.isNotEmpty) 'deviceId': deviceId,
      };
}

class RefreshTokenRequest {
  const RefreshTokenRequest({
    required this.refreshToken,
    this.deviceId,
  });

  final String refreshToken;
  final String? deviceId;

  Map<String, dynamic> toJson() => {
        'refreshToken': refreshToken,
        if (deviceId != null && deviceId!.isNotEmpty) 'deviceId': deviceId,
      };
}

class CreateNetworkRequest {
  const CreateNetworkRequest({
    required this.name,
    this.description,
    this.cidr,
    this.allocationStartIp,
    this.allocationEndIp,
    this.bindDeviceId,
  });

  final String name;
  final String? description;
  final String? cidr;
  final String? allocationStartIp;
  final String? allocationEndIp;
  final String? bindDeviceId;

  Map<String, dynamic> toJson() => {
        'name': name,
        if (description != null && description!.isNotEmpty)
          'description': description,
        if (cidr != null && cidr!.isNotEmpty) 'cidr': cidr,
        if (allocationStartIp != null && allocationStartIp!.isNotEmpty)
          'allocationStartIp': allocationStartIp,
        if (allocationEndIp != null && allocationEndIp!.isNotEmpty)
          'allocationEndIp': allocationEndIp,
        if (bindDeviceId != null && bindDeviceId!.isNotEmpty)
          'bindDeviceId': bindDeviceId,
      };
}

class JoinNetworkRequest {
  const JoinNetworkRequest({required this.deviceId});

  final String deviceId;

  Map<String, dynamic> toJson() => {
        'deviceId': deviceId,
      };
}

class SwitchNetworkRequest {
  const SwitchNetworkRequest({required this.deviceId});

  final String deviceId;

  Map<String, dynamic> toJson() => {
        'deviceId': deviceId,
      };
}

class DeactivateNetworkRequest {
  const DeactivateNetworkRequest({required this.deviceId});

  final String deviceId;

  Map<String, dynamic> toJson() => {
        'deviceId': deviceId,
      };
}

class UpdateNetworkMemberStatusRequest {
  const UpdateNetworkMemberStatusRequest({required this.status});

  final String status;

  Map<String, dynamic> toJson() => {
        'status': status,
      };
}

class JoinNetworkByKeyRequest {
  const JoinNetworkByKeyRequest({
    required this.joinKey,
    required this.deviceId,
  });

  final String joinKey;
  final String deviceId;

  Map<String, dynamic> toJson() => {
        'joinKey': joinKey,
        'deviceId': deviceId,
      };
}

class UpdateNetworkRequest {
  const UpdateNetworkRequest({
    this.name,
    this.description,
    required this.cidr,
  });

  final String? name;
  final String? description;
  final String cidr;

  Map<String, dynamic> toJson() => {
        if (name != null && name!.isNotEmpty) 'name': name,
        if (description != null && description!.isNotEmpty)
          'description': description,
        'cidr': cidr,
      };
}

class UpdateNetworkDNSRequest {
  const UpdateNetworkDNSRequest({
    this.servers = const [],
    this.searchDomains = const [],
    this.wildcards = const [],
  });

  final List<String> servers;
  final List<String> searchDomains;
  final List<String> wildcards;

  Map<String, dynamic> toJson() => {
        'servers': servers,
        'searchDomains': searchDomains,
        'wildcards': wildcards,
      };
}

class UpdateAttachmentIPRequest {
  const UpdateAttachmentIPRequest({required this.virtualIp});

  final String virtualIp;

  Map<String, dynamic> toJson() => {
        'virtualIp': virtualIp,
      };
}

class UpdateAttachmentRemarkRequest {
  const UpdateAttachmentRemarkRequest({this.remark});

  final String? remark;

  Map<String, dynamic> toJson() => {
        if (remark != null) 'remark': remark,
      };
}

class RegisterDeviceRequest {
  const RegisterDeviceRequest({
    required this.name,
    required this.platform,
    this.deviceVersion,
    required this.machineId,
    required this.publicKey,
  });

  final String name;
  final String platform;
  final String? deviceVersion;
  final String machineId;
  final String publicKey;

  Map<String, dynamic> toJson() => {
        'name': name,
        'platform': platform,
        if (deviceVersion != null && deviceVersion!.isNotEmpty)
          'deviceVersion': deviceVersion,
        'machineId': machineId,
        'publicKey': publicKey,
      };
}

class RegisterNodeRequest {
  const RegisterNodeRequest({
    required this.deviceId,
    required this.nodeId,
    required this.nodePublicKey,
    this.capabilities = const [],
  });

  final String deviceId;
  final String nodeId;
  final String nodePublicKey;
  final List<String> capabilities;

  Map<String, dynamic> toJson() => {
        'deviceId': deviceId,
        'nodeId': nodeId,
        'nodePublicKey': nodePublicKey,
        'capabilities': capabilities,
      };
}

class BootstrapRequest {
  const BootstrapRequest({
    required this.nodeId,
    required this.networkId,
  });

  final String nodeId;
  final String networkId;

  Map<String, dynamic> toJson() => {
        'nodeId': nodeId,
        'networkId': networkId,
      };
}

class RelayTicketRequest {
  const RelayTicketRequest({
    required this.networkId,
    required this.srcNodeId,
    required this.dstNodeId,
    required this.reason,
    this.derpClusterId,
    this.preferredDerpNodeIds = const [],
    this.relayRegionId,
  });

  final String networkId;
  final String srcNodeId;
  final String dstNodeId;
  final String? derpClusterId;
  final List<String> preferredDerpNodeIds;
  final String reason;
  final String? relayRegionId;

  Map<String, dynamic> toJson() => {
        'networkId': networkId,
        'srcNodeId': srcNodeId,
        'dstNodeId': dstNodeId,
        if (derpClusterId != null && derpClusterId!.isNotEmpty)
          'derpClusterId': derpClusterId,
        if (preferredDerpNodeIds.isNotEmpty)
          'preferredDerpNodeIds': preferredDerpNodeIds,
        'reason': reason,
        if (relayRegionId != null && relayRegionId!.isNotEmpty)
          'relayRegionId': relayRegionId,
      };
}
