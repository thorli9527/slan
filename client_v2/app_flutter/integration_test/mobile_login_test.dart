import 'dart:async';
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

  testWidgets('mobile password login signs in through client-core-service',
      (tester) async {
    const bizUrl = String.fromEnvironment(
      'SLAN_TEST_BIZ_URL',
      defaultValue: 'http://127.0.0.1:28080',
    );
    const checkSwitch = bool.fromEnvironment(
      'SLAN_TEST_CHECK_SWITCH',
      defaultValue: false,
    );
    const reenableNetwork = bool.fromEnvironment(
      'SLAN_TEST_REENABLE_NETWORK',
      defaultValue: false,
    );
    const waitMqtt = bool.fromEnvironment(
      'SLAN_TEST_WAIT_MQTT',
      defaultValue: false,
    );
    const configuredEmail = String.fromEnvironment('SLAN_TEST_EMAIL');
    const password = String.fromEnvironment(
      'SLAN_TEST_PASSWORD',
      defaultValue: 'Password123!',
    );
    const registerUser = bool.fromEnvironment(
      'SLAN_TEST_REGISTER_USER',
      defaultValue: true,
    );
    const expectedDeviceId =
        String.fromEnvironment('SLAN_TEST_EXPECT_DEVICE_ID');
    const sendTargetDeviceId =
        String.fromEnvironment('SLAN_TEST_SEND_TARGET_DEVICE_ID');
    const sendBody = String.fromEnvironment('SLAN_TEST_SEND_BODY');
    const expectMessageFromDeviceId =
        String.fromEnvironment('SLAN_TEST_EXPECT_MESSAGE_FROM_DEVICE_ID');
    const expectMessageBody =
        String.fromEnvironment('SLAN_TEST_EXPECT_MESSAGE_BODY');
    const holdSeconds = int.fromEnvironment(
      'SLAN_TEST_HOLD_SECONDS',
      defaultValue: 0,
    );
    const udpEchoPort = int.fromEnvironment(
      'SLAN_TEST_UDP_ECHO_PORT',
      defaultValue: 0,
    );
    const udpSendTarget = String.fromEnvironment('SLAN_TEST_UDP_SEND_TARGET');
    const udpSendBody = String.fromEnvironment(
      'SLAN_TEST_UDP_SEND_BODY',
      defaultValue: 'slan-mobile-socket-smoke',
    );
    const postEnableWaitSeconds = int.fromEnvironment(
      'SLAN_TEST_POST_ENABLE_WAIT_SECONDS',
      defaultValue: 0,
    );
    const tcpEchoPort = int.fromEnvironment(
      'SLAN_TEST_TCP_ECHO_PORT',
      defaultValue: 0,
    );
    const tcpSendTarget = String.fromEnvironment('SLAN_TEST_TCP_SEND_TARGET');
    const tcpSendBody = String.fromEnvironment(
      'SLAN_TEST_TCP_SEND_BODY',
      defaultValue: 'slan-mobile-tcp-smoke',
    );
    final email = configuredEmail.trim().isEmpty
        ? 'mobile-login-${DateTime.now().microsecondsSinceEpoch}@example.test'
        : configuredEmail.trim();
    if (registerUser) {
      await _registerTestUser(bizUrl, email, password);
    }

    final bridge = MethodChannelClientCoreBridge();
    await tester.pumpWidget(SlanClientV2App(bridge: bridge));
    await tester.pumpAndSettle(const Duration(seconds: 1));

    if (tester.any(find.text('当前用户邮箱')) && !tester.any(find.text(email))) {
      await tester.logoutSignedInUser();
    }
    if (!tester.any(find.text('当前用户邮箱'))) {
      await tester.setMobileServerUrl(bizUrl);
      await tester.enterText(find.byKey(const Key('login-email')), email);
      await tester.enterText(find.byKey(const Key('login-password')), password);
      final loginButton = find.byKey(const Key('login-submit'));
      await tester.ensureVisible(loginButton);
      await tester.tap(loginButton);
      await tester.pump();

      await tester.pumpUntilSignedInOrLoginFailed(
        timeout: const Duration(seconds: 15),
      );
    }
    expect(find.text(email), findsOneWidget);
    expect(find.byKey(const Key('network-switch')), findsOneWidget);
    expect(find.byKey(const Key('client-message-target')), findsOneWidget);
    expect(find.byKey(const Key('client-device-id-value')), findsOneWidget);
    final currentDeviceId = tester
        .widget<Text>(find.byKey(const Key('client-device-id-value')))
        .data
        ?.trim();
    if (currentDeviceId == null || currentDeviceId.isEmpty) {
      fail('signed in UI returned empty device id');
    }
    debugPrint('SLAN_TEST_CLIENT_DEVICE_ID=$currentDeviceId');
    if (expectedDeviceId.trim().isNotEmpty) {
      expect(find.text(expectedDeviceId.trim()), findsOneWidget);
    }
    if (waitMqtt) {
      await tester.pumpUntilMqttConnected(
        bridge,
        timeout: const Duration(seconds: 45),
      );
    }

    if (checkSwitch) {
      if (reenableNetwork && tester.networkIpText() != null) {
        await tester.tap(find.byKey(const Key('network-switch')));
        await tester.pump();
        await tester.pumpUntilNetworkDisabledOrFailed(
          timeout: const Duration(seconds: 20),
        );
      }
      if (tester.networkIpText() == null) {
        await tester.tap(find.byKey(const Key('network-switch')));
      }
      await tester.pump();
      await tester.pumpUntilNetworkEnabledOrFailed(
        timeout: const Duration(seconds: 45),
      );
      expect(find.text('操作失败'), findsNothing);
      expect(find.byKey(const Key('network-ip-value')), findsOneWidget);
      final ipText = tester
          .widget<Text>(find.byKey(const Key('network-ip-value')))
          .data
          ?.trim();
      expect(ipText, isNotNull);
      expect(ipText, isNotEmpty);
      expect(ipText, isNot('未启用'));
      debugPrint('SLAN_TEST_NETWORK_IP=$ipText');
      await tester.logPlatformTunnelState();
      if (postEnableWaitSeconds > 0) {
        await tester.pump(Duration(seconds: postEnableWaitSeconds));
      }
    }

    RawDatagramSocket? udpEchoSocket;
    ServerSocket? tcpEchoServer;
    if (udpEchoPort > 0) {
      if (!checkSwitch) {
        fail('SLAN_TEST_UDP_ECHO_PORT requires SLAN_TEST_CHECK_SWITCH=true');
      }
      udpEchoSocket = await tester.startUdpEchoServer(udpEchoPort);
    }
    if (tcpEchoPort > 0) {
      if (!checkSwitch) {
        fail('SLAN_TEST_TCP_ECHO_PORT requires SLAN_TEST_CHECK_SWITCH=true');
      }
      tcpEchoServer = await tester.startTcpEchoServer(tcpEchoPort);
    }

    if (udpSendTarget.trim().isNotEmpty) {
      if (!checkSwitch) {
        fail('SLAN_TEST_UDP_SEND_TARGET requires SLAN_TEST_CHECK_SWITCH=true');
      }
      await tester.sendUdpEcho(
        target: udpSendTarget.trim(),
        body: udpSendBody.trim(),
        timeout: const Duration(seconds: 12),
      );
    }
    if (tcpSendTarget.trim().isNotEmpty) {
      if (!checkSwitch) {
        fail('SLAN_TEST_TCP_SEND_TARGET requires SLAN_TEST_CHECK_SWITCH=true');
      }
      await tester.sendTcpEcho(
        target: tcpSendTarget.trim(),
        body: tcpSendBody.trim(),
        timeout: const Duration(seconds: 12),
      );
    }

    if (sendTargetDeviceId.trim().isNotEmpty || sendBody.trim().isNotEmpty) {
      if (sendTargetDeviceId.trim().isEmpty || sendBody.trim().isEmpty) {
        fail(
            'SLAN_TEST_SEND_TARGET_DEVICE_ID and SLAN_TEST_SEND_BODY must be set together');
      }
      await tester.sendClientMessage(
        targetDeviceId: sendTargetDeviceId.trim(),
        body: sendBody.trim(),
      );
    }

    if (expectMessageFromDeviceId.trim().isNotEmpty ||
        expectMessageBody.trim().isNotEmpty) {
      if (expectMessageFromDeviceId.trim().isEmpty ||
          expectMessageBody.trim().isEmpty) {
        fail(
            'SLAN_TEST_EXPECT_MESSAGE_FROM_DEVICE_ID and SLAN_TEST_EXPECT_MESSAGE_BODY must be set together');
      }
      await tester.pumpUntilMqttConnected(
        bridge,
        timeout: const Duration(seconds: 15),
      );
      await tester.pumpUntilClientMessage(
        fromDeviceId: expectMessageFromDeviceId.trim(),
        body: expectMessageBody.trim(),
        timeout: const Duration(seconds: 45),
      );
    }

    if (holdSeconds > 0) {
      await tester.pump(Duration(seconds: holdSeconds));
    }
    udpEchoSocket?.close();
    await tcpEchoServer?.close();
  });
}

