import NetworkExtension
import XCTest
@testable import TunnelControlHarness

final class PacketTunnelManagerTests: XCTestCase {
  func testApplySavesSerializedConfiguration() throws {
    let record = FakePacketTunnelManagerRecord()
    record.localizedDescription = "SLAN PacketTunnel"
    let manager = PacketTunnelManager(loader: FakePacketTunnelManagerLoader(primaryRecord: record))

    try manager.apply(configuration: makeConfiguration())

    XCTAssertEqual(record.localizedDescription, "SLAN PacketTunnel")
    XCTAssertTrue(record.isEnabled)
    XCTAssertEqual(record.saveCallCount, 1)
    XCTAssertEqual(record.loadCallCount, 2)
    XCTAssertEqual(record.protocolConfiguration?.providerBundleIdentifier, "com.example.slanApp.PacketTunnel")
    XCTAssertEqual(record.protocolConfiguration?.serverAddress, "203.0.113.10:51820")
    let providerConfiguration = record.protocolConfiguration?.providerConfiguration
    XCTAssertEqual(providerConfiguration?["transport"] as? String, "relay")
    XCTAssertEqual(providerConfiguration?["localVirtualIp"] as? String, "100.64.0.10")
    XCTAssertEqual(providerConfiguration?["peerVirtualIp"] as? String, "100.64.0.2")
  }

  func testApplySerializesDebugEngineModeOverride() throws {
    let record = FakePacketTunnelManagerRecord()
    record.localizedDescription = "SLAN PacketTunnel"
    let manager = PacketTunnelManager(
      loader: FakePacketTunnelManagerLoader(primaryRecord: record),
      debugEngineModeOverride: "loopback"
    )

    try manager.apply(configuration: makeConfiguration())

    let providerConfiguration = record.protocolConfiguration?.providerConfiguration
    XCTAssertEqual(providerConfiguration?["debugEngineMode"] as? String, "loopback")
  }

  func testApplySerializesConfigurationDebugEngineModeWithoutOverride() throws {
    let record = FakePacketTunnelManagerRecord()
    record.localizedDescription = "SLAN PacketTunnel"
    let manager = PacketTunnelManager(
      loader: FakePacketTunnelManagerLoader(primaryRecord: record),
      debugEngineModeOverride: nil
    )
    var configuration = makeConfiguration()
    configuration = WireGuardTunnelConfiguration(
      transport: configuration.transport,
      localVirtualIp: configuration.localVirtualIp,
      peerVirtualIp: configuration.peerVirtualIp,
      debugEngineMode: "loopback",
      interface: configuration.interface,
      peer: configuration.peer
    )

    try manager.apply(configuration: configuration)

    let providerConfiguration = record.protocolConfiguration?.providerConfiguration
    XCTAssertEqual(providerConfiguration?["debugEngineMode"] as? String, "loopback")
  }

  func testCurrentConfigurationParsesStoredProviderConfiguration() throws {
    let record = FakePacketTunnelManagerRecord()
    let configuration = makeConfiguration()
    record.localizedDescription = "SLAN PacketTunnel"
    record.protocolConfiguration = serializedProtocol(from: configuration)
    let manager = PacketTunnelManager(loader: FakePacketTunnelManagerLoader(primaryRecord: record))

    let loaded = try manager.currentConfiguration()

    XCTAssertEqual(loaded, configuration)
  }

  func testBringUpEnablesAndStartsTunnel() throws {
    let record = FakePacketTunnelManagerRecord()
    record.localizedDescription = "SLAN PacketTunnel"
    record.protocolConfiguration = serializedProtocol(from: makeConfiguration())
    record.isEnabled = false
    let manager = PacketTunnelManager(loader: FakePacketTunnelManagerLoader(primaryRecord: record))

    try manager.bringUp()

    XCTAssertTrue(record.isEnabled)
    XCTAssertEqual(record.saveCallCount, 1)
    XCTAssertEqual(record.startCallCount, 1)
  }

  func testBringDownStopsTunnel() throws {
    let record = FakePacketTunnelManagerRecord()
    record.localizedDescription = "SLAN PacketTunnel"
    let manager = PacketTunnelManager(loader: FakePacketTunnelManagerLoader(primaryRecord: record))

    try manager.bringDown()

    XCTAssertEqual(record.stopCallCount, 1)
  }

  func testRuntimePayloadReturnsProviderMessageJson() throws {
    let record = FakePacketTunnelManagerRecord()
    record.localizedDescription = "SLAN PacketTunnel"
    record.providerMessageResponse = try JSONSerialization.data(
      withJSONObject: ["state": "connected", "selectedEndpoint": "198.51.100.10:51820"],
      options: []
    )
    let manager = PacketTunnelManager(loader: FakePacketTunnelManagerLoader(primaryRecord: record))

    let payload = try manager.runtimePayload()

    XCTAssertEqual(record.sendProviderMessageCallCount, 1)
    XCTAssertEqual(payload?["state"] as? String, "connected")
    XCTAssertEqual(payload?["selectedEndpoint"] as? String, "198.51.100.10:51820")
  }

