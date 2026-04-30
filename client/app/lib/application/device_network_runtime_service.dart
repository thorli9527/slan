import '../infra/app_core/models/identity_models.dart';
import '../infra/app_core/models/network_models.dart';

abstract class DeviceNetworkRuntimeService {
  String? assignedVirtualIpForDevice({
    required DeviceModel? device,
    required NetworkModel? network,
    required String deviceId,
  });

  DeviceModel copyDeviceWithVirtualIp(DeviceModel device, String? virtualIp);

  DeviceModel? syncDeviceVirtualIpFromNetwork({
    required DeviceModel? device,
    required NetworkModel? network,
  });
}

class DefaultDeviceNetworkRuntimeService
    implements DeviceNetworkRuntimeService {
  const DefaultDeviceNetworkRuntimeService();

  @override
  String? assignedVirtualIpForDevice({
    required DeviceModel? device,
    required NetworkModel? network,
    required String deviceId,
  }) {
    if (network != null) {
      for (final member in network.members) {
        final memberIp = member.virtualIp?.trim();
        if (member.deviceId == deviceId &&
            memberIp != null &&
            memberIp.isNotEmpty) {
          return memberIp;
        }
      }
    }
    final deviceIp = device?.virtualIp?.trim();
    if (device?.deviceId == deviceId &&
        deviceIp != null &&
        deviceIp.isNotEmpty) {
      return deviceIp;
    }
    return null;
  }

  @override
  DeviceModel? syncDeviceVirtualIpFromNetwork({
    required DeviceModel? device,
    required NetworkModel? network,
  }) {
    if (device == null) {
      return null;
    }
    final assignedIp = assignedVirtualIpForDevice(
      device: device,
      network: network,
      deviceId: device.deviceId,
    );
    if (assignedIp == null || assignedIp == device.virtualIp?.trim()) {
      return null;
    }
    return copyDeviceWithVirtualIp(device, assignedIp);
  }

  @override
  DeviceModel copyDeviceWithVirtualIp(DeviceModel device, String? virtualIp) {
    return DeviceModel(
      deviceId: device.deviceId,
      name: device.name,
      platform: device.platform,
      deviceVersion: device.deviceVersion,
      status: device.status,
      virtualIp: virtualIp,
      publicKey: device.publicKey,
      ownerEmail: device.ownerEmail,
      machineId: device.machineId,
      linkStatus: device.linkStatus,
      connectivityProtocol: device.connectivityProtocol,
      joinedAt: device.joinedAt,
      membershipStatus: device.membershipStatus,
      networkRole: device.networkRole,
      createdAt: device.createdAt,
      networkIds: device.networkIds,
      mqtt: device.mqtt,
      networkState: device.networkState,
    );
  }
}
