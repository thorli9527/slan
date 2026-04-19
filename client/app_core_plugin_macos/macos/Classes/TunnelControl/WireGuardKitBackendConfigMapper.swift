import Foundation

struct WireGuardKitBackendConfiguration: Equatable {
  let transport: String
  let debugEngineMode: String
  let localVirtualIp: String
  let peerVirtualIp: String
  let interfacePrivateKey: String
  let interfacePublicKey: String
  let interfaceAddresses: [String]
  let dnsServers: [String]
  let mtu: Int?
  let listenPort: Int?
  let peerPublicKey: String
  let endpoint: String?
  let allowedIps: [String]
  let persistentKeepaliveSeconds: Int?
}

enum WireGuardKitBackendConfigMapper {
  static func map(_ configuration: PacketTunnelProviderConfiguration) -> WireGuardKitBackendConfiguration {
    WireGuardKitBackendConfiguration(
      transport: "relay",
      debugEngineMode: configuration.debugEngineMode.rawValue,
      localVirtualIp: configuration.localVirtualIp,
      peerVirtualIp: configuration.peerVirtualIp,
      interfacePrivateKey: "pending-wireguardkit-private-key",
      interfacePublicKey: "pending-wireguardkit-public-key",
      interfaceAddresses: [configuration.interfaceAddress.address],
      dnsServers: configuration.dnsServers,
      mtu: configuration.mtu,
      listenPort: nil,
      peerPublicKey: "pending-wireguardkit-peer-public-key",
      endpoint: configuration.selectedEndpoint,
      allowedIps: configuration.allowedIps,
      persistentKeepaliveSeconds: nil
    )
  }
}
