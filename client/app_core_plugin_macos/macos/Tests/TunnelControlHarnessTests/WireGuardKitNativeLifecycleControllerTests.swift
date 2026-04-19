import XCTest
@testable import TunnelControlHarness

final class WireGuardKitNativeLifecycleControllerTests: XCTestCase {
  func testDefaultLifecycleFailsWithStableMessage() throws {
    let lifecycle = WireGuardKitNativeLifecycleController()
    let configuration = try makeWireGuardKitNativeBackendConfiguration()

    XCTAssertThrowsError(try lifecycle.start(configuration: configuration)) { error in
      let providerError = error as? PacketTunnelProviderError
      XCTAssertEqual(providerError?.message, "WireGuardKit native backend session is not integrated yet")
    }
  }

  func testLifecycleDelegatesToAdapterAndHandle() throws {
    let session = FakeWireGuardKitNativeSession()
    let adapter = FakeWireGuardKitNativeAdapter(session: session)
    let lifecycle = WireGuardKitNativeLifecycleController(
      adapter: adapter,
      nowMs: { 1712345678123 }
    )
    let configuration = try makeWireGuardKitNativeBackendConfiguration()
    let packet = Data([0x10, 0x20])
    let proto = NSNumber(value: AF_INET6)

    let handle = try lifecycle.start(configuration: configuration)
    let output = try handle.handleInboundPackets([packet], protocols: [proto])
    handle.stop()

    XCTAssertEqual(adapter.startConfiguration?.peerAddress, "100.64.0.2")
    XCTAssertEqual(session.inboundPackets, [packet])
    XCTAssertEqual(session.inboundProtocols, [proto])
    XCTAssertTrue(session.stopCalled)
    XCTAssertEqual(output.outboundPackets, [Data([0xcc])])
    XCTAssertEqual(output.outboundProtocols, [NSNumber(value: AF_INET)])
  }

  func testHandlePublishesRunningAndStoppedSnapshot() throws {
    let configuration = try makeWireGuardKitNativeBackendConfiguration()
    let session = FakeWireGuardKitNativeSession()
    let handle = WireGuardKitNativeBackendHandle(
      session: session,
      configuration: configuration,
      startedAtMs: 1712345678123
    )

    let runningSnapshot = handle.runtimeSnapshot()
    handle.stop()
    let stoppedSnapshot = handle.runtimeSnapshot()

    XCTAssertEqual(runningSnapshot?.backendName, "wireguardkit-native")
    XCTAssertEqual(runningSnapshot?.backendState, "running")
    XCTAssertEqual(runningSnapshot?.lastStartedAtMs, 1712345678123)
    XCTAssertEqual(runningSnapshot?.peerVirtualIp, "100.64.0.2")
    XCTAssertEqual(runningSnapshot?.selectedEndpoint, "203.0.113.10:51820")
    XCTAssertEqual(stoppedSnapshot?.backendState, "stopped")
  }

  func testHandlePrefersSessionRuntimeSnapshot() throws {
    let configuration = try makeWireGuardKitNativeBackendConfiguration()
    let session = FakeWireGuardKitNativeSession()
    session.snapshot = WireGuardBackendRuntimeSnapshot(
      backendName: "wireguardkit-sdk",
      backendState: "sdk-running",
      lastBackendError: "sdk warning",
      lastStartedAtMs: 1712345678999,
      peerVirtualIp: "100.64.0.77",
      selectedEndpoint: "198.51.100.77:51820"
    )
    let handle = WireGuardKitNativeBackendHandle(
      session: session,
      configuration: configuration,
      startedAtMs: 1712345678123
    )

    let snapshot = handle.runtimeSnapshot()

    XCTAssertEqual(snapshot?.backendName, "wireguardkit-sdk")
    XCTAssertEqual(snapshot?.backendState, "sdk-running")
    XCTAssertEqual(snapshot?.lastBackendError, "sdk warning")
    XCTAssertEqual(snapshot?.lastStartedAtMs, 1712345678999)
    XCTAssertEqual(snapshot?.peerVirtualIp, "100.64.0.77")
    XCTAssertEqual(snapshot?.selectedEndpoint, "198.51.100.77:51820")
  }
}

private final class FakeWireGuardKitNativeAdapter: WireGuardKitNativeAdapting {
  var startConfiguration: WireGuardKitNativeBackendConfiguration?
  let session: WireGuardKitNativeSessioning

  init(session: WireGuardKitNativeSessioning) {
    self.session = session
  }

  func makeSession(configuration: WireGuardKitNativeBackendConfiguration) throws -> WireGuardKitNativeSessioning {
    startConfiguration = configuration
    return session
  }
}

private final class FakeWireGuardKitNativeSession: WireGuardKitNativeSessioning {
  var inboundPackets: [Data] = []
  var inboundProtocols: [NSNumber] = []
  var stopCalled = false
  var snapshot: WireGuardBackendRuntimeSnapshot?

  func stop() {
    stopCalled = true
  }

  func handleInboundPackets(_ packets: [Data], protocols: [NSNumber]) throws -> PacketTunnelEngineOutput {
    inboundPackets = packets
    inboundProtocols = protocols
    return PacketTunnelEngineOutput(
      outboundPackets: [Data([0xcc])],
      outboundProtocols: [NSNumber(value: AF_INET)]
    )
  }

  func runtimeSnapshot() -> WireGuardBackendRuntimeSnapshot? {
    snapshot
  }
}
