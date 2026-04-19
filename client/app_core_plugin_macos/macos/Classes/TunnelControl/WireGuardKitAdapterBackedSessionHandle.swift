import Foundation

struct WireGuardKitAdapterBackedSessionHandleConfiguration: Equatable {
  let backendName: String
  let frameworkPath: String?
  let moduleSource: String
  let interfaceAddress: String?
  let peerVirtualIp: String
  let selectedEndpoint: String?
}

enum WireGuardKitAdapterBackedSessionHandleConfigurationMapper {
  static func map(
    _ configuration: WireGuardKitAdapterBackedSessionConfiguration
  ) -> WireGuardKitAdapterBackedSessionHandleConfiguration {
    WireGuardKitAdapterBackedSessionHandleConfiguration(
      backendName: configuration.backendName,
      frameworkPath: configuration.frameworkPath,
      moduleSource: configuration.moduleSource,
      interfaceAddress: configuration.interfaceAddress,
      peerVirtualIp: configuration.peerVirtualIp,
      selectedEndpoint: configuration.selectedEndpoint
    )
  }
}

enum WireGuardKitAdapterBackedSessionHandleTerminalConfigurationMapper {
  static func map(
    _ configuration: WireGuardKitAdapterBackedSessionHandleConfiguration
  ) -> WireGuardKitTerminalRuntimeHandleConfiguration {
    WireGuardKitTerminalRuntimeHandleConfiguration(
      backendName: configuration.backendName,
      peerVirtualIp: configuration.peerVirtualIp,
      selectedEndpoint: configuration.selectedEndpoint
    )
  }
}

protocol WireGuardKitAdapterBackedSessionHandleControlling: AnyObject {
  func stop()
  func handleInboundPackets(_ packets: [Data], protocols: [NSNumber]) throws -> PacketTunnelEngineOutput
  func runtimeSnapshot() -> WireGuardBackendRuntimeSnapshot?
}

protocol WireGuardKitAdapterBackedSessionHandleCreating {
  func makeHandle(
    configuration: WireGuardKitAdapterBackedSessionHandleConfiguration,
    startedAtMs: Int64
  ) throws -> WireGuardKitAdapterBackedSessionHandleControlling
}

protocol WireGuardKitAdapterBackedSessionHandleFactorying {
  func makeHandle(
    configuration: WireGuardKitAdapterBackedSessionConfiguration,
    startedAtMs: Int64
  ) throws -> WireGuardKitAdapterBackedSessionHandleControlling
}

final class UnavailableWireGuardKitAdapterBackedSessionNativeHandle:
  WireGuardKitAdapterBackedSessionNativeHandleControlling
{
  func stop() {}

  func handleInboundPackets(_ packets: [Data], protocols: [NSNumber]) throws -> PacketTunnelEngineOutput {
    PacketTunnelEngineOutput(outboundPackets: [], outboundProtocols: [])
  }

  func runtimeSnapshot() -> WireGuardBackendRuntimeSnapshot? {
    nil
  }
}

final class WireGuardKitAdapterBackedSessionHandle: WireGuardKitDeepBackedRootHandleControlling {
  private let nativeHandle: WireGuardKitDeepBackedNativeHandleControlling
  private let runtimeHandle: WireGuardKitTerminalRuntimeHandleControlling
  private let synthesizedSnapshot: WireGuardBackendRuntimeSnapshot
  private var backendState = "running"

  init(
    configuration: WireGuardKitDeepBackedRootHandleConfiguration,
    startedAtMs: Int64,
    nativeHandle: WireGuardKitDeepBackedNativeHandleControlling,
    runtimeHandle: WireGuardKitTerminalRuntimeHandleControlling
  ) {
    self.nativeHandle = nativeHandle
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
    nativeHandle.stop()
    runtimeHandle.stop()
    backendState = "stopped"
  }

  func handleInboundPackets(_ packets: [Data], protocols: [NSNumber]) throws -> PacketTunnelEngineOutput {
    try nativeHandle.handleInboundPackets(packets, protocols: protocols)
  }

