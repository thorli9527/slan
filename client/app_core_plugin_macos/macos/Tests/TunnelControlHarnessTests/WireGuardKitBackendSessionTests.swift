import XCTest
@testable import TunnelControlHarness

final class WireGuardKitBackendSessionTests: XCTestCase {
  func testDefaultSessionFailsWithUnavailableBackend() throws {
    let session = WireGuardKitBackendSession()
    let configuration = try makeWireGuardKitBackendConfiguration()

    XCTAssertThrowsError(try session.start(configuration: configuration)) { error in
      let providerError = error as? PacketTunnelProviderError
      XCTAssertEqual(providerError?.message, "WireGuardKit framework is not linked into the app bundle")
    }
  }

  func testSessionDelegatesMappedConfigurationToController() throws {
    let controller = FakeWireGuardKitController()
    let session = WireGuardKitBackendSession(controller: controller)
    let configuration = try makeWireGuardKitBackendConfiguration()
    let packet = Data([0x01, 0x02])
    let proto = NSNumber(value: AF_INET)

    try session.start(configuration: configuration)
    let output = try session.handleInboundPackets([packet], protocols: [proto])
    session.stop()

    XCTAssertEqual(controller.startConfiguration?.peerAddress, "100.64.0.2")
    XCTAssertEqual(controller.startConfiguration?.endpoint, "203.0.113.10:51820")
    XCTAssertEqual(controller.startConfiguration?.allowedIps, ["100.64.0.2/32"])
    XCTAssertEqual(controller.inboundPackets, [packet])
    XCTAssertEqual(controller.inboundProtocols, [proto])
    XCTAssertTrue(controller.stopCalled)
    XCTAssertEqual(output.outboundPackets, [Data([0xee])])
    XCTAssertEqual(output.outboundProtocols, [NSNumber(value: AF_INET6)])
  }
}

private final class FakeWireGuardKitController: WireGuardKitBackendSessionControlling {
  var startConfiguration: WireGuardKitSessionConfiguration?
  var inboundPackets: [Data] = []
  var inboundProtocols: [NSNumber] = []
  var stopCalled = false

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
      outboundPackets: [Data([0xee])],
      outboundProtocols: [NSNumber(value: AF_INET6)]
    )
  }

  func runtimeSnapshot() -> WireGuardBackendRuntimeSnapshot? {
    nil
  }
}
