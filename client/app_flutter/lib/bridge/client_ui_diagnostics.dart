import 'dart:async';
import 'dart:convert';
import 'dart:io';

import 'package:flutter/foundation.dart';

import 'client_view_state.dart';

/// Flutter UI 诊断日志工具。
///
/// 默认关闭，只有通过 `--dart-define=SLAN_UI_DIAGNOSTICS=true` 打开后才写日志。
/// 诊断日志不能影响用户流程，因此写入失败会被吞掉。
class ClientUiDiagnostics {
  const ClientUiDiagnostics._();

  /// 是否启用 UI 诊断。
  static const bool enabled = bool.fromEnvironment(
    'SLAN_UI_DIAGNOSTICS',
    defaultValue: false,
  );

  /// 串行写入队列，避免并发追加日志造成行交错。
  static Future<void> _writeQueue = Future<void>.value();

  /// 写入一条结构化诊断事件。
  ///
  /// [state] 会被裁剪成不含敏感信息的摘要；[fields] 用于补充当前事件
  /// 独有的上下文，例如命令名、错误消息或平台状态。
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

  /// 始终记录影响用户操作的关键事件，不受诊断构建开关控制。
  ///
  /// 调用方不得在 [fields] 中传入 token、登录 key 或完整设备标识。
  static Future<void> logCritical(
    String event, {
    ClientViewState? state,
    Map<String, Object?> fields = const {},
  }) async {
    final entry = <String, Object?>{
      'ts': DateTime.now().toIso8601String(),
      'event': event,
      if (state != null) 'state': state.toDiagnosticsJson(),
      ...fields,
    };
    await _enqueueWrite(jsonEncode(entry));
  }

  /// fire-and-forget 版本，适合 UI 事件处理里调用。
  static void unawaitedLog(
    String event, {
    ClientViewState? state,
    Map<String, Object?> fields = const {},
  }) {
    log(event, state: state, fields: fields).ignore();
  }

  /// fire-and-forget 的关键事件日志。
  static void unawaitedCriticalLog(
    String event, {
    ClientViewState? state,
    Map<String, Object?> fields = const {},
  }) {
    logCritical(event, state: state, fields: fields).ignore();
  }

  /// 把日志追加操作接到串行队列末尾。
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

  /// 根据平台返回诊断日志路径。
  ///
  /// Windows 写入 ProgramData，其他平台写入系统临时目录，避免普通用户
  /// 权限不足时影响 UI。
  static String _logPath() {
    if (Platform.isWindows) {
      final programData =
          Platform.environment['ProgramData'] ?? r'C:\ProgramData';
      return '$programData\\SLAN\\client-v2-ui.log';
    }
    if (Platform.isMacOS) {
      return '/tmp/slan/client-v2-ui.log';
    }
    final tmp = Directory.systemTemp.path;
    return '$tmp/slan/client-v2-ui.log';
  }
}

/// 把 UI 状态转换成诊断 JSON 的扩展。
///
/// 这里只保留布尔、状态码和“是否存在”类信息，不输出用户邮箱、设备 ID
/// 等完整敏感值。
extension ClientViewStateDiagnostics on ClientViewState {
  /// 生成可写入日志的状态摘要。
  Map<String, Object?> toDiagnosticsJson() {
    return {
      'activated': activated,
      'networkEnabled': networkEnabled,
      'syncing': syncing,
      'syncReason': syncReason,
      'switchEnabled': switchEnabled,
      'virtualIp': virtualIp,
      'notice': notice,
      'error': error,
      'hasDeviceId': deviceId?.trim().isNotEmpty == true,
    };
  }
}
