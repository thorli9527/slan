import Foundation

enum WireGuardTunnelRuntimeViewMapper {
  static func toJson(_ runtime: WireGuardTunnelRuntimeView) -> [String: Any] {
    [
      "state": runtime.state,
      "transport": runtime.transport,
      "debugEngineMode": runtime.debugEngineMode ?? NSNull(),
      "backendName": runtime.backendName ?? NSNull(),
      "backendState": runtime.backendState ?? NSNull(),
      "backendLastError": runtime.backendLastError ?? NSNull(),
      "backendLastStartedAtMs": runtime.backendLastStartedAtMs ?? NSNull(),
      "backendPeerVirtualIp": runtime.backendPeerVirtualIp ?? NSNull(),
      "backendSelectedEndpoint": runtime.backendSelectedEndpoint ?? NSNull(),
      "peerVirtualIp": runtime.peerVirtualIp,
      "peerPublicKey": runtime.peerPublicKey,
      "selectedEndpoint": runtime.selectedEndpoint ?? NSNull(),
      "interfaceName": runtime.interfaceName ?? NSNull(),
      "dnsServers": runtime.dnsServers,
      "allowedIps": runtime.allowedIps,
      "localVirtualIp": runtime.localVirtualIp,
      "remoteAddress": runtime.remoteAddress,
      "mtu": runtime.mtu ?? NSNull(),
      "interfaceAddresses": runtime.interfaceAddresses,
      "includedRoutes": runtime.includedRoutes,
      "packetRxCount": runtime.packetRxCount,
      "packetRxBytes": runtime.packetRxBytes,
      "packetTxCount": runtime.packetTxCount,
      "packetTxBytes": runtime.packetTxBytes,
      "lastPacketAtMs": runtime.lastPacketAtMs ?? NSNull(),
      "lastAppliedAtMs": runtime.lastAppliedAtMs ?? NSNull(),
      "lastError": runtime.lastError ?? NSNull(),
    ]
  }
}
