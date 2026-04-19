import Foundation

struct WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleSessionBackedSessionConfiguration:
  Equatable
{
  let backendName: String
  let frameworkPath: String?
  let moduleSource: String
  let interfaceAddress: String?
  let peerVirtualIp: String
  let selectedEndpoint: String?
}

enum WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleSessionBackedSessionConfigurationMapper {
  static func map(
    _ configuration: WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleSessionConfiguration
  ) -> WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleSessionBackedSessionConfiguration {
    WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleSessionBackedSessionConfiguration(
      backendName: configuration.backendName,
      frameworkPath: configuration.frameworkPath,
      moduleSource: configuration.moduleSource,
      interfaceAddress: configuration.interfaceAddress,
      peerVirtualIp: configuration.peerVirtualIp,
      selectedEndpoint: configuration.selectedEndpoint
    )
  }
}

protocol WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleSessionBackedSessioning:
  AnyObject
{
  func stop()
  func handleInboundPackets(_ packets: [Data], protocols: [NSNumber]) throws -> PacketTunnelEngineOutput
  func runtimeSnapshot() -> WireGuardBackendRuntimeSnapshot?
}

protocol WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleSessionBackedSessionCreating
{
  func makeSession(
    configuration:
      WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleSessionBackedSessionConfiguration,
    startedAtMs: Int64
  ) throws
    -> WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleSessionBackedSessioning
}

protocol WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleSessionBackedSessionFactorying
{
  func makeSession(
    configuration:
      WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleSessionConfiguration,
    startedAtMs: Int64
  ) throws
    -> WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleSessionBackedSessioning
}

final class WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleSessionBackedSession:
  WireGuardKitDeepBackedSessioning
{
  private let handle: WireGuardKitDeepBackedHandleControlling
  private let synthesizedSnapshot: WireGuardBackendRuntimeSnapshot
  private var backendState = "running"

  init(
    configuration: WireGuardKitDeepBackedSessionConfiguration,
    startedAtMs: Int64,
    handle: WireGuardKitDeepBackedHandleControlling
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

final class UnavailableWireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleSessionBackedSessionCreator:
  WireGuardKitDeepBackedSessionCreating
{
  func makeSession(
    configuration: WireGuardKitDeepBackedSessionConfiguration,
    startedAtMs: Int64
  ) throws -> WireGuardKitDeepBackedSessioning {
    throw PacketTunnelProviderError.invalidConfiguration("WireGuardKit native backend session is not integrated yet")
  }
}

final class WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleSessionBackedSessionCreator:
  WireGuardKitDeepBackedSessionCreating
{
  private let handleFactory: WireGuardKitDeepBackedHandleFactorying

  init(
    handleFactory: WireGuardKitDeepBackedHandleFactorying =
      WireGuardKitDeepBackedHandleFactory()
  ) {
    self.handleFactory = handleFactory
  }

  func makeSession(
    configuration: WireGuardKitDeepBackedSessionConfiguration,
    startedAtMs: Int64
  ) throws -> WireGuardKitDeepBackedSessioning {
    let handle = try handleFactory.makeHandle(
      configuration: configuration,
      startedAtMs: startedAtMs
    )
    return WireGuardKitDeepBackedSession(
      configuration: configuration,
      startedAtMs: startedAtMs,
      handle: handle
    )
  }
}

final class UnavailableWireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleSessionBackedSessionFactory:
  WireGuardKitDeepBackedSessionFactorying
{
  private let creator: WireGuardKitDeepBackedSessionCreating

  init(
    creator: WireGuardKitDeepBackedSessionCreating =
      UnavailableWireGuardKitDeepBackedSessionCreator()
  ) {
    self.creator = creator
  }

  func makeSession(
    configuration: WireGuardKitDeepRuntimeSessionConfiguration,
    startedAtMs: Int64
  ) throws -> WireGuardKitDeepBackedSessioning {
    try creator.makeSession(
      configuration: WireGuardKitDeepBackedSessionConfigurationMapper.map(configuration),
      startedAtMs: startedAtMs
    )
  }
}

final class WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleSessionBackedSessionFactory:
  WireGuardKitDeepBackedSessionFactorying
{
  private let creator: WireGuardKitDeepBackedSessionCreating

  init(
    creator: WireGuardKitDeepBackedSessionCreating =
      WireGuardKitDeepBackedSessionCreator()
  ) {
    self.creator = creator
  }

  func makeSession(
    configuration: WireGuardKitDeepRuntimeSessionConfiguration,
    startedAtMs: Int64
  ) throws -> WireGuardKitDeepBackedSessioning {
    try creator.makeSession(
      configuration: WireGuardKitDeepBackedSessionConfigurationMapper.map(configuration),
      startedAtMs: startedAtMs
    )
  }
}
