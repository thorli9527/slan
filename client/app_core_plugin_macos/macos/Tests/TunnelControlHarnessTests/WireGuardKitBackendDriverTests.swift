import XCTest
@testable import TunnelControlHarness

final class WireGuardKitBackendDriverTests: XCTestCase {
  func testNativeDriverFailsWithUnavailableDefaultSession() throws {
    let driver = WireGuardKitNativeBackendDriver()
    let configuration = try makeWireGuardKitSessionConfiguration()

    XCTAssertThrowsError(try driver.start(sessionConfiguration: configuration)) { error in
      let providerError = error as? PacketTunnelProviderError
      XCTAssertEqual(providerError?.message, "WireGuardKit native backend session is not integrated yet")
    }

    let snapshot = driver.runtimeSnapshot()
    XCTAssertEqual(snapshot?.backendName, "wireguardkit")
    XCTAssertEqual(snapshot?.backendState, "failed")
    XCTAssertEqual(snapshot?.lastBackendError, "WireGuardKit native backend session is not integrated yet")
    XCTAssertEqual(snapshot?.peerVirtualIp, "100.64.0.2")
    XCTAssertEqual(snapshot?.selectedEndpoint, "203.0.113.10:51820")
  }

  func testNativeDriverDelegatesToSession() throws {
    let session = FakeWireGuardKitNativeBackendSession()
    let driver = WireGuardKitNativeBackendDriver(sessionFactory: { session })
    let configuration = try makeWireGuardKitSessionConfiguration()
    let packet = Data([0x10, 0x20])
    let proto = NSNumber(value: AF_INET)

    try driver.start(sessionConfiguration: configuration)
    let output = try driver.handleInboundPackets([packet], protocols: [proto])
    driver.stop()

    XCTAssertEqual(session.startConfiguration?.peerAddress, "100.64.0.2")
    XCTAssertEqual(session.inboundPackets, [packet])
    XCTAssertEqual(session.inboundProtocols, [proto])
    XCTAssertTrue(session.stopCalled)
    XCTAssertEqual(output.outboundPackets, [Data([0x44])])
    XCTAssertEqual(output.outboundProtocols, [NSNumber(value: AF_INET6)])
  }

  func testNativeDriverPrefersSessionRuntimeSnapshot() throws {
    let session = FakeWireGuardKitNativeBackendSession()
    session.snapshot = WireGuardBackendRuntimeSnapshot(
      backendName: "wireguardkit-native",
      backendState: "session-running",
      lastBackendError: "session warning",
      lastStartedAtMs: 1712345678777,
      peerVirtualIp: "100.64.0.77",
      selectedEndpoint: "198.51.100.77:51820"
    )
    let driver = WireGuardKitNativeBackendDriver(sessionFactory: { session })

    try driver.start(sessionConfiguration: makeWireGuardKitSessionConfiguration())
    let snapshot = driver.runtimeSnapshot()

    XCTAssertEqual(snapshot?.backendName, "wireguardkit-native")
    XCTAssertEqual(snapshot?.backendState, "session-running")
    XCTAssertEqual(snapshot?.lastBackendError, "session warning")
    XCTAssertEqual(snapshot?.lastStartedAtMs, 1712345678777)
    XCTAssertEqual(snapshot?.peerVirtualIp, "100.64.0.77")
    XCTAssertEqual(snapshot?.selectedEndpoint, "198.51.100.77:51820")
  }
}

private final class FakeWireGuardKitNativeBackendSession: WireGuardKitNativeBackendSessioning {
  var startConfiguration: WireGuardKitSessionConfiguration?
  var inboundPackets: [Data] = []
  var inboundProtocols: [NSNumber] = []
  var stopCalled = false
  var snapshot: WireGuardBackendRuntimeSnapshot?

  func start(sessionConfiguration: WireGuardKitSessionConfiguration) throws {
    startConfiguration = sessionConfiguration
  }

  func stop() {
    stopCalled = true
  }

  func handleInboundPackets(_ packets: [Data], protocols: [NSNumber]) throws -> PacketTunnelEngineOutput {
    inboundPackets = packets
    inboundProtocols = protocols
    return PacketTunnelEngineOutput(
      outboundPackets: [Data([0x44])],
      outboundProtocols: [NSNumber(value: AF_INET6)]
    )
  }

  func runtimeSnapshot() -> WireGuardBackendRuntimeSnapshot? {
    snapshot
  }
}
