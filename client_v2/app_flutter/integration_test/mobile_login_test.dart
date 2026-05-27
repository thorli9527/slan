import 'dart:async';
import 'dart:convert';
import 'dart:io';

import 'package:client_core_plugin/client_core_plugin.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:integration_test/integration_test.dart';
import 'package:slan_client_v2/app/slan_client_v2_app.dart';
import 'package:slan_client_v2/bridge/client_commands.dart';
import 'package:slan_client_v2/bridge/client_core_bridge.dart';

void main() {
  IntegrationTestWidgetsFlutterBinding.ensureInitialized();

  testWidgets('mobile password login signs in through client-core-service',
      (tester) async {
    const bizUrl = String.fromEnvironment(
      'SLAN_TEST_BIZ_URL',
      defaultValue: 'http://api.dev.staticlss.com',
    );
    const checkSwitch = bool.fromEnvironment(
      'SLAN_TEST_CHECK_SWITCH',
      defaultValue: false,
    );
    const requireTunnelState = bool.fromEnvironment(
      'SLAN_TEST_REQUIRE_TUNNEL_STATE',
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
    const expectNetworkModule = bool.fromEnvironment(
      'SLAN_TEST_EXPECT_NETWORK_MODULE',
      defaultValue: false,
    );
    const minNetworkModulePeers = int.fromEnvironment(
      'SLAN_TEST_MIN_NETWORK_MODULE_PEERS',
      defaultValue: 0,
    );
    const minNetworkModuleDnsRecords = int.fromEnvironment(
      'SLAN_TEST_MIN_NETWORK_MODULE_DNS_RECORDS',
      defaultValue: 0,
    );
    const minNetworkModuleSecurityRules = int.fromEnvironment(
      'SLAN_TEST_MIN_NETWORK_MODULE_SECURITY_RULES',
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
    await bridge.updateServerBaseUrl(bizUrl);
    await tester.pumpAndSettle(const Duration(seconds: 1));
    if (tester.any(find.byKey(const Key('server-settings')))) {
      await tester.setMobileServerUrl(bizUrl);
    }

    if (tester.any(find.text('当前用户邮箱')) && !tester.any(find.text(email))) {
      await tester.logoutSignedInUser();
    }
    if (!tester.any(find.text('当前用户邮箱'))) {
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
    expect(find.byKey(const Key('client-ping-target')), findsOneWidget);
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
      final authorizationButton = find.text('授权');
      if (tester.any(authorizationButton)) {
        await tester.tap(authorizationButton);
        await tester.pump(const Duration(seconds: 3));
      }
      if (tester.networkIpText() == null) {
        await tester.tap(find.byKey(const Key('network-switch')));
      }
      await tester.pump();
      await tester.pumpUntilNetworkEnabledOrFailed(
        timeout: const Duration(seconds: 45),
      );
      expect(find.text('操作失败'), findsNothing);
      final ipText = tester.networkIpText();
      expect(ipText, isNotNull);
      expect(ipText, isNotEmpty);
      debugPrint('SLAN_TEST_NETWORK_IP=$ipText');
      if (requireTunnelState) {
        await tester.logPlatformTunnelState();
      }
      if (postEnableWaitSeconds > 0) {
        await Future<void>.delayed(Duration(seconds: postEnableWaitSeconds));
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
      await tester.ensureRealPacketTunnelForSocketSend();
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
      await tester.ensureRealPacketTunnelForSocketSend();
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
      await bridge.dispatch(
        ClientCommand(
          ClientCommandType.sendClientMessage,
          {
            'targetDeviceId': sendTargetDeviceId.trim(),
            'body': sendBody.trim(),
          },
        ),
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
        bridge,
        fromDeviceId: expectMessageFromDeviceId.trim(),
        body: expectMessageBody.trim(),
        timeout: const Duration(seconds: 45),
      );
    }

    if (expectNetworkModule) {
      await tester.pumpUntilNetworkModule(
        bridge,
        minPeers: minNetworkModulePeers,
        minDnsRecords: minNetworkModuleDnsRecords,
        minSecurityRules: minNetworkModuleSecurityRules,
        timeout: const Duration(seconds: 45),
      );
    }

    await _logAndroidRuntimeStats('SLAN_ANDROID_RUNTIME_STATS_BEFORE_HOLD');
    if (holdSeconds > 0) {
      await tester.pump(Duration(seconds: holdSeconds));
      await _logAndroidRuntimeStats('SLAN_ANDROID_RUNTIME_STATS_AFTER_HOLD');
    }
    udpEchoSocket?.close();
    await tcpEchoServer?.close();
  });
}

Future<void> _logAndroidRuntimeStats(String label) async {
  try {
    final stats = await ClientCorePlugin().androidRuntimeState();
    debugPrint('$label=${jsonEncode(stats)}');
  } on Object catch (error) {
    debugPrint('${label}_ERROR=$error');
  }
}

Future<void> _registerTestUser(
  String bizUrl,
  String email,
  String password,
) async {
  final uri = Uri.parse(bizUrl).resolve('/api/auth/register');
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
    final auth = json['auth'] as Map<String, dynamic>?;
    final session = auth?['session'] as Map<String, dynamic>?;
    final accessToken = (json['accessToken'] as String? ?? '').trim();
    final nestedToken = (session?['token'] as String? ?? '').trim();
    if (accessToken.isEmpty && nestedToken.isEmpty) {
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
    final settledIp = networkIpText();
    if (settledIp != null) {
      return;
    }
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
      final texts = widgetList<Text>(find.byType(Text))
          .map((widget) => widget.data)
          .whereType<String>()
          .toList();
      if (texts.contains('网络已启用') &&
          texts
              .any((text) => RegExp(r'\b10\.\d+\.\d+\.\d+\b').hasMatch(text))) {
        return;
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
    if (any(ipFinder)) {
      final ipText = widget<Text>(ipFinder).data?.trim();
      if (ipText != null && ipText.isNotEmpty && ipText != '未启用') {
        return ipText;
      }
    }
    final texts = widgetList<Text>(find.byType(Text))
        .map((widget) => widget.data)
        .whereType<String>()
        .toList();
    if (texts.contains('网络已启用')) {
      final ipPattern = RegExp(r'\b10\.\d+\.\d+\.\d+\b');
      for (final text in texts) {
        final match = ipPattern.firstMatch(text);
        if (match != null) {
          return match.group(0);
        }
      }
    }
    return null;
  }

  Future<void> pumpUntilClientMessage(
    ClientCoreBridge bridge, {
    required String fromDeviceId,
    required String body,
    required Duration timeout,
  }) async {
    final end = DateTime.now().add(timeout);
    while (DateTime.now().isBefore(end)) {
      await pump(const Duration(milliseconds: 500));
      final state = bridge.state.value;
      if (state.lastClientMessageFromDeviceId == fromDeviceId &&
          state.lastClientMessageBody == body) {
        return;
      }
    }
    fail(
      'client message not received: expected="$fromDeviceId: $body" '
      'state="${bridge.state.value.lastClientMessageFromDeviceId}: '
      '${bridge.state.value.lastClientMessageBody}"',
    );
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

  Future<void> pumpUntilNetworkModule(
    MethodChannelClientCoreBridge bridge, {
    required int minPeers,
    required int minDnsRecords,
    required int minSecurityRules,
    required Duration timeout,
  }) async {
    final plugin = ClientCorePlugin();
    final end = DateTime.now().add(timeout);
    Map<String, Object?>? lastSnapshot;
    while (DateTime.now().isBefore(end)) {
      await pump(const Duration(milliseconds: 500));
      final snapshot = await plugin.embeddedServiceRequest(jsonEncode({
        'method': 'localNetworkModule',
        'args': <String, Object?>{},
      }));
      lastSnapshot = snapshot;
      final peers = (snapshot?['peerCount'] as num?)?.toInt() ?? 0;
      final dnsRecords = (snapshot?['dnsRecordCount'] as num?)?.toInt() ?? 0;
      final securityRules =
          (snapshot?['securityRuleCount'] as num?)?.toInt() ?? 0;
      if (peers >= minPeers &&
          dnsRecords >= minDnsRecords &&
          securityRules >= minSecurityRules) {
        debugPrint(
          'SLAN_TEST_NETWORK_MODULE=peers=$peers dnsRecords=$dnsRecords securityRules=$securityRules',
        );
        return;
      }
    }
    fail(
      'network module did not reach expected counts: '
      'minPeers=$minPeers minDnsRecords=$minDnsRecords '
      'minSecurityRules=$minSecurityRules last=$lastSnapshot',
    );
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
      final expected = 'echo:$body';
      final deadline = DateTime.now().add(timeout);
      final events = socket.asBroadcastStream();
      var attempts = 0;
      while (DateTime.now().isBefore(deadline)) {
        attempts += 1;
        final remaining = deadline.difference(DateTime.now());
        if (remaining <= Duration.zero) {
          break;
        }
        final receiveWindow = remaining < const Duration(seconds: 2)
            ? remaining
            : const Duration(seconds: 2);
        final receiveFuture = events
            .where((event) => event == RawSocketEvent.read)
            .map((_) => socket.receive())
            .where((datagram) => datagram != null)
            .cast<Datagram>()
            .map((datagram) => utf8.decode(datagram.data))
            .first
            .timeout(receiveWindow, onTimeout: () => '');
        socket.send(utf8.encode(body), InternetAddress(host), port);
        final received = await receiveFuture;
        if (received == expected) {
          debugPrint('SLAN_TEST_UDP_ECHO_OK=$target attempts=$attempts');
          return;
        }
        if (received.isNotEmpty) {
          fail(
              'unexpected UDP echo response: got="$received" want="$expected"');
        }
      }
      await logPlatformTunnelState(prefix: 'SLAN_TEST_UDP_TIMEOUT_STATE');
      throw TimeoutException(
          'UDP echo response not received after $attempts attempts', timeout);
    } finally {
      socket.close();
    }
  }

  Future<ServerSocket> startTcpEchoServer(int port) async {
    final server = await ServerSocket.bind(InternetAddress.anyIPv4, port);
    server.listen((socket) {
      final lines = socket
          .map((data) => utf8.decode(data))
          .transform(const LineSplitter());
      lines.listen(
        (body) async {
          debugPrint(
            'SLAN_TEST_TCP_ECHO_RECEIVED=${socket.remoteAddress.address}:${socket.remotePort} body=$body',
          );
          socket.writeln('echo:$body');
          await socket.flush();
          await socket.close();
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
    debugPrint('SLAN_TEST_TCP_SEND_TARGET=$host:$port body=$body');
    final expected = 'echo:$body';
    final deadline = DateTime.now().add(timeout);
    Object? lastError;
    var attempts = 0;
    while (DateTime.now().isBefore(deadline)) {
      attempts += 1;
      Socket? socket;
      try {
        final remaining = deadline.difference(DateTime.now());
        if (remaining <= Duration.zero) {
          break;
        }
        final connectTimeout = remaining < const Duration(seconds: 3)
            ? remaining
            : const Duration(seconds: 3);
        socket = await Socket.connect(host, port, timeout: connectTimeout);
        socket.writeln(body);
        await socket.flush();
        final received = await socket
            .map((data) => utf8.decode(data))
            .transform(const LineSplitter())
            .first
            .timeout(connectTimeout);
        if (received != expected) {
          fail(
              'unexpected TCP echo response: got="$received" want="$expected"');
        }
        debugPrint('SLAN_TEST_TCP_ECHO_OK=$target attempts=$attempts');
        return;
      } on Object catch (error) {
        lastError = error;
        debugPrint('SLAN_TEST_TCP_ATTEMPT_ERROR#$attempts=$error');
      } finally {
        socket?.destroy();
      }
    }
    await logPlatformTunnelState(prefix: 'SLAN_TEST_TCP_ERROR_STATE');
    throw TimeoutException(
        'TCP echo response not received after $attempts attempts; last=$lastError',
        timeout);
  }

  Future<void> logPlatformTunnelState({
    String prefix = 'SLAN_TEST_TUNNEL_STATE',
  }) async {
    final plugin = ClientCorePlugin();
    if (Platform.isIOS) {
      try {
        final stats = await plugin.iosPacketTunnelStats();
        debugPrint('$prefix=${jsonEncode(stats ?? <String, Object?>{})}');
      } catch (error) {
        debugPrint('$prefix.error=$error');
      }
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
      if (state is Map &&
          (state['networkEnabled'] != true ||
              state['adapterPresent'] != true)) {
        fail('Android tunnel is not running: ${jsonEncode(state)}');
      }
    }
  }

  Future<void> ensureRealPacketTunnelForSocketSend() async {
    if (!Platform.isIOS) {
      return;
    }
    final stats = await ClientCorePlugin().iosPacketTunnelStats();
    final tunnelStats = stats ?? <String, Object?>{};
    debugPrint('SLAN_TEST_IOS_PACKET_TUNNEL_STATE=${jsonEncode(tunnelStats)}');
    if (tunnelStats['simulatorFallback'] == true) {
      fail(
        'iOS Simulator uses PacketTunnel fallback and cannot route real '
        'UDP/TCP sockets through the tunnel. Use a real iOS device for '
        'system packet send/receive tests.',
      );
    }
  }
}
