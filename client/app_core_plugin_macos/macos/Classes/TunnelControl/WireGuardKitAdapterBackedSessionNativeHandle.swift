import Foundation

struct WireGuardKitAdapterBackedSessionNativeHandleConfiguration: Equatable {
  let backendName: String
  let frameworkPath: String?
  let moduleSource: String
  let interfaceAddress: String?
  let peerVirtualIp: String
  let selectedEndpoint: String?
}

enum WireGuardKitAdapterBackedSessionNativeHandleConfigurationMapper {
  static func map(
    _ configuration: WireGuardKitAdapterBackedSessionHandleConfiguration
  ) -> WireGuardKitAdapterBackedSessionNativeHandleConfiguration {
    WireGuardKitAdapterBackedSessionNativeHandleConfiguration(
      backendName: configuration.backendName,
      frameworkPath: configuration.frameworkPath,
      moduleSource: configuration.moduleSource,
      interfaceAddress: configuration.interfaceAddress,
      peerVirtualIp: configuration.peerVirtualIp,
      selectedEndpoint: configuration.selectedEndpoint
    )
  }
}

enum WireGuardKitAdapterBackedSessionNativeHandleTerminalConfigurationMapper {
  static func map(
    _ configuration: WireGuardKitAdapterBackedSessionNativeHandleConfiguration
  ) -> WireGuardKitTerminalRuntimeHandleConfiguration {
    WireGuardKitTerminalRuntimeHandleConfiguration(
      backendName: configuration.backendName,
      peerVirtualIp: configuration.peerVirtualIp,
      selectedEndpoint: configuration.selectedEndpoint
    )
  }
}

protocol WireGuardKitAdapterBackedSessionNativeHandleControlling: AnyObject {
  func stop()
  func handleInboundPackets(_ packets: [Data], protocols: [NSNumber]) throws -> PacketTunnelEngineOutput
  func runtimeSnapshot() -> WireGuardBackendRuntimeSnapshot?
}

protocol WireGuardKitAdapterBackedSessionNativeHandleCreating {
  func makeHandle(
    configuration: WireGuardKitAdapterBackedSessionNativeHandleConfiguration,
    startedAtMs: Int64
  ) throws -> WireGuardKitAdapterBackedSessionNativeHandleControlling
}

protocol WireGuardKitAdapterBackedSessionNativeHandleFactorying {
  func makeHandle(
    configuration: WireGuardKitAdapterBackedSessionHandleConfiguration,
    startedAtMs: Int64
  ) throws -> WireGuardKitAdapterBackedSessionNativeHandleControlling
}

final class UnavailableWireGuardKitAdapterBackedSessionNativeHandleSession:
  WireGuardKitAdapterBackedSessionNativeHandleSessioning
{
  func stop() {}

  func handleInboundPackets(_ packets: [Data], protocols: [NSNumber]) throws -> PacketTunnelEngineOutput {
    PacketTunnelEngineOutput(outboundPackets: [], outboundProtocols: [])
  }

  func runtimeSnapshot() -> WireGuardBackendRuntimeSnapshot? {
    nil
  }
}

final class WireGuardKitAdapterBackedSessionNativeHandle: WireGuardKitDeepBackedNativeHandleControlling {
  private let session: WireGuardKitAdapterBackedSessionNativeHandleSessioning
  private let runtimeHandle: WireGuardKitTerminalRuntimeHandleControlling
  private let synthesizedSnapshot: WireGuardBackendRuntimeSnapshot
  private var backendState = "running"

