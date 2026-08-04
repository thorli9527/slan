import 'dart:convert';
import 'dart:io';

import 'package:client_core_plugin/client_core_plugin.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:integration_test/integration_test.dart';
import 'package:slan_client_v2/app/slan_client_v2_app.dart';
import 'package:slan_client_v2/bridge/client_core_bridge.dart';

void main() {
  IntegrationTestWidgetsFlutterBinding.ensureInitialized();

  runDeviceActivationTest();
}

void runDeviceActivationTest() {
  testWidgets('authorization key activates the device', (tester) async {
    const bizUrl = String.fromEnvironment('SLAN_TEST_BIZ_URL');
    const authorizationKey = String.fromEnvironment(
      'SLAN_TEST_DEVICE_AUTHORIZATION_KEY',
    );
    const checkSwitch = bool.fromEnvironment(
      'SLAN_TEST_CHECK_SWITCH',
      defaultValue: false,
    );
    const postEnableWaitSeconds = int.fromEnvironment(
      'SLAN_TEST_POST_ENABLE_WAIT_SECONDS',
      defaultValue: 0,
    );
    const holdSeconds = int.fromEnvironment(
      'SLAN_TEST_HOLD_SECONDS',
      defaultValue: 0,
    );
    const udpEchoPort = int.fromEnvironment('SLAN_TEST_UDP_ECHO_PORT');
    const tcpEchoPort = int.fromEnvironment('SLAN_TEST_TCP_ECHO_PORT');
    const udpSendTarget = String.fromEnvironment('SLAN_TEST_UDP_SEND_TARGET');
    const udpSendBody = String.fromEnvironment('SLAN_TEST_UDP_SEND_BODY');
    const tcpSendTarget = String.fromEnvironment('SLAN_TEST_TCP_SEND_TARGET');
    const tcpSendBody = String.fromEnvironment('SLAN_TEST_TCP_SEND_BODY');
    const waitMqtt = bool.fromEnvironment('SLAN_TEST_WAIT_MQTT');
    const forceRelayOnlyValue = String.fromEnvironment(
      'SLAN_TEST_FORCE_RELAY_ONLY',
    );
    const relayTransportAllowlist = String.fromEnvironment(
      'SLAN_TEST_RELAY_TRANSPORT_ALLOWLIST',
    );
    const mqttTimeoutSeconds = int.fromEnvironment(
      'SLAN_TEST_EXPECT_MQTT_TIMEOUT_SECONDS',
      defaultValue: 45,
    );
    const messageTimeoutSeconds = int.fromEnvironment(
      'SLAN_TEST_EXPECT_MESSAGE_TIMEOUT_SECONDS',
      defaultValue: 45,
    );
    const sendTargetDeviceId = String.fromEnvironment(
      'SLAN_TEST_SEND_TARGET_DEVICE_ID',
    );
    const sendBody = String.fromEnvironment('SLAN_TEST_SEND_BODY');
    const expectFromDeviceId = String.fromEnvironment(
      'SLAN_TEST_EXPECT_MESSAGE_FROM_DEVICE_ID',
    );
    const expectBody = String.fromEnvironment('SLAN_TEST_EXPECT_MESSAGE_BODY');
    if (authorizationKey.isEmpty) {
      markTestSkipped(
        'SLAN_TEST_DEVICE_AUTHORIZATION_KEY is required for activation test',
      );
      return;
    }

    final plugin = ClientCorePlugin();
    if (Platform.isAndroid) {
      final forceRelayOnly = const {'1', 'true', 'yes', 'on'}.contains(
        forceRelayOnlyValue.trim().toLowerCase(),
      );
      await plugin.setAndroidTestForceRelayOnly(forceRelayOnly);
      final transports = relayTransportAllowlist
          .split(',')
          .map((value) => value.trim())
          .where((value) => value.isNotEmpty)
          .toList();
      await plugin.embeddedServiceRequest(
        jsonEncode({
          'method': 'localSetRelayTransportAllowlist',
          'args': {'transports': transports},
        }),
      );
    }

    final bridge = MethodChannelClientCoreBridge();
    addTearDown(bridge.close);
    await tester.pumpWidget(SlanClientV2App(bridge: bridge));
    await tester.pumpAndSettle(const Duration(seconds: 1));

    if (bizUrl.trim().isNotEmpty) {
      await bridge.updateServerBaseUrl(bizUrl);
      await tester.pumpAndSettle(const Duration(seconds: 1));
    }

    final readyDeadline = DateTime.now().add(const Duration(seconds: 30));
    while (
        bridge.state.value.syncing && DateTime.now().isBefore(readyDeadline)) {
      await tester.pump(const Duration(milliseconds: 250));
    }

    if (!bridge.state.value.activated) {
      await bridge.activateDevice(authorizationKey);
      await tester.pumpAndSettle(const Duration(milliseconds: 250));
    }

    final deadline = DateTime.now().add(const Duration(seconds: 30));
    while (!bridge.state.value.activated && DateTime.now().isBefore(deadline)) {
      await tester.pump(const Duration(milliseconds: 250));
    }

    if (!bridge.state.value.activated) {
      debugPrint(
        'SLAN_TEST_ACTIVATION_FAILED '
        'deviceId=${bridge.state.value.deviceId} '
        'syncing=${bridge.state.value.syncing} '
        'syncReason=${bridge.state.value.syncReason} '
        'error=${bridge.state.value.error} '
        'errorSource=${bridge.state.value.errorSource} '
        'notice=${bridge.state.value.notice}',
      );
    }
    expect(bridge.state.value.activated, isTrue);
    expect(bridge.state.value.deviceId, isNotEmpty);
    expect(find.byKey(const Key('current-device-value')), findsOneWidget);
    debugPrint('SLAN_TEST_CLIENT_DEVICE_ID=${bridge.state.value.deviceId}');
    await tester.pump(const Duration(seconds: 2));

    if (waitMqtt || expectFromDeviceId.isNotEmpty || expectBody.isNotEmpty) {
      await _waitForMqtt(tester, bridge, mqttTimeoutSeconds);
    }

    if (checkSwitch) {
      await _enableNetwork(tester, bridge);
      if (postEnableWaitSeconds > 0) {
        await tester.pump(Duration(seconds: postEnableWaitSeconds));
      }
    }

    RawDatagramSocket? udpEchoSocket;
    ServerSocket? tcpEchoServer;
    if (udpEchoPort > 0) {
      udpEchoSocket = await _startUdpEchoServer(udpEchoPort);
    }
    if (tcpEchoPort > 0) {
      tcpEchoServer = await _startTcpEchoServer(tcpEchoPort);
    }
    if (udpSendTarget.trim().isNotEmpty) {
      await _sendUdpEcho(udpSendTarget, udpSendBody);
    }
    if (tcpSendTarget.trim().isNotEmpty) {
      await _sendTcpEcho(tcpSendTarget, tcpSendBody);
    }
    if (sendTargetDeviceId.trim().isNotEmpty || sendBody.trim().isNotEmpty) {
      if (sendTargetDeviceId.trim().isEmpty || sendBody.trim().isEmpty) {
        fail('message target device and body must be set together');
      }
      await bridge.requestLocalApi('localSendClientMessage', {
        'targetDeviceId': sendTargetDeviceId.trim(),
        'body': sendBody.trim(),
      });
      debugPrint(
        'SLAN_TEST_CLIENT_MESSAGE_SENT=${sendTargetDeviceId.trim()}:${sendBody.trim()}',
      );
    }
    if (expectFromDeviceId.trim().isNotEmpty || expectBody.trim().isNotEmpty) {
      await _waitForClientMessage(
        tester,
        bridge,
        fromDeviceId: expectFromDeviceId,
        body: expectBody,
        timeoutSeconds: messageTimeoutSeconds,
      );
    }
    await _logAndroidRuntimeStats('SLAN_ANDROID_RUNTIME_STATS_BEFORE_HOLD');
    if (holdSeconds > 0) {
      await tester.pump(Duration(seconds: holdSeconds));
      await _logAndroidRuntimeStats('SLAN_ANDROID_RUNTIME_STATS_AFTER_HOLD');
    }
    udpEchoSocket?.close();
    await tcpEchoServer?.close();
    await bridge.close();
    await tester.pumpWidget(const SizedBox.shrink());
    await tester.pumpAndSettle();
  }, semanticsEnabled: false);
}

