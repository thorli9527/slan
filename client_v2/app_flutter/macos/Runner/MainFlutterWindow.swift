import Cocoa
import FlutterMacOS

class MainFlutterWindow: NSWindow {
  override func awakeFromNib() {
    let flutterViewController = FlutterViewController()
    self.contentViewController = flutterViewController
    let contentSize = NSSize(width: 480, height: 230)
    self.setContentSize(contentSize)
    self.minSize = contentSize
    self.maxSize = contentSize
    self.styleMask.remove([.miniaturizable, .resizable])
    self.standardWindowButton(.miniaturizeButton)?.isHidden = true
    self.standardWindowButton(.zoomButton)?.isHidden = true
    self.center()

    RegisterGeneratedPlugins(registry: flutterViewController)

    super.awakeFromNib()
  }
}
