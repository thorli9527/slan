import 'package:flutter_test/flutter_test.dart';
import 'package:slan_app_core_plugin_platform_interface/slan_app_core_plugin_platform_interface.dart';

void main() {
  group('AppCoreMethodModels', () {
    test('decodes bootstrap payload', () {
      final payload = AppCoreBootstrapPayload.fromJson({
        'device': {
          'device': {
            'deviceId': 'dev-1',
            'name': 'thor-mac',
            'platform': 'macos',
            'status': 'online',
          },
          'attachments': [
            {
              'networkId': 'net-1',
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
                'networkId': 'net-1',
                'cidr': '100.64.0.0/24',
                'isDefault': true,
              },
            ],
            'members': [
              {
                'deviceId': 'dev-1',
                'role': 'owner',
              },
            ],
          },
        ],
        'controlPlane': {
          'wsUrl': 'ws://127.0.0.1:8080/control/ws',
          'heartbeatSeconds': 15,
        },
        'stunServers': ['stun:stun.l.google.com:19302'],
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
                          'address': '127.0.0.1:19000',
                          'priority': 10,
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

      expect(payload.device.device.deviceId, 'dev-1');
      expect(payload.device.attachments.single.virtualIp, '100.64.0.10');
      expect(payload.networks.single.subnets.single.isDefault, isTrue);
      expect(payload.controlPlane.wsUrl, 'ws://127.0.0.1:8080/control/ws');
      expect(payload.relay.defaultClusterId, 'cn-local-a');
      expect(payload.networkMap?.networkId, 'net-1');
    });

    test('decodes control status payload', () {
      final payload = AppCoreControlStatusPayload.fromJson({
        'status': 'connected',
        'wsUrl': 'ws://127.0.0.1:8080/control/ws',
        'heartbeatSeconds': 15,
        'sessionTokenPresent': true,
        'networkMapPresent': true,
        'networkId': 'net-1',
        'nodeId': 'node-1',
        'deviceId': 'dev-1',
        'peerCount': 1,
        'connectPlanCount': 1,
        'connectPlans': [
          {
            'peerNodeId': 'node-2',
            'preferDirect': false,
            'pathCount': 1,
            'preferredPath': {
              'pathType': 'relay',
              'endpoint': 'udp://127.0.0.1:19000',
              'priority': 10,
            },
            'derpClusterId': 'cn-local-a',
            'preferredDerpNodeIds': ['relay-cn-local-udp'],
            'relayTicketId': 'ticket-1',
          },
        ],
      });

      expect(payload.status, 'connected');
      expect(payload.networkMapPresent, isTrue);
      expect(payload.connectPlans.single.peerNodeId, 'node-2');
      expect(payload.connectPlans.single.preferredPath?.pathType, 'relay');
      expect(payload.connectPlans.single.relayTicketId, 'ticket-1');
    });

    test('decodes data plane probe payload', () {
      final payload = AppCoreDataPlaneProbePayload.fromJson({
        'probeId': 'probe-1',
        'sampledAtMs': 1,
        'activePath': {
          'relay': {'peer_node_id': 'peer-1'},
        },
        'bytesSent': 5,
        'replyObserved': true,
        'replyBytesReceived': 5,
        'replySampledAtMs': 3,
        'replyRttMs': 2,
        'tunnelPeerVirtualIp': '100.64.0.2',
        'observedRttMs': 2,
        'packetLossPpm': 0,
        'pathScore': 100,
        'derpClusterId': 'cn-local-a',
        'derpNodeId': 'relay-cn-local-udp',
      });

      expect(payload.probeId, 'probe-1');
      expect(payload.activePath.kind, 'relay');
      expect(payload.activePath.isRelay, isTrue);
      expect(payload.activePath.details['peer_node_id'], 'peer-1');
      expect(payload.replyObserved, isTrue);
      expect(payload.replyRttMs, 2);
      expect(payload.derpClusterId, 'cn-local-a');
      expect(payload.derpNodeId, 'relay-cn-local-udp');
    });
  });

  group('SlanAppCorePluginPlatform', () {
    test('wraps typed methods and arguments', () async {
      final platform = _FakePlatform({
        'register': {
          'userId': 'user-1',
          'accessToken': 'token-1',
          'refreshToken': 'refresh-1',
          'expiresIn': 3600,
        },
        'listDevices': {
          'items': [
            {
              'deviceId': 'dev-1',
              'name': 'thor-mac',
              'platform': 'macos',
              'status': 'online',
            },
          ],
        },
        'connect': {
          'status': 'connected',
          'path': 'derp',
        },
        'probe': {
          'probeId': 'probe-1',
          'sampledAtMs': 1,
          'activePath': {
            'relay': {'peer_node_id': 'peer-1'},
          },
          'bytesSent': 5,
          'replyObserved': true,
          'replyBytesReceived': 5,
          'replySampledAtMs': 3,
          'replyRttMs': 2,
          'tunnelPeerVirtualIp': '100.64.0.2',
          'observedRttMs': null,
          'packetLossPpm': null,
          'pathScore': null,
          'derpClusterId': null,
          'derpNodeId': null,
        },
        'send': {
          'status': 'sent',
          'bytesSent': 5,
          'path': 'relay',
        },
        'disconnect': null,
      });

      final session = await platform.register(
        email: 'user@example.com',
        password: 'password123',
      );
      final devices = await platform.listDevices();
      final connect = await platform.connect(
        networkId: 'net-1',
        peerNodeId: 'node-2',
      );
      final probe = await platform.probe(
        payload: 'hello',
        probeTimeoutMs: 7,
      );
      final bytesSent = await platform.send(payload: 'hello');
      await platform.disconnect();

      expect(session.accessToken, 'token-1');
      expect(devices.single.deviceId, 'dev-1');
      expect(connect.path, 'derp');
      expect(probe.probeId, 'probe-1');
      expect(bytesSent, 5);
      expect(platform.calls.map((call) => call.method), [
        'register',
        'listDevices',
        'connect',
        'probe',
        'send',
        'disconnect',
      ]);
      expect(platform.calls.first.args, {
        'email': 'user@example.com',
        'password': 'password123',
      });
      expect(platform.calls[2].args, {
        'networkId': 'net-1',
        'peerNodeId': 'node-2',
      });
      expect(platform.calls[3].args, {
        'payload': 'hello',
        'probeTimeoutMs': 7,
      });
      expect(platform.calls[4].args, {
        'payload': 'hello',
      });
    });
  });
}

class _FakePlatform extends SlanAppCorePluginPlatform {
  _FakePlatform(this._responses);

  final Map<String, Object?> _responses;
  final List<_Call> calls = [];

  @override
  Future<Object?> invoke(
    String method, [
    Map<String, Object?> args = const {},
  ]) async {
    calls.add(_Call(method: method, args: Map<String, Object?>.from(args)));
    if (!_responses.containsKey(method)) {
      throw StateError('Missing fake response for $method');
    }
    return _responses[method];
  }
}

class _Call {
  const _Call({
    required this.method,
    required this.args,
  });

  final String method;
  final Map<String, Object?> args;
}