  init(
    configuration: WireGuardKitDeepBackedNativeHandleConfiguration,
    startedAtMs: Int64,
    session: WireGuardKitAdapterBackedSessionNativeHandleSessioning,
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

final class UnavailableWireGuardKitAdapterBackedSessionNativeHandleCreator: WireGuardKitDeepBackedNativeHandleCreating {
  private let runtimeHandleCreator: WireGuardKitTerminalRuntimeHandleCreating

  init(
    runtimeHandleCreator: WireGuardKitTerminalRuntimeHandleCreating =
      UnavailableWireGuardKitTerminalRuntimeHandleCreator()
  ) {
    self.runtimeHandleCreator = runtimeHandleCreator
  }

  func makeHandle(
    configuration: WireGuardKitDeepBackedNativeHandleConfiguration,
    startedAtMs: Int64
  ) throws -> WireGuardKitDeepBackedNativeHandleControlling {
    let runtimeHandle = try runtimeHandleCreator.makeHandle(
      configuration: WireGuardKitDeepBackedNativeHandleTerminalConfigurationMapper.map(configuration),
      startedAtMs: startedAtMs
    )
    return WireGuardKitDeepBackedNativeHandle(
      configuration: configuration,
      startedAtMs: startedAtMs,
      session: UnavailableWireGuardKitAdapterBackedSessionNativeHandleSession(),
      runtimeHandle: runtimeHandle
    )
  }
}

final class WireGuardKitAdapterBackedSessionNativeHandleCreator: WireGuardKitDeepBackedNativeHandleCreating {
  private let sessionFactory: WireGuardKitAdapterBackedSessionNativeHandleSessionFactorying
  private let runtimeHandleCreator: WireGuardKitTerminalRuntimeHandleCreating

  init(
    sessionFactory: WireGuardKitAdapterBackedSessionNativeHandleSessionFactorying =
      WireGuardKitAdapterBackedSessionNativeHandleSessionFactory(),
    runtimeHandleCreator: WireGuardKitTerminalRuntimeHandleCreating =
      WireGuardKitTerminalRuntimeHandleCreator()
  ) {
    self.sessionFactory = sessionFactory
    self.runtimeHandleCreator = runtimeHandleCreator
  }

  func makeHandle(
    configuration: WireGuardKitDeepBackedNativeHandleConfiguration,
    startedAtMs: Int64
  ) throws -> WireGuardKitDeepBackedNativeHandleControlling {
    let session = try sessionFactory.makeSession(
      configuration: configuration,
      startedAtMs: startedAtMs
    )
    let runtimeHandle = try runtimeHandleCreator.makeHandle(
      configuration: WireGuardKitDeepBackedNativeHandleTerminalConfigurationMapper.map(configuration),
      startedAtMs: startedAtMs
    )
    return WireGuardKitDeepBackedNativeHandle(
      configuration: configuration,
      startedAtMs: startedAtMs,
      session: session,
      runtimeHandle: runtimeHandle
    )
  }
}

final class UnavailableWireGuardKitAdapterBackedSessionNativeHandleFactory:
  WireGuardKitDeepBackedNativeHandleFactorying
{
  private let creator: WireGuardKitDeepBackedNativeHandleCreating

  init(
    creator: WireGuardKitDeepBackedNativeHandleCreating =
      UnavailableWireGuardKitDeepBackedNativeHandleCreator()
  ) {
    self.creator = creator
  }

  func makeHandle(
    configuration: WireGuardKitAdapterBackedSessionHandleConfiguration,
    startedAtMs: Int64
  ) throws -> WireGuardKitDeepBackedNativeHandleControlling {
    try creator.makeHandle(
      configuration: WireGuardKitDeepBackedNativeHandleConfigurationMapper.map(configuration),
      startedAtMs: startedAtMs
    )
  }
}

final class WireGuardKitAdapterBackedSessionNativeHandleFactory:
  WireGuardKitDeepBackedNativeHandleFactorying
{
  private let creator: WireGuardKitDeepBackedNativeHandleCreating

  init(
    creator: WireGuardKitDeepBackedNativeHandleCreating =
      WireGuardKitDeepBackedNativeHandleCreator()
  ) {
    self.creator = creator
  }

  func makeHandle(
    configuration: WireGuardKitAdapterBackedSessionHandleConfiguration,
    startedAtMs: Int64
  ) throws -> WireGuardKitDeepBackedNativeHandleControlling {
    try creator.makeHandle(
      configuration: WireGuardKitDeepBackedNativeHandleConfigurationMapper.map(configuration),
      startedAtMs: startedAtMs
    )
  }
}
