library slan_app.application.tunnel_configuration_service;

import 'package:slan_app_core_plugin/slan_app_core_plugin.dart';

import '../infra/app_core/api/dev_defaults.dart';
import '../infra/app_core/models/network_models.dart';

class TunnelConfigurationService {
  const TunnelConfigurationService();

  WireGuardTunnelConfiguration buildActiveNetworkConfiguration({
    required NetworkModel network,
    required String deviceId,
    required String? devicePublicKey,
  }) {
    final localVirtualIp = _localVirtualIpFor(network, deviceId);
    final peerVirtualIp = _peerVirtualIpFor(network, deviceId, localVirtualIp);
    return WireGuardTunnelConfiguration(
      transport: 'relay',
      localVirtualIp: localVirtualIp,
      peerVirtualIp: peerVirtualIp,
      interface: WireGuardTunnelInterfaceConfiguration(
        keyPair: WireGuardTunnelKeyPair(
          privateKey: 'debug-private-key',
          publicKey: devicePublicKey?.trim().isNotEmpty == true
              ? devicePublicKey!.trim()
              : 'debug-public-key',
        ),
        listenPort: 51820,
        mtu: 1280,
        addresses: ['$localVirtualIp/32'],
        dnsServers: const ['1.1.1.1'],
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
