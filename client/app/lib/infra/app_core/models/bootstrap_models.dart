import 'identity_models.dart';
import 'network_models.dart';
import 'relay_models.dart';

/// 控制面连接配置。
class ControlPlaneConfigModel {
  const ControlPlaneConfigModel({
    required this.wsUrl,
    this.sessionToken,
    required this.heartbeatSeconds,
  });

  final String wsUrl;
  final String? sessionToken;
  final int heartbeatSeconds;
}

/// 启动配置模型。
class BootstrapModel {
  const BootstrapModel({
    required this.device,
    required this.networks,
    required this.controlPlane,
    required this.stunServers,
    required this.relay,
  });

  final DeviceModel device;
  final List<NetworkModel> networks;
  final ControlPlaneConfigModel controlPlane;
  final List<String> stunServers;
  final RelayConfigModel relay;
}
