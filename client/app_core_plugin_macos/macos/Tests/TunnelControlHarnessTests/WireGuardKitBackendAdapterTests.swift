import XCTest
@testable import TunnelControlHarness

final class WireGuardKitBackendAdapterTests: XCTestCase {
  func testStartCachesFailureState() throws {
    var providerConfiguration = makeWireGuardKitProviderConfiguration()
    providerConfiguration["debugEngineMode"] = "external"
    let configuration = try PacketTunnelProviderConfiguration(providerConfiguration: providerConfiguration)
    let session = WireGuardKitBackendSession(
      controller: WireGuardKitController(nowMs: { 1712345677000 })
    )
    let adapter = WireGuardKitBackendAdapter(
      session: session,
      nowMs: { 1712345677000 }
    )

    XCTAssertThrowsError(try adapter.start(configuration: configuration)) { error in
      let providerError = error as? PacketTunnelProviderError
      XCTAssertEqual(providerError?.message, "WireGuardKit framework is not linked into the app bundle")
    }

    let snapshot = adapter.runtimeSnapshot()
    XCTAssertEqual(snapshot.backendName, "wireguardkit")
    XCTAssertEqual(snapshot.backendState, "unavailable")
    XCTAssertEqual(snapshot.lastBackendError, "WireGuardKit framework is not linked into the app bundle")
    XCTAssertEqual(snapshot.lastStartedAtMs, 1712345677000)
    XCTAssertEqual(snapshot.peerVirtualIp, "100.64.0.2")
    XCTAssertEqual(snapshot.selectedEndpoint, "203.0.113.10:51820")
  }

  func testStopTransitionsBackendStateToStopped() {
    let adapter = WireGuardKitBackendAdapter()

    adapter.stop()

    let snapshot = adapter.runtimeSnapshot()
    XCTAssertEqual(snapshot.backendName, "wireguardkit")
    XCTAssertEqual(snapshot.backendState, "stopped")
    XCTAssertNil(snapshot.lastStartedAtMs)
    XCTAssertNil(snapshot.peerVirtualIp)
    XCTAssertNil(snapshot.selectedEndpoint)
  }

  func testStartCachesStartedStateWhenSessionStarts() throws {
    let session = FakeWireGuardKitBackendSession()
    var providerConfiguration = makeWireGuardKitProviderConfiguration()
    providerConfiguration["debugEngineMode"] = "external"
    let configuration = try PacketTunnelProviderConfiguration(providerConfiguration: providerConfiguration)
    let adapter = WireGuardKitBackendAdapter(
      session: session,
      nowMs: { 1712345677001 }
    )

    try adapter.start(configuration: configuration)

    XCTAssertEqual(session.startConfiguration?.peerVirtualIp, "100.64.0.2")
    let snapshot = adapter.runtimeSnapshot()
    XCTAssertEqual(snapshot.backendName, "wireguardkit")
    XCTAssertEqual(snapshot.backendState, "started")
    XCTAssertNil(snapshot.lastBackendError)
    XCTAssertEqual(snapshot.lastStartedAtMs, 1712345677001)
    XCTAssertEqual(snapshot.peerVirtualIp, "100.64.0.2")
    XCTAssertEqual(snapshot.selectedEndpoint, "203.0.113.10:51820")
  }

  func testHandleInboundPacketsDelegatesToSession() throws {
    let session = FakeWireGuardKitBackendSession()
    var providerConfiguration = makeWireGuardKitProviderConfiguration()
    providerConfiguration["debugEngineMode"] = "external"
    let configuration = try PacketTunnelProviderConfiguration(providerConfiguration: providerConfiguration)
    let adapter = WireGuardKitBackendAdapter(session: session)
    let packet = Data([0xaa, 0xbb])
    let proto = NSNumber(value: AF_INET)

    try adapter.start(configuration: configuration)
    let output = try adapter.handleInboundPackets([packet], protocols: [proto])

    XCTAssertEqual(session.inboundPackets, [packet])
    XCTAssertEqual(session.inboundProtocols, [proto])
    XCTAssertEqual(output.outboundPackets, [Data([0xcc])])
    XCTAssertEqual(output.outboundProtocols, [NSNumber(value: AF_INET6)])
  }

  func testRuntimeSnapshotPrefersSessionSnapshotWhenAvailable() {
    let session = FakeWireGuardKitBackendSession()
    session.snapshot = WireGuardBackendRuntimeSnapshot(
      backendName: "wireguardkit-session",
      backendState: "running",
      lastBackendError: "backend warning",
      lastStartedAtMs: 1712345677999,
      peerVirtualIp: "100.64.0.200",
      selectedEndpoint: "198.51.100.10:51820"
    )
    let adapter = WireGuardKitBackendAdapter(session: session)

    let snapshot = adapter.runtimeSnapshot()

    XCTAssertEqual(snapshot.backendName, "wireguardkit-session")
    XCTAssertEqual(snapshot.backendState, "running")
    XCTAssertEqual(snapshot.lastBackendError, "backend warning")
    XCTAssertEqual(snapshot.lastStartedAtMs, 1712345677999)
    XCTAssertEqual(snapshot.peerVirtualIp, "100.64.0.200")
    XCTAssertEqual(snapshot.selectedEndpoint, "198.51.100.10:51820")
  }
}

private final class FakeWireGuardKitBackendSession: WireGuardKitBackendSessioning {
  var startConfiguration: WireGuardKitBackendConfiguration?
  var inboundPackets: [Data] = []
  var inboundProtocols: [NSNumber] = []
  var snapshot: WireGuardBackendRuntimeSnapshot?

  func start(configuration: WireGuardKitBackendConfiguration) throws {
    startConfiguration = configuration
  }

  func stop() {}

  func handleInboundPackets(_ packets: [Data], protocols: [NSNumber]) throws -> PacketTunnelEngineOutput {
    inboundPackets = packets
    inboundProtocols = protocols
    return PacketTunnelEngineOutput(
      outboundPackets: [Data([0xcc])],
      outboundProtocols: [NSNumber(value: AF_INET6)]
    )
  }

  func runtimeSnapshot() -> WireGuardBackendRuntimeSnapshot? {
    snapshot
  }
}

private func makeWireGuardKitProviderConfiguration() -> [String: Any] {
  [
    "localVirtualIp": "100.64.0.10",
    "peerVirtualIp": "100.64.0.2",
    "transport": "relay",
    "wireguardInterface": [
      "addresses": ["100.64.0.10/32"],
      "dnsServers": ["1.1.1.1"],
      "mtu": 1280,
    ],
    "wireguardPeer": [
      "publicKey": "peer-pub",
      "endpoint": "203.0.113.10:51820",
      "allowedIps": [
        ["cidr": "100.64.0.2/32"],
      ],
    ],
  ]
}
