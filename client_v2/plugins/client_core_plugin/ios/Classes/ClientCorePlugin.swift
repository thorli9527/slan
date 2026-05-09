import Flutter
import NetworkExtension
import UIKit

#if SLAN_CLIENT_CORE_FFI
@_silgen_name("client_core_v2_service_request_json")
private func clientCoreV2ServiceRequestJson(
  _ requestJson: UnsafePointer<CChar>
) -> UnsafeMutablePointer<CChar>?

@_silgen_name("client_core_v2_free_string")
private func clientCoreV2FreeString(_ value: UnsafeMutablePointer<CChar>?)
#endif

public class ClientCorePlugin: NSObject, FlutterPlugin {
  private static let deviceIdKey = "dev.slan.client.v2.ios.deviceId"
  private static let nodeIdKey = "dev.slan.client.v2.ios.nodeId"
  private static let serverBaseUrlKey = "dev.slan.client.v2.mobile.serverBaseUrl"
  private static let packetTunnelProviderBundleId = "dev.slan.client.v2.SLANPacketTunnel"
  private static let packetTunnelDescription = "SLAN Packet Tunnel"

  private var packetTunnelManager: NETunnelProviderManager?
  private var state: [String: Any?] = [
    "signedIn": false,
    "userLabel": nil,
    "deviceId": nil,
    "nodeId": nil,
    "virtualIp": nil,
    "networkEnabled": false,
    "syncing": false,
    "syncReason": nil,
    "switchEnabled": true,
    "notice": "iosClientReady",
    "error": nil,
    "lastClientMessageId": nil,
    "lastClientMessageFromDeviceId": nil,
    "lastClientMessageBody": nil
  ]

  public static func register(with registrar: FlutterPluginRegistrar) {
    let channel = FlutterMethodChannel(
      name: "dev.slan/client_core_v2",
      binaryMessenger: registrar.messenger()
    )
    let plugin = ClientCorePlugin()
    plugin.state["deviceId"] = plugin.stableDeviceId()
    plugin.state["nodeId"] = plugin.stableNodeId()
    registrar.addMethodCallDelegate(plugin, channel: channel)
  }

  public func handle(_ call: FlutterMethodCall, result: @escaping FlutterResult) {
    switch call.method {
    case "iosPacketTunnelStats":
      iosPacketTunnelStats(result: result)
    case "iosSharedStoreDiagnostics":
      result(SLANIosSharedStore.diagnostics())
    case "iosStartPacketTunnel":
      iosStartPacketTunnel(call.arguments, result: result)
    case "iosStopPacketTunnel":
      iosStopPacketTunnel(result: result)
    case "embeddedServiceRequest":
      result(handleEmbeddedServiceRequest(call.arguments))
    case "mobileServerBaseUrl":
      result(mobileServerBaseUrl())
    case "setMobileServerBaseUrl":
      setMobileServerBaseUrl(call.arguments as? String ?? "")
      result(true)
    default:
      result(FlutterMethodNotImplemented)
    }
  }

  private func compactState() -> [String: Any] {
    var output: [String: Any] = [:]
    for (key, value) in state {
      if let value = value {
        output[key] = value
      }
    }
    return output
  }

  private func handleEmbeddedServiceRequest(_ arguments: Any?) -> Any {
    guard let requestJson = arguments as? String,
      !requestJson.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty
    else {
      return ["error": "embedded service request json is empty"]
    }
#if SLAN_CLIENT_CORE_FFI
    guard let responsePointer = requestJson.withCString({ pointer in
      clientCoreV2ServiceRequestJson(pointer)
    }) else {
      return ["error": "embedded service returned null response"]
    }
    defer {
      clientCoreV2FreeString(responsePointer)
    }
    let responseJson = String(cString: responsePointer)
    guard let data = responseJson.data(using: .utf8),
      let object = try? JSONSerialization.jsonObject(with: data)
    else {
      return ["error": "embedded service returned invalid json"]
    }
    return object
#else
    return ["error": "ios client-core-service embedded FFI is not linked yet"]
#endif
  }

