import XCTest
@testable import TunnelControlHarness

final class WireGuardKitControllerTests: XCTestCase {
  func testDefaultControllerFailsWithUnavailableBackend() throws {
    let controller = WireGuardKitController()
    let configuration = try makeWireGuardKitSessionConfiguration()

    XCTAssertThrowsError(try controller.start(sessionConfiguration: configuration)) { error in
      let providerError = error as? PacketTunnelProviderError
      XCTAssertEqual(providerError?.message, "WireGuardKit framework is not linked into the app bundle")
    }

    let snapshot = controller.runtimeSnapshot()
    XCTAssertEqual(snapshot?.backendName, "wireguardkit")
    XCTAssertEqual(snapshot?.backendState, "unavailable")
    XCTAssertEqual(snapshot?.lastBackendError, "WireGuardKit framework is not linked into the app bundle")
    XCTAssertEqual(snapshot?.peerVirtualIp, "100.64.0.2")
    XCTAssertEqual(snapshot?.selectedEndpoint, "203.0.113.10:51820")
  }

  func testControllerDelegatesToActiveHandleFromLifecycle() throws {
    let lifecycle = FakeWireGuardKitLifecycleController()
    let availabilityChecker = FakeWireGuardKitAvailabilityChecker()
    let controller = WireGuardKitController(
      lifecycleController: lifecycle,
      availabilityChecker: availabilityChecker,
      nowMs: { 1712345678000 }
    )
    let configuration = try makeWireGuardKitSessionConfiguration()
    let packet = Data([0x0a, 0x0b])
    let proto = NSNumber(value: AF_INET)

    try controller.start(sessionConfiguration: configuration)
    let output = try controller.handleInboundPackets([packet], protocols: [proto])
    controller.stop()

    XCTAssertEqual(lifecycle.startConfiguration?.peerAddress, "100.64.0.2")
    XCTAssertEqual(lifecycle.handle.inboundPackets, [packet])
    XCTAssertEqual(lifecycle.handle.inboundProtocols, [proto])
    XCTAssertTrue(lifecycle.handle.stopCalled)
    XCTAssertEqual(output.outboundPackets, [Data([0xdd])])
    XCTAssertEqual(output.outboundProtocols, [NSNumber(value: AF_INET6)])
    let snapshot = controller.runtimeSnapshot()
    XCTAssertEqual(snapshot?.backendName, "wireguardkit")
    XCTAssertEqual(snapshot?.backendState, "stopped")
    XCTAssertNil(snapshot?.lastBackendError)
    XCTAssertEqual(snapshot?.lastStartedAtMs, 1712345678000)
    XCTAssertEqual(snapshot?.peerVirtualIp, "100.64.0.2")
    XCTAssertEqual(snapshot?.selectedEndpoint, "203.0.113.10:51820")
  }

  func testControllerPrefersActiveHandleRuntimeSnapshot() throws {
    let lifecycle = FakeWireGuardKitLifecycleController()
    lifecycle.handle.snapshot = WireGuardBackendRuntimeSnapshot(
      backendName: "wireguardkit",
      backendState: "running",
      lastBackendError: nil,
      lastStartedAtMs: 1712345678999,
      peerVirtualIp: "100.64.0.44",
      selectedEndpoint: "198.51.100.10:51820"
    )
    let controller = WireGuardKitController(
      lifecycleController: lifecycle,
      availabilityChecker: FakeWireGuardKitAvailabilityChecker()
    )
    try controller.start(sessionConfiguration: makeWireGuardKitSessionConfiguration())

    let snapshot = controller.runtimeSnapshot()

    XCTAssertEqual(snapshot?.backendState, "running")
    XCTAssertEqual(snapshot?.peerVirtualIp, "100.64.0.44")
    XCTAssertEqual(snapshot?.selectedEndpoint, "198.51.100.10:51820")
  }

  func testControllerReturnsEmptyOutputWithoutActiveHandle() throws {
    let controller = WireGuardKitController(
      lifecycleController: FakeWireGuardKitLifecycleController(),
      availabilityChecker: FakeWireGuardKitAvailabilityChecker()
    )

    let output = try controller.handleInboundPackets([Data([0x01])], protocols: [NSNumber(value: AF_INET)])

    XCTAssertTrue(output.outboundPackets.isEmpty)
    XCTAssertTrue(output.outboundProtocols.isEmpty)
  }

  func testDefaultLifecycleFailsWithUnavailableNativeSession() throws {
    let controller = WireGuardKitController(
      lifecycleController: DefaultWireGuardKitBackendLifecycleController(nowMs: { 1712345678123 }),
      availabilityChecker: FakeWireGuardKitAvailabilityChecker(),
      nowMs: { 1712345678000 }
    )

    XCTAssertThrowsError(try controller.start(sessionConfiguration: makeWireGuardKitSessionConfiguration())) { error in
      let providerError = error as? PacketTunnelProviderError
      XCTAssertEqual(providerError?.message, "WireGuardKit native backend session is not integrated yet")
    }
    let snapshot = controller.runtimeSnapshot()

    XCTAssertEqual(snapshot?.backendName, "wireguardkit")
    XCTAssertEqual(snapshot?.backendState, "failed")
    XCTAssertEqual(snapshot?.lastBackendError, "WireGuardKit native backend session is not integrated yet")
    XCTAssertEqual(snapshot?.lastStartedAtMs, 1712345678000)
    XCTAssertEqual(snapshot?.peerVirtualIp, "100.64.0.2")
    XCTAssertEqual(snapshot?.selectedEndpoint, "203.0.113.10:51820")
  }

