import NetworkExtension
import XCTest
@testable import TunnelControlHarness

final class PacketTunnelBackendAdapterTests: XCTestCase {
  func testBringUpFailsWithoutConfiguration() {
    let manager = FakePacketTunnelManager()
    let adapter = PacketTunnelBackendAdapter(packetTunnelManager: manager)

    XCTAssertThrowsError(try adapter.bringUp()) { error in
      let pluginError = error as? SlanAppCorePluginError
      XCTAssertEqual(pluginError?.code, "app_core_tunnel_backend_unavailable")
      XCTAssertTrue(pluginError?.message.contains("Cannot bring up tunnel") == true)
    }
  }

  func testRuntimeViewUsesRuntimePayloadWhenAvailable() throws {
    let configuration = makeConfiguration()
    let manager = FakePacketTunnelManager()
    manager.configuration = configuration
    manager.status = .connected
    manager.runtime = [
      "state": "connected",
      "debugEngineMode": "loopback",
      "backendName": "wireguardkit",
      "backendState": "started",
      "backendLastError": NSNull(),
      "backendLastStartedAtMs": NSNumber(value: 1712345677000 as Int64),
      "backendPeerVirtualIp": "100.64.0.2",
      "backendSelectedEndpoint": "198.51.100.10:51820",
      "selectedEndpoint": "198.51.100.10:51820",
      "interfaceName": "utun42",
      "dnsServers": ["9.9.9.9"],
      "allowedIps": ["10.0.0.0/24"],
      "localVirtualIp": "100.64.0.99",
      "remoteAddress": "198.51.100.10",
      "mtu": 1420,
      "interfaceAddresses": ["100.64.0.99"],
      "includedRoutes": ["10.0.0.0/255.255.255.0"],
      "packetRxCount": NSNumber(value: 4),
      "packetRxBytes": NSNumber(value: 256),
      "packetTxCount": NSNumber(value: 0),
      "packetTxBytes": NSNumber(value: 0),
      "lastPacketAtMs": NSNumber(value: 1712345678123),
      "lastAppliedAtMs": NSNumber(value: 1712345678000),
      "lastError": "none",
    ]

    let adapter = PacketTunnelBackendAdapter(packetTunnelManager: manager)
    let runtime = try adapter.runtimeView(peerVirtualIp: configuration.peerVirtualIp)

    XCTAssertEqual(runtime?.state, "connected")
    XCTAssertEqual(runtime?.transport, configuration.transport)
    XCTAssertEqual(runtime?.debugEngineMode, "loopback")
    XCTAssertEqual(runtime?.backendName, "wireguardkit")
    XCTAssertEqual(runtime?.backendState, "started")
    XCTAssertNil(runtime?.backendLastError)
    XCTAssertEqual(runtime?.backendLastStartedAtMs, 1712345677000)
    XCTAssertEqual(runtime?.backendPeerVirtualIp, "100.64.0.2")
    XCTAssertEqual(runtime?.backendSelectedEndpoint, "198.51.100.10:51820")
    XCTAssertEqual(runtime?.peerPublicKey, configuration.peer.publicKey)
    XCTAssertEqual(runtime?.selectedEndpoint, "198.51.100.10:51820")
    XCTAssertEqual(runtime?.interfaceName, "utun42")
    XCTAssertEqual(runtime?.dnsServers, ["9.9.9.9"])
    XCTAssertEqual(runtime?.allowedIps, ["10.0.0.0/24"])
    XCTAssertEqual(runtime?.localVirtualIp, "100.64.0.99")
    XCTAssertEqual(runtime?.remoteAddress, "198.51.100.10")
    XCTAssertEqual(runtime?.mtu, 1420)
    XCTAssertEqual(runtime?.interfaceAddresses, ["100.64.0.99"])
    XCTAssertEqual(runtime?.includedRoutes, ["10.0.0.0/255.255.255.0"])
    XCTAssertEqual(runtime?.packetRxCount, 4)
    XCTAssertEqual(runtime?.packetRxBytes, 256)
    XCTAssertEqual(runtime?.packetTxCount, 0)
    XCTAssertEqual(runtime?.packetTxBytes, 0)
    XCTAssertEqual(runtime?.lastPacketAtMs, 1712345678123)
    XCTAssertEqual(runtime?.lastAppliedAtMs, 1712345678000)
    XCTAssertEqual(runtime?.lastError, "none")
  }

