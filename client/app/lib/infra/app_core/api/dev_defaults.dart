/// Flutter 客户端本地联调使用的默认控制面与 relay 地址。
///
/// 这些值只用于 mock、演示页面默认表单和其他开发期占位场景，
/// 不参与正式环境配置下发。
library slan_app.infra.app_core.dev_defaults;

const String kDevControlHost = '127.0.0.1';
const int kDevControlPort = 8080;
const int kDevRelayUdpPort = 9000;
const int kDevRelayTcpPort = 9001;
const int kDevPeerEndpointPort = 40000;
const String kDevStunServer = 'stun:stun.l.google.com:19302';
const String kDevTunnelEndpoint = '203.0.113.10:51820';

const String kDevControlMqttUrl = 'mqtt://$kDevControlHost:1883';
const String kDevRelayUdpAddress = '$kDevControlHost:$kDevRelayUdpPort';
const String kDevRelayTcpAddress = '$kDevControlHost:$kDevRelayTcpPort';
const String kDevPeerEndpointAddress = '$kDevControlHost:$kDevPeerEndpointPort';
const String kDevRelayUdpUrl = 'udp://$kDevRelayUdpAddress';
