import Foundation

struct WireGuardKitAdapterInterfaceConfiguration: Equatable {
  let privateKey: String
  let publicKey: String
  let addresses: [String]
  let dnsServers: [String]
  let mtu: Int?
  let listenPort: Int?
}

struct WireGuardKitAdapterPeerConfiguration: Equatable {
  let publicKey: String
  let address: String
  let endpoint: String?
  let allowedIps: [String]
  let persistentKeepaliveSeconds: Int?
}

struct WireGuardKitAdapterSessionConfiguration: Equatable {
  let backendName: String
  let frameworkPath: String?
  let moduleSource: String
  let interface: WireGuardKitAdapterInterfaceConfiguration
  let peer: WireGuardKitAdapterPeerConfiguration
}

protocol WireGuardKitAdapterSessionCreating {
  func makeSession(configuration: WireGuardKitAdapterSessionConfiguration) throws -> WireGuardKitNativeSessioning
}

protocol WireGuardKitAdapterSessionHandleControlling: AnyObject {
  func stop()
  func handleInboundPackets(_ packets: [Data], protocols: [NSNumber]) throws -> PacketTunnelEngineOutput
  func runtimeSnapshot() -> WireGuardBackendRuntimeSnapshot?
}

protocol WireGuardKitAdapterSessionLifecycleControlling {
  func start(configuration: WireGuardKitAdapterSessionConfiguration) throws -> WireGuardKitAdapterSessionHandleControlling
}

protocol WireGuardKitAdapterSessionHandleFactorying {
  func makeHandle(configuration: WireGuardKitAdapterSessionConfiguration) throws -> WireGuardKitAdapterSessionHandleControlling
}

struct WireGuardKitAdapterSessionHandleDescriptor: Equatable {
  let backendName: String
  let frameworkPath: String?
  let moduleSource: String
  let interface: WireGuardKitAdapterInterfaceConfiguration
  let peer: WireGuardKitAdapterPeerConfiguration
}

enum WireGuardKitAdapterSessionHandleDescriptorMapper {
  static func map(_ configuration: WireGuardKitAdapterSessionConfiguration) -> WireGuardKitAdapterSessionHandleDescriptor {
    WireGuardKitAdapterSessionHandleDescriptor(
      backendName: configuration.backendName,
      frameworkPath: configuration.frameworkPath,
      moduleSource: configuration.moduleSource,
      interface: configuration.interface,
      peer: configuration.peer
    )
  }
}

protocol WireGuardKitAdapterSessionHandleCreating {
  func makeHandle(
    descriptor: WireGuardKitAdapterSessionHandleDescriptor,
    startedAtMs: Int64
  ) throws -> WireGuardKitAdapterSessionHandleControlling
}

final class WireGuardKitAdapterSessionHandle: WireGuardKitAdapterSessionHandleControlling {
  private let nativeHandle: WireGuardKitAdapterNativeHandleControlling
  private let synthesizedSnapshot: WireGuardBackendRuntimeSnapshot
  private var backendState = "running"

  init(
    descriptor: WireGuardKitAdapterSessionHandleDescriptor,
    startedAtMs: Int64,
    nativeHandle: WireGuardKitAdapterNativeHandleControlling
  ) {
    self.nativeHandle = nativeHandle
    self.synthesizedSnapshot = WireGuardBackendRuntimeSnapshot(
      backendName: descriptor.backendName,
      backendState: "running",
      lastBackendError: nil,
      lastStartedAtMs: startedAtMs,
      peerVirtualIp: descriptor.peer.address,
      selectedEndpoint: descriptor.peer.endpoint
    )
  }

  func stop() {
    nativeHandle.stop()
    backendState = "stopped"
  }

  func handleInboundPackets(_ packets: [Data], protocols: [NSNumber]) throws -> PacketTunnelEngineOutput {
    try nativeHandle.handleInboundPackets(packets, protocols: protocols)
  }

  func runtimeSnapshot() -> WireGuardBackendRuntimeSnapshot? {
    if let snapshot = nativeHandle.runtimeSnapshot() {
      return snapshot
    }

    return WireGuardBackendRuntimeSnapshot(
      backendName: synthesizedSnapshot.backendName,
      backendState: backendState,
      lastBackendError: synthesizedSnapshot.lastBackendError,
      lastStartedAtMs: synthesizedSnapshot.lastStartedAtMs,
      peerVirtualIp: synthesizedSnapshot.peerVirtualIp,
      selectedEndpoint: synthesizedSnapshot.selectedEndpoint
    )
  }
}

final class UnavailableWireGuardKitAdapterSessionHandleCreator: WireGuardKitAdapterSessionHandleCreating {
  func makeHandle(
    descriptor: WireGuardKitAdapterSessionHandleDescriptor,
    startedAtMs: Int64
  ) throws -> WireGuardKitAdapterSessionHandleControlling {
    throw PacketTunnelProviderError.invalidConfiguration("WireGuardKit native backend session is not integrated yet")
  }
}

final class WireGuardKitAdapterSessionHandleCreator: WireGuardKitAdapterSessionHandleCreating {
  private let nativeHandleFactory: WireGuardKitAdapterNativeHandleFactorying

