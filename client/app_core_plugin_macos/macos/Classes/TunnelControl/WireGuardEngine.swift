import Foundation

struct PacketTunnelEngineOutput {
  let outboundPackets: [Data]
  let outboundProtocols: [NSNumber]

  static let empty = PacketTunnelEngineOutput(outboundPackets: [], outboundProtocols: [])
}

enum PacketTunnelDebugEngineMode: String {
  case noop
  case loopback
  case external
}

protocol WireGuardEngine {
  func start(configuration: PacketTunnelProviderConfiguration) throws
  func stop()
  func handleInboundPackets(_ packets: [Data], protocols: [NSNumber]) throws -> PacketTunnelEngineOutput
}

protocol WireGuardBackendAdapting {
  func start(configuration: PacketTunnelProviderConfiguration) throws
  func stop()
  func handleInboundPackets(_ packets: [Data], protocols: [NSNumber]) throws -> PacketTunnelEngineOutput
  func runtimeSnapshot() -> WireGuardBackendRuntimeSnapshot
}

struct WireGuardBackendRuntimeSnapshot {
  let backendName: String
  let backendState: String
  let lastBackendError: String?
  let lastStartedAtMs: Int64?
  let peerVirtualIp: String?
  let selectedEndpoint: String?
}

final class NoopWireGuardEngine: WireGuardEngine {
  func start(configuration: PacketTunnelProviderConfiguration) throws {}

  func stop() {}

  func handleInboundPackets(_ packets: [Data], protocols: [NSNumber]) throws -> PacketTunnelEngineOutput {
    .empty
  }
}

final class LoopbackWireGuardEngine: WireGuardEngine {
  func start(configuration: PacketTunnelProviderConfiguration) throws {}

  func stop() {}

  func handleInboundPackets(_ packets: [Data], protocols: [NSNumber]) throws -> PacketTunnelEngineOutput {
    PacketTunnelEngineOutput(outboundPackets: packets, outboundProtocols: protocols)
  }
}

final class ExternalWireGuardEngine: WireGuardEngine {
  private let backendAdapter: WireGuardBackendAdapting

  init(backendAdapter: WireGuardBackendAdapting) {
    self.backendAdapter = backendAdapter
  }

  func start(configuration: PacketTunnelProviderConfiguration) throws {
    try backendAdapter.start(configuration: configuration)
  }

  func stop() {
    backendAdapter.stop()
  }

  func handleInboundPackets(_ packets: [Data], protocols: [NSNumber]) throws -> PacketTunnelEngineOutput {
    try backendAdapter.handleInboundPackets(packets, protocols: protocols)
  }
}

enum WireGuardEngineFactory {
  static func make(
    from configuration: PacketTunnelProviderConfiguration,
    backendAdapter: WireGuardBackendAdapting = WireGuardKitBackendAdapter()
  ) -> WireGuardEngine {
    switch configuration.debugEngineMode {
    case .noop:
      return NoopWireGuardEngine()
    case .loopback:
      return LoopbackWireGuardEngine()
    case .external:
      return ExternalWireGuardEngine(backendAdapter: backendAdapter)
    }
  }
}