Future<void> _registerTestUser(
  String bizUrl,
  String email,
  String password,
) async {
  final uri = Uri.parse(bizUrl).resolve('/auth/register');
  final client = HttpClient();
  client.connectionTimeout = const Duration(seconds: 5);
  try {
    final request =
        await client.postUrl(uri).timeout(const Duration(seconds: 5));
    request.headers.contentType = ContentType.json;
    request.write(jsonEncode({
      'email': email,
      'password': password,
    }));
    final response = await request.close().timeout(const Duration(seconds: 10));
    final body = await response
        .transform(utf8.decoder)
        .join()
        .timeout(const Duration(seconds: 5));
    if (response.statusCode == HttpStatus.conflict) {
      return;
    }
    if (response.statusCode < 200 || response.statusCode >= 300) {
      fail('register failed: HTTP ${response.statusCode}: $body');
    }
    final json = jsonDecode(body) as Map<String, dynamic>;
    if ((json['accessToken'] as String? ?? '').trim().isEmpty) {
      fail('register returned empty accessToken: $body');
    }
  } finally {
    client.close(force: true);
  }
}

extension on WidgetTester {
  Future<void> setMobileServerUrl(String serverUrl) async {
    var expected = serverUrl.trim();
    if (!expected.contains('://')) {
      expected = 'http://$expected';
    }
    expected = expected.replaceFirst(RegExp(r'/+$'), '');
    final settings = find.byKey(const Key('server-settings'));
    await ensureVisible(settings);
    await tap(settings);
    await pumpAndSettle();
    await enterText(find.byKey(const Key('server-base-url')), serverUrl);
    await tap(find.byKey(const Key('server-save')));
    await pumpAndSettle();
    expect(find.text(expected), findsOneWidget);
  }

