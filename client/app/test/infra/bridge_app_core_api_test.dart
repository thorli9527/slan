import 'dart:async';
import 'dart:convert';
import 'dart:io';

import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:slan_app/infra/app_core/api/dev_defaults.dart';
import 'package:slan_app/infra/app_core/bridge/app_core_bridge.dart';
import 'package:slan_app/infra/app_core/bridge/bridge_app_core_api.dart';
import 'package:slan_app/infra/app_core/models/models.dart';

void main() {
  test(
      'BridgeAppCoreApi forwards bootstrap and relay topology through facade bridge',
      () async {
    final bridge = _FakeAppCoreBridge({
      'register': {
        'userId': 'user@example.com',
        'accessToken': 'token-1',
        'refreshToken': 'refresh-1',
        'expiresIn': 3600,
      },
      'bootstrap': {
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
      },
      'connect': {
        'status': 'connected',
        'path': 'derp',
      },
    });
    final api = BridgeAppCoreApi(bridge: bridge);

    final session = await api.register(
      email: 'user@example.com',
      password: 'password123',
    );
    final bootstrap = await api.bootstrap(
      nodeId: 'node-1',
      networkId: 'net-1',
    );
    final state = await api.connect(
      networkId: 'net-1',
      peerNodeId: 'node-2',
    );

    expect(session.accessToken, 'token-1');
    expect(bridge.calls.first.method, 'register');
    expect(bridge.calls.first.args, {
      'email': 'user@example.com',
      'password': 'password123',
    });
    expect(bridge.calls[1].method, 'bootstrap');
    expect(
        bootstrap.relay.countries.single.cities.single.clusters.single.nodes
            .single.nodeId,
        'relay-cn-local-udp');
    expect(state.status, 'connected');
    expect(state.path?.name, 'relay');
  });

  test('BridgeAppCoreApi maps listNetworks and relay ticket payloads',
      () async {
    final bridge = _FakeAppCoreBridge({
      'listNetworks': {
        'items': [
          {
            'networkId': 'net-1',
            'name': 'home',
            'defaultSubnetCidr': '100.64.0.0/24',
          },
        ],
      },
      'issueRelayTicket': {
        'ticketId': 'ticket-1',
        'networkId': 'net-1',
        'sessionId': 'session-1',
        'srcNodeId': 'node-1',
        'dstNodeId': 'node-2',
        'derpClusterId': 'cn-local-a',
        'allowedDerpNodeIds': ['relay-cn-local-udp'],
        'relayUrl': kDevRelayUdpUrl,
        'expiresAt': '2026-04-17T10:00:00Z',
        'signature': 'signed',
      },
    });
    final api = BridgeAppCoreApi(bridge: bridge);

    final networks = await api.listNetworks();
    final ticket = await api.issueRelayTicket(
      networkId: 'net-1',
      srcNodeId: 'node-1',
      dstNodeId: 'node-2',
      reason: 'p2p_failed',
    );

    expect(networks.single.cidr, '100.64.0.0/24');
    expect(ticket.derpClusterId, 'cn-local-a');
    expect(ticket.allowedDerpNodeIds, ['relay-cn-local-udp']);
    expect(bridge.calls[1].args['reason'], 'p2p_failed');
  });

  test('BridgeAppCoreApi forwards probe timeout and parses probe payload',
      () async {
    final bridge = _FakeAppCoreBridge({
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
    });
    final api = BridgeAppCoreApi(bridge: bridge);

    final probe = await api.probe(
      payload: 'hello',
      probeTimeoutMs: 7,
    );

    expect(probe.probeId, 'probe-1');
    expect(probe.replyObserved, isTrue);
    expect(probe.replyRttMs, 2);
    expect(bridge.calls.single.method, 'probe');
    expect(bridge.calls.single.args, {
      'payload': 'hello',
      'probeTimeoutMs': 7,
    });
  });

  test('BridgeAppCoreApi classifies probe timeout failures', () async {
    final bridge = _FakeAppCoreBridge({});
    bridge.probeErrorCode = 'probe_timeout';
    bridge.probeErrorMessage = 'timed out waiting for probe reply';
    final api = BridgeAppCoreApi(bridge: bridge);

    try {
      await api.probe(payload: 'hello', probeTimeoutMs: 7);
      fail('expected probe to throw');
    } on ProbeException catch (err) {
      expect(err.failure.kind, ProbeFailureKind.timeout);
      expect(err.failure.code, 'probe_timeout');
      expect(err.failure.message, 'timed out waiting for probe reply');
    }
  });

  test('BridgeAppCoreApi classifies probe unsupported failures', () async {
    final bridge = _FakeAppCoreBridge({});
    bridge.probeErrorCode = 'probe_unsupported_path';
    bridge.probeErrorMessage = 'active path does not support probe';
    final api = BridgeAppCoreApi(bridge: bridge);

    try {
      await api.probe(payload: 'hello', probeTimeoutMs: 7);
      fail('expected probe to throw');
    } on ProbeException catch (err) {
      expect(err.failure.kind, ProbeFailureKind.unsupported);
      expect(err.failure.code, 'probe_unsupported_path');
      expect(err.failure.message, 'active path does not support probe');
    }
  });

  test('BridgeAppCoreApi forwards send payload and parses bytes sent',
      () async {
    final bridge = _FakeAppCoreBridge({
      'send': {
        'status': 'sent',
        'bytesSent': 5,
        'path': 'relay',
      },
    });
    final api = BridgeAppCoreApi(bridge: bridge);

    final bytesSent = await api.send(payload: 'hello');

    expect(bytesSent, 5);
    expect(bridge.calls.single.method, 'send');
    expect(bridge.calls.single.args, {
      'payload': 'hello',
    });
  });

  test('BridgeAppCoreApi classifies send transport failures', () async {
    final bridge = _FakeAppCoreBridge({});
    bridge.sendErrorCode = 'send_transport_error';
    bridge.sendErrorMessage = 'relay client has no active connection';
    final api = BridgeAppCoreApi(bridge: bridge);

    try {
      await api.send(payload: 'hello');
      fail('expected send to throw');
    } on SendException catch (err) {
      expect(err.failure.kind, SendFailureKind.transport);
      expect(err.failure.code, 'send_transport_error');
      expect(err.failure.message, 'relay client has no active connection');
    }
  });

  test('BridgeAppCoreApi classifies send unsupported failures', () async {
    final bridge = _FakeAppCoreBridge({});
    bridge.sendErrorCode = 'send_unsupported_path';
    bridge.sendErrorMessage = 'active path does not support send';
    final api = BridgeAppCoreApi(bridge: bridge);

    try {
      await api.send(payload: 'hello');
      fail('expected send to throw');
    } on SendException catch (err) {
      expect(err.failure.kind, SendFailureKind.unsupported);
      expect(err.failure.code, 'send_unsupported_path');
      expect(err.failure.message, 'active path does not support send');
    }
  });

  test('BridgeAppCoreApi leaves generic send failures as unknown', () async {
    final bridge = _FakeAppCoreBridge({});
    bridge.sendErrorCode = 'send_failed';
    bridge.sendErrorMessage = 'send failed unexpectedly';
    final api = BridgeAppCoreApi(bridge: bridge);

    try {
      await api.send(payload: 'hello');
      fail('expected send to throw');
    } on SendException catch (err) {
      expect(err.failure.kind, SendFailureKind.unknown);
      expect(err.failure.code, 'send_failed');
      expect(err.failure.message, 'send failed unexpectedly');
    }
  });

  test(
      'BridgeAppCoreApi classifies probe unsupported failures from real helper process',
      () async {
    final bridge = await _HelperProcessBridge.start(
      environment: {
        'SLAN_APP_CORE_HELPER_TEST_PROBE_ERROR':
            'probe_unsupported_path: active path does not support probe',
      },
    );
    addTearDown(bridge.close);
    final api = BridgeAppCoreApi(bridge: bridge);

    try {
      await api.probe(payload: 'hello', probeTimeoutMs: 7);
      fail('expected probe to throw');
    } on ProbeException catch (err) {
      expect(err.failure.kind, ProbeFailureKind.unsupported);
      expect(err.failure.code, 'probe_unsupported_path');
      expect(err.failure.message, 'active path does not support probe');
    }
  });

  test(
      'BridgeAppCoreApi classifies send timeout failures from real helper process',
      () async {
    final bridge = await _HelperProcessBridge.start(
      environment: {
        'SLAN_APP_CORE_HELPER_TEST_SEND_ERROR':
            'send_timeout: timed out waiting for send reply',
      },
    );
    addTearDown(bridge.close);
    final api = BridgeAppCoreApi(bridge: bridge);

    try {
      await api.send(payload: 'hello');
      fail('expected send to throw');
    } on SendException catch (err) {
      expect(err.failure.kind, SendFailureKind.timeout);
      expect(err.failure.code, 'send_timeout');
      expect(err.failure.message, 'timed out waiting for send reply');
    }
  });
}

