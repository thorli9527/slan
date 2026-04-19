import Foundation

struct WireGuardKitAdapterBackedSessionConfiguration: Equatable {
  let backendName: String
  let frameworkPath: String?
  let moduleSource: String
  let interfaceAddress: String?
  let peerVirtualIp: String
  let selectedEndpoint: String?
}

enum WireGuardKitAdapterBackedSessionConfigurationMapper {
  static func map(
    descriptor: WireGuardKitAdapterSessionHandleDescriptor
  ) -> WireGuardKitAdapterBackedSessionConfiguration {
    WireGuardKitAdapterBackedSessionConfiguration(
      backendName: descriptor.backendName,
      frameworkPath: descriptor.frameworkPath,
      moduleSource: descriptor.moduleSource,
      interfaceAddress: descriptor.interface.addresses.first,
      peerVirtualIp: descriptor.peer.address,
      selectedEndpoint: descriptor.peer.endpoint
    )
  }
}

protocol WireGuardKitAdapterBackedSessionCreating {
  func makeSession(
    configuration: WireGuardKitAdapterBackedSessionConfiguration,
    startedAtMs: Int64
  ) throws -> WireGuardKitAdapterNativeHandleSessioning
}

final class UnavailableWireGuardKitAdapterBackedSessionCreator: WireGuardKitAdapterBackedSessionCreating {
  func makeSession(
    configuration: WireGuardKitAdapterBackedSessionConfiguration,
    startedAtMs: Int64
  ) throws -> WireGuardKitAdapterNativeHandleSessioning {
    throw PacketTunnelProviderError.invalidConfiguration("WireGuardKit native backend session is not integrated yet")
  }
}

final class WireGuardKitAdapterBackedSession: WireGuardKitAdapterNativeHandleSessioning {
  private let handle: WireGuardKitAdapterBackedSessionHandleControlling
  private let synthesizedSnapshot: WireGuardBackendRuntimeSnapshot
  private var backendState = "running"

  init(
    configuration: WireGuardKitAdapterBackedSessionConfiguration,
    startedAtMs: Int64,
    handle: WireGuardKitAdapterBackedSessionHandleControlling
  ) {
    self.handle = handle
    self.synthesizedSnapshot = WireGuardBackendRuntimeSnapshot(
      backendName: configuration.backendName,
      backendState: "running",
      lastBackendError: nil,
      lastStartedAtMs: startedAtMs,
      peerVirtualIp: configuration.peerVirtualIp,
      selectedEndpoint: configuration.selectedEndpoint
    )
  }

  func stop() {
    handle.stop()
    backendState = "stopped"
  }

  func handleInboundPackets(_ packets: [Data], protocols: [NSNumber]) throws -> PacketTunnelEngineOutput {
    try handle.handleInboundPackets(packets, protocols: protocols)
  }

  func runtimeSnapshot() -> WireGuardBackendRuntimeSnapshot? {
    if let snapshot = handle.runtimeSnapshot() {
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

final class WireGuardKitAdapterBackedSessionCreator: WireGuardKitAdapterBackedSessionCreating {
  private let handleFactory: WireGuardKitAdapterBackedSessionHandleFactorying

  init(
    handleFactory: WireGuardKitAdapterBackedSessionHandleFactorying = WireGuardKitAdapterBackedSessionHandleFactory()
  ) {
    self.handleFactory = handleFactory
  }

  func makeSession(
    configuration: WireGuardKitAdapterBackedSessionConfiguration,
    startedAtMs: Int64
  ) throws -> WireGuardKitAdapterNativeHandleSessioning {
    let handle = try handleFactory.makeHandle(
      configuration: configuration,
      startedAtMs: startedAtMs
    )
    return WireGuardKitAdapterBackedSession(
      configuration: configuration,
      startedAtMs: startedAtMs,
      handle: handle
    )
  }
}
