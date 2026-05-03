import 'dart:convert';
import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:slan_client_v2/bridge/client_commands.dart';
import 'package:slan_client_v2/bridge/client_core_bridge.dart';
import 'package:slan_client_v2/bridge/client_view_state.dart';

void main() {
  test('enable switch updates asynchronously after service result', () async {
    final service = await _FakeClientService.start([
      _ServiceReply(
        delay: const Duration(milliseconds: 150),
        expectedMethod: 'watchBusinessEvent',
        body: _businessEvent(
          ClientBusinessEventType.networkSwitchFinished,
          {
            'signedIn': true,
            'networkEnabled': true,
            'syncing': false,
            'switchEnabled': true,
            'virtualIp': '100.64.0.10',
          },
        ),
      ),
      _ServiceReply(
        expectedMethod: 'state',
        body: {
          'signedIn': true,
          'networkEnabled': true,
          'syncing': false,
          'switchEnabled': true,
          'virtualIp': '100.64.0.10',
        },
      ),
      _ServiceReply(
        expectedMethod: 'activateNetwork',
        body: {
          'signedIn': true,
          'networkEnabled': true,
          'syncing': false,
          'switchEnabled': true,
          'virtualIp': '100.64.0.10',
        },
      ),
    ]);
    addTearDown(service.close);

    final bridge = MethodChannelClientCoreBridge(
      localServiceHost: service.host,
    );
    await bridge.start();

    final stopwatch = Stopwatch()..start();
    await bridge.dispatch(
      const ClientCommand(ClientCommandType.enableNetwork),
    );
    stopwatch.stop();

    expect(stopwatch.elapsedMilliseconds, lessThan(100));
    expect(bridge.state.value.syncing, isTrue);
    expect(bridge.state.value.switchEnabled, isFalse);
    expect(bridge.state.value.networkEnabled, isTrue);

    await _waitFor(
      () => bridge.state.value.virtualIp == '100.64.0.10',
      reason: 'enable result should update state asynchronously',
    );
    expect(bridge.state.value.syncing, isFalse);
    expect(bridge.state.value.switchEnabled, isTrue);
    expect(bridge.state.value.virtualIp, '100.64.0.10');
  });

  test('in-flight toggle ignores repeated clicks', () async {
    final service = await _FakeClientService.start([
      _ServiceReply(
        delay: const Duration(milliseconds: 250),
        expectedMethod: 'watchBusinessEvent',
        body: _businessEvent(
          ClientBusinessEventType.networkSwitchFinished,
          {
            'signedIn': true,
            'networkEnabled': true,
            'syncing': false,
            'switchEnabled': true,
            'virtualIp': '100.64.0.10',
          },
        ),
      ),
      _ServiceReply(
        expectedMethod: 'state',
        body: {
          'signedIn': true,
          'networkEnabled': true,
          'syncing': false,
          'switchEnabled': true,
          'virtualIp': '100.64.0.10',
        },
      ),
      _ServiceReply(
        expectedMethod: 'activateNetwork',
        body: {
          'signedIn': true,
          'networkEnabled': true,
          'syncing': false,
          'switchEnabled': true,
          'virtualIp': '100.64.0.10',
        },
      ),
    ]);
    addTearDown(service.close);

    final bridge = MethodChannelClientCoreBridge(
      localServiceHost: service.host,
    );
    await bridge.start();

    await bridge.dispatch(
      const ClientCommand(ClientCommandType.enableNetwork),
    );
    await bridge.dispatch(
      const ClientCommand(ClientCommandType.disableNetwork),
    );
    await bridge.dispatch(
      const ClientCommand(ClientCommandType.enableNetwork),
    );

    await _waitFor(
      () => bridge.state.value.virtualIp == '100.64.0.10',
      reason: 'first toggle should settle without repeated clicks racing it',
    );
    expect(service.seenMethods, isNot(contains('deactivateNetwork')));
    expect(
      service.seenMethods.where((method) => method == 'activateNetwork'),
      hasLength(1),
    );
    expect(bridge.state.value.networkEnabled, isTrue);
    expect(bridge.state.value.switchEnabled, isTrue);
    expect(bridge.state.value.virtualIp, '100.64.0.10');
  });

  test('disable switch updates asynchronously after service result', () async {
    final service = await _FakeClientService.start([
      _ServiceReply(
        delay: const Duration(milliseconds: 150),
        expectedMethod: 'watchBusinessEvent',
        body: _businessEvent(
          ClientBusinessEventType.networkSwitchFinished,
          {
            'signedIn': true,
            'networkEnabled': false,
            'syncing': false,
            'switchEnabled': true,
            'virtualIp': null,
          },
        ),
      ),
      _ServiceReply(
        expectedMethod: 'state',
        body: {
          'signedIn': true,
          'networkEnabled': false,
          'syncing': false,
          'switchEnabled': true,
          'virtualIp': null,
        },
      ),
      _ServiceReply(
        expectedMethod: 'deactivateNetwork',
        body: {
          'signedIn': true,
          'networkEnabled': false,
          'syncing': false,
          'switchEnabled': true,
          'virtualIp': null,
        },
      ),
    ]);
    addTearDown(service.close);

    final bridge = MethodChannelClientCoreBridge(
      localServiceHost: service.host,
    );
    await bridge.start();

    final stopwatch = Stopwatch()..start();
    await bridge.dispatch(
      const ClientCommand(ClientCommandType.disableNetwork),
    );
    stopwatch.stop();

    expect(stopwatch.elapsedMilliseconds, lessThan(100));
    expect(bridge.state.value.syncing, isTrue);
    expect(bridge.state.value.switchEnabled, isFalse);
    expect(bridge.state.value.networkEnabled, isFalse);
    expect(bridge.state.value.virtualIp, isNull);

    await _waitFor(
      () => bridge.state.value.syncing == false,
      reason: 'disable result should update state asynchronously',
    );
    expect(bridge.state.value.switchEnabled, isTrue);
    expect(bridge.state.value.networkEnabled, isFalse);
    expect(bridge.state.value.virtualIp, isNull);
  });

  test('enable failure restores switch', () async {
    final service = await _FakeClientService.start([
      _ServiceReply(
        delay: const Duration(milliseconds: 80),
        expectedMethod: 'watchBusinessEvent',
        body: _businessEvent(
          ClientBusinessEventType.networkSwitchFailed,
          {
            'signedIn': true,
            'networkEnabled': false,
            'syncing': false,
            'switchEnabled': true,
            'error': 'device unavailable: current device has been disabled',
          },
        ),
      ),
      _ServiceReply(
        expectedMethod: 'state',
        body: {
          'signedIn': true,
          'networkEnabled': false,
          'syncing': false,
          'switchEnabled': true,
          'error': 'device unavailable: current device has been disabled',
        },
      ),
      _ServiceReply(
        expectedMethod: 'activateNetwork',
        body: {
          'signedIn': true,
          'networkEnabled': false,
          'syncing': false,
          'switchEnabled': true,
          'error': 'device unavailable: current device has been disabled',
        },
      ),
    ]);
    addTearDown(service.close);

    final bridge = MethodChannelClientCoreBridge(
      localServiceHost: service.host,
    );
    await bridge.start();

    await bridge.dispatch(
      const ClientCommand(ClientCommandType.enableNetwork),
    );

    await _waitFor(
      () => bridge.state.value.error != null,
      reason: 'enable failure should be shown',
    );
    expect(bridge.state.value.networkEnabled, isFalse);
    expect(bridge.state.value.syncing, isFalse);
    expect(bridge.state.value.switchEnabled, isTrue);
    expect(bridge.state.value.errorSource, ClientErrorSource.networkSwitch);
  });

  test('service error result restores switch without waiting for event timeout',
      () async {
    final service = await _FakeClientService.start([
      _ServiceReply(
        expectedMethod: 'activateNetwork',
        body: {
          'signedIn': true,
          'networkEnabled': false,
          'syncing': false,
          'switchEnabled': true,
          'error': '服务端停用，请联系管理员',
        },
      ),
    ]);
    addTearDown(service.close);

    final bridge = MethodChannelClientCoreBridge(
      localServiceHost: service.host,
    );

    await bridge.dispatch(
      const ClientCommand(ClientCommandType.enableNetwork),
    );

    await _waitFor(
      () => bridge.state.value.errorSource == ClientErrorSource.networkSwitch,
      reason: 'service error should settle the switch immediately',
    );
    expect(bridge.state.value.error, '服务端停用，请联系管理员');
    expect(bridge.state.value.networkEnabled, isFalse);
    expect(bridge.state.value.syncing, isFalse);
    expect(bridge.state.value.switchEnabled, isTrue);
    expect(service.seenMethods, isNot(contains('watchBusinessEvent')));
  });

  test('network switch failed keeps event error after state query', () async {
    final service = await _FakeClientService.start([
      _ServiceReply(
        delay: const Duration(milliseconds: 80),
        expectedMethod: 'watchBusinessEvent',
        body: _businessEvent(
          ClientBusinessEventType.networkSwitchFailed,
          {
            'signedIn': true,
            'networkEnabled': false,
            'syncing': false,
            'switchEnabled': true,
            'error':
                'device unavailable: current device has been disabled by network admin',
          },
        ),
      ),
      _ServiceReply(
        expectedMethod: 'state',
        body: {
          'signedIn': true,
          'networkEnabled': false,
          'syncing': false,
          'switchEnabled': true,
        },
      ),
      _ServiceReply(
        expectedMethod: 'activateNetwork',
        body: {
          'signedIn': true,
          'networkEnabled': false,
          'syncing': false,
          'switchEnabled': true,
          'error':
              'device unavailable: current device has been disabled by network admin',
        },
      ),
    ]);
    addTearDown(service.close);

    final bridge = MethodChannelClientCoreBridge(
      localServiceHost: service.host,
    );
    await bridge.start();

    await bridge.dispatch(
      const ClientCommand(ClientCommandType.enableNetwork),
    );

    await _waitFor(
      () => bridge.state.value.errorSource == ClientErrorSource.networkSwitch,
      reason: 'failed event error should be preserved',
    );
    expect(
      bridge.state.value.error,
      'device unavailable: current device has been disabled by network admin',
    );
    expect(bridge.state.value.networkEnabled, isFalse);
    expect(bridge.state.value.switchEnabled, isTrue);
  });

  test('enable exception rolls back optimistic switch state', () async {
    final service = await _FakeClientService.start([
      const _ServiceReply(
        expectedMethod: 'activateNetwork',
        closeWithoutResponse: true,
        body: {},
      ),
    ]);
    addTearDown(service.close);

    final bridge = MethodChannelClientCoreBridge(
      localServiceHost: service.host,
    );

    await bridge.dispatch(
      const ClientCommand(ClientCommandType.enableNetwork),
    );

    expect(bridge.state.value.networkEnabled, isTrue);
    expect(bridge.state.value.switchEnabled, isFalse);

    await _waitFor(
      () => bridge.state.value.errorSource == ClientErrorSource.networkSwitch,
      reason: 'enable exception should be marked as network switch failure',
    );
    expect(bridge.state.value.networkEnabled, isFalse);
    expect(bridge.state.value.syncing, isFalse);
    expect(bridge.state.value.switchEnabled, isTrue);
    expect(bridge.state.value.virtualIp, isNull);
  });

  test('disable exception restores previous enabled state and ip', () async {
    final service = await _FakeClientService.start([
      _ServiceReply(
        delay: const Duration(milliseconds: 80),
        expectedMethod: 'watchBusinessEvent',
        body: _businessEvent(
          ClientBusinessEventType.networkSwitchFinished,
          {
            'signedIn': true,
            'networkEnabled': true,
            'syncing': false,
            'switchEnabled': true,
            'virtualIp': '100.64.0.10',
          },
        ),
      ),
      _ServiceReply(
        expectedMethod: 'state',
        body: {
          'signedIn': true,
          'networkEnabled': true,
          'syncing': false,
          'switchEnabled': true,
          'virtualIp': '100.64.0.10',
        },
      ),
      _ServiceReply(
        expectedMethod: 'activateNetwork',
        body: {
          'signedIn': true,
          'networkEnabled': true,
          'syncing': false,
          'switchEnabled': true,
          'virtualIp': '100.64.0.10',
        },
      ),
      const _ServiceReply(
        expectedMethod: 'deactivateNetwork',
        closeWithoutResponse: true,
        body: {},
      ),
    ]);
    addTearDown(service.close);

    final bridge = MethodChannelClientCoreBridge(
      localServiceHost: service.host,
    );
    await bridge.start();

    await bridge.dispatch(
      const ClientCommand(ClientCommandType.enableNetwork),
    );
    await _waitFor(
      () => bridge.state.value.virtualIp == '100.64.0.10',
      reason: 'precondition enable should settle',
    );

    await bridge.dispatch(
      const ClientCommand(ClientCommandType.disableNetwork),
    );

    expect(bridge.state.value.networkEnabled, isFalse);
    expect(bridge.state.value.virtualIp, isNull);
    expect(bridge.state.value.switchEnabled, isFalse);

    await _waitFor(
      () => bridge.state.value.errorSource == ClientErrorSource.networkSwitch,
      reason: 'disable exception should be marked as network switch failure',
    );
    expect(bridge.state.value.networkEnabled, isTrue);
    expect(bridge.state.value.syncing, isFalse);
    expect(bridge.state.value.switchEnabled, isTrue);
    expect(bridge.state.value.virtualIp, '100.64.0.10');
  });

  test('successful switch does not issue extra refresh', () async {
    final service = await _FakeClientService.start([
      _ServiceReply(
        delay: const Duration(milliseconds: 80),
        expectedMethod: 'watchBusinessEvent',
        body: _businessEvent(
          ClientBusinessEventType.networkSwitchFinished,
          {
            'signedIn': true,
            'networkEnabled': true,
            'syncing': false,
            'switchEnabled': true,
            'virtualIp': '100.64.0.10',
          },
        ),
      ),
      _ServiceReply(
        expectedMethod: 'state',
        body: {
          'signedIn': true,
          'networkEnabled': true,
          'syncing': false,
          'switchEnabled': true,
          'virtualIp': '100.64.0.10',
        },
      ),
      _ServiceReply(
        expectedMethod: 'activateNetwork',
        body: {
          'signedIn': true,
          'networkEnabled': true,
          'syncing': false,
          'switchEnabled': true,
          'virtualIp': '100.64.0.10',
        },
      ),
    ]);
    addTearDown(service.close);

    final bridge = MethodChannelClientCoreBridge(
      localServiceHost: service.host,
    );
    await bridge.start();

    await bridge.dispatch(
      const ClientCommand(ClientCommandType.enableNetwork),
    );

    await _waitFor(
      () => bridge.state.value.virtualIp == '100.64.0.10',
      reason: 'enable result should settle',
    );
    await Future<void>.delayed(const Duration(milliseconds: 50));
    expect(service.seenMethods, isNot(contains('refresh')));
    expect(service.seenMethods,
        containsAll(['activateNetwork', 'watchBusinessEvent', 'state']));
  });

  test('logout clears local service session before returning', () async {
    final service = await _FakeClientService.start([
      _ServiceReply(
        expectedMethod: 'logout',
        body: {
          'signedIn': false,
          'networkEnabled': false,
          'syncing': false,
          'switchEnabled': true,
          'notice': 'signedOut',
        },
      ),
    ]);
    addTearDown(service.close);

    final bridge = MethodChannelClientCoreBridge(
      localServiceHost: service.host,
    );

    await bridge.dispatch(
      const ClientCommand(ClientCommandType.logout),
    );

    expect(service.seenMethods, contains('logout'));
    expect(bridge.state.value.signedIn, isFalse);
    expect(bridge.state.value.networkEnabled, isFalse);
    expect(bridge.state.value.virtualIp, isNull);
  });

  test('disabled network state does not expose stale virtual ip', () {
    final state = ClientViewState.fromJson({
      'signedIn': true,
      'networkEnabled': false,
      'syncing': false,
      'switchEnabled': true,
      'virtualIp': '10.0.0.99',
    });

    expect(state.networkEnabled, isFalse);
    expect(state.virtualIp, isNull);
  });

}

