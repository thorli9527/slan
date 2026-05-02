import 'dart:async';
import 'dart:convert';
import 'dart:io';

import 'package:flutter/foundation.dart';

import 'client_view_state.dart';

class ClientUiDiagnostics {
  const ClientUiDiagnostics._();

  static const bool enabled = bool.fromEnvironment(
    'SLAN_UI_DIAGNOSTICS',
    defaultValue: false,
  );
  static Future<void> _writeQueue = Future<void>.value();

  static Future<void> log(
    String event, {
    ClientViewState? state,
    Map<String, Object?> fields = const {},
  }) async {
    if (!enabled) {
      return;
    }
    final entry = <String, Object?>{
      'ts': DateTime.now().toIso8601String(),
      'event': event,
      if (state != null) 'state': state.toDiagnosticsJson(),
      ...fields,
    };
    final line = jsonEncode(entry);
    debugPrint('SLAN_UI $line');
    await _enqueueWrite(line);
  }

  static void unawaitedLog(
    String event, {
    ClientViewState? state,
    Map<String, Object?> fields = const {},
  }) {
    log(event, state: state, fields: fields).ignore();
  }

  static Future<void> _enqueueWrite(String line) {
    _writeQueue = _writeQueue.then((_) async {
      try {
        final file = File(_logPath());
        await file.parent.create(recursive: true);
        await file.writeAsString('$line\n', mode: FileMode.append);
      } on Object {
        // Diagnostics must never affect UI behavior.
      }
    });
    return _writeQueue;
  }

  static String _logPath() {
    if (Platform.isWindows) {
      final programData =
          Platform.environment['ProgramData'] ?? r'C:\ProgramData';
      return '$programData\\SLAN\\client-v2-ui.log';
    }
    final tmp = Directory.systemTemp.path;
    return '$tmp/slan/client-v2-ui.log';
  }
}

extension ClientViewStateDiagnostics on ClientViewState {
  Map<String, Object?> toDiagnosticsJson() {
    return {
      'signedIn': signedIn,
      'networkEnabled': networkEnabled,
      'syncing': syncing,
      'syncReason': syncReason,
      'switchEnabled': switchEnabled,
      'virtualIp': virtualIp,
      'notice': notice,
      'error': error,
      'hasUserLabel': userLabel?.trim().isNotEmpty == true,
      'hasDeviceId': deviceId?.trim().isNotEmpty == true,
    };
  }
}
