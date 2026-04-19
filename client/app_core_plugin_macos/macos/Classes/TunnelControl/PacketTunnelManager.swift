import Foundation
import NetworkExtension

final class PacketTunnelManager: PacketTunnelManaging {
  private let loader: PacketTunnelManagerLoading
  private let managerDescription = "SLAN PacketTunnel"
  private let providerBundleIdentifier = "com.example.slanApp.PacketTunnel"
  private let debugEngineModeOverride: String?

  init(
    loader: PacketTunnelManagerLoading = SystemPacketTunnelManagerLoader(),
    debugEngineModeOverride: String? = ProcessInfo.processInfo.environment["SLAN_PACKET_TUNNEL_DEBUG_ENGINE_MODE"]
  ) {
    self.loader = loader
    self.debugEngineModeOverride = debugEngineModeOverride
  }

  func apply(configuration: WireGuardTunnelConfiguration) throws {
    let manager = try loadOrCreateManager()
    let tunnelProtocol =
      manager.protocolConfiguration ?? NETunnelProviderProtocol()
    tunnelProtocol.providerBundleIdentifier = providerBundleIdentifier
    tunnelProtocol.serverAddress = configuration.peer.endpoint ?? configuration.peerVirtualIp
    tunnelProtocol.providerConfiguration = serialize(configuration: configuration)
    manager.localizedDescription = managerDescription
    manager.protocolConfiguration = tunnelProtocol
    manager.isEnabled = true
    try save(manager: manager)
  }

  func clearConfiguration() throws {
    let manager = try loadOrCreateManager()
    manager.protocolConfiguration = nil
    manager.isEnabled = false
    try save(manager: manager)
  }

  func bringUp() throws {
    let manager = try loadOrCreateManager()
    if !manager.isEnabled {
      manager.isEnabled = true
      try save(manager: manager)
    }
    do {
      try manager.startVPNTunnel()
    } catch {
      throw SlanAppCorePluginError(
        code: "app_core_tunnel_backend_unavailable",
        message: "Failed to start packet tunnel: \(error.localizedDescription)"
      )
    }
  }

  func bringDown() throws {
    let manager = try loadOrCreateManager()
    manager.stopVPNTunnel()
  }

  func currentConfiguration() throws -> WireGuardTunnelConfiguration? {
    let manager = try loadOrCreateManager()
    guard let providerConfiguration = manager.protocolConfiguration?.providerConfiguration else {
      return nil
    }
    return try? WireGuardTunnelConfigMapper.mapConfiguration(providerConfiguration)
  }

  func runtimePayload() throws -> [String: Any]? {
    let manager = try loadOrCreateManager()
    let request = try JSONSerialization.data(withJSONObject: ["method": "runtimeView"], options: [])
    let semaphore = DispatchSemaphore(value: 0)
    var responseData: Data?
    do {
      try manager.sendProviderMessage(request) { data in
        responseData = data
        semaphore.signal()
      }
    } catch {
      throw SlanAppCorePluginError(
        code: "app_core_tunnel_backend_unavailable",
        message: "Failed to message packet tunnel provider: \(error.localizedDescription)"
      )
    }
    _ = semaphore.wait(timeout: .now() + .seconds(5))
    guard let responseData else {
      return nil
    }
    let object = try JSONSerialization.jsonObject(with: responseData, options: [])
    return object as? [String: Any]
  }

  func connectionStatus() throws -> NEVPNStatus {
    try loadOrCreateManager().connectionStatus
  }

  private func loadOrCreateManager() throws -> PacketTunnelManagerRecord {
    if let existing = try loadManagers().first(where: { $0.localizedDescription == managerDescription }) {
      try reload(manager: existing)
      return existing
    }
    let manager = loader.makeManager()
    manager.localizedDescription = managerDescription
    return manager
  }

