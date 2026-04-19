import Foundation

struct WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleSessionBackedSessionHandleSessionConfiguration:
  Equatable
{
  let backendName: String
  let frameworkPath: String?
  let moduleSource: String
  let interfaceAddress: String?
  let peerVirtualIp: String
  let selectedEndpoint: String?
}

enum WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleSessionBackedSessionHandleSessionConfigurationMapper {
  static func map(
    _ configuration: WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleSessionBackedSessionHandleConfiguration
  ) -> WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleSessionBackedSessionHandleSessionConfiguration {
    WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleSessionBackedSessionHandleSessionConfiguration(
      backendName: configuration.backendName,
      frameworkPath: configuration.frameworkPath,
      moduleSource: configuration.moduleSource,
      interfaceAddress: configuration.interfaceAddress,
      peerVirtualIp: configuration.peerVirtualIp,
      selectedEndpoint: configuration.selectedEndpoint
    )
  }
}

protocol WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleSessionBackedSessionHandleSessioning:
  AnyObject
{
  func stop()
  func handleInboundPackets(_ packets: [Data], protocols: [NSNumber]) throws -> PacketTunnelEngineOutput
  func runtimeSnapshot() -> WireGuardBackendRuntimeSnapshot?
}

protocol WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleSessionBackedSessionHandleSessionCreating
{
  func makeSession(
    configuration:
      WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleSessionBackedSessionHandleSessionConfiguration,
    startedAtMs: Int64
  ) throws
    -> WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleSessionBackedSessionHandleSessioning
}

protocol WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleSessionBackedSessionHandleSessionFactorying
{
  func makeSession(
    configuration:
      WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleSessionBackedSessionHandleConfiguration,
    startedAtMs: Int64
  ) throws
    -> WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleSessionBackedSessionHandleSessioning
}

final class WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleSessionBackedSessionHandleSession:
  WireGuardKitDeepBackedHandleSessioning
{
  private let handle: WireGuardKitDeepLeafHandleControlling
  private let synthesizedSnapshot: WireGuardBackendRuntimeSnapshot
  private var backendState = "running"

  init(
    configuration: WireGuardKitDeepBackedHandleSessionConfiguration,
    startedAtMs: Int64,
    handle: WireGuardKitDeepLeafHandleControlling
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

final class UnavailableWireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleSessionBackedSessionHandleSessionCreator:
  WireGuardKitDeepBackedHandleSessionCreating
{
  func makeSession(
    configuration: WireGuardKitDeepBackedHandleSessionConfiguration,
    startedAtMs: Int64
  ) throws -> WireGuardKitDeepBackedHandleSessioning {
    throw PacketTunnelProviderError.invalidConfiguration("WireGuardKit native backend session is not integrated yet")
  }
}

final class WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleSessionBackedSessionHandleSessionCreator:
  WireGuardKitDeepBackedHandleSessionCreating
{
  private let handleFactory: WireGuardKitDeepLeafHandleFactorying

  init(
    handleFactory: WireGuardKitDeepLeafHandleFactorying =
      WireGuardKitDeepLeafHandleFactory()
  ) {
    self.handleFactory = handleFactory
  }

  func makeSession(
    configuration: WireGuardKitDeepBackedHandleSessionConfiguration,
    startedAtMs: Int64
  ) throws -> WireGuardKitDeepBackedHandleSessioning {
    let handle = try handleFactory.makeHandle(
      configuration: configuration,
      startedAtMs: startedAtMs
    )
    return WireGuardKitDeepBackedHandleSession(
      configuration: configuration,
      startedAtMs: startedAtMs,
      handle: handle
    )
  }
}

final class UnavailableWireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleSessionBackedSessionHandleSessionFactory:
  WireGuardKitDeepBackedHandleSessionFactorying
{
  private let creator: WireGuardKitDeepBackedHandleSessionCreating

  init(
    creator: WireGuardKitDeepBackedHandleSessionCreating =
      UnavailableWireGuardKitDeepBackedHandleSessionCreator()
  ) {
    self.creator = creator
  }

  func makeSession(
    configuration: WireGuardKitDeepBackedHandleConfiguration,
    startedAtMs: Int64
  ) throws -> WireGuardKitDeepBackedHandleSessioning {
    try creator.makeSession(
      configuration: WireGuardKitDeepBackedHandleSessionConfigurationMapper.map(configuration),
      startedAtMs: startedAtMs
    )
  }
}

final class WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleSessionBackedSessionHandleSessionFactory:
  WireGuardKitDeepBackedHandleSessionFactorying
{
  private let creator: WireGuardKitDeepBackedHandleSessionCreating

  init(
    creator: WireGuardKitDeepBackedHandleSessionCreating =
      WireGuardKitDeepBackedHandleSessionCreator()
  ) {
    self.creator = creator
  }

  func makeSession(
    configuration: WireGuardKitDeepBackedHandleConfiguration,
    startedAtMs: Int64
  ) throws -> WireGuardKitDeepBackedHandleSessioning {
    try creator.makeSession(
      configuration: WireGuardKitDeepBackedHandleSessionConfigurationMapper.map(configuration),
      startedAtMs: startedAtMs
    )
  }
}