  Future<void> pumpUntilSignedInOrLoginFailed({
    required Duration timeout,
  }) async {
    final signedIn = find.text('当前用户邮箱');
    final loginFailed = find.textContaining('登录失败');
    final end = DateTime.now().add(timeout);
    while (DateTime.now().isBefore(end)) {
      await pump(const Duration(milliseconds: 250));
      if (any(signedIn)) {
        return;
      }
      if (any(loginFailed)) {
        final texts = widgetList<Text>(find.byType(Text))
            .map((widget) => widget.data)
            .whereType<String>()
            .toList();
        fail('login failed UI text: ${texts.join(' | ')}');
      }
    }
    await pumpAndSettle();
    final texts = widgetList<Text>(find.byType(Text))
        .map((widget) => widget.data)
        .whereType<String>()
        .toList();
    debugPrint('visible text on timeout: ${texts.join(' | ')}');
    expect(signedIn, findsOneWidget);
  }

  Future<void> logoutSignedInUser() async {
    final logoutButton = find.text('Logout');
    await ensureVisible(logoutButton);
    await tap(logoutButton);
    final loginEmail = find.byKey(const Key('login-email'));
    final end = DateTime.now().add(const Duration(seconds: 10));
    while (DateTime.now().isBefore(end)) {
      await pump(const Duration(milliseconds: 250));
      if (any(loginEmail)) {
        return;
      }
    }
    await pumpAndSettle();
    expect(loginEmail, findsOneWidget);
  }

  Future<void> pumpUntilNetworkEnabledOrFailed({
    required Duration timeout,
  }) async {
    final failure = find.text('操作失败');
    final end = DateTime.now().add(timeout);
    while (DateTime.now().isBefore(end)) {
      await pump(const Duration(milliseconds: 500));
      if (any(failure)) {
        final texts = widgetList<Text>(find.byType(Text))
            .map((widget) => widget.data)
            .whereType<String>()
            .toList();
        fail('network switch failed UI text: ${texts.join(' | ')}');
      }
      final ipFinder = find.byKey(const Key('network-ip-value'));
      if (any(ipFinder)) {
        final ipText = widget<Text>(ipFinder).data?.trim();
        if (ipText != null && ipText.isNotEmpty && ipText != '未启用') {
          return;
        }
      }
    }
    await pumpAndSettle();
    final texts = widgetList<Text>(find.byType(Text))
        .map((widget) => widget.data)
        .whereType<String>()
        .toList();
    fail('network switch did not enable before timeout: ${texts.join(' | ')}');
  }