  private func loadManagers() throws -> [PacketTunnelManagerRecord] {
    let semaphore = DispatchSemaphore(value: 0)
    var result: Result<[PacketTunnelManagerRecord], Error>?
    loader.loadAllFromPreferences { managers, error in
      if let error {
        result = .failure(error)
      } else {
        result = .success(managers ?? [])
      }
      semaphore.signal()
    }
    _ = semaphore.wait(timeout: .now() + .seconds(10))
    switch result {
    case .success(let managers):
      return managers
    case .failure(let error):
      throw SlanAppCorePluginError(
        code: "app_core_tunnel_backend_unavailable",
        message: "Failed to load packet tunnel managers: \(error.localizedDescription)"
      )
    case .none:
      throw SlanAppCorePluginError(
        code: "app_core_tunnel_backend_unavailable",
        message: "Timed out loading packet tunnel managers"
      )
    }
  }

  private func save(manager: PacketTunnelManagerRecord) throws {
    let semaphore = DispatchSemaphore(value: 0)
    var savedError: Error?
    manager.saveToPreferences { error in
      savedError = error
      semaphore.signal()
    }
    _ = semaphore.wait(timeout: .now() + .seconds(10))
    if let savedError {
      throw SlanAppCorePluginError(
        code: "app_core_tunnel_backend_unavailable",
        message: "Failed to save packet tunnel manager: \(savedError.localizedDescription)"
      )
    }
    try reload(manager: manager)
  }

  private func reload(manager: PacketTunnelManagerRecord) throws {
    let semaphore = DispatchSemaphore(value: 0)
    var loadError: Error?
    manager.loadFromPreferences { error in
      loadError = error
      semaphore.signal()
    }
    _ = semaphore.wait(timeout: .now() + .seconds(10))
    if let loadError {
      throw SlanAppCorePluginError(
        code: "app_core_tunnel_backend_unavailable",
        message: "Failed to reload packet tunnel manager: \(loadError.localizedDescription)"
      )
    }
  }

  private func serialize(configuration: WireGuardTunnelConfiguration) -> [String: Any] {
    var json: [String: Any] = [
      "transport": configuration.transport,
      "localVirtualIp": configuration.localVirtualIp,
      "peerVirtualIp": configuration.peerVirtualIp,
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
    let selectedDebugEngineMode = debugEngineModeOverride?.isEmpty == false
      ? debugEngineModeOverride
      : configuration.debugEngineMode
    if let selectedDebugEngineMode, !selectedDebugEngineMode.isEmpty {
      json["debugEngineMode"] = selectedDebugEngineMode
    }
    return json
  }
}

final class SystemPacketTunnelManagerLoader: PacketTunnelManagerLoading {
  func loadAllFromPreferences(completion: @escaping ([PacketTunnelManagerRecord]?, Error?) -> Void) {
    NETunnelProviderManager.loadAllFromPreferences { managers, error in
      completion(managers?.map(SystemPacketTunnelManagerRecord.init), error)
    }
  }

  func makeManager() -> PacketTunnelManagerRecord {
    SystemPacketTunnelManagerRecord(NETunnelProviderManager())
  }
}

final class SystemPacketTunnelManagerRecord: PacketTunnelManagerRecord {
  private let manager: NETunnelProviderManager

  init(_ manager: NETunnelProviderManager) {
    self.manager = manager
  }

  var localizedDescription: String? {
    get { manager.localizedDescription }
    set { manager.localizedDescription = newValue }
  }

  var protocolConfiguration: NETunnelProviderProtocol? {
    get { manager.protocolConfiguration as? NETunnelProviderProtocol }
    set { manager.protocolConfiguration = newValue }
  }

  var isEnabled: Bool {
    get { manager.isEnabled }
    set { manager.isEnabled = newValue }
  }

  var connectionStatus: NEVPNStatus {
    manager.connection.status
  }

  func saveToPreferences(completion: @escaping (Error?) -> Void) {
    manager.saveToPreferences(completionHandler: completion)
  }

  func loadFromPreferences(completion: @escaping (Error?) -> Void) {
    manager.loadFromPreferences(completionHandler: completion)
  }

  func startVPNTunnel() throws {
    try manager.connection.startVPNTunnel()
  }

  func stopVPNTunnel() {
    manager.connection.stopVPNTunnel()
  }

  func sendProviderMessage(_ messageData: Data, completionHandler: @escaping (Data?) -> Void) throws {
    guard let session = manager.connection as? NETunnelProviderSession else {
      completionHandler(nil)
      return
    }
    try session.sendProviderMessage(messageData, responseHandler: completionHandler)
  }
}
