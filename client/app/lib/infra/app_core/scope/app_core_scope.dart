/// AppCore 依赖注入入口。
///
/// 默认仍挂接 mock；联调时可切到真实控制面 HTTP 或 facade bridge。
library slan_app.infra.app_core.scope;

import 'dart:convert';
import 'dart:core';
import 'dart:async';

import 'package:flutter/foundation.dart';
import 'package:shared_preferences/shared_preferences.dart';

import '../../../application/tunnel_host_gateway.dart';
import '../api/app_core_api.dart';
import '../models/identity_models.dart';
import '../store/app_core_coordinator.dart';
import '../store/app_session_store.dart';
import '../store/app_tunnel_store.dart';
import '../store/app_session_controller.dart';
import '../store/app_tunnel_controller.dart';
import '../bridge/bridge_app_core_api.dart';
import '../api/http_app_core_api.dart';
import '../api/mock_app_core_api.dart';
import '../../logging/startup_log.dart';
import 'app_host_config.dart';

class AppCoreScope {
  AppCoreScope._();
  static const _hostPreferenceKey = 'slan.server_host';
  static const _clientMachineIdKey = 'slan.client_machine_id';
  static const _sessionPreferenceKey = 'slan.session';

  static const String _appCoreMode =
      String.fromEnvironment('SLAN_APP_CORE_MODE');
  static const String _serverHost = String.fromEnvironment('SLAN_SERVER_HOST');
  static const String _controlBaseUrl =
      String.fromEnvironment('SLAN_CONTROL_BASE_URL');
  static const String _serverUiUrl =
      String.fromEnvironment('SLAN_SERVER_UI_URL');
  static const String _tunnelHostMode =
      String.fromEnvironment('SLAN_TUNNEL_HOST_MODE');
  static const String _helperHostAddress =
      String.fromEnvironment('SLAN_APP_CORE_HELPER_HOST');
  static String? _runtimeHostInput = _serverHost.isEmpty ? null : _serverHost;
  static String? _runtimeControlBaseUrl =
      _controlBaseUrl.isEmpty ? null : _controlBaseUrl;
  static String? _runtimeServerUiUrl =
      _serverUiUrl.isEmpty ? null : _serverUiUrl;
  static String? _runtimeClientMachineId;

  static AppCoreApi _instance = _buildDefaultInstance();
  static final TunnelHostGateway _tunnelHostGateway = _buildTunnelHostGateway();
  static AppCoreCoordinator _coordinator =
      AppCoreCoordinator(hostGateway: _tunnelHostGateway);
  static AppSessionController _sessionController =
      AppSessionController(_coordinator);
  static AppTunnelController _tunnelController =
      AppTunnelController(_coordinator);

  static const _persistedSessionValid = _PersistedSessionValidationResult(
    isValid: true,
    shouldClearPersistedSession: false,
  );

  static Future<void> initialize() async {
    await StartupLog.write('app core initialize start mode=$_appCoreMode');
    await StartupLog.write(
      'tunnel host mode=${_resolvedTunnelHostMode()} helperHost=${_resolvedHelperHostAddress() ?? 'plugin'}',
    );
    if (_appCoreMode == 'bridge') {
      await StartupLog.write('app core initialize skipped: bridge mode');
      return;
    }
    final preferences = await SharedPreferences.getInstance();
    await StartupLog.write('preferences loaded');
    final persistedHost = preferences.getString(_hostPreferenceKey)?.trim();
    if (persistedHost != null && persistedHost.isNotEmpty) {
      _runtimeHostInput = persistedHost;
      _runtimeControlBaseUrl = null;
      _runtimeServerUiUrl = null;
      await StartupLog.write('persisted host restored: $_runtimeHostInput');
    }
    final persistedMachineId =
        preferences.getString(_clientMachineIdKey)?.trim();
    if (persistedMachineId != null && persistedMachineId.isNotEmpty) {
      _runtimeClientMachineId = persistedMachineId;
    }
    _runtimeClientMachineId ??=
        'client-${DateTime.now().microsecondsSinceEpoch}';
    _instance = _buildDefaultInstance();
    await StartupLog.write(
      'app core api ready controlBaseUrl=${controlBaseUrl ?? 'mock'} clientMachineId=$clientMachineId',
    );
    unawaited(_persistClientMachineId());
    final persistedSession =
        _readPersistedSession(preferences.getString(_sessionPreferenceKey));
    if (persistedSession != null) {
      await StartupLog.write(
        'persisted session found userId=${persistedSession.userId} deviceId=${persistedSession.deviceId}',
      );
      final validationResult = await _validatePersistedSession(persistedSession);
      await StartupLog.write(
        'persisted session validation result=${validationResult.isValid} clear=${validationResult.shouldClearPersistedSession}',
      );
      if (validationResult.isValid) {
        await _coordinator.applyExternalSessionInternal(
          persistedSession,
          persistSession: false,
        );
        await StartupLog.write('persisted session applied');
      } else {
        _coordinator.resetState();
        if (validationResult.shouldClearPersistedSession) {
          await clearPersistedSession();
          await StartupLog.write(
            'persisted session cleared after validation failure',
          );
        } else {
          await StartupLog.write(
            'persisted session skipped after transient validation failure',
          );
        }
        await _cleanupInactiveTunnelBackend('invalid persisted session');
      }
    } else {
      await StartupLog.write('no persisted session');
      await _cleanupInactiveTunnelBackend('no persisted session');
    }
    await StartupLog.write('app core initialize done');
  }

