import Foundation

struct WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleConfiguration:
  Equatable
{
  let backendName: String
  let frameworkPath: String?
  let moduleSource: String
  let interfaceAddress: String?
  let peerVirtualIp: String
  let selectedEndpoint: String?
}

enum WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleConfigurationMapper {
  static func map(
    _ configuration: WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleConfiguration
  ) -> WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleConfiguration {
    WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleConfiguration(
      backendName: configuration.backendName,
      frameworkPath: configuration.frameworkPath,
      moduleSource: configuration.moduleSource,
      interfaceAddress: configuration.interfaceAddress,
      peerVirtualIp: configuration.peerVirtualIp,
      selectedEndpoint: configuration.selectedEndpoint
    )
  }
}

enum WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleTerminalConfigurationMapper
{
  static func map(
    _ configuration:
      WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleConfiguration
  ) -> WireGuardKitTerminalRuntimeHandleConfiguration {
    WireGuardKitTerminalRuntimeHandleConfiguration(
      backendName: configuration.backendName,
      peerVirtualIp: configuration.peerVirtualIp,
      selectedEndpoint: configuration.selectedEndpoint
    )
  }
}

protocol WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleControlling:
  AnyObject
{
  func stop()
  func handleInboundPackets(_ packets: [Data], protocols: [NSNumber]) throws -> PacketTunnelEngineOutput
  func runtimeSnapshot() -> WireGuardBackendRuntimeSnapshot?
}

protocol WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleCreating {
  func makeHandle(
    configuration: WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleConfiguration,
    startedAtMs: Int64
  ) throws -> WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleControlling
}

final class UnavailableWireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleSession:
  WireGuardKitDeepRuntimeSessioning
{
  func stop() {}

  func handleInboundPackets(_ packets: [Data], protocols: [NSNumber]) throws -> PacketTunnelEngineOutput {
    PacketTunnelEngineOutput(outboundPackets: [], outboundProtocols: [])
  }

  func runtimeSnapshot() -> WireGuardBackendRuntimeSnapshot? {
    nil
  }
}

final class WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandle:
  WireGuardKitDeepRuntimeHandleControlling
{
  private let session: WireGuardKitDeepRuntimeSessioning
  private let runtimeHandle: WireGuardKitTerminalRuntimeHandleControlling
  private let synthesizedSnapshot: WireGuardBackendRuntimeSnapshot
  private var backendState = "running"

  init(
    configuration: WireGuardKitDeepRuntimeHandleConfiguration,
    startedAtMs: Int64,
    session: WireGuardKitDeepRuntimeSessioning,
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

final class UnavailableWireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleCreator:
  WireGuardKitDeepRuntimeHandleCreating
{
  private let runtimeHandleCreator: WireGuardKitTerminalRuntimeHandleCreating

  init(
    runtimeHandleCreator: WireGuardKitTerminalRuntimeHandleCreating =
      UnavailableWireGuardKitTerminalRuntimeHandleCreator()
  ) {
    self.runtimeHandleCreator = runtimeHandleCreator
  }

  func makeHandle(
    configuration: WireGuardKitDeepRuntimeHandleConfiguration,
    startedAtMs: Int64
  ) throws -> WireGuardKitDeepRuntimeHandleControlling {
    let runtimeHandle = try runtimeHandleCreator.makeHandle(
      configuration: WireGuardKitDeepRuntimeHandleTerminalConfigurationMapper.map(configuration),
      startedAtMs: startedAtMs
    )
    return WireGuardKitDeepRuntimeHandle(
      configuration: configuration,
      startedAtMs: startedAtMs,
      session:
        UnavailableWireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleSession(),
      runtimeHandle: runtimeHandle
    )
  }
}

final class WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleCreator:
  WireGuardKitDeepRuntimeHandleCreating
{
  private let sessionFactory: WireGuardKitDeepRuntimeSessionFactorying
  private let runtimeHandleCreator: WireGuardKitTerminalRuntimeHandleCreating

  init(
    sessionFactory: WireGuardKitDeepRuntimeSessionFactorying =
      WireGuardKitDeepRuntimeSessionFactory(),
    runtimeHandleCreator: WireGuardKitTerminalRuntimeHandleCreating =
      WireGuardKitTerminalRuntimeHandleCreator()
  ) {
    self.sessionFactory = sessionFactory
    self.runtimeHandleCreator = runtimeHandleCreator
  }

  func makeHandle(
    configuration: WireGuardKitDeepRuntimeHandleConfiguration,
    startedAtMs: Int64
  ) throws -> WireGuardKitDeepRuntimeHandleControlling {
    let session = try sessionFactory.makeSession(configuration: configuration, startedAtMs: startedAtMs)
    let runtimeHandle = try runtimeHandleCreator.makeHandle(
      configuration: WireGuardKitDeepRuntimeHandleTerminalConfigurationMapper.map(configuration),
      startedAtMs: startedAtMs
    )
    return WireGuardKitDeepRuntimeHandle(
      configuration: configuration,
      startedAtMs: startedAtMs,
      session: session,
      runtimeHandle: runtimeHandle
    )
  }
}