  Future<void> pumpUntilNetworkDisabledOrFailed({
    required Duration timeout,
  }) async {
    final failure = find.text('操作失败');
    final end = DateTime.now().add(timeout);
    while (DateTime.now().isBefore(end)) {
      await pump(const Duration(milliseconds: 500));
      if (any(failure)) {
        final texts = widgetList<Text>(find.byType(Text))
            .map((widget) => widget.data)
            .whereType<String>()
            .toList();
        fail('network disable failed UI text: ${texts.join(' | ')}');
      }
      if (networkIpText() == null) {
        return;
      }
    }
    final texts = widgetList<Text>(find.byType(Text))
        .map((widget) => widget.data)
        .whereType<String>()
        .toList();
    fail('network switch did not disable before timeout: ${texts.join(' | ')}');
  }

  String? networkIpText() {
    final ipFinder = find.byKey(const Key('network-ip-value'));
    if (!any(ipFinder)) {
      return null;
    }
    final ipText = widget<Text>(ipFinder).data?.trim();
    if (ipText == null || ipText.isEmpty || ipText == '未启用') {
      return null;
    }
    return ipText;
  }

  Future<void> sendClientMessage({
    required String targetDeviceId,
    required String body,
  }) async {
    await enterText(
        find.byKey(const Key('client-message-target')), targetDeviceId);
    await enterText(find.byKey(const Key('client-message-body')), body);
    await tap(find.byKey(const Key('client-message-send')));
    await pumpUntilMessageSentOrFailed(timeout: const Duration(seconds: 12));
  }

  Future<void> pumpUntilMessageSentOrFailed({
    required Duration timeout,
  }) async {
    final sent = find.text('消息已发送');
    final failed = find.textContaining('消息发送失败');
    final end = DateTime.now().add(timeout);
    while (DateTime.now().isBefore(end)) {
      await pump(const Duration(milliseconds: 250));
      if (any(sent)) {
        return;
      }
      if (any(failed)) {
        final texts = widgetList<Text>(find.byType(Text))
            .map((widget) => widget.data)
            .whereType<String>()
            .toList();
        fail('message send failed UI text: ${texts.join(' | ')}');
      }
    }
    fail('message send did not complete before timeout');
  }

  Future<void> pumpUntilClientMessage({
    required String fromDeviceId,
    required String body,
    required Duration timeout,
  }) async {
    final expectedText = '$fromDeviceId: $body';
    final expected = find.text(expectedText);
    final end = DateTime.now().add(timeout);
    while (DateTime.now().isBefore(end)) {
      await pump(const Duration(milliseconds: 500));
      if (any(expected)) {
        return;
      }
    }
    final texts = widgetList<Text>(find.byType(Text))
        .map((widget) => widget.data)
        .whereType<String>()
        .toList();
    fail(
        'client message not received: expected="$expectedText" visible=${texts.join(' | ')}');
  }

  Future<void> pumpUntilMqttConnected(
    ClientCoreBridge bridge, {
    required Duration timeout,
  }) async {
    final end = DateTime.now().add(timeout);
    Object? lastStatus;
    while (DateTime.now().isBefore(end)) {
      await pump(const Duration(milliseconds: 500));
      final status = await bridge.localControlStatus();
      lastStatus =
          'ready=${status?.ready} mqttConnected=${status?.mqttConnected} '
          'missing=${status?.missing.join(',')} '
          'lastError=${status?.mqttLastError} '
          'lastMessageType=${status?.mqttLastMessageType}';
      if (status?.mqttConnected == true) {
        debugPrint('SLAN_TEST_MQTT_STATUS=$lastStatus');
        return;
      }
    }
    fail('MQTT did not connect before waiting for client message: $lastStatus');
  }

  Future<RawDatagramSocket> startUdpEchoServer(int port) async {
    final socket = await RawDatagramSocket.bind(InternetAddress.anyIPv4, port);
    socket.listen((event) {
      if (event != RawSocketEvent.read) {
        return;
      }
      Datagram? datagram;
      while ((datagram = socket.receive()) != null) {
        final current = datagram;
        if (current == null) {
          continue;
        }
        debugPrint(
          'SLAN_TEST_UDP_ECHO_RECEIVED=${current.address.address}:${current.port} '
          'body=${utf8.decode(current.data)}',
        );
        final payload = utf8.encode('echo:${utf8.decode(current.data)}');
        socket.send(payload, current.address, current.port);
      }
    });
    debugPrint('SLAN_TEST_UDP_ECHO_PORT=$port');
    return socket;
  }

