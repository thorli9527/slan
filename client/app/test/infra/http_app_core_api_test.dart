import 'dart:convert';
import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:slan_app/infra/app_core/api/dev_defaults.dart';
import 'package:slan_app/infra/app_core/api/http_app_core_api.dart';

void main() {
  test('HttpAppCoreApi maps real control-plane responses into app models',
      () async {
    final server = await HttpServer.bind(InternetAddress.loopbackIPv4, 0);
    addTearDown(server.close);

    server.listen((request) async {
      final body = await utf8.decoder.bind(request).join();
      final json = body.isEmpty
          ? <String, dynamic>{}
          : jsonDecode(body) as Map<String, dynamic>;
      final route = '${request.method} ${request.uri.path}';

      if (route == 'POST /auth/register') {
        request.response
          ..statusCode = HttpStatus.created
          ..headers.contentType = ContentType.json
          ..write(jsonEncode({
            'userId': json['email'],
            'accessToken': 'token-1',
            'refreshToken': 'refresh-1',
            'expiresIn': 3600,
          }));
      } else if (route == 'POST /devices/register') {
        expect(request.headers.value(HttpHeaders.authorizationHeader),
            'Bearer token-1');
        request.response
          ..statusCode = HttpStatus.created
          ..headers.contentType = ContentType.json
          ..write(jsonEncode({
            'deviceId': json['machineId'],
            'name': json['name'],
            'platform': json['platform'],
            'status': 'online',
            'ownerEmail': 'user@example.com',
            'currentVirtualIp': '100.64.0.10',
            'linkStatus': 'direct',
            'connectivityProtocol': 'p2p',
            'joinedAt': 1713340000,
            'membershipStatus': 'active',
            'networkRole': 'owner',
            'createdAt': 1713330000,
            'publicKey': json['publicKey'],
            'networkIds': ['net-1'],
          }));
      } else if (route == 'POST /nodes/register') {
        request.response
          ..statusCode = HttpStatus.created
          ..headers.contentType = ContentType.json
          ..write(jsonEncode({
            'nodeId': json['nodeId'],
            'deviceId': json['deviceId'],
            'nodePublicKey': json['nodePublicKey'],
            'networkIds': ['net-1'],
            'capabilities': json['capabilities'],
          }));
      } else if (route == 'GET /networks') {
        request.response
          ..statusCode = HttpStatus.ok
          ..headers.contentType = ContentType.json
          ..write(jsonEncode({
            'items': [
              {
                'networkId': 'net-1',
                'name': 'home',
                'description': 'primary network',
                'defaultSubnetId': 'subnet-1',
                'defaultSubnetCidr': '100.64.0.0/24',
                'joinKeyConfigured': true,
              },
            ],
          }));
      } else if (route == 'GET /devices') {
        request.response
          ..statusCode = HttpStatus.ok
          ..headers.contentType = ContentType.json
          ..write(jsonEncode({
            'items': [
              {
                'deviceId': 'machine-1',
                'name': 'thor-mac',
                'platform': 'macos',
                'status': 'online',
                'currentVirtualIp': '100.64.0.10',
                'membershipStatus': 'active',
                'networkRole': 'owner',
                'networkIds': ['net-1'],
              },
              {
                'deviceId': 'guest-1',
                'name': 'guest-laptop',
                'platform': 'windows',
                'status': 'offline',
                'membershipStatus': 'pending',
                'networkRole': 'member',
                'networkIds': ['net-1'],
              },
            ],
          }));
      } else if (route == 'POST /bootstrap') {
        request.response
          ..statusCode = HttpStatus.ok
          ..headers.contentType = ContentType.json
          ..write(jsonEncode({
            'device': {
              'device': {
                'deviceId': 'machine-1',
                'name': 'thor-mac',
                'platform': 'macos',
                'status': 'online',
                'membershipStatus': 'active',
                'networkRole': 'owner',
                'networkIds': ['net-1'],
                'publicKey': 'device-pub',
              },
              'attachments': [
                {
                  'networkId': 'net-1',
                  'subnetId': 'subnet-1',
                  'deviceId': 'machine-1',
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
                    'deviceId': 'machine-1',
                    'role': 'owner',
                    'createdAt': 1713340000,
                    'status': 'active',
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
              'defaultClusterId': 'cn-local-a',
              'countries': [
                {
                  'countryCode': 'CN',
                  'countryName': 'China',
                  'cities': [
                    {
                      'cityCode': 'local',
                      'cityName': 'Local',
                      'clusters': [
                        {
                          'clusterId': 'cn-local-a',
                          'clusterName': 'CN Local A',
                          'nodes': [
                            {
                              'nodeId': 'relay-cn-local-udp',
                              'transport': 'udp',
                              'address': kDevRelayUdpAddress,
                              'priority': 10,
                            },
                            {
                              'nodeId': 'relay-cn-local-tcp',
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
                  'clusterId': 'cn-local-a',
                  'clusterName': 'CN Local A',
                  'regionId': 'cn-local',
                  'regionName': 'CN Local',
                  'countryCode': 'CN',
                  'countryName': 'China',
                  'cityCode': 'local',
                  'cityName': 'Local',
                  'recommendedFanout': 1,
                  'nodes': [
                    {
                      'nodeId': 'relay-cn-local-udp',
                      'host': '127.0.0.1',
                      'port': 19000,
                      'transport': 'udp',
                      'priority': 10,
                    },
                    {
                      'nodeId': 'relay-cn-local-tcp',
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
              'selfDeviceId': 'machine-1',
              'selfNodeId': 'node-1',
              'networkId': 'net-1',
              'revision': 7,
              'heartbeatSeconds': 15,
              'stunServers': [kDevStunServer],
              'peers': [
                {
                  'nodeId': 'node-2',
                  'deviceId': 'machine-2',
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
                  'regionId': 'cn-local',
                  'regionName': 'CN Local',
                  'countryCode': 'CN',
                  'countryName': 'China',
                  'cityCode': 'local',
                  'cityName': 'Local',
                  'clusterId': 'cn-local-a',
                  'clusterName': 'CN Local A',
                  'endpoints': [
                    {
                      'endpointId': 'relay-cn-local-udp',
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
          }));
      } else if (route == 'POST /relay/tickets') {
        expect(json['reason'], 'p2p_failed');
        request.response
          ..statusCode = HttpStatus.created
          ..headers.contentType = ContentType.json
          ..write(jsonEncode({
            'ticketId': 'ticket-1',
            'networkId': json['networkId'],
            'sessionId': 'session-1',
            'srcNodeId': json['srcNodeId'],
            'dstNodeId': json['dstNodeId'],
            'derpClusterId': 'cn-local-a',
            'countryCode': 'CN',
            'cityCode': 'local',
            'allowedDerpNodeIds': [
              'relay-cn-local-udp',
              'relay-cn-local-tcp',
            ],
            'relayUrl': kDevRelayUdpUrl,
            'expiresAt': '2026-04-17T10:00:00Z',
            'sessionKey': 'session-key',
            'signature': 'signed-payload',
          }));
      } else {
        request.response.statusCode = HttpStatus.notFound;
      }

      await request.response.close();
    });

    final api = HttpAppCoreApi(
      baseUrl: 'http://${server.address.host}:${server.port}',
    );

    final session = await api.register(
      email: 'user@example.com',
      password: 'password123',
    );
    final device = await api.registerDevice(
      name: 'thor-mac',
      platform: 'macos',
      machineId: 'machine-1',
      publicKey: 'device-pub',
    );
    final node = await api.registerNode(
      deviceId: device.deviceId,
      nodeId: 'node-1',
      nodePublicKey: 'node-pub',
      capabilities: const ['relay'],
    );
    final networks = await api.listNetworks();
    final devices = await api.listDevices();
    final bootstrap = await api.bootstrap(
      nodeId: node.nodeId,
      networkId: 'net-1',
    );
    final ticket = await api.issueRelayTicket(
      networkId: 'net-1',
      srcNodeId: node.nodeId,
      dstNodeId: 'node-2',
      reason: 'p2p_failed',
    );

    expect(session.accessToken, 'token-1');
    expect(device.deviceId, 'machine-1');
    expect(device.virtualIp, '100.64.0.10');
    expect(device.membershipStatus, 'active');
    expect(device.networkRole, 'owner');
    expect(device.networkIds, ['net-1']);
    expect(node.capabilities, ['relay']);
    expect(networks.single.cidr, '100.64.0.0/24');
    expect(networks.single.description, 'primary network');
    expect(networks.single.defaultSubnetId, 'subnet-1');
    expect(networks.single.joinKeyConfigured, isTrue);
    expect(devices, hasLength(2));
    expect(devices.last.membershipStatus, 'pending');
    expect(devices.last.networkRole, 'member');
    expect(bootstrap.device.virtualIp, '100.64.0.10');
    expect(bootstrap.networks.single.members.single.memberId, 'member-1');
    expect(bootstrap.networks.single.members.single.status, 'active');
    expect(bootstrap.relay.countries.single.cities.single.clusters.single.nodes,
        hasLength(2));
    expect(ticket.allowedDerpNodeIds, [
      'relay-cn-local-udp',
      'relay-cn-local-tcp',
    ]);
  });

  test('HttpAppCoreApi sends network lifecycle bodies for join and activation',
      () async {
    final server = await HttpServer.bind(InternetAddress.loopbackIPv4, 0);
    addTearDown(server.close);

    final seenRoutes = <String, Map<String, dynamic>>{};

    server.listen((request) async {
      final body = await utf8.decoder.bind(request).join();
      final json = body.isEmpty
          ? <String, dynamic>{}
          : jsonDecode(body) as Map<String, dynamic>;
      final route = '${request.method} ${request.uri.path}';

      if (route == 'POST /auth/login') {
        request.response
          ..statusCode = HttpStatus.ok
          ..headers.contentType = ContentType.json
          ..write(jsonEncode({
            'userId': 'user-1',
            'accessToken': 'token-2',
            'expiresIn': 3600,
          }));
      } else if (route == 'POST /networks/join-by-owner-email' ||
          route == 'POST /networks/join-by-key' ||
          route == 'POST /networks/net-1/activate' ||
          route == 'POST /networks/net-1/switch' ||
          route == 'POST /networks/net-1/deactivate' ||
          route == 'PUT /networks/net-1/attachments/att-1/remark') {
        expect(request.headers.value(HttpHeaders.authorizationHeader),
            'Bearer token-2');
        seenRoutes[route] = json;
        request.response
          ..statusCode = HttpStatus.ok
          ..headers.contentType = ContentType.json
          ..write(jsonEncode({
            if (route == 'POST /networks/join-by-owner-email')
              'network': {
                'networkId': 'net-1',
                'name': 'home',
                'defaultSubnetCidr': '100.64.0.0/24',
              },
            if (route == 'POST /networks/join-by-owner-email' ||
                route == 'POST /networks/join-by-key' ||
                route.endsWith('/activate') ||
                route.endsWith('/switch'))
              'member': {
                'memberId': 'member-1',
                'networkId': 'net-1',
                'deviceId': 'dev-1',
                'role': 'member',
              },
            if (route == 'POST /networks/join-by-owner-email' ||
                route == 'POST /networks/join-by-key' ||
                route.endsWith('/activate') ||
                route.endsWith('/switch'))
              'attachment': {
                'attachmentId': 'att-1',
                'networkId': 'net-1',
                'subnetId': 'subnet-1',
                'deviceId': 'dev-1',
                'virtualIp': '100.64.0.2',
              },
            if (route.endsWith('/deactivate')) 'status': 'deactivated',
            if (route.endsWith('/remark')) 'attachmentId': 'att-1',
            if (route.endsWith('/remark')) 'networkId': 'net-1',
            if (route.endsWith('/remark')) 'subnetId': 'subnet-1',
            if (route.endsWith('/remark')) 'deviceId': 'dev-1',
            if (route.endsWith('/remark')) 'deviceName': 'dev-1',
            if (route.endsWith('/remark')) 'userId': 'user-1',
            if (route.endsWith('/remark')) 'userEmail': 'user@example.com',
            if (route.endsWith('/remark')) 'role': 'member',
            if (route.endsWith('/remark')) 'remark': json['remark'],
          }));
      } else {
        request.response.statusCode = HttpStatus.notFound;
      }

      await request.response.close();
    });

    final api = HttpAppCoreApi(
      baseUrl: 'http://${server.address.host}:${server.port}',
    );

    await api.login(email: 'user@example.com', password: 'password123');
    await api.joinNetworkByOwnerEmail(
      ownerEmail: 'owner@example.com',
      deviceId: 'dev-1',
    );
    await api.joinNetworkByKey(joinKey: 'join-key-1', deviceId: 'dev-1');
    final remark = await api.updateAttachmentRemark(
      networkId: 'net-1',
      attachmentId: 'att-1',
      remark: 'Thor laptop',
    );
    final activated =
        await api.activateNetwork(networkId: 'net-1', deviceId: 'dev-1');
    final switched =
        await api.switchNetwork(networkId: 'net-1', deviceId: 'dev-1');
    await api.deactivateNetwork(networkId: 'net-1', deviceId: 'dev-1');

    expect(seenRoutes['POST /networks/join-by-owner-email'], {
      'ownerEmail': 'owner@example.com',
      'deviceId': 'dev-1',
    });
    expect(seenRoutes['POST /networks/join-by-key'], {
      'joinKey': 'join-key-1',
      'deviceId': 'dev-1',
    });
    expect(seenRoutes['PUT /networks/net-1/attachments/att-1/remark'], {
      'remark': 'Thor laptop',
    });
    expect(remark.attachmentId, 'att-1');
    expect(remark.remark, 'Thor laptop');
    expect(seenRoutes['POST /networks/net-1/activate'], {
      'deviceId': 'dev-1',
    });
    expect(activated.attachmentId, 'att-1');
    expect(seenRoutes['POST /networks/net-1/switch'], {
      'deviceId': 'dev-1',
    });
    expect(switched.attachmentId, 'att-1');
    expect(seenRoutes['POST /networks/net-1/deactivate'], {
      'deviceId': 'dev-1',
    });
  });

  test('HttpAppCoreApi refreshes session with refresh token body', () async {
    final server = await HttpServer.bind(InternetAddress.loopbackIPv4, 0);
    addTearDown(server.close);

    Map<String, dynamic>? seenRefreshBody;

    server.listen((request) async {
      final body = await utf8.decoder.bind(request).join();
      final json = body.isEmpty
          ? <String, dynamic>{}
          : jsonDecode(body) as Map<String, dynamic>;
      final route = '${request.method} ${request.uri.path}';

      if (route == 'POST /auth/refresh') {
        seenRefreshBody = json;
        request.response
          ..statusCode = HttpStatus.ok
          ..headers.contentType = ContentType.json
          ..write(jsonEncode({
            'userId': 'user-1',
            'accessToken': 'token-refreshed',
            'refreshToken': 'refresh-2',
            'expiresIn': 3600,
          }));
      } else {
        request.response.statusCode = HttpStatus.notFound;
      }

      await request.response.close();
    });

    final api = HttpAppCoreApi(
      baseUrl: 'http://${server.address.host}:${server.port}',
    );

    final session = await api.refreshSession(
      refreshToken: 'refresh-1',
      deviceId: 'dev-1',
    );

    expect(session.accessToken, 'token-refreshed');
    expect(seenRefreshBody, {
      'refreshToken': 'refresh-1',
      'deviceId': 'dev-1',
    });
  });
}
