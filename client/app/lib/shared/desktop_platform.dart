library slan_app.shared.desktop_platform;

import 'dart:io';

final class DesktopPlatform {
  const DesktopPlatform._();

  static String get currentId {
    if (Platform.isWindows) {
      return 'windows';
    }
    if (Platform.isLinux) {
      return 'linux';
    }
    if (Platform.isMacOS) {
      return 'macos';
    }
    return 'unknown';
  }

  static String get currentVersion => Platform.operatingSystemVersion;

  static String get currentLabel {
    if (Platform.isWindows) {
      return 'Windows';
    }
    if (Platform.isLinux) {
      return 'Linux';
    }
    if (Platform.isMacOS) {
      return 'macOS';
    }
    return 'Desktop';
  }

  static bool get supportsNativeTunnel {
    if (Platform.isWindows) {
      return true;
    }
    return true;
  }

  static String get nativeTunnelLabel {
    if (Platform.isWindows) {
      return 'Wintun';
    }
    if (Platform.isMacOS) {
      return 'PacketTunnel';
    }
    if (Platform.isLinux) {
      return 'WireGuard';
    }
    return 'Tunnel';
  }
}
