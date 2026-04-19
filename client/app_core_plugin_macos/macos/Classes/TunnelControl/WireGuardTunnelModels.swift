import Foundation

struct WireGuardTunnelKeyPair: Equatable {
  let publicKey: String
  let privateKey: String
}

struct WireGuardTunnelPeerConfiguration: Equatable {
  let peerNodeId: String?
  let publicKey: String
  let presharedKey: String?
  let endpoint: String?
  let allowedIps: [String]
  let persistentKeepaliveSeconds: Int?
}

struct WireGuardTunnelInterfaceConfiguration: Equatable {
  let interfaceName: String?
  let keyPair: WireGuardTunnelKeyPair
  let listenPort: Int?
  let mtu: Int?
  let addresses: [String]
  let dnsServers: [String]
}

struct WireGuardTunnelConfiguration: Equatable {
  let transport: String
  let localVirtualIp: String
  let peerVirtualIp: String
  let debugEngineMode: String?
  let interface: WireGuardTunnelInterfaceConfiguration
  let peer: WireGuardTunnelPeerConfiguration
}

struct WireGuardTunnelRuntimeView: Equatable {
  let state: String
  let transport: String
  let debugEngineMode: String?
  let backendName: String?
  let backendState: String?
  let backendLastError: String?
  let backendLastStartedAtMs: Int64?
  let backendPeerVirtualIp: String?
  let backendSelectedEndpoint: String?
  let peerVirtualIp: String
  let peerPublicKey: String
  let selectedEndpoint: String?
  let interfaceName: String?
  let dnsServers: [String]
  let allowedIps: [String]
  let localVirtualIp: String
  let remoteAddress: String
  let mtu: Int?
  let interfaceAddresses: [String]
  let includedRoutes: [String]
  let packetRxCount: Int64
  let packetRxBytes: Int64
  let packetTxCount: Int64
  let packetTxBytes: Int64
  let lastPacketAtMs: Int64?
  let lastAppliedAtMs: Int64?
  let lastError: String?
}
