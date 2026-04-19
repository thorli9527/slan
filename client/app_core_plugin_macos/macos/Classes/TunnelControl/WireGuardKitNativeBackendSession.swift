import Foundation

struct WireGuardKitNativeBackendConfiguration: Equatable {
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

enum WireGuardKitNativeBackendConfigMapper {
  static func map(_ configuration: WireGuardKitSessionConfiguration) -> WireGuardKitNativeBackendConfiguration {
    WireGuardKitNativeBackendConfiguration(
      localAddress: configuration.localAddress,
      peerAddress: configuration.peerAddress,
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

protocol WireGuardKitNativeBackendSessionControlling {
  func start(configuration: WireGuardKitNativeBackendConfiguration) throws
  func stop()
  func handleInboundPackets(_ packets: [Data], protocols: [NSNumber]) throws -> PacketTunnelEngineOutput
  func runtimeSnapshot() -> WireGuardBackendRuntimeSnapshot?
}

protocol WireGuardKitNativeBackendSessioning {
  func start(sessionConfiguration: WireGuardKitSessionConfiguration) throws
  func stop()
  func handleInboundPackets(_ packets: [Data], protocols: [NSNumber]) throws -> PacketTunnelEngineOutput
  func runtimeSnapshot() -> WireGuardBackendRuntimeSnapshot?
}

final class WireGuardKitNativeBackendSession: WireGuardKitNativeBackendSessioning {
  private let controller: WireGuardKitNativeBackendSessionControlling

  init(controller: WireGuardKitNativeBackendSessionControlling = WireGuardKitNativeController()) {
    self.controller = controller
  }

  func start(sessionConfiguration: WireGuardKitSessionConfiguration) throws {
    try controller.start(configuration: WireGuardKitNativeBackendConfigMapper.map(sessionConfiguration))
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

final class UnavailableWireGuardKitNativeBackendSession: WireGuardKitNativeBackendSessioning {
  private let session = WireGuardKitNativeBackendSession()

  func start(sessionConfiguration: WireGuardKitSessionConfiguration) throws {
    try session.start(sessionConfiguration: sessionConfiguration)
  }

  func stop() {
    session.stop()
  }

  func handleInboundPackets(_ packets: [Data], protocols: [NSNumber]) throws -> PacketTunnelEngineOutput {
    try session.handleInboundPackets(packets, protocols: protocols)
  }

  func runtimeSnapshot() -> WireGuardBackendRuntimeSnapshot? {
    session.runtimeSnapshot()
  }
}
