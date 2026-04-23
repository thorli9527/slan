import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:slan_app/features/devices/devices_page.dart';
import 'package:slan_app/infra/app_core/api/dev_defaults.dart';
import 'package:slan_app/testing/app_test_keys.dart';

class FakeHost {
  final List<HostCall> calls = [];
  int _connectCount = 0;
  final List<Map<String, Object?>> _networks = [
    <String, Object?>{
      'networkId': 'net-1',
      'name': 'home',
      'defaultSubnetCidr': '100.64.0.0/24',
    },
  ];
  bool failSend = false;
  String sendFailureCode = 'send_transport_error';
  String sendFailureMessage = 'relay client has no active connection';
  bool failProbe = false;
  String probeFailureCode = 'probe_timeout';
  String probeFailureMessage = 'timed out waiting for probe reply';
  Map<String, Object?>? _tunnelConfiguration;
  bool _tunnelRunning = false;

  void expectRelayFallbackFlow() {
    expect(
      calls.map((call) => call.method),
      containsAllInOrder(<String>[
        'registerDevice',
        'listNetworks',
        'registerNode',
        'bootstrap',
        'controlStatus',
        'bootstrap',
        'controlStatus',
        'connect',
        'issueRelayTicket',
        'connect',
        if (calls.any((call) => call.method == 'send')) 'send',
        if (calls.any((call) => call.method == 'probe')) 'probe',
      ]),
    );
    expect(_nthCall('bootstrap', 0).arguments, {
      'nodeId': 'node-1',
      'networkId': 'net-1',
    });
    expect(_nthCall('connect', 0).arguments, {
      'networkId': 'net-1',
      'peerNodeId': 'fail-peer-node-1',
    });
    expect(_singleCall('issueRelayTicket').arguments, {
      'networkId': 'net-1',
      'srcNodeId': 'node-1',
      'dstNodeId': 'fail-peer-node-1',
      'reason': 'timeout',
    });
    expect(_nthCall('connect', 1).arguments, {
      'networkId': 'net-1',
      'peerNodeId': 'relay-fail-peer-node-1',
    });
  }

  void expectSendPayload(String payload) {
    expect(_singleCall('send').arguments, {
      'payload': payload,
    });
  }

  void expectProbePayload(String payload, {required int probeTimeoutMs}) {
    expect(_singleCall('probe').arguments, {
      'payload': payload,
      'probeTimeoutMs': probeTimeoutMs,
    });
  }

  void expectTunnelApply({
    required String localVirtualIp,
    required String peerVirtualIp,
    required String endpoint,
    String? debugEngineMode,
  }) {
    expect(_singleCall('applyTunnelConfiguration').arguments, {
      'transport': 'relay',
      'localVirtualIp': localVirtualIp,
      'peerVirtualIp': peerVirtualIp,
      'debugEngineMode': debugEngineMode,
      'wireguardInterface': {
        'interfaceName': null,
        'keyPair': {
          'publicKey': 'debug-public-key',
          'privateKey': 'debug-private-key',
        },
        'listenPort': 51820,
        'mtu': 1280,
        'addresses': ['$localVirtualIp/32'],
        'dnsServers': ['1.1.1.1'],
      },
      'wireguardPeer': {
        'peerNodeId': null,
        'publicKey': 'peer-debug-public-key',
        'presharedKey': null,
        'endpoint': endpoint,
        'allowedIps': [
          {'cidr': '$peerVirtualIp/32'},
        ],
        'persistentKeepaliveSeconds': null,
      },
    });
  }

  void expectTunnelLifecycle() {
    expect(_singleCall('bringTunnelUp').arguments, isEmpty);
    expect(_singleCall('tunnelRuntimeView').arguments, {
      'peerVirtualIp': '100.64.0.2',
    });
  }

  void expectTunnelLifecycleWithTeardown() {
    expect(_singleCall('bringTunnelUp').arguments, isEmpty);
    expect(_singleCall('bringTunnelDown').arguments, isEmpty);
    expect(_singleCall('removeTunnelPeer').arguments, {
      'peerVirtualIp': '100.64.0.2',
    });
    final runtimeCalls =
        calls.where((call) => call.method == 'tunnelRuntimeView').toList();
    expect(runtimeCalls, hasLength(3));
    for (final call in runtimeCalls) {
      expect(call.arguments, {
        'peerVirtualIp': '100.64.0.2',
      });
    }
  }

