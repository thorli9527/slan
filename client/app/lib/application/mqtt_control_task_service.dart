import 'dart:io';

typedef MqttControlTaskPathProvider = String Function();

abstract class MqttControlTaskService {
  Future<void> enqueueNetworkTask({
    required String taskType,
    required String networkId,
    required String deviceId,
    required String uiRefreshReason,
  });

  void markUiRefreshConsumed(String? taskId);
}

class XmlMqttControlTaskService implements MqttControlTaskService {
  const XmlMqttControlTaskService({
    MqttControlTaskPathProvider? tasksPathProvider,
  }) : _tasksPathProvider = tasksPathProvider;

  final MqttControlTaskPathProvider? _tasksPathProvider;

  @override
  Future<void> enqueueNetworkTask({
    required String taskType,
    required String networkId,
    required String deviceId,
    required String uiRefreshReason,
  }) async {
    final targetNetworkId = networkId.trim();
    final targetDeviceId = deviceId.trim();
    if (targetDeviceId.isEmpty) {
      throw StateError('当前设备尚未注册完成。');
    }
    if (targetNetworkId.isEmpty) {
      throw StateError('当前没有可操作的网络。');
    }
    final now = DateTime.now().millisecondsSinceEpoch;
    final id = '$taskType:$targetNetworkId:$targetDeviceId';
    final file = File(_tasksPath());
    if (!file.parent.existsSync()) {
      file.parent.createSync(recursive: true);
    }
    final lines = file.existsSync()
        ? file.readAsLinesSync()
        : const <String>[
            '<?xml version="1.0" encoding="utf-8"?>',
            '<mqttControlTasks>',
            '</mqttControlTasks>',
          ];
    final nextTaskLine = _taskLine(
      id: id,
      taskType: taskType,
      networkId: targetNetworkId,
      deviceId: targetDeviceId,
      now: now,
      reason: uiRefreshReason,
    );
    var replaced = false;
    final output = <String>[];
    for (final line in lines) {
      final trimmed = line.trim();
      if (trimmed.startsWith('<task ') && _xmlAttr(trimmed, 'id') == id) {
        output.add(nextTaskLine);
        replaced = true;
        continue;
      }
      if (trimmed == '</mqttControlTasks>' && !replaced) {
        output.add(nextTaskLine);
        replaced = true;
      }
      output.add(line);
    }
    if (output.isEmpty || !output.first.trim().startsWith('<?xml')) {
      output.insert(0, '<?xml version="1.0" encoding="utf-8"?>');
    }
    if (!output.any((line) => line.trim() == '<mqttControlTasks>')) {
      output.insert(1, '<mqttControlTasks>');
    }
    if (!output.any((line) => line.trim() == '</mqttControlTasks>')) {
      if (!replaced) {
        output.add(nextTaskLine);
      }
      output.add('</mqttControlTasks>');
    }
    file.writeAsStringSync('${output.join('\n')}\n');
  }

  @override
  void markUiRefreshConsumed(String? taskId) {
    final targetTaskId = taskId?.trim();
    if (targetTaskId == null || targetTaskId.isEmpty) {
      return;
    }
    final file = File(_tasksPath());
    if (!file.existsSync()) {
      return;
    }
    final now = DateTime.now().millisecondsSinceEpoch;
    final lines = file.readAsLinesSync();
    var changed = false;
    final output = lines.map((line) {
      final trimmed = line.trim();
      if (!trimmed.startsWith('<task ') ||
          _xmlAttr(trimmed, 'id') != targetTaskId) {
        return line;
      }
      changed = true;
      return _replaceXmlAttr(line, 'uiRefreshAtMs', '$now');
    }).toList(growable: false);
    if (changed) {
      file.writeAsStringSync('${output.join('\n')}\n');
    }
  }

  String _tasksPath() {
    final provided = _tasksPathProvider?.call().trim();
    if (provided != null && provided.isNotEmpty) {
      return provided;
    }
    return defaultTasksPath();
  }

  static String defaultTasksPath() {
    final configured = Platform.environment['SLAN_MQTT_CONTROL_TASKS_FILE'];
    if (configured != null && configured.trim().isNotEmpty) {
      return configured.trim();
    }
    final programData = Platform.environment['ProgramData'];
    final root = programData != null && programData.trim().isNotEmpty
        ? programData.trim()
        : r'C:\ProgramData';
    return '$root\\SLAN\\mqtt-control-tasks.xml';
  }

  String _taskLine({
    required String id,
    required String taskType,
    required String networkId,
    required String deviceId,
    required int now,
    required String reason,
  }) {
    return '  <task'
        ' id="${_xmlEscape(id)}"'
        ' type="${_xmlEscape(taskType)}"'
        ' networkId="${_xmlEscape(networkId)}"'
        ' deviceId="${_xmlEscape(deviceId)}"'
        ' status="pending"'
        ' attempts="0"'
        ' createdAtMs="$now"'
        ' updatedAtMs="$now"'
        ' uiRefreshRequired="true"'
        ' uiRefreshReason="${_xmlEscape(reason)}"'
        ' uiRefreshAtMs="0"'
        ' error=""'
        ' />';
  }

  String? _xmlAttr(String line, String name) {
    final needle = '$name="';
    final start = line.indexOf(needle);
    if (start < 0) {
      return null;
    }
    final valueStart = start + needle.length;
    final end = line.indexOf('"', valueStart);
    if (end < 0) {
      return null;
    }
    return _xmlUnescape(line.substring(valueStart, end));
  }

  String _xmlEscape(String value) => value
      .replaceAll('&', '&amp;')
      .replaceAll('"', '&quot;')
      .replaceAll('<', '&lt;')
      .replaceAll('>', '&gt;');

  String _xmlUnescape(String value) => value
      .replaceAll('&quot;', '"')
      .replaceAll('&gt;', '>')
      .replaceAll('&lt;', '<')
      .replaceAll('&amp;', '&');

  String _replaceXmlAttr(String line, String name, String value) {
    final escapedValue = _xmlEscape(value);
    final pattern = RegExp('$name="[^"]*"');
    if (pattern.hasMatch(line)) {
      return line.replaceFirst(pattern, '$name="$escapedValue"');
    }
    final insertAt = line.lastIndexOf('/>');
    if (insertAt >= 0) {
      return '${line.substring(0, insertAt).trimRight()} '
          '$name="$escapedValue" ${line.substring(insertAt)}';
    }
    return line;
  }
}
