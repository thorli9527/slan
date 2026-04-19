import Foundation

struct WireGuardKitAdapterBackedSessionAdapterHandleConfiguration: Equatable {
  let backendName: String
  let frameworkPath: String?
  let moduleSource: String
  let interfaceAddress: String?
  let peerVirtualIp: String
  let selectedEndpoint: String?
}

enum WireGuardKitAdapterBackedSessionAdapterHandleConfigurationMapper {
  static func map(
    _ configuration: WireGuardKitAdapterBackedSessionNativeHandleSessionConfiguration
  ) -> WireGuardKitAdapterBackedSessionAdapterHandleConfiguration {
    WireGuardKitAdapterBackedSessionAdapterHandleConfiguration(
      backendName: configuration.backendName,
      frameworkPath: configuration.frameworkPath,
      moduleSource: configuration.moduleSource,
      interfaceAddress: configuration.interfaceAddress,
      peerVirtualIp: configuration.peerVirtualIp,
      selectedEndpoint: configuration.selectedEndpoint
    )
  }
}

enum WireGuardKitAdapterBackedSessionAdapterHandleTerminalConfigurationMapper {
  static func map(
    _ configuration: WireGuardKitAdapterBackedSessionAdapterHandleConfiguration
  ) -> WireGuardKitTerminalRuntimeHandleConfiguration {
    WireGuardKitTerminalRuntimeHandleConfiguration(
      backendName: configuration.backendName,
      peerVirtualIp: configuration.peerVirtualIp,
      selectedEndpoint: configuration.selectedEndpoint
    )
  }
}

protocol WireGuardKitAdapterBackedSessionAdapterHandleControlling: AnyObject {
  func stop()
  func handleInboundPackets(_ packets: [Data], protocols: [NSNumber]) throws -> PacketTunnelEngineOutput
  func runtimeSnapshot() -> WireGuardBackendRuntimeSnapshot?
}

protocol WireGuardKitAdapterBackedSessionAdapterHandleCreating {
  func makeHandle(
    configuration: WireGuardKitAdapterBackedSessionAdapterHandleConfiguration,
    startedAtMs: Int64
  ) throws -> WireGuardKitAdapterBackedSessionAdapterHandleControlling
}

final class UnavailableWireGuardKitAdapterBackedSessionAdapterHandleSession:
  WireGuardKitAdapterBackedSessionAdapterHandleSessioning
{
  func stop() {}

  func handleInboundPackets(_ packets: [Data], protocols: [NSNumber]) throws -> PacketTunnelEngineOutput {
    PacketTunnelEngineOutput(outboundPackets: [], outboundProtocols: [])
  }

  func runtimeSnapshot() -> WireGuardBackendRuntimeSnapshot? {
    nil
  }
}

final class WireGuardKitAdapterBackedSessionAdapterHandle:
  WireGuardKitDeepAdapterHandleControlling
{
  private let session: WireGuardKitDeepAdapterSessioning
  private let runtimeHandle: WireGuardKitTerminalRuntimeHandleControlling
  private let synthesizedSnapshot: WireGuardBackendRuntimeSnapshot
  private var backendState = "running"

  init(
    configuration: WireGuardKitDeepAdapterHandleConfiguration,
    startedAtMs: Int64,
    session: WireGuardKitDeepAdapterSessioning,
    runtimeHandle: WireGuardKitTerminalRuntimeHandleControlling
  ) {
    self.session = session
    self.runtimeHandle = runtimeHandle
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

final class UnavailableWireGuardKitAdapterBackedSessionAdapterHandleCreator:
  WireGuardKitDeepAdapterHandleCreating
{
  private let runtimeHandleCreator: WireGuardKitTerminalRuntimeHandleCreating

  init(
    runtimeHandleCreator: WireGuardKitTerminalRuntimeHandleCreating =
      UnavailableWireGuardKitTerminalRuntimeHandleCreator()
  ) {
    self.runtimeHandleCreator = runtimeHandleCreator
  }

  func makeHandle(
    configuration: WireGuardKitDeepAdapterHandleConfiguration,
    startedAtMs: Int64
  ) throws -> WireGuardKitDeepAdapterHandleControlling {
    let runtimeHandle = try runtimeHandleCreator.makeHandle(
      configuration: WireGuardKitDeepAdapterHandleTerminalConfigurationMapper.map(configuration),
      startedAtMs: startedAtMs
    )
    return WireGuardKitDeepAdapterHandle(
      configuration: configuration,
      startedAtMs: startedAtMs,
      session: UnavailableWireGuardKitDeepAdapterSession(),
      runtimeHandle: runtimeHandle
    )
  }
}

final class WireGuardKitAdapterBackedSessionAdapterHandleCreator:
  WireGuardKitDeepAdapterHandleCreating
{
  private let sessionFactory: WireGuardKitDeepAdapterSessionFactorying
  private let runtimeHandleCreator: WireGuardKitTerminalRuntimeHandleCreating

  init(
    sessionFactory: WireGuardKitDeepAdapterSessionFactorying =
      WireGuardKitDeepAdapterSessionFactory(),
    runtimeHandleCreator: WireGuardKitTerminalRuntimeHandleCreating =
      WireGuardKitTerminalRuntimeHandleCreator()
  ) {
    self.sessionFactory = sessionFactory
    self.runtimeHandleCreator = runtimeHandleCreator
  }

  func makeHandle(
    configuration: WireGuardKitDeepAdapterHandleConfiguration,
    startedAtMs: Int64
  ) throws -> WireGuardKitDeepAdapterHandleControlling {
    let session = try sessionFactory.makeSession(
      configuration: configuration,
      startedAtMs: startedAtMs
    )
    let runtimeHandle = try runtimeHandleCreator.makeHandle(
      configuration: WireGuardKitDeepAdapterHandleTerminalConfigurationMapper.map(configuration),
      startedAtMs: startedAtMs
    )
    return WireGuardKitDeepAdapterHandle(
      configuration: configuration,
      startedAtMs: startedAtMs,
      session: session,
      runtimeHandle: runtimeHandle
    )
  }
}
