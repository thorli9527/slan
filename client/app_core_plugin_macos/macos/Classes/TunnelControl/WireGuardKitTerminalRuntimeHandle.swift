import Foundation

struct WireGuardKitTerminalRuntimeHandleConfiguration: Equatable {
  let backendName: String
  let peerVirtualIp: String
  let selectedEndpoint: String?
}

protocol WireGuardKitTerminalRuntimeHandleControlling: AnyObject {
  func stop()
  func handleInboundPackets(_ packets: [Data], protocols: [NSNumber]) throws -> PacketTunnelEngineOutput
  func runtimeSnapshot() -> WireGuardBackendRuntimeSnapshot?
}

protocol WireGuardKitTerminalRuntimeHandleCreating {
  func makeHandle(
    configuration: WireGuardKitTerminalRuntimeHandleConfiguration,
    startedAtMs: Int64
  ) throws -> WireGuardKitTerminalRuntimeHandleControlling
}

final class WireGuardKitTerminalRuntimeHandle: WireGuardKitTerminalRuntimeHandleControlling {
  private let synthesizedSnapshot: WireGuardBackendRuntimeSnapshot
  private var backendState = "running"

  init(
    configuration: WireGuardKitTerminalRuntimeHandleConfiguration,
    startedAtMs: Int64
  ) {
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
    backendState = "stopped"
  }

  func handleInboundPackets(_ packets: [Data], protocols: [NSNumber]) throws -> PacketTunnelEngineOutput {
    .empty
  }

  func runtimeSnapshot() -> WireGuardBackendRuntimeSnapshot? {
    WireGuardBackendRuntimeSnapshot(
      backendName: synthesizedSnapshot.backendName,
      backendState: backendState,
      lastBackendError: synthesizedSnapshot.lastBackendError,
      lastStartedAtMs: synthesizedSnapshot.lastStartedAtMs,
      peerVirtualIp: synthesizedSnapshot.peerVirtualIp,
      selectedEndpoint: synthesizedSnapshot.selectedEndpoint
    )
  }
}

final class UnavailableWireGuardKitTerminalRuntimeHandleCreator: WireGuardKitTerminalRuntimeHandleCreating {
  func makeHandle(
    configuration: WireGuardKitTerminalRuntimeHandleConfiguration,
    startedAtMs: Int64
  ) throws -> WireGuardKitTerminalRuntimeHandleControlling {
    throw PacketTunnelProviderError.invalidConfiguration("WireGuardKit native backend session is not integrated yet")
  }
}

final class WireGuardKitTerminalRuntimeHandleCreator: WireGuardKitTerminalRuntimeHandleCreating {
  func makeHandle(
    configuration: WireGuardKitTerminalRuntimeHandleConfiguration,
    startedAtMs: Int64
  ) throws -> WireGuardKitTerminalRuntimeHandleControlling {
    WireGuardKitTerminalRuntimeHandle(configuration: configuration, startedAtMs: startedAtMs)
  }
}
