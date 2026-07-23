import Cocoa
import FlutterMacOS

final class KeyUpEventGuard {
  private var pressedKeyCodes = Set<UInt16>()

  func shouldForward(type: NSEvent.EventType, keyCode: UInt16) -> Bool {
    switch type {
    case .keyDown:
      pressedKeyCodes.insert(keyCode)
      return true
    case .keyUp:
      return pressedKeyCodes.remove(keyCode) != nil
    default:
      return true
    }
  }

  func reset() {
    pressedKeyCodes.removeAll()
  }
}

@main
class AppDelegate: FlutterAppDelegate {
  private let keyUpEventGuard = KeyUpEventGuard()
  private var keyEventMonitor: Any?
  private var resignActiveObserver: NSObjectProtocol?

  override func applicationDidFinishLaunching(_ notification: Notification) {
    NSApp.setActivationPolicy(.accessory)
    super.applicationDidFinishLaunching(notification)
    installKeyEventGuard()
  }

  override func applicationWillTerminate(_ notification: Notification) {
    if let keyEventMonitor {
      NSEvent.removeMonitor(keyEventMonitor)
    }
    if let resignActiveObserver {
      NotificationCenter.default.removeObserver(resignActiveObserver)
    }
    super.applicationWillTerminate(notification)
  }

  override func applicationShouldTerminateAfterLastWindowClosed(_ sender: NSApplication) -> Bool {
    return false
  }

  override func applicationSupportsSecureRestorableState(_ app: NSApplication) -> Bool {
    return true
  }

  private func installKeyEventGuard() {
    keyEventMonitor = NSEvent.addLocalMonitorForEvents(matching: [.keyDown, .keyUp]) {
      [weak self] event in
      guard let self else {
        return event
      }
      return self.keyUpEventGuard.shouldForward(type: event.type, keyCode: event.keyCode)
        ? event
        : nil
    }
    resignActiveObserver = NotificationCenter.default.addObserver(
      forName: NSApplication.didResignActiveNotification,
      object: nil,
      queue: .main
    ) { [weak self] _ in
      self?.keyUpEventGuard.reset()
    }
  }
}
