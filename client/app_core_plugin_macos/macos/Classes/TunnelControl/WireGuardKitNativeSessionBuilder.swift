import Foundation

enum WireGuardKitAdapterSessionConfigurationMapper {
  static func map(_ configuration: WireGuardKitNativeSessionConfiguration) -> WireGuardKitAdapterSessionConfiguration {
    WireGuardKitAdapterSessionConfiguration(
      backendName: configuration.backendName,
      frameworkPath: configuration.frameworkPath,
      moduleSource: configuration.moduleSource,
      interface: WireGuardKitAdapterInterfaceConfiguration(
        privateKey: configuration.interface.privateKey,
        publicKey: configuration.interface.publicKey,
        addresses: configuration.interface.addresses,
        dnsServers: configuration.interface.dnsServers,
        mtu: configuration.interface.mtu,
        listenPort: configuration.interface.listenPort
      ),
      peer: WireGuardKitAdapterPeerConfiguration(
        publicKey: configuration.peer.publicKey,
        address: configuration.peer.address,
        endpoint: configuration.peer.endpoint,
        allowedIps: configuration.peer.allowedIps,
        persistentKeepaliveSeconds: configuration.peer.persistentKeepaliveSeconds
      )
    )
  }
}

protocol WireGuardKitNativeSessionBuilding {
  func makeSession(configuration: WireGuardKitNativeSessionConfiguration) throws -> WireGuardKitNativeSessioning
}

final class UnavailableWireGuardKitNativeSessionBuilder: WireGuardKitNativeSessionBuilding {
  private let creator: WireGuardKitAdapterSessionCreating

  init(creator: WireGuardKitAdapterSessionCreating = UnavailableWireGuardKitAdapterSessionCreator()) {
    self.creator = creator
  }

  func makeSession(configuration: WireGuardKitNativeSessionConfiguration) throws -> WireGuardKitNativeSessioning {
    try creator.makeSession(configuration: WireGuardKitAdapterSessionConfigurationMapper.map(configuration))
  }
}

final class WireGuardKitNativeSessionBuilder: WireGuardKitNativeSessionBuilding {
  private let sessionCreator: WireGuardKitAdapterSessionCreating

  init(
    sessionCreator: WireGuardKitAdapterSessionCreating = WireGuardKitAdapterSessionCreator()
  ) {
    self.sessionCreator = sessionCreator
  }

  func makeSession(configuration: WireGuardKitNativeSessionConfiguration) throws -> WireGuardKitNativeSessioning {
    try sessionCreator.makeSession(configuration: WireGuardKitAdapterSessionConfigurationMapper.map(configuration))
  }
}