Future<void> _logAndroidRuntimeStats(String label) async {
  if (!Platform.isAndroid) {
    return;
  }
  try {
    var stats = await ClientCorePlugin().androidRuntimeState();
    var bestScore = _androidRuntimeStatsScore(stats);
    for (var attempt = 0; attempt < 4; attempt += 1) {
      await Future<void>.delayed(const Duration(milliseconds: 500));
      final next = await ClientCorePlugin().androidRuntimeState();
      final nextScore = _androidRuntimeStatsScore(next);
      if (nextScore >= bestScore) {
        stats = next;
        bestScore = nextScore;
      }
    }
    debugPrint('$label=${jsonEncode(stats)}');
  } on Object catch (error) {
    debugPrint('${label}_ERROR=$error');
  }
}

int _androidRuntimeStatsScore(Object? stats) {
  if (stats is! Map) {
    return 0;
  }
  const keys = <String>[
    'directUdpFramesSent',
    'directUdpFramesReceived',
    'relayFramesSent',
    'relayFramesReceived',
    'relayTcpFramesReceived',
    'relayTcpSynAckReceived',
    'relayTcpPshReceived',
    'packetsRead',
    'bytesRead',
    'bytesWritten',
  ];
  var score = 0;
  for (final key in keys) {
    final value = stats[key];
    if (value is num) {
      score += value.toInt();
    }
  }
  return score;
}