class _FakeAppCoreBridge implements AppCoreBridge {
  _FakeAppCoreBridge(this._responses);

  final Map<String, Object?> _responses;
  final List<_BridgeCall> calls = [];
  String? probeErrorCode;
  String? probeErrorMessage;
  String? sendErrorCode;
  String? sendErrorMessage;

  @override
  Future<Object?> invoke(String method,
      [Map<String, Object?> args = const {}]) async {
    calls.add(
        _BridgeCall(method: method, args: Map<String, Object?>.from(args)));
    if (method == 'probe' && probeErrorCode != null) {
      throw PlatformException(
        code: probeErrorCode!,
        message: probeErrorMessage,
      );
    }
    if (method == 'send' && sendErrorCode != null) {
      throw PlatformException(
        code: sendErrorCode!,
        message: sendErrorMessage,
      );
    }
    if (!_responses.containsKey(method)) {
      throw StateError('Missing fake bridge response for $method');
    }
    return _responses[method];
  }
}

class _BridgeCall {
  const _BridgeCall({
    required this.method,
    required this.args,
  });

  final String method;
  final Map<String, Object?> args;
}

class _HelperProcessBridge implements AppCoreBridge {
  _HelperProcessBridge._({
    required Process process,
    required StreamIterator<String> stdoutLines,
    required this.fallbackMessage,
  })  : _process = process,
        _stdoutLines = stdoutLines;

