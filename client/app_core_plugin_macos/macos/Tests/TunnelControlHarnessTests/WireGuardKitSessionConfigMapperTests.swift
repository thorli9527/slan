import XCTest
@testable import TunnelControlHarness

final class WireGuardKitSessionConfigMapperTests: XCTestCase {
  func testMapCarriesBackendConfigurationIntoSessionShape() throws {
    let backendConfiguration = try makeWireGuardKitBackendConfiguration()

    let sessionConfiguration = WireGuardKitSessionConfigMapper.map(backendConfiguration)

    XCTAssertEqual(sessionConfiguration.localAddress, "100.64.0.10")
    XCTAssertEqual(sessionConfiguration.peerAddress, "100.64.0.2")
    XCTAssertEqual(sessionConfiguration.interfacePrivateKey, "pending-wireguardkit-private-key")
    XCTAssertEqual(sessionConfiguration.interfacePublicKey, "pending-wireguardkit-public-key")
    XCTAssertEqual(sessionConfiguration.interfaceAddresses, ["100.64.0.10"])
    XCTAssertEqual(sessionConfiguration.dnsServers, ["1.1.1.1"])
    XCTAssertEqual(sessionConfiguration.mtu, 1280)
    XCTAssertNil(sessionConfiguration.listenPort)
    XCTAssertEqual(sessionConfiguration.peerPublicKey, "pending-wireguardkit-peer-public-key")
    XCTAssertEqual(sessionConfiguration.endpoint, "203.0.113.10:51820")
    XCTAssertEqual(sessionConfiguration.allowedIps, ["100.64.0.2/32"])
    XCTAssertNil(sessionConfiguration.persistentKeepaliveSeconds)
  }
}
