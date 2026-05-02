import Cocoa
import FlutterMacOS
import Network

public class ClientCorePlugin: NSObject, FlutterPlugin, NSWindowDelegate {
  private var statusItem: NSStatusItem?
  private var state: [String: Any?] = [
    "signedIn": false,
    "userLabel": nil,
    "deviceId": nil,
    "authCallbackId": nil,
    "virtualIp": nil,
    "networkEnabled": false,
    "syncing": false,
    "syncReason": nil,
    "switchEnabled": true,
    "notice": "macosStatusItemReady",
    "error": nil
  ]

  public static func register(with registrar: FlutterPluginRegistrar) {
    let channel = FlutterMethodChannel(
      name: "dev.slan/client_core_v2",
      binaryMessenger: registrar.messenger
    )
    let instance = ClientCorePlugin()
    instance.installStatusItem()
    instance.installWindowClosePolicy()
    registrar.addMethodCallDelegate(instance, channel: channel)
  }

  public func handle(_ call: FlutterMethodCall, result: @escaping FlutterResult) {
    let commandType = readCommandType(call.arguments)
    if let serviceResponse = forwardToServiceWithAutoStart(
      method: call.method,
      arguments: call.arguments
    ) {
      if commandType == "openWebConsole" {
        openConsole()
      } else if commandType == "loginWithBrowser" {
        openConsole(
          callbackId: extractStringField(serviceResponse, "authCallbackId"),
          deviceId: extractStringField(serviceResponse, "deviceId")
        )
      }
      result(serviceResponse)
      return
    }

    switch call.method {
    case "start", "state", "refresh":
      result(compactState())
    case "dispatch":
      handleDispatch(call.arguments)
      if commandType == "openWebConsole" {
        openConsole()
      } else if commandType == "loginWithBrowser" {
        openConsole(
          callbackId: state["authCallbackId"] as? String ?? "",
          deviceId: state["deviceId"] as? String ?? ""
        )
      }
      result(compactState())
    case "enqueueControlTask", "enqueueDownstreamControlTask", "ingestDownstreamControlMessage":
      state["syncing"] = false
      state["switchEnabled"] = true
      state["error"] = "local service unavailable"
      result(compactState())
    case "controlTransportStatus":
      result([
        "mqttCredentialReady": false,
        "controlSessionReady": false,
        "ready": false,
        "missing": ["localService"]
      ])
    case "controlTransportPlan":
      result([
        "heartbeatQos": "qos0",
        "controlQos": "qos2"
      ])
    case "controlTransportCadence":
      result([
        "ackFlushIntervalMs": 1000,
        "heartbeatIntervalMs": 30000,
        "runtimeStateIntervalMs": 10000
      ])
    case "controlTransportTickPlan":
      result([
        "nowMs": 0,
        "outbox": [
          "includeHeartbeat": true,
          "includeRuntimeState": true,
          "includeControlAcks": true
        ],
        "nextAckFlushDueMs": 0,
        "nextHeartbeatDueMs": 0,
        "nextRuntimeStateDueMs": 0
      ])
    case "controlTransportOutbox":
      result([
        "messages": []
      ])
    case "pendingControlAcks":
      result([])
    case "markControlAcked":
      result([
        "acknowledged": false,
        "error": "local service unavailable"
      ])
    case "markTransportPublished":
      result([
        "published": false,
        "error": "local service unavailable"
      ])
    case "shutdownNetwork":
      shutdownNetworkBeforeQuit()
      state["networkEnabled"] = false
      state["virtualIp"] = nil
      state["notice"] = "networkShutdown"
      result(compactState())
    default:
      result(FlutterMethodNotImplemented)
    }
  }

  private func handleDispatch(_ arguments: Any?) {
    guard
      let command = arguments as? [String: Any],
      let type = command["type"] as? String
    else {
      state["error"] = "invalid command"
      return
    }

    state["error"] = nil
    state["syncReason"] = type
    switch type {
    case "loginWithBrowser":
      state["authCallbackId"] = "cb-\(Int(Date().timeIntervalSince1970 * 1000))"
      state["notice"] = "loginBrowserRequested"
    case "openWebConsole":
      state["notice"] = "webConsoleRequested"
    case "logout":
      state["signedIn"] = false
      state["userLabel"] = nil
      state["deviceId"] = nil
      state["authCallbackId"] = nil
      state["virtualIp"] = nil
      state["networkEnabled"] = false
      state["notice"] = "signedOut"
    default:
      state["notice"] = "\(type)Requested"
    }
    state["syncReason"] = nil
  }