  HostCall _singleCall(String method) {
    final matches = calls.where((call) => call.method == method).toList();
    expect(matches, hasLength(1), reason: 'expected exactly one $method call');
    return matches.single;
  }

  HostCall _nthCall(String method, int index) {
    final matches = calls.where((call) => call.method == method).toList();
    expect(matches.length, greaterThan(index),
        reason: 'expected $method call at index $index');
    return matches[index];
  }

  Future<Object?> handle(MethodCall call) async {
    final arguments = Map<String, Object?>.from(
      (call.arguments as Map<Object?, Object?>?)?.map(
            (key, value) => MapEntry(key.toString(), value),
          ) ??
          const <String, Object?>{},
    );
    calls.add(HostCall(method: call.method, arguments: arguments));

    switch (call.method) {
      case 'listNetworks':
        return {
          'items': _networks,
        };
      case 'createNetwork':
        final network = <String, Object?>{
          'networkId': 'net-1',
          'name': arguments['name'],
          'defaultSubnetCidr': arguments['cidr'],
        };
        _networks
          ..clear()
          ..add(network);
        return network;
      case 'registerDevice':
        return {
          'deviceId': 'dev-1',
          'name': arguments['name'],
          'platform': arguments['platform'],
          'status': 'online',
          'publicKey': arguments['publicKey'],
        };
      case 'registerNode':
        return {
          'nodeId': arguments['nodeId'],
          'deviceId': 'dev-1',
          'nodePublicKey': arguments['nodePublicKey'],
          'networkIds': ['net-1'],
          'capabilities': <Object?>[],
        };
      case 'bootstrap':
        return {
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
        };
      case 'controlStatus':
        return {
          'status': 'connected',
          'wsUrl': kDevControlWsUrl,
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
              'peerNodeId': 'fail-peer-node-1',
              'preferDirect': false,
              'pathCount': 1,
              'derpClusterId': 'cn-local-a',
              'preferredDerpNodeIds': ['relay-cn-local-udp'],
              'relayTicketId': 'ticket-net-1-fail-peer-node-1',
            },
          ],
        };
      case 'connect':
        _connectCount += 1;
        if (_connectCount == 1) {
          return {
            'status': 'failed',
            'reason': 'p2p handshake timeout',
          };
        }
        return {
          'status': 'connected',
          'path': 'relay',
        };
      case 'issueRelayTicket':
        return {
          'ticketId': 'ticket-net-1-fail-peer-node-1',
          'networkId': 'net-1',
          'sessionId': 'session-node-1-fail-peer-node-1',
          'srcNodeId': 'node-1',
          'dstNodeId': 'fail-peer-node-1',
          'derpClusterId': 'cn-local-a',
          'allowedDerpNodeIds': ['relay-cn-local-udp'],
          'relayUrl': kDevRelayUdpUrl,
          'expiresAt': '2026-04-17T10:00:00Z',
          'signature': 'signed',
        };
      case 'disconnect':
        return null;
      case 'send':
        if (failSend) {
          throw PlatformException(
            code: sendFailureCode,
            message: sendFailureMessage,
          );
        }
        return {
          'status': 'sent',
          'bytesSent': (arguments['payload'] as String).length,
          'path': 'relay',
        };
      case 'probe':
        if (failProbe) {
          throw PlatformException(
            code: probeFailureCode,
            message: probeFailureMessage,
          );
        }
        return {
          'probeId': 'probe-1',
          'sampledAtMs': 1,
          'activePath': {
            'relay': {'peer_node_id': 'fail-peer-node-1'},
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
        };
      case 'applyTunnelConfiguration':
        _tunnelConfiguration = arguments;
        return null;
      case 'bringTunnelUp':
        if (_tunnelConfiguration == null) {
          throw PlatformException(
            code: 'app_core_tunnel_backend_unavailable',
            message: 'missing tunnel configuration',
          );
        }
        _tunnelRunning = true;
        return null;
      case 'bringTunnelDown':
        _tunnelRunning = false;
        return null;
      case 'removeTunnelPeer':
        _tunnelConfiguration = null;
        _tunnelRunning = false;
        return null;
      case 'tunnelRuntimeView':
        if (_tunnelConfiguration == null) {
          return null;
        }
        final peer = Map<String, Object?>.from(
          _tunnelConfiguration!['wireguardPeer']! as Map<Object?, Object?>,
        );
        final interface = Map<String, Object?>.from(
          _tunnelConfiguration!['wireguardInterface']! as Map<Object?, Object?>,
        );
        return {
          'state': _tunnelRunning ? 'configured' : 'disconnected',
          'transport': _tunnelConfiguration!['transport'],
          'debugEngineMode': _tunnelConfiguration!['debugEngineMode'] ?? 'noop',
          'backendName': 'wireguardkit',
          'backendState': _tunnelConfiguration!['debugEngineMode'] == 'external'
              ? 'failed'
              : (_tunnelRunning ? 'started' : 'idle'),
          'backendLastError':
              _tunnelConfiguration!['debugEngineMode'] == 'external'
                  ? 'WireGuardKit backend is not integrated yet'
                  : null,
          'backendLastStartedAtMs': _tunnelRunning ? 1712345677000 : null,
          'backendPeerVirtualIp': _tunnelConfiguration!['peerVirtualIp'],
          'backendSelectedEndpoint': peer['endpoint'],
          'localVirtualIp': _tunnelConfiguration!['localVirtualIp'],
          'peerVirtualIp': _tunnelConfiguration!['peerVirtualIp'],
          'peerPublicKey': peer['publicKey'],
          'selectedEndpoint': peer['endpoint'],
          'remoteAddress': peer['endpoint'],
          'interfaceName': interface['interfaceName'],
          'mtu': interface['mtu'],
          'interfaceAddresses': interface['addresses'],
          'dnsServers': interface['dnsServers'],
          'allowedIps': (peer['allowedIps'] as List<Object?>)
              .map((item) => (item as Map<Object?, Object?>)['cidr'] as String)
              .toList(),
          'includedRoutes': (peer['allowedIps'] as List<Object?>)
              .map((item) => (item as Map<Object?, Object?>)['cidr'] as String)
              .toList(),
          'packetRxCount': _tunnelRunning ? 3 : 0,
          'packetRxBytes': _tunnelRunning ? 192 : 0,
          'packetTxCount': _tunnelRunning &&
                  _tunnelConfiguration!['debugEngineMode'] == 'loopback'
              ? 3
              : 0,
          'packetTxBytes': _tunnelRunning &&
                  _tunnelConfiguration!['debugEngineMode'] == 'loopback'
              ? 192
              : 0,
          'lastPacketAtMs': _tunnelRunning ? 1712345678123 : null,
          'lastAppliedAtMs': _tunnelRunning ? 1712345678000 : null,
          'lastError': null,
        };
    }

