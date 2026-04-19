import Foundation

struct WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionConfiguration: Equatable {
  let backendName: String
  let frameworkPath: String?
  let moduleSource: String
  let interfaceAddress: String?
  let peerVirtualIp: String
  let selectedEndpoint: String?
}

enum WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionConfigurationMapper {
  static func map(
    _ configuration: WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleConfiguration
  ) -> WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionConfiguration {
    WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionConfiguration(
      backendName: configuration.backendName,
      frameworkPath: configuration.frameworkPath,
      moduleSource: configuration.moduleSource,
      interfaceAddress: configuration.interfaceAddress,
      peerVirtualIp: configuration.peerVirtualIp,
      selectedEndpoint: configuration.selectedEndpoint
    )
  }
}

protocol WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessioning: AnyObject {
  func stop()
  func handleInboundPackets(_ packets: [Data], protocols: [NSNumber]) throws -> PacketTunnelEngineOutput
  func runtimeSnapshot() -> WireGuardBackendRuntimeSnapshot?
}

protocol WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionCreating {
  func makeSession(
    configuration: WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionConfiguration,
    startedAtMs: Int64
  ) throws -> WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessioning
}

protocol WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionFactorying {
  func makeSession(
    configuration: WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleConfiguration,
    startedAtMs: Int64
  ) throws -> WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessioning
}

final class WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSession:
  WireGuardKitDeepNativeSessioning
{
  private let handle: WireGuardKitDeepRuntimeBridgeHandleControlling
  private let synthesizedSnapshot: WireGuardBackendRuntimeSnapshot
  private var backendState = "running"

  init(
    configuration: WireGuardKitDeepNativeSessionConfiguration,
    startedAtMs: Int64,
    handle: WireGuardKitDeepRuntimeBridgeHandleControlling
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

final class UnavailableWireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionCreator:
  WireGuardKitDeepNativeSessionCreating
{
  func makeSession(
    configuration: WireGuardKitDeepNativeSessionConfiguration,
    startedAtMs: Int64
  ) throws -> WireGuardKitDeepNativeSessioning {
    throw PacketTunnelProviderError.invalidConfiguration("WireGuardKit native backend session is not integrated yet")
  }
}

final class WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionCreator:
  WireGuardKitDeepNativeSessionCreating
{
  private let handleCreator: WireGuardKitDeepRuntimeBridgeHandleCreating

  init(
    handleCreator: WireGuardKitDeepRuntimeBridgeHandleCreating =
      WireGuardKitDeepRuntimeBridgeHandleCreator()
  ) {
    self.handleCreator = handleCreator
  }

  func makeSession(
    configuration: WireGuardKitDeepNativeSessionConfiguration,
    startedAtMs: Int64
  ) throws -> WireGuardKitDeepNativeSessioning {
    let handle = try handleCreator.makeHandle(
      configuration: WireGuardKitDeepRuntimeBridgeHandleConfigurationMapper.map(configuration),
      startedAtMs: startedAtMs
    )
    return WireGuardKitDeepNativeSession(
      configuration: configuration,
      startedAtMs: startedAtMs,
      handle: handle
    )
  }
}

final class UnavailableWireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionFactory:
  WireGuardKitDeepNativeSessionFactorying
{
  private let creator: WireGuardKitDeepNativeSessionCreating

  init(
    creator: WireGuardKitDeepNativeSessionCreating =
      UnavailableWireGuardKitDeepNativeSessionCreator()
  ) {
    self.creator = creator
  }

  func makeSession(
    configuration: WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleConfiguration,
    startedAtMs: Int64
  ) throws -> WireGuardKitDeepNativeSessioning {
    try creator.makeSession(
      configuration: WireGuardKitDeepNativeSessionConfigurationMapper.map(configuration),
      startedAtMs: startedAtMs
    )
  }
}

final class WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionFactory:
  WireGuardKitDeepNativeSessionFactorying
{
  private let creator: WireGuardKitDeepNativeSessionCreating

  init(
    creator: WireGuardKitDeepNativeSessionCreating =
      WireGuardKitDeepNativeSessionCreator()
  ) {
    self.creator = creator
  }

  func makeSession(
    configuration: WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleConfiguration,
    startedAtMs: Int64
  ) throws -> WireGuardKitDeepNativeSessioning {
    try creator.makeSession(
      configuration: WireGuardKitDeepNativeSessionConfigurationMapper.map(configuration),
      startedAtMs: startedAtMs
    )
  }
}