  private func installStatusItem() {
    let item = NSStatusBar.system.statusItem(withLength: NSStatusItem.variableLength)
    item.button?.title = "SLAN"

    let menu = NSMenu()
    menu.addItem(NSMenuItem(
      title: "Open SLAN Client",
      action: #selector(openMainWindow),
      keyEquivalent: ""
    ))
    menu.addItem(NSMenuItem.separator())
    menu.addItem(NSMenuItem(
      title: "Quit",
      action: #selector(quitShell),
      keyEquivalent: "q"
    ))
    item.menu = menu
    statusItem = item
  }

  private func installWindowClosePolicy() {
    assignWindowDelegates()
    NotificationCenter.default.addObserver(
      self,
      selector: #selector(windowDidBecomeKey(_:)),
      name: NSWindow.didBecomeKeyNotification,
      object: nil
    )
  }

  private func assignWindowDelegates() {
    for window in NSApp.windows {
      window.delegate = self
    }
  }

  @objc private func windowDidBecomeKey(_ notification: Notification) {
    if let window = notification.object as? NSWindow {
      window.delegate = self
    }
  }

  public func windowShouldClose(_ sender: NSWindow) -> Bool {
    sender.orderOut(nil)
    return false
  }

  @objc private func openMainWindow() {
    NSApp.activate(ignoringOtherApps: true)
    for window in NSApp.windows {
      window.makeKeyAndOrderFront(nil)
    }
  }

  @objc private func quitShell() {
    shutdownNetworkBeforeQuit()
    NSApp.terminate(nil)
  }

  private func openConsole(callbackId: String = "", deviceId: String = "") {
    let target = ProcessInfo.processInfo.environment["SLAN_WEB_CONSOLE_URL"]
      ?? "http://127.0.0.1:24200"
    var components = URLComponents(string: target)
    var queryItems = components?.queryItems ?? []
    if !callbackId.isEmpty {
      queryItems.append(URLQueryItem(name: "auth", value: "login"))
      queryItems.append(URLQueryItem(name: "callbackId", value: callbackId))
    }
    let safeDeviceId = usableClientDeviceId(deviceId)
    if !safeDeviceId.isEmpty {
      queryItems.append(URLQueryItem(name: "deviceId", value: safeDeviceId))
    }
    queryItems.append(URLQueryItem(name: "clientPlatform", value: "macos"))
    queryItems.append(URLQueryItem(name: "clientName", value: "SLAN Client V2"))
    if !queryItems.isEmpty {
      components?.queryItems = queryItems
    }
    if let url = components?.url ?? URL(string: target) {
      NSWorkspace.shared.open(url)
    }
  }

  private func usableClientDeviceId(_ deviceId: String) -> String {
    let value = deviceId.trimmingCharacters(in: .whitespacesAndNewlines)
    if value.isEmpty {
      return ""
    }
    let lower = value.lowercased()
    if lower == "authcallbackid" || lower == "windows-plugin-login" || lower == "macos-plugin-login" {
      return ""
    }
    if lower.hasPrefix("cb-") {
      return ""
    }
    return value
  }

  private func readCommandType(_ arguments: Any?) -> String {
    guard
      let command = arguments as? [String: Any],
      let type = command["type"] as? String
    else {
      return ""
    }
    return type
  }

  private func extractStringField(_ json: String, _ field: String) -> String {
    guard
      let data = json.data(using: .utf8),
      let object = try? JSONSerialization.jsonObject(with: data) as? [String: Any],
      let value = object[field] as? String
    else {
      return ""
    }
    return value
  }

  private func forwardToServiceWithAutoStart(method: String, arguments: Any?) -> String? {
    if let response = forwardToService(method: method, arguments: arguments) {
      return response
    }
    guard tryStartBundledService() else {
      return nil
    }
    for _ in 0..<15 {
      if let response = forwardToService(method: method, arguments: arguments) {
        return response
      }
      Thread.sleep(forTimeInterval: 0.1)
    }
    return nil
  }

