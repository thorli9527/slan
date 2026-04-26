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
}
