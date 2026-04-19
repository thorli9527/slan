import NetworkExtension
import XCTest
@testable import TunnelControlHarness

final class PacketTunnelProviderSupportTests: XCTestCase {
  func testNoopWireGuardEngineProducesNoOutboundPackets() throws {
    let configuration = try PacketTunnelProviderConfiguration(providerConfiguration: makeProviderConfiguration())
    let engine = NoopWireGuardEngine()

    try engine.start(configuration: configuration)
    let output = try engine.handleInboundPackets([Data([0x01, 0x02])], protocols: [NSNumber(value: AF_INET)])
    engine.stop()

    XCTAssertTrue(output.outboundPackets.isEmpty)
    XCTAssertTrue(output.outboundProtocols.isEmpty)
  }

  func testLoopbackWireGuardEngineEchoesInboundPackets() throws {
    var providerConfiguration = makeProviderConfiguration()
    providerConfiguration["debugEngineMode"] = "loopback"
    let configuration = try PacketTunnelProviderConfiguration(providerConfiguration: providerConfiguration)
    let engine = PacketTunnelProviderSupport.makeWireGuardEngine(from: configuration)
    let packet = Data([0xde, 0xad, 0xbe, 0xef])
    let proto = NSNumber(value: AF_INET)

    try engine.start(configuration: configuration)
    let output = try engine.handleInboundPackets([packet], protocols: [proto])
    engine.stop()

    XCTAssertEqual(output.outboundPackets, [packet])
    XCTAssertEqual(output.outboundProtocols, [proto])
  }

  func testExternalWireGuardEngineFailsToStart() throws {
    var providerConfiguration = makeProviderConfiguration()
    providerConfiguration["debugEngineMode"] = "external"
    let configuration = try PacketTunnelProviderConfiguration(providerConfiguration: providerConfiguration)
    let engine = PacketTunnelProviderSupport.makeWireGuardEngine(from: configuration)

    XCTAssertThrowsError(try engine.start(configuration: configuration)) { error in
      let providerError = error as? PacketTunnelProviderError
      XCTAssertEqual(providerError?.message, "WireGuardKit framework is not linked into the app bundle")
    }
  }

  func testExternalWireGuardEngineDelegatesToBackendAdapter() throws {
    var providerConfiguration = makeProviderConfiguration()
    providerConfiguration["debugEngineMode"] = "external"
    let configuration = try PacketTunnelProviderConfiguration(providerConfiguration: providerConfiguration)
    let backendAdapter = FakeWireGuardBackendAdapter()
    let engine = PacketTunnelProviderSupport.makeWireGuardEngine(
      from: configuration,
      backendAdapter: backendAdapter
    )
    let packet = Data([0xaa, 0xbb])
    let proto = NSNumber(value: AF_INET)

    try engine.start(configuration: configuration)
    let output = try engine.handleInboundPackets([packet], protocols: [proto])
    engine.stop()

    XCTAssertEqual(backendAdapter.startConfiguration?.peerVirtualIp, "100.64.0.2")
    XCTAssertEqual(backendAdapter.inboundPackets, [packet])
    XCTAssertEqual(backendAdapter.inboundProtocols, [proto])
    XCTAssertEqual(output.outboundPackets, [Data([0xcc])])
    XCTAssertEqual(output.outboundProtocols, [NSNumber(value: AF_INET6)])
    XCTAssertTrue(backendAdapter.stopCalled)
  }

  func testParsesProviderConfiguration() throws {
    let configuration = try PacketTunnelProviderConfiguration(providerConfiguration: makeProviderConfiguration())

    XCTAssertEqual(configuration.localVirtualIp, "100.64.0.10")
    XCTAssertEqual(configuration.peerVirtualIp, "100.64.0.2")
    XCTAssertEqual(configuration.remoteAddress, "203.0.113.10")
    XCTAssertEqual(configuration.selectedEndpoint, "203.0.113.10:51820")
    XCTAssertEqual(configuration.debugEngineMode, .noop)
    XCTAssertEqual(configuration.dnsServers, ["1.1.1.1", "8.8.8.8"])
    XCTAssertEqual(configuration.mtu, 1280)
    XCTAssertEqual(configuration.interfaceAddress.address, "100.64.0.10")
    XCTAssertEqual(configuration.interfaceAddress.subnetMask, "255.255.255.255")
    XCTAssertEqual(configuration.allowedIps, ["100.64.0.2/32", "10.0.0.0/24"])
    XCTAssertEqual(configuration.allowedIPv4Routes.count, 2)
    XCTAssertEqual(configuration.allowedIPv4Routes[0].destinationAddress, "100.64.0.2")
    XCTAssertEqual(configuration.allowedIPv4Routes[0].destinationSubnetMask, "255.255.255.255")
  }