Future<void> _waitForMqtt(
  WidgetTester tester,
  MethodChannelClientCoreBridge bridge,
  int timeoutSeconds,
) async {
  final deadline = DateTime.now().add(Duration(seconds: timeoutSeconds));
  Object? lastStatus;
  while (DateTime.now().isBefore(deadline)) {
    await tester.pump(const Duration(milliseconds: 500));
    final status = await bridge.localControlStatus();
    lastStatus = status;
    if (status?.mqttConnected == true) {
      debugPrint('SLAN_TEST_MQTT_STATUS=$status');
      return;
    }
  }
  fail('MQTT did not connect before timeout: $lastStatus');
}

Future<void> _waitForClientMessage(
  WidgetTester tester,
  MethodChannelClientCoreBridge bridge, {
  required String fromDeviceId,
  required String body,
  required int timeoutSeconds,
}) async {
  final expectedFrom = fromDeviceId.trim();
  final expectedBody = body.trim();
  final deadline = DateTime.now().add(Duration(seconds: timeoutSeconds));
  Map<String, Object?>? lastState;
  while (DateTime.now().isBefore(deadline)) {
    await tester.pump(const Duration(milliseconds: 500));
    lastState = await bridge.requestLocalApi('localState');
    final actualFrom =
        '${lastState?['lastClientMessageFromDeviceId'] ?? ''}'.trim();
    final actualBody = '${lastState?['lastClientMessageBody'] ?? ''}'.trim();
    if ((expectedFrom.isEmpty || actualFrom == expectedFrom) &&
        (expectedBody.isEmpty || actualBody == expectedBody)) {
      debugPrint('SLAN_TEST_CLIENT_MESSAGE_OK=$actualFrom:$actualBody');
      return;
    }
  }
  fail(
    'client message not received: from="$expectedFrom" body="$expectedBody" state="$lastState"',
  );
}

Future<void> _enableNetwork(
  WidgetTester tester,
  MethodChannelClientCoreBridge bridge,
) async {
  try {
    await bridge.prepareAndroidNetworkAuthorization();
  } on Object catch (error) {
    debugPrint('SLAN_TEST_NETWORK_AUTHORIZATION_PENDING=$error');
  }

  final deadline = DateTime.now().add(const Duration(seconds: 45));
  var nextAttemptAt = DateTime.fromMillisecondsSinceEpoch(0);
  while (DateTime.now().isBefore(deadline)) {
    final confirmButton = find.text('确定');
    if (tester.any(confirmButton)) {
      await tester.tap(confirmButton, warnIfMissed: false);
      await tester.pump(const Duration(milliseconds: 500));
    }
    if (!bridge.state.value.networkEnabled &&
        !DateTime.now().isBefore(nextAttemptAt)) {
      final networkSwitch = find.byKey(const Key('network-switch'));
      await tester.ensureVisible(networkSwitch);
      await tester.tap(networkSwitch, warnIfMissed: false);
      nextAttemptAt = DateTime.now().add(const Duration(seconds: 5));
      await tester.pump();
    }
    await tester.pump(const Duration(milliseconds: 500));
    final state = bridge.state.value;
    final virtualIp = state.virtualIp?.trim() ?? '';
    if (state.networkEnabled && virtualIp.isNotEmpty) {
      debugPrint('SLAN_TEST_NETWORK_IP=$virtualIp');
      return;
    }
  }
  fail('network did not enable before timeout: ${bridge.state.value}');
}

