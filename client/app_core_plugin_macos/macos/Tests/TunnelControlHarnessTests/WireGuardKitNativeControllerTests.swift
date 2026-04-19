import XCTest
@testable import TunnelControlHarness

final class WireGuardKitNativeControllerTests: XCTestCase {
  func testDefaultControllerFailsWithUnavailableNativeBackend() throws {
    let controller = WireGuardKitNativeController()
    let configuration = try makeWireGuardKitNativeBackendConfiguration()

    XCTAssertThrowsError(try controller.start(configuration: configuration)) { error in
      let providerError = error as? PacketTunnelProviderError
      XCTAssertEqual(providerError?.message, "WireGuardKit native backend session is not integrated yet")
    }
  }

  func testControllerDelegatesToActiveHandleFromLifecycle() throws {
    let lifecycle = FakeWireGuardKitNativeBackendLifecycleController()
    let controller = WireGuardKitNativeController(lifecycleController: lifecycle)
    let configuration = try makeWireGuardKitNativeBackendConfiguration()
    let packet = Data([0x0a, 0x0b])
    let proto = NSNumber(value: AF_INET6)

    try controller.start(configuration: configuration)
    let output = try controller.handleInboundPackets([packet], protocols: [proto])
    let snapshot = controller.runtimeSnapshot()
    controller.stop()

    XCTAssertEqual(lifecycle.startConfiguration?.peerAddress, "100.64.0.2")
    XCTAssertEqual(lifecycle.handle.inboundPackets, [packet])
    XCTAssertEqual(lifecycle.handle.inboundProtocols, [proto])
    XCTAssertEqual(output.outboundPackets, [Data([0xbb])])
    XCTAssertEqual(output.outboundProtocols, [NSNumber(value: AF_INET)])
    XCTAssertEqual(snapshot?.backendName, "wireguardkit-native")
    XCTAssertEqual(snapshot?.backendState, "running")
    XCTAssertEqual(snapshot?.peerVirtualIp, "100.64.0.55")
    XCTAssertEqual(snapshot?.selectedEndpoint, "198.51.100.55:51820")
    XCTAssertTrue(lifecycle.handle.stopCalled)
  }

  func testControllerReturnsEmptyOutputWithoutActiveHandle() throws {
    let controller = WireGuardKitNativeController()

    let output = try controller.handleInboundPackets([Data([0x01])], protocols: [NSNumber(value: AF_INET)])

    XCTAssertTrue(output.outboundPackets.isEmpty)
    XCTAssertTrue(output.outboundProtocols.isEmpty)
  }
}

private final class FakeWireGuardKitNativeBackendLifecycleController: WireGuardKitNativeBackendLifecycleControlling {
  var startConfiguration: WireGuardKitNativeBackendConfiguration?
  let handle = FakeWireGuardKitNativeBackendHandle()

  func start(configuration: WireGuardKitNativeBackendConfiguration) throws -> WireGuardKitNativeBackendHandleControlling {
    startConfiguration = configuration
    return handle
  }
}

private final class FakeWireGuardKitNativeBackendHandle: WireGuardKitNativeBackendHandleControlling {
  var inboundPackets: [Data] = []
  var inboundProtocols: [NSNumber] = []
  var stopCalled = false

  func stop() {
    stopCalled = true
  }

  func handleInboundPackets(_ packets: [Data], protocols: [NSNumber]) throws -> PacketTunnelEngineOutput {
    inboundPackets = packets
    inboundProtocols = protocols
    return PacketTunnelEngineOutput(
      outboundPackets: [Data([0xbb])],
      outboundProtocols: [NSNumber(value: AF_INET)]
    )
  }

  func runtimeSnapshot() -> WireGuardBackendRuntimeSnapshot? {
    WireGuardBackendRuntimeSnapshot(
      backendName: "wireguardkit-native",
      backendState: "running",
      lastBackendError: nil,
      lastStartedAtMs: 1712345678999,
      peerVirtualIp: "100.64.0.55",
      selectedEndpoint: "198.51.100.55:51820"
    )
  }
}
