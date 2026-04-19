import XCTest
@testable import TunnelControlHarness

final class WireGuardTunnelRuntimeViewMapperTests: XCTestCase {
  func testSerializesRuntimeView() {
    let runtime = WireGuardTunnelRuntimeView(
      state: "connected",
      transport: "relay",
      debugEngineMode: "loopback",
      backendName: "wireguardkit",
      backendState: "started",
      backendLastError: nil,
      backendLastStartedAtMs: 1712345677000,
      backendPeerVirtualIp: "100.64.0.2",
      backendSelectedEndpoint: "203.0.113.10:51820",
      peerVirtualIp: "100.64.0.2",
      peerPublicKey: "peer-pub",
      selectedEndpoint: "203.0.113.10:51820",
      interfaceName: "utun9",
      dnsServers: ["1.1.1.1"],
      allowedIps: ["100.64.0.2/32"],
      localVirtualIp: "100.64.0.10",
      remoteAddress: "203.0.113.10",
      mtu: 1280,
      interfaceAddresses: ["100.64.0.10"],
      includedRoutes: ["100.64.0.2/255.255.255.255"],
      packetRxCount: 4,
      packetRxBytes: 256,
      packetTxCount: 0,
      packetTxBytes: 0,
      lastPacketAtMs: 1712345678123,
      lastAppliedAtMs: 1712345678000,
      lastError: nil
    )

    let json = WireGuardTunnelRuntimeViewMapper.toJson(runtime)

    XCTAssertEqual(json["state"] as? String, "connected")
    XCTAssertEqual(json["transport"] as? String, "relay")
    XCTAssertEqual(json["debugEngineMode"] as? String, "loopback")
    XCTAssertEqual(json["backendName"] as? String, "wireguardkit")
    XCTAssertEqual(json["backendState"] as? String, "started")
    XCTAssertTrue(json["backendLastError"] is NSNull)
    XCTAssertEqual(json["backendLastStartedAtMs"] as? Int64, 1712345677000)
    XCTAssertEqual(json["backendPeerVirtualIp"] as? String, "100.64.0.2")
    XCTAssertEqual(json["backendSelectedEndpoint"] as? String, "203.0.113.10:51820")
    XCTAssertEqual(json["peerVirtualIp"] as? String, "100.64.0.2")
    XCTAssertEqual(json["peerPublicKey"] as? String, "peer-pub")
    XCTAssertEqual(json["selectedEndpoint"] as? String, "203.0.113.10:51820")
    XCTAssertEqual(json["interfaceName"] as? String, "utun9")
    XCTAssertEqual(json["dnsServers"] as? [String], ["1.1.1.1"])
    XCTAssertEqual(json["allowedIps"] as? [String], ["100.64.0.2/32"])
    XCTAssertEqual(json["localVirtualIp"] as? String, "100.64.0.10")
    XCTAssertEqual(json["remoteAddress"] as? String, "203.0.113.10")
    XCTAssertEqual(json["mtu"] as? Int, 1280)
    XCTAssertEqual(json["interfaceAddresses"] as? [String], ["100.64.0.10"])
    XCTAssertEqual(json["includedRoutes"] as? [String], ["100.64.0.2/255.255.255.255"])
    XCTAssertEqual(json["packetRxCount"] as? Int64, 4)
    XCTAssertEqual(json["packetRxBytes"] as? Int64, 256)
    XCTAssertEqual(json["packetTxCount"] as? Int64, 0)
    XCTAssertEqual(json["packetTxBytes"] as? Int64, 0)
    XCTAssertEqual(json["lastPacketAtMs"] as? Int64, 1712345678123)
    XCTAssertEqual(json["lastAppliedAtMs"] as? Int64, 1712345678000)
    XCTAssertTrue(json["lastError"] is NSNull)
  }

  func testSerializesRuntimeViewWithLastError() {
    let runtime = WireGuardTunnelRuntimeView(
      state: "failed",
      transport: "relay",
      debugEngineMode: nil,
      backendName: nil,
      backendState: nil,
      backendLastError: nil,
      backendLastStartedAtMs: nil,
      backendPeerVirtualIp: nil,
      backendSelectedEndpoint: nil,
      peerVirtualIp: "100.64.0.2",
      peerPublicKey: "peer-pub",
      selectedEndpoint: nil,
      interfaceName: nil,
      dnsServers: [],
      allowedIps: [],
      localVirtualIp: "100.64.0.10",
      remoteAddress: "100.64.0.2",
      mtu: nil,
      interfaceAddresses: [],
      includedRoutes: [],
      packetRxCount: 0,
      packetRxBytes: 0,
      packetTxCount: 0,
      packetTxBytes: 0,
      lastPacketAtMs: nil,
      lastAppliedAtMs: nil,
      lastError: "setTunnelNetworkSettings failed"
    )

    let json = WireGuardTunnelRuntimeViewMapper.toJson(runtime)

    XCTAssertEqual(json["state"] as? String, "failed")
    XCTAssertTrue(json["debugEngineMode"] is NSNull)
    XCTAssertTrue(json["backendName"] is NSNull)
    XCTAssertTrue(json["backendState"] is NSNull)
    XCTAssertTrue(json["backendLastError"] is NSNull)
    XCTAssertTrue(json["backendLastStartedAtMs"] is NSNull)
    XCTAssertTrue(json["backendPeerVirtualIp"] is NSNull)
    XCTAssertTrue(json["backendSelectedEndpoint"] is NSNull)
    XCTAssertEqual(json["packetRxCount"] as? Int64, 0)
    XCTAssertEqual(json["packetRxBytes"] as? Int64, 0)
    XCTAssertEqual(json["packetTxCount"] as? Int64, 0)
    XCTAssertEqual(json["packetTxBytes"] as? Int64, 0)
    XCTAssertTrue(json["lastPacketAtMs"] is NSNull)
    XCTAssertEqual(json["lastError"] as? String, "setTunnelNetworkSettings failed")
    XCTAssertTrue(json["lastAppliedAtMs"] is NSNull)
  }
}