  static AppCoreApi get instance => _instance;
  static AppSessionStore get sessionStore => _coordinator.sessionStore;
  static AppTunnelStore get tunnelStore => _coordinator.tunnelStore;
  static AppSessionController get sessionController => _sessionController;
  static AppTunnelController get tunnelController => _tunnelController;
  static TunnelHostGateway get tunnelHostGateway => _tunnelHostGateway;
  static String get mode => _appCoreMode;
  static AppHostConfig? get hostConfig =>
      AppHostConfig.tryParse(_runtimeHostInput) ??
      AppHostConfig.tryParse(_runtimeControlBaseUrl);
  static String? get hostInput => _runtimeHostInput ?? hostConfig?.rawInput;
  static String? get controlBaseUrl =>
      hostConfig?.controlBaseUrl ?? _runtimeControlBaseUrl;
  static String? get webConsoleUrl =>
      _runtimeServerUiUrl ??
      hostConfig?.webConsoleUrl ??
      _deriveWebConsoleUrl(_runtimeControlBaseUrl);
  static String get clientMachineId => _runtimeClientMachineId ??=
      'client-${DateTime.now().microsecondsSinceEpoch}';

  static AppCoreApi _buildDefaultInstance() {
    final controlBaseUrl = AppCoreScope.controlBaseUrl;
    return switch (_appCoreMode) {
      'bridge' => BridgeAppCoreApi(),
      _ => controlBaseUrl == null || controlBaseUrl.isEmpty
          ? MockAppCoreApi()
          : HttpAppCoreApi(baseUrl: controlBaseUrl),
    };
  }

  static void configureHost({
    required String host,
  }) {
    if (_appCoreMode == 'bridge') {
      return;
    }
    final normalized = host.trim();
    _runtimeHostInput = normalized.isEmpty ? null : normalized;
    _runtimeControlBaseUrl = null;
    _runtimeServerUiUrl = null;
    _instance = _buildDefaultInstance();
    _coordinator.resetForServerSwitch();
    unawaited(_persistHost());
  }

  static void configureServer({
    required String baseUrl,
    String? webBaseUrl,
  }) {
    if (_appCoreMode == 'bridge') {
      return;
    }
    _runtimeHostInput = null;
    final normalized = baseUrl.trim();
    _runtimeControlBaseUrl = normalized.isEmpty ? null : normalized;
    final webNormalized = webBaseUrl?.trim() ?? '';
    _runtimeServerUiUrl = webNormalized.isEmpty ? null : webNormalized;
    _instance = _buildDefaultInstance();
    _coordinator.resetForServerSwitch();
    unawaited(_persistHost(clear: true));
  }

  @visibleForTesting
  static void configureForTest({
    required AppCoreApi appCoreApi,
  }) {
    _instance = appCoreApi;
    _resetStoreBindings();
  }

  @visibleForTesting
  static void resetForTest() {
    _runtimeHostInput = _serverHost.isEmpty ? null : _serverHost;
    _runtimeControlBaseUrl = _controlBaseUrl.isEmpty ? null : _controlBaseUrl;
    _runtimeServerUiUrl = _serverUiUrl.isEmpty ? null : _serverUiUrl;
    _runtimeClientMachineId = null;
    _instance = _buildDefaultInstance();
    _resetStoreBindings();
  }

  static void _resetStoreBindings() {
    _coordinator = AppCoreCoordinator(hostGateway: _tunnelHostGateway);
    _sessionController = AppSessionController(_coordinator);
    _tunnelController = AppTunnelController(_coordinator);
  }

  static TunnelHostGateway _buildTunnelHostGateway() {
    final mode = _resolvedTunnelHostMode();
    final helperHostAddress = _resolvedHelperHostAddress();
    if ((mode == 'service' || mode == 'helper' || mode == 'helper-host') &&
        helperHostAddress != null) {
      return HelperServiceTunnelHostGateway(address: helperHostAddress);
    }
    return const PluginTunnelHostGateway();
  }

