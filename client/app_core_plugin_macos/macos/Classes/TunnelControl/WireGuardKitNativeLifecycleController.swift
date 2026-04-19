import Foundation

protocol WireGuardKitNativeBackendHandleControlling {
  func stop()
  func handleInboundPackets(_ packets: [Data], protocols: [NSNumber]) throws -> PacketTunnelEngineOutput
  func runtimeSnapshot() -> WireGuardBackendRuntimeSnapshot?
}

protocol WireGuardKitNativeBackendLifecycleControlling {
  func start(configuration: WireGuardKitNativeBackendConfiguration) throws -> WireGuardKitNativeBackendHandleControlling
}

final class WireGuardKitNativeBackendHandle: WireGuardKitNativeBackendHandleControlling {
  private let session: WireGuardKitNativeSessioning
  private let backendName: String
  private let configuration: WireGuardKitNativeBackendConfiguration
  private let startedAtMs: Int64
  private var backendState = "running"

  init(
    session: WireGuardKitNativeSessioning,
    backendName: String = "wireguardkit-native",
    configuration: WireGuardKitNativeBackendConfiguration,
    startedAtMs: Int64
  ) {
    self.session = session
    self.backendName = backendName
    self.configuration = configuration
    self.startedAtMs = startedAtMs
  }

  func stop() {
    session.stop()
    backendState = "stopped"
  }

  func handleInboundPackets(_ packets: [Data], protocols: [NSNumber]) throws -> PacketTunnelEngineOutput {
    try session.handleInboundPackets(packets, protocols: protocols)
  }

  func runtimeSnapshot() -> WireGuardBackendRuntimeSnapshot? {
    let sessionSnapshot = session.runtimeSnapshot()
    return WireGuardBackendRuntimeSnapshot(
      backendName: sessionSnapshot?.backendName ?? backendName,
      backendState: sessionSnapshot?.backendState ?? backendState,
      lastBackendError: sessionSnapshot?.lastBackendError,
      lastStartedAtMs: sessionSnapshot?.lastStartedAtMs ?? startedAtMs,
      peerVirtualIp: sessionSnapshot?.peerVirtualIp ?? configuration.peerAddress,
      selectedEndpoint: sessionSnapshot?.selectedEndpoint ?? configuration.endpoint
    )
  }
}

final class WireGuardKitNativeLifecycleController: WireGuardKitNativeBackendLifecycleControlling {
  private let adapter: WireGuardKitNativeAdapting
  private let nowMs: () -> Int64

  init(
    adapter: WireGuardKitNativeAdapting = WireGuardKitNativeAdapter(),
    nowMs: @escaping () -> Int64 = { Int64(Date().timeIntervalSince1970 * 1000) }
  ) {
    self.adapter = adapter
    self.nowMs = nowMs
  }

  func start(configuration: WireGuardKitNativeBackendConfiguration) throws -> WireGuardKitNativeBackendHandleControlling {
    let session = try adapter.makeSession(configuration: configuration)
    return WireGuardKitNativeBackendHandle(
      session: session,
      configuration: configuration,
      startedAtMs: nowMs()
    )
  }
}

final class UnavailableWireGuardKitNativeBackendLifecycleController: WireGuardKitNativeBackendLifecycleControlling {
  func start(configuration: WireGuardKitNativeBackendConfiguration) throws -> WireGuardKitNativeBackendHandleControlling {
    throw PacketTunnelProviderError.invalidConfiguration("WireGuardKit native backend session is not integrated yet")
  }
}