  func testDefaultLifecycleProducesRunningSnapshotWhenNativeSessionStarts() throws {
    let session = FakeWireGuardKitNativeBackendSession()
    let controller = WireGuardKitController(
      lifecycleController: DefaultWireGuardKitBackendLifecycleController(
        nowMs: { 1712345678123 },
        driverFactory: {
          WireGuardKitNativeBackendDriver(sessionFactory: { session })
        }
      ),
      availabilityChecker: FakeWireGuardKitAvailabilityChecker(),
      nowMs: { 1712345678000 }
    )

    try controller.start(sessionConfiguration: makeWireGuardKitSessionConfiguration())
    let snapshot = controller.runtimeSnapshot()

    XCTAssertEqual(snapshot?.backendName, "wireguardkit")
    XCTAssertEqual(snapshot?.backendState, "running")
    XCTAssertNil(snapshot?.lastBackendError)
    XCTAssertEqual(snapshot?.lastStartedAtMs, 1712345678123)
    XCTAssertEqual(snapshot?.peerVirtualIp, "100.64.0.2")
    XCTAssertEqual(snapshot?.selectedEndpoint, "203.0.113.10:51820")
  }

  func testDefaultLifecycleDelegatesToDriver() throws {
    let driver = FakeWireGuardKitBackendDriver()
    let controller = WireGuardKitController(
      lifecycleController: DefaultWireGuardKitBackendLifecycleController(
        nowMs: { 1712345678124 },
        driverFactory: { driver }
      ),
      availabilityChecker: FakeWireGuardKitAvailabilityChecker()
    )
    let configuration = try makeWireGuardKitSessionConfiguration()
    let packet = Data([0xab, 0xcd])
    let proto = NSNumber(value: AF_INET6)

    try controller.start(sessionConfiguration: configuration)
    let output = try controller.handleInboundPackets([packet], protocols: [proto])
    controller.stop()

    XCTAssertEqual(driver.startConfiguration?.peerAddress, "100.64.0.2")
    XCTAssertEqual(driver.inboundPackets, [packet])
    XCTAssertEqual(driver.inboundProtocols, [proto])
    XCTAssertTrue(driver.stopCalled)
    XCTAssertEqual(output.outboundPackets, [Data([0xfa])])
    XCTAssertEqual(output.outboundProtocols, [NSNumber(value: AF_INET)])
  }

  func testDefaultLifecyclePrefersDriverRuntimeSnapshot() throws {
    let driver = FakeWireGuardKitBackendDriver()
    driver.snapshot = WireGuardBackendRuntimeSnapshot(
      backendName: "wireguardkit-driver",
      backendState: "driver-running",
      lastBackendError: "driver warning",
      lastStartedAtMs: 1712345678998,
      peerVirtualIp: "100.64.0.88",
      selectedEndpoint: "198.51.100.88:51820"
    )
    let controller = WireGuardKitController(
      lifecycleController: DefaultWireGuardKitBackendLifecycleController(
        nowMs: { 1712345678125 },
        driverFactory: { driver }
      ),
      availabilityChecker: FakeWireGuardKitAvailabilityChecker()
    )

    try controller.start(sessionConfiguration: makeWireGuardKitSessionConfiguration())
    let snapshot = controller.runtimeSnapshot()

    XCTAssertEqual(snapshot?.backendName, "wireguardkit-driver")
    XCTAssertEqual(snapshot?.backendState, "driver-running")
    XCTAssertEqual(snapshot?.lastBackendError, "driver warning")
    XCTAssertEqual(snapshot?.lastStartedAtMs, 1712345678998)
    XCTAssertEqual(snapshot?.peerVirtualIp, "100.64.0.88")
    XCTAssertEqual(snapshot?.selectedEndpoint, "198.51.100.88:51820")
  }
}

private final class FakeWireGuardKitLifecycleController: WireGuardKitBackendSessionLifecycleControlling {
  var startConfiguration: WireGuardKitSessionConfiguration?
  let handle = FakeWireGuardKitBackendHandle()

  func start(sessionConfiguration: WireGuardKitSessionConfiguration) throws -> WireGuardKitBackendHandle {
    startConfiguration = sessionConfiguration
    return handle
  }
}

private final class FakeWireGuardKitBackendHandle: WireGuardKitBackendHandle {
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
      outboundPackets: [Data([0xdd])],
      outboundProtocols: [NSNumber(value: AF_INET6)]
    )
  }

  func runtimeSnapshot() -> WireGuardBackendRuntimeSnapshot? {
    snapshot
  }
}

private struct FakeWireGuardKitAvailabilityChecker: WireGuardKitBackendAvailabilityChecking {
  let backendName = "wireguardkit"

  func availability() -> WireGuardKitBackendAvailability {
    WireGuardKitBackendAvailability(isAvailable: true, reason: nil)
  }
}

private final class FakeWireGuardKitBackendDriver: WireGuardKitBackendDriving {
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
      outboundPackets: [Data([0xfa])],
      outboundProtocols: [NSNumber(value: AF_INET)]
    )
  }

  func runtimeSnapshot() -> WireGuardBackendRuntimeSnapshot? {
    snapshot
  }
}

private final class FakeWireGuardKitNativeBackendSession: WireGuardKitNativeBackendSessioning {
  func start(sessionConfiguration: WireGuardKitSessionConfiguration) throws {}

  func stop() {}

  func handleInboundPackets(_ packets: [Data], protocols: [NSNumber]) throws -> PacketTunnelEngineOutput {
    .empty
  }

  func runtimeSnapshot() -> WireGuardBackendRuntimeSnapshot? {
    nil
  }
}