Future<RawDatagramSocket> _startUdpEchoServer(int port) async {
  final socket = await RawDatagramSocket.bind(InternetAddress.anyIPv4, port);
  socket.listen((event) {
    if (event != RawSocketEvent.read) return;
    final packet = socket.receive();
    if (packet != null) {
      socket.send(
        utf8.encode('echo:${utf8.decode(packet.data)}'),
        packet.address,
        packet.port,
      );
    }
  });
  debugPrint('SLAN_TEST_UDP_ECHO_PORT=$port');
  return socket;
}

Future<ServerSocket> _startTcpEchoServer(int port) async {
  final server = await ServerSocket.bind(InternetAddress.anyIPv4, port);
  server.listen((socket) {
    socket.listen(
      (data) => socket.add(utf8.encode('echo:${utf8.decode(data)}')),
      onDone: socket.destroy,
      onError: (_) => socket.destroy(),
    );
  });
  debugPrint('SLAN_TEST_TCP_ECHO_PORT=$port');
  return server;
}

Future<void> _sendUdpEcho(String target, String body) async {
  final endpoint = await _resolveEndpoint(target);
  final socket = await RawDatagramSocket.bind(InternetAddress.anyIPv4, 0);
  try {
    final response = socket
        .where((event) => event == RawSocketEvent.read)
        .map((_) => socket.receive())
        .where((packet) => packet != null)
        .cast<Datagram>()
        .first
        .timeout(const Duration(seconds: 12));
    socket.send(utf8.encode(body), endpoint.$1, endpoint.$2);
    final packet = await response;
    expect(utf8.decode(packet.data), 'echo:$body');
    debugPrint('SLAN_TEST_UDP_ECHO_OK=$target');
  } finally {
    socket.close();
  }
}

Future<void> _sendTcpEcho(String target, String body) async {
  final endpoint = await _resolveEndpoint(target);
  Object? lastError;
  for (var attempt = 1; attempt <= 3; attempt += 1) {
    Socket? socket;
    try {
      socket = await Socket.connect(endpoint.$1, endpoint.$2)
          .timeout(const Duration(seconds: 12));
      socket.add(utf8.encode('$body\n'));
      await socket.flush();
      final response = await utf8.decoder
          .bind(socket)
          .first
          .timeout(const Duration(seconds: 12));
      expect(response.trimRight(), 'echo:$body');
      debugPrint('SLAN_TEST_TCP_ECHO_OK=$target attempts=$attempt');
      return;
    } on Object catch (error) {
      lastError = error;
      debugPrint('SLAN_TEST_TCP_ATTEMPT_ERROR#$attempt=$error');
      if (attempt < 3) {
        await Future<void>.delayed(const Duration(milliseconds: 750));
      }
    } finally {
      socket?.destroy();
    }
  }
  throw lastError!;
}

Future<(InternetAddress, int)> _resolveEndpoint(String value) async {
  final separator = value.lastIndexOf(':');
  if (separator <= 0 || separator == value.length - 1) {
    fail('invalid socket endpoint: $value');
  }
  final host = value.substring(0, separator);
  final port = int.parse(value.substring(separator + 1));
  final literal = InternetAddress.tryParse(host);
  if (literal != null) {
    return (literal, port);
  }
  final addresses = await InternetAddress.lookup(
    host,
    type: InternetAddressType.IPv4,
  );
  if (addresses.isEmpty) {
    fail('socket endpoint did not resolve: $value');
  }
  return (addresses.first, port);
}