  static String _resolvedTunnelHostMode() {
    final normalized = _tunnelHostMode.trim().toLowerCase();
    return normalized.isEmpty ? 'plugin' : normalized;
  }

  static String? _resolvedHelperHostAddress() {
    final normalized = _helperHostAddress.trim();
    if (normalized.isEmpty) {
      return null;
    }
    return normalized;
  }

  static Future<void> _persistHost({bool clear = false}) async {
    final preferences = await SharedPreferences.getInstance();
    if (clear || _runtimeHostInput == null || _runtimeHostInput!.isEmpty) {
      await preferences.remove(_hostPreferenceKey);
      return;
    }
    await preferences.setString(_hostPreferenceKey, _runtimeHostInput!);
  }

  static Future<void> _persistClientMachineId() async {
    final preferences = await SharedPreferences.getInstance();
    final value = _runtimeClientMachineId;
    if (value == null || value.isEmpty) {
      await preferences.remove(_clientMachineIdKey);
      return;
    }
    await preferences.setString(_clientMachineIdKey, value);
  }

  static Future<void> persistSession(SessionModel session) async {
    if (_appCoreMode == 'bridge') {
      return;
    }
    final preferences = await SharedPreferences.getInstance();
    await preferences.setString(
      _sessionPreferenceKey,
      jsonEncode(session.toJson()),
    );
  }

  static Future<void> clearPersistedSession() async {
    if (_appCoreMode == 'bridge') {
      return;
    }
    final preferences = await SharedPreferences.getInstance();
    await preferences.remove(_sessionPreferenceKey);
  }

  static SessionModel? _readPersistedSession(String? raw) {
    if (raw == null || raw.isEmpty) {
      return null;
    }
    try {
      final decoded = jsonDecode(raw);
      if (decoded is! Map<String, dynamic>) {
        return null;
      }
      final session = SessionModel.fromJson(decoded);
      if (session.userId.isEmpty || session.accessToken.isEmpty) {
        return null;
      }
      return session;
    } catch (_) {
      return null;
    }
  }

  static Future<_PersistedSessionValidationResult> _validatePersistedSession(
    SessionModel session,
  ) async {
    try {
      await StartupLog.write('validate persisted session start');
      _instance.restoreSession(session);
      await _instance.listDevices();
      await StartupLog.write('validate persisted session success');
      return _persistedSessionValid;
    } catch (error) {
      await StartupLog.write('validate persisted session failed: $error');
      if (_isUnauthorizedSessionError(error)) {
        return const _PersistedSessionValidationResult(
          isValid: false,
          shouldClearPersistedSession: true,
        );
      }
      return const _PersistedSessionValidationResult(
        isValid: false,
        shouldClearPersistedSession: false,
      );
    }
  }

  static bool _isUnauthorizedSessionError(Object error) {
    if (error is HttpAppCoreException) {
      if (error.statusCode == 401 || error.statusCode == 403) {
        return true;
      }
      final code = error.code?.trim().toLowerCase();
      return code == 'unauthorized' ||
          code == 'invalid_token' ||
          code == 'session_invalid' ||
          code == 'session_expired';
    }
    return false;
  }

  static String? _deriveWebConsoleUrl(String? baseUrl) {
    final normalized = baseUrl?.trim();
    if (normalized == null || normalized.isEmpty) {
      return null;
    }
    final uri = Uri.tryParse(normalized);
    if (uri == null || uri.host.isEmpty) {
      return null;
    }
    if (uri.host == '127.0.0.1' || uri.host == 'localhost' || uri.host == '::1') {
      return 'https://web.slan.localhost:18443';
    }
    if (uri.host == 'slan.localhost') {
      return uri
          .replace(
              host: 'web.slan.localhost', path: '', query: null, fragment: null)
          .toString();
    }
    final port = uri.hasPort ? uri.port : (uri.scheme == 'https' ? 443 : 80);
    final targetPort = switch (port) {
      28080 => 4200,
      8080 => 4200,
      80 => 4200,
      443 => 4200,
      _ => 4200,
    };
    return uri
        .replace(
          port: targetPort,
          path: '',
          query: null,
          fragment: null,
        )
        .toString();
  }

  static Future<void> _cleanupInactiveTunnelBackend(String reason) async {
    try {
      final result = await _tunnelHostGateway.bringTunnelDown();
      await StartupLog.write(
        'inactive tunnel cleanup reason=$reason accepted=${result.accepted} detail=${result.detail}',
      );
    } catch (error) {
      await StartupLog.write(
        'inactive tunnel cleanup skipped reason=$reason error=$error',
      );
    }
  }
}

final class _PersistedSessionValidationResult {
  const _PersistedSessionValidationResult({
    required this.isValid,
    required this.shouldClearPersistedSession,
  });

  final bool isValid;
  final bool shouldClearPersistedSession;
}