  func testConnectionStatusDelegatesToRecord() throws {
    let record = FakePacketTunnelManagerRecord()
    record.localizedDescription = "SLAN PacketTunnel"
    record.status = .disconnecting
    let manager = PacketTunnelManager(loader: FakePacketTunnelManagerLoader(primaryRecord: record))

    let status = try manager.connectionStatus()

    XCTAssertEqual(status, .disconnecting)
  }

  func testLoadManagersFailureMapsToPluginError() {
    let manager = PacketTunnelManager(
      loader: FakePacketTunnelManagerLoader(loadError: NSError(domain: "test", code: 1))
    )

    XCTAssertThrowsError(try manager.connectionStatus()) { error in
      let pluginError = error as? SlanAppCorePluginError
      XCTAssertEqual(pluginError?.code, "app_core_tunnel_backend_unavailable")
      XCTAssertTrue(pluginError?.message.contains("Failed to load packet tunnel managers") == true)
    }
  }
}

private final class FakePacketTunnelManagerLoader: PacketTunnelManagerLoading {
  private let primaryRecord: PacketTunnelManagerRecord
  private let loadError: Error?

  init(primaryRecord: PacketTunnelManagerRecord = FakePacketTunnelManagerRecord(), loadError: Error? = nil) {
    self.primaryRecord = primaryRecord
    self.loadError = loadError
  }

  func loadAllFromPreferences(completion: @escaping ([PacketTunnelManagerRecord]?, Error?) -> Void) {
    completion([primaryRecord], loadError)
  }

  func makeManager() -> PacketTunnelManagerRecord {
    primaryRecord
  }
}

private final class FakePacketTunnelManagerRecord: PacketTunnelManagerRecord {
  var localizedDescription: String?
  var protocolConfiguration: NETunnelProviderProtocol?
  var isEnabled = false
  var status: NEVPNStatus = .disconnected
  var providerMessageResponse: Data?
  var saveCallCount = 0
  var loadCallCount = 0
  var startCallCount = 0
  var stopCallCount = 0
  var sendProviderMessageCallCount = 0

  var connectionStatus: NEVPNStatus { status }

  func saveToPreferences(completion: @escaping (Error?) -> Void) {
    saveCallCount += 1
    completion(nil)
  }

  func loadFromPreferences(completion: @escaping (Error?) -> Void) {
    loadCallCount += 1
    completion(nil)
  }

  func startVPNTunnel() throws {
    startCallCount += 1
  }

  func stopVPNTunnel() {
    stopCallCount += 1
  }

  func sendProviderMessage(_ messageData: Data, completionHandler: @escaping (Data?) -> Void) throws {
    sendProviderMessageCallCount += 1
    completionHandler(providerMessageResponse)
  }
}

private func serializedProtocol(from configuration: WireGuardTunnelConfiguration) -> NETunnelProviderProtocol {
  let tunnelProtocol = NETunnelProviderProtocol()
  tunnelProtocol.providerBundleIdentifier = "com.example.slanApp.PacketTunnel"
  tunnelProtocol.serverAddress = configuration.peer.endpoint ?? configuration.peerVirtualIp
  tunnelProtocol.providerConfiguration = [
    "transport": configuration.transport,
    "localVirtualIp": configuration.localVirtualIp,
    "peerVirtualIp": configuration.peerVirtualIp,
    "debugEngineMode": configuration.debugEngineMode as Any,
    "wireguardInterface": [
      "interfaceName": configuration.interface.interfaceName as Any,
      "keyPair": [
        "publicKey": configuration.interface.keyPair.publicKey,
        "privateKey": configuration.interface.keyPair.privateKey,
      ],
      "listenPort": configuration.interface.listenPort as Any,
      "mtu": configuration.interface.mtu as Any,
      "addresses": configuration.interface.addresses,
      "dnsServers": configuration.interface.dnsServers,
    ],
    "wireguardPeer": [
      "peerNodeId": configuration.peer.peerNodeId as Any,
      "publicKey": configuration.peer.publicKey,
      "presharedKey": configuration.peer.presharedKey as Any,
      "endpoint": configuration.peer.endpoint as Any,
      "allowedIps": configuration.peer.allowedIps.map { ["cidr": $0] },
      "persistentKeepaliveSeconds": configuration.peer.persistentKeepaliveSeconds as Any,
    ],
  ]
  return tunnelProtocol
}
