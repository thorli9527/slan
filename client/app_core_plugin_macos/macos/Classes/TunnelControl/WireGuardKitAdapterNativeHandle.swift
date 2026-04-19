import Foundation

protocol WireGuardKitAdapterNativeHandleControlling: AnyObject {
  func stop()
  func handleInboundPackets(_ packets: [Data], protocols: [NSNumber]) throws -> PacketTunnelEngineOutput
  func runtimeSnapshot() -> WireGuardBackendRuntimeSnapshot?
}

protocol WireGuardKitAdapterNativeHandleCreating {
  func makeHandle(
    descriptor: WireGuardKitAdapterSessionHandleDescriptor,
    startedAtMs: Int64
  ) throws -> WireGuardKitAdapterNativeHandleControlling
}

enum WireGuardKitAdapterNativeHandleTerminalConfigurationMapper {
  static func map(
    _ descriptor: WireGuardKitAdapterSessionHandleDescriptor
  ) -> WireGuardKitTerminalRuntimeHandleConfiguration {
    WireGuardKitTerminalRuntimeHandleConfiguration(
      backendName: descriptor.backendName,
      peerVirtualIp: descriptor.peer.address,
      selectedEndpoint: descriptor.peer.endpoint
    )
  }
}

final class WireGuardKitAdapterNativeHandle: WireGuardKitAdapterNativeHandleControlling {
  private let session: WireGuardKitAdapterNativeHandleSessioning
  private let runtimeHandle: WireGuardKitTerminalRuntimeHandleControlling
  private let synthesizedSnapshot: WireGuardBackendRuntimeSnapshot
  private var backendState = "running"

  init(
    descriptor: WireGuardKitAdapterSessionHandleDescriptor,
    startedAtMs: Int64,
    session: WireGuardKitAdapterNativeHandleSessioning,
    runtimeHandle: WireGuardKitTerminalRuntimeHandleControlling
  ) {
    self.session = session
    self.runtimeHandle = runtimeHandle
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
    session.stop()
    runtimeHandle.stop()
    backendState = "stopped"
  }

  func handleInboundPackets(_ packets: [Data], protocols: [NSNumber]) throws -> PacketTunnelEngineOutput {
    try session.handleInboundPackets(packets, protocols: protocols)
  }

  func runtimeSnapshot() -> WireGuardBackendRuntimeSnapshot? {
    if let snapshot = session.runtimeSnapshot() {
      return snapshot
    }

    if let snapshot = runtimeHandle.runtimeSnapshot() {
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

final class UnavailableWireGuardKitAdapterNativeHandleCreator: WireGuardKitAdapterNativeHandleCreating {
  private let sessionFactory: WireGuardKitAdapterNativeHandleSessionFactorying
  private let runtimeHandleCreator: WireGuardKitTerminalRuntimeHandleCreating

  init(
    sessionFactory: WireGuardKitAdapterNativeHandleSessionFactorying =
      UnavailableWireGuardKitAdapterNativeHandleSessionFactory(),
    runtimeHandleCreator: WireGuardKitTerminalRuntimeHandleCreating = UnavailableWireGuardKitTerminalRuntimeHandleCreator()
  ) {
    self.sessionFactory = sessionFactory
    self.runtimeHandleCreator = runtimeHandleCreator
  }

  func makeHandle(
    descriptor: WireGuardKitAdapterSessionHandleDescriptor,
    startedAtMs: Int64
  ) throws -> WireGuardKitAdapterNativeHandleControlling {
    let session = try sessionFactory.makeSession(
      descriptor: descriptor,
      startedAtMs: startedAtMs
    )
    let runtimeHandle = try runtimeHandleCreator.makeHandle(
      configuration: WireGuardKitAdapterNativeHandleTerminalConfigurationMapper.map(descriptor),
      startedAtMs: startedAtMs
    )
    return WireGuardKitAdapterNativeHandle(
      descriptor: descriptor,
      startedAtMs: startedAtMs,
      session: session,
      runtimeHandle: runtimeHandle
    )
  }
}

final class WireGuardKitAdapterNativeHandleCreator: WireGuardKitAdapterNativeHandleCreating {
  private let sessionFactory: WireGuardKitAdapterNativeHandleSessionFactorying
  private let runtimeHandleCreator: WireGuardKitTerminalRuntimeHandleCreating

  init(
    sessionFactory: WireGuardKitAdapterNativeHandleSessionFactorying =
      WireGuardKitAdapterNativeHandleSessionFactory(),
    runtimeHandleCreator: WireGuardKitTerminalRuntimeHandleCreating = WireGuardKitTerminalRuntimeHandleCreator()
  ) {
    self.sessionFactory = sessionFactory
    self.runtimeHandleCreator = runtimeHandleCreator
  }

  func makeHandle(
    descriptor: WireGuardKitAdapterSessionHandleDescriptor,
    startedAtMs: Int64
  ) throws -> WireGuardKitAdapterNativeHandleControlling {
    let session = try sessionFactory.makeSession(
      descriptor: descriptor,
      startedAtMs: startedAtMs
    )
    let runtimeHandle = try runtimeHandleCreator.makeHandle(
      configuration: WireGuardKitAdapterNativeHandleTerminalConfigurationMapper.map(descriptor),
      startedAtMs: startedAtMs
    )
    return WireGuardKitAdapterNativeHandle(
      descriptor: descriptor,
      startedAtMs: startedAtMs,
      session: session,
      runtimeHandle: runtimeHandle
    )
  }
}
