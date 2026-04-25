import 'package:flutter_test/flutter_test.dart';
import 'package:slan_app/infra/app_core/api/dev_defaults.dart';
import 'package:slan_app/infra/control_api_responses/response_dtos.dart';
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
          'machineId': 'machine-1',
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
              'createdAt': 1713340000,
              'status': 'active',
            },
            {
              'memberId': 'member-2',
              'networkId': 'net-1',
              'deviceId': 'dev-2',
              'role': 'member',
              'createdAt': 1713340100,
              'status': 'pending',
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
      'derpMap': {
        'probeIntervalSeconds': 30,
        'clusters': [
          {
            'clusterId': 'cn-sh-a',
            'clusterName': 'Shanghai A',
            'regionId': 'cn-sh',
            'regionName': 'Shanghai',
            'countryCode': 'CN',
            'countryName': 'China',
            'cityCode': 'sh',
            'cityName': 'Shanghai',
            'recommendedFanout': 1,
            'nodes': [
              {
                'nodeId': 'relay-cn-sh-udp',
                'host': '127.0.0.1',
                'port': 19000,
                'transport': 'udp',
                'priority': 10,
                'tags': ['default'],
              },
              {
                'nodeId': 'relay-cn-sh-tcp',
                'host': '127.0.0.1',
                'port': 19001,
                'transport': 'tcp',
                'priority': 20,
              },
            ],
          },
        ],
      },
      'networkMap': {
        'selfUserId': 'user-1',
        'selfDeviceId': 'dev-1',
        'selfNodeId': 'node-1',
        'networkId': 'net-1',
        'revision': 7,
        'heartbeatSeconds': 15,
        'stunServers': [kDevStunServer],
        'peers': [
          {
            'nodeId': 'node-2',
            'deviceId': 'dev-2',
            'publicKey': 'node-2-pub',
            'status': 'online',
            'relayAllowed': true,
            'virtualIps': ['100.64.0.11'],
            'endpoints': [
              {
                'type': 'relay',
                'address': kDevRelayUdpAddress,
                'updatedAt': 1713340200,
              },
            ],
            'allowedRoutes': ['100.64.0.11/32'],
          },
        ],
        'routes': [
          {
            'cidr': '100.64.0.0/24',
            'viaNodeId': 'node-1',
            'metric': 'local',
          },
        ],
        'relayRegions': [
          {
            'regionId': 'cn-sh',
            'regionName': 'Shanghai',
            'countryCode': 'CN',
            'countryName': 'China',
            'cityCode': 'sh',
            'cityName': 'Shanghai',
            'clusterId': 'cn-sh-a',
            'clusterName': 'Shanghai A',
            'endpoints': [
              {
                'endpointId': 'relay-cn-sh-udp',
                'transport': 'udp',
                'address': kDevRelayUdpAddress,
              },
            ],
          },
        ],
        'dns': {
          'servers': ['100.64.0.1'],
          'searchDomains': ['slan.local'],
        },
        'mtu': 1280,
      },
    });

    expect(bootstrap.device.deviceId, 'dev-1');
    expect(bootstrap.device.machineId, 'machine-1');
    expect(bootstrap.device.virtualIp, '100.64.0.10');
    expect(bootstrap.networks, hasLength(1));
    expect(bootstrap.networks.first.cidr, '100.64.0.0/24');
    expect(bootstrap.networks.first.members.first.virtualIp, '100.64.0.10');
    expect(bootstrap.networks.first.members.first.memberId, 'member-1');
    expect(bootstrap.networks.first.members.first.status, 'active');
    expect(bootstrap.networks.first.members.last.status, 'pending');

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

  test('BootstrapResponseDto parses complete network map payload', () {
    final response = BootstrapResponseDto.fromJson({
      'device': {
        'device': {
          'deviceId': 'dev-1',
          'name': 'macbook',
          'platform': 'macos',
          'status': 'online',
        },
        'attachments': const [],
      },
      'networks': const [],
      'controlPlane': {
        'wsUrl': kDevControlWsUrl,
        'heartbeatSeconds': 15,
      },
      'stunServers': [kDevStunServer],
      'relay': {
        'defaultClusterId': 'cn-sh-a',
        'countries': const [],
      },
      'derpMap': {
        'probeIntervalSeconds': 30,
        'clusters': [
          {
            'clusterId': 'cn-sh-a',
            'clusterName': 'Shanghai A',
            'regionId': 'cn-sh',
            'regionName': 'Shanghai',
            'countryCode': 'CN',
            'countryName': 'China',
            'cityCode': 'sh',
            'cityName': 'Shanghai',
            'recommendedFanout': 1,
            'nodes': [
              {
                'nodeId': 'relay-cn-sh-udp',
                'host': '127.0.0.1',
                'port': 19000,
                'transport': 'udp',
                'priority': 10,
                'tags': ['default'],
              },
            ],
          },
        ],
      },
      'networkMap': {
        'selfUserId': 'user-1',
        'selfDeviceId': 'dev-1',
        'selfNodeId': 'node-1',
        'networkId': 'net-1',
        'revision': 7,
        'heartbeatSeconds': 15,
        'stunServers': [kDevStunServer],
        'peers': [
          {
            'nodeId': 'node-2',
            'deviceId': 'dev-2',
            'publicKey': 'node-2-pub',
            'status': 'online',
            'relayAllowed': true,
            'virtualIps': ['100.64.0.11'],
            'endpoints': [
              {
                'type': 'relay',
                'address': kDevRelayUdpAddress,
                'updatedAt': 1713340200,
              },
            ],
            'allowedRoutes': ['100.64.0.11/32'],
          },
        ],
        'routes': [
          {
            'cidr': '100.64.0.0/24',
            'viaNodeId': 'node-1',
            'metric': 'local',
          },
        ],
        'relayRegions': [
          {
            'regionId': 'cn-sh',
            'regionName': 'Shanghai',
            'countryCode': 'CN',
            'countryName': 'China',
            'cityCode': 'sh',
            'cityName': 'Shanghai',
            'clusterId': 'cn-sh-a',
            'clusterName': 'Shanghai A',
            'endpoints': [
              {
                'endpointId': 'relay-cn-sh-udp',
                'transport': 'udp',
                'address': kDevRelayUdpAddress,
              },
            ],
          },
        ],
        'dns': {
          'servers': ['100.64.0.1'],
          'searchDomains': ['slan.local'],
        },
        'mtu': 1280,
      },
    });

    final map = response.networkMap!;
    expect(response.derpMap.clusters.single.nodes.single.nodeId,
        'relay-cn-sh-udp');
    expect(map.selfNodeId, 'node-1');
    expect(map.revision, 7);
    expect(map.peers.single.endpoints.single.type, 'relay');
    expect(map.routes.single.metric, 'local');
    expect(map.relayRegions.single.clusterId, 'cn-sh-a');
    expect(
        map.relayRegions.single.endpoints.single.endpointId, 'relay-cn-sh-udp');
    expect(map.dns.searchDomains, ['slan.local']);
    expect(map.mtu, 1280);
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

  test('parseNetworkJoinByOwnerEmailResponse accepts network wrapper', () {
    final join = parseNetworkJoinByOwnerEmailResponse({
      'network': {
        'networkId': 'net-1',
        'name': 'home',
        'defaultSubnetCidr': '100.64.0.0/24',
      },
      'member': {
        'memberId': 'member-1',
        'networkId': 'net-1',
        'deviceId': 'dev-1',
        'role': 'member',
        'status': 'active',
      },
      'attachment': {
        'attachmentId': 'att-1',
        'networkId': 'net-1',
        'subnetId': 'subnet-1',
        'deviceId': 'dev-1',
        'virtualIp': '100.64.0.2',
      },
    });

    expect(join.networkId, 'net-1');
    expect(join.memberId, 'member-1');
    expect(join.attachmentId, 'att-1');
    expect(join.virtualIp, '100.64.0.2');
  });

  test('parseNetworkJoinResponse maps member attachment response', () {
    final join = parseNetworkJoinResponse({
      'member': {
        'memberId': 'member-1',
        'networkId': 'net-1',
        'deviceId': 'dev-1',
        'role': 'member',
        'status': 'active',
      },
      'attachment': {
        'attachmentId': 'att-1',
        'networkId': 'net-1',
        'subnetId': 'subnet-1',
        'deviceId': 'dev-1',
        'virtualIp': '100.64.0.2',
      },
    });

    expect(join.networkId, 'net-1');
    expect(join.deviceId, 'dev-1');
    expect(join.memberId, 'member-1');
    expect(join.attachmentId, 'att-1');
    expect(join.virtualIp, '100.64.0.2');
  });
}
