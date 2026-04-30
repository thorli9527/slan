library slan_app.infra.app_core.scope.host_config;

final class AppHostConfig {
  static const _localControlBaseUrl = 'http://127.0.0.1:28080';
  static const _localWebBaseUrl = 'http://127.0.0.1:24200';
  static const _localWebHost = 'web.slan.localhost';
  static const _localCaddyPort = 18443;

  AppHostConfig({
    required this.rawInput,
    required this.displayHost,
    required this.controlBaseUrl,
    required this.webConsoleUrl,
    required this.secure,
  });

  final String rawInput;
  final String displayHost;
  final String controlBaseUrl;
  final String webConsoleUrl;
  final bool secure;

  String get authLoginUrl => Uri.parse(webConsoleUrl).replace(
        path: '/',
        queryParameters: const {'auth': 'login'},
      ).toString();

  static AppHostConfig? tryParse(String? raw) {
    final normalized = raw?.trim() ?? '';
    if (normalized.isEmpty) {
      return null;
    }

    final explicitUri = Uri.tryParse(normalized);
    if (explicitUri != null &&
        explicitUri.hasScheme &&
        explicitUri.host.isNotEmpty) {
      return _fromExplicitUri(explicitUri, normalized);
    }

    final host = normalized;
    if (host.isEmpty) {
      return null;
    }
    if (_isLoopbackHost(host)) {
      return AppHostConfig(
        rawInput: normalized,
        displayHost: host,
        controlBaseUrl: _localControlBaseUrl,
        webConsoleUrl: _localWebBaseUrl,
        secure: false,
      );
    }
    final secure = host == 'slan.localhost' || host.endsWith('.slan.localhost');
    if (secure) {
      return AppHostConfig(
        rawInput: normalized,
        displayHost: host,
        controlBaseUrl: _localControlBaseUrl,
        webConsoleUrl: 'https://$_localWebHost:$_localCaddyPort',
        secure: false,
      );
    }
    final scheme = secure ? 'https' : 'http';
    final controlPort = secure ? 18443 : 28080;
    final webPort = secure ? 18443 : 24200;
    final webHost = secure ? 'web.slan.localhost' : host;
    return AppHostConfig(
      rawInput: normalized,
      displayHost: host,
      controlBaseUrl: '$scheme://$host:$controlPort',
      webConsoleUrl: '$scheme://$webHost:$webPort',
      secure: secure,
    );
  }

  static AppHostConfig _fromExplicitUri(Uri uri, String rawInput) {
    if (_isLoopbackHost(uri.host)) {
      return AppHostConfig(
        rawInput: rawInput,
        displayHost: uri.host,
        controlBaseUrl: _localControlBaseUrl,
        webConsoleUrl: _localWebBaseUrl,
        secure: false,
      );
    }
    if (uri.host == 'slan.localhost' || uri.host == 'web.slan.localhost') {
      return AppHostConfig(
        rawInput: rawInput,
        displayHost: uri.host,
        controlBaseUrl: _localControlBaseUrl,
        webConsoleUrl: 'https://$_localWebHost:$_localCaddyPort',
        secure: false,
      );
    }
    final secure = uri.scheme == 'https';
    final controlUrl =
        uri.replace(path: '', query: null, fragment: null).toString();
    final webUri = uri.host == 'slan.localhost'
        ? uri.replace(
            host: 'web.slan.localhost',
            path: '',
            query: null,
            fragment: null,
          )
        : uri.replace(
            port: secure ? 4200 : 24200,
            path: '',
            query: null,
            fragment: null,
          );
    return AppHostConfig(
      rawInput: rawInput,
      displayHost: uri.host,
      controlBaseUrl: controlUrl,
      webConsoleUrl: webUri.toString(),
      secure: secure,
    );
  }

  static bool _isLoopbackHost(String host) {
    final normalized = host.trim().toLowerCase();
    return normalized == '127.0.0.1' ||
        normalized == 'localhost' ||
        normalized == '::1';
  }
}
