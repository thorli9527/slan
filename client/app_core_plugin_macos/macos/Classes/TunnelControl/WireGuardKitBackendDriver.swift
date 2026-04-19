import Foundation

protocol WireGuardKitBackendDriving {
  func start(sessionConfiguration: WireGuardKitSessionConfiguration) throws
  func stop()
  func handleInboundPackets(_ packets: [Data], protocols: [NSNumber]) throws -> PacketTunnelEngineOutput
  func runtimeSnapshot() -> WireGuardBackendRuntimeSnapshot?
}

final class WireGuardKitNativeBackendDriver: WireGuardKitBackendDriving {
  private let backendName: String
  private let sessionFactory: () -> WireGuardKitNativeBackendSessioning
  private var activeSession: WireGuardKitNativeBackendSessioning?
  private var lastSessionConfiguration: WireGuardKitSessionConfiguration?
  private var backendState = "idle"
  private var lastBackendError: String?

  init(
    backendName: String = "wireguardkit",
    sessionFactory: @escaping () -> WireGuardKitNativeBackendSessioning = { WireGuardKitNativeBackendSession() }
  ) {
    self.backendName = backendName
    self.sessionFactory = sessionFactory
  }

  func start(sessionConfiguration: WireGuardKitSessionConfiguration) throws {
    let session = sessionFactory()
    lastSessionConfiguration = sessionConfiguration
    do {
      try session.start(sessionConfiguration: sessionConfiguration)
      activeSession = session
      backendState = "running"
      lastBackendError = nil
    } catch {
      activeSession = nil
      backendState = "failed"
      lastBackendError = Self.errorMessage(from: error)
      throw error
    }
  }

  func stop() {
    activeSession?.stop()
    activeSession = nil
    backendState = "stopped"
  }

  func handleInboundPackets(_ packets: [Data], protocols: [NSNumber]) throws -> PacketTunnelEngineOutput {
    guard let activeSession else {
      return .empty
    }
    return try activeSession.handleInboundPackets(packets, protocols: protocols)
  }

  func runtimeSnapshot() -> WireGuardBackendRuntimeSnapshot? {
    let sessionSnapshot = activeSession?.runtimeSnapshot()
    guard sessionSnapshot != nil || lastSessionConfiguration != nil || lastBackendError != nil else {
      return nil
    }
    return WireGuardBackendRuntimeSnapshot(
      backendName: sessionSnapshot?.backendName ?? backendName,
      backendState: sessionSnapshot?.backendState ?? backendState,
      lastBackendError: sessionSnapshot?.lastBackendError ?? lastBackendError,
      lastStartedAtMs: sessionSnapshot?.lastStartedAtMs,
      peerVirtualIp: sessionSnapshot?.peerVirtualIp ?? lastSessionConfiguration?.peerAddress,
      selectedEndpoint: sessionSnapshot?.selectedEndpoint ?? lastSessionConfiguration?.endpoint
    )
  }

  private static func errorMessage(from error: Error) -> String {
    if let providerError = error as? PacketTunnelProviderError {
      return providerError.message
    }
    return error.localizedDescription
  }
}

final class NoopWireGuardKitBackendDriver: WireGuardKitBackendDriving {
  func start(sessionConfiguration: WireGuardKitSessionConfiguration) throws {}

  func stop() {}

  func handleInboundPackets(_ packets: [Data], protocols: [NSNumber]) throws -> PacketTunnelEngineOutput {
    .empty
  }

  func runtimeSnapshot() -> WireGuardBackendRuntimeSnapshot? {
    nil
  }
}
