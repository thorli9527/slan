import 'dart:convert';
import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:integration_test/integration_test.dart';
import 'package:slan_client_v2/app/slan_client_v2_app.dart';
import 'package:slan_client_v2/bridge/client_core_bridge.dart';

void main() {
  IntegrationTestWidgetsFlutterBinding.ensureInitialized();

  testWidgets('desktop browser login updates the running client',
      (tester) async {
    const webBaseUrl = String.fromEnvironment(
      'SLAN_TEST_WEB_BASE_URL',
      defaultValue: 'http://47.245.40.231:24200',
    );
    const password = String.fromEnvironment(
      'SLAN_TEST_PASSWORD',
      defaultValue: 'Password123!',
    );
    final email =
        'desktop-browser-${DateTime.now().microsecondsSinceEpoch}@example.test';
    final bridge = MethodChannelClientCoreBridge();

    await tester.pumpWidget(SlanClientV2App(bridge: bridge));
    await tester.pumpAndSettle(const Duration(seconds: 1));

    final loginButton = find.byKey(const Key('desktop-browser-login'));
    expect(loginButton, findsOneWidget);
    await tester.ensureVisible(loginButton);
    await tester.tap(loginButton);
    await tester.pump(const Duration(seconds: 1));

    final deviceId = await _waitForDeviceId(bridge);
    final accessToken = await _registerWebUser(
      webBaseUrl,
      email,
      password,
    );
    await _completeDeviceLogin(webBaseUrl, deviceId, accessToken);

    final deadline = DateTime.now().add(const Duration(seconds: 30));
    while (DateTime.now().isBefore(deadline) && !bridge.state.value.signedIn) {
      await tester.pump(const Duration(milliseconds: 250));
    }

    expect(bridge.state.value.signedIn, isTrue);
    expect(bridge.state.value.userLabel, email);
    expect(loginButton, findsNothing);
    await bridge.close();
  });
}

Future<String> _waitForDeviceId(MethodChannelClientCoreBridge bridge) async {
  final deadline = DateTime.now().add(const Duration(seconds: 10));
  while (DateTime.now().isBefore(deadline)) {
    final deviceId = bridge.state.value.deviceId?.trim() ?? '';
    if (deviceId.isNotEmpty) {
      return deviceId;
    }
    await Future<void>.delayed(const Duration(milliseconds: 200));
  }
  fail('desktop login did not prepare a device id');
}

Future<String> _registerWebUser(
  String baseUrl,
  String email,
  String password,
) async {
  final response = await _postJson(
    Uri.parse(baseUrl).resolve('/api/web/auth/register'),
    {
      'email': email,
      'password': password,
      'name': 'Desktop Browser Login Test',
    },
  );
  final auth = response['auth'];
  final session = auth is Map ? auth['session'] : null;
  final token = session is Map
      ? '${session['token'] ?? session['accessToken'] ?? ''}'.trim()
      : '';
  if (token.isEmpty) {
    fail('web registration returned no access token: $response');
  }
  return token;
}

Future<void> _completeDeviceLogin(
  String baseUrl,
  String deviceId,
  String accessToken,
) async {
  await _postJson(
    Uri.parse(baseUrl)
        .resolve('/api/web/auth/device-login-devices/$deviceId/complete'),
    {'accessToken': accessToken},
  );
}

Future<Map<String, dynamic>> _postJson(
  Uri uri,
  Map<String, Object?> body,
) async {
  final client = HttpClient();
  client.connectionTimeout = const Duration(seconds: 5);
  try {
    final request = await client.postUrl(uri);
    request.headers.contentType = ContentType.json;
    request.write(jsonEncode(body));
    final response = await request.close().timeout(const Duration(seconds: 15));
    final responseBody = await response.transform(utf8.decoder).join();
    if (response.statusCode < 200 || response.statusCode >= 300) {
      fail('POST $uri failed: HTTP ${response.statusCode}: $responseBody');
    }
    return jsonDecode(responseBody) as Map<String, dynamic>;
  } finally {
    client.close(force: true);
  }
}
