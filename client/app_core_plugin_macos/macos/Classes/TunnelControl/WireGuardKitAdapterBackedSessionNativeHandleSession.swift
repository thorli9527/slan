import Foundation

struct WireGuardKitAdapterBackedSessionNativeHandleSessionConfiguration: Equatable {
  let backendName: String
  let frameworkPath: String?
  let moduleSource: String
  let interfaceAddress: String?
  let peerVirtualIp: String
  let selectedEndpoint: String?
}

enum WireGuardKitAdapterBackedSessionNativeHandleSessionConfigurationMapper {
  static func map(
    _ configuration: WireGuardKitAdapterBackedSessionNativeHandleConfiguration
  ) -> WireGuardKitAdapterBackedSessionNativeHandleSessionConfiguration {
    WireGuardKitAdapterBackedSessionNativeHandleSessionConfiguration(
      backendName: configuration.backendName,
      frameworkPath: configuration.frameworkPath,
      moduleSource: configuration.moduleSource,
      interfaceAddress: configuration.interfaceAddress,
      peerVirtualIp: configuration.peerVirtualIp,
      selectedEndpoint: configuration.selectedEndpoint
    )
  }
}

enum WireGuardKitAdapterBackedSessionNativeHandleSessionTerminalConfigurationMapper {
  static func map(
    _ configuration: WireGuardKitAdapterBackedSessionNativeHandleSessionConfiguration
  ) -> WireGuardKitTerminalRuntimeHandleConfiguration {
    WireGuardKitTerminalRuntimeHandleConfiguration(
      backendName: configuration.backendName,
      peerVirtualIp: configuration.peerVirtualIp,
      selectedEndpoint: configuration.selectedEndpoint
    )
  }
}

protocol WireGuardKitAdapterBackedSessionNativeHandleSessioning: AnyObject {
  func stop()
  func handleInboundPackets(_ packets: [Data], protocols: [NSNumber]) throws -> PacketTunnelEngineOutput
  func runtimeSnapshot() -> WireGuardBackendRuntimeSnapshot?
}

protocol WireGuardKitAdapterBackedSessionNativeHandleSessionCreating {
  func makeSession(
    configuration: WireGuardKitAdapterBackedSessionNativeHandleSessionConfiguration,
    startedAtMs: Int64
  ) throws -> WireGuardKitAdapterBackedSessionNativeHandleSessioning
}

protocol WireGuardKitAdapterBackedSessionNativeHandleSessionFactorying {
  func makeSession(
    configuration: WireGuardKitAdapterBackedSessionNativeHandleConfiguration,
    startedAtMs: Int64
  ) throws -> WireGuardKitAdapterBackedSessionNativeHandleSessioning
}

final class UnavailableWireGuardKitAdapterBackedSessionAdapterHandle:
  WireGuardKitAdapterBackedSessionAdapterHandleControlling
{
  func stop() {}

  func handleInboundPackets(_ packets: [Data], protocols: [NSNumber]) throws -> PacketTunnelEngineOutput {
    PacketTunnelEngineOutput(outboundPackets: [], outboundProtocols: [])
  }

  func runtimeSnapshot() -> WireGuardBackendRuntimeSnapshot? {
    nil
  }
}

