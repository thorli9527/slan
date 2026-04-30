import 'package:flutter_test/flutter_test.dart';
import 'package:slan_app/application/device_network_runtime_service.dart';
import 'package:slan_app/infra/app_core/models/identity_models.dart';
import 'package:slan_app/infra/app_core/models/network_models.dart';

void main() {
  const service = DefaultDeviceNetworkRuntimeService();
  const device = DeviceModel(
    deviceId: 'dev-1',
    name: 'desktop',
    platform: 'windows',
    status: 'active',
    virtualIp: '10.0.0.9',
  );
  const network = NetworkModel(
    networkId: 'net-1',
    name: 'default',
    cidr: '10.0.0.0/24',
    members: [
      NetworkMemberModel(
        deviceId: 'dev-1',
        role: 'member',
        virtualIp: '10.0.0.2',
      ),
    ],
  );

  test('assignedVirtualIpForDevice prefers network assignment', () {
    final ip = service.assignedVirtualIpForDevice(
      device: device,
      network: network,
      deviceId: 'dev-1',
    );

    expect(ip, '10.0.0.2');
  });

  test('assignedVirtualIpForDevice falls back to device virtual ip', () {
    final ip = service.assignedVirtualIpForDevice(
      device: device,
      network: null,
      deviceId: 'dev-1',
    );

    expect(ip, '10.0.0.9');
  });

  test('syncDeviceVirtualIpFromNetwork copies only when changed', () {
    final synced = service.syncDeviceVirtualIpFromNetwork(
      device: device,
      network: network,
    );

    expect(synced, isNotNull);
    expect(synced!.virtualIp, '10.0.0.2');

    final unchanged = service.syncDeviceVirtualIpFromNetwork(
      device: synced,
      network: network,
    );
    expect(unchanged, isNull);
  });
}
