import 'dart:async';
import 'dart:convert';
import 'dart:io';

import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:slan_client_v2/bridge/client_commands.dart';
import 'package:slan_client_v2/bridge/client_core_bridge.dart';
import 'package:slan_client_v2/bridge/client_core_bridge_support.dart';
import 'package:slan_client_v2/bridge/client_view_state.dart';
import 'package:slan_client_v2/bridge/control_transport_status.dart';

Map<String, Object?> _nativePlatformNetworkConfigPayload({
  String virtualIp = '10.0.0.44',
  List<String> resolverServers = const ['10.0.0.1'],
}) {
  return {
    'sessionName': 'SLAN',
    'virtualIp': virtualIp,
    'prefixLen': 32,
    ..._nativePlatformResolverTransportPayload(resolverServers),
    'routes': [
      {'destination': '10.0.0.0/8'}
    ],
    'mtu': 1280,
  };
}

Map<String, Object?> _nativePlatformResolverTransportPayload(
  List<String> servers,
) {
  return {
    'resolver': {
      'servers': servers,
    },
  };
}

void main() {
  test('control transport status preserves all network subscriptions', () {
    final status = ControlTransportStatus.fromJson({
      'mqttCredentialReady': true,
      'controlSessionReady': true,
      'ready': true,
      'missing': <String>[],
      'mqttConnected': true,
      'mqttNetworkEventTopics': <String>[
        'slan/networks/network-a/broadcast',
        'slan/networks/network-b/broadcast',
      ],
      'mqttNetworkEventSubscribed': true,
    });

    expect(status.mqttNetworkEventTopics, [
      'slan/networks/network-a/broadcast',
      'slan/networks/network-b/broadcast',
    ]);
    expect(status.mqttNetworkEventSubscribed, isTrue);
  });

  test('business event snapshot wins over stale event state', () {
    final current = ClientViewState.fromJson({
      'activated': true,
      'networkEnabled': true,
      'virtualIp': '10.0.0.1',
      'syncing': false,
      'switchEnabled': true,
    });
    final staleData = ClientViewState.fromJson({
      'activated': true,
      'networkEnabled': false,
      'syncing': false,
      'switchEnabled': true,
      'notice': 'networkDisabled',
    });
    final snapshot = ClientViewState.fromJson({
      'activated': true,
      'networkEnabled': true,
      'virtualIp': '10.0.0.1',
      'syncing': false,
      'switchEnabled': true,
      'notice': 'networkEnabled',
    });

    final next = reduceBusinessEvent(
      current,
      {
        'businessType': ClientBusinessEventType.networkRuntimeChanged,
        'businessData': {
          'networkEnabled': false,
          'notice': 'networkDisabled',
        },
        'snapshot': {
          'networkEnabled': true,
          'notice': 'networkEnabled',
        },
      },
      dataState: staleData,
      snapshotState: snapshot,
    );

    expect(next?.networkEnabled, isTrue);
    expect(next?.virtualIp, '10.0.0.1');
  });

  test('runtime event does not unlock an in-flight network toggle', () {
    final pending = ClientViewState.fromJson({
      'activated': true,
      'networkEnabled': true,
      'syncing': true,
      'syncReason': 'enableNetwork',
      'switchEnabled': false,
    });
    final runtime = ClientViewState.fromJson({
      'activated': true,
      'networkEnabled': false,
      'syncing': false,
      'switchEnabled': true,
    });

    final next = reduceBusinessEvent(
      pending,
      {'businessType': ClientBusinessEventType.networkRuntimeChanged},
      dataState: runtime,
      snapshotState: runtime,
      networkToggleInFlight: true,
    );

    expect(next?.networkEnabled, isTrue);
    expect(next?.syncing, isTrue);
    expect(next?.syncReason, 'enableNetwork');
    expect(next?.switchEnabled, isFalse);
  });

  test('only terminal network events settle a toggle', () {
    expect(
      businessEventSettlesNetworkToggle(
        ClientBusinessEventType.networkRuntimeChanged,
      ),
      isFalse,
    );
    expect(
      businessEventSettlesNetworkToggle(
        ClientBusinessEventType.networkSwitchFinished,
      ),
      isTrue,
    );
    expect(
      businessEventSettlesNetworkToggle(
        ClientBusinessEventType.networkSwitchFailed,
      ),
      isTrue,
    );
  });

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
      'embeddedServicePendingLimit': 129,
      'embeddedServicePendingCount': 3,
      'embeddedServiceQueueDepth': 2,
      'embeddedServiceActiveCount': 1,
      'embeddedServiceCompletedTotal': 40,
      'embeddedServiceRejectedTotal': 2,
      'embeddedWatchPendingLimit': 3,
      'embeddedWatchPendingCount': 1,
      'embeddedWatchQueueDepth': 0,
      'embeddedWatchActiveCount': 1,
      'embeddedWatchCompletedTotal': 12,
      'embeddedWatchRejectedTotal': 0,
    });

    expect(fields['requestedRelaySessionCount'], 2);
    expect(fields['attachedRelaySessionCount'], 1);
    expect(fields['relayDetachSent'], 1);
    expect(fields['lastRelayAttachError'], 'relay attach timed out');
    expect(fields['embeddedServicePendingLimit'], 129);
    expect(fields['embeddedServicePendingCount'], 3);
    expect(fields['embeddedServiceQueueDepth'], 2);
    expect(fields['embeddedServiceActiveCount'], 1);
    expect(fields['embeddedServiceCompletedTotal'], 40);
    expect(fields['embeddedServiceRejectedTotal'], 2);
    expect(fields['embeddedWatchPendingLimit'], 3);
    expect(fields['embeddedWatchPendingCount'], 1);
    expect(fields['embeddedWatchQueueDepth'], 0);
    expect(fields['embeddedWatchActiveCount'], 1);
    expect(fields['embeddedWatchCompletedTotal'], 12);
    expect(fields['embeddedWatchRejectedTotal'], 0);
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

  test('mobile platform event updates UI only after rust business event',
      () async {
    const channel = MethodChannel('dev.slan/client_core_v2');
    final ingestSeen = Completer<void>();
    final releaseBusinessEvent = Completer<void>();
    var platformEventSent = false;
    var businessEventSent = false;

    TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
        .setMockMethodCallHandler(channel, (call) async {
      if (call.method == 'androidWatchNetworkEvent') {
        if (platformEventSent) {
          await Future<void>.delayed(const Duration(milliseconds: 20));
          return null;
        }
        platformEventSent = true;
        return {
          'eventType': 'vpnStarted',
          'runtimeState': {
            'adapterPresent': true,
            'networkEnabled': true,
            'virtualIp': '10.0.0.44',
          },
        };
      }
      if (call.method != 'embeddedServiceRequest') {
        return <String, Object?>{};
      }
      final request =
          jsonDecode(call.arguments as String) as Map<String, Object?>;
      final method = request['method'] as String;
      if (method == 'start') {
        return {
          'activated': false,
          'networkEnabled': false,
          'syncing': false,
          'switchEnabled': true,
        };
      }
      if (method == 'ingestPlatformRuntimeState') {
        if (!ingestSeen.isCompleted) {
          ingestSeen.complete();
        }
        return {
          'accepted': true,
          'state': {
            'activated': true,
            'networkEnabled': true,
            'virtualIp': '10.0.0.44',
            'syncing': false,
            'switchEnabled': true,
          },
        };
      }
      if (method == 'localBusinessEventWatch') {
        await releaseBusinessEvent.future;
        if (businessEventSent) {
          return null;
        }
        businessEventSent = true;
        return {
          'revision': 1,
          'businessType': ClientBusinessEventType.networkRuntimeChanged,
          'businessData': {
            'activated': true,
            'networkEnabled': true,
            'virtualIp': '10.0.0.44',
            'syncing': false,
            'switchEnabled': true,
            'messageType': 'platform_runtime_state',
          },
          'snapshot': {
            'activated': true,
            'networkEnabled': true,
            'virtualIp': '10.0.0.44',
            'syncing': false,
            'switchEnabled': true,
          },
        };
      }
      fail('unexpected embedded method $method');
    });
    addTearDown(() {
      if (!releaseBusinessEvent.isCompleted) {
        releaseBusinessEvent.complete();
      }
      TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
          .setMockMethodCallHandler(channel, null);
    });

    final bridge = MethodChannelClientCoreBridge(
      localServiceHost: await _unusedLoopbackHost(),
      runtimePlatform: ClientBridgeRuntimePlatform.android,
      useMobileControlPlane: true,
    );
    _closeBridgeOnTearDown(bridge);
    await bridge.start();
    await ingestSeen.future.timeout(const Duration(seconds: 2));

    expect(bridge.state.value.networkEnabled, isFalse);
    releaseBusinessEvent.complete();
    await _waitFor(
      () => bridge.state.value.networkEnabled,
      reason: 'rust business event should be the only UI state writer',
    );
    expect(bridge.state.value.virtualIp, '10.0.0.44');
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
            'activated': true,
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
              'activated': true,
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
    _closeBridgeOnTearDown(bridge);
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
    expect(bridge.state.value.activated, isTrue);
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
    _closeBridgeOnTearDown(bridge);

    await expectLater(
      bridge.dispatch(const ClientCommand(ClientCommandType.refresh)),
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
        final networkConfigPayload = _nativePlatformNetworkConfigPayload();
        return networkConfigPayload;
      }
      if (call.method == 'iosStartPacketTunnel') {
        final config = (call.arguments as Map).cast<String, Object?>();
        expect(config['virtualIp'], '10.0.0.44');
        return {
          'activated': true,
          'networkEnabled': true,
          'virtualIp': '10.0.0.44',
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
    _closeBridgeOnTearDown(bridge);

    await bridge.dispatch(
      const ClientCommand(ClientCommandType.enableNetwork),
    );

    await _waitFor(
      () => bridge.state.value.virtualIp == '10.0.0.44',
      reason: 'iOS switch should start packet tunnel with embedded config',
    );
    expect(calls.take(2), ['embeddedServiceRequest', 'iosStartPacketTunnel']);
    expect(calls, isNot(contains('dispatch')));
    expect(bridge.state.value.networkEnabled, isTrue);
    expect(bridge.state.value.switchEnabled, isTrue);
  });

  test('ios resume reconnects control and hot reloads packet tunnel', () async {
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
        if (method == 'localPlatformNetworkConfig') {
          return _nativePlatformNetworkConfigPayload();
        }
        return {'accepted': true};
      }
      if (call.method == 'iosStartPacketTunnel' ||
          call.method == 'iosRefreshPacketTunnel' ||
          call.method == 'iosRuntimeState') {
        return {
          'adapterPresent': true,
          'networkEnabled': true,
          'virtualIp': '10.0.0.44',
          'syncing': false,
          'switchEnabled': true,
        };
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
    _closeBridgeOnTearDown(bridge);
    await bridge.dispatch(
      const ClientCommand(ClientCommandType.enableNetwork),
    );
    await _waitFor(
      () => bridge.state.value.networkEnabled,
      reason: 'iOS network should be enabled before resume recovery',
    );
    calls.clear();
    embeddedMethods.clear();

    await bridge.notifyAppResumed();

    expect(embeddedMethods, [
      'localConnectivityChanged',
      'localPlatformNetworkConfig',
    ]);
    expect(
        calls,
        containsAllInOrder([
          'embeddedServiceRequest',
          'embeddedServiceRequest',
          'iosRuntimeState',
          'iosRefreshPacketTunnel',
        ]));
    expect(calls, isNot(contains('iosStartPacketTunnel')));
  });

  test('android resume reconnects control and rebuilds vpn data plane',
      () async {
    const channel = MethodChannel('dev.slan/client_core_v2');
    final calls = <String>[];
    TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
        .setMockMethodCallHandler(channel, (call) async {
      calls.add(call.method);
      if (call.method == 'embeddedServiceRequest') {
        final request =
            jsonDecode(call.arguments as String) as Map<String, Object?>;
        if (request['method'] == 'localPlatformNetworkConfig') {
          return _nativePlatformNetworkConfigPayload();
        }
        if (request['method'] == 'dispatch') {
          return {
            'activated': true,
            'networkEnabled': false,
            'syncing': false,
            'switchEnabled': true,
          };
        }
        return {'accepted': true};
      }
      if (call.method == 'androidStartVpn' ||
          call.method == 'androidRuntimeState') {
        return {
          'adapterPresent': true,
          'networkEnabled': true,
          'virtualIp': '10.0.0.44',
          'syncing': false,
          'switchEnabled': true,
        };
      }
      if (call.method == 'androidVpnPermissionState') {
        return 'granted';
      }
      return <String, Object?>{};
    });
    addTearDown(() {
      TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
          .setMockMethodCallHandler(channel, null);
    });

    final bridge = MethodChannelClientCoreBridge(
      localServiceHost: await _unusedLoopbackHost(),
      runtimePlatform: ClientBridgeRuntimePlatform.android,
    );
    _closeBridgeOnTearDown(bridge);
    await bridge.dispatch(const ClientCommand(ClientCommandType.refresh));
    await bridge.dispatch(
      const ClientCommand(ClientCommandType.enableNetwork),
    );
    await _waitFor(
      () => bridge.state.value.networkEnabled,
      reason: 'Android network should be enabled before resume recovery',
    );
    calls.clear();

    await bridge.notifyAppResumed();

    expect(
        calls,
        containsAllInOrder([
          'embeddedServiceRequest',
          'embeddedServiceRequest',
          'androidRuntimeState',
          'androidStartVpn',
        ]));
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
            'activated': true,
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
            'activated': true,
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
    _closeBridgeOnTearDown(bridge);
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

  test('mobile business event keeps control sync handling on embedded watch',
      () async {
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
            'activated': true,
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
              'activated': true,
              'networkEnabled': false,
              'syncing': false,
              'switchEnabled': true,
              'messageType': 'network_event',
              'eventType': 'member_online',
            },
            'snapshot': {
              'activated': true,
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
    _closeBridgeOnTearDown(bridge);
    await bridge.start();

    await _waitFor(
      () => bridge.state.value.lastControlSyncMessageType == 'network_event',
      reason: 'business event should update control sync metadata',
    );
    expect(bridge.state.value.lastControlSyncMessageType, 'network_event');
    expect(bridge.state.value.lastControlSyncReconfigureRequired, false);
    expect(embeddedMethods, contains('localBusinessEventWatch'));
  });

  test('mobile control sync business event uses embedded payload metadata',
      () async {
    const channel = MethodChannel('dev.slan/client_core_v2');
    final embeddedMethods = <String>[];
    TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
        .setMockMethodCallHandler(channel, (call) async {
      if (call.method != 'embeddedServiceRequest') {
        return <String, Object?>{};
      }
      final request =
          jsonDecode(call.arguments as String) as Map<String, Object?>;
      final method = request['method'] as String;
      embeddedMethods.add(method);
      if (method == 'start') {
        return {
          'activated': true,
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
            'activated': true,
            'networkEnabled': false,
            'syncing': false,
            'switchEnabled': true,
            'messageType': 'network_event',
            'eventType': 'member_online',
          },
          'snapshot': {
            'activated': true,
            'networkEnabled': false,
            'syncing': false,
            'switchEnabled': true,
          },
        };
      }
      fail('unexpected embedded method $method');
    });
    addTearDown(() {
      TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
          .setMockMethodCallHandler(channel, null);
    });

    final bridge = MethodChannelClientCoreBridge(
      localServiceHost: await _unusedLoopbackHost(),
      useMobileControlPlane: true,
    );
    _closeBridgeOnTearDown(bridge);
    await bridge.start();

    await _waitFor(
      () => bridge.state.value.lastControlSyncMessageType == 'network_event',
      reason: 'control sync event should use metadata from payload',
    );
    expect(
      bridge.state.value.lastControlSyncMessageType,
      'network_event',
    );
    expect(bridge.state.value.lastControlSyncReconfigureRequired, false);
    expect(embeddedMethods, isNot(contains('localState')));
  });

  test('mobile control sync business event stores control sync metadata',
      () async {
    const channel = MethodChannel('dev.slan/client_core_v2');
    TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
        .setMockMethodCallHandler(channel, (call) async {
      if (call.method != 'embeddedServiceRequest') {
        return <String, Object?>{};
      }
      final request =
          jsonDecode(call.arguments as String) as Map<String, Object?>;
      final method = request['method'] as String;
      if (method == 'start') {
        return {
          'activated': true,
          'networkEnabled': true,
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
            'activated': true,
            'networkEnabled': true,
            'syncing': false,
            'switchEnabled': true,
            'messageType': 'network_event',
            'eventType': 'network_config_changed',
            'reconfigureRequired': true,
          },
          'snapshot': {
            'activated': true,
            'networkEnabled': true,
            'syncing': false,
            'switchEnabled': true,
          },
        };
      }
      if (method == 'localState') {
        return {
          'activated': true,
          'networkEnabled': true,
          'syncing': false,
          'switchEnabled': true,
        };
      }
      fail('unexpected embedded method $method');
    });
    addTearDown(() {
      TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
          .setMockMethodCallHandler(channel, null);
    });

    final bridge = MethodChannelClientCoreBridge(
      localServiceHost: await _unusedLoopbackHost(),
      useMobileControlPlane: true,
    );
    _closeBridgeOnTearDown(bridge);
    await bridge.start();

    await _waitFor(
      () => bridge.state.value.lastControlSyncMessageType == 'network_event',
      reason: 'control sync metadata should be stored in bridge state',
    );
    expect(bridge.state.value.lastControlSyncReconfigureRequired, true);
  });

  test(
      'mobile control sync uses rust reconfigure flag for network config change',
      () async {
    const channel = MethodChannel('dev.slan/client_core_v2');
    final calls = <String>[];
    final embeddedMethods = <String>[];
    var emittedBusinessEvent = false;
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
            'activated': true,
            'networkEnabled': true,
            'syncing': false,
            'switchEnabled': true,
            'virtualIp': '10.0.0.44',
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
        if (method == 'localPlatformNetworkConfig') {
          final networkConfigPayload = _nativePlatformNetworkConfigPayload();
          return networkConfigPayload;
        }
        if (method == 'localBusinessEventWatch') {
          if (emittedBusinessEvent) {
            return {
              'revision': 1,
              'businessType': ClientBusinessEventType.stateChanged,
              'businessData': <String, Object?>{},
              'snapshot': {
                'activated': true,
                'networkEnabled': true,
                'syncing': false,
                'switchEnabled': true,
                'virtualIp': '10.0.0.44',
              },
            };
          }
          emittedBusinessEvent = true;
          return {
            'revision': 1,
            'businessType': ClientBusinessEventType.controlSyncChanged,
            'businessData': {
              'activated': true,
              'networkEnabled': true,
              'syncing': false,
              'switchEnabled': true,
              'virtualIp': '10.0.0.44',
              'messageType': 'network_event',
              'eventType': 'network_config_changed',
              'reconfigureRequired': true,
            },
            'snapshot': {
              'activated': true,
              'networkEnabled': true,
              'syncing': false,
              'switchEnabled': true,
              'virtualIp': '10.0.0.44',
            },
          };
        }
        if (method == 'localState') {
          return {
            'activated': true,
            'networkEnabled': true,
            'syncing': false,
            'switchEnabled': true,
            'virtualIp': '10.0.0.44',
          };
        }
        fail('unexpected embedded method $method');
      }
      if (call.method == 'androidStartVpn') {
        final config = (call.arguments as Map).cast<String, Object?>();
        expect(config['virtualIp'], '10.0.0.44');
        return {
          'activated': true,
          'networkEnabled': true,
          'virtualIp': '10.0.0.44',
          'syncing': false,
          'switchEnabled': true,
        };
      }
      if (call.method == 'androidWatchNetworkEvent') {
        await Future<void>.delayed(const Duration(milliseconds: 50));
        return null;
      }
      return <String, Object?>{};
    });
    addTearDown(() {
      TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
          .setMockMethodCallHandler(channel, null);
    });

    final bridge = MethodChannelClientCoreBridge(
      localServiceHost: await _unusedLoopbackHost(),
      runtimePlatform: ClientBridgeRuntimePlatform.android,
      useMobileControlPlane: true,
    );
    _closeBridgeOnTearDown(bridge);
    await bridge.start();

    await _waitFor(
      () => calls.contains('androidStartVpn'),
      reason:
          'network config change with rust reconfigure flag should refresh mobile config',
    );
    expect(embeddedMethods, contains('localPlatformNetworkConfig'));
    await bridge.close();
  });

  test('control sync event key changes with versioned network event payloads',
      () {
    final first = controlSyncEventKey(
      {
        'revision': 1,
      },
      {
        'messageType': 'network_event',
        'eventType': 'network_config_changed',
        'networkId': 'net-1',
        'configVersion': 1,
        'eventId': 'evt-1',
      },
    );
    final second = controlSyncEventKey(
      {
        'revision': 2,
      },
      {
        'messageType': 'network_event',
        'eventType': 'network_config_changed',
        'networkId': 'net-1',
        'configVersion': 2,
        'eventId': 'evt-2',
      },
    );

    expect(first, isNot(second));
    expect(first, contains('evt-1'));
    expect(second, contains('evt-2'));
  });

  test('mobile local send client message request uses embedded service request',
      () async {
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
        expect(args.containsKey('deviceId'), isFalse);
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
    _closeBridgeOnTearDown(bridge);

    await bridge.requestLocalApi(
      'localSendClientMessage',
      const {
        'targetDeviceId': 'ios-target',
        'body': 'hello',
      },
    );

    expect(embeddedMethods, ['localSendClientMessage']);
  });

  test('mobile local control status request does not inject device id',
      () async {
    const channel = MethodChannel('dev.slan/client_core_v2');
    TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
        .setMockMethodCallHandler(channel, (call) async {
      if (call.method == 'embeddedServiceRequest') {
        final request =
            jsonDecode(call.arguments as String) as Map<String, Object?>;
        expect(request['method'], 'localControlStatus');
        final args = request['args'] as Map<Object?, Object?>;
        expect(args.containsKey('deviceId'), isFalse);
        expect(args.containsKey('deviceIdOverride'), isFalse);
        return <String, Object?>{
          'ready': true,
          'mqttConnected': true,
        };
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
    _closeBridgeOnTearDown(bridge);

    await bridge.localControlStatus();
  });

  test('business event replay gap restores snapshot and advances cursor',
      () async {
    final recoveredState = <String, Object?>{
      'activated': true,
      'networkEnabled': true,
      'syncing': false,
      'switchEnabled': true,
      'virtualIp': '10.0.0.20',
    };
    final service = await _FakeClientService.start([
      _ServiceReply(
        expectedMethod: 'localBusinessEventWatch',
        body: {
          'revision': 3,
          'streamId': 'stream-a',
          'oldestAvailableRevision': 3,
          'latestRevision': 9,
          'replayGap': true,
          'businessType': ClientBusinessEventType.stateChanged,
          'businessData': <String, Object?>{},
          'snapshot': recoveredState,
        },
      ),
      _ServiceReply(
        expectedMethod: 'localBusinessEventWatch',
        body: {
          'revision': 8,
          'streamId': 'stream-a',
          'oldestAvailableRevision': 3,
          'latestRevision': 9,
          'replayGap': false,
          'businessType': ClientBusinessEventType.stateChanged,
          'businessData': <String, Object?>{},
          'snapshot': recoveredState,
        },
      ),
      _ServiceReply(
        expectedMethod: 'localBusinessEventWatch',
        body: {
          'revision': 9,
          'streamId': 'stream-a',
          'oldestAvailableRevision': 3,
          'latestRevision': 9,
          'replayGap': false,
          'businessType': ClientBusinessEventType.stateChanged,
          'businessData': <String, Object?>{},
          'snapshot': recoveredState,
        },
      ),
    ]);
    addTearDown(service.close);

    final bridge =
        MethodChannelClientCoreBridge(localServiceHost: service.host);
    _closeBridgeOnTearDown(bridge);
    await bridge.start();

    await _waitFor(
      () =>
          bridge.state.value.virtualIp == '10.0.0.20' &&
          service.seenRequests
                  .where((request) =>
                      request['method'] == 'localBusinessEventWatch')
                  .length >=
              3,
      reason: 'replay gap should restore snapshot and issue next watch',
    );
    final watchRequests = service.seenRequests
        .where((request) => request['method'] == 'localBusinessEventWatch')
        .toList();
    final secondArgs =
        (watchRequests[1]['args'] as Map).cast<String, Object?>();
    final thirdArgs = (watchRequests[2]['args'] as Map).cast<String, Object?>();
    expect(secondArgs['lastRevision'], 9);
    expect(secondArgs['streamId'], 'stream-a');
    expect(thirdArgs['lastRevision'], 9);
    expect(thirdArgs['streamId'], 'stream-a');
  });

  test('first business event watch follows latest service revision', () async {
    final service = await _FakeClientService.start([
      const _ServiceReply(
        expectedMethod: 'localBusinessEventWatch',
        body: {
          'revision': 7,
          'streamId': 'stream-a',
          'oldestAvailableRevision': 1,
          'latestRevision': 7,
          'replayGap': false,
          'businessType': ClientBusinessEventType.stateChanged,
          'businessData': <String, Object?>{},
          'snapshot': {
            'activated': true,
            'networkEnabled': true,
            'syncing': false,
            'switchEnabled': true,
            'virtualIp': '10.0.0.1',
          },
        },
      ),
    ]);
    addTearDown(service.close);

    final bridge =
        MethodChannelClientCoreBridge(localServiceHost: service.host);
    _closeBridgeOnTearDown(bridge);
    await bridge.start();

    await _waitFor(
      () => service.seenRequests.any(
        (request) => request['method'] == 'localBusinessEventWatch',
      ),
      reason: 'bridge should start the business event watch',
    );
    final request = service.seenRequests.firstWhere(
      (request) => request['method'] == 'localBusinessEventWatch',
    );
    final args = (request['args'] as Map).cast<String, Object?>();
    expect(args['lastRevision'], 0);
    expect(args['followLatest'], isTrue);
  });

  test('business event stream reset recovers after rust service restart',
      () async {
    Map<String, Object?> state(String virtualIp) => {
          'activated': true,
          'networkEnabled': true,
          'syncing': false,
          'switchEnabled': true,
          'virtualIp': virtualIp,
        };
    final service = await _FakeClientService.start([
      _ServiceReply(
        expectedMethod: 'localBusinessEventWatch',
        body: {
          'revision': 1,
          'streamId': 'stream-a',
          'streamReset': false,
          'oldestAvailableRevision': 1,
          'latestRevision': 1,
          'replayGap': false,
          'businessType': ClientBusinessEventType.stateChanged,
          'businessData': state('10.0.0.20'),
          'snapshot': state('10.0.0.20'),
        },
      ),
      _ServiceReply(
        expectedMethod: 'localBusinessEventWatch',
        body: {
          'revision': 0,
          'streamId': 'stream-b',
          'streamReset': true,
          'oldestAvailableRevision': null,
          'latestRevision': 0,
          'replayGap': false,
          'businessType': ClientBusinessEventType.stateChanged,
          'businessData': <String, Object?>{},
          'snapshot': state('10.0.0.21'),
        },
      ),
      _ServiceReply(
        expectedMethod: 'localBusinessEventWatch',
        body: {
          'revision': 0,
          'streamId': 'stream-b',
          'streamReset': false,
          'oldestAvailableRevision': null,
          'latestRevision': 0,
          'replayGap': false,
          'businessType': ClientBusinessEventType.stateChanged,
          'businessData': <String, Object?>{},
          'snapshot': state('10.0.0.21'),
        },
      ),
    ]);
    addTearDown(service.close);

    final bridge =
        MethodChannelClientCoreBridge(localServiceHost: service.host);
    _closeBridgeOnTearDown(bridge);
    await bridge.start();

    await _waitFor(
      () =>
          bridge.state.value.virtualIp == '10.0.0.21' &&
          service.seenRequests
                  .where((request) =>
                      request['method'] == 'localBusinessEventWatch')
                  .length >=
              3,
      reason: 'stream reset should restore new snapshot and resume watching',
    );
    final watchRequests = service.seenRequests
        .where((request) => request['method'] == 'localBusinessEventWatch')
        .toList();
    final secondArgs =
        (watchRequests[1]['args'] as Map).cast<String, Object?>();
    final thirdArgs = (watchRequests[2]['args'] as Map).cast<String, Object?>();
    expect(secondArgs['streamId'], 'stream-a');
    expect(secondArgs['lastRevision'], 1);
    expect(thirdArgs['streamId'], 'stream-b');
    expect(thirdArgs['lastRevision'], 0);
  });

  test('enable switch updates asynchronously after service result', () async {
    final service = await _FakeClientService.start([
      _ServiceReply(
        delay: const Duration(milliseconds: 150),
        expectedMethod: 'localBusinessEventWatch',
        body: _businessEvent(
          ClientBusinessEventType.networkSwitchFinished,
          {
            'activated': true,
            'networkEnabled': true,
            'syncing': false,
            'switchEnabled': true,
            'virtualIp': '10.0.0.10',
          },
        ),
      ),
      _ServiceReply(
        expectedMethod: 'localState',
        body: {
          'activated': true,
          'networkEnabled': true,
          'syncing': false,
          'switchEnabled': true,
          'virtualIp': '10.0.0.10',
        },
      ),
      _ServiceReply(
        expectedMethod: 'localNetworkActivate',
        body: {
          'activated': true,
          'networkEnabled': true,
          'syncing': false,
          'switchEnabled': true,
          'virtualIp': '10.0.0.10',
        },
      ),
    ]);
    addTearDown(service.close);

    final bridge = MethodChannelClientCoreBridge(
      localServiceHost: service.host,
    );
    _closeBridgeOnTearDown(bridge);
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
      () => bridge.state.value.virtualIp == '10.0.0.10',
      reason: 'enable result should update state asynchronously',
    );
    expect(bridge.state.value.syncing, isFalse);
    expect(bridge.state.value.switchEnabled, isTrue);
    expect(bridge.state.value.virtualIp, '10.0.0.10');
  });

  test('in-flight toggle ignores repeated clicks', () async {
    final service = await _FakeClientService.start([
      _ServiceReply(
        delay: const Duration(milliseconds: 250),
        expectedMethod: 'localBusinessEventWatch',
        body: _businessEvent(
          ClientBusinessEventType.networkSwitchFinished,
          {
            'activated': true,
            'networkEnabled': true,
            'syncing': false,
            'switchEnabled': true,
            'virtualIp': '10.0.0.10',
          },
        ),
      ),
      _ServiceReply(
        expectedMethod: 'localState',
        body: {
          'activated': true,
          'networkEnabled': true,
          'syncing': false,
          'switchEnabled': true,
          'virtualIp': '10.0.0.10',
        },
      ),
      _ServiceReply(
        expectedMethod: 'localNetworkActivate',
        body: {
          'activated': true,
          'networkEnabled': true,
          'syncing': false,
          'switchEnabled': true,
          'virtualIp': '10.0.0.10',
        },
      ),
    ]);
    addTearDown(service.close);

    final bridge = MethodChannelClientCoreBridge(
      localServiceHost: service.host,
    );
    _closeBridgeOnTearDown(bridge);
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
      () => bridge.state.value.virtualIp == '10.0.0.10',
      reason: 'first toggle should settle without repeated clicks racing it',
    );
    expect(service.seenMethods, isNot(contains('localNetworkDeactivate')));
    expect(
      service.seenMethods.where((method) => method == 'localNetworkActivate'),
      hasLength(1),
    );
    expect(bridge.state.value.networkEnabled, isTrue);
    expect(bridge.state.value.switchEnabled, isTrue);
    expect(bridge.state.value.virtualIp, '10.0.0.10');
  });

  test('disable switch updates asynchronously after service result', () async {
    final service = await _FakeClientService.start([
      _ServiceReply(
        delay: const Duration(milliseconds: 150),
        expectedMethod: 'localBusinessEventWatch',
        body: _businessEvent(
          ClientBusinessEventType.networkSwitchFinished,
          {
            'activated': true,
            'networkEnabled': false,
            'syncing': false,
            'switchEnabled': true,
            'virtualIp': '10.0.0.10',
          },
        ),
      ),
      _ServiceReply(
        expectedMethod: 'start',
        body: {
          'activated': true,
          'networkEnabled': false,
          'syncing': false,
          'switchEnabled': true,
          'virtualIp': '10.0.0.10',
        },
      ),
      _ServiceReply(
        expectedMethod: 'localNetworkDeactivate',
        body: {
          'activated': true,
          'networkEnabled': false,
          'syncing': false,
          'switchEnabled': true,
          'virtualIp': '10.0.0.10',
        },
      ),
    ]);
    addTearDown(service.close);

    final bridge = MethodChannelClientCoreBridge(
      localServiceHost: service.host,
    );
    _closeBridgeOnTearDown(bridge);
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
    expect(bridge.state.value.virtualIp, '10.0.0.10');

    await _waitFor(
      () => bridge.state.value.syncing == false,
      reason: 'disable result should update state asynchronously',
    );
    expect(bridge.state.value.switchEnabled, isTrue);
    expect(bridge.state.value.networkEnabled, isFalse);
    expect(bridge.state.value.virtualIp, '10.0.0.10');
  });

  test('enable failure restores switch', () async {
    final service = await _FakeClientService.start([
      _ServiceReply(
        delay: const Duration(milliseconds: 80),
        expectedMethod: 'localBusinessEventWatch',
        body: _businessEvent(
          ClientBusinessEventType.networkSwitchFailed,
          {
            'activated': true,
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
          'activated': true,
          'networkEnabled': false,
          'syncing': false,
          'switchEnabled': true,
          'error': 'device unavailable: current device has been disabled',
        },
      ),
      _ServiceReply(
        expectedMethod: 'localNetworkActivate',
        body: {
          'activated': true,
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
    _closeBridgeOnTearDown(bridge);
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
          'activated': true,
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
    _closeBridgeOnTearDown(bridge);

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
            'activated': true,
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
          'activated': true,
          'networkEnabled': false,
          'syncing': false,
          'switchEnabled': true,
        },
      ),
      _ServiceReply(
        expectedMethod: 'localNetworkActivate',
        body: {
          'activated': true,
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
    _closeBridgeOnTearDown(bridge);
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
    _closeBridgeOnTearDown(bridge);

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

  test('desktop transport error rolls back optimistic switch state', () async {
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
    _closeBridgeOnTearDown(bridge);

    await bridge.dispatch(
      const ClientCommand(ClientCommandType.enableNetwork),
    );

    await _waitFor(
      () => bridge.state.value.errorSource == ClientErrorSource.networkSwitch,
      reason: 'transport error should settle switch as network switch failure',
    );
    expect(bridge.state.value.error, contains('No element'));
    expect(bridge.state.value.networkEnabled, isFalse);
    expect(bridge.state.value.switchEnabled, isTrue);
  });

  test('desktop switch preserves service error text without Bad state prefix',
      () async {
    final service = await _FakeClientService.start([
      _ServiceReply(
        expectedMethod: 'localNetworkActivate',
        body: {
          'activated': true,
          'networkEnabled': false,
          'syncing': false,
          'switchEnabled': true,
          'error': 'vpn adapter unavailable',
        },
      ),
    ]);
    addTearDown(service.close);

    final bridge = MethodChannelClientCoreBridge(
      localServiceHost: service.host,
    );
    _closeBridgeOnTearDown(bridge);

    await bridge.dispatch(
      const ClientCommand(ClientCommandType.enableNetwork),
    );

    await _waitFor(
      () => bridge.state.value.errorSource == ClientErrorSource.networkSwitch,
      reason: 'desktop switch service error should settle immediately',
    );
    expect(bridge.state.value.error, 'vpn adapter unavailable');
  });

  test('disable exception restores previous enabled state and ip', () async {
    final service = await _FakeClientService.start([
      _ServiceReply(
        delay: const Duration(milliseconds: 80),
        expectedMethod: 'localBusinessEventWatch',
        body: _businessEvent(
          ClientBusinessEventType.networkSwitchFinished,
          {
            'activated': true,
            'networkEnabled': true,
            'syncing': false,
            'switchEnabled': true,
            'virtualIp': '10.0.0.10',
          },
        ),
      ),
      _ServiceReply(
        expectedMethod: 'localState',
        body: {
          'activated': true,
          'networkEnabled': true,
          'syncing': false,
          'switchEnabled': true,
          'virtualIp': '10.0.0.10',
        },
      ),
      _ServiceReply(
        expectedMethod: 'localNetworkActivate',
        body: {
          'activated': true,
          'networkEnabled': true,
          'syncing': false,
          'switchEnabled': true,
          'virtualIp': '10.0.0.10',
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
    _closeBridgeOnTearDown(bridge);
    await bridge.start();

    await bridge.dispatch(
      const ClientCommand(ClientCommandType.enableNetwork),
    );
    await _waitFor(
      () => bridge.state.value.virtualIp == '10.0.0.10',
      reason: 'precondition enable should settle',
    );

    await bridge.dispatch(
      const ClientCommand(ClientCommandType.disableNetwork),
    );

    expect(bridge.state.value.networkEnabled, isFalse);
    expect(bridge.state.value.virtualIp, '10.0.0.10');
    expect(bridge.state.value.switchEnabled, isFalse);

    await _waitFor(
      () => bridge.state.value.errorSource == ClientErrorSource.networkSwitch,
      reason: 'disable exception should be marked as network switch failure',
    );
    expect(bridge.state.value.networkEnabled, isTrue);
    expect(bridge.state.value.syncing, isFalse);
    expect(bridge.state.value.switchEnabled, isTrue);
    expect(bridge.state.value.virtualIp, '10.0.0.10');
  });

  test('successful switch does not issue extra refresh', () async {
    final service = await _FakeClientService.start([
      _ServiceReply(
        delay: const Duration(milliseconds: 80),
        expectedMethod: 'localBusinessEventWatch',
        body: _businessEvent(
          ClientBusinessEventType.networkSwitchFinished,
          {
            'activated': true,
            'networkEnabled': true,
            'syncing': false,
            'switchEnabled': true,
            'virtualIp': '10.0.0.10',
          },
        ),
      ),
      _ServiceReply(
        expectedMethod: 'localState',
        body: {
          'activated': true,
          'networkEnabled': true,
          'syncing': false,
          'switchEnabled': true,
          'virtualIp': '10.0.0.10',
        },
      ),
      _ServiceReply(
        expectedMethod: 'localNetworkActivate',
        body: {
          'activated': true,
          'networkEnabled': true,
          'syncing': false,
          'switchEnabled': true,
          'virtualIp': '10.0.0.10',
        },
      ),
    ]);
    addTearDown(service.close);

    final bridge = MethodChannelClientCoreBridge(
      localServiceHost: service.host,
    );
    _closeBridgeOnTearDown(bridge);
    await bridge.start();

    await bridge.dispatch(
      const ClientCommand(ClientCommandType.enableNetwork),
    );

    await _waitFor(
      () => bridge.state.value.virtualIp == '10.0.0.10',
      reason: 'enable result should settle',
    );
    await Future<void>.delayed(const Duration(milliseconds: 50));
    expect(service.seenMethods, isNot(contains('refresh')));
    expect(
        service.seenMethods,
        containsAll(
            ['localNetworkActivate', 'localBusinessEventWatch', 'localState']));
  });

  test('disabled network state retains assigned device ip', () {
    final state = ClientViewState.fromJson({
      'activated': true,
      'networkEnabled': false,
      'syncing': false,
      'switchEnabled': true,
      'virtualIp': '10.0.0.99',
    });

    expect(state.networkEnabled, isFalse);
    expect(state.virtualIp, '10.0.0.99');
  });
}

void _closeBridgeOnTearDown(MethodChannelClientCoreBridge bridge) {
  addTearDown(bridge.close);
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
  final List<Map<String, Object?>> seenRequests = [];

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
      seenRequests.add(requestJson);
      final method = requestJson['method'];
      if (method is String) {
        seenMethods.add(method);
      }
      final reply = _takeReply(method);
      if (reply.expectedMethod != null && method != reply.expectedMethod) {
        socket.write(
          '${jsonEncode({
                'activated': true,
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
