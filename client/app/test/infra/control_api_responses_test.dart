import 'package:flutter_test/flutter_test.dart';
import 'package:slan_app/infra/app_core/api/dev_defaults.dart';
import 'package:slan_app/infra/control_api_responses/response_parsers.dart';

void main() {
  test('parseBootstrapResponse maps four-level relay topology into app models',
      () {
    final bootstrap = parseBootstrapResponse({
      'device': {
        'device': {
          'deviceId': 'dev-1',
          'name': 'macbook',
          'platform': 'macos',
          'status': 'online',
          'publicKey': 'device-pub',
        },
        'attachments': [
          {
            'networkId': 'net-1',
            'subnetId': 'subnet-1',
            'deviceId': 'dev-1',
            'virtualIp': '100.64.0.10',
          },
        ],
      },
      'networks': [
        {
          'networkId': 'net-1',
          'name': 'home',
          'defaultSubnetCidr': '100.64.0.0/24',
          'subnets': [
            {
              'subnetId': 'subnet-1',
              'networkId': 'net-1',
              'name': 'default',
              'cidr': '100.64.0.0/24',
              'isDefault': true,
            },
          ],
          'members': [
            {
              'memberId': 'member-1',
              'networkId': 'net-1',
              'deviceId': 'dev-1',
              'role': 'owner',
            },
            {
              'memberId': 'member-2',
              'networkId': 'net-1',
              'deviceId': 'dev-2',
              'role': 'member',
            },
          ],
        },
      ],
      'controlPlane': {
        'wsUrl': kDevControlWsUrl,
        'heartbeatSeconds': 15,
      },
      'stunServers': [kDevStunServer],
      'relay': {
        'defaultClusterId': 'cn-sh-a',
        'countries': [
          {
            'countryCode': 'CN',
            'countryName': 'China',
            'cities': [
              {
                'cityCode': 'sh',
                'cityName': 'Shanghai',
                'clusters': [
                  {
                    'clusterId': 'cn-sh-a',
                    'clusterName': 'Shanghai A',
                    'nodes': [
                      {
                        'nodeId': 'relay-cn-sh-udp',
                        'transport': 'udp',
                        'address': kDevRelayUdpAddress,
                        'priority': 10,
                        'tags': ['default'],
                      },
                      {
                        'nodeId': 'relay-cn-sh-tcp',
                        'transport': 'tcp',
                        'address': kDevRelayTcpAddress,
                        'priority': 20,
                      },
                    ],
                  },
                ],
              },
            ],
          },
        ],
      },
      'networkMap': {
        'networkId': 'net-1',
      },
    });

    expect(bootstrap.device.deviceId, 'dev-1');
    expect(bootstrap.device.virtualIp, '100.64.0.10');
    expect(bootstrap.networks, hasLength(1));
    expect(bootstrap.networks.first.cidr, '100.64.0.0/24');
    expect(bootstrap.networks.first.members.first.virtualIp, '100.64.0.10');

    expect(bootstrap.relay.defaultClusterId, 'cn-sh-a');
    expect(bootstrap.relay.countries, hasLength(1));
    expect(bootstrap.relay.countries.first.countryCode, 'CN');
    expect(bootstrap.relay.countries.first.cities, hasLength(1));
    expect(bootstrap.relay.countries.first.cities.first.cityCode, 'sh');
    expect(bootstrap.relay.countries.first.cities.first.clusters, hasLength(1));
    expect(
      bootstrap.relay.countries.first.cities.first.clusters.first.clusterId,
      'cn-sh-a',
    );
    expect(
      bootstrap.relay.countries.first.cities.first.clusters.first.nodes
          .map((node) => node.nodeId),
      ['relay-cn-sh-udp', 'relay-cn-sh-tcp'],
    );
  });

  test('parseRelayTicketResponse keeps derp cluster and allowed node ids', () {
    final ticket = parseRelayTicketResponse({
      'ticketId': 'ticket-1',
      'networkId': 'net-1',
      'sessionId': 'session-1',
      'srcNodeId': 'node-a',
      'dstNodeId': 'node-b',
      'derpClusterId': 'cn-sh-a',
      'countryCode': 'CN',
      'cityCode': 'sh',
      'allowedDerpNodeIds': ['relay-cn-sh-udp', 'relay-cn-sh-tcp'],
      'relayUrl': kDevRelayUdpUrl,
      'expiresAt': '2026-04-17T10:00:00Z',
      'sessionKey': 'opaque-session-key',
      'signature': 'signed-payload',
    });

    expect(ticket.ticketId, 'ticket-1');
    expect(ticket.derpClusterId, 'cn-sh-a');
    expect(ticket.allowedDerpNodeIds, ['relay-cn-sh-udp', 'relay-cn-sh-tcp']);
    expect(ticket.relayUrl, kDevRelayUdpUrl);
    expect(ticket.signature, 'signed-payload');
  });
}