    throw PlatformException(
      code: 'missing_fake_response',
      message: 'Missing fake response for ${call.method}',
    );
  }
}

class HostCall {
  const HostCall({
    required this.method,
    required this.arguments,
  });

  final String method;
  final Map<String, Object?> arguments;
}

class DevicesPageHarness {
  const DevicesPageHarness(this.tester);

  final WidgetTester tester;

  Finder get _devicesScrollable => find
      .descendant(
        of: find.byKey(AppTestKeys.devicesScrollView),
        matching: find.byType(Scrollable),
      )
      .first;

  Finder get _sendPayloadField =>
      find.byKey(AppTestKeys.devicesSendPayloadField, skipOffstage: false);
  Finder get _sendButton =>
      find.byKey(AppTestKeys.devicesSendButton, skipOffstage: false);
  Finder get _probePayloadField =>
      find.byKey(AppTestKeys.devicesProbePayloadField, skipOffstage: false);
  Finder get _probeTimeoutField =>
      find.byKey(AppTestKeys.devicesProbeTimeoutField, skipOffstage: false);
  Finder get _probeButton =>
      find.byKey(AppTestKeys.devicesProbeButton, skipOffstage: false);
  Finder get _tunnelLocalIpField =>
      find.byKey(AppTestKeys.devicesTunnelLocalIpField);
  Finder get _tunnelPeerIpField =>
      find.byKey(AppTestKeys.devicesTunnelPeerIpField);
  Finder get _tunnelEndpointField =>
      find.byKey(AppTestKeys.devicesTunnelEndpointField);
  Finder get _tunnelDebugEngineModeField =>
      find.byKey(AppTestKeys.devicesTunnelDebugEngineModeField);
  Finder get _tunnelApplyButton =>
      find.byKey(AppTestKeys.devicesTunnelApplyButton);
  Finder get _tunnelUpButton => find.byKey(AppTestKeys.devicesTunnelUpButton);
  Finder get _tunnelViewButton =>
      find.byKey(AppTestKeys.devicesTunnelViewButton);
  Finder get _tunnelDownButton =>
      find.byKey(AppTestKeys.devicesTunnelDownButton);
  Finder get _tunnelRemoveButtonOffstage => find.byKey(
        AppTestKeys.devicesTunnelRemoveButton,
        skipOffstage: false,
      );
  Finder get _stateCard =>
      find.byKey(AppTestKeys.devicesStateCard, skipOffstage: false);

