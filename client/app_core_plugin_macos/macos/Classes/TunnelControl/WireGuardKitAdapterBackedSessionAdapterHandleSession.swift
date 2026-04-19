import Foundation

struct WireGuardKitAdapterBackedSessionAdapterHandleSessionConfiguration: Equatable {
  let backendName: String
  let frameworkPath: String?
  let moduleSource: String
  let interfaceAddress: String?
  let peerVirtualIp: String
  let selectedEndpoint: String?
}

enum WireGuardKitAdapterBackedSessionAdapterHandleSessionConfigurationMapper {
  static func map(
    _ configuration: WireGuardKitAdapterBackedSessionAdapterHandleConfiguration
  ) -> WireGuardKitAdapterBackedSessionAdapterHandleSessionConfiguration {
    WireGuardKitAdapterBackedSessionAdapterHandleSessionConfiguration(
      backendName: configuration.backendName,
      frameworkPath: configuration.frameworkPath,
      moduleSource: configuration.moduleSource,
      interfaceAddress: configuration.interfaceAddress,
      peerVirtualIp: configuration.peerVirtualIp,
      selectedEndpoint: configuration.selectedEndpoint
    )
  }
}

protocol WireGuardKitAdapterBackedSessionAdapterHandleSessioning: AnyObject {
  func stop()
  func handleInboundPackets(_ packets: [Data], protocols: [NSNumber]) throws -> PacketTunnelEngineOutput
  func runtimeSnapshot() -> WireGuardBackendRuntimeSnapshot?
}

protocol WireGuardKitAdapterBackedSessionAdapterHandleSessionCreating {
  func makeSession(
    configuration: WireGuardKitAdapterBackedSessionAdapterHandleSessionConfiguration,
    startedAtMs: Int64
  ) throws -> WireGuardKitAdapterBackedSessionAdapterHandleSessioning
}

protocol WireGuardKitAdapterBackedSessionAdapterHandleSessionFactorying {
  func makeSession(
    configuration: WireGuardKitAdapterBackedSessionAdapterHandleConfiguration,
    startedAtMs: Int64
  ) throws -> WireGuardKitAdapterBackedSessionAdapterHandleSessioning
}

final class WireGuardKitAdapterBackedSessionAdapterHandleSession:
  WireGuardKitDeepAdapterSessioning
{
  private let nativeHandle: WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleControlling
  private let synthesizedSnapshot: WireGuardBackendRuntimeSnapshot
  private var backendState = "running"

  init(
    configuration: WireGuardKitDeepAdapterSessionConfiguration,
    startedAtMs: Int64,
    nativeHandle: WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleControlling
  ) {
    self.nativeHandle = nativeHandle
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
    nativeHandle.stop()
    backendState = "stopped"
  }

  func handleInboundPackets(_ packets: [Data], protocols: [NSNumber]) throws -> PacketTunnelEngineOutput {
    try nativeHandle.handleInboundPackets(packets, protocols: protocols)
  }

  func runtimeSnapshot() -> WireGuardBackendRuntimeSnapshot? {
    if let snapshot = nativeHandle.runtimeSnapshot() {
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

final class UnavailableWireGuardKitAdapterBackedSessionAdapterHandleSessionCreator:
  WireGuardKitDeepAdapterSessionCreating
{
  func makeSession(
    configuration: WireGuardKitDeepAdapterSessionConfiguration,
    startedAtMs: Int64
  ) throws -> WireGuardKitDeepAdapterSessioning {
    throw PacketTunnelProviderError.invalidConfiguration("WireGuardKit native backend session is not integrated yet")
  }
}

final class WireGuardKitAdapterBackedSessionAdapterHandleSessionCreator:
  WireGuardKitDeepAdapterSessionCreating
{
  private let nativeHandleFactory: WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleFactorying

  init(
    nativeHandleFactory: WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleFactorying =
      WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleFactory()
  ) {
    self.nativeHandleFactory = nativeHandleFactory
  }

  func makeSession(
    configuration: WireGuardKitDeepAdapterSessionConfiguration,
    startedAtMs: Int64
  ) throws -> WireGuardKitDeepAdapterSessioning {
    let nativeHandle = try nativeHandleFactory.makeHandle(
      configuration: configuration,
      startedAtMs: startedAtMs
    )
    return WireGuardKitDeepAdapterSession(
      configuration: configuration,
      startedAtMs: startedAtMs,
      nativeHandle: nativeHandle
    )
  }
}

final class UnavailableWireGuardKitAdapterBackedSessionAdapterHandleSessionFactory:
  WireGuardKitDeepAdapterSessionFactorying
{
  private let creator: WireGuardKitDeepAdapterSessionCreating

  init(
    creator: WireGuardKitDeepAdapterSessionCreating =
      UnavailableWireGuardKitDeepAdapterSessionCreator()
  ) {
    self.creator = creator
  }

  func makeSession(
    configuration: WireGuardKitAdapterBackedSessionAdapterHandleConfiguration,
    startedAtMs: Int64
  ) throws -> WireGuardKitDeepAdapterSessioning {
    try creator.makeSession(
      configuration: WireGuardKitDeepAdapterSessionConfigurationMapper.map(configuration),
      startedAtMs: startedAtMs
    )
  }
}

final class WireGuardKitAdapterBackedSessionAdapterHandleSessionFactory:
  WireGuardKitDeepAdapterSessionFactorying
{
  private let creator: WireGuardKitDeepAdapterSessionCreating

  init(
    creator: WireGuardKitDeepAdapterSessionCreating =
      WireGuardKitDeepAdapterSessionCreator()
  ) {
    self.creator = creator
  }

  func makeSession(
    configuration: WireGuardKitAdapterBackedSessionAdapterHandleConfiguration,
    startedAtMs: Int64
  ) throws -> WireGuardKitDeepAdapterSessioning {
    try creator.makeSession(
      configuration: WireGuardKitDeepAdapterSessionConfigurationMapper.map(configuration),
      startedAtMs: startedAtMs
    )
  }
}
