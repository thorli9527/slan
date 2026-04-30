import 'package:flutter_test/flutter_test.dart';
import 'package:slan_app/infra/app_core/scope/app_host_config.dart';

void main() {
  test('authLoginUrl rewrites console path to root login entrypoint', () {
    final config =
        AppHostConfig.tryParse('https://demo.slan.example:4200/console');

    expect(config, isNotNull);
    expect(
      config!.authLoginUrl,
      'https://demo.slan.example:4200/?auth=login',
    );
  });

  test('secure localhost host resolves to local control and web endpoints', () {
    final config = AppHostConfig.tryParse('slan.localhost');

    expect(config, isNotNull);
    expect(config!.controlBaseUrl, 'http://127.0.0.1:28080');
    expect(config.webConsoleUrl, 'https://web.slan.localhost:18443');
    expect(config.authLoginUrl, 'https://web.slan.localhost:18443/?auth=login');
  });

  test('loopback host resolves to local HTTP web endpoint', () {
    final config = AppHostConfig.tryParse('127.0.0.1');

    expect(config, isNotNull);
    expect(config!.controlBaseUrl, 'http://127.0.0.1:28080');
    expect(config.webConsoleUrl, 'http://127.0.0.1:24200');
    expect(config.authLoginUrl, 'http://127.0.0.1:24200/?auth=login');
  });
}
