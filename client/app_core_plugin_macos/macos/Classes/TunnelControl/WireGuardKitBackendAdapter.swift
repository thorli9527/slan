import Foundation

final class WireGuardKitBackendAdapter: WireGuardBackendAdapting {
  private let session: WireGuardKitBackendSessioning
  private let nowMs: () -> Int64
  private var lastStartConfiguration: WireGuardKitBackendConfiguration?
  private var backendState = "idle"
  private var lastBackendError: String?
  private var lastStartedAtMs: Int64?

  init(
    session: WireGuardKitBackendSessioning = WireGuardKitBackendSession(),
    nowMs: @escaping () -> Int64 = { Int64(Date().timeIntervalSince1970 * 1000) }
  ) {
    self.session = session
    self.nowMs = nowMs
  }

  func start(configuration: PacketTunnelProviderConfiguration) throws {
    let backendConfiguration = WireGuardKitBackendConfigMapper.map(configuration)
    lastStartConfiguration = backendConfiguration
    lastStartedAtMs = nowMs()
    do {
      try session.start(configuration: backendConfiguration)
      backendState = "started"
      lastBackendError = nil
    } catch {
      backendState = "failed"
      lastBackendError = Self.errorMessage(from: error)
      throw PacketTunnelProviderError.invalidConfiguration(lastBackendError!)
    }
  }

  func stop() {
    session.stop()
    backendState = "stopped"
  }

  func handleInboundPackets(_ packets: [Data], protocols: [NSNumber]) throws -> PacketTunnelEngineOutput {
    try session.handleInboundPackets(packets, protocols: protocols)
  }

  func runtimeSnapshot() -> WireGuardBackendRuntimeSnapshot {
    let sessionSnapshot = session.runtimeSnapshot()
    return WireGuardBackendRuntimeSnapshot(
      backendName: sessionSnapshot?.backendName ?? "wireguardkit",
      backendState: sessionSnapshot?.backendState ?? backendState,
      lastBackendError: sessionSnapshot?.lastBackendError ?? lastBackendError,
      lastStartedAtMs: sessionSnapshot?.lastStartedAtMs ?? lastStartedAtMs,
      peerVirtualIp: sessionSnapshot?.peerVirtualIp ?? lastStartConfiguration?.peerVirtualIp,
      selectedEndpoint: sessionSnapshot?.selectedEndpoint ?? lastStartConfiguration?.endpoint
    )
  }

  private static func errorMessage(from error: Error) -> String {
    if let providerError = error as? PacketTunnelProviderError {
      return providerError.message
    }
    return error.localizedDescription
  }
}
