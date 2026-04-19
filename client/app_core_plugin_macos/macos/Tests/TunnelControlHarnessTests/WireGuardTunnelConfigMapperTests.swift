import XCTest
@testable import TunnelControlHarness

final class WireGuardTunnelConfigMapperTests: XCTestCase {
  func testMapsValidConfiguration() throws {
    let configuration = try WireGuardTunnelConfigMapper.mapConfiguration([
      "transport": "relay",
      "localVirtualIp": "100.64.0.10",
      "peerVirtualIp": "100.64.0.2",
      "debugEngineMode": "loopback",
      "wireguardInterface": [
        "interfaceName": "utun9",
        "keyPair": [
          "publicKey": "pub",
          "privateKey": "priv",
        ],
        "listenPort": 51820,
        "mtu": 1280,
        "addresses": ["100.64.0.10/32"],
        "dnsServers": ["1.1.1.1", "8.8.8.8"],
      ],
      "wireguardPeer": [
        "peerNodeId": "peer-1",
        "publicKey": "peer-pub",
        "endpoint": "203.0.113.10:51820",
        "allowedIps": [
          ["cidr": "100.64.0.2/32"],
          ["cidr": "10.0.0.0/24"],
        ],
        "persistentKeepaliveSeconds": 25,
      ],
    ])

    XCTAssertEqual(configuration.transport, "relay")
    XCTAssertEqual(configuration.localVirtualIp, "100.64.0.10")
    XCTAssertEqual(configuration.peerVirtualIp, "100.64.0.2")
    XCTAssertEqual(configuration.debugEngineMode, "loopback")
    XCTAssertEqual(configuration.interface.interfaceName, "utun9")
    XCTAssertEqual(configuration.interface.keyPair.publicKey, "pub")
    XCTAssertEqual(configuration.interface.keyPair.privateKey, "priv")
    XCTAssertEqual(configuration.interface.listenPort, 51820)
    XCTAssertEqual(configuration.interface.mtu, 1280)
    XCTAssertEqual(configuration.interface.addresses, ["100.64.0.10/32"])
    XCTAssertEqual(configuration.interface.dnsServers, ["1.1.1.1", "8.8.8.8"])
    XCTAssertEqual(configuration.peer.peerNodeId, "peer-1")
    XCTAssertEqual(configuration.peer.publicKey, "peer-pub")
    XCTAssertEqual(configuration.peer.endpoint, "203.0.113.10:51820")
    XCTAssertEqual(configuration.peer.allowedIps, ["100.64.0.2/32", "10.0.0.0/24"])
    XCTAssertEqual(configuration.peer.persistentKeepaliveSeconds, 25)
  }

  func testRejectsInvalidAllowedIpObject() {
    XCTAssertThrowsError(
      try WireGuardTunnelConfigMapper.mapConfiguration([
        "transport": "relay",
        "localVirtualIp": "100.64.0.10",
        "peerVirtualIp": "100.64.0.2",
        "wireguardInterface": [
          "keyPair": ["publicKey": "pub", "privateKey": "priv"],
          "addresses": ["100.64.0.10/32"],
        ],
        "wireguardPeer": [
          "publicKey": "peer-pub",
          "allowedIps": ["100.64.0.2/32"],
        ],
      ])
    ) { error in
      let pluginError = error as? SlanAppCorePluginError
      XCTAssertEqual(pluginError?.code, "app_core_invalid_tunnel_config")
      XCTAssertTrue(pluginError?.message.contains("allowedIps[0]") == true)
    }
  }
}
