import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:slan_app/application/mqtt_control_task_service.dart';

void main() {
  test('enqueueNetworkTask writes and replaces task by id', () async {
    final tempDir = Directory.systemTemp.createTempSync('slan-mqtt-task-test-');
    addTearDown(() => tempDir.deleteSync(recursive: true));
    final taskFile = '${tempDir.path}${Platform.pathSeparator}tasks.xml';
    final service = XmlMqttControlTaskService(
      tasksPathProvider: () => taskFile,
    );

    await service.enqueueNetworkTask(
      taskType: 'enable_network',
      networkId: ' net-1 ',
      deviceId: ' dev-1 ',
      uiRefreshReason: 'network_enabled',
    );
    await service.enqueueNetworkTask(
      taskType: 'enable_network',
      networkId: 'net-1',
      deviceId: 'dev-1',
      uiRefreshReason: 'network_enabled',
    );

    final payload = File(taskFile).readAsStringSync();
    expect(_count(payload, '<task '), 1);
    expect(payload, contains('id="enable_network:net-1:dev-1"'));
    expect(payload, contains('status="pending"'));
    expect(payload, contains('uiRefreshRequired="true"'));
  });

  test('markUiRefreshConsumed updates matching task only', () async {
    final tempDir = Directory.systemTemp.createTempSync('slan-mqtt-task-test-');
    addTearDown(() => tempDir.deleteSync(recursive: true));
    final taskFile = '${tempDir.path}${Platform.pathSeparator}tasks.xml';
    File(taskFile).writeAsStringSync(
      '<?xml version="1.0" encoding="utf-8"?>\n'
      '<mqttControlTasks>\n'
      '  <task id="disable_network:net-1:dev-1" type="disable_network" networkId="net-1" deviceId="dev-1" status="succeeded" attempts="1" createdAtMs="1" updatedAtMs="2" uiRefreshRequired="true" uiRefreshReason="network_disabled" uiRefreshAtMs="0" error="" />\n'
      '</mqttControlTasks>\n',
    );
    final service = XmlMqttControlTaskService(
      tasksPathProvider: () => taskFile,
    );

    service.markUiRefreshConsumed('disable_network:net-1:dev-1');

    final payload = File(taskFile).readAsStringSync();
    expect(payload, isNot(contains('uiRefreshAtMs="0"')));
    expect(payload, contains('id="disable_network:net-1:dev-1"'));
  });
}

int _count(String value, String pattern) {
  var count = 0;
  var index = 0;
  while (true) {
    index = value.indexOf(pattern, index);
    if (index < 0) {
      return count;
    }
    count += 1;
    index += pattern.length;
  }
}
