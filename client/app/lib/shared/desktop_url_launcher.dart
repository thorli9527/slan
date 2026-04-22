import 'dart:io';

final class DesktopUrlLauncher {
  DesktopUrlLauncher._();

  static Future<void> open(String url) async {
    if (Platform.isMacOS) {
      await Process.start('open', [url]);
      return;
    }
    if (Platform.isLinux) {
      await Process.start('xdg-open', [url]);
      return;
    }
    if (Platform.isWindows) {
      final escaped = url.replaceAll("'", "''");
      await Process.start(
        'powershell.exe',
        [
          '-NoProfile',
          '-NonInteractive',
          '-ExecutionPolicy',
          'Bypass',
          '-Command',
          "Start-Process '$escaped'",
        ],
      );
      return;
    }
    throw UnsupportedError('unsupported desktop platform');
  }
}