  func testRejectsInvalidCidr() {
    var providerConfiguration = makeProviderConfiguration()
    providerConfiguration["wireguardPeer"] = [
      "publicKey": "peer-pub",
      "endpoint": "203.0.113.10:51820",
      "allowedIps": [
        ["cidr": "100.64.0.2/99"],
      ],
    ]

    XCTAssertThrowsError(try PacketTunnelProviderConfiguration(providerConfiguration: providerConfiguration)) { error in
      let providerError = error as? PacketTunnelProviderError
      XCTAssertTrue(providerError?.message.contains("invalid cidr prefix") == true)
    }
  }

  func testBuildsNetworkSettings() throws {
    let configuration = try PacketTunnelProviderConfiguration(providerConfiguration: makeProviderConfiguration())

    let settings = try PacketTunnelProviderSupport.makeNetworkSettings(from: configuration)

    XCTAssertEqual(settings.tunnelRemoteAddress, "203.0.113.10")
    XCTAssertEqual(settings.ipv4Settings?.addresses, ["100.64.0.10"])
    XCTAssertEqual(settings.ipv4Settings?.subnetMasks, ["255.255.255.255"])
    XCTAssertEqual(settings.ipv4Settings?.includedRoutes?.count, 2)
    XCTAssertEqual(settings.dnsSettings?.servers, ["1.1.1.1", "8.8.8.8"])
    XCTAssertEqual(settings.mtu?.intValue, 1280)
  }

  func testBuildsDefaultRouteWhenAllowedIpsEmpty() throws {
    var providerConfiguration = makeProviderConfiguration()
    providerConfiguration["wireguardPeer"] = [
      "publicKey": "peer-pub",
      "endpoint": "203.0.113.10:51820",
      "allowedIps": [],
    ]
    let configuration = try PacketTunnelProviderConfiguration(providerConfiguration: providerConfiguration)

    let settings = try PacketTunnelProviderSupport.makeNetworkSettings(from: configuration)

    XCTAssertEqual(settings.ipv4Settings?.includedRoutes?.count, 1)
    XCTAssertEqual(settings.ipv4Settings?.includedRoutes?.first?.destinationAddress, "0.0.0.0")
    XCTAssertEqual(settings.ipv4Settings?.includedRoutes?.first?.destinationSubnetMask, "0.0.0.0")
  }

  func testBuildsRuntimeViewFromSettings() throws {
    let configuration = try PacketTunnelProviderConfiguration(providerConfiguration: makeProviderConfiguration())
    let settings = try PacketTunnelProviderSupport.makeNetworkSettings(from: configuration)

    let runtime = PacketTunnelProviderSupport.makeRuntimeView(
      configuration: configuration,
      settings: settings,
      state: "connected",
      packetRxCount: 4,
      packetRxBytes: 256,
      packetTxCount: 0,
      packetTxBytes: 0,
      lastPacketAtMs: 1712345678123,
      lastAppliedAtMs: 1712345678000,
      lastError: nil
    )

    XCTAssertEqual(runtime.state, "connected")
    XCTAssertEqual(runtime.debugEngineMode, "noop")
    XCTAssertNil(runtime.backendName)
    XCTAssertNil(runtime.backendState)
    XCTAssertNil(runtime.backendLastError)
    XCTAssertNil(runtime.backendLastStartedAtMs)
    XCTAssertNil(runtime.backendPeerVirtualIp)
    XCTAssertNil(runtime.backendSelectedEndpoint)
    XCTAssertEqual(runtime.peerVirtualIp, "100.64.0.2")
    XCTAssertEqual(runtime.selectedEndpoint, "203.0.113.10:51820")
    XCTAssertEqual(runtime.interfaceName, "utun")
    XCTAssertEqual(runtime.dnsServers, ["1.1.1.1", "8.8.8.8"])
    XCTAssertEqual(runtime.allowedIps, ["100.64.0.2/32", "10.0.0.0/24"])
    XCTAssertEqual(runtime.localVirtualIp, "100.64.0.10")
    XCTAssertEqual(runtime.remoteAddress, "203.0.113.10")
    XCTAssertEqual(runtime.mtu, 1280)
    XCTAssertEqual(runtime.interfaceAddresses, ["100.64.0.10"])
    XCTAssertEqual(runtime.includedRoutes, ["100.64.0.2/255.255.255.255", "10.0.0.0/255.255.255.0"])
    XCTAssertEqual(runtime.packetRxCount, 4)
    XCTAssertEqual(runtime.packetRxBytes, 256)
    XCTAssertEqual(runtime.packetTxCount, 0)
    XCTAssertEqual(runtime.packetTxBytes, 0)
    XCTAssertEqual(runtime.lastPacketAtMs, 1712345678123)
    XCTAssertEqual(runtime.lastAppliedAtMs, 1712345678000)
    XCTAssertNil(runtime.lastError)
  }

