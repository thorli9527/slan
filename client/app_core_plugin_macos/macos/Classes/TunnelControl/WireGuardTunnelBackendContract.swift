import Foundation
import NetworkExtension

protocol WireGuardTunnelBackendInvoking {
  func apply(configuration: WireGuardTunnelConfiguration) throws
  func removePeer(peerVirtualIp: String) throws
  func bringUp() throws
  func bringDown() throws
  func runtimeView(peerVirtualIp: String) throws -> WireGuardTunnelRuntimeView?
  func actionSnapshot(preferredPeerVirtualIp: String?) throws -> WireGuardTunnelBackendActionSnapshot
}

struct WireGuardTunnelBackendActionSnapshot {
  let connectionStatus: String
  let hasConfiguration: Bool
  let configurationPeerVirtualIp: String?
  let runtimeState: String?
  let backendState: String?
  let runtimeLastError: String?

  func toJson(
    action: String,
    accepted: Bool,
    phase: String,
    detail: String,
    source: String = "native-backend"
  ) -> [String: Any] {
    [
      "action": action,
      "accepted": accepted,
      "phase": phase,
      "source": source,
      "detail": detail,
      "connectionStatus": connectionStatus,
      "hasConfiguration": hasConfiguration,
      "configurationPeerVirtualIp": configurationPeerVirtualIp as Any,
      "runtimeState": runtimeState as Any,
      "backendState": backendState as Any,
      "runtimeLastError": runtimeLastError as Any,
    ]
  }
}

protocol PacketTunnelManaging {
  func apply(configuration: WireGuardTunnelConfiguration) throws
  func clearConfiguration() throws
  func bringUp() throws
  func bringDown() throws
  func currentConfiguration() throws -> WireGuardTunnelConfiguration?
  func runtimePayload() throws -> [String: Any]?
  func connectionStatus() throws -> NEVPNStatus
}

protocol PacketTunnelManagerRecord: AnyObject {
  var localizedDescription: String? { get set }
  var protocolConfiguration: NETunnelProviderProtocol? { get set }
  var isEnabled: Bool { get set }
  var connectionStatus: NEVPNStatus { get }

  func saveToPreferences(completion: @escaping (Error?) -> Void)
  func loadFromPreferences(completion: @escaping (Error?) -> Void)
  func startVPNTunnel() throws
  func stopVPNTunnel()
  func sendProviderMessage(_ messageData: Data, completionHandler: @escaping (Data?) -> Void) throws
}

protocol PacketTunnelManagerLoading {
  func loadAllFromPreferences(completion: @escaping ([PacketTunnelManagerRecord]?, Error?) -> Void)
  func makeManager() -> PacketTunnelManagerRecord
}

enum WireGuardTunnelBackendMethod: String {
  case applyConfiguration = "applyTunnelConfiguration"
  case removePeer = "removeTunnelPeer"
  case bringUp = "bringTunnelUp"
  case bringDown = "bringTunnelDown"
  case runtimeView = "tunnelRuntimeView"
}
