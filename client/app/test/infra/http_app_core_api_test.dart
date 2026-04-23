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
            'publicKey': json['publicKey'],
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
                'defaultSubnetCidr': '100.64.0.0/24',
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
            'networkMap': {
              'networkId': 'net-1',
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
    expect(node.capabilities, ['relay']);
    expect(networks.single.cidr, '100.64.0.0/24');
    expect(bootstrap.device.virtualIp, '100.64.0.10');
    expect(bootstrap.relay.countries.single.cities.single.clusters.single.nodes,
        hasLength(2));
    expect(ticket.allowedDerpNodeIds, [
      'relay-cn-local-udp',
      'relay-cn-local-tcp',
    ]);
  });

  test('HttpAppCoreApi sends network switch-shaped bodies for activate/deactivate',
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
      } else if (route == 'POST /networks/net-1/activate' ||
          route == 'POST /networks/net-1/deactivate') {
        expect(request.headers.value(HttpHeaders.authorizationHeader),
            'Bearer token-2');
        seenRoutes[route] = json;
        request.response
          ..statusCode = HttpStatus.ok
          ..headers.contentType = ContentType.json
          ..write(jsonEncode({
            if (route.endsWith('/activate'))
              'member': {
                'memberId': 'member-1',
                'networkId': 'net-1',
                'deviceId': 'dev-1',
                'role': 'member',
              },
            if (route.endsWith('/activate'))
              'attachment': {
                'attachmentId': 'att-1',
                'networkId': 'net-1',
                'subnetId': 'subnet-1',
                'deviceId': 'dev-1',
                'virtualIp': '100.64.0.2',
              },
            if (route.endsWith('/deactivate')) 'status': 'deactivated',
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
    await api.activateNetwork(networkId: 'net-1', deviceId: 'dev-1');
    await api.deactivateNetwork(networkId: 'net-1', deviceId: 'dev-1');

    expect(seenRoutes['POST /networks/net-1/activate'], {
      'deviceId': 'dev-1',
    });
    expect(seenRoutes['POST /networks/net-1/deactivate'], {
      'deviceId': 'dev-1',
    });
  });
}
