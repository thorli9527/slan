import Cocoa
import FlutterMacOS
import Network

public class ClientCorePlugin: NSObject, FlutterPlugin, NSWindowDelegate {
  private static let launchdServiceLabel = "dev.slan.client-core-service"
  private var statusItem: NSStatusItem?
  private var networkMenuItem: NSMenuItem?
  private let stateWatchQueue = DispatchQueue(
    label: "dev.slan.client_core_v2.macos.stateWatch",
    qos: .utility
  )
  private var stateWatchStarted = false
  private var lastStateRevision: Int64 = 0
  private var latestMenuState: [String: Any]?
  private var state: [String: Any?] = [
    "signedIn": false,
    "userLabel": nil,
    "deviceId": nil,
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
    NSApp.setActivationPolicy(.accessory)
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
        openAuthenticatedConsole()
      } else if commandType == "loginWithBrowser" {
        openConsole(
          deviceId: extractStringField(serviceResponse, "deviceId"),
          browserLogin: true
        )
      }
      result(serviceResponse)
      return
    }

    result(FlutterMethodNotImplemented)
  }

  private func installStatusItem() {
    let item = NSStatusBar.system.statusItem(withLength: NSStatusItem.squareLength)
    applyStatusIcon(networkEnabled: false, serviceAvailable: false)
    item.button?.toolTip = "SLAN Client"

    let menu = NSMenu()
    menu.addItem(makeMenuItem(
      title: "Settings",
      action: #selector(openMainWindow),
      keyEquivalent: ""
    ))
    let networkItem = makeMenuItem(
      title: "Enable Network",
      action: #selector(toggleNetwork),
      keyEquivalent: ""
    )
    networkItem.isEnabled = false
    menu.addItem(networkItem)
    menu.addItem(NSMenuItem.separator())
    menu.addItem(makeMenuItem(
      title: "Quit",
      action: #selector(quitShell),
      keyEquivalent: "q"
    ))
    item.menu = menu
    networkMenuItem = networkItem
    self.statusItem = item
    applyStatusIcon(networkEnabled: false, serviceAvailable: false)
    startStateWatchLoop()
  }

  private func applyStatusIcon(networkEnabled: Bool, serviceAvailable: Bool) {
    guard let button = statusItem?.button else {
      return
    }
    let image = makeVLStatusImage(networkEnabled: networkEnabled, serviceAvailable: serviceAvailable)
    button.image = image
    button.imagePosition = .imageOnly
  }

  private func makeVLStatusImage(networkEnabled: Bool, serviceAvailable: Bool) -> NSImage {
    let size = NSSize(width: 22, height: 22)
    let image = NSImage(size: size)
    image.lockFocus()
    let fill = networkEnabled
      ? NSColor(calibratedRed: 0.04, green: 0.39, blue: 0.96, alpha: 1)
      : (serviceAvailable
        ? NSColor(calibratedRed: 0.43, green: 0.48, blue: 0.57, alpha: 1)
        : NSColor(calibratedRed: 0.83, green: 0.20, blue: 0.17, alpha: 1))
    fill.setFill()
    NSBezierPath(roundedRect: NSRect(x: 1, y: 1, width: 20, height: 20), xRadius: 5, yRadius: 5).fill()
    let paragraph = NSMutableParagraphStyle()
    paragraph.alignment = .center
    let attributes: [NSAttributedString.Key: Any] = [
      .font: NSFont.boldSystemFont(ofSize: 11),
      .foregroundColor: NSColor.white,
      .paragraphStyle: paragraph,
    ]
    NSString(string: "VL").draw(in: NSRect(x: 0, y: 4.1, width: 22, height: 14), withAttributes: attributes)
    image.unlockFocus()
    image.isTemplate = false
    return image
  }

  private func makeMenuItem(title: String, action: Selector, keyEquivalent: String) -> NSMenuItem {
    let item = NSMenuItem(title: title, action: action, keyEquivalent: keyEquivalent)
    item.target = self
    return item
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

  @objc public func windowDidBecomeKey(_ notification: Notification) {
    if let window = notification.object as? NSWindow {
      window.delegate = self
    }
  }

  public func windowShouldClose(_ sender: NSWindow) -> Bool {
    sender.orderOut(nil)
    NSApp.setActivationPolicy(.accessory)
    return false
  }

  @objc private func openMainWindow() {
    NSApp.setActivationPolicy(.regular)
    NSApp.unhide(nil)
    NSApp.activate(ignoringOtherApps: true)
    for window in NSApp.windows {
      if !window.isVisible {
        window.center()
      }
      positionWindowOnCurrentScreen(window)
      window.deminiaturize(nil)
      window.level = .floating
      window.makeKeyAndOrderFront(nil)
      window.orderFrontRegardless()
      DispatchQueue.main.asyncAfter(deadline: .now() + 0.35) {
        window.level = .normal
      }
    }
  }

  private func positionWindowOnCurrentScreen(_ window: NSWindow) {
    let mouse = NSEvent.mouseLocation
    let screen = NSScreen.screens.first { screen in
      screen.frame.contains(mouse)
    } ?? NSScreen.main
    guard let visibleFrame = screen?.visibleFrame else {
      return
    }
    let size = window.frame.size
    let x = visibleFrame.midX - size.width / 2
    let y = visibleFrame.midY - size.height / 2
    window.setFrameOrigin(NSPoint(x: x, y: y))
  }

  @objc private func quitShell() {
    shutdownNetworkBeforeQuit()
    NSApp.terminate(nil)
  }

  @objc private func toggleNetwork() {
    guard boolField(latestMenuState, "signedIn"),
      !boolField(latestMenuState, "syncing"),
      boolField(latestMenuState, "switchEnabled")
    else {
      return
    }
    let enabled = boolField(latestMenuState, "networkEnabled")
    let method = enabled ? "localNetworkDeactivate" : "localNetworkActivate"
    _ = forwardToServiceWithAutoStart(method: method, arguments: nil)
  }

  private func startStateWatchLoop() {
    guard !stateWatchStarted else {
      return
    }
    stateWatchStarted = true
    stateWatchQueue.async { [weak self] in
      self?.runStateWatchLoop()
    }
  }

  private func runStateWatchLoop() {
    while true {
      let arguments: [String: Any] = [
        "lastRevision": lastStateRevision,
        "timeoutMs": 30000
      ]
      guard
        let response = forwardToServiceWithAutoStart(
          method: "localStateWatch",
          arguments: arguments
        ),
        let watch = parseJsonObject(response),
        let snapshot = watch["state"] as? [String: Any]
      else {
        DispatchQueue.main.async { [weak self] in
          self?.applyServiceUnavailableMenuState()
        }
        Thread.sleep(forTimeInterval: 2)
        continue
      }
      let revision = int64Field(watch, "revision")
      if revision > lastStateRevision {
        lastStateRevision = revision
      }
      DispatchQueue.main.async { [weak self] in
        self?.applyMenuStateIfChanged(snapshot)
      }
    }
  }

  private func applyServiceUnavailableMenuState() {
    if latestMenuState == nil && statusItem?.button?.toolTip == "SLAN Client - Service unavailable" {
      return
    }
    latestMenuState = nil
    statusItem?.button?.toolTip = "SLAN Client - Service unavailable"
    applyStatusIcon(networkEnabled: false, serviceAvailable: false)
    networkMenuItem?.title = "Enable Network"
    networkMenuItem?.isEnabled = false
  }

  private func applyMenuStateIfChanged(_ snapshot: [String: Any]) {
    if menuStateEquals(latestMenuState, snapshot) {
      return
    }
    latestMenuState = snapshot
    let signedIn = boolField(snapshot, "signedIn")
    let networkEnabled = boolField(snapshot, "networkEnabled")
    let syncing = boolField(snapshot, "syncing")
    let switchEnabled = boolField(snapshot, "switchEnabled")
    let error = stringField(snapshot, "error")
    let statusText: String
    if !error.isEmpty {
      statusText = "Error"
    } else if syncing {
      statusText = "Connecting"
    } else if networkEnabled {
      statusText = "Connected"
    } else if signedIn {
      statusText = "Disconnected"
    } else {
      statusText = "Signed out"
    }
    networkMenuItem?.title = networkEnabled ? "Disable Network" : "Enable Network"
    networkMenuItem?.isEnabled = signedIn && switchEnabled && !syncing
    applyStatusIcon(networkEnabled: networkEnabled, serviceAvailable: true)
    statusItem?.button?.toolTip = error.isEmpty ? "SLAN Client - \(statusText)" : "SLAN Client - \(statusText): \(error)"
  }

  private func menuStateEquals(_ left: [String: Any]?, _ right: [String: Any]) -> Bool {
    return boolField(left, "signedIn") == boolField(right, "signedIn")
      && boolField(left, "networkEnabled") == boolField(right, "networkEnabled")
      && boolField(left, "syncing") == boolField(right, "syncing")
      && boolField(left, "switchEnabled") == boolField(right, "switchEnabled")
      && stringField(left, "error") == stringField(right, "error")
      && stringField(left, "deviceId") == stringField(right, "deviceId")
  }

  private func openAuthenticatedConsole() {
    if let response = forwardToServiceWithAutoStart(method: "consoleLoginKey", arguments: nil) {
      let loginKey = extractStringField(response, "loginKey")
      if loginKey.isEmpty {
        let error = stringField(parseJsonObject(response), "error")
        showWebConsoleOpenError(error.isEmpty ? "failed to create console login key" : error)
        return
      }
      openConsole(
        deviceId: extractStringField(response, "deviceId"),
        consoleLoginKey: loginKey
      )
      return
    }
    showWebConsoleOpenError("client-core-service is not available")
  }

  private func showWebConsoleOpenError(_ message: String) {
    DispatchQueue.main.async {
      let alert = NSAlert()
      alert.messageText = "Web Console 打开失败"
      alert.informativeText = message
      alert.alertStyle = .warning
      alert.addButton(withTitle: "确定")
      alert.runModal()
    }
  }

  private func openConsole(
    deviceId: String = "",
    consoleLoginKey: String = "",
    browserLogin: Bool = false
  ) {
    let target = resolveWebConsoleUrl()
    var components = URLComponents(string: target)
    var queryItems = components?.queryItems ?? []
    if browserLogin {
      queryItems.append(URLQueryItem(name: "auth", value: "login"))
    }
    if !consoleLoginKey.isEmpty {
      queryItems.append(URLQueryItem(name: "consoleLoginKey", value: consoleLoginKey))
    }
    let safeDeviceId = usableClientDeviceId(deviceId)
    if !safeDeviceId.isEmpty {
      queryItems.append(URLQueryItem(name: "deviceId", value: safeDeviceId))
    }
    if !queryItems.isEmpty {
      components?.queryItems = queryItems
    }
    if let url = components?.url ?? URL(string: target) {
      NSWorkspace.shared.open(url)
    }
  }

  private func resolveWebConsoleUrl() -> String {
    let environment = ProcessInfo.processInfo.environment
    if let value = environment["SLAN_WEB_CONSOLE_URL"]?.trimmingCharacters(in: .whitespacesAndNewlines),
      !value.isEmpty
    {
      return value
    }
    if let value = environment["SLAN_CONTROL_BASE_URL"]?.trimmingCharacters(in: .whitespacesAndNewlines),
      !value.isEmpty
    {
      return webConsoleUrl(fromControlBaseUrl: value)
    }
    return "http://47.245.40.231:24200"
  }

  private func webConsoleUrl(fromControlBaseUrl value: String) -> String {
    guard let components = URLComponents(string: value),
      let host = components.host?.lowercased()
    else {
      return "http://47.245.40.231:24200"
    }
    let scheme = components.scheme?.isEmpty == false ? components.scheme! : "http"
    if host == "api.dev.staticlss.com" {
      return "\(scheme)://web.dev.staticlss.com"
    }
    if host == "api.slan.localhost" || host == "slan.localhost" {
      return "\(scheme)://web.slan.localhost"
    }
    if host == "127.0.0.1" || host == "localhost" || host == "::1" || components.port == 28080 {
      return "\(scheme)://\(host):24200"
    }
    return "http://47.245.40.231:24200"
  }

  private func usableClientDeviceId(_ deviceId: String) -> String {
    return deviceId.trimmingCharacters(in: .whitespacesAndNewlines)
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
    return stringField(parseJsonObject(json), field)
  }

  private func parseJsonObject(_ json: String) -> [String: Any]? {
    guard
      let data = json.data(using: .utf8),
      let object = try? JSONSerialization.jsonObject(with: data) as? [String: Any]
    else {
      return nil
    }
    return object
  }

  private func stringField(_ object: [String: Any]?, _ field: String) -> String {
    return (object?[field] as? String)?.trimmingCharacters(in: .whitespacesAndNewlines) ?? ""
  }

  private func boolField(_ object: [String: Any]?, _ field: String) -> Bool {
    return object?[field] as? Bool ?? false
  }

  private func int64Field(_ object: [String: Any]?, _ field: String) -> Int64 {
    if let value = object?[field] as? Int64 {
      return value
    }
    if let value = object?[field] as? Int {
      return Int64(value)
    }
    if let value = object?[field] as? NSNumber {
      return value.int64Value
    }
    return 0
  }

  private func forwardToServiceWithAutoStart(method: String, arguments: Any?) -> String? {
    if let response = forwardToService(method: method, arguments: arguments) {
      return response
    }
    if tryStartLaunchdService() {
      for _ in 0..<15 {
        if let response = forwardToService(method: method, arguments: arguments) {
          return response
        }
        Thread.sleep(forTimeInterval: 0.1)
      }
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

  private func tryStartLaunchdService() -> Bool {
    guard launchdServiceLoaded() else {
      return false
    }
    let process = Process()
    process.executableURL = URL(fileURLWithPath: "/bin/launchctl")
    process.arguments = [
      "kickstart",
      "-k",
      "system/\(Self.launchdServiceLabel)"
    ]
    do {
      try process.run()
      process.waitUntilExit()
      return process.terminationStatus == 0
    } catch {
      return false
    }
  }

  private func launchdServiceLoaded() -> Bool {
    let process = Process()
    process.executableURL = URL(fileURLWithPath: "/bin/launchctl")
    process.arguments = [
      "print",
      "system/\(Self.launchdServiceLabel)"
    ]
    process.standardOutput = Pipe()
    process.standardError = Pipe()
    do {
      try process.run()
      process.waitUntilExit()
      return process.terminationStatus == 0
    } catch {
      return false
    }
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
    let payload = "{\"method\":\"localNetworkShutdown\",\"args\":{}}\n".data(using: .utf8)
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
