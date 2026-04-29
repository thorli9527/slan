import 'package:flutter_test/flutter_test.dart';
import 'package:slan_app/application/tunnel_configuration_service.dart';
import 'package:slan_app/infra/app_core/models/network_models.dart';

void main() {
  test('uses active network CIDR prefix for Windows adapter address', () {
    const service = TunnelConfigurationService();

    final config = service.buildActiveNetworkConfiguration(
      network: const NetworkModel(
        networkId: 'net-1',
        name: 'default',
        cidr: '10.0.0.0/16',
        members: [
          NetworkMemberModel(
            deviceId: 'dev-1',
            role: 'owner',
            virtualIp: '10.0.0.2',
          ),
        ],
      ),
      deviceId: 'dev-1',
      devicePublicKey: 'pub',
    );

    expect(config.interface.addresses, ['10.0.0.2/16']);
  });

  test('falls back to host prefix when network CIDR is invalid', () {
    const service = TunnelConfigurationService();

    final config = service.buildActiveNetworkConfiguration(
      network: const NetworkModel(
        networkId: 'net-1',
        name: 'default',
        cidr: 'not-a-cidr',
        members: [
          NetworkMemberModel(
            deviceId: 'dev-1',
            role: 'owner',
            virtualIp: '10.0.0.2',
          ),
        ],
      ),
      deviceId: 'dev-1',
      devicePublicKey: 'pub',
    );

    expect(config.interface.addresses, ['10.0.0.2/32']);
  });

  test('uses localhost DNS only when wildcard DNS is configured', () {
    const service = TunnelConfigurationService();

    final disabled = service.buildActiveNetworkConfiguration(
      network: const NetworkModel(
        networkId: 'net-1',
        name: 'default',
        cidr: '10.0.0.0/24',
        members: [
          NetworkMemberModel(
            deviceId: 'dev-1',
            role: 'owner',
            virtualIp: '10.0.0.2',
          ),
        ],
      ),
      deviceId: 'dev-1',
      devicePublicKey: 'pub',
    );
    final enabled = service.buildActiveNetworkConfiguration(
      network: const NetworkModel(
        networkId: 'net-1',
        name: 'default',
        cidr: '10.0.0.0/24',
        dns: DNSConfigModel(wildcards: ['*.xx.com=10.0.0.2']),
        members: [
          NetworkMemberModel(
            deviceId: 'dev-1',
            role: 'owner',
            virtualIp: '10.0.0.2',
          ),
        ],
      ),
      deviceId: 'dev-1',
      devicePublicKey: 'pub',
    );

    expect(disabled.interface.dnsServers, isEmpty);
    expect(enabled.interface.dnsServers, ['127.0.0.1']);
  });

  test('prefers current device virtual IP over stale network member IP', () {
    const service = TunnelConfigurationService();

    final config = service.buildActiveNetworkConfiguration(
      network: const NetworkModel(
        networkId: 'net-1',
        name: 'default',
        cidr: '10.0.0.0/24',
        members: [
          NetworkMemberModel(
            deviceId: 'dev-1',
            role: 'owner',
            virtualIp: '10.0.0.10',
          ),
        ],
      ),
      deviceId: 'dev-1',
      devicePublicKey: 'pub',
      deviceVirtualIp: '10.0.0.2',
    );

    expect(config.localVirtualIp, '10.0.0.2');
    expect(config.interface.addresses, ['10.0.0.2/24']);
  });

  test('throws when current device has no assigned virtual IP', () {
    const service = TunnelConfigurationService();

    expect(
      () => service.buildActiveNetworkConfiguration(
        network: const NetworkModel(
          networkId: 'net-1',
          name: 'default',
          cidr: '10.0.0.0/24',
        ),
        deviceId: 'dev-1',
        devicePublicKey: 'pub',
      ),
      throwsA(isA<StateError>()),
    );
  });
}