  final Process _process;
  final StreamIterator<String> _stdoutLines;
  final String fallbackMessage;

  static Future<_HelperProcessBridge> start({
    Map<String, String> environment = const {},
  }) async {
    final process = await _startProcess(environment);
    process.stderr.transform(utf8.decoder).listen((_) {});
    return _HelperProcessBridge._(
      process: process,
      stdoutLines: StreamIterator(process.stdout
          .transform(utf8.decoder)
          .transform(const LineSplitter())),
      fallbackMessage: 'unknown app-core helper error',
    );
  }

  @override
  Future<Object?> invoke(
    String method, [
    Map<String, Object?> args = const {},
  ]) async {
    _process.stdin.writeln(jsonEncode({
      'method': method,
      'args': args,
    }));
    if (!await _stdoutLines.moveNext()) {
      throw PlatformException(
        code: 'app_core_process_closed',
        message: 'app-core helper closed stdout unexpectedly',
      );
    }
    final decoded = jsonDecode(_stdoutLines.current);
    if (decoded is! Map) {
      throw const FormatException(
          'Expected object payload from helper process');
    }
    final payload = decoded.map(
      (key, value) => MapEntry(key.toString(), value),
    );
    final ok = payload['ok'];
    if (ok is! bool) {
      throw const FormatException('Expected ok flag from helper process');
    }
    if (ok) {
      return payload['result'];
    }
    throw PlatformException(
      code: payload['errorCode'] as String? ?? 'app_core_helper_error',
      message: payload['errorMessage'] as String? ??
          payload['error'] as String? ??
          fallbackMessage,
    );
  }

  Future<void> close() async {
    await _stdoutLines.cancel();
    await _process.stdin.close();
    if (_process.kill()) {
      try {
        await _process.exitCode.timeout(const Duration(seconds: 2));
      } on TimeoutException {
        _process.kill(ProcessSignal.sigkill);
      }
    }
  }

  static Future<Process> _startProcess(Map<String, String> environment) async {
    final helperBinary = File(
      '${Directory.current.path}/../app_core/target/debug/app-core-helper',
    );
    final mergedEnvironment = {
      ...Platform.environment,
      'SLAN_CONTROL_BASE_URL': 'http://127.0.0.1:18081',
      ...environment,
    };
    if (helperBinary.existsSync()) {
      return Process.start(
        helperBinary.path,
        const [],
        environment: mergedEnvironment,
      );
    }
    return Process.start(
      'cargo',
      const ['run', '-q', '-p', 'app-core-helper'],
      workingDirectory: '${Directory.current.path}/../app_core',
      environment: mergedEnvironment,
    );
  }
}
