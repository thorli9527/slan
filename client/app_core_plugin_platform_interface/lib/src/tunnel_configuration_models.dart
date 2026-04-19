class WireGuardTunnelKeyPair {
  const WireGuardTunnelKeyPair({
    required this.publicKey,
    required this.privateKey,
  });

  final String publicKey;
  final String privateKey;

  Map<String, Object?> toJson() => {
        'publicKey': publicKey,
        'privateKey': privateKey,
      };
}

class WireGuardTunnelPeerConfiguration {
  const WireGuardTunnelPeerConfiguration({
    required this.publicKey,
    required this.allowedIps,
    this.peerNodeId,
    this.presharedKey,
    this.endpoint,
    this.persistentKeepaliveSeconds,
  });

  final String? peerNodeId;
  final String publicKey;
  final String? presharedKey;
  final String? endpoint;
  final List<String> allowedIps;
  final int? persistentKeepaliveSeconds;

  Map<String, Object?> toJson() => {
        'peerNodeId': peerNodeId,
        'publicKey': publicKey,
        'presharedKey': presharedKey,
        'endpoint': endpoint,
        'allowedIps': allowedIps.map((cidr) => {'cidr': cidr}).toList(),
        'persistentKeepaliveSeconds': persistentKeepaliveSeconds,
      };
}

class WireGuardTunnelInterfaceConfiguration {
  const WireGuardTunnelInterfaceConfiguration({
    required this.keyPair,
    required this.addresses,
    this.interfaceName,
    this.listenPort,
    this.mtu,
    this.dnsServers = const [],
  });

  final String? interfaceName;
  final WireGuardTunnelKeyPair keyPair;
  final int? listenPort;
  final int? mtu;
  final List<String> addresses;
  final List<String> dnsServers;

  Map<String, Object?> toJson() => {
        'interfaceName': interfaceName,
        'keyPair': keyPair.toJson(),
        'listenPort': listenPort,
        'mtu': mtu,
        'addresses': addresses,
        'dnsServers': dnsServers,
      };
}

class WireGuardTunnelConfiguration {
  const WireGuardTunnelConfiguration({
    required this.transport,
    required this.localVirtualIp,
    required this.peerVirtualIp,
    required this.interface,
    required this.peer,
    this.debugEngineMode,
  });

  final String transport;
  final String localVirtualIp;
  final String peerVirtualIp;
  final WireGuardTunnelInterfaceConfiguration interface;
  final WireGuardTunnelPeerConfiguration peer;
  final String? debugEngineMode;

  Map<String, Object?> toJson() => {
        'transport': transport,
        'localVirtualIp': localVirtualIp,
        'peerVirtualIp': peerVirtualIp,
        'debugEngineMode': debugEngineMode,
        'wireguardInterface': interface.toJson(),
        'wireguardPeer': peer.toJson(),
      };
}
