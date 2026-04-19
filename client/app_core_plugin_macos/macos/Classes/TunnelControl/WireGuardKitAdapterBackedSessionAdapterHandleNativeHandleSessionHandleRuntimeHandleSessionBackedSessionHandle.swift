import Foundation

struct WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleSessionBackedSessionHandleConfiguration:
  Equatable
{
  let backendName: String
  let frameworkPath: String?
  let moduleSource: String
  let interfaceAddress: String?
  let peerVirtualIp: String
  let selectedEndpoint: String?
}

enum WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleSessionBackedSessionHandleConfigurationMapper {
  static func map(
    _ configuration: WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleSessionBackedSessionConfiguration
  ) -> WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleSessionBackedSessionHandleConfiguration {
    WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleSessionBackedSessionHandleConfiguration(
      backendName: configuration.backendName,
      frameworkPath: configuration.frameworkPath,
      moduleSource: configuration.moduleSource,
      interfaceAddress: configuration.interfaceAddress,
      peerVirtualIp: configuration.peerVirtualIp,
      selectedEndpoint: configuration.selectedEndpoint
    )
  }
}

protocol WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleSessionBackedSessionHandleControlling:
  AnyObject
{
  func stop()
  func handleInboundPackets(_ packets: [Data], protocols: [NSNumber]) throws -> PacketTunnelEngineOutput
  func runtimeSnapshot() -> WireGuardBackendRuntimeSnapshot?
}

protocol WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleSessionBackedSessionHandleCreating {
  func makeHandle(
    configuration:
      WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleSessionBackedSessionHandleConfiguration,
    startedAtMs: Int64
  ) throws
    -> WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleSessionBackedSessionHandleControlling
}

protocol WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleSessionBackedSessionHandleFactorying
{
  func makeHandle(
    configuration:
      WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleSessionBackedSessionConfiguration,
    startedAtMs: Int64
  ) throws
    -> WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleSessionBackedSessionHandleControlling
}

final class WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleSessionBackedSessionHandle:
  WireGuardKitDeepBackedHandleControlling
{
  private let session: WireGuardKitDeepBackedHandleSessioning
  private let synthesizedSnapshot: WireGuardBackendRuntimeSnapshot
  private var backendState = "running"

  init(
    configuration: WireGuardKitDeepBackedHandleConfiguration,
    startedAtMs: Int64,
    session: WireGuardKitDeepBackedHandleSessioning
  ) {
    self.session = session
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
    backendState = "stopped"
  }

  func handleInboundPackets(_ packets: [Data], protocols: [NSNumber]) throws -> PacketTunnelEngineOutput {
    try session.handleInboundPackets(packets, protocols: protocols)
  }

  func runtimeSnapshot() -> WireGuardBackendRuntimeSnapshot? {
    if let snapshot = session.runtimeSnapshot() {
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

final class UnavailableWireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleSessionBackedSessionHandleCreator:
  WireGuardKitDeepBackedHandleCreating
{
  func makeHandle(
    configuration: WireGuardKitDeepBackedHandleConfiguration,
    startedAtMs: Int64
  ) throws -> WireGuardKitDeepBackedHandleControlling {
    throw PacketTunnelProviderError.invalidConfiguration("WireGuardKit native backend session is not integrated yet")
  }
}

final class WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleSessionBackedSessionHandleCreator:
  WireGuardKitDeepBackedHandleCreating
{
  private let sessionFactory: WireGuardKitDeepBackedHandleSessionFactorying

  init(
    sessionFactory: WireGuardKitDeepBackedHandleSessionFactorying =
      WireGuardKitDeepBackedHandleSessionFactory()
  ) {
    self.sessionFactory = sessionFactory
  }

  func makeHandle(
    configuration: WireGuardKitDeepBackedHandleConfiguration,
    startedAtMs: Int64
  ) throws -> WireGuardKitDeepBackedHandleControlling {
    let session = try sessionFactory.makeSession(
      configuration: configuration,
      startedAtMs: startedAtMs
    )
    return WireGuardKitDeepBackedHandle(
      configuration: configuration,
      startedAtMs: startedAtMs,
      session: session
    )
  }
}

final class UnavailableWireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleSessionBackedSessionHandleFactory:
  WireGuardKitDeepBackedHandleFactorying
{
  private let creator: WireGuardKitDeepBackedHandleCreating

  init(
    creator: WireGuardKitDeepBackedHandleCreating =
      UnavailableWireGuardKitDeepBackedHandleCreator()
  ) {
    self.creator = creator
  }

  func makeHandle(
    configuration: WireGuardKitDeepBackedSessionConfiguration,
    startedAtMs: Int64
  ) throws -> WireGuardKitDeepBackedHandleControlling {
    try creator.makeHandle(
      configuration: WireGuardKitDeepBackedHandleConfigurationMapper.map(configuration),
      startedAtMs: startedAtMs
    )
  }
}

final class WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleSessionBackedSessionHandleFactory:
  WireGuardKitDeepBackedHandleFactorying
{
  private let creator: WireGuardKitDeepBackedHandleCreating

  init(
    creator: WireGuardKitDeepBackedHandleCreating =
      WireGuardKitDeepBackedHandleCreator()
  ) {
    self.creator = creator
  }

  func makeHandle(
    configuration: WireGuardKitDeepBackedSessionConfiguration,
    startedAtMs: Int64
  ) throws -> WireGuardKitDeepBackedHandleControlling {
    try creator.makeHandle(
      configuration: WireGuardKitDeepBackedHandleConfigurationMapper.map(configuration),
      startedAtMs: startedAtMs
    )
  }
}
