import Foundation

struct WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleSessionBackedSessionHandleSessionHandleConfiguration:
  Equatable
{
  let backendName: String
  let frameworkPath: String?
  let moduleSource: String
  let interfaceAddress: String?
  let peerVirtualIp: String
  let selectedEndpoint: String?
}

enum WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleSessionBackedSessionHandleSessionHandleConfigurationMapper {
  static func map(
    _ configuration: WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleSessionBackedSessionHandleSessionConfiguration
  ) -> WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleSessionBackedSessionHandleSessionHandleConfiguration {
    WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleSessionBackedSessionHandleSessionHandleConfiguration(
      backendName: configuration.backendName,
      frameworkPath: configuration.frameworkPath,
      moduleSource: configuration.moduleSource,
      interfaceAddress: configuration.interfaceAddress,
      peerVirtualIp: configuration.peerVirtualIp,
      selectedEndpoint: configuration.selectedEndpoint
    )
  }
}

enum WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleSessionBackedSessionHandleSessionHandleTerminalConfigurationMapper {
  static func map(
    _ configuration: WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleSessionBackedSessionHandleSessionHandleConfiguration
  ) -> WireGuardKitTerminalRuntimeHandleConfiguration {
    WireGuardKitTerminalRuntimeHandleConfiguration(
      backendName: configuration.backendName,
      peerVirtualIp: configuration.peerVirtualIp,
      selectedEndpoint: configuration.selectedEndpoint
    )
  }
}

protocol WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleSessionBackedSessionHandleSessionHandleControlling:
  AnyObject
{
  func stop()
  func handleInboundPackets(_ packets: [Data], protocols: [NSNumber]) throws -> PacketTunnelEngineOutput
  func runtimeSnapshot() -> WireGuardBackendRuntimeSnapshot?
}

protocol WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleSessionBackedSessionHandleSessionHandleCreating
{
  func makeHandle(
    configuration:
      WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleSessionBackedSessionHandleSessionHandleConfiguration,
    startedAtMs: Int64
  ) throws
    -> WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleSessionBackedSessionHandleSessionHandleControlling
}

protocol WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleSessionBackedSessionHandleSessionHandleFactorying
{
  func makeHandle(
    configuration:
      WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleSessionBackedSessionHandleSessionConfiguration,
    startedAtMs: Int64
  ) throws
    -> WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleSessionBackedSessionHandleSessionHandleControlling
}

final class WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleSessionBackedSessionHandleSessionHandle:
  WireGuardKitDeepLeafHandleControlling
{
  private let runtimeHandle: WireGuardKitTerminalRuntimeHandleControlling
  private let synthesizedSnapshot: WireGuardBackendRuntimeSnapshot
  private var backendState = "running"

  init(
    configuration: WireGuardKitDeepLeafHandleConfiguration,
    startedAtMs: Int64,
    runtimeHandle: WireGuardKitTerminalRuntimeHandleControlling
  ) {
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
    runtimeHandle.stop()
    backendState = "stopped"
  }

  func handleInboundPackets(_ packets: [Data], protocols: [NSNumber]) throws -> PacketTunnelEngineOutput {
    try runtimeHandle.handleInboundPackets(packets, protocols: protocols)
  }

  func runtimeSnapshot() -> WireGuardBackendRuntimeSnapshot? {
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

final class UnavailableWireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleSessionBackedSessionHandleSessionHandleCreator:
  WireGuardKitDeepLeafHandleCreating
{
  func makeHandle(
    configuration: WireGuardKitDeepLeafHandleConfiguration,
    startedAtMs: Int64
  ) throws -> WireGuardKitDeepLeafHandleControlling {
    throw PacketTunnelProviderError.invalidConfiguration("WireGuardKit native backend session is not integrated yet")
  }
}

final class WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleSessionBackedSessionHandleSessionHandleCreator:
  WireGuardKitDeepLeafHandleCreating
{
  private let runtimeHandleCreator: WireGuardKitTerminalRuntimeHandleCreating

  init(
    runtimeHandleCreator: WireGuardKitTerminalRuntimeHandleCreating = WireGuardKitTerminalRuntimeHandleCreator()
  ) {
    self.runtimeHandleCreator = runtimeHandleCreator
  }

  func makeHandle(
    configuration: WireGuardKitDeepLeafHandleConfiguration,
    startedAtMs: Int64
  ) throws -> WireGuardKitDeepLeafHandleControlling {
    let runtimeHandle = try runtimeHandleCreator.makeHandle(
      configuration: WireGuardKitDeepLeafHandleTerminalConfigurationMapper.map(configuration),
      startedAtMs: startedAtMs
    )
    return WireGuardKitDeepLeafHandle(
      configuration: configuration,
      startedAtMs: startedAtMs,
      runtimeHandle: runtimeHandle
    )
  }
}

final class UnavailableWireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleSessionBackedSessionHandleSessionHandleFactory:
  WireGuardKitDeepLeafHandleFactorying
{
  private let creator: WireGuardKitDeepLeafHandleCreating

  init(
    creator: WireGuardKitDeepLeafHandleCreating =
      UnavailableWireGuardKitDeepLeafHandleCreator()
  ) {
    self.creator = creator
  }

  func makeHandle(
    configuration: WireGuardKitDeepBackedHandleSessionConfiguration,
    startedAtMs: Int64
  ) throws -> WireGuardKitDeepLeafHandleControlling {
    try creator.makeHandle(
      configuration: WireGuardKitDeepLeafHandleConfigurationMapper.map(configuration),
      startedAtMs: startedAtMs
    )
  }
}

final class WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleSessionBackedSessionHandleSessionHandleFactory:
  WireGuardKitDeepLeafHandleFactorying
{
  private let creator: WireGuardKitDeepLeafHandleCreating

  init(
    creator: WireGuardKitDeepLeafHandleCreating =
      WireGuardKitDeepLeafHandleCreator()
  ) {
    self.creator = creator
  }

  func makeHandle(
    configuration: WireGuardKitDeepBackedHandleSessionConfiguration,
    startedAtMs: Int64
  ) throws -> WireGuardKitDeepLeafHandleControlling {
    try creator.makeHandle(
      configuration: WireGuardKitDeepLeafHandleConfigurationMapper.map(configuration),
      startedAtMs: startedAtMs
    )
  }
}