class _ServiceReply {
  const _ServiceReply({
    required this.body,
    this.delay = Duration.zero,
    this.expectedMethod,
    this.closeWithoutResponse = false,
  });

  final Map<String, Object?> body;
  final Duration delay;
  final String? expectedMethod;
  final bool closeWithoutResponse;
}

class _FakeClientService {
  _FakeClientService(this._server, this._replies);

  final ServerSocket _server;
  final List<_ServiceReply> _replies;
  int _index = 0;
  final List<String> seenMethods = [];

  String get host => '127.0.0.1:${_server.port}';

  static Future<_FakeClientService> start(List<_ServiceReply> replies) async {
    final server = await ServerSocket.bind(InternetAddress.loopbackIPv4, 0);
    final service = _FakeClientService(server, replies);
    server.listen(service._handle);
    return service;
  }

  Future<void> close() => _server.close();

  Future<void> _handle(Socket socket) async {
    try {
      final request = await socket
          .cast<List<int>>()
          .transform(utf8.decoder)
          .transform(const LineSplitter())
          .first;
      final requestJson = jsonDecode(request) as Map<String, Object?>;
      final method = requestJson['method'];
      if (method is String) {
        seenMethods.add(method);
      }
      final reply = _takeReply(method);
      if (reply.expectedMethod != null && method != reply.expectedMethod) {
        socket.write(
          '${jsonEncode({
                'signedIn': true,
                'networkEnabled': false,
                'syncing': false,
                'switchEnabled': true,
                'error': 'expected ${reply.expectedMethod}, got $method',
              })}\n',
        );
        await socket.flush();
        return;
      }
      if (reply.delay > Duration.zero) {
        await Future<void>.delayed(reply.delay);
      }
      if (reply.closeWithoutResponse) {
        return;
      }
      socket.write('${jsonEncode(reply.body)}\n');
      await socket.flush();
    } finally {
      await socket.close();
    }
  }

  _ServiceReply _takeReply(Object? method) {
    if (_replies.isEmpty) {
      return const _ServiceReply(body: {});
    }
    if (method is String) {
      for (var index = _index; index < _replies.length; index += 1) {
        final reply = _replies[index];
        if (reply.expectedMethod == null || reply.expectedMethod == method) {
          _replies.removeAt(index);
          if (_index > index) {
            _index -= 1;
          }
          return reply;
        }
      }
      return const _ServiceReply(
        body: {},
        closeWithoutResponse: true,
      );
    }
    final safeIndex = _index.clamp(0, _replies.length - 1);
    final reply = _replies.removeAt(safeIndex);
    if (_index >= _replies.length) {
      _index = _replies.length - 1;
    }
    if (_index < 0) {
      _index = 0;
    }
    return reply;
  }
}

Map<String, Object?> _businessEvent(
  String businessType,
  Map<String, Object?> state,
) {
  return {
    'revision': 1,
    'businessType': businessType,
    'businessData': state,
    'snapshot': state,
  };
}

Future<void> _waitFor(
  bool Function() condition, {
  required String reason,
}) async {
  final deadline = DateTime.now().add(const Duration(seconds: 2));
  while (!condition()) {
    if (DateTime.now().isAfter(deadline)) {
      fail(reason);
    }
    await Future<void>.delayed(const Duration(milliseconds: 20));
  }
}
