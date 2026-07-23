import Cocoa
import FlutterMacOS
import Network

public class ClientCorePlugin: NSObject, FlutterPlugin, NSWindowDelegate {
  private static let launchdServiceLabel = "dev.slan.client-core-service"
  private static let defaultServiceHost = "127.0.0.1:46392"
  private static let trayOpenTitle = "Open"
  private static let trayNetworkTitle = "Network"
  private static let trayQuitTitle = "Quit"
  private var statusItem: NSStatusItem?
  private var trayStatusLabel: NSTextField?
  private var trayAccountEmailLabel: NSTextField?
  private var networkSwitch: NSSwitch?
  private let bundledServiceLock = NSLock()
  private let browserLogLock = NSLock()
  private var bundledServiceProcess: Process?
  private var bundledServicePreferredHost: String?
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
    if call.method == "openExternalUrl" {
      guard let rawUrl = call.arguments as? String else {
        result(FlutterError(
          code: "invalid_url",
          message: "external URL must be a string",
          details: nil
        ))
        return
      }
      openExternalUrl(rawUrl, result: result)
      return
    }
    if let serviceResponse = forwardToServiceWithAutoStart(
      method: call.method,
      arguments: call.arguments
    ) {
      result(serviceResponse)
      return
    }

    result(FlutterMethodNotImplemented)
  }

  private func openExternalUrl(_ rawUrl: String, result: @escaping FlutterResult) {
    guard let url = URL(string: rawUrl), let scheme = url.scheme,
      scheme == "http" || scheme == "https"
    else {
      result(FlutterError(
        code: "invalid_url",
        message: "only HTTP and HTTPS URLs can be opened",
        details: rawUrl
      ))
      return
    }

    writeBrowserLog(
      "native.request scheme=\(url.scheme ?? "") host=\(url.host ?? "")"
    )
    let applicationUrl = NSWorkspace.shared.urlForApplication(toOpen: url)
    let applicationBundle = applicationUrl.flatMap { Bundle(url: $0) }
    let bundleId = applicationBundle?.bundleIdentifier ?? ""
    let chromiumBrowser = bundleId.hasPrefix("com.google.Chrome")
      || bundleId.hasPrefix("org.chromium.Chromium")
      || bundleId.hasPrefix("com.microsoft.edgemac")
      || bundleId.hasPrefix("com.brave.Browser")

    DispatchQueue.global(qos: .userInitiated).async {
      let process = Process()
      if chromiumBrowser, let executableUrl = applicationBundle?.executableURL {
        process.executableURL = executableUrl
        process.arguments = ["--new-window", rawUrl]
      } else {
        process.executableURL = URL(fileURLWithPath: "/usr/bin/open")
        process.arguments = [rawUrl]
      }
      process.standardOutput = FileHandle.nullDevice
      process.standardError = FileHandle.nullDevice
      do {
        try process.run()
        process.waitUntilExit()
      } catch {
        self.finishBrowserOpen(
          result: result,
          rawUrl: rawUrl,
          bundleId: bundleId,
          error: error.localizedDescription
        )
        return
      }
      guard process.terminationStatus == 0 else {
        self.finishBrowserOpen(
          result: result,
          rawUrl: rawUrl,
          bundleId: bundleId,
          error: "open returned \(process.terminationStatus)"
        )
        return
      }
      self.finishBrowserOpen(
        result: result,
        rawUrl: rawUrl,
        bundleId: bundleId
      )
    }
  }

  private func finishBrowserOpen(
    result: @escaping FlutterResult,
    rawUrl: String,
    bundleId: String,
    error: String? = nil
  ) {
    DispatchQueue.main.async {
      if let error = error {
        self.writeBrowserLog("native.failed error=\(error)")
        result(FlutterError(
          code: "open_browser_failed",
          message: error,
          details: rawUrl
        ))
        return
      }
      let application = NSRunningApplication
        .runningApplications(withBundleIdentifier: bundleId)
        .last
      application?.unhide()
      let activated = application?.activate(
        options: [.activateAllWindows, .activateIgnoringOtherApps]
      ) ?? false
      self.writeBrowserLog(
        "native.completed bundle=\(bundleId) activated=\(activated)"
      )
      result(true)
    }
  }

  private func writeBrowserLog(_ message: String) {
    browserLogLock.lock()
    defer { browserLogLock.unlock() }
    let formatter = ISO8601DateFormatter()
    let line = "\(formatter.string(from: Date())) SLAN_BROWSER \(message)\n"
    let directory = URL(fileURLWithPath: "/tmp/slan", isDirectory: true)
    let file = directory.appendingPathComponent("client-v2-native.log")
    do {
      try FileManager.default.createDirectory(
        at: directory,
        withIntermediateDirectories: true
      )
      if !FileManager.default.fileExists(atPath: file.path) {
        FileManager.default.createFile(atPath: file.path, contents: nil)
      }
      let handle = try FileHandle(forWritingTo: file)
      defer { handle.closeFile() }
      handle.seekToEndOfFile()
      if let data = line.data(using: .utf8) {
        handle.write(data)
      }
    } catch {
      NSLog("SLAN_BROWSER log.failed error=%@", error.localizedDescription)
    }
  }

  private func installStatusItem() {
    let item = NSStatusBar.system.statusItem(withLength: NSStatusItem.squareLength)
    applyStatusIcon(signedIn: false, networkEnabled: false, serviceAvailable: false)
    item.button?.toolTip = "SLAN Client"

    let menu = NSMenu()
    menu.minimumWidth = 340
    let headerItem = NSMenuItem()
    headerItem.view = makeStatusMenuHeader()
    menu.addItem(headerItem)
    menu.addItem(NSMenuItem.separator())
    menu.addItem(makeMenuItem(
      title: Self.trayOpenTitle,
      action: #selector(openMainWindow),
      keyEquivalent: ""
    ))
    menu.addItem(makeMenuItem(
      title: Self.trayQuitTitle,
      action: #selector(quitShell),
      keyEquivalent: "q"
    ))
    item.menu = menu
    self.statusItem = item
    applyStatusIcon(signedIn: false, networkEnabled: false, serviceAvailable: false)
    startStateWatchLoop()
  }

  private func makeStatusMenuHeader() -> NSView {
    let view = NSView(frame: NSRect(x: 0, y: 0, width: 340, height: 100))

    let title = makeMenuLabel("SLAN", size: 18, weight: .semibold, color: .labelColor)
    title.frame = NSRect(x: 16, y: 70, width: 220, height: 23)
    view.addSubview(title)

    let status = makeMenuLabel("正在连接", size: 12, weight: .regular, color: .secondaryLabelColor)
    status.frame = NSRect(x: 16, y: 51, width: 220, height: 18)
    view.addSubview(status)
    trayStatusLabel = status

    let toggle = NSSwitch(frame: NSRect(x: 270, y: 54, width: 50, height: 26))
    toggle.target = self
    toggle.action = #selector(toggleNetwork)
    toggle.isEnabled = false
    view.addSubview(toggle)
    networkSwitch = toggle

    let separator = NSBox(frame: NSRect(x: 16, y: 40, width: 308, height: 1))
    separator.boxType = .separator
    view.addSubview(separator)

    let user = makeMenuLabel("用户", size: 12, weight: .semibold, color: .secondaryLabelColor)
    user.frame = NSRect(x: 16, y: 10, width: 48, height: 20)
    view.addSubview(user)

    let email = makeMenuLabel("未登录", size: 14, weight: .medium, color: .labelColor)
    email.frame = NSRect(x: 76, y: 10, width: 248, height: 20)
    email.lineBreakMode = .byTruncatingTail
    view.addSubview(email)
    trayAccountEmailLabel = email

    return view
  }

  private func makeMenuLabel(
    _ text: String,
    size: CGFloat,
    weight: NSFont.Weight,
    color: NSColor
  ) -> NSTextField {
    let label = NSTextField(labelWithString: text)
    label.font = NSFont.systemFont(ofSize: size, weight: weight)
    label.textColor = color
    label.isSelectable = false
    return label
  }

  private func applyStatusIcon(
    signedIn: Bool,
    networkEnabled: Bool,
    serviceAvailable: Bool
  ) {
    guard let button = statusItem?.button else {
      return
    }
    let image = makeVLStatusImage(
      signedIn: signedIn,
      networkEnabled: networkEnabled,
      serviceAvailable: serviceAvailable
    )
    button.image = image
    button.imagePosition = .imageOnly
  }

  private func makeVLStatusImage(
    signedIn: Bool,
    networkEnabled: Bool,
    serviceAvailable: Bool
  ) -> NSImage {
    let size = NSSize(width: 22, height: 22)
    let image = NSImage(size: size)
    image.lockFocus()
    let fill: NSColor
    if !serviceAvailable {
      fill = NSColor(calibratedRed: 0.83, green: 0.20, blue: 0.17, alpha: 1)
    } else if networkEnabled {
      fill = NSColor(calibratedRed: 0.04, green: 0.39, blue: 0.96, alpha: 1)
    } else if signedIn {
      fill = NSColor(calibratedRed: 0.93, green: 0.39, blue: 0.06, alpha: 1)
    } else {
      fill = NSColor(calibratedRed: 0.43, green: 0.48, blue: 0.57, alpha: 1)
    }
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
    applyStatusIcon(signedIn: false, networkEnabled: false, serviceAvailable: false)
    trayStatusLabel?.stringValue = "服务暂不可用"
    networkSwitch?.isEnabled = false
    networkSwitch?.state = .off
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
      statusText = "连接异常"
    } else if syncing {
      statusText = "正在连接"
    } else if networkEnabled {
      statusText = "已连接"
    } else if signedIn {
      statusText = "未连接"
    } else {
      statusText = "未登录"
    }
    trayStatusLabel?.stringValue = statusText
    networkSwitch?.state = networkEnabled ? .on : .off
    networkSwitch?.isEnabled = trayNetworkItemEnabled(
      signedIn: signedIn,
      syncing: syncing,
      switchEnabled: switchEnabled
    )
    let userLabel = stringField(snapshot, "userLabel")
    trayAccountEmailLabel?.stringValue = userLabel.isEmpty ? "未登录" : userLabel
    applyStatusIcon(
      signedIn: signedIn,
      networkEnabled: networkEnabled,
      serviceAvailable: true
    )
    statusItem?.button?.toolTip = error.isEmpty ? "SLAN Client - \(statusText)" : "SLAN Client - \(statusText): \(error)"
    statusItem?.button?.setAccessibilityLabel("SLAN Client - \(statusText)")
  }

  private func trayNetworkItemEnabled(
    signedIn: Bool,
    syncing: Bool,
    switchEnabled: Bool
  ) -> Bool {
    return signedIn && switchEnabled && !syncing
  }

  private func menuStateEquals(_ left: [String: Any]?, _ right: [String: Any]) -> Bool {
    return boolField(left, "signedIn") == boolField(right, "signedIn")
      && boolField(left, "networkEnabled") == boolField(right, "networkEnabled")
      && boolField(left, "syncing") == boolField(right, "syncing")
      && boolField(left, "switchEnabled") == boolField(right, "switchEnabled")
      && stringField(left, "error") == stringField(right, "error")
      && stringField(left, "userLabel") == stringField(right, "userLabel")
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
      if !openConsole(
        deviceId: extractStringField(response, "deviceId"),
        consoleLoginKey: loginKey
      ) {
        showWebConsoleOpenError("macOS did not accept the Web Console URL")
      }
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

  @discardableResult
  private func openConsole(
    deviceId: String = "",
    consoleLoginKey: String = "",
    browserLogin: Bool = false
  ) -> Bool {
    let target = resolveWebConsoleUrl()
    var components = URLComponents(string: target)
    var queryItems = components?.queryItems ?? []
    if browserLogin {
      queryItems.append(URLQueryItem(name: "auth", value: "login"))
      queryItems.append(URLQueryItem(name: "source", value: "client"))
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
    guard let url = components?.url ?? URL(string: target) else {
      return false
    }
    if openWithSystemCommand(url) {
      return true
    }
    if Thread.isMainThread {
      return NSWorkspace.shared.open(url)
    }
    return DispatchQueue.main.sync { NSWorkspace.shared.open(url) }
  }

  private func openWithSystemCommand(_ url: URL) -> Bool {
    let process = Process()
    process.executableURL = URL(fileURLWithPath: "/usr/bin/open")
    process.arguments = [url.absoluteString]
    process.standardOutput = FileHandle.nullDevice
    process.standardError = FileHandle.nullDevice
    do {
      try process.run()
      process.waitUntilExit()
      return process.terminationStatus == 0
    } catch {
      return false
    }
  }

  private func resolveWebConsoleUrl() -> String {
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
    if tryStartBundledService() {
      for _ in 0..<15 {
        if let response = forwardToService(method: method, arguments: arguments) {
          return response
        }
        Thread.sleep(forTimeInterval: 0.1)
      }
    }
    return tryStartBundledService(forceRestart: true)
      ? waitForServiceResponse(method: method, arguments: arguments)
      : nil
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
    return tryStartBundledService(forceRestart: false)
  }

  private func tryStartBundledService(forceRestart: Bool) -> Bool {
    bundledServiceLock.lock()
    defer { bundledServiceLock.unlock() }

    let serviceHost = configuredServiceHost()
    if let process = bundledServiceProcess, process.isRunning {
      bundledServicePreferredHost = serviceHost
      return true
    }
    if forceRestart {
      bundledServiceProcess?.terminate()
      bundledServiceProcess = nil
      bundledServicePreferredHost = nil
    }

    guard let executableDir = Bundle.main.executableURL?.deletingLastPathComponent() else {
      return false
    }
    let serviceURL = executableDir.appendingPathComponent("client-core-service")
    guard FileManager.default.isExecutableFile(atPath: serviceURL.path) else {
      return false
    }

    let stateRoot = FileManager.default.urls(
      for: .applicationSupportDirectory,
      in: .userDomainMask
    ).first
    if let stateRoot {
      try? FileManager.default.createDirectory(
        at: stateRoot,
        withIntermediateDirectories: true
      )
    }

    let process = Process()
    process.executableURL = serviceURL
    var environment = ProcessInfo.processInfo.environment
    environment["SLAN_CLIENT_CORE_SERVICE_HOST"] = serviceHost
    if let stateRoot {
      environment["SLAN_STATE_DIR"] = stateRoot.path
    }
    process.environment = environment
    process.terminationHandler = { [weak self] terminated in
      guard let self else { return }
      self.bundledServiceLock.lock()
      defer { self.bundledServiceLock.unlock() }
      if self.bundledServiceProcess === terminated {
        self.bundledServiceProcess = nil
        if self.bundledServicePreferredHost == serviceHost {
          self.bundledServicePreferredHost = nil
        }
      }
    }
    do {
      try process.run()
      bundledServiceProcess = process
      bundledServicePreferredHost = serviceHost
      Thread.sleep(forTimeInterval: 0.3)
      return true
    } catch {
      bundledServiceProcess = nil
      bundledServicePreferredHost = nil
      return false
    }
  }

  private func waitForServiceResponse(method: String, arguments: Any?) -> String? {
    for _ in 0..<15 {
      if let response = forwardToService(method: method, arguments: arguments) {
        return response
      }
      Thread.sleep(forTimeInterval: 0.1)
    }
    return nil
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

    let serviceHost = resolvedServiceHost()
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
    let responseTimeout: DispatchTimeInterval = method == "localStateWatch"
      ? .seconds(35)
      : .seconds(2)
    let waitResult = semaphore.wait(timeout: .now() + responseTimeout)
    connection.cancel()
    if waitResult == .timedOut {
      return nil
    }

    guard let text = String(data: response, encoding: .utf8), !text.isEmpty else {
      return nil
    }
    return text.components(separatedBy: "\n").first
  }

  private func shutdownNetworkBeforeQuit() {
    let serviceHost = resolvedServiceHost()
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

  private func resolvedServiceHost() -> String {
    bundledServiceLock.lock()
    let preferredHost = bundledServicePreferredHost
    let bundledRunning = bundledServiceProcess?.isRunning == true
    bundledServiceLock.unlock()
    if bundledRunning, let preferredHost, !preferredHost.isEmpty {
      return preferredHost
    }
    return configuredServiceHost()
  }

  private func configuredServiceHost() -> String {
    return ProcessInfo.processInfo.environment["SLAN_CLIENT_CORE_SERVICE_HOST"]
      ?? Self.defaultServiceHost
  }

  private func compactState() -> [String: Any] {
    var compact: [String: Any] = [:]
    for (key, value) in state {
      compact[key] = value ?? NSNull()
    }
    return compact
  }
}
