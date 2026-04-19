import XCTest
@testable import TunnelControlHarness

final class WireGuardKitNativeBackendSessionTests: XCTestCase {
  func testDefaultSessionFailsWithStableMessage() throws {
    let session = WireGuardKitNativeBackendSession()
    let configuration = try makeWireGuardKitSessionConfiguration()

    XCTAssertThrowsError(try session.start(sessionConfiguration: configuration)) { error in
      let providerError = error as? PacketTunnelProviderError
      XCTAssertEqual(providerError?.message, "WireGuardKit native backend session is not integrated yet")
    }
  }

  func testSessionDelegatesMappedConfigurationToController() throws {
    let controller = FakeWireGuardKitNativeBackendController()
    let session = WireGuardKitNativeBackendSession(controller: controller)
    let configuration = try makeWireGuardKitSessionConfiguration()
    let packet = Data([0x01, 0x02])
    let proto = NSNumber(value: AF_INET6)

    try session.start(sessionConfiguration: configuration)
    let output = try session.handleInboundPackets([packet], protocols: [proto])
    session.stop()

    XCTAssertEqual(controller.startConfiguration?.peerAddress, "100.64.0.2")
    XCTAssertEqual(controller.startConfiguration?.endpoint, "203.0.113.10:51820")
    XCTAssertEqual(controller.inboundPackets, [packet])
    XCTAssertEqual(controller.inboundProtocols, [proto])
    XCTAssertTrue(controller.stopCalled)
    XCTAssertEqual(output.outboundPackets, [Data([0xaa])])
    XCTAssertEqual(output.outboundProtocols, [NSNumber(value: AF_INET)])
  }

  func testUnavailableSessionFailsWithStableMessage() throws {
    let session = UnavailableWireGuardKitNativeBackendSession()
    let configuration = try makeWireGuardKitSessionConfiguration()

    XCTAssertThrowsError(try session.start(sessionConfiguration: configuration)) { error in
      let providerError = error as? PacketTunnelProviderError
      XCTAssertEqual(providerError?.message, "WireGuardKit native backend session is not integrated yet")
    }
  }

  func testUnavailableSessionReturnsEmptyIoAndNoSnapshot() throws {
    let session = UnavailableWireGuardKitNativeBackendSession()
    let output = try session.handleInboundPackets([Data([0x01, 0x02])], protocols: [NSNumber(value: AF_INET)])

    XCTAssertTrue(output.outboundPackets.isEmpty)
    XCTAssertTrue(output.outboundProtocols.isEmpty)
    XCTAssertNil(session.runtimeSnapshot())
  }
}

private final class FakeWireGuardKitNativeBackendController: WireGuardKitNativeBackendSessionControlling {
  var startConfiguration: WireGuardKitNativeBackendConfiguration?
  var inboundPackets: [Data] = []
  var inboundProtocols: [NSNumber] = []
  var stopCalled = false

  func start(configuration: WireGuardKitNativeBackendConfiguration) throws {
    startConfiguration = configuration
  }

  func stop() {
    stopCalled = true
  }

  func handleInboundPackets(_ packets: [Data], protocols: [NSNumber]) throws -> PacketTunnelEngineOutput {
    inboundPackets = packets
    inboundProtocols = protocols
    return PacketTunnelEngineOutput(
      outboundPackets: [Data([0xaa])],
      outboundProtocols: [NSNumber(value: AF_INET)]
    )
  }

  func runtimeSnapshot() -> WireGuardBackendRuntimeSnapshot? {
    nil
  }
}
