import 'dart:convert';
import 'dart:io';

abstract class DesktopUrlLauncherContract {
  Future<void> open(String url);
}

final class DesktopUrlLauncher implements DesktopUrlLauncherContract {
  const DesktopUrlLauncher();

  static const instance = DesktopUrlLauncher();

  @override
  Future<void> open(String url) async {
    if (Platform.isMacOS) {
      await Process.start('open', [url]);
      return;
    }
    if (Platform.isLinux) {
      await Process.start('xdg-open', [url]);
      return;
    }
    if (Platform.isWindows) {
      final browserPath = _windowsBrowserPath();
      if (browserPath != null) {
        await Process.start(browserPath, [url]);
        return;
      }
      final encodedUrl = base64Encode(utf8.encode(url));
      await Process.start(
        'powershell.exe',
        [
          '-NoProfile',
          '-NonInteractive',
          '-ExecutionPolicy',
          'Bypass',
          '-Command',
          r'$u=[Text.Encoding]::UTF8.GetString([Convert]::FromBase64String($env:SLAN_OPEN_URL_B64)); $p=New-Object Diagnostics.ProcessStartInfo; $p.FileName=$u; $p.UseShellExecute=$true; [Diagnostics.Process]::Start($p) | Out-Null',
        ],
        environment: {'SLAN_OPEN_URL_B64': encodedUrl},
      );
      return;
    }
    throw UnsupportedError('unsupported desktop platform');
  }

  String? _windowsBrowserPath() {
    final candidates = <String>[
      r'C:\Program Files\Google\Chrome\Application\chrome.exe',
      '${Platform.environment['LOCALAPPDATA']}\\Google\\Chrome\\Application\\chrome.exe',
      r'C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe',
      r'C:\Program Files\Microsoft\Edge\Application\msedge.exe',
      '${Platform.environment['LOCALAPPDATA']}\\Microsoft\\Edge\\Application\\msedge.exe',
    ];
    for (final candidate in candidates) {
      if (candidate.startsWith('null\\')) {
        continue;
      }
      if (File(candidate).existsSync()) {
        return candidate;
      }
    }
    return null;
  }
}