  private func iosStartPacketTunnel(_ arguments: Any?, result: @escaping FlutterResult) {
    guard var config = arguments as? [String: Any] else {
      result([
        "networkEnabled": false,
        "syncing": false,
        "switchEnabled": true,
        "error": "ios packet tunnel config is missing"
      ])
      return
    }
    if config["sessionName"] == nil {
      config["sessionName"] = "SLAN"
    }
    SLANIosSharedStore.writeNetworkConfig(config)
    startPacketTunnel(config: config) { [weak self] startResult in
      guard let self = self else { return }
      DispatchQueue.main.async {
        switch startResult {
        case .failure(let error):
          self.state["networkEnabled"] = false
          self.state["syncing"] = false
          self.state["switchEnabled"] = true
          self.state["notice"] = "iosPacketTunnelStartFailed"
          self.state["error"] = error.localizedDescription
        case .success:
          self.state["virtualIp"] = self.stringField(config, "virtualIp")
          self.state["networkEnabled"] = true
          self.state["syncing"] = false
          self.state["switchEnabled"] = true
          self.state["notice"] = "networkEnabled"
          self.state["error"] = nil
        }
        result(self.compactState())
      }
    }
  }

  private func iosStopPacketTunnel(result: @escaping FlutterResult) {
    stopPacketTunnel { [weak self] error in
      guard let self = self else { return }
      DispatchQueue.main.async {
        self.state["networkEnabled"] = false
        self.state["virtualIp"] = nil
        self.state["syncing"] = false
        self.state["switchEnabled"] = true
        self.state["notice"] = "networkDisabled"
        self.state["error"] = error?.localizedDescription
        result(self.compactState())
      }
    }
  }

  private func startPacketTunnel(
    config: [String: Any],
    completion: @escaping (Result<Void, Error>) -> Void
  ) {
#if targetEnvironment(simulator)
    writeSimulatorPacketTunnelStats(config: config, enabled: true)
    completion(.success(()))
    return
#else
    loadPacketTunnelManager { [weak self] result in
      guard let self = self else { return }
      switch result {
      case .failure(let error):
        completion(.failure(error))
      case .success(let manager):
        let tunnelProtocol = NETunnelProviderProtocol()
        tunnelProtocol.providerBundleIdentifier = Self.packetTunnelProviderBundleId
        tunnelProtocol.serverAddress = "SLAN"
        tunnelProtocol.providerConfiguration = self.propertyListConfig(config)
        manager.localizedDescription = Self.packetTunnelDescription
        manager.protocolConfiguration = tunnelProtocol
        manager.isEnabled = true
        manager.saveToPreferences { error in
          if let error = error {
            completion(.failure(error))
            return
          }
          manager.loadFromPreferences { error in
            if let error = error {
              completion(.failure(error))
              return
            }
            do {
              try manager.connection.startVPNTunnel()
              self.packetTunnelManager = manager
              completion(.success(()))
            } catch {
              completion(.failure(error))
            }
          }
        }
      }
    }
#endif
  }

  private func stopPacketTunnel(completion: @escaping (Error?) -> Void) {
#if targetEnvironment(simulator)
    writeSimulatorPacketTunnelStats(config: SLANIosSharedStore.readNetworkConfig() ?? [:], enabled: false)
    completion(nil)
    return
#else
    loadPacketTunnelManager { [weak self] result in
      switch result {
      case .failure(let error):
        completion(error)
      case .success(let manager):
        manager.connection.stopVPNTunnel()
        self?.packetTunnelManager = manager
        completion(nil)
      }
    }
#endif
  }

  private func iosPacketTunnelStats(result: @escaping FlutterResult) {
#if targetEnvironment(simulator)
    result(SLANIosSharedStore.readPacketTunnelStats() ?? [:])
    return
#else
    loadPacketTunnelManager { managerResult in
      switch managerResult {
      case .failure(let error):
        result(SLANIosSharedStore.readPacketTunnelStats() ?? [
          "error": error.localizedDescription
        ])
      case .success(let manager):
        guard let session = manager.connection as? NETunnelProviderSession else {
          result(SLANIosSharedStore.readPacketTunnelStats() ?? [:])
          return
        }
        do {
          try session.sendProviderMessage(Data("stats".utf8)) { data in
            guard let data = data,
              let value = try? JSONSerialization.jsonObject(with: data) as? [String: Any]
            else {
              result(SLANIosSharedStore.readPacketTunnelStats() ?? [:])
              return
            }
            result(value)
          }
        } catch {
          result(SLANIosSharedStore.readPacketTunnelStats() ?? [
            "error": error.localizedDescription
          ])
        }
      }
    }
#endif
  }