  Future<void> pumpAndConnectRelayFallback() async {
    tester.view.physicalSize = const Size(1440, 1400);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.reset);

    await tester.pumpWidget(
      const MaterialApp(
        home: DevicesPage(),
      ),
    );
    await tester.pumpAndSettle();

    await tester.enterText(
      find.byKey(AppTestKeys.devicesMachineIdField),
      'machine-1',
    );
    await tester.tap(find.byKey(AppTestKeys.devicesRegisterDeviceButton));
    await tester.pumpAndSettle();
    await tester.enterText(
      find.byKey(AppTestKeys.devicesNodeIdField),
      'node-1',
    );
    await tester.enterText(
      find.byKey(AppTestKeys.devicesNodePublicKeyField),
      'node-pubkey-1',
    );
    await _scrollUntilVisible(
        find.byKey(AppTestKeys.devicesRegisterNodeButton));
    await tester.tap(find.byKey(AppTestKeys.devicesRegisterNodeButton));
    await tester.pumpAndSettle();
    await _scrollUntilVisible(
      find.byKey(AppTestKeys.devicesBootstrapNetworkIdField),
    );
    await tester.enterText(
      find.byKey(AppTestKeys.devicesBootstrapNetworkIdField),
      'net-1',
    );
    await _scrollUntilVisible(
        find.byKey(AppTestKeys.devicesLoadBootstrapButton));
    await tester.tap(find.byKey(AppTestKeys.devicesLoadBootstrapButton));
    await tester.pumpAndSettle();
    await _scrollUntilVisible(find.byKey(AppTestKeys.devicesPeerNodeIdField));
    await tester.enterText(
      find.byKey(AppTestKeys.devicesPeerNodeIdField),
      'fail-peer-node-1',
    );
    await tester.enterText(
      find.byKey(AppTestKeys.devicesFailureReasonField),
      'timeout',
    );
    await _scrollUntilVisible(find.byKey(AppTestKeys.devicesConnectButton));
    await tester.tap(find.byKey(AppTestKeys.devicesConnectButton));
    await tester.pumpAndSettle();
  }

  Future<void> sendPayload(String payload) async {
    await _scrollUntilVisible(_sendPayloadField);
    await tester.enterText(_sendPayloadField, payload);
    await _scrollUntilVisible(_sendButton);
    await tester.tap(_sendButton);
    await tester.pumpAndSettle();
  }

  Future<void> probePayload(String payload, {required String timeoutMs}) async {
    await _scrollUntilVisible(_probePayloadField);
    await tester.enterText(_probePayloadField, payload);
    await tester.enterText(_probeTimeoutField, timeoutMs);
    await _scrollUntilVisible(_probeButton);
    await tester.tap(_probeButton);
    await tester.pumpAndSettle();
  }

  Future<void> scrollToStateCard() async {
    await _scrollUntilVisible(_stateCard);
    await tester.pumpAndSettle();
  }

  Future<void> expectConnectionGuidance() async {
    await _scrollUntilVisible(find.text('Connection Guidance'));
    expect(find.text('Connection Guidance'), findsOneWidget);
    expect(find.text('relay relay-cn-local-udp'), findsWidgets);
    expect(find.text('relay-cn-local-udp'), findsWidgets);
    expect(find.text('matched'), findsWidgets);
    expect(
      find.textContaining(
        'Connected over relay path. Recommendation matched.',
        findRichText: true,
      ),
      findsOneWidget,
    );
  }

  Future<void> applyTunnelConfiguration({
    required String localVirtualIp,
    required String peerVirtualIp,
    required String endpoint,
    String? debugEngineMode,
  }) async {
    await _scrollUntilVisible(_tunnelLocalIpField);
    await tester.enterText(_tunnelLocalIpField, localVirtualIp);
    await tester.enterText(_tunnelPeerIpField, peerVirtualIp);
    await tester.enterText(_tunnelEndpointField, endpoint);
    await tester.enterText(_tunnelDebugEngineModeField, debugEngineMode ?? '');
    await _scrollUntilVisible(_tunnelApplyButton);
    await tester.tap(_tunnelApplyButton);
    await tester.pumpAndSettle();
  }

  Future<void> bringTunnelUp() async {
    await _scrollUntilVisible(_tunnelUpButton);
    await tester.tap(_tunnelUpButton);
    await tester.pumpAndSettle();
  }

  Future<void> viewTunnelRuntime() async {
    await _scrollUntilVisible(_tunnelViewButton);
    await tester.tap(_tunnelViewButton);
    await tester.pumpAndSettle();
  }

  Future<void> bringTunnelDown() async {
    await _scrollUntilVisible(_tunnelDownButton);
    await tester.tap(_tunnelDownButton);
    await tester.pumpAndSettle();
  }

  Future<void> removeTunnelPeer() async {
    await _scrollUntilVisible(_tunnelRemoveButtonOffstage);
    final button =
        tester.widget<OutlinedButton>(_tunnelRemoveButtonOffstage.first);
    button.onPressed!.call();
    await tester.pumpAndSettle();
  }

  void expectSendSuccess({required int bytes}) {
    expect(find.text('Send bytes'), findsWidgets);
    expect(find.text('$bytes'), findsWidgets);
    expect(find.text('Send failure'), findsWidgets);
  }

  void expectSendFailure({
    required String kind,
    required String detail,
  }) {
    expect(find.text('Send bytes'), findsWidgets);
    expect(find.text('Send failure'), findsWidgets);
    expect(find.textContaining(kind, findRichText: true), findsWidgets);
    expect(find.textContaining(detail, findRichText: true), findsWidgets);
  }

  void expectProbeSuccess({
    required String probeId,
    required int bytes,
    required bool replyObserved,
    required int replyRttMs,
  }) {
    expect(find.text('Probe'), findsWidgets);
    expect(find.text(probeId), findsWidgets);
    expect(find.text('Send bytes'), findsWidgets);
    expect(find.text('$bytes'), findsWidgets);
    expect(find.text('Probe RTT'), findsWidgets);
    expect(find.text('$replyRttMs'), findsWidgets);
    expect(replyObserved, isTrue);
  }

  void expectProbeFailure({
    required String kind,
    required String detail,
  }) {
    expect(find.text('Probe'), findsWidgets);
    expect(find.text('none'), findsWidgets);
    expect(find.text('Probe failure'), findsWidgets);
    expect(find.textContaining(kind, findRichText: true), findsWidgets);
    expect(find.textContaining(detail, findRichText: true), findsWidgets);
  }

  void expectTunnelRuntime({
    required String state,
    required String transport,
    required String peerVirtualIp,
    required String endpoint,
    required String interfaceName,
    String? debugEngineMode,
    String? backendName,
    String? backendState,
    String? backendLastError,
    int? backendLastStartedAtMs,
    String? backendPeerVirtualIp,
    String? backendSelectedEndpoint,
  }) {
    expect(find.text('Tunnel Runtime'), findsOneWidget);
    expect(find.text('state $state'), findsWidgets);
    expect(find.text('backend ${backendState ?? 'unavailable'}'), findsWidgets);
    expect(find.text('engine ${debugEngineMode ?? '-'}'), findsWidgets);
    expect(find.text(transport), findsWidgets);
    expect(find.text(peerVirtualIp), findsWidgets);
    expect(find.text(endpoint), findsWidgets);
    expect(
      find.text('${backendName ?? '-'} / ${backendState ?? '-'}'),
      findsWidgets,
    );
    expect(find.text('${backendLastStartedAtMs ?? '-'}'), findsWidgets);
    expect(find.text(backendPeerVirtualIp ?? '-'), findsWidgets);
    expect(find.text(backendSelectedEndpoint ?? '-'), findsWidgets);
    expect(find.text(backendLastError ?? '-'), findsWidgets);
  }

  void expectTunnelRuntimeCleared() {
    expect(find.text('Tunnel Runtime'), findsOneWidget);
    expect(find.text('state idle'), findsWidgets);
    expect(find.text('backend unavailable'), findsWidgets);
    expect(find.text('engine -'), findsWidgets);
  }

  Future<void> _scrollUntilVisible(Finder finder) async {
    await tester.scrollUntilVisible(
      finder,
      200,
      scrollable: _devicesScrollable,
    );
  }
}

Future<void> cleanupDesktopIntegrationApp() async {
  if (Platform.isMacOS) {
    await Process.run('pkill', [
      '-f',
      'slan_app.app/Contents/MacOS/slan_app',
    ]);
  }
}
