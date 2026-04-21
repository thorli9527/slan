import 'package:flutter/foundation.dart';

abstract final class AppTestKeys {
  static const dashboardTab = ValueKey<String>('home.tab.dashboard');
  static const networksTab = ValueKey<String>('home.tab.networks');
  static const devicesTab = ValueKey<String>('home.tab.devices');
  static const homeOpenNetworksButton =
      ValueKey<String>('home.open_networks_button');
  static const homeOpenDevicesButton =
      ValueKey<String>('home.open_devices_button');
  static const homeEnableNetworkButton =
      ValueKey<String>('home.enable_network_button');
  static const homeDisableNetworkButton =
      ValueKey<String>('home.disable_network_button');
  static const homeManageNetworkButton =
      ValueKey<String>('home.manage_network_button');
  static const homeLoginButton = ValueKey<String>('home.login_button');
  static const homeSettingsButton = ValueKey<String>('home.settings_button');
  static const homeAddButton = ValueKey<String>('home.add_button');
  static const homeLogoutButton = ValueKey<String>('home.logout_button');

  static const authEmailField = ValueKey<String>('auth.email');
  static const authPasswordField = ValueKey<String>('auth.password');
  static const authRegisterButton = ValueKey<String>('auth.register');
  static const authLoginButton = ValueKey<String>('auth.login');
  static const authOpenLoginButton = ValueKey<String>('auth.open_login');
  static const authOpenConsoleButton = ValueKey<String>('auth.open_console');
  static const authHostField = ValueKey<String>('auth.host');
  static const authApplyHostButton = ValueKey<String>('auth.apply_host');

  static const networksNameField = ValueKey<String>('networks.name');
  static const networksCidrField = ValueKey<String>('networks.cidr');
  static const networksCreateButton =
      ValueKey<String>('networks.create_button');
  static const networksRefreshButton =
      ValueKey<String>('networks.refresh_button');

  static const devicesNameField = ValueKey<String>('devices.name');
  static const devicesScrollView = ValueKey<String>('devices.scroll_view');
  static const devicesPlatformField = ValueKey<String>('devices.platform');
  static const devicesMachineIdField = ValueKey<String>('devices.machine_id');
  static const devicesPublicKeyField = ValueKey<String>('devices.public_key');
  static const devicesRegisterDeviceButton =
      ValueKey<String>('devices.register_device_button');
  static const devicesNodeIdField = ValueKey<String>('devices.node_id');
  static const devicesNodePublicKeyField =
      ValueKey<String>('devices.node_public_key');
  static const devicesRegisterNodeButton =
      ValueKey<String>('devices.register_node_button');
  static const devicesBootstrapNodeIdField =
      ValueKey<String>('devices.bootstrap.node_id');
  static const devicesBootstrapNetworkIdField =
      ValueKey<String>('devices.bootstrap.network_id');
  static const devicesLoadBootstrapButton =
      ValueKey<String>('devices.bootstrap.load_button');
  static const devicesPeerNodeIdField =
      ValueKey<String>('devices.connect.peer_node_id');
  static const devicesFailureReasonField =
      ValueKey<String>('devices.connect.failure_reason');
  static const devicesConnectButton =
      ValueKey<String>('devices.connect.connect_button');
  static const devicesDisconnectButton =
      ValueKey<String>('devices.connect.disconnect_button');
  static const devicesProbePayloadField =
      ValueKey<String>('devices.probe.payload');
  static const devicesProbeTimeoutField =
      ValueKey<String>('devices.probe.timeout_ms');
  static const devicesProbeButton = ValueKey<String>('devices.probe.button');
  static const devicesSendPayloadField =
      ValueKey<String>('devices.send.payload');
  static const devicesSendButton = ValueKey<String>('devices.send.button');
  static const devicesTunnelLocalIpField =
      ValueKey<String>('devices.tunnel.local_ip');
  static const devicesTunnelPeerIpField =
      ValueKey<String>('devices.tunnel.peer_ip');
  static const devicesTunnelPrivateKeyField =
      ValueKey<String>('devices.tunnel.private_key');
  static const devicesTunnelPublicKeyField =
      ValueKey<String>('devices.tunnel.public_key');
  static const devicesTunnelPeerPublicKeyField =
      ValueKey<String>('devices.tunnel.peer_public_key');
  static const devicesTunnelEndpointField =
      ValueKey<String>('devices.tunnel.endpoint');
  static const devicesTunnelDebugEngineModeField =
      ValueKey<String>('devices.tunnel.debug_engine_mode');
  static const devicesTunnelApplyButton =
      ValueKey<String>('devices.tunnel.apply_button');
  static const devicesTunnelUpButton =
      ValueKey<String>('devices.tunnel.up_button');
  static const devicesTunnelViewButton =
      ValueKey<String>('devices.tunnel.view_button');
  static const devicesTunnelDownButton =
      ValueKey<String>('devices.tunnel.down_button');
  static const devicesTunnelRemoveButton =
      ValueKey<String>('devices.tunnel.remove_button');
  static const devicesStateCard = ValueKey<String>('devices.state_card');
}
