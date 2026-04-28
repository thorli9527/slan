import 'dart:convert';
import 'dart:io';

import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:slan_app/application/tunnel_host_gateway.dart';
import 'package:slan_app_core_plugin/slan_app_core_plugin.dart';

void main() {
  test('HelperServiceTunnelHostGateway parses action results over TCP',
      () async {
    final server = await ServerSocket.bind(InternetAddress.loopbackIPv4, 0);
    addTearDown(server.close);
    server.listen((client) {
      client
          .cast<List<int>>()
          .transform(utf8.decoder)
          .transform(const LineSplitter())
          .listen((line) {
        final request = jsonDecode(line) as Map<String, dynamic>;
        expect(request['method'], 'bringTunnelUp');
        client.writeln(jsonEncode({
          'ok': true,
          'result': {
            'action': 'bringTunnelUp',
            'accepted': true,
            'phase': 'started',
            'source': 'rust-helper',
            'detail': 'started',
            'connectionStatus': 'connected',
            'hasConfiguration': true,
            'configurationPeerVirtualIp': '100.64.0.2',
            'runtimeState': 'configured',
            'backendState': 'started',
          },
        }));
      });
    });

    final gateway = HelperServiceTunnelHostGateway(
      address: '127.0.0.1:${server.port}',
    );

    final result = await gateway.bringTunnelUp();
    expect(result.accepted, isTrue);
    expect(result.action, 'bringTunnelUp');
    expect(result.phase, WireGuardTunnelActionPhase.started);
    expect(result.configurationPeerVirtualIp, '100.64.0.2');
  });

  test('HelperServiceTunnelHostGateway surfaces structured helper errors',
      () async {
    final server = await ServerSocket.bind(InternetAddress.loopbackIPv4, 0);
    addTearDown(server.close);
    server.listen((client) {
      client.writeln(jsonEncode({
        'ok': false,
        'errorCode': 'app_core_helper_error',
        'errorMessage': 'helper failed',
        'error': 'helper failed',
      }));
    });

    final gateway = HelperServiceTunnelHostGateway(
      address: 'tcp://127.0.0.1:${server.port}',
    );

    await expectLater(
      gateway.bringTunnelDown(),
      throwsA(
        isA<PlatformException>()
            .having((error) => error.code, 'code', 'app_core_helper_error')
            .having((error) => error.message, 'message', 'helper failed'),
      ),
    );
  });

  test('HelperServiceTunnelHostGateway returns null runtime view payloads',
      () async {
    final server = await ServerSocket.bind(InternetAddress.loopbackIPv4, 0);
    addTearDown(server.close);
    server.listen((client) {
      client.writeln(jsonEncode({
        'ok': true,
        'result': null,
      }));
    });

    final gateway = HelperServiceTunnelHostGateway(
      address: '127.0.0.1:${server.port}',
    );

    final runtime = await gateway.tunnelRuntimeView('100.64.0.2');
    expect(runtime, isNull);
  });

  test('HelperServiceTunnelHostGateway rejects invalid helper host addresses',
      () async {
    const gateway = HelperServiceTunnelHostGateway(address: 'not-a-host');

    await expectLater(
      gateway.bringTunnelUp(),
      throwsA(isA<FormatException>()),
    );
  });

  test('PluginTunnelHostGateway delegates tunnel actions to plugin platform',
      () async {
    final platform = _FakeTunnelPluginPlatform();
    final gateway = PluginTunnelHostGateway(pluginPlatform: platform);
    const configuration = WireGuardTunnelConfiguration(
      transport: 'relay',
      localVirtualIp: '100.64.0.10',
      peerVirtualIp: '100.64.0.2',
      interface: WireGuardTunnelInterfaceConfiguration(
        interfaceName: 'utun9',
        keyPair: WireGuardTunnelKeyPair(
          publicKey: 'pub',
          privateKey: 'priv',
        ),
        listenPort: 51820,
        mtu: 1280,
        addresses: ['100.64.0.10/32'],
        dnsServers: ['1.1.1.1'],
      ),
      peer: WireGuardTunnelPeerConfiguration(
        publicKey: 'peer-pub',
        endpoint: '203.0.113.10:51820',
        allowedIps: ['100.64.0.2/32'],
      ),
      debugEngineMode: 'loopback',
    );

    final apply = await gateway.applyTunnelConfiguration(configuration);
    final up = await gateway.bringTunnelUp();
    final down = await gateway.bringTunnelDown();
    final remove = await gateway.removeTunnelPeer('100.64.0.2');
    final runtime = await gateway.tunnelRuntimeView('100.64.0.2');

    expect(apply.action, 'applyTunnelConfiguration');
    expect(up.action, 'bringTunnelUp');
    expect(down.action, 'bringTunnelDown');
    expect(remove.action, 'removeTunnelPeer');
    expect(runtime?.peerVirtualIp, '100.64.0.2');
    expect(
      platform.calls,
      <String>[
        'applyTunnelConfiguration',
        'bringTunnelUp',
        'bringTunnelDown',
        'removeTunnelPeer',
        'tunnelRuntimeView',
      ],
    );
  });
}

class _FakeTunnelPluginPlatform extends SlanAppCorePluginPlatform {
  final List<String> calls = [];

  @override
  Future<WireGuardTunnelActionResult> applyTunnelConfiguration(
    WireGuardTunnelConfiguration configuration,
  ) async {
    calls.add('applyTunnelConfiguration');
    return _actionResult('applyTunnelConfiguration');
  }

  @override
  Future<WireGuardTunnelActionResult> bringTunnelUp() async {
    calls.add('bringTunnelUp');
    return _actionResult('bringTunnelUp');
  }

  @override
  Future<WireGuardTunnelActionResult> bringTunnelDown() async {
    calls.add('bringTunnelDown');
    return _actionResult('bringTunnelDown');
  }

  @override
  Future<WireGuardTunnelActionResult> removeTunnelPeer(
    String peerVirtualIp,
  ) async {
    calls.add('removeTunnelPeer');
    return _actionResult('removeTunnelPeer');
  }

  @override
  Future<WireGuardTunnelRuntimeView?> tunnelRuntimeView(
    String peerVirtualIp,
  ) async {
    calls.add('tunnelRuntimeView');
    return WireGuardTunnelRuntimeView.fromJson({
      'state': 'configured',
      'transport': 'relay',
      'peerPublicKey': 'peer-pub',
      'peerVirtualIp': peerVirtualIp,
      'localVirtualIp': '100.64.0.10',
      'remoteAddress': '203.0.113.10:51820',
      'selectedEndpoint': '203.0.113.10:51820',
      'interfaceName': 'utun9',
      'backendName': 'wireguardkit',
      'backendState': 'started',
      'packetRxCount': 3,
      'packetRxBytes': 192,
      'packetTxCount': 3,
      'packetTxBytes': 192,
    });
  }

  WireGuardTunnelActionResult _actionResult(String action) {
    return WireGuardTunnelActionResult.fromJson({
      'action': action,
      'accepted': true,
      'phase': 'accepted',
      'source': 'plugin',
      'detail': '$action accepted',
    });
  }
}
