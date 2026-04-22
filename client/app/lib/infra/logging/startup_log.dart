library slan_app.infra.logging.startup_log;

import 'dart:io';

import 'package:flutter/foundation.dart';

final class StartupLog {
  StartupLog._();

  static final File _file = File(
    '${Directory.systemTemp.path}${Platform.pathSeparator}slan_app_startup.log',
  );

  static Future<void> write(String message) async {
    final line =
        '${DateTime.now().toIso8601String()} ${message.trimRight()}\n';
    debugPrint(line.trimRight());
    try {
      await _file.parent.create(recursive: true);
      await _file.writeAsString(line, mode: FileMode.append, flush: true);
    } catch (_) {
      // Logging must never break startup.
    }
  }

  static Future<void> reset() async {
    try {
      await _file.parent.create(recursive: true);
      if (await _file.exists()) {
        await _file.writeAsString('', flush: true);
      }
    } catch (_) {
      // Best effort only.
    }
  }
}
