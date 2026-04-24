library slan_app.application.auth_session_service;

import '../infra/app_core/api/app_core_api.dart';
import '../infra/app_core/models/identity_models.dart';
import '../infra/app_core/models/network_models.dart';

class AuthSessionHydrationResult {
  const AuthSessionHydrationResult({
    required this.session,
    required this.device,
    required this.devices,
    required this.networks,
    required this.notice,
  });

  final SessionModel session;
  final DeviceModel? device;
  final List<DeviceModel> devices;
  final List<NetworkModel> networks;
  final String notice;
}

class AuthSessionService {
  const AuthSessionService({
    required AppCoreApi Function() apiProvider,
  }) : _apiProvider = apiProvider;

  final AppCoreApi Function() _apiProvider;

  AppCoreApi get _api => _apiProvider();

  Future<AuthSessionHydrationResult> hydrateExternalSession(
    SessionModel session,
  ) async {
    _api.restoreSession(session);
    final devices = await _api.listDevices();
    final preferredDeviceId = session.deviceId?.trim();

    DeviceModel? matchedDevice;
    if (preferredDeviceId == null || preferredDeviceId.isEmpty) {
      matchedDevice = devices.isNotEmpty ? devices.first : null;
    } else {
      for (final device in devices) {
        if (device.deviceId == preferredDeviceId) {
          matchedDevice = device;
          break;
        }
      }
    }

    final networks = await _api.listNetworks();
    final notice = networks.isEmpty ? '登录成功，当前还没有活动网络。' : '登录成功，已进入默认主页。';

    return AuthSessionHydrationResult(
      session: session,
      device: matchedDevice,
      devices: devices,
      networks: networks,
      notice: notice,
    );
  }
}
