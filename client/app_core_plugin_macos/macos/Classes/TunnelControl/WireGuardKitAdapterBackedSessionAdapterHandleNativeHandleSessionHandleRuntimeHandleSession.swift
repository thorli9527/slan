import Foundation

struct WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleSessionConfiguration:
  Equatable
{
  let backendName: String
  let frameworkPath: String?
  let moduleSource: String
  let interfaceAddress: String?
  let peerVirtualIp: String
  let selectedEndpoint: String?
}

enum WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleSessionConfigurationMapper {
  static func map(
    _ configuration: WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleConfiguration
  ) -> WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleSessionConfiguration {
    WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleSessionConfiguration(
      backendName: configuration.backendName,
      frameworkPath: configuration.frameworkPath,
      moduleSource: configuration.moduleSource,
      interfaceAddress: configuration.interfaceAddress,
      peerVirtualIp: configuration.peerVirtualIp,
      selectedEndpoint: configuration.selectedEndpoint
    )
  }
}

enum WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleSessionTerminalConfigurationMapper {
  static func map(
    _ configuration:
      WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleSessionConfiguration
  ) -> WireGuardKitTerminalRuntimeHandleConfiguration {
    WireGuardKitTerminalRuntimeHandleConfiguration(
      backendName: configuration.backendName,
      peerVirtualIp: configuration.peerVirtualIp,
      selectedEndpoint: configuration.selectedEndpoint
    )
  }
}

protocol WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleSessioning:
  AnyObject
{
  func stop()
  func handleInboundPackets(_ packets: [Data], protocols: [NSNumber]) throws -> PacketTunnelEngineOutput
  func runtimeSnapshot() -> WireGuardBackendRuntimeSnapshot?
}

protocol WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleSessionCreating {
  func makeSession(
    configuration:
      WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleSessionConfiguration,
    startedAtMs: Int64
  ) throws
    -> WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleSessioning
}

protocol WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleSessionFactorying {
  func makeSession(
    configuration:
      WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleConfiguration,
    startedAtMs: Int64
  ) throws
    -> WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleSessioning
}

final class UnavailableWireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleSessionBackedSession:
  WireGuardKitDeepBackedSessioning
{
  func stop() {}

  func handleInboundPackets(_ packets: [Data], protocols: [NSNumber]) throws -> PacketTunnelEngineOutput {
    PacketTunnelEngineOutput(outboundPackets: [], outboundProtocols: [])
  }

  func runtimeSnapshot() -> WireGuardBackendRuntimeSnapshot? {
    nil
  }
}

final class WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleSession:
  WireGuardKitDeepRuntimeSessioning
{
  private let backedSession: WireGuardKitDeepBackedSessioning
  private let runtimeHandle: WireGuardKitTerminalRuntimeHandleControlling
  private let synthesizedSnapshot: WireGuardBackendRuntimeSnapshot
  private var backendState = "running"

  init(
    configuration: WireGuardKitDeepRuntimeSessionConfiguration,
    startedAtMs: Int64,
    backedSession: WireGuardKitDeepBackedSessioning,
    runtimeHandle: WireGuardKitTerminalRuntimeHandleControlling
  ) {
    self.backedSession = backedSession
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
    backedSession.stop()
    runtimeHandle.stop()
    backendState = "stopped"
  }

  func handleInboundPackets(_ packets: [Data], protocols: [NSNumber]) throws -> PacketTunnelEngineOutput {
    try backedSession.handleInboundPackets(packets, protocols: protocols)
  }

  func runtimeSnapshot() -> WireGuardBackendRuntimeSnapshot? {
    if let snapshot = backedSession.runtimeSnapshot() {
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

final class UnavailableWireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleSessionCreator:
  WireGuardKitDeepRuntimeSessionCreating
{
  private let runtimeHandleCreator: WireGuardKitTerminalRuntimeHandleCreating

  init(
    runtimeHandleCreator: WireGuardKitTerminalRuntimeHandleCreating =
      UnavailableWireGuardKitTerminalRuntimeHandleCreator()
  ) {
    self.runtimeHandleCreator = runtimeHandleCreator
  }

  func makeSession(
    configuration: WireGuardKitDeepRuntimeSessionConfiguration,
    startedAtMs: Int64
  ) throws -> WireGuardKitDeepRuntimeSessioning {
    let runtimeHandle = try runtimeHandleCreator.makeHandle(
      configuration: WireGuardKitDeepRuntimeSessionTerminalConfigurationMapper.map(configuration),
      startedAtMs: startedAtMs
    )
    return WireGuardKitDeepRuntimeSession(
      configuration: configuration,
      startedAtMs: startedAtMs,
      backedSession: UnavailableWireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleSessionBackedSession(),
      runtimeHandle: runtimeHandle
    )
  }
}

final class WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleSessionCreator:
  WireGuardKitDeepRuntimeSessionCreating
{
  private let backedSessionFactory: WireGuardKitDeepBackedSessionFactorying
  private let runtimeHandleCreator: WireGuardKitTerminalRuntimeHandleCreating

  init(
    backedSessionFactory: WireGuardKitDeepBackedSessionFactorying =
      WireGuardKitDeepBackedSessionFactory(),
    runtimeHandleCreator: WireGuardKitTerminalRuntimeHandleCreating =
      WireGuardKitTerminalRuntimeHandleCreator()
  ) {
    self.backedSessionFactory = backedSessionFactory
    self.runtimeHandleCreator = runtimeHandleCreator
  }

  func makeSession(
    configuration: WireGuardKitDeepRuntimeSessionConfiguration,
    startedAtMs: Int64
  ) throws -> WireGuardKitDeepRuntimeSessioning {
    let backedSession = try backedSessionFactory.makeSession(
      configuration: configuration,
      startedAtMs: startedAtMs
    )
    let runtimeHandle = try runtimeHandleCreator.makeHandle(
      configuration: WireGuardKitDeepRuntimeSessionTerminalConfigurationMapper.map(configuration),
      startedAtMs: startedAtMs
    )
    return WireGuardKitDeepRuntimeSession(
      configuration: configuration,
      startedAtMs: startedAtMs,
      backedSession: backedSession,
      runtimeHandle: runtimeHandle
    )
  }
}

final class UnavailableWireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleSessionFactory:
  WireGuardKitDeepRuntimeSessionFactorying
{
  private let creator: WireGuardKitDeepRuntimeSessionCreating

  init(
    creator: WireGuardKitDeepRuntimeSessionCreating =
      UnavailableWireGuardKitDeepRuntimeSessionCreator()
  ) {
    self.creator = creator
  }

  func makeSession(
    configuration:
      WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleConfiguration,
    startedAtMs: Int64
  ) throws -> WireGuardKitDeepRuntimeSessioning {
    try creator.makeSession(
      configuration: WireGuardKitDeepRuntimeSessionConfigurationMapper.map(configuration),
      startedAtMs: startedAtMs
    )
  }
}

final class WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleSessionFactory:
  WireGuardKitDeepRuntimeSessionFactorying
{
  private let creator: WireGuardKitDeepRuntimeSessionCreating

  init(
    creator: WireGuardKitDeepRuntimeSessionCreating =
      WireGuardKitDeepRuntimeSessionCreator()
  ) {
    self.creator = creator
  }

  func makeSession(
    configuration:
      WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleConfiguration,
    startedAtMs: Int64
  ) throws -> WireGuardKitDeepRuntimeSessioning {
    try creator.makeSession(
      configuration: WireGuardKitDeepRuntimeSessionConfigurationMapper.map(configuration),
      startedAtMs: startedAtMs
    )
  }
}
