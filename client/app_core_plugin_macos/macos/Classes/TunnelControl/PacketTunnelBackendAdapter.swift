import Foundation
import NetworkExtension

final class PacketTunnelBackendAdapter: WireGuardTunnelBackendInvoking {
  let hostSource = "packet-tunnel-host"
  private let lock = NSLock()
  private let packetTunnelManager: PacketTunnelManaging

  init(packetTunnelManager: PacketTunnelManaging = PacketTunnelManager()) {
    self.packetTunnelManager = packetTunnelManager
  }

  func apply(configuration: WireGuardTunnelConfiguration) throws {
    lock.lock()
    defer { lock.unlock() }

    try packetTunnelManager.apply(configuration: configuration)
  }

  func removePeer(peerVirtualIp: String) throws {
    lock.lock()
    defer { lock.unlock() }

    guard let configuration = try packetTunnelManager.currentConfiguration() else {
      return
    }
    guard configuration.peerVirtualIp == peerVirtualIp else {
      return
    }
    try packetTunnelManager.clearConfiguration()
  }

  func bringUp() throws {
    lock.lock()
    defer { lock.unlock() }

    guard try packetTunnelManager.currentConfiguration() != nil else {
      throw SlanAppCorePluginError(
        code: "app_core_tunnel_backend_unavailable",
        message: "Cannot bring up tunnel before applying configuration"
      )
    }
    try packetTunnelManager.bringUp()
  }

  func bringDown() throws {
    lock.lock()
    defer { lock.unlock() }

    try packetTunnelManager.bringDown()
  }

  func runtimeView(peerVirtualIp: String) throws -> WireGuardTunnelRuntimeView? {
    lock.lock()
    defer { lock.unlock() }

    guard let configuration = try packetTunnelManager.currentConfiguration(),
          configuration.peerVirtualIp == peerVirtualIp else {
      return nil
    }
    let runtimePayload = try? packetTunnelManager.runtimePayload()
    let fallbackState = mapStatus(try packetTunnelManager.connectionStatus())
    return WireGuardTunnelRuntimeView(
      state: runtimePayload?["state"] as? String ?? fallbackState,
      transport: configuration.transport,
      debugEngineMode: runtimePayload?["debugEngineMode"] as? String ?? configuration.debugEngineMode,
      backendName: runtimePayload?["backendName"] as? String,
      backendState: runtimePayload?["backendState"] as? String,
      backendLastError: runtimePayload?["backendLastError"] as? String,
      backendLastStartedAtMs: runtimePayload?["backendLastStartedAtMs"] as? Int64
        ?? (runtimePayload?["backendLastStartedAtMs"] as? NSNumber)?.int64Value,
      backendPeerVirtualIp: runtimePayload?["backendPeerVirtualIp"] as? String,
      backendSelectedEndpoint: runtimePayload?["backendSelectedEndpoint"] as? String,
      peerVirtualIp: configuration.peerVirtualIp,
      peerPublicKey: configuration.peer.publicKey,
      selectedEndpoint: runtimePayload?["selectedEndpoint"] as? String ?? configuration.peer.endpoint,
      interfaceName: runtimePayload?["interfaceName"] as? String ?? configuration.interface.interfaceName,
      dnsServers: runtimePayload?["dnsServers"] as? [String] ?? configuration.interface.dnsServers,
      allowedIps: runtimePayload?["allowedIps"] as? [String] ?? configuration.peer.allowedIps,
      localVirtualIp: runtimePayload?["localVirtualIp"] as? String ?? configuration.localVirtualIp,
      remoteAddress: runtimePayload?["remoteAddress"] as? String ?? configuration.peer.endpoint ?? configuration.peerVirtualIp,
      mtu: runtimePayload?["mtu"] as? Int ?? configuration.interface.mtu,
      interfaceAddresses: runtimePayload?["interfaceAddresses"] as? [String] ?? configuration.interface.addresses,
      includedRoutes: runtimePayload?["includedRoutes"] as? [String] ?? configuration.peer.allowedIps,
      packetRxCount: runtimePayload?["packetRxCount"] as? Int64
        ?? (runtimePayload?["packetRxCount"] as? NSNumber)?.int64Value
        ?? 0,
      packetRxBytes: runtimePayload?["packetRxBytes"] as? Int64
        ?? (runtimePayload?["packetRxBytes"] as? NSNumber)?.int64Value
        ?? 0,
      packetTxCount: runtimePayload?["packetTxCount"] as? Int64
        ?? (runtimePayload?["packetTxCount"] as? NSNumber)?.int64Value
        ?? 0,
      packetTxBytes: runtimePayload?["packetTxBytes"] as? Int64
        ?? (runtimePayload?["packetTxBytes"] as? NSNumber)?.int64Value
        ?? 0,
      lastPacketAtMs: runtimePayload?["lastPacketAtMs"] as? Int64
        ?? (runtimePayload?["lastPacketAtMs"] as? NSNumber)?.int64Value,
      lastAppliedAtMs: runtimePayload?["lastAppliedAtMs"] as? Int64
        ?? (runtimePayload?["lastAppliedAtMs"] as? NSNumber)?.int64Value,
      lastError: runtimePayload?["lastError"] as? String
    )
  }

  func actionSnapshot(preferredPeerVirtualIp: String?) throws -> WireGuardTunnelBackendActionSnapshot {
    lock.lock()
    defer { lock.unlock() }

    let configuration = try packetTunnelManager.currentConfiguration()
    let runtimePayload = try? packetTunnelManager.runtimePayload()
    let targetPeerVirtualIp = preferredPeerVirtualIp ?? configuration?.peerVirtualIp
    let runtimeMatchesPeer = targetPeerVirtualIp == nil ||
      (runtimePayload?["peerVirtualIp"] as? String) == targetPeerVirtualIp

    return WireGuardTunnelBackendActionSnapshot(
      connectionStatus: mapStatus(try packetTunnelManager.connectionStatus()),
      hasConfiguration: configuration != nil,
      configurationPeerVirtualIp: configuration?.peerVirtualIp,
      runtimeState: runtimeMatchesPeer ? runtimePayload?["state"] as? String : nil,
      backendState: runtimeMatchesPeer ? runtimePayload?["backendState"] as? String : nil,
      runtimeLastError: runtimeMatchesPeer
        ? (runtimePayload?["backendLastError"] as? String ?? runtimePayload?["lastError"] as? String)
        : nil
    )
  }

  private func mapStatus(_ status: NEVPNStatus) -> String {
    switch status {
    case .invalid:
      return "invalid"
    case .disconnected:
      return "disconnected"
    case .connecting, .reasserting:
      return "connecting"
    case .connected:
      return "connected"
    case .disconnecting:
      return "disconnecting"
    @unknown default:
      return "disconnected"
    }
  }
}
