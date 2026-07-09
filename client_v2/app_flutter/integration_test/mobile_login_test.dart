import 'dart:async';
import 'dart:convert';
import 'dart:io';

import 'package:client_core_plugin/client_core_plugin.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:integration_test/integration_test.dart';
import 'package:slan_client_v2/app/slan_client_v2_app.dart';
import 'package:slan_client_v2/bridge/client_core_bridge.dart';

final _ipv4Pattern = RegExp(r'\b(?:\d{1,3}\.){3}\d{1,3}\b');
final _ipv4WithPrefixPattern = RegExp(r'\b(?:\d{1,3}\.){3}\d{1,3}/\d{1,2}\b');
final _virtualIpv4Pattern = RegExp(r'\b10(?:\.\d{1,3}){3}\b');
const _defaultTestControlBaseUrl = String.fromEnvironment(
  'SLAN_DEFAULT_CONTROL_BASE_URL',
  defaultValue: 'http://47.245.40.231:28080',
);

class _LocalApiClientMessageCheckConfig {
  const _LocalApiClientMessageCheckConfig({
    required this.sendTargetDeviceId,
    required this.sendBody,
    required this.expectFromDeviceId,
    required this.expectBody,
    required this.expectMessageTimeoutSeconds,
    required this.expectMqttTimeoutSeconds,
  });

  final String sendTargetDeviceId;
  final String sendBody;
  final String expectFromDeviceId;
  final String expectBody;
  final int expectMessageTimeoutSeconds;
  final int expectMqttTimeoutSeconds;

  bool get hasSend =>
      sendTargetDeviceId.trim().isNotEmpty || sendBody.trim().isNotEmpty;

  bool get hasExpect =>
      expectFromDeviceId.trim().isNotEmpty || expectBody.trim().isNotEmpty;

  String get trimmedSendTargetDeviceId => sendTargetDeviceId.trim();
  String get trimmedSendBody => sendBody.trim();
  String get trimmedExpectFromDeviceId => expectFromDeviceId.trim();
  String get trimmedExpectBody => expectBody.trim();

  void validate() {
    if (hasSend &&
        (trimmedSendTargetDeviceId.isEmpty || trimmedSendBody.isEmpty)) {
      fail(
        'SLAN_TEST_SEND_TARGET_DEVICE_ID and SLAN_TEST_SEND_BODY must be set together',
      );
    }
    if (hasExpect &&
        (trimmedExpectFromDeviceId.isEmpty && trimmedExpectBody.isEmpty)) {
      fail(
        'SLAN_TEST_EXPECT_MESSAGE_FROM_DEVICE_ID or '
        'SLAN_TEST_EXPECT_MESSAGE_BODY must be set when message expectation is enabled',
      );
    }
  }
}

