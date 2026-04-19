import XCTest
@testable import TunnelControlHarness

final class WireGuardKitNativeSessionFactoryTests: XCTestCase {
  func testDefaultFactoryProducesRunningSessionSnapshot() throws {
    let factory = WireGuardKitNativeSessionFactory()

    let session = try factory.makeSession(
      module: makeWireGuardKitNativeModule(),
      configuration: makeWireGuardKitNativeBackendConfiguration()
    )
    let snapshot = session.runtimeSnapshot()

    XCTAssertEqual(snapshot?.backendName, "wireguardkit-native")
    XCTAssertEqual(snapshot?.backendState, "running")
    XCTAssertNil(snapshot?.lastBackendError)
    XCTAssertEqual(snapshot?.peerVirtualIp, "100.64.0.2")
    XCTAssertEqual(snapshot?.selectedEndpoint, "203.0.113.10:51820")
  }

  func testFactoryBuildsStructuredSessionConfigurationFromModuleAndConfiguration() throws {
    let session = FakeWireGuardKitNativeSessionForFactory()
    var capturedConfiguration: WireGuardKitNativeSessionConfiguration?
    let builder = FakeWireGuardKitNativeSessionBuilder(session: session) { configuration in
      capturedConfiguration = configuration
    }
    let factory = WireGuardKitNativeSessionFactory(sessionBuilder: builder)

    let resolvedSession = try factory.makeSession(
      module: makeWireGuardKitNativeModule(),
      configuration: makeWireGuardKitNativeBackendConfiguration()
    )

    XCTAssertTrue(resolvedSession === session)
    XCTAssertEqual(capturedConfiguration?.backendName, "wireguardkit-native")
    XCTAssertEqual(capturedConfiguration?.frameworkPath, "/tmp/WireGuardKit.framework")
    XCTAssertEqual(capturedConfiguration?.moduleSource, "framework")
    XCTAssertEqual(capturedConfiguration?.interface.privateKey, "pending-wireguardkit-private-key")
    XCTAssertEqual(capturedConfiguration?.interface.publicKey, "pending-wireguardkit-public-key")
    XCTAssertEqual(capturedConfiguration?.interface.addresses, ["100.64.0.10"])
    XCTAssertEqual(capturedConfiguration?.interface.dnsServers, ["1.1.1.1"])
    XCTAssertEqual(capturedConfiguration?.interface.mtu, 1280)
    XCTAssertNil(capturedConfiguration?.interface.listenPort)
    XCTAssertEqual(capturedConfiguration?.peer.publicKey, "pending-wireguardkit-peer-public-key")
    XCTAssertEqual(capturedConfiguration?.peer.address, "100.64.0.2")
    XCTAssertEqual(capturedConfiguration?.peer.endpoint, "203.0.113.10:51820")
    XCTAssertEqual(capturedConfiguration?.peer.allowedIps, ["100.64.0.2/32"])
    XCTAssertNil(capturedConfiguration?.peer.persistentKeepaliveSeconds)
  }
}

private final class FakeWireGuardKitNativeSessionBuilder: WireGuardKitNativeSessionBuilding {
  private let providedSession: WireGuardKitNativeSessioning
  private let onConfiguration: (WireGuardKitNativeSessionConfiguration) -> Void

  init(
    session: WireGuardKitNativeSessioning,
    onConfiguration: @escaping (WireGuardKitNativeSessionConfiguration) -> Void = { _ in }
  ) {
    self.providedSession = session
    self.onConfiguration = onConfiguration
  }

  func makeSession(configuration: WireGuardKitNativeSessionConfiguration) throws -> WireGuardKitNativeSessioning {
    onConfiguration(configuration)
    return providedSession
  }
}

private final class FakeWireGuardKitNativeSessionForFactory: WireGuardKitNativeSessioning {
  func stop() {}

  func handleInboundPackets(_ packets: [Data], protocols: [NSNumber]) throws -> PacketTunnelEngineOutput {
    .empty
  }

  func runtimeSnapshot() -> WireGuardBackendRuntimeSnapshot? {
    nil
  }
}
