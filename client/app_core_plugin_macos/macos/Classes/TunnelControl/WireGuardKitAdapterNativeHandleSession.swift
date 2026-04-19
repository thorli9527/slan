import Foundation

protocol WireGuardKitAdapterNativeHandleSessioning: AnyObject {
  func stop()
  func handleInboundPackets(_ packets: [Data], protocols: [NSNumber]) throws -> PacketTunnelEngineOutput
  func runtimeSnapshot() -> WireGuardBackendRuntimeSnapshot?
}

final class WireGuardKitAdapterNativeHandleSession: WireGuardKitAdapterNativeHandleSessioning {
  private let descriptor: WireGuardKitAdapterSessionHandleDescriptor
  private let startedAtMs: Int64
  private var backendState = "running"

  init(
    descriptor: WireGuardKitAdapterSessionHandleDescriptor,
    startedAtMs: Int64
  ) {
    self.descriptor = descriptor
    self.startedAtMs = startedAtMs
  }

  func stop() {
    backendState = "stopped"
  }

  func handleInboundPackets(_ packets: [Data], protocols: [NSNumber]) throws -> PacketTunnelEngineOutput {
    .empty
  }

  func runtimeSnapshot() -> WireGuardBackendRuntimeSnapshot? {
    WireGuardBackendRuntimeSnapshot(
      backendName: descriptor.backendName,
      backendState: backendState,
      lastBackendError: nil,
      lastStartedAtMs: startedAtMs,
      peerVirtualIp: descriptor.peer.address,
      selectedEndpoint: descriptor.peer.endpoint
    )
  }
}
