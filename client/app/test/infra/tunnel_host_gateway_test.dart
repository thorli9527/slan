import 'dart:convert';
import 'dart:io';

import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:slan_app/application/tunnel_host_gateway.dart';
import 'package:slan_app/infra/logging/startup_log.dart';
import 'package:slan_app_core_plugin/slan_app_core_plugin.dart';

void main() {
  test('HelperServiceTunnelHostGateway parses action results over TCP', () async {
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

  test('HelperServiceTunnelHostGateway surfaces structured helper errors', () async {
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

  test('HelperServiceTunnelHostGateway returns null runtime view payloads', () async {
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

  test('HelperServiceTunnelHostGateway rejects invalid helper host addresses', () async {
    final gateway = HelperServiceTunnelHostGateway(address: 'not-a-host');

    await expectLater(
      gateway.bringTunnelUp(),
      throwsA(isA<FormatException>()),
    );
  });
}
