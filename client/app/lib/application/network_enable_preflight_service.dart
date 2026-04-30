import '../infra/app_core/api/app_core_api.dart';
import '../infra/app_core/models/identity_models.dart';
import '../infra/app_core/models/network_models.dart';
import 'device_network_runtime_service.dart';

abstract class NetworkEnablePreflightService {
  Future<NetworkEnablePreflightResult> verifyRemoteNetwork({
    required AppCoreApi remoteApi,
    required SessionModel? session,
    required DeviceModel? currentDevice,
    required String? preferredNetworkId,
  });
}

class DefaultNetworkEnablePreflightService
    implements NetworkEnablePreflightService {
  const DefaultNetworkEnablePreflightService({
    DeviceNetworkRuntimeService runtimeService =
        const DefaultDeviceNetworkRuntimeService(),
  }) : _runtimeService = runtimeService;

  final DeviceNetworkRuntimeService _runtimeService;

  @override
  Future<NetworkEnablePreflightResult> verifyRemoteNetwork({
    required AppCoreApi remoteApi,
    required SessionModel? session,
    required DeviceModel? currentDevice,
    required String? preferredNetworkId,
  }) async {
    if (session == null) {
      throw StateError('网络错误：无法从服务器拉取最新设备 IP 和 DNS 信息，请联系管理员。');
    }
    final currentDeviceId = currentDevice?.deviceId ?? session.deviceId;
    if (currentDeviceId == null || currentDeviceId.trim().isEmpty) {
      throw StateError('device unavailable: 当前设备尚未注册完成，请联系管理员。');
    }
    try {
      remoteApi.restoreSession(session);
      final devices = await remoteApi.listDevices();
      final refreshedDevice = _findDevice(devices, currentDeviceId.trim()) ??
          currentDevice;
      if (refreshedDevice == null) {
        throw StateError('device unavailable: 当前设备尚未注册完成，请联系管理员。');
      }

      final networks = await remoteApi.listNetworks();
      final selectedNetwork = _selectNetwork(
        networks,
        preferredNetworkId: preferredNetworkId,
      );
      final memberStatus = _currentDeviceMemberStatus(
        selectedNetwork,
        refreshedDevice.deviceId,
      );
      if (memberStatus == 'disabled' ||
          memberStatus == 'suspended' ||
          memberStatus == 'rejected') {
        throw StateError('device unavailable: 当前设备已被网络管理员停用，请联系管理员。');
      }
      final assignedIp = _runtimeService.assignedVirtualIpForDevice(
        device: refreshedDevice,
        network: selectedNetwork,
        deviceId: refreshedDevice.deviceId,
      );
      if (assignedIp == null || assignedIp.trim().isEmpty) {
        throw StateError('device unavailable: 当前设备没有分配远程 IP，请联系管理员。');
      }
      return NetworkEnablePreflightResult(
        devices: devices,
        networks: networks,
        device: _runtimeService.copyDeviceWithVirtualIp(
          refreshedDevice,
          assignedIp,
        ),
        selectedNetworkId: selectedNetwork?.networkId,
        assignedVirtualIp: assignedIp,
      );
    } on StateError {
      rethrow;
    } catch (error) {
      final message = error.toString().toLowerCase();
      if (message.contains('forbidden') ||
          message.contains('device unavailable') ||
          message.contains('no active network attachment') ||
          message.contains('has no active network attachment')) {
        throw StateError('device unavailable: 当前设备无法启用网络，请联系管理员。');
      }
      throw StateError('网络错误：无法从服务器拉取最新设备 IP 和 DNS 信息，请联系管理员。');
    }
  }

  DeviceModel? _findDevice(List<DeviceModel> devices, String deviceId) {
    for (final device in devices) {
      if (device.deviceId == deviceId) {
        return device;
      }
    }
    return null;
  }

  NetworkModel? _selectNetwork(
    List<NetworkModel> networks, {
    required String? preferredNetworkId,
  }) {
    final preferred = preferredNetworkId?.trim();
    if (preferred != null && preferred.isNotEmpty) {
      for (final network in networks) {
        if (network.networkId == preferred) {
          return network;
        }
      }
    }
    return networks.isEmpty ? null : networks.first;
  }

  String? _currentDeviceMemberStatus(NetworkModel? network, String deviceId) {
    if (network == null) {
      return null;
    }
    for (final member in network.members) {
      if (member.deviceId == deviceId) {
        return member.status?.trim().toLowerCase();
      }
    }
    return null;
  }
}

class NetworkEnablePreflightResult {
  const NetworkEnablePreflightResult({
    required this.devices,
    required this.networks,
    required this.device,
    required this.selectedNetworkId,
    required this.assignedVirtualIp,
  });

  final List<DeviceModel> devices;
  final List<NetworkModel> networks;
  final DeviceModel device;
  final String? selectedNetworkId;
  final String assignedVirtualIp;
}
