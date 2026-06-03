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

/// ClientCorePlugin 是 iOS Flutter 插件入口。
///
/// 它桥接 Flutter MethodChannel、内嵌 Rust client-core-service FFI、iOS
/// PacketTunnel 启停和 App Group 共享存储。
public class ClientCorePlugin: NSObject, FlutterPlugin {
  private static let deviceIdKey = "dev.slan.client.v2.ios.deviceId"
  private static let nodeIdKey = "dev.slan.client.v2.ios.nodeId"
  private static let serverBaseUrlKey = "dev.slan.client.v2.mobile.serverBaseUrl"
  private static let packetTunnelProviderBundleId = "dev.slan.client.v2.SLANPacketTunnel"
  private static let packetTunnelDescription = "SLAN Packet Tunnel"

  private var packetTunnelManager: NETunnelProviderManager?
  private var pendingNetworkEventResult: FlutterResult?
  private var pendingNetworkEventToken = 0
  private var networkEvents: [[String: Any]] = []
  /// state 是返回给 Flutter UI 的轻量运行状态缓存。
  private var state: [String: Any?] = [
    "signedIn": false,
    "userLabel": nil,
    "deviceId": nil,
    "nodeId": nil,
    "adapterPresent": false,
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

  /// 注册 Flutter MethodChannel，并初始化稳定 device/node ID。
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

  /// 分发 Flutter 侧调用到 iOS 原生能力。
  public func handle(_ call: FlutterMethodCall, result: @escaping FlutterResult) {
    switch call.method {
    case "iosPacketTunnelStats":
      iosPacketTunnelStats(result: result)
    case "iosSharedStoreDiagnostics":
      result(SLANIosSharedStore.diagnostics())
    case "iosRuntimeState":
      result(iosRuntimeState())
    case "iosWatchNetworkEvent":
      iosWatchNetworkEvent(result: result)
    case "iosStartPacketTunnel":
      iosStartPacketTunnel(call.arguments, result: result)
    case "iosStopPacketTunnel":
      iosStopPacketTunnel(result: result)
    case "embeddedServiceRequest":
      result(handleEmbeddedServiceRequest(embeddedServiceRequestJson(call.arguments)))
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

  private func embeddedServiceRequestJson(_ arguments: Any?) -> String {
    let requestJson = (arguments as? String) ?? "{}"
    guard let data = requestJson.data(using: .utf8),
      var request = (try? JSONSerialization.jsonObject(with: data)) as? [String: Any]
    else {
      return requestJson
    }
    var args = request["args"] as? [String: Any] ?? [:]
    args["deviceId"] = stableDeviceId()
    if let stateDir = FileManager.default.urls(
      for: .applicationSupportDirectory,
      in: .userDomainMask
    ).first?.path {
      args["stateDir"] = stateDir
    }
    request["args"] = args
    guard let encoded = try? JSONSerialization.data(withJSONObject: request),
      let output = String(data: encoded, encoding: .utf8)
    else {
      return requestJson
    }
    return output
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
          self.pushNetworkEvent(
            eventType: "error",
            message: error.localizedDescription,
            runtimeState: self.iosRuntimeState()
          )
        case .success:
          self.state["virtualIp"] = self.stringField(config, "virtualIp")
          self.state["adapterPresent"] = true
          self.state["networkEnabled"] = true
          self.state["syncing"] = false
          self.state["switchEnabled"] = true
          self.state["notice"] = "networkEnabled"
          self.state["error"] = nil
          self.pushNetworkEvent(
            eventType: "vpnStarted",
            message: "iOS PacketTunnel started",
            runtimeState: self.iosRuntimeState()
          )
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
        self.state["adapterPresent"] = self.packetTunnelManager != nil
        self.state["virtualIp"] = nil
        self.state["syncing"] = false
        self.state["switchEnabled"] = true
        self.state["notice"] = "networkDisabled"
        self.state["error"] = error?.localizedDescription
        self.pushNetworkEvent(
          eventType: error == nil ? "vpnStopped" : "error",
          message: error?.localizedDescription ?? "iOS PacketTunnel stopped",
          runtimeState: self.iosRuntimeState()
        )
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

  private func iosWatchNetworkEvent(result: @escaping FlutterResult) {
    DispatchQueue.main.async { [weak self] in
      guard let self = self else {
        result(nil)
        return
      }
      if !self.networkEvents.isEmpty {
        result(self.networkEvents.removeFirst())
        return
      }
      if self.pendingNetworkEventResult != nil {
        self.pendingNetworkEventResult?(nil)
      }
      self.pendingNetworkEventToken += 1
      let token = self.pendingNetworkEventToken
      self.pendingNetworkEventResult = result
      DispatchQueue.main.asyncAfter(deadline: .now() + 1.0) { [weak self] in
        guard let self = self,
          self.pendingNetworkEventToken == token,
          let pending = self.pendingNetworkEventResult
        else {
          return
        }
        self.pendingNetworkEventResult = nil
        if self.networkEvents.isEmpty {
          pending(nil)
        } else {
          pending(self.networkEvents.removeFirst())
        }
      }
    }
  }

  private func pushNetworkEvent(
    eventType: String,
    message: String?,
    runtimeState: [String: Any]
  ) {
    var event: [String: Any] = [
      "eventType": eventType,
      "runtimeState": runtimeState
    ]
    if let message = message, !message.isEmpty {
      event["message"] = message
    }
    DispatchQueue.main.async { [weak self] in
      guard let self = self else {
        return
      }
      if let pending = self.pendingNetworkEventResult {
        self.pendingNetworkEventResult = nil
        self.pendingNetworkEventToken += 1
        pending(event)
        return
      }
      self.networkEvents.append(event)
      while self.networkEvents.count > 32 {
        self.networkEvents.removeFirst()
      }
    }
  }

  private func iosRuntimeState() -> [String: Any] {
    var runtime = compactState()
    let config = SLANIosSharedStore.readNetworkConfig()
    let stats = SLANIosSharedStore.readPacketTunnelStats()
    let stoppedAtMs = intField(stats, "stoppedAtMs") ?? 0
    let updatedAtMs = intField(stats, "updatedAtMs") ?? 0
    let statsNetworkEnabled = (stats?["networkEnabled"] as? Bool) == true
      || (stoppedAtMs == 0
        && updatedAtMs > 0
        && !(stats?["virtualIp"] as? String ?? "").isEmpty)
    runtime["adapterPresent"] = (state["adapterPresent"] as? Bool) == true
      || config != nil
      || packetTunnelManager != nil
      || stats != nil
    runtime["networkEnabled"] = (state["networkEnabled"] as? Bool) == true || statsNetworkEnabled
    if runtime["virtualIp"] == nil,
      let virtualIp = (stats?["virtualIp"] as? String) ?? (config?["virtualIp"] as? String),
      !virtualIp.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty
    {
      runtime["virtualIp"] = virtualIp.trimmingCharacters(in: .whitespacesAndNewlines)
    }
    if runtime["mtu"] == nil {
      runtime["mtu"] = stats?["mtu"] ?? config?["mtu"]
    }
    if runtime["relayAddress"] == nil {
      runtime["relayAddress"] = relayAddress(config)
    }
    if runtime["relaySessionCount"] == nil {
      let relaySessionCount = intField(stats, "relaySessionCount")
        ?? relaySessionCount(config)
      if let relaySessionCount = relaySessionCount {
        runtime["relaySessionCount"] = relaySessionCount
        runtime["requestedRelaySessionCount"] = relaySessionCount
      }
    }
    if runtime["attachedRelaySessionCount"] == nil,
      let attached = intField(stats, "relayAttachedSessionCount")
    {
      runtime["attachedRelaySessionCount"] = attached
    }
    if runtime["relayAttachFailures"] == nil,
      let failures = intField(stats, "relayAttachFailures")
    {
      runtime["relayAttachFailures"] = failures
    }
    if runtime["lastRelayAttachError"] == nil,
      let error = stats?["lastRelayAttachError"] as? String,
      !error.isEmpty
    {
      runtime["lastRelayAttachError"] = error
    }
    return runtime
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
      isUuidV4(existing.trimmingCharacters(in: .whitespacesAndNewlines))
    {
      return existing.trimmingCharacters(in: .whitespacesAndNewlines)
    }
    let value = UUID().uuidString.lowercased()
    defaults.set(value, forKey: Self.deviceIdKey)
    return value
  }

  private func isUuidV4(_ value: String) -> Bool {
    let pattern = #"^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-4[0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}$"#
    return value.range(of: pattern, options: .regularExpression) != nil
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

  private func intField(_ object: [String: Any]?, _ field: String) -> Int? {
    switch object?[field] {
    case let value as Int:
      return value
    case let value as Int64:
      return Int(value)
    case let value as NSNumber:
      return value.intValue
    case let value as String:
      return Int(value.trimmingCharacters(in: .whitespacesAndNewlines))
    default:
      return nil
    }
  }

  private func relayAddress(_ config: [String: Any]?) -> String? {
    if let relayAddress = config?["relayAddress"] as? String,
      !relayAddress.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty
    {
      return relayAddress.trimmingCharacters(in: .whitespacesAndNewlines)
    }
    let relayDataPlane = config?["relayDataPlane"] as? [String: Any]
    let relayAddress = relayDataPlane?["relayAddress"] as? String ?? ""
    let trimmed = relayAddress.trimmingCharacters(in: .whitespacesAndNewlines)
    return trimmed.isEmpty ? nil : trimmed
  }

  private func relaySessionCount(_ config: [String: Any]?) -> Int? {
    guard let relayDataPlane = config?["relayDataPlane"] as? [String: Any],
      relayDataPlane["enabled"] as? Bool == true
    else {
      return 0
    }
    return (relayDataPlane["sessions"] as? [Any])?.count ?? 0
  }

  private func writeSimulatorPacketTunnelStats(config: [String: Any], enabled: Bool) {
    let now = Int64(Date().timeIntervalSince1970 * 1000)
    let virtualIp = stringField(config, "virtualIp")
    let relayDataPlane = config["relayDataPlane"] as? [String: Any]
    let relaySessions = relaySessionCount(config) ?? 0
    let directPeers = (relayDataPlane?["peerPaths"] as? [Any])?.count ?? 0
    var stats: [String: Any] = [
      "simulatorFallback": true,
      "networkEnabled": enabled,
      "virtualIp": enabled ? virtualIp : "",
      "relaySessionCount": relaySessions,
      "relayAttachedSessionCount": enabled ? relaySessions : 0,
      "relayAttachFailures": 0,
      "lastRelayAttachError": "",
      "packetsRead": 0,
      "bytesRead": 0,
      "bytesWritten": 0,
      "routedPackets": 0,
      "unroutablePackets": 0,
      "nonIpv4Packets": 0,
      "relayFramesSent": 0,
      "relayFramesReceived": 0,
      "relayPacketsWritten": 0,
      "relayDetachSent": 0,
      "relayNoPeerPackets": 0,
      "directUdpAttachedPeerCount": enabled ? directPeers : 0,
      "directUdpReadyPeerCount": 0,
      "directUdpProbesSent": 0,
      "directUdpProbesReceived": 0,
      "directUdpPongsSent": 0,
      "directUdpPongsReceived": 0,
      "directUdpFramesSent": 0,
      "directUdpFramesReceived": 0,
      "startedAtMs": enabled ? now : 0,
      "stoppedAtMs": enabled ? 0 : now,
      "updatedAtMs": now
    ]
    if let mtu = config["mtu"] {
      stats["mtu"] = mtu
    }
    if let relayAddress = relayAddress(config) {
      stats["relayAddress"] = relayAddress
    }
    SLANIosSharedStore.writePacketTunnelStats(stats)
  }
}
