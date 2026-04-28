import Cocoa
import FlutterMacOS

class MainFlutterWindow: NSWindow {
  override func awakeFromNib() {
    let flutterViewController = FlutterViewController()
    let fixedSize = NSSize(width: 960, height: 560)
    let windowFrame = NSRect(origin: self.frame.origin, size: fixedSize)
    self.minSize = fixedSize
    self.maxSize = fixedSize
    self.contentViewController = flutterViewController
    self.setFrame(windowFrame, display: true)
    self.styleMask.remove(.resizable)

    RegisterGeneratedPlugins(registry: flutterViewController)

    super.awakeFromNib()
  }
}
