library slan_app.application.tunnel_configuration_service;

import 'package:slan_app_core_plugin/slan_app_core_plugin.dart';

import '../infra/app_core/api/dev_defaults.dart';
import '../infra/app_core/models/network_models.dart';

class TunnelConfigurationService {
  const TunnelConfigurationService();

  static const windowsTunnelInterfaceName = 'SLAN LAN Adapter';

  WireGuardTunnelConfiguration buildActiveNetworkConfiguration({
    required NetworkModel network,
    required String deviceId,
    required String? devicePublicKey,
  }) {
    final localVirtualIp = _localVirtualIpFor(network, deviceId);
    final peerVirtualIp = _peerVirtualIpFor(network, deviceId, localVirtualIp);
    final interfaceAddress = _interfaceAddressFor(localVirtualIp, network.cidr);
    return WireGuardTunnelConfiguration(
      transport: 'relay',
      localVirtualIp: localVirtualIp,
      peerVirtualIp: peerVirtualIp,
      interface: WireGuardTunnelInterfaceConfiguration(
        interfaceName: windowsTunnelInterfaceName,
        keyPair: WireGuardTunnelKeyPair(
          privateKey: 'debug-private-key',
          publicKey: devicePublicKey?.trim().isNotEmpty == true
              ? devicePublicKey!.trim()
              : 'debug-public-key',
        ),
        listenPort: 51820,
        mtu: 1280,
        addresses: [interfaceAddress],
        dnsServers: network.dns.enabled ? const ['127.0.0.1'] : const [],
      ),
      peer: WireGuardTunnelPeerConfiguration(
        publicKey: 'peer-debug-public-key',
        endpoint: kDevTunnelEndpoint,
        allowedIps: ['$peerVirtualIp/32'],
      ),
    );
  }
}

String _localVirtualIpFor(NetworkModel network, String deviceId) {
  for (final member in network.members) {
    if (member.deviceId == deviceId &&
        member.virtualIp != null &&
        member.virtualIp!.trim().isNotEmpty) {
      return member.virtualIp!.trim();
    }
  }
  return '10.0.0.10';
}

String _peerVirtualIpFor(
  NetworkModel network,
  String deviceId,
  String localVirtualIp,
) {
  for (final member in network.members) {
    final candidate = member.virtualIp?.trim();
    if (member.deviceId != deviceId &&
        candidate != null &&
        candidate.isNotEmpty) {
      return candidate;
    }
  }
  if (localVirtualIp == '10.0.0.2') {
    return '10.0.0.3';
  }
  return '10.0.0.2';
}

String _interfaceAddressFor(String localVirtualIp, String networkCidr) {
  final prefix = _prefixLengthFromCidr(networkCidr);
  return '$localVirtualIp/$prefix';
}

int _prefixLengthFromCidr(String cidr) {
  final slash = cidr.trim().lastIndexOf('/');
  if (slash <= 0 || slash == cidr.length - 1) {
    return 32;
  }
  final parsed = int.tryParse(cidr.substring(slash + 1).trim());
  if (parsed == null || parsed < 0 || parsed > 32) {
    return 32;
  }
  return parsed;
}