  private func loadPacketTunnelManager(
    completion: @escaping (Result<NETunnelProviderManager, Error>) -> Void
  ) {
    NETunnelProviderManager.loadAllFromPreferences { [weak self] managers, error in
      if let error = error {
        completion(.failure(error))
        return
      }
      let manager = managers?.first(where: {
        $0.localizedDescription == Self.packetTunnelDescription
      }) ?? NETunnelProviderManager()
      self?.packetTunnelManager = manager
      completion(.success(manager))
    }
  }

  private func propertyListConfig(_ config: [String: Any]) -> [String: NSObject] {
    var out: [String: NSObject] = [:]
    for (key, value) in config {
      if let value = propertyListObject(value) {
        out[key] = value
      }
    }
    return out
  }

  private func propertyListObject(_ value: Any) -> NSObject? {
    if let value = value as? String {
      return value as NSString
    }
    if let value = value as? Int {
      return NSNumber(value: value)
    }
    if let value = value as? Int64 {
      return NSNumber(value: value)
    }
    if let value = value as? Bool {
      return NSNumber(value: value)
    }
    if let value = value as? NSNumber {
      return value
    }
    if let value = value as? [String] {
      return value as NSArray
    }
    if let value = value as? [Any] {
      return value.compactMap { propertyListObject($0) } as NSArray
    }
    if let value = value as? [String: Any] {
      return propertyListDictionary(value)
    }
    return nil
  }

  private func propertyListDictionary(_ value: [String: Any]) -> NSDictionary {
    var out: [String: NSObject] = [:]
    for (key, item) in value {
      if let item = propertyListObject(item) {
        out[key] = item
      }
    }
    return out as NSDictionary
  }

  private func stableDeviceId() -> String {
    let defaults = UserDefaults.standard
    if let existing = defaults.string(forKey: Self.deviceIdKey),
      !existing.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty
    {
      return existing
    }
    let value = "ios-\(UUID().uuidString.lowercased())"
    defaults.set(value, forKey: Self.deviceIdKey)
    return value
  }

  private func stableNodeId() -> String {
    let defaults = UserDefaults.standard
    if let existing = defaults.string(forKey: Self.nodeIdKey),
      !existing.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty
    {
      return existing
    }
    let value = "node-ios-\(UUID().uuidString.lowercased())"
    defaults.set(value, forKey: Self.nodeIdKey)
    return value
  }

  private func mobileServerBaseUrl() -> String {
    UserDefaults.standard.string(forKey: Self.serverBaseUrlKey)?
      .trimmingCharacters(in: .whitespacesAndNewlines) ?? ""
  }

  private func setMobileServerBaseUrl(_ value: String) {
    UserDefaults.standard.set(
      value.trimmingCharacters(in: .whitespacesAndNewlines),
      forKey: Self.serverBaseUrlKey
    )
  }

  private func stringField(_ object: [String: Any], _ field: String) -> String {
    return (object[field] as? String)?.trimmingCharacters(in: .whitespacesAndNewlines) ?? ""
  }

  private func writeSimulatorPacketTunnelStats(config: [String: Any], enabled: Bool) {
    let now = Int64(Date().timeIntervalSince1970 * 1000)
    let virtualIp = stringField(config, "virtualIp")
    SLANIosSharedStore.writePacketTunnelStats([
      "simulatorFallback": true,
      "networkEnabled": enabled,
      "virtualIp": enabled ? virtualIp : "",
      "packetsRead": 0,
      "bytesRead": 0,
      "bytesWritten": 0,
      "routedPackets": 0,
      "unroutablePackets": 0,
      "nonIpv4Packets": 0,
      "relayFramesSent": 0,
      "relayFramesReceived": 0,
      "startedAtMs": enabled ? now : 0,
      "stoppedAtMs": enabled ? 0 : now,
      "updatedAtMs": now
    ])
  }
}