  private func tryStartBundledService() -> Bool {
    guard let executableDir = Bundle.main.executableURL?.deletingLastPathComponent() else {
      return false
    }
    let serviceURL = executableDir.appendingPathComponent("client-core-service")
    guard FileManager.default.isExecutableFile(atPath: serviceURL.path) else {
      return false
    }
    let process = Process()
    process.executableURL = serviceURL
    do {
      try process.run()
      Thread.sleep(forTimeInterval: 0.3)
      return true
    } catch {
      return false
    }
  }

  private func forwardToService(method: String, arguments: Any?) -> String? {
    let argsValue: Any = arguments ?? [String: Any]()
    let envelope: [String: Any] = [
      "method": method,
      "args": argsValue
    ]
    guard
      JSONSerialization.isValidJSONObject(envelope),
      let requestData = try? JSONSerialization.data(withJSONObject: envelope),
      var request = String(data: requestData, encoding: .utf8)
    else {
      return nil
    }
    request.append("\n")

    let serviceHost = ProcessInfo.processInfo.environment["SLAN_CLIENT_CORE_SERVICE_HOST"]
      ?? "127.0.0.1:46392"
    let parts = serviceHost.split(separator: ":", maxSplits: 1).map(String.init)
    let host = parts.first?.isEmpty == false ? parts[0] : "127.0.0.1"
    let portValue: UInt16 = parts.count > 1 ? (UInt16(parts[1]) ?? 46392) : 46392
    guard let port = NWEndpoint.Port(rawValue: portValue) else {
      return nil
    }

    let connection = NWConnection(
      host: NWEndpoint.Host(host),
      port: port,
      using: .tcp
    )
    let semaphore = DispatchSemaphore(value: 0)
    let payload = request.data(using: .utf8)
    var response = Data()

    func receiveNext() {
      connection.receive(minimumIncompleteLength: 1, maximumLength: 4096) { data, _, isComplete, error in
        if let data = data {
          response.append(data)
        }
        if response.contains(0x0A) || isComplete || error != nil {
          connection.cancel()
          semaphore.signal()
          return
        }
        receiveNext()
      }
    }

    connection.stateUpdateHandler = { state in
      if case .ready = state {
        connection.send(content: payload, completion: .contentProcessed { error in
          if error != nil {
            connection.cancel()
            semaphore.signal()
            return
          }
          receiveNext()
        })
      }
      if case .failed = state {
        connection.cancel()
        semaphore.signal()
      }
    }
    connection.start(queue: DispatchQueue.global(qos: .utility))
    _ = semaphore.wait(timeout: .now() + 2)

    guard let text = String(data: response, encoding: .utf8), !text.isEmpty else {
      return nil
    }
    return text.components(separatedBy: "\n").first
  }

  private func shutdownNetworkBeforeQuit() {
    let serviceHost = ProcessInfo.processInfo.environment["SLAN_CLIENT_CORE_SERVICE_HOST"]
      ?? "127.0.0.1:46392"
    let parts = serviceHost.split(separator: ":", maxSplits: 1).map(String.init)
    let host = parts.first?.isEmpty == false ? parts[0] : "127.0.0.1"
    let portValue: UInt16 = parts.count > 1 ? (UInt16(parts[1]) ?? 46392) : 46392
    guard let port = NWEndpoint.Port(rawValue: portValue) else {
      return
    }
    let connection = NWConnection(
      host: NWEndpoint.Host(host),
      port: port,
      using: .tcp
    )
    let semaphore = DispatchSemaphore(value: 0)
    let payload = "{\"method\":\"shutdownNetwork\",\"args\":{}}\n".data(using: .utf8)
    connection.stateUpdateHandler = { state in
      if case .ready = state {
        connection.send(content: payload, completion: .contentProcessed { _ in
          connection.cancel()
          semaphore.signal()
        })
      }
      if case .failed = state {
        connection.cancel()
        semaphore.signal()
      }
    }
    connection.start(queue: DispatchQueue.global(qos: .utility))
    _ = semaphore.wait(timeout: .now() + 2)
  }

  private func compactState() -> [String: Any] {
    var compact: [String: Any] = [:]
    for (key, value) in state {
      compact[key] = value ?? NSNull()
    }
    return compact
  }
}
