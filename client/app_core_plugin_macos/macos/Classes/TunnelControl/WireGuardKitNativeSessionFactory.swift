import Foundation

struct WireGuardKitNativeSessionInterfaceConfiguration: Equatable {
  let privateKey: String
  let publicKey: String
  let addresses: [String]
  let dnsServers: [String]
  let mtu: Int?
  let listenPort: Int?
}

struct WireGuardKitNativeSessionPeerConfiguration: Equatable {
  let publicKey: String
  let address: String
  let endpoint: String?
  let allowedIps: [String]
  let persistentKeepaliveSeconds: Int?
}

struct WireGuardKitNativeSessionConfiguration: Equatable {
  let backendName: String
  let frameworkPath: String?
  let moduleSource: String
  let interface: WireGuardKitNativeSessionInterfaceConfiguration
  let peer: WireGuardKitNativeSessionPeerConfiguration
}

enum WireGuardKitNativeSessionConfigurationMapper {
  static func map(
    module: WireGuardKitNativeModule,
    configuration: WireGuardKitNativeBackendConfiguration
  ) -> WireGuardKitNativeSessionConfiguration {
    WireGuardKitNativeSessionConfiguration(
      backendName: module.backendName,
      frameworkPath: module.frameworkPath,
      moduleSource: module.moduleSource,
      interface: WireGuardKitNativeSessionInterfaceConfiguration(
        privateKey: configuration.interfacePrivateKey,
        publicKey: configuration.interfacePublicKey,
        addresses: configuration.interfaceAddresses,
        dnsServers: configuration.dnsServers,
        mtu: configuration.mtu,
        listenPort: configuration.listenPort
      ),
      peer: WireGuardKitNativeSessionPeerConfiguration(
        publicKey: configuration.peerPublicKey,
        address: configuration.peerAddress,
        endpoint: configuration.endpoint,
        allowedIps: configuration.allowedIps,
        persistentKeepaliveSeconds: configuration.persistentKeepaliveSeconds
      )
    )
  }
}

protocol WireGuardKitNativeSessionFactorying {
  func makeSession(
    module: WireGuardKitNativeModule,
    configuration: WireGuardKitNativeBackendConfiguration
  ) throws -> WireGuardKitNativeSessioning
}

final class UnavailableWireGuardKitNativeSessionFactory: WireGuardKitNativeSessionFactorying {
  private let builder: WireGuardKitNativeSessionBuilding

  init(builder: WireGuardKitNativeSessionBuilding = UnavailableWireGuardKitNativeSessionBuilder()) {
    self.builder = builder
  }

  func makeSession(
    module: WireGuardKitNativeModule,
    configuration: WireGuardKitNativeBackendConfiguration
  ) throws -> WireGuardKitNativeSessioning {
    let sessionConfiguration = WireGuardKitNativeSessionConfigurationMapper.map(
      module: module,
      configuration: configuration
    )
    return try builder.makeSession(configuration: sessionConfiguration)
  }
}

final class WireGuardKitNativeSessionFactory: WireGuardKitNativeSessionFactorying {
  private let sessionBuilder: WireGuardKitNativeSessionBuilding

  init(
    sessionBuilder: WireGuardKitNativeSessionBuilding = WireGuardKitNativeSessionBuilder()
  ) {
    self.sessionBuilder = sessionBuilder
  }

  func makeSession(
    module: WireGuardKitNativeModule,
    configuration: WireGuardKitNativeBackendConfiguration
  ) throws -> WireGuardKitNativeSessioning {
    let sessionConfiguration = WireGuardKitNativeSessionConfigurationMapper.map(
      module: module,
      configuration: configuration
    )
    return try sessionBuilder.makeSession(configuration: sessionConfiguration)
  }
}
