import 'dart:convert';
import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:integration_test/integration_test.dart';
import 'package:slan_client_v2/app/slan_client_v2_app.dart';
import 'package:slan_client_v2/bridge/client_core_bridge.dart';

void main() {
  IntegrationTestWidgetsFlutterBinding.ensureInitialized();

  testWidgets('desktop password login updates the running client',
      (tester) async {
    const bizBaseUrl = String.fromEnvironment(
      'SLAN_TEST_BIZ_URL',
      defaultValue: 'http://47.245.40.231:28080',
    );
    const password = String.fromEnvironment(
      'SLAN_TEST_PASSWORD',
      defaultValue: 'Password123!',
    );
    const opsBaseUrl = String.fromEnvironment(
      'SLAN_TEST_OPS_URL',
      defaultValue: 'http://47.245.40.231:24201',
    );
    final email =
        'desktop-password-${DateTime.now().microsecondsSinceEpoch}@example.test';
    await _createUser(opsBaseUrl, email, password);

    final settingsDirectory = await Directory.systemTemp.createTemp(
      'slan-desktop-login-test-',
    );
    addTearDown(() => settingsDirectory.delete(recursive: true));
    final bridge = MethodChannelClientCoreBridge(
      desktopSettingsFile: File('${settingsDirectory.path}/client-ui.json'),
    );
    addTearDown(bridge.close);
    await bridge.updateServerBaseUrl(bizBaseUrl);
    await tester.pumpWidget(SlanClientV2App(bridge: bridge));
    await tester.pumpAndSettle(const Duration(seconds: 1));

    expect(find.byKey(const Key('desktop-browser-login')), findsNothing);
    expect(find.byKey(const Key('server-settings')), findsOneWidget);
    await tester.enterText(find.byKey(const Key('login-email')), email);
    await tester.enterText(find.byKey(const Key('login-password')), password);
    await tester.tap(find.byKey(const Key('login-submit')));

    final deadline = DateTime.now().add(const Duration(seconds: 30));
    while (DateTime.now().isBefore(deadline) && !bridge.state.value.signedIn) {
      await tester.pump(const Duration(milliseconds: 250));
    }

    expect(bridge.state.value.signedIn, isTrue);
    expect(bridge.state.value.userLabel, email);
    expect(find.byKey(const Key('open-web-console')), findsNothing);
    expect(find.byKey(const Key('accept-network-invite')), findsNothing);
  });
}

Future<void> _createUser(
  String baseUrl,
  String email,
  String password,
) async {
  final client = HttpClient()..connectionTimeout = const Duration(seconds: 5);
  try {
    const opsEmail = String.fromEnvironment(
      'SLAN_TEST_OPS_EMAIL',
      defaultValue: 'admin1',
    );
    const opsPassword = String.fromEnvironment(
      'SLAN_TEST_OPS_PASSWORD',
      defaultValue: 'admin1',
    );
    final login = await _postJSON(
        client,
        Uri.parse(baseUrl).resolve('/api/ops/auth/login'),
        null,
        {'email': opsEmail, 'password': opsPassword});
    final token = '${login['token'] ?? ''}'.trim();
    if (token.isEmpty) fail('operator login returned no token');
    await _postJSON(client, Uri.parse(baseUrl).resolve('/api/ops/users'), token,
        {'email': email, 'password': password, 'name': email});
  } finally {
    client.close(force: true);
  }
}

Future<Map<String, dynamic>> _postJSON(HttpClient client, Uri uri,
    String? token, Map<String, Object?> body) async {
  final request = await client.postUrl(uri);
  request.headers.contentType = ContentType.json;
  if (token != null) request.headers.set('Authorization', 'Bearer $token');
  request.write(jsonEncode(body));
  final response = await request.close().timeout(const Duration(seconds: 15));
  final responseBody = await response.transform(utf8.decoder).join();
  if (response.statusCode < 200 || response.statusCode >= 300) {
    fail('POST $uri failed: HTTP ${response.statusCode}: $responseBody');
  }
  return (jsonDecode(responseBody) as Map).cast<String, dynamic>();
}