  func runtimeSnapshot() -> WireGuardBackendRuntimeSnapshot? {
    if let snapshot = nativeHandle.runtimeSnapshot() {
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

final class UnavailableWireGuardKitAdapterBackedSessionHandleCreator: WireGuardKitDeepBackedRootHandleCreating {
  private let runtimeHandleCreator: WireGuardKitTerminalRuntimeHandleCreating

  init(
    runtimeHandleCreator: WireGuardKitTerminalRuntimeHandleCreating =
      UnavailableWireGuardKitTerminalRuntimeHandleCreator()
  ) {
    self.runtimeHandleCreator = runtimeHandleCreator
  }

  func makeHandle(
    configuration: WireGuardKitDeepBackedRootHandleConfiguration,
    startedAtMs: Int64
  ) throws -> WireGuardKitDeepBackedRootHandleControlling {
    let runtimeHandle = try runtimeHandleCreator.makeHandle(
      configuration: WireGuardKitDeepBackedRootHandleTerminalConfigurationMapper.map(configuration),
      startedAtMs: startedAtMs
    )
    return WireGuardKitDeepBackedRootHandle(
      configuration: configuration,
      startedAtMs: startedAtMs,
      nativeHandle: UnavailableWireGuardKitDeepBackedNativeHandle(),
      runtimeHandle: runtimeHandle
    )
  }
}

final class WireGuardKitAdapterBackedSessionHandleCreator: WireGuardKitDeepBackedRootHandleCreating {
  private let nativeHandleFactory: WireGuardKitDeepBackedNativeHandleFactorying
  private let runtimeHandleCreator: WireGuardKitTerminalRuntimeHandleCreating

  init(
    nativeHandleFactory: WireGuardKitDeepBackedNativeHandleFactorying =
      WireGuardKitDeepBackedNativeHandleFactory(),
    runtimeHandleCreator: WireGuardKitTerminalRuntimeHandleCreating =
      WireGuardKitTerminalRuntimeHandleCreator()
  ) {
    self.nativeHandleFactory = nativeHandleFactory
    self.runtimeHandleCreator = runtimeHandleCreator
  }

  func makeHandle(
    configuration: WireGuardKitDeepBackedRootHandleConfiguration,
    startedAtMs: Int64
  ) throws -> WireGuardKitDeepBackedRootHandleControlling {
    let nativeHandle = try nativeHandleFactory.makeHandle(
      configuration: configuration,
      startedAtMs: startedAtMs
    )
    let runtimeHandle = try runtimeHandleCreator.makeHandle(
      configuration: WireGuardKitDeepBackedRootHandleTerminalConfigurationMapper.map(configuration),
      startedAtMs: startedAtMs
    )
    return WireGuardKitDeepBackedRootHandle(
      configuration: configuration,
      startedAtMs: startedAtMs,
      nativeHandle: nativeHandle,
      runtimeHandle: runtimeHandle
    )
  }
}

final class UnavailableWireGuardKitAdapterBackedSessionHandleFactory: WireGuardKitAdapterBackedSessionHandleFactorying {
  private let creator: WireGuardKitDeepBackedRootHandleCreating

  init(
    creator: WireGuardKitDeepBackedRootHandleCreating = UnavailableWireGuardKitDeepBackedRootHandleCreator()
  ) {
    self.creator = creator
  }

  func makeHandle(
    configuration: WireGuardKitAdapterBackedSessionConfiguration,
    startedAtMs: Int64
  ) throws -> WireGuardKitDeepBackedRootHandleControlling {
    try creator.makeHandle(
      configuration: WireGuardKitDeepBackedRootHandleConfigurationMapper.map(configuration),
      startedAtMs: startedAtMs
    )
  }
}

final class WireGuardKitAdapterBackedSessionHandleFactory: WireGuardKitDeepBackedRootHandleFactorying {
  private let creator: WireGuardKitDeepBackedRootHandleCreating

  init(
    creator: WireGuardKitDeepBackedRootHandleCreating = WireGuardKitDeepBackedRootHandleCreator()
  ) {
    self.creator = creator
  }

  func makeHandle(
    configuration: WireGuardKitAdapterBackedSessionConfiguration,
    startedAtMs: Int64
  ) throws -> WireGuardKitDeepBackedRootHandleControlling {
    try creator.makeHandle(
      configuration: WireGuardKitDeepBackedRootHandleConfigurationMapper.map(configuration),
      startedAtMs: startedAtMs
    )
  }
}
