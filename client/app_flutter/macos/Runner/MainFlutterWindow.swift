import Cocoa
import FlutterMacOS

class MainFlutterWindow: NSWindow {
  override func awakeFromNib() {
    let flutterViewController = FlutterViewController()
    self.contentViewController = flutterViewController
    // The Flutter desktop home is a compact utility surface. Keep the native
    // content area matched to it so activated and inactive states do not
    // leave a large unused region below their actions.
    let contentSize = NSSize(width: 460, height: 176)
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
