import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:slan_app/infra/logging/startup_log.dart';

void main() {
  test('StartupLog writes to the system temp directory', () async {
    final logFile = File(
      '${Directory.systemTemp.path}${Platform.pathSeparator}slan_app_startup.log',
    );
    final marker = 'test entry ${DateTime.now().microsecondsSinceEpoch}';

    await StartupLog.write(marker);

    expect(await logFile.exists(), isTrue);
    final contents = await logFile.readAsString();
    expect(contents, contains(marker));
  });
}
