import XCTest
@testable import TunnelControlHarness

final class WireGuardKitBackendConfigMapperTests: XCTestCase {
  func testMapCarriesProviderConfigurationIntoBackendShape() throws {
    let configuration = try PacketTunnelProviderConfiguration(
      providerConfiguration: [
        "localVirtualIp": "100.64.0.10",
        "peerVirtualIp": "100.64.0.2",
        "debugEngineMode": "external",
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
    )

    let backendConfiguration = WireGuardKitBackendConfigMapper.map(configuration)

    XCTAssertEqual(backendConfiguration.transport, "relay")
    XCTAssertEqual(backendConfiguration.debugEngineMode, "external")
    XCTAssertEqual(backendConfiguration.localVirtualIp, "100.64.0.10")
    XCTAssertEqual(backendConfiguration.peerVirtualIp, "100.64.0.2")
    XCTAssertEqual(backendConfiguration.interfaceAddresses, ["100.64.0.10"])
    XCTAssertEqual(backendConfiguration.dnsServers, ["1.1.1.1", "8.8.8.8"])
    XCTAssertEqual(backendConfiguration.mtu, 1280)
    XCTAssertEqual(backendConfiguration.endpoint, "203.0.113.10:51820")
    XCTAssertEqual(backendConfiguration.allowedIps, ["100.64.0.2/32", "10.0.0.0/24"])
    XCTAssertEqual(backendConfiguration.interfacePrivateKey, "pending-wireguardkit-private-key")
    XCTAssertEqual(backendConfiguration.interfacePublicKey, "pending-wireguardkit-public-key")
    XCTAssertEqual(backendConfiguration.peerPublicKey, "pending-wireguardkit-peer-public-key")
  }
}
