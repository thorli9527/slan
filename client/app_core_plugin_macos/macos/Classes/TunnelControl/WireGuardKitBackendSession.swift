import Foundation

struct WireGuardKitSessionConfiguration: Equatable {
  let localAddress: String
  let peerAddress: String
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

enum WireGuardKitSessionConfigMapper {
  static func map(_ configuration: WireGuardKitBackendConfiguration) -> WireGuardKitSessionConfiguration {
    WireGuardKitSessionConfiguration(
      localAddress: configuration.localVirtualIp,
      peerAddress: configuration.peerVirtualIp,
      interfacePrivateKey: configuration.interfacePrivateKey,
      interfacePublicKey: configuration.interfacePublicKey,
      interfaceAddresses: configuration.interfaceAddresses,
      dnsServers: configuration.dnsServers,
      mtu: configuration.mtu,
      listenPort: configuration.listenPort,
      peerPublicKey: configuration.peerPublicKey,
      endpoint: configuration.endpoint,
      allowedIps: configuration.allowedIps,
      persistentKeepaliveSeconds: configuration.persistentKeepaliveSeconds
    )
  }
}

protocol WireGuardKitBackendSessioning {
  func start(configuration: WireGuardKitBackendConfiguration) throws
  func stop()
  func handleInboundPackets(_ packets: [Data], protocols: [NSNumber]) throws -> PacketTunnelEngineOutput
  func runtimeSnapshot() -> WireGuardBackendRuntimeSnapshot?
}

final class WireGuardKitBackendSession: WireGuardKitBackendSessioning {
  private let controller: WireGuardKitBackendSessionControlling

  init(controller: WireGuardKitBackendSessionControlling = WireGuardKitController()) {
    self.controller = controller
  }

  func start(configuration: WireGuardKitBackendConfiguration) throws {
    try controller.start(sessionConfiguration: WireGuardKitSessionConfigMapper.map(configuration))
  }

  func stop() {
    controller.stop()
  }

  func handleInboundPackets(_ packets: [Data], protocols: [NSNumber]) throws -> PacketTunnelEngineOutput {
    try controller.handleInboundPackets(packets, protocols: protocols)
  }

  func runtimeSnapshot() -> WireGuardBackendRuntimeSnapshot? {
    controller.runtimeSnapshot()
  }
}