  Future<void> sendUdpEcho({
    required String target,
    required String body,
    required Duration timeout,
  }) async {
    final separator = target.lastIndexOf(':');
    if (separator <= 0 || separator == target.length - 1) {
      fail('SLAN_TEST_UDP_SEND_TARGET must be host:port, got $target');
    }
    final host = target.substring(0, separator);
    final port = int.tryParse(target.substring(separator + 1));
    if (port == null || port <= 0) {
      fail('invalid UDP target port in $target');
    }
    final socket = await RawDatagramSocket.bind(InternetAddress.anyIPv4, 0);
    try {
      debugPrint('SLAN_TEST_UDP_SEND_TARGET=$host:$port body=$body');
      socket.send(utf8.encode(body), InternetAddress(host), port);
      final expected = 'echo:$body';
      final received = await socket
          .where((event) => event == RawSocketEvent.read)
          .map((_) => socket.receive())
          .where((datagram) => datagram != null)
          .cast<Datagram>()
          .map((datagram) => utf8.decode(datagram.data))
          .first
          .timeout(timeout)
          .onError<TimeoutException>((error, stackTrace) async {
        await logPlatformTunnelState(prefix: 'SLAN_TEST_UDP_TIMEOUT_STATE');
        throw error;
      });
      if (received != expected) {
        fail('unexpected UDP echo response: got="$received" want="$expected"');
      }
      debugPrint('SLAN_TEST_UDP_ECHO_OK=$target');
    } finally {
      socket.close();
    }
  }

  Future<ServerSocket> startTcpEchoServer(int port) async {
    final server = await ServerSocket.bind(InternetAddress.anyIPv4, port);
    server.listen((socket) {
      socket.listen(
        (data) {
          final body = utf8.decode(data);
          debugPrint(
            'SLAN_TEST_TCP_ECHO_RECEIVED=${socket.remoteAddress.address}:${socket.remotePort} body=$body',
          );
          socket.write('echo:$body');
        },
        onDone: () => socket.destroy(),
        onError: (_) => socket.destroy(),
      );
    });
    debugPrint('SLAN_TEST_TCP_ECHO_PORT=$port');
    return server;
  }

  Future<void> sendTcpEcho({
    required String target,
    required String body,
    required Duration timeout,
  }) async {
    final separator = target.lastIndexOf(':');
    if (separator <= 0 || separator == target.length - 1) {
      fail('SLAN_TEST_TCP_SEND_TARGET must be host:port, got $target');
    }
    final host = target.substring(0, separator);
    final port = int.tryParse(target.substring(separator + 1));
    if (port == null || port <= 0) {
      fail('invalid TCP target port in $target');
    }
    Socket? socket;
    try {
      debugPrint('SLAN_TEST_TCP_SEND_TARGET=$host:$port body=$body');
      socket = await Socket.connect(host, port, timeout: timeout);
      socket.write(body);
      await socket.flush();
      final expected = 'echo:$body';
      final receivedData = await socket.first
          .timeout(timeout)
          .onError<TimeoutException>((error, stackTrace) async {
        await logPlatformTunnelState(prefix: 'SLAN_TEST_TCP_TIMEOUT_STATE');
        throw error;
      });
      final received = utf8.decode(receivedData);
      if (received != expected) {
        fail('unexpected TCP echo response: got="$received" want="$expected"');
      }
      debugPrint('SLAN_TEST_TCP_ECHO_OK=$target');
    } finally {
      socket?.destroy();
    }
  }

  Future<void> logPlatformTunnelState({
    String prefix = 'SLAN_TEST_TUNNEL_STATE',
  }) async {
    final plugin = ClientCorePlugin();
    try {
      if (Platform.isIOS) {
        final stats = await plugin.iosPacketTunnelStats();
        debugPrint('$prefix=${jsonEncode(stats ?? <String, Object?>{})}');
        return;
      }
      if (Platform.isAndroid) {
        var state = await plugin.androidRuntimeState();
        final end = DateTime.now().add(const Duration(seconds: 10));
        while (DateTime.now().isBefore(end) &&
            state is Map &&
            state['networkEnabled'] != true &&
            state['adapterPresent'] != true) {
          await pump(const Duration(milliseconds: 500));
          state = await plugin.androidRuntimeState();
        }
        debugPrint('$prefix=${jsonEncode(state)}');
      }
    } catch (error) {
      debugPrint('$prefix.error=$error');
    }
  }
}