final class WireGuardKitAdapterBackedSessionNativeHandleSession:
  WireGuardKitAdapterBackedSessionNativeHandleSessioning
{
  private let handle: WireGuardKitAdapterBackedSessionAdapterHandleControlling
  private let runtimeHandle: WireGuardKitTerminalRuntimeHandleControlling
  private let synthesizedSnapshot: WireGuardBackendRuntimeSnapshot
  private var backendState = "running"

  init(
    configuration: WireGuardKitAdapterBackedSessionNativeHandleSessionConfiguration,
    startedAtMs: Int64,
    handle: WireGuardKitAdapterBackedSessionAdapterHandleControlling,
    runtimeHandle: WireGuardKitTerminalRuntimeHandleControlling
  ) {
    self.handle = handle
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
    handle.stop()
    runtimeHandle.stop()
    backendState = "stopped"
  }

  func handleInboundPackets(_ packets: [Data], protocols: [NSNumber]) throws -> PacketTunnelEngineOutput {
    try handle.handleInboundPackets(packets, protocols: protocols)
  }

  func runtimeSnapshot() -> WireGuardBackendRuntimeSnapshot? {
    if let snapshot = handle.runtimeSnapshot() {
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

final class UnavailableWireGuardKitAdapterBackedSessionNativeHandleSessionCreator:
  WireGuardKitAdapterBackedSessionNativeHandleSessionCreating
{
  private let runtimeHandleCreator: WireGuardKitTerminalRuntimeHandleCreating

  init(
    runtimeHandleCreator: WireGuardKitTerminalRuntimeHandleCreating =
      UnavailableWireGuardKitTerminalRuntimeHandleCreator()
  ) {
    self.runtimeHandleCreator = runtimeHandleCreator
  }

  func makeSession(
    configuration: WireGuardKitAdapterBackedSessionNativeHandleSessionConfiguration,
    startedAtMs: Int64
  ) throws -> WireGuardKitAdapterBackedSessionNativeHandleSessioning {
    let runtimeHandle = try runtimeHandleCreator.makeHandle(
      configuration:
        WireGuardKitAdapterBackedSessionNativeHandleSessionTerminalConfigurationMapper.map(configuration),
      startedAtMs: startedAtMs
    )
    return WireGuardKitAdapterBackedSessionNativeHandleSession(
      configuration: configuration,
      startedAtMs: startedAtMs,
      handle: UnavailableWireGuardKitAdapterBackedSessionAdapterHandle(),
      runtimeHandle: runtimeHandle
    )
  }
}

final class WireGuardKitAdapterBackedSessionNativeHandleSessionCreator:
  WireGuardKitAdapterBackedSessionNativeHandleSessionCreating
{
  private let handleCreator: WireGuardKitAdapterBackedSessionAdapterHandleCreating
  private let runtimeHandleCreator: WireGuardKitTerminalRuntimeHandleCreating

  init(
    handleCreator: WireGuardKitAdapterBackedSessionAdapterHandleCreating =
      WireGuardKitAdapterBackedSessionAdapterHandleCreator(),
    runtimeHandleCreator: WireGuardKitTerminalRuntimeHandleCreating =
      WireGuardKitTerminalRuntimeHandleCreator()
  ) {
    self.handleCreator = handleCreator
    self.runtimeHandleCreator = runtimeHandleCreator
  }

  func makeSession(
    configuration: WireGuardKitAdapterBackedSessionNativeHandleSessionConfiguration,
    startedAtMs: Int64
  ) throws -> WireGuardKitAdapterBackedSessionNativeHandleSessioning {
    let handle = try handleCreator.makeHandle(
      configuration: WireGuardKitAdapterBackedSessionAdapterHandleConfigurationMapper.map(configuration),
      startedAtMs: startedAtMs
    )
    let runtimeHandle = try runtimeHandleCreator.makeHandle(
      configuration:
        WireGuardKitAdapterBackedSessionNativeHandleSessionTerminalConfigurationMapper.map(configuration),
      startedAtMs: startedAtMs
    )
    return WireGuardKitAdapterBackedSessionNativeHandleSession(
      configuration: configuration,
      startedAtMs: startedAtMs,
      handle: handle,
      runtimeHandle: runtimeHandle
    )
  }
}

final class UnavailableWireGuardKitAdapterBackedSessionNativeHandleSessionFactory:
  WireGuardKitAdapterBackedSessionNativeHandleSessionFactorying
{
  private let creator: WireGuardKitAdapterBackedSessionNativeHandleSessionCreating

  init(
    creator: WireGuardKitAdapterBackedSessionNativeHandleSessionCreating =
      UnavailableWireGuardKitAdapterBackedSessionNativeHandleSessionCreator()
  ) {
    self.creator = creator
  }

  func makeSession(
    configuration: WireGuardKitAdapterBackedSessionNativeHandleConfiguration,
    startedAtMs: Int64
  ) throws -> WireGuardKitAdapterBackedSessionNativeHandleSessioning {
    try creator.makeSession(
      configuration: WireGuardKitAdapterBackedSessionNativeHandleSessionConfigurationMapper.map(configuration),
      startedAtMs: startedAtMs
    )
  }
}

final class WireGuardKitAdapterBackedSessionNativeHandleSessionFactory:
  WireGuardKitAdapterBackedSessionNativeHandleSessionFactorying
{
  private let creator: WireGuardKitAdapterBackedSessionNativeHandleSessionCreating

  init(
    creator: WireGuardKitAdapterBackedSessionNativeHandleSessionCreating =
      WireGuardKitAdapterBackedSessionNativeHandleSessionCreator()
  ) {
    self.creator = creator
  }

  func makeSession(
    configuration: WireGuardKitAdapterBackedSessionNativeHandleConfiguration,
    startedAtMs: Int64
  ) throws -> WireGuardKitAdapterBackedSessionNativeHandleSessioning {
    try creator.makeSession(
      configuration: WireGuardKitAdapterBackedSessionNativeHandleSessionConfigurationMapper.map(configuration),
      startedAtMs: startedAtMs
    )
  }
}