  func testRuntimeViewJsonUsesNullForMissingOptionals() throws {
    let configuration = try PacketTunnelProviderConfiguration(providerConfiguration: makeProviderConfiguration())

    let runtime = PacketTunnelProviderSupport.makeRuntimeView(
      configuration: configuration,
      settings: nil,
      state: "failed",
      lastAppliedAtMs: nil,
      lastError: "setTunnelNetworkSettings failed"
    )
    let json = runtime.toJson()

    XCTAssertEqual(json["state"] as? String, "failed")
    XCTAssertEqual(json["debugEngineMode"] as? String, "noop")
    XCTAssertTrue(json["backendName"] is NSNull)
    XCTAssertTrue(json["backendState"] is NSNull)
    XCTAssertTrue(json["backendLastError"] is NSNull)
    XCTAssertTrue(json["backendLastStartedAtMs"] is NSNull)
    XCTAssertTrue(json["backendPeerVirtualIp"] is NSNull)
    XCTAssertTrue(json["backendSelectedEndpoint"] is NSNull)
    XCTAssertEqual(json["remoteAddress"] as? String, "203.0.113.10")
    XCTAssertEqual(json["packetRxCount"] as? Int64, 0)
    XCTAssertEqual(json["packetRxBytes"] as? Int64, 0)
    XCTAssertEqual(json["packetTxCount"] as? Int64, 0)
    XCTAssertEqual(json["packetTxBytes"] as? Int64, 0)
    XCTAssertTrue(json["lastPacketAtMs"] is NSNull)
    XCTAssertEqual(json["lastError"] as? String, "setTunnelNetworkSettings failed")
    XCTAssertTrue(json["lastAppliedAtMs"] is NSNull)
  }
}

private func makeProviderConfiguration() -> [String: Any] {
  [
    "localVirtualIp": "100.64.0.10",
    "peerVirtualIp": "100.64.0.2",
    "wireguardInterface": [
      "addresses": ["100.64.0.10/32"],
      "dnsServers": ["1.1.1.1", "8.8.8.8"],
      "mtu": 1280,
    ],
    "wireguardPeer": [
      "publicKey": "peer-pub",
      "endpoint": "203.0.113.10:51820",
      "allowedIps": [
        ["cidr": "100.64.0.2/32"],
        ["cidr": "10.0.0.0/24"],
      ],
    ],
  ]
}

private final class FakeWireGuardBackendAdapter: WireGuardBackendAdapting {
  var startConfiguration: PacketTunnelProviderConfiguration?
  var inboundPackets: [Data] = []
  var inboundProtocols: [NSNumber] = []
  var stopCalled = false

  func start(configuration: PacketTunnelProviderConfiguration) throws {
    startConfiguration = configuration
  }

  func stop() {
    stopCalled = true
  }

  func handleInboundPackets(_ packets: [Data], protocols: [NSNumber]) throws -> PacketTunnelEngineOutput {
    inboundPackets = packets
    inboundProtocols = protocols
    return PacketTunnelEngineOutput(
      outboundPackets: [Data([0xcc])],
      outboundProtocols: [NSNumber(value: AF_INET6)]
    )
  }

  func runtimeSnapshot() -> WireGuardBackendRuntimeSnapshot {
    WireGuardBackendRuntimeSnapshot(
      backendName: "fake",
      backendState: "started",
      lastBackendError: nil,
      lastStartedAtMs: nil,
      peerVirtualIp: nil,
      selectedEndpoint: nil
    )
  }
}
