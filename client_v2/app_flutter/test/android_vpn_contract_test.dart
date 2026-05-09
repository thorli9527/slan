import 'package:client_core_plugin/client_core_plugin.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test('android vpn config preserves relay data plane', () {
    final config = AndroidVpnSessionConfig.fromJson({
      'sessionName': 'SLAN',
      'virtualIp': '10.0.0.2',
      'prefixLen': 32,
      'dnsServers': ['10.0.0.1'],
      'routes': [
        {'destination': '10.0.0.9/32'},
      ],
      'mtu': 1280,
      'relayDataPlane': {
        'enabled': true,
        'transport': 'udp',
        'relayAddress': '127.0.0.1:3478',
        'localNodeId': 'node-a',
        'networkId': 'net-1',
        'pathPolicy': {
          'preferred': [
            'lan_udp',
            'ipv6_udp',
            'direct_udp',
            'relay_udp',
            'derp_tcp_tls_443'
          ],
          'fallbackEnabled': true,
          'probeIntervalMs': 15000,
          'failoverAfterMs': 30000,
        },
        'peerPaths': [
          {
            'peerNodeId': 'node-b',
            'peerVirtualIps': ['10.0.0.9'],
            'candidates': [
              {
                'kind': 'direct_udp',
                'state': 'probing',
                'address': '192.168.1.8:41000',
                'transport': 'udp',
              },
              {
                'kind': 'relay_udp',
                'state': 'standby',
                'endpointId': 'relay-cn-1',
                'address': '127.0.0.1:3478',
                'sessionId': 'session-1',
                'transport': 'udp',
                'pathScore': 700,
              },
            ],
          },
        ],
        'relayMtu': 1280,
        'maxFramePayload': 1200,
        'sessions': [
          {
            'sessionId': 'session-1',
            'peerNodeId': 'node-b',
            'peerVirtualIps': ['10.0.0.9'],
            'ticket': {
              'ticketId': 'ticket-1',
              'networkId': 'net-1',
              'sessionId': 'session-1',
              'srcNodeId': 'node-a',
              'dstNodeId': 'node-b',
              'relayUrl': 'udp://127.0.0.1:3478',
              'expiresAt': '2026-05-03T10:10:00Z',
              'sessionKey': 'key',
              'signature': 'sig',
            },
          },
        ],
      },
    });

    final relay = config.relayDataPlane;
    expect(relay, isNotNull);
    expect(relay!.enabled, isTrue);
    expect(relay.pathPolicy!.preferred, [
      'lan_udp',
      'ipv6_udp',
      'direct_udp',
      'relay_udp',
      'derp_tcp_tls_443',
    ]);
    expect(relay.peerPaths.single.candidates.first.kind, 'direct_udp');
    expect(relay.sessions.single.peerVirtualIps, ['10.0.0.9']);
    expect(relay.sessions.single.ticket.signature, 'sig');

    final json = config.toJson();
    final relayJson = json['relayDataPlane'] as Map<String, Object?>;
    final peerPaths = relayJson['peerPaths'] as List<Object?>;
    final peerPath = peerPaths.single as Map<String, Object?>;
    final candidates = peerPath['candidates'] as List<Object?>;
    final sessions = relayJson['sessions'] as List<Object?>;
    final session = sessions.single as Map<String, Object?>;
    final ticket = session['ticket'] as Map<String, Object?>;
    expect(relayJson['relayAddress'], '127.0.0.1:3478');
    expect((relayJson['pathPolicy'] as Map<String, Object?>)['fallbackEnabled'],
        isTrue);
    expect(peerPath['peerNodeId'], 'node-b');
    expect((candidates.first as Map<String, Object?>)['kind'], 'direct_udp');
    expect(session['peerNodeId'], 'node-b');
    expect(ticket['ticketId'], 'ticket-1');
  });

  test('android path policy defaults include all canonical path kinds', () {
    const policy = PathPolicy();

    expect(policy.preferred, [
      PathKind.lanUdp,
      PathKind.ipv6Udp,
      PathKind.directUdp,
      PathKind.relayUdp,
      PathKind.derpTcpTls443,
    ]);
  });
}
