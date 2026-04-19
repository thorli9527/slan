import Foundation

struct WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleConfiguration: Equatable {
  let backendName: String
  let frameworkPath: String?
  let moduleSource: String
  let interfaceAddress: String?
  let peerVirtualIp: String
  let selectedEndpoint: String?
}

enum WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleConfigurationMapper {
  static func map(
    _ configuration: WireGuardKitAdapterBackedSessionAdapterHandleSessionConfiguration
  ) -> WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleConfiguration {
    WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleConfiguration(
      backendName: configuration.backendName,
      frameworkPath: configuration.frameworkPath,
      moduleSource: configuration.moduleSource,
      interfaceAddress: configuration.interfaceAddress,
      peerVirtualIp: configuration.peerVirtualIp,
      selectedEndpoint: configuration.selectedEndpoint
    )
  }
}

protocol WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleControlling: AnyObject {
  func stop()
  func handleInboundPackets(_ packets: [Data], protocols: [NSNumber]) throws -> PacketTunnelEngineOutput
  func runtimeSnapshot() -> WireGuardBackendRuntimeSnapshot?
}

protocol WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleCreating {
  func makeHandle(
    configuration: WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleConfiguration,
    startedAtMs: Int64
  ) throws -> WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleControlling
}

protocol WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleFactorying {
  func makeHandle(
    configuration: WireGuardKitAdapterBackedSessionAdapterHandleSessionConfiguration,
    startedAtMs: Int64
  ) throws -> WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleControlling
}

final class WireGuardKitAdapterBackedSessionAdapterHandleNativeHandle:
  WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleControlling
{
  private let session: WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessioning
  private let synthesizedSnapshot: WireGuardBackendRuntimeSnapshot
  private var backendState = "running"

  init(
    configuration: WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleConfiguration,
    startedAtMs: Int64,
    session: WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessioning
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

final class UnavailableWireGuardKitAdapterBackedSessionAdapterHandleNativeHandleCreator:
  WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleCreating
{
  func makeHandle(
    configuration: WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleConfiguration,
    startedAtMs: Int64
  ) throws -> WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleControlling {
    throw PacketTunnelProviderError.invalidConfiguration("WireGuardKit native backend session is not integrated yet")
  }
}

final class WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleCreator:
  WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleCreating
{
  private let sessionFactory: WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionFactorying

  init(
    sessionFactory: WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionFactorying =
      WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionFactory()
  ) {
    self.sessionFactory = sessionFactory
  }

  func makeHandle(
    configuration: WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleConfiguration,
    startedAtMs: Int64
  ) throws -> WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleControlling {
    let session = try sessionFactory.makeSession(
      configuration: configuration,
      startedAtMs: startedAtMs
    )
    return WireGuardKitAdapterBackedSessionAdapterHandleNativeHandle(
      configuration: configuration,
      startedAtMs: startedAtMs,
      session: session
    )
  }
}

final class UnavailableWireGuardKitAdapterBackedSessionAdapterHandleNativeHandleFactory:
  WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleFactorying
{
  private let creator: WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleCreating

  init(
    creator: WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleCreating =
      UnavailableWireGuardKitAdapterBackedSessionAdapterHandleNativeHandleCreator()
  ) {
    self.creator = creator
  }

  func makeHandle(
    configuration: WireGuardKitAdapterBackedSessionAdapterHandleSessionConfiguration,
    startedAtMs: Int64
  ) throws -> WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleControlling {
    try creator.makeHandle(
      configuration: WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleConfigurationMapper.map(configuration),
      startedAtMs: startedAtMs
    )
  }
}

final class WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleFactory:
  WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleFactorying
{
  private let creator: WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleCreating

  init(
    creator: WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleCreating =
      WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleCreator()
  ) {
    self.creator = creator
  }

  func makeHandle(
    configuration: WireGuardKitAdapterBackedSessionAdapterHandleSessionConfiguration,
    startedAtMs: Int64
  ) throws -> WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleControlling {
    try creator.makeHandle(
      configuration: WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleConfigurationMapper.map(configuration),
      startedAtMs: startedAtMs
    )
  }
}