  func testRuntimeViewFallsBackToConfigurationAndStatus() throws {
    let configuration = makeConfiguration()
    let manager = FakePacketTunnelManager()
    manager.configuration = configuration
    manager.status = .connecting

    let adapter = PacketTunnelBackendAdapter(packetTunnelManager: manager)
    let runtime = try adapter.runtimeView(peerVirtualIp: configuration.peerVirtualIp)

    XCTAssertEqual(runtime?.state, "connecting")
    XCTAssertNil(runtime?.debugEngineMode)
    XCTAssertNil(runtime?.backendName)
    XCTAssertNil(runtime?.backendState)
    XCTAssertNil(runtime?.backendLastError)
    XCTAssertNil(runtime?.backendLastStartedAtMs)
    XCTAssertNil(runtime?.backendPeerVirtualIp)
    XCTAssertNil(runtime?.backendSelectedEndpoint)
    XCTAssertEqual(runtime?.selectedEndpoint, configuration.peer.endpoint)
    XCTAssertEqual(runtime?.interfaceName, configuration.interface.interfaceName)
    XCTAssertEqual(runtime?.dnsServers, configuration.interface.dnsServers)
    XCTAssertEqual(runtime?.allowedIps, configuration.peer.allowedIps)
    XCTAssertEqual(runtime?.localVirtualIp, configuration.localVirtualIp)
    XCTAssertEqual(runtime?.remoteAddress, configuration.peer.endpoint)
    XCTAssertEqual(runtime?.mtu, configuration.interface.mtu)
    XCTAssertEqual(runtime?.interfaceAddresses, configuration.interface.addresses)
    XCTAssertEqual(runtime?.includedRoutes, configuration.peer.allowedIps)
    XCTAssertEqual(runtime?.packetRxCount, 0)
    XCTAssertEqual(runtime?.packetRxBytes, 0)
    XCTAssertEqual(runtime?.packetTxCount, 0)
    XCTAssertEqual(runtime?.packetTxBytes, 0)
    XCTAssertNil(runtime?.lastPacketAtMs)
    XCTAssertNil(runtime?.lastAppliedAtMs)
    XCTAssertNil(runtime?.lastError)
  }

  func testRuntimeViewReturnsNilForMismatchedPeer() throws {
    let manager = FakePacketTunnelManager()
    manager.configuration = makeConfiguration()
    let adapter = PacketTunnelBackendAdapter(packetTunnelManager: manager)

    let runtime = try adapter.runtimeView(peerVirtualIp: "100.64.0.88")

    XCTAssertNil(runtime)
  }

  func testRemovePeerClearsMatchingConfiguration() throws {
    let configuration = makeConfiguration()
    let manager = FakePacketTunnelManager()
    manager.configuration = configuration
    let adapter = PacketTunnelBackendAdapter(packetTunnelManager: manager)

    try adapter.removePeer(peerVirtualIp: configuration.peerVirtualIp)

    XCTAssertEqual(manager.clearConfigurationCallCount, 1)
  }

  func testRemovePeerIgnoresMismatchedConfiguration() throws {
    let manager = FakePacketTunnelManager()
    manager.configuration = makeConfiguration()
    let adapter = PacketTunnelBackendAdapter(packetTunnelManager: manager)

    try adapter.removePeer(peerVirtualIp: "100.64.0.88")

    XCTAssertEqual(manager.clearConfigurationCallCount, 0)
  }
}

private final class FakePacketTunnelManager: PacketTunnelManaging {
  var configuration: WireGuardTunnelConfiguration?
  var runtime: [String: Any]?
  var status: NEVPNStatus = .disconnected
  var clearConfigurationCallCount = 0
  var bringUpCallCount = 0
  var bringDownCallCount = 0
  var applyCallCount = 0

  func apply(configuration: WireGuardTunnelConfiguration) throws {
    applyCallCount += 1
    self.configuration = configuration
  }

  func clearConfiguration() throws {
    clearConfigurationCallCount += 1
    configuration = nil
  }

  func bringUp() throws {
    bringUpCallCount += 1
  }

  func bringDown() throws {
    bringDownCallCount += 1
  }

  func currentConfiguration() throws -> WireGuardTunnelConfiguration? {
    configuration
  }

  func runtimePayload() throws -> [String: Any]? {
    runtime
  }

  func connectionStatus() throws -> NEVPNStatus {
    status
  }
}