  init(
    nativeHandleFactory: WireGuardKitAdapterNativeHandleFactorying = WireGuardKitAdapterNativeHandleFactory()
  ) {
    self.nativeHandleFactory = nativeHandleFactory
  }

  func makeHandle(
    descriptor: WireGuardKitAdapterSessionHandleDescriptor,
    startedAtMs: Int64
  ) throws -> WireGuardKitAdapterSessionHandleControlling {
    let nativeHandle = try nativeHandleFactory.makeHandle(
      descriptor: descriptor,
      startedAtMs: startedAtMs
    )
    return WireGuardKitAdapterSessionHandle(
      descriptor: descriptor,
      startedAtMs: startedAtMs,
      nativeHandle: nativeHandle
    )
  }
}

final class UnavailableWireGuardKitAdapterSessionHandleFactory: WireGuardKitAdapterSessionHandleFactorying {
  func makeHandle(configuration: WireGuardKitAdapterSessionConfiguration) throws -> WireGuardKitAdapterSessionHandleControlling {
    throw PacketTunnelProviderError.invalidConfiguration("WireGuardKit native backend session is not integrated yet")
  }
}

final class WireGuardKitAdapterSessionHandleFactory: WireGuardKitAdapterSessionHandleFactorying {
  private let handleCreator: WireGuardKitAdapterSessionHandleCreating
  private let nowMs: () -> Int64

  init(
    handleCreator: WireGuardKitAdapterSessionHandleCreating = WireGuardKitAdapterSessionHandleCreator(),
    nowMs: @escaping () -> Int64 = { Int64(Date().timeIntervalSince1970 * 1000) }
  ) {
    self.handleCreator = handleCreator
    self.nowMs = nowMs
  }

  func makeHandle(configuration: WireGuardKitAdapterSessionConfiguration) throws -> WireGuardKitAdapterSessionHandleControlling {
    try handleCreator.makeHandle(
      descriptor: WireGuardKitAdapterSessionHandleDescriptorMapper.map(configuration),
      startedAtMs: nowMs()
    )
  }
}

final class UnavailableWireGuardKitAdapterSessionLifecycleController: WireGuardKitAdapterSessionLifecycleControlling {
  func start(configuration: WireGuardKitAdapterSessionConfiguration) throws -> WireGuardKitAdapterSessionHandleControlling {
    throw PacketTunnelProviderError.invalidConfiguration("WireGuardKit native backend session is not integrated yet")
  }
}

final class WireGuardKitAdapterSessionLifecycleController: WireGuardKitAdapterSessionLifecycleControlling {
  private let handleFactory: WireGuardKitAdapterSessionHandleFactorying

  init(
    handleFactory: WireGuardKitAdapterSessionHandleFactorying = WireGuardKitAdapterSessionHandleFactory()
  ) {
    self.handleFactory = handleFactory
  }

  func start(configuration: WireGuardKitAdapterSessionConfiguration) throws -> WireGuardKitAdapterSessionHandleControlling {
    try handleFactory.makeHandle(configuration: configuration)
  }
}

final class WireGuardKitAdapterBackedNativeSession: WireGuardKitNativeSessioning {
  private let handle: WireGuardKitAdapterSessionHandleControlling

  init(handle: WireGuardKitAdapterSessionHandleControlling) {
    self.handle = handle
  }

  func stop() {
    handle.stop()
  }

  func handleInboundPackets(_ packets: [Data], protocols: [NSNumber]) throws -> PacketTunnelEngineOutput {
    try handle.handleInboundPackets(packets, protocols: protocols)
  }

  func runtimeSnapshot() -> WireGuardBackendRuntimeSnapshot? {
    handle.runtimeSnapshot()
  }
}

final class UnavailableWireGuardKitAdapterSessionCreator: WireGuardKitAdapterSessionCreating {
  private let lifecycleController: WireGuardKitAdapterSessionLifecycleControlling

  init(
    lifecycleController: WireGuardKitAdapterSessionLifecycleControlling = UnavailableWireGuardKitAdapterSessionLifecycleController()
  ) {
    self.lifecycleController = lifecycleController
  }

  func makeSession(configuration: WireGuardKitAdapterSessionConfiguration) throws -> WireGuardKitNativeSessioning {
    let handle = try lifecycleController.start(configuration: configuration)
    return WireGuardKitAdapterBackedNativeSession(handle: handle)
  }
}

final class WireGuardKitAdapterSessionCreator: WireGuardKitAdapterSessionCreating {
  private let lifecycleController: WireGuardKitAdapterSessionLifecycleControlling

  init(
    lifecycleController: WireGuardKitAdapterSessionLifecycleControlling = WireGuardKitAdapterSessionLifecycleController()
  ) {
    self.lifecycleController = lifecycleController
  }

  func makeSession(configuration: WireGuardKitAdapterSessionConfiguration) throws -> WireGuardKitNativeSessioning {
    let handle = try lifecycleController.start(configuration: configuration)
    return WireGuardKitAdapterBackedNativeSession(handle: handle)
  }
}