void main() {
  IntegrationTestWidgetsFlutterBinding.ensureInitialized();

  testWidgets('mobile password login signs in through client-core-service',
      (tester) async {
    addTearDown(() {
      tester.platformDispatcher.semanticsEnabledTestValue = false;
    });
    try {
      const bizUrl = String.fromEnvironment(
        'SLAN_TEST_BIZ_URL',
        defaultValue: _defaultTestControlBaseUrl,
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
      const setAndroidVpnBypassOnly = bool.fromEnvironment(
        'SLAN_TEST_ANDROID_SET_VPN_BYPASS_ONLY',
        defaultValue: false,
      );
      const androidDebugEmulatorVpnBypass = bool.fromEnvironment(
        'SLAN_TEST_ANDROID_DEBUG_EMULATOR_VPN_BYPASS',
        defaultValue: false,
      );
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
      const messageCheck = _LocalApiClientMessageCheckConfig(
        sendTargetDeviceId: String.fromEnvironment(
          'SLAN_TEST_SEND_TARGET_DEVICE_ID',
        ),
        sendBody: String.fromEnvironment('SLAN_TEST_SEND_BODY'),
        expectFromDeviceId: String.fromEnvironment(
          'SLAN_TEST_EXPECT_MESSAGE_FROM_DEVICE_ID',
        ),
        expectBody: String.fromEnvironment('SLAN_TEST_EXPECT_MESSAGE_BODY'),
        expectMessageTimeoutSeconds: int.fromEnvironment(
          'SLAN_TEST_EXPECT_MESSAGE_TIMEOUT_SECONDS',
          defaultValue: 45,
        ),
        expectMqttTimeoutSeconds: int.fromEnvironment(
          'SLAN_TEST_EXPECT_MQTT_TIMEOUT_SECONDS',
          defaultValue: 15,
        ),
      );
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
      const relayTransportAllowlist = String.fromEnvironment(
        'SLAN_TEST_RELAY_TRANSPORT_ALLOWLIST',
      );
      if (setAndroidVpnBypassOnly) {
        await ClientCorePlugin().setAndroidDebugEmulatorVpnBypass(
          androidDebugEmulatorVpnBypass,
        );
        return;
      }
      messageCheck.validate();

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
      debugPrint('SLAN_TEST_CONFIGURED_BIZ_URL=$bizUrl');
      debugPrint(
          'SLAN_TEST_BRIDGE_SERVER_BASE_URL=${await bridge.serverBaseUrl()}');
      debugPrint(
        'SLAN_TEST_PLUGIN_MOBILE_SERVER_BASE_URL='
        '${await ClientCorePlugin().mobileServerBaseUrl()}',
      );
      await tester.configureRelayTransportAllowlist(relayTransportAllowlist);
      if (tester.any(find.byKey(const Key('server-settings')))) {
        await tester.setMobileServerUrl(bizUrl);
      }

      final signedInEmail = tester.signedInEmailText();
      if (signedInEmail != null && signedInEmail != email) {
        await tester.logoutSignedInUser();
      }
      if (tester.signedInEmailText() == null) {
        await tester.enterText(find.byKey(const Key('login-email')), email);
        await tester.enterText(
          find.byKey(const Key('login-password')),
          password,
        );
        final loginButton = find.byKey(const Key('login-submit'));
        await tester.ensureVisible(loginButton);
        await tester.tap(loginButton);
        await tester.pump();

        await tester.pumpUntilSignedInOrLoginFailed(
          timeout: const Duration(seconds: 15),
        );
      }
      expect(tester.signedInEmailText(), email);
      expect(find.byKey(const Key('network-switch')), findsOneWidget);
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
      await tester.logRelayCandidates(
        prefix: 'SLAN_TEST_RELAY_CANDIDATES_AFTER_LOGIN',
      );

      if (checkSwitch) {
        if (reenableNetwork && tester.networkIpText() != null) {
          await tester.tap(find.byKey(const Key('network-switch')));
          await tester.pump();
          await tester.pumpUntilNetworkDisabledOrFailed(
            timeout: const Duration(seconds: 20),
          );
        }
        await tester.ensureAndroidNetworkAuthorizationReady(
          timeout: const Duration(seconds: 12),
        );
        if (!await tester.isNetworkEffectivelyEnabled()) {
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
        await tester.logRelayCandidates(
          prefix: 'SLAN_TEST_RELAY_CANDIDATES_AFTER_ENABLE',
        );
        if (requireTunnelState) {
          await tester.logPlatformTunnelState();
        }
        if (postEnableWaitSeconds > 0) {
          await Future<void>.delayed(
            Duration(seconds: postEnableWaitSeconds),
          );
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
          fail(
              'SLAN_TEST_UDP_SEND_TARGET requires SLAN_TEST_CHECK_SWITCH=true');
        }
        await tester.pumpUntilSocketTargetsReady(
          targets: [udpSendTarget.trim()],
          timeout: const Duration(seconds: 45),
        );
        await tester.ensureRealPacketTunnelForSocketSend();
        await tester.sendUdpEcho(
          target: udpSendTarget.trim(),
          body: udpSendBody.trim(),
          timeout: const Duration(seconds: 12),
        );
      }
      if (tcpSendTarget.trim().isNotEmpty) {
        if (!checkSwitch) {
          fail(
              'SLAN_TEST_TCP_SEND_TARGET requires SLAN_TEST_CHECK_SWITCH=true');
        }
        await tester.pumpUntilSocketTargetsReady(
          targets: [tcpSendTarget.trim()],
          timeout: const Duration(seconds: 45),
        );
        await tester.ensureRealPacketTunnelForSocketSend();
        await tester.sendTcpEcho(
          target: tcpSendTarget.trim(),
          body: tcpSendBody.trim(),
          timeout: const Duration(seconds: 12),
        );
      }

      await tester.performLocalApiClientMessageCheckIfRequested(
        bridge,
        messageCheck,
      );

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
    } finally {}
  });
}

Future<void> _logAndroidRuntimeStats(String label) async {
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

Future<void> _registerTestUser(
  String bizUrl,
  String email,
  String password,
) async {
  try {
    final bridge = MethodChannelClientCoreBridge();
    await bridge.updateServerBaseUrl(bizUrl);
    final result = await bridge.requestLocalApi(
      'localRegisterTestUser',
      {
        'email': email,
        'password': password,
      },
    );
    final accessToken = (result?['accessToken'] as String? ?? '').trim();
    final deviceId = (result?['deviceId'] as String? ?? '').trim();
    final auth = result?['auth'];
    final session = auth is Map ? auth['session'] : null;
    final nestedToken =
        session is Map ? '${session['token'] ?? ''}'.trim() : '';
    if (accessToken.isNotEmpty ||
        deviceId.isNotEmpty ||
        nestedToken.isNotEmpty) {
      return;
    }
    debugPrint('SLAN_TEST_REGISTER_FALLBACK_LOCAL_API_EMPTY=$result');
  } on Object catch (error) {
    debugPrint('SLAN_TEST_REGISTER_FALLBACK_LOCAL_API_ERROR=$error');
  }

  final uri = Uri.parse(bizUrl).resolve('/api/app/auth/register');
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
  } finally {
    client.close(force: true);
  }
}

extension on WidgetTester {
  Future<void> configureRelayTransportAllowlist(String rawAllowlist) async {
    final transports = rawAllowlist
        .split(',')
        .map((value) => value.trim())
        .where((value) => value.isNotEmpty)
        .toList();
    if (transports.isEmpty) {
      return;
    }
    final response =
        await ClientCorePlugin().embeddedServiceRequest(jsonEncode({
      'method': 'localSetRelayTransportAllowlist',
      'args': {
        'transports': transports,
      },
    }));
    debugPrint(
      'SLAN_TEST_RELAY_TRANSPORT_ALLOWLIST_RESPONSE='
      '${jsonEncode(response ?? <String, Object?>{})}',
    );
  }

  Future<void> logRelayCandidates({
    bool refresh = false,
    String prefix = 'SLAN_TEST_RELAY_CANDIDATES',
  }) async {
    final response =
        await ClientCorePlugin().embeddedServiceRequest(jsonEncode({
      'method':
          refresh ? 'localRefreshRelayCandidates' : 'localRelayCandidates',
      'args': const <String, Object?>{},
    }));
    debugPrint('$prefix=${jsonEncode(response ?? <String, Object?>{})}');
  }

  Future<void> ensureAndroidNetworkAuthorizationReady({
    required Duration timeout,
  }) async {
    if (!Platform.isAndroid) {
      await _dismissFailureDialogIfPresent();
      final authorizationButton = find.text('授权');
      if (any(authorizationButton)) {
        await ensureVisible(authorizationButton);
        await tap(authorizationButton, warnIfMissed: false);
        await pump(const Duration(seconds: 3));
      }
      return;
    }
    final end = DateTime.now().add(timeout);
    while (DateTime.now().isBefore(end)) {
      await _dismissFailureDialogIfPresent();
      if (_androidConfigReadyUiSignal()) {
        return;
      }
      if (await _androidAuthorizationAlreadyPrepared()) {
        return;
      }
      final authorizationButton = find.text('授权');
      if (any(authorizationButton)) {
        await ensureVisible(authorizationButton);
        await tap(authorizationButton, warnIfMissed: false);
        debugPrint('SLAN_TEST_ANDROID_AUTH_TAP=requested');
        await pump(const Duration(seconds: 1));
        await _waitForAndroidAuthorizationProgress(
          timeout: const Duration(seconds: 6),
        );
        continue;
      }
      await pump(const Duration(milliseconds: 250));
    }
    await _dismissFailureDialogIfPresent();
    if (_androidConfigReadyUiSignal() ||
        await _androidAuthorizationAlreadyPrepared()) {
      return;
    }
    final authorizationButton = find.text('授权');
    if (any(authorizationButton)) {
      await ensureVisible(authorizationButton);
      await tap(authorizationButton, warnIfMissed: false);
      debugPrint('SLAN_TEST_ANDROID_AUTH_TAP=final');
      await pump(const Duration(seconds: 1));
      await _waitForAndroidAuthorizationProgress(
        timeout: const Duration(seconds: 6),
      );
    }
  }

  Future<void> _dismissFailureDialogIfPresent() async {
    final confirmButton = find.text('确定');
    if (!any(confirmButton)) {
      return;
    }
    await ensureVisible(confirmButton);
    await tap(confirmButton, warnIfMissed: false);
    await pump(const Duration(seconds: 1));
  }

  Future<void> _waitForAndroidAuthorizationProgress({
    required Duration timeout,
  }) async {
    final end = DateTime.now().add(timeout);
    while (DateTime.now().isBefore(end)) {
      await _dismissFailureDialogIfPresent();
      if (_androidConfigReadyUiSignal() ||
          await _androidAuthorizationAlreadyPrepared()) {
        return;
      }
      final authorizationButton = find.text('授权');
      if (!any(authorizationButton)) {
        await pump(const Duration(milliseconds: 500));
        continue;
      }
      await pump(const Duration(milliseconds: 500));
    }
  }

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

  String? signedInEmailText() {
    final emailFinder = find.byKey(const Key('current-user-email-value'));
    if (!any(emailFinder)) {
      return null;
    }
    return widget<Text>(emailFinder).data?.trim();
  }

  Future<void> pumpUntilNetworkEnabledOrFailed({
    required Duration timeout,
  }) async {
    final failure = find.text('操作失败');
    var attemptedAndroidAuthorizationRecovery = false;
    final end = DateTime.now().add(timeout);
    while (DateTime.now().isBefore(end)) {
      await pump(const Duration(milliseconds: 500));
      final ipText = networkIpText();
      if (ipText != null) {
        if (Platform.isAndroid) {
          final runtime = await ClientCorePlugin().androidRuntimeState();
          debugPrint(
            'SLAN_TEST_ANDROID_ENABLE_RUNTIME_CHECK ip=$ipText state=${jsonEncode(runtime)}',
          );
        }
        if (await _platformNetworkRuntimeReady()) {
          return;
        }
      }
      if (hasEnabledUiSignal()) {
        if (Platform.isAndroid) {
          final runtime = await ClientCorePlugin().androidRuntimeState();
          debugPrint(
            'SLAN_TEST_ANDROID_ENABLE_TEXT_RUNTIME_CHECK state=${jsonEncode(runtime)}',
          );
        }
        if (await _platformNetworkRuntimeReady()) {
          return;
        }
      }
      if (!Platform.isAndroid && _androidConfigReadyUiSignal()) {
        return;
      }
      if (any(failure)) {
        final texts = widgetList<Text>(find.byType(Text))
            .map((widget) => widget.data)
            .whereType<String>()
            .toList();
        final needsAndroidAuthorization = Platform.isAndroid &&
            texts.any(
              (text) =>
                  text.contains('Android 网络需要授权后才能启用') ||
                  text.contains('Android VPN permission requested'),
            );
        if (needsAndroidAuthorization &&
            !attemptedAndroidAuthorizationRecovery) {
          attemptedAndroidAuthorizationRecovery = true;
          final confirmButton = find.text('确定');
          if (any(confirmButton)) {
            await ensureVisible(confirmButton);
            await tap(confirmButton, warnIfMissed: false);
            await pump(const Duration(seconds: 1));
          }
          final authorizationButton = find.text('授权');
          if (any(authorizationButton)) {
            await ensureVisible(authorizationButton);
            await tap(authorizationButton, warnIfMissed: false);
            await pump(const Duration(seconds: 3));
          } else {
            await pump(const Duration(seconds: 2));
          }
          if (!await isNetworkEffectivelyEnabled()) {
            await tap(find.byKey(const Key('network-switch')));
            await pump();
          }
          attemptedAndroidAuthorizationRecovery = false;
          continue;
        }
        final textShowsEnabled = texts.contains('网络已启用') &&
            texts.any((text) => _ipv4Pattern.hasMatch(text));
        final runtimeReady = await _platformNetworkRuntimeReady();
        if (texts.any((text) => text.contains('already bound')) &&
            (ipText != null ||
                hasEnabledUiSignal() ||
                (!Platform.isAndroid && _androidConfigReadyUiSignal()) ||
                textShowsEnabled ||
                runtimeReady ||
                await isNetworkEffectivelyEnabled())) {
          debugPrint(
            'SLAN_TEST_ANDROID_ALREADY_BOUND_TREATED_AS_ENABLED '
            'texts=${texts.join(' | ')}',
          );
          return;
        }
        fail('network switch failed UI text: ${texts.join(' | ')}');
      }
    }
    await pumpAndSettle();
    final texts = widgetList<Text>(find.byType(Text))
        .map((widget) => widget.data)
        .whereType<String>()
        .toList();
    fail('network switch did not enable before timeout: ${texts.join(' | ')}');
  }

  Future<bool> _platformNetworkRuntimeReady() async {
    if (Platform.isAndroid) {
      final state = await ClientCorePlugin().androidRuntimeState();
      return state is Map &&
          state['networkEnabled'] == true &&
          state['adapterPresent'] == true;
    }
    if (Platform.isIOS) {
      final state = await ClientCorePlugin().iosRuntimeState();
      return state is Map &&
          state['networkEnabled'] == true &&
          state['adapterPresent'] == true;
    }
    return true;
  }

  Future<bool> _androidAuthorizationAlreadyPrepared() async {
    if (!Platform.isAndroid) {
      return true;
    }
    final state = await ClientCorePlugin().androidRuntimeState();
    if (state is! Map) {
      return false;
    }
    if (state['adapterPresent'] == true || state['networkEnabled'] == true) {
      return true;
    }
    final permissionState = '${state['permissionState'] ?? ''}'.trim();
    final authorizationReady = state['authorizationReady'] == true;
    return authorizationReady ||
        permissionState == 'authorized' ||
        permissionState == 'granted' ||
        permissionState == 'prepared';
  }

  Future<bool> isNetworkEffectivelyEnabled() async {
    if (Platform.isAndroid) {
      return _platformNetworkRuntimeReady();
    }
    if (networkIpText() != null) {
      return true;
    }
    if (hasEnabledUiSignal()) {
      return true;
    }
    if (_androidConfigReadyUiSignal()) {
      return true;
    }
    return _platformNetworkRuntimeReady();
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
      for (final text in texts) {
        final match = _ipv4Pattern.firstMatch(text);
        if (match != null) {
          return match.group(0);
        }
      }
    }
    for (final text in texts) {
      final cidrMatch = _ipv4WithPrefixPattern.firstMatch(text);
      if (cidrMatch != null) {
        return cidrMatch.group(0)?.split('/').first;
      }
    }
    return null;
  }

  bool hasEnabledUiSignal() {
    final texts = widgetList<Text>(find.byType(Text))
        .map((widget) => widget.data)
        .whereType<String>()
        .toList();
    return texts.contains('网络已启用') &&
        texts.any((text) => _ipv4Pattern.hasMatch(text));
  }

  bool _androidConfigReadyUiSignal() {
    if (!Platform.isAndroid) {
      return false;
    }
    final texts = widgetList<Text>(find.byType(Text))
        .map((widget) => widget.data)
        .whereType<String>()
        .toList();
    final hasReadyText = texts.any(
      (text) => text.contains('Android 网络配置已就绪') || text.contains('网络配置已就绪'),
    );
    final hasAddress = texts.any(
      (text) =>
          _ipv4Pattern.hasMatch(text) || _ipv4WithPrefixPattern.hasMatch(text),
    );
    return hasReadyText && hasAddress;
  }

  Future<void> sendClientMessageViaLocalApi(
    MethodChannelClientCoreBridge bridge, {
    required String targetDeviceId,
    required String body,
  }) async {
    await bridge.requestLocalApi(
      'localSendClientMessage',
      {
        'targetDeviceId': targetDeviceId,
        'body': body,
      },
    );
  }

  Future<void> performLocalApiClientMessageCheckIfRequested(
    MethodChannelClientCoreBridge bridge,
    _LocalApiClientMessageCheckConfig config,
  ) async {
    if (config.hasSend) {
      await sendClientMessageViaLocalApi(
        bridge,
        targetDeviceId: config.trimmedSendTargetDeviceId,
        body: config.trimmedSendBody,
      );
      debugPrint(
        'SLAN_TEST_CLIENT_MESSAGE_SENT='
        '${config.trimmedSendTargetDeviceId}:${config.trimmedSendBody}',
      );
    }

    if (!config.hasExpect) {
      return;
    }
    await pumpUntilMqttConnected(
      bridge,
      timeout: Duration(seconds: config.expectMqttTimeoutSeconds),
    );
    await pumpUntilLocalApiClientMessage(
      bridge,
      fromDeviceId: config.trimmedExpectFromDeviceId,
      body: config.trimmedExpectBody,
      timeout: Duration(seconds: config.expectMessageTimeoutSeconds),
    );
  }

  Future<void> pumpUntilLocalApiClientMessage(
    MethodChannelClientCoreBridge bridge, {
    required String fromDeviceId,
    required String body,
    required Duration timeout,
  }) async {
    final expectedFrom = fromDeviceId.trim();
    final expectedBody = body.trim();
    if (expectedFrom.isEmpty && expectedBody.isEmpty) {
      fail(
        'pumpUntilLocalApiClientMessage requires fromDeviceId or body to be set',
      );
    }
    final end = DateTime.now().add(timeout);
    Map<String, Object?>? lastState;
    while (DateTime.now().isBefore(end)) {
      await pump(const Duration(milliseconds: 500));
      final state = await bridge.requestLocalApi('localState');
      lastState = state;
      final actualFrom =
          '${state?['lastClientMessageFromDeviceId'] ?? ''}'.trim();
      final actualBody = '${state?['lastClientMessageBody'] ?? ''}'.trim();
      final fromMatches = expectedFrom.isEmpty || actualFrom == expectedFrom;
      final bodyMatches = expectedBody.isEmpty || actualBody == expectedBody;
      if (fromMatches && bodyMatches) {
        debugPrint(
          'SLAN_TEST_CLIENT_MESSAGE_OK='
          '$actualFrom:$actualBody',
        );
        return;
      }
    }
    fail(
      'client message not received: expectedFrom="$expectedFrom" '
      'expectedBody="$expectedBody" '
      'state="$lastState"',
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

  Future<void> pumpUntilSocketTargetsReady({
    required List<String> targets,
    required Duration timeout,
  }) async {
    final plugin = ClientCorePlugin();
    final hosts = targets
        .map((target) => _targetHost(target))
        .where((host) => host.isNotEmpty)
        .toSet()
        .toList();
    if (hosts.isEmpty) {
      fail('socket targets must contain at least one host');
    }
    final pendingDnsHosts =
        hosts.where((host) => InternetAddress.tryParse(host) == null).toSet();
    final deadline = DateTime.now().add(timeout);
    Map<String, Object?>? lastSnapshot;
    Object? lastRuntime;
    while (DateTime.now().isBefore(deadline)) {
      await pump(const Duration(milliseconds: 500));
      final snapshot = await plugin.embeddedServiceRequest(jsonEncode({
        'method': 'localNetworkModule',
        'args': <String, Object?>{},
      }));
      lastSnapshot = snapshot;
      final resolvedHosts = <String>{};
      var peerCount = 0;
      final configs = snapshot?['configs'];
      if (configs is List) {
        for (final config in configs) {
          if (config is! Map) {
            continue;
          }
          final peers = config['peers'];
          if (peers is List) {
            peerCount += peers.whereType<Map>().length;
          }
          final dnsRecords = config['dnsRecords'];
          if (dnsRecords is! List) {
            continue;
          }
          for (final record in dnsRecords) {
            if (record is! Map) {
              continue;
            }
            final fqdn = '${record['fqdn'] ?? ''}'.trim().toLowerCase();
            final name = '${record['name'] ?? ''}'.trim().toLowerCase();
            final targetIp = '${record['targetIp'] ?? ''}'.trim();
            final targetDeviceId = '${record['targetDeviceId'] ?? ''}'.trim();
            final targetReachable =
                InternetAddress.tryParse(targetIp) != null ||
                    targetDeviceId.isNotEmpty;
            if (!targetReachable) {
              continue;
            }
            for (final host in pendingDnsHosts) {
              final normalized = host.toLowerCase();
              if (fqdn == normalized || name == normalized) {
                resolvedHosts.add(host);
              }
            }
          }
        }
      }
      final dnsReady = pendingDnsHosts.every(resolvedHosts.contains);
      var pathReady = true;
      if (Platform.isAndroid) {
        lastRuntime = await plugin.androidRuntimeState();
        if (lastRuntime is Map) {
          pathReady = _androidSocketTargetsReadyForSend(lastRuntime);
        }
      }
      if (peerCount > 0 && dnsReady && pathReady) {
        debugPrint(
          'SLAN_TEST_SOCKET_TARGETS_READY='
          'hosts=${hosts.join(",")} peerCount=$peerCount '
          'resolvedDns=${resolvedHosts.join(",")} runtime=${jsonEncode(lastRuntime)}',
        );
        return;
      }
    }
    fail(
      'socket targets not ready before timeout: '
      'targets=$targets pendingDns=${pendingDnsHosts.join(",")} '
      'lastSnapshot=$lastSnapshot lastRuntime=$lastRuntime',
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
        final sent = socket.send(payload, current.address, current.port);
        debugPrint(
          'SLAN_TEST_UDP_ECHO_SENT=${current.address.address}:${current.port} '
          'bytes=$sent body=${utf8.decode(payload)}',
        );
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
      final targetAddress = await _resolveTargetAddress(host);
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
        socket.send(utf8.encode(body), targetAddress, port);
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

  String _targetHost(String target) {
    final separator = target.lastIndexOf(':');
    if (separator <= 0 || separator == target.length - 1) {
      fail('socket target must be host:port, got $target');
    }
    return target.substring(0, separator).trim();
  }

  Future<InternetAddress> _resolveTargetAddress(String host) async {
    final trimmed = host.trim();
    if (trimmed.isEmpty) {
      fail('target host is empty');
    }
    final direct = InternetAddress.tryParse(trimmed);
    if (direct != null) {
      return direct;
    }
    final embeddedResolved =
        await _resolveTargetAddressFromEmbeddedNetworkModule(
      trimmed,
    );
    if (embeddedResolved != null) {
      return embeddedResolved;
    }
    final resolved = await InternetAddress.lookup(trimmed);
    final ipv4 =
        resolved.where((address) => address.type == InternetAddressType.IPv4);
    if (ipv4.isNotEmpty) {
      return ipv4.first;
    }
    if (resolved.isNotEmpty) {
      return resolved.first;
    }
    fail('failed to resolve target host: $trimmed');
  }

  Future<InternetAddress?> _resolveTargetAddressFromEmbeddedNetworkModule(
    String host,
  ) async {
    final plugin = ClientCorePlugin();
    final snapshot = await plugin.embeddedServiceRequest(jsonEncode({
      'method': 'localNetworkModule',
      'args': <String, Object?>{},
    }));
    final configs = snapshot?['configs'];
    if (configs is! List) {
      return null;
    }
    for (final config in configs) {
      if (config is! Map) {
        continue;
      }
      final dnsRecords = config['dnsRecords'];
      if (dnsRecords is! List) {
        continue;
      }
      final peers = config['peers'];
      final peerIpByDeviceId = <String, String>{};
      if (peers is List) {
        for (final peer in peers) {
          if (peer is! Map) {
            continue;
          }
          final deviceId = '${peer['deviceId'] ?? ''}'.trim();
          if (deviceId.isEmpty) {
            continue;
          }
          final virtualIps = peer['virtualIps'];
          if (virtualIps is List) {
            for (final value in virtualIps) {
              final ip = '$value'.trim();
              if (InternetAddress.tryParse(ip) != null) {
                peerIpByDeviceId[deviceId] = ip;
                break;
              }
            }
          }
          peerIpByDeviceId.putIfAbsent(
            deviceId,
            () => '${peer['globalIp'] ?? ''}'.trim(),
          );
        }
      }
      for (final record in dnsRecords) {
        if (record is! Map) {
          continue;
        }
        final fqdn = '${record['fqdn'] ?? ''}'.trim().toLowerCase();
        final name = '${record['name'] ?? ''}'.trim().toLowerCase();
        if (fqdn != host.toLowerCase() && name != host.toLowerCase()) {
          continue;
        }
        final targetIp = '${record['targetIp'] ?? ''}'.trim();
        if (InternetAddress.tryParse(targetIp) case final InternetAddress ip?) {
          debugPrint('SLAN_TEST_EMBEDDED_DNS_RESOLVE=$host->$targetIp');
          return ip;
        }
        final targetDeviceId = '${record['targetDeviceId'] ?? ''}'.trim();
        final peerIp = peerIpByDeviceId[targetDeviceId]?.trim() ?? '';
        if (InternetAddress.tryParse(peerIp) case final InternetAddress ip?) {
          debugPrint(
            'SLAN_TEST_EMBEDDED_DNS_RESOLVE=$host->$peerIp deviceId=$targetDeviceId',
          );
          return ip;
        }
      }
    }
    return null;
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
          debugPrint(
            'SLAN_TEST_TCP_ECHO_SENT=${socket.remoteAddress.address}:${socket.remotePort} body=echo:$body',
          );
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
    final targetAddress = await _resolveTargetAddress(host);
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
        socket = await Socket.connect(
          targetAddress,
          port,
          timeout: connectTimeout,
        );
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
    if (Platform.isAndroid) {
      final plugin = ClientCorePlugin();
      final end = DateTime.now().add(const Duration(seconds: 45));
      Object? state = await plugin.androidRuntimeState();
      while (DateTime.now().isBefore(end)) {
        if (state is Map &&
            state['networkEnabled'] == true &&
            state['adapterPresent'] == true &&
            _androidDataPathReadyForSocketSend(state)) {
          debugPrint(
            'SLAN_TEST_ANDROID_PACKET_TUNNEL_READY=${jsonEncode(state)}',
          );
          return;
        }
        await pump(const Duration(milliseconds: 500));
        state = await plugin.androidRuntimeState();
      }
      debugPrint('SLAN_TEST_ANDROID_PACKET_TUNNEL_WAIT_TIMEOUT=$state');
      await logPlatformTunnelState(
          prefix: 'SLAN_TEST_ANDROID_PACKET_TUNNEL_WAIT');
      fail('Android packet tunnel data path not ready for socket send: $state');
    }
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

  bool _androidDataPathReadyForSocketSend(Map<dynamic, dynamic> state) {
    return _androidSocketTargetsReadyForSend(state);
  }

  bool _androidSocketTargetsReadyForSend(Map<dynamic, dynamic> state) {
    final requestedRelaySessions =
        (state['requestedRelaySessionCount'] as num?)?.toInt() ?? 0;
    final attachedPeers =
        (state['directUdpAttachedPeerCount'] as num?)?.toInt() ?? 0;
    final readyPeers = (state['directUdpReadyPeerCount'] as num?)?.toInt() ?? 0;
    final relaySessions = (state['relaySessionCount'] as num?)?.toInt() ?? 0;
    final noPeerPackets = (state['relayNoPeerPackets'] as num?)?.toInt() ?? 0;
    final attachedRelaySessions =
        (state['attachedRelaySessionCount'] as num?)?.toInt() ?? 0;
    final lastNoPeerPacket = '${state['lastNoPeerPacket'] ?? ''}'.trim();
    final relayFramesReceived =
        (state['relayFramesReceived'] as num?)?.toInt() ?? 0;
    final directUdpFramesReceived =
        (state['directUdpFramesReceived'] as num?)?.toInt() ?? 0;
    final relayControlPacketsReceived =
        (state['relayControlPacketsReceived'] as num?)?.toInt() ?? 0;
    final lastRelayControlKind =
        '${state['lastRelayControlKind'] ?? ''}'.trim();
    final hasRelayTransportSignal = relayFramesReceived > 0 ||
        relayControlPacketsReceived > 0 ||
        lastRelayControlKind.isNotEmpty;
    if (readyPeers > 0) {
      return true;
    }
    if (attachedPeers > 0 &&
        directUdpFramesReceived > 0 &&
        noPeerPackets <= 0) {
      return true;
    }
    if (requestedRelaySessions <= 0) {
      return attachedPeers > 0 || directUdpFramesReceived > 0;
    }
    if (relaySessions > 0 &&
        attachedRelaySessions > 0 &&
        hasRelayTransportSignal &&
        noPeerPackets <= 0) {
      return true;
    }
    if (_isBenignAndroidNoPeerPacket(lastNoPeerPacket) &&
        relaySessions > 0 &&
        attachedRelaySessions > 0 &&
        hasRelayTransportSignal) {
      return true;
    }
    return false;
  }

  bool _isBenignAndroidNoPeerPacket(String packetSummary) {
    if (packetSummary.isEmpty) {
      return false;
    }
    if (!packetSummary.contains('firstByte=0x60')) {
      return false;
    }
    return !_virtualIpv4Pattern.hasMatch(packetSummary);
  }
}
