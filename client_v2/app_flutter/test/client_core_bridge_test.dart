import 'dart:convert';
import 'dart:io';

import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:slan_client_v2/bridge/client_commands.dart';
import 'package:slan_client_v2/bridge/client_core_bridge.dart';
import 'package:slan_client_v2/bridge/client_view_state.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  test('android runtime diagnostics preserve native relay counters', () {
    final fields = androidRuntimeDiagnosticsFields({
      'adapterPresent': true,
      'networkEnabled': true,
      'virtualIp': '10.0.0.2',
      'mtu': 1280,
      'relayAddress': '127.0.0.1:3478',
      'relaySessionCount': 2,
      'requestedRelaySessionCount': 2,
      'attachedRelaySessionCount': 1,
      'relayAttachFailures': 1,
      'lastRelayAttachError': 'relay attach timed out',
      'packetsRead': 7,
      'bytesRead': 900,
      'packetsTooLarge': 1,
      'relayFramesSent': 3,
      'relayFramesReceived': 4,
      'relayDetachSent': 1,
      'relayNoPeerPackets': 2,
      'relayWriteFailures': 1,
      'tunWriteFailures': 1,
    });

    expect(fields['requestedRelaySessionCount'], 2);
    expect(fields['attachedRelaySessionCount'], 1);
    expect(fields['relayDetachSent'], 1);
    expect(fields['lastRelayAttachError'], 'relay attach timed out');
  });

  test('ios packet tunnel diagnostics preserve routing counters', () {
    final fields = iosPacketTunnelDiagnosticsFields({
      'relaySessionCount': 2,
      'relayAttachedSessionCount': 2,
      'relayAttachFailures': 0,
      'lastRelayAttachError': null,
      'packetsRead': 12,
      'bytesRead': 1800,
      'routedPackets': 9,
      'unroutablePackets': 2,
      'nonIpv4Packets': 1,
      'relayFramesSent': 8,
      'relayFramesReceived': 7,
      'relayPacketsWritten': 6,
      'relayDetachSent': 2,
      'relayNoPeerPackets': 2,
      'lastDestination': '10.0.0.3',
      'lastRoute': '10.0.0.3/32',
      'lastRoutedAtMs': 1780000000000,
      'updatedAtMs': 1780000005000,
    });

    expect(fields['bytesRead'], 1800);
    expect(fields['unroutablePackets'], 2);
    expect(fields['nonIpv4Packets'], 1);
    expect(fields['relayPacketsWritten'], 6);
    expect(fields['lastRoutedAtMs'], 1780000000000);
  });

  test('mobile control dispatch uses embedded service before old native path',
      () async {
    const channel = MethodChannel('dev.slan/client_core_v2');
    final calls = <String>[];
    final embeddedMethods = <String>[];
    TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
        .setMockMethodCallHandler(channel, (call) async {
      calls.add(call.method);
      if (call.method == 'embeddedServiceRequest') {
        final request =
            jsonDecode(call.arguments as String) as Map<String, Object?>;
        final method = request['method'] as String;
        embeddedMethods.add(method);
        if (method == 'dispatch') {
          final args = request['args'] as Map<Object?, Object?>;
          expect(args['type'], ClientCommandType.loginWithPassword.name);
          return {
            'signedIn': true,
            'userLabel': 'ios-user@example.com',
            'deviceId': 'ios-device-1',
            'networkEnabled': false,
            'syncing': false,
            'switchEnabled': true,
          };
        }
        if (method == 'localEnsureDevice') {
          return {
            'registered': true,
            'deviceId': 'ios-device-1',
            'mqttCredentialReady': true,
          };
        }
        if (method == 'localConnectControlMqtt') {
          return {
            'connected': true,
            'deviceId': 'ios-device-1',
            'downstreamTopic': 'slan/devices/ios-device-1/control/down',
          };
        }
        fail('unexpected embedded method $method');
      }
      if (call.method == 'dispatch') {
        fail('deprecated native dispatch should not be used before embedded');
      }
      return <String, Object?>{};
    });
    addTearDown(() {
      TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
          .setMockMethodCallHandler(channel, null);
    });

    final bridge = MethodChannelClientCoreBridge(
      localServiceHost: await _unusedLoopbackHost(),
      useMobileControlPlane: true,
    );

    await bridge.dispatch(const ClientCommand(
      ClientCommandType.loginWithPassword,
      {
        'email': 'ios-user@example.com',
        'password': 'secret',
      },
    ));

    expect(calls, [
      'embeddedServiceRequest',
      'embeddedServiceRequest',
      'embeddedServiceRequest',
    ]);
    expect(embeddedMethods, [
      'dispatch',
      'localEnsureDevice',
      'localConnectControlMqtt',
    ]);
    expect(bridge.state.value.signedIn, isTrue);
    expect(bridge.state.value.userLabel, 'ios-user@example.com');
    expect(bridge.state.value.deviceId, 'ios-device-1');
  });

  test('mobile start uses embedded service before native state', () async {
    const channel = MethodChannel('dev.slan/client_core_v2');
    final calls = <String>[];
    final embeddedMethods = <String>[];
    TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
        .setMockMethodCallHandler(channel, (call) async {
      calls.add(call.method);
      if (call.method == 'embeddedServiceRequest') {
        final request =
            jsonDecode(call.arguments as String) as Map<String, Object?>;
        final method = request['method'] as String;
        embeddedMethods.add(method);
        if (method == 'start') {
          return {
            'signedIn': true,
            'userLabel': 'embedded-user@example.com',
            'deviceId': 'embedded-device-1',
            'networkEnabled': false,
            'syncing': false,
            'switchEnabled': true,
          };
        }
        if (method == 'localEnsureDevice') {
          return {
            'registered': true,
            'deviceId': 'embedded-device-1',
            'mqttCredentialReady': true,
          };
        }
        if (method == 'localConnectControlMqtt') {
          return {
            'connected': true,
            'deviceId': 'embedded-device-1',
            'downstreamTopic': 'slan/devices/embedded-device-1/control/down',
          };
        }
        if (method == 'localBusinessEventWatch') {
          return {
            'revision': 0,
            'businessType': ClientBusinessEventType.stateChanged,
            'businessData': <String, Object?>{},
            'snapshot': {
              'signedIn': true,
              'networkEnabled': false,
              'syncing': false,
              'switchEnabled': true,
            },
          };
        }
        fail('unexpected embedded method $method');
      }
      if (call.method == 'start' || call.method == 'localState') {
        fail('mobile start should not use deprecated native state');
      }
      return <String, Object?>{};
    });
    addTearDown(() {
      TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
          .setMockMethodCallHandler(channel, null);
    });

    final bridge = MethodChannelClientCoreBridge(
      localServiceHost: await _unusedLoopbackHost(),
      useMobileControlPlane: true,
    );
    await bridge.start();

    expect(calls.first, 'mobileServerBaseUrl');
    expect(calls.skip(1).first, 'embeddedServiceRequest');
    expect(calls, isNot(contains('start')));
    await _waitFor(
      () => embeddedMethods.contains('localConnectControlMqtt'),
      reason: 'mobile start should register device before connecting mqtt',
    );
    expect(embeddedMethods.take(3), [
      'start',
      'localEnsureDevice',
      'localConnectControlMqtt',
    ]);
    expect(bridge.state.value.signedIn, isTrue);
    expect(bridge.state.value.deviceId, 'embedded-device-1');
  });

  test(
      'mobile control dispatch does not fall back to old native on embedded error',
      () async {
    const channel = MethodChannel('dev.slan/client_core_v2');
    final calls = <String>[];
    TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
        .setMockMethodCallHandler(channel, (call) async {
      calls.add(call.method);
      if (call.method == 'embeddedServiceRequest') {
        return {'error': 'embedded unavailable'};
      }
      if (call.method == 'dispatch') {
        fail(
            'deprecated native dispatch should not be used after embedded error');
      }
      return <String, Object?>{};
    });
    addTearDown(() {
      TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
          .setMockMethodCallHandler(channel, null);
    });

    final bridge = MethodChannelClientCoreBridge(
      localServiceHost: await _unusedLoopbackHost(),
      useMobileControlPlane: true,
    );

    await expectLater(
      bridge.dispatch(const ClientCommand(
        ClientCommandType.loginWithPassword,
        {
          'email': 'ios-user@example.com',
          'password': 'secret',
        },
      )),
      throwsA(isA<PlatformException>().having(
        (error) => error.code,
        'code',
        'embedded_service_error',
      )),
    );
    expect(calls, ['embeddedServiceRequest']);
  });

  test('ios network switch uses embedded config and packet tunnel plugin',
      () async {
    const channel = MethodChannel('dev.slan/client_core_v2');
    final calls = <String>[];
    TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
        .setMockMethodCallHandler(channel, (call) async {
      calls.add(call.method);
      if (call.method == 'embeddedServiceRequest') {
        final request =
            jsonDecode(call.arguments as String) as Map<String, Object?>;
        expect(request['method'], 'localPlatformNetworkConfig');
        return {
          'sessionName': 'SLAN',
          'virtualIp': '100.64.0.44',
          'prefixLen': 32,
          'dnsServers': ['100.64.0.1'],
          'routes': [
            {'destination': '100.64.0.0/10'}
          ],
          'mtu': 1280,
        };
      }
      if (call.method == 'iosStartPacketTunnel') {
        final config = (call.arguments as Map).cast<String, Object?>();
        expect(config['virtualIp'], '100.64.0.44');
        return {
          'signedIn': true,
          'networkEnabled': true,
          'virtualIp': '100.64.0.44',
          'syncing': false,
          'switchEnabled': true,
          'notice': 'networkEnabled',
        };
      }
      if (call.method == 'dispatch') {
        fail('iOS switch should not call deprecated native dispatch');
      }
      return <String, Object?>{};
    });
    addTearDown(() {
      TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
          .setMockMethodCallHandler(channel, null);
    });

    final bridge = MethodChannelClientCoreBridge(
      localServiceHost: await _unusedLoopbackHost(),
      runtimePlatform: ClientBridgeRuntimePlatform.ios,
    );

    await bridge.dispatch(
      const ClientCommand(ClientCommandType.enableNetwork),
    );

    await _waitFor(
      () => bridge.state.value.virtualIp == '100.64.0.44',
      reason: 'iOS switch should start packet tunnel with embedded config',
    );
    expect(calls.take(2), ['embeddedServiceRequest', 'iosStartPacketTunnel']);
    expect(calls, isNot(contains('dispatch')));
    expect(bridge.state.value.networkEnabled, isTrue);
    expect(bridge.state.value.switchEnabled, isTrue);
  });

  test('mobile business event watch uses embedded service before native queue',
      () async {
    const channel = MethodChannel('dev.slan/client_core_v2');
    final calls = <String>[];
    final embeddedMethods = <String>[];
    TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
        .setMockMethodCallHandler(channel, (call) async {
      calls.add(call.method);
      if (call.method == 'embeddedServiceRequest') {
        final request =
            jsonDecode(call.arguments as String) as Map<String, Object?>;
        final method = request['method'] as String;
        embeddedMethods.add(method);
        if (method == 'start') {
          return {
            'signedIn': true,
            'networkEnabled': false,
            'syncing': false,
            'switchEnabled': true,
          };
        }
        if (method == 'localEnsureDevice') {
          return {
            'registered': true,
            'deviceId': 'embedded-device-1',
            'mqttCredentialReady': true,
          };
        }
        if (method == 'localConnectControlMqtt') {
          return {
            'connected': true,
            'deviceId': 'embedded-device-1',
            'downstreamTopic': 'slan/devices/embedded-device-1/control/down',
          };
        }
        expect(method, 'localBusinessEventWatch');
        return {
          'revision': 0,
          'businessType': ClientBusinessEventType.stateChanged,
          'businessData': <String, Object?>{},
          'snapshot': {
            'signedIn': true,
            'networkEnabled': false,
            'syncing': false,
            'switchEnabled': true,
          },
        };
      }
      if (call.method == 'localBusinessEventWatch') {
        fail('deprecated native business event queue should not be used');
      }
      return <String, Object?>{};
    });
    addTearDown(() {
      TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
          .setMockMethodCallHandler(channel, null);
    });

    final bridge = MethodChannelClientCoreBridge(
      localServiceHost: await _unusedLoopbackHost(),
      useMobileControlPlane: true,
    );
    await bridge.start();

    await _waitFor(
      () => calls.contains('embeddedServiceRequest'),
      reason: 'mobile business event watch should fall back to embedded',
    );
    await _waitFor(
      () => embeddedMethods.contains('localConnectControlMqtt'),
      reason: 'mobile start should connect mqtt after device registration',
    );
    expect(embeddedMethods.take(3), [
      'start',
      'localEnsureDevice',
      'localConnectControlMqtt',
    ]);
    expect(calls, isNot(contains('localBusinessEventWatch')));
  });

  test('mobile business event applies latest client message fields', () async {
    const channel = MethodChannel('dev.slan/client_core_v2');
    final embeddedMethods = <String>[];
    TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
        .setMockMethodCallHandler(channel, (call) async {
      if (call.method == 'embeddedServiceRequest') {
        final request =
            jsonDecode(call.arguments as String) as Map<String, Object?>;
        final method = request['method'] as String;
        embeddedMethods.add(method);
        if (method == 'start') {
          return {
            'signedIn': true,
            'networkEnabled': false,
            'syncing': false,
            'switchEnabled': true,
          };
        }
        if (method == 'localEnsureDevice') {
          return {
            'registered': true,
            'deviceId': 'embedded-device-1',
            'mqttCredentialReady': true,
          };
        }
        if (method == 'localConnectControlMqtt') {
          return {
            'connected': true,
            'deviceId': 'embedded-device-1',
            'downstreamTopic': 'slan/devices/embedded-device-1/control/down',
          };
        }
        if (method == 'localBusinessEventWatch') {
          return {
            'revision': 1,
            'businessType': ClientBusinessEventType.controlSyncChanged,
            'businessData': {
              'signedIn': true,
              'networkEnabled': false,
              'syncing': false,
              'switchEnabled': true,
              'lastClientMessageId': 'msg-1',
              'lastClientMessageFromDeviceId': 'ios-peer',
              'lastClientMessageBody': 'hello',
            },
            'snapshot': {
              'signedIn': true,
              'networkEnabled': false,
              'syncing': false,
              'switchEnabled': true,
            },
          };
        }
        fail('unexpected embedded method $method');
      }
      return <String, Object?>{};
    });
    addTearDown(() {
      TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
          .setMockMethodCallHandler(channel, null);
    });

    final bridge = MethodChannelClientCoreBridge(
      localServiceHost: await _unusedLoopbackHost(),
      useMobileControlPlane: true,
    );
    await bridge.start();

    await _waitFor(
      () => bridge.state.value.lastClientMessageBody == 'hello',
      reason: 'business event should update latest client message fields',
    );
    expect(bridge.state.value.lastClientMessageId, 'msg-1');
    expect(bridge.state.value.lastClientMessageFromDeviceId, 'ios-peer');
    expect(embeddedMethods, contains('localBusinessEventWatch'));
  });

  test('mobile send client message uses embedded service request', () async {
    const channel = MethodChannel('dev.slan/client_core_v2');
    final embeddedMethods = <String>[];
    TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
        .setMockMethodCallHandler(channel, (call) async {
      if (call.method == 'embeddedServiceRequest') {
        final request =
            jsonDecode(call.arguments as String) as Map<String, Object?>;
        embeddedMethods.add(request['method'] as String);
        expect(request['method'], 'localSendClientMessage');
        final args = request['args'] as Map<Object?, Object?>;
        expect(args['targetDeviceId'], 'ios-target');
        expect(args['body'], 'hello');
        return {
          'messageId': 'client-msg-1',
          'networkId': 'net-a',
          'fromDeviceId': 'ios-source',
          'targetDeviceId': 'ios-target',
          'sentAtMs': 1780000000000,
        };
      }
      if (call.method == 'dispatch') {
        fail('deprecated native dispatch should not send client messages');
      }
      return <String, Object?>{};
    });
    addTearDown(() {
      TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
          .setMockMethodCallHandler(channel, null);
    });

    final bridge = MethodChannelClientCoreBridge(
      localServiceHost: await _unusedLoopbackHost(),
      useMobileControlPlane: true,
    );

    await bridge.dispatch(const ClientCommand(
      ClientCommandType.sendClientMessage,
      {
        'targetDeviceId': 'ios-target',
        'body': 'hello',
      },
    ));

    expect(embeddedMethods, ['localSendClientMessage']);
  });

  test('enable switch updates asynchronously after service result', () async {
    final service = await _FakeClientService.start([
      _ServiceReply(
        delay: const Duration(milliseconds: 150),
        expectedMethod: 'localBusinessEventWatch',
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
        expectedMethod: 'localState',
        body: {
          'signedIn': true,
          'networkEnabled': true,
          'syncing': false,
          'switchEnabled': true,
          'virtualIp': '100.64.0.10',
        },
      ),
      _ServiceReply(
        expectedMethod: 'localNetworkActivate',
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
        expectedMethod: 'localBusinessEventWatch',
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
        expectedMethod: 'localState',
        body: {
          'signedIn': true,
          'networkEnabled': true,
          'syncing': false,
          'switchEnabled': true,
          'virtualIp': '100.64.0.10',
        },
      ),
      _ServiceReply(
        expectedMethod: 'localNetworkActivate',
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
    expect(service.seenMethods, isNot(contains('localNetworkDeactivate')));
    expect(
      service.seenMethods.where((method) => method == 'localNetworkActivate'),
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
        expectedMethod: 'localBusinessEventWatch',
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
        expectedMethod: 'localState',
        body: {
          'signedIn': true,
          'networkEnabled': false,
          'syncing': false,
          'switchEnabled': true,
          'virtualIp': null,
        },
      ),
      _ServiceReply(
        expectedMethod: 'localNetworkDeactivate',
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
        expectedMethod: 'localBusinessEventWatch',
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
        expectedMethod: 'localState',
        body: {
          'signedIn': true,
          'networkEnabled': false,
          'syncing': false,
          'switchEnabled': true,
          'error': 'device unavailable: current device has been disabled',
        },
      ),
      _ServiceReply(
        expectedMethod: 'localNetworkActivate',
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
        expectedMethod: 'localNetworkActivate',
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
    expect(service.seenMethods, isNot(contains('localBusinessEventWatch')));
  });

  test('network switch failed keeps event error after state query', () async {
    final service = await _FakeClientService.start([
      _ServiceReply(
        delay: const Duration(milliseconds: 80),
        expectedMethod: 'localBusinessEventWatch',
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
        expectedMethod: 'localState',
        body: {
          'signedIn': true,
          'networkEnabled': false,
          'syncing': false,
          'switchEnabled': true,
        },
      ),
      _ServiceReply(
        expectedMethod: 'localNetworkActivate',
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
        expectedMethod: 'localNetworkActivate',
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
        expectedMethod: 'localBusinessEventWatch',
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
        expectedMethod: 'localState',
        body: {
          'signedIn': true,
          'networkEnabled': true,
          'syncing': false,
          'switchEnabled': true,
          'virtualIp': '100.64.0.10',
        },
      ),
      _ServiceReply(
        expectedMethod: 'localNetworkActivate',
        body: {
          'signedIn': true,
          'networkEnabled': true,
          'syncing': false,
          'switchEnabled': true,
          'virtualIp': '100.64.0.10',
        },
      ),
      const _ServiceReply(
        expectedMethod: 'localNetworkDeactivate',
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
        expectedMethod: 'localBusinessEventWatch',
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
        expectedMethod: 'localState',
        body: {
          'signedIn': true,
          'networkEnabled': true,
          'syncing': false,
          'switchEnabled': true,
          'virtualIp': '100.64.0.10',
        },
      ),
      _ServiceReply(
        expectedMethod: 'localNetworkActivate',
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
    expect(
        service.seenMethods,
        containsAll(
            ['localNetworkActivate', 'localBusinessEventWatch', 'localState']));
  });

  test('logout clears local service session before returning', () async {
    final service = await _FakeClientService.start([
      _ServiceReply(
        expectedMethod: 'localLogout',
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

    expect(service.seenMethods, contains('localLogout'));
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

Future<String> _unusedLoopbackHost() async {
  final server = await ServerSocket.bind(InternetAddress.loopbackIPv4, 0);
  final port = server.port;
  await server.close();
  return '127.0.0.1:$port';
}
