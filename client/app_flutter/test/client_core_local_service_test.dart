import 'dart:async';
import 'dart:convert';
import 'dart:io';

import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:slan_client_v2/bridge/client_core_local_service.dart';

void main() {
  test('platform network config uses command response timeout', () {
    final service = ClientCoreLocalService();

    expect(
      service.responseTimeoutFor('localPlatformNetworkConfig', null),
      const Duration(seconds: 65),
    );
  });

  test('local service returns first JSON line', () async {
    final server = await ServerSocket.bind(InternetAddress.loopbackIPv4, 0);
    final receivedRequest = Completer<Map<String, Object?>>();
    final subscription = server.listen((socket) async {
      final line = await socket
          .cast<List<int>>()
          .transform(utf8.decoder)
          .transform(const LineSplitter())
          .first;
      receivedRequest.complete(
        (jsonDecode(line) as Map).cast<String, Object?>(),
      );
      socket.writeln('{"activated":true}');
      await socket.flush();
      socket.destroy();
    });
    addTearDown(() async {
      await subscription.cancel();
      await server.close();
    });

    final service = ClientCoreLocalService(
      host: '127.0.0.1:${server.port}',
    );
    final state = await service.localState();
    final request = await receivedRequest.future;
    final arguments = (request['args'] as Map).cast<String, Object?>();

    expect(state?['activated'], true);
    expect(arguments['requestId'], startsWith('flutter'));
  });

  test('local service preserves caller request id', () async {
    final server = await ServerSocket.bind(InternetAddress.loopbackIPv4, 0);
    final receivedRequest = Completer<Map<String, Object?>>();
    final subscription = server.listen((socket) async {
      final line = await socket
          .cast<List<int>>()
          .transform(utf8.decoder)
          .transform(const LineSplitter())
          .first;
      receivedRequest.complete(
        (jsonDecode(line) as Map).cast<String, Object?>(),
      );
      socket.writeln('{"ok":true}');
      await socket.flush();
      socket.destroy();
    });
    addTearDown(() async {
      await subscription.cancel();
      await server.close();
    });

    final service = ClientCoreLocalService(
      host: '127.0.0.1:${server.port}',
    );
    await service.request(
      'localState',
      arguments: {'requestId': 'caller-request-1'},
    );
    final request = await receivedRequest.future;
    final arguments = (request['args'] as Map).cast<String, Object?>();

    expect(arguments['requestId'], 'caller-request-1');
  });

  test('local service timeout closes request with stable error code', () async {
    final server = await ServerSocket.bind(InternetAddress.loopbackIPv4, 0);
    final sockets = <Socket>[];
    final subscription = server.listen(sockets.add);
    addTearDown(() async {
      for (final socket in sockets) {
        socket.destroy();
      }
      await subscription.cancel();
      await server.close();
    });

    final service = ClientCoreLocalService(
      host: '127.0.0.1:${server.port}',
      responseTimeoutOverride: const Duration(milliseconds: 20),
    );

    await expectLater(
      service.localState(),
      throwsA(
        isA<PlatformException>().having(
          (error) => error.code,
          'code',
          'local_service_timeout',
        ),
      ),
    );
  });
}
