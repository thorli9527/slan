/// AppCore 依赖注入入口。
///
/// 默认仍挂接 mock；联调时可切到真实控制面 HTTP 或 facade bridge。
library slan_app.infra.app_core.scope;

import 'dart:convert';
import 'dart:core';
import 'dart:async';
import 'dart:io' show Platform;

import 'package:flutter/foundation.dart';
import 'package:flutter/services.dart';
import 'package:shared_preferences/shared_preferences.dart';

import '../../../application/app_workspace_service.dart';
import '../../../application/auth_session_service.dart';
import '../../../application/device_runtime_service.dart';
import '../../../application/device_setup_service.dart';
import '../../../application/local_dns_service.dart';
import '../../../application/mqtt_control_task_service.dart';
import '../../../application/network_enable_preflight_service.dart';
import '../../../application/tunnel_configuration_service.dart';
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
import 'machine_identity.dart';

class AppCoreScope {
  AppCoreScope._();
  static const _hostPreferenceKey = 'slan.server_host';
  static const _clientMachineIdKey = 'slan.client_machine_id';
  static const _sessionPreferenceKey = 'slan.session';
  static const _networkUsagePreferenceKey = 'slan.last_network_usage_state';
  static const _bridgeHelperReadyTimeout = Duration(seconds: 8);
  static const _bridgeHelperReadyPollInterval = Duration(milliseconds: 250);

  static const String _appCoreMode =
      String.fromEnvironment('SLAN_APP_CORE_MODE');
  static const String _serverHost = String.fromEnvironment('SLAN_SERVER_HOST');
  static const String _controlBaseUrl =
      String.fromEnvironment('SLAN_CONTROL_BASE_URL');
  static const String _serverUiUrl =
      String.fromEnvironment('SLAN_SERVER_UI_URL');
  static const String _tunnelHostMode =
      String.fromEnvironment('SLAN_TUNNEL_HOST_MODE');
  static const String _serviceHostAddress =
      String.fromEnvironment('SLAN_APP_CORE_SERVICE_HOST');
  static const String _helperHostAddress =
      String.fromEnvironment('SLAN_APP_CORE_HELPER_HOST');
  static String? _runtimeHostInput = _serverHost.isEmpty ? null : _serverHost;
  static String? _runtimeControlBaseUrl =
      _controlBaseUrl.isEmpty ? null : _controlBaseUrl;
  static String? _runtimeServerUiUrl =
      _serverUiUrl.isEmpty ? null : _serverUiUrl;
  static String? _runtimeClientMachineId;
  static String? _runtimeModeOverride;

  static AppCoreApi _instance = _buildDefaultInstance();
  static TunnelHostGateway _tunnelHostGateway = _buildTunnelHostGateway();
  static MqttControlTaskService _mqttControlTaskService =
      const XmlMqttControlTaskService();
  static NetworkEnablePreflightService _networkEnablePreflightService =
      const DefaultNetworkEnablePreflightService();
  static LocalDnsServiceContract? _localDnsServiceOverride;
  static AppWorkspaceServiceContract? _workspaceServiceOverride;
  static AuthSessionServiceContract? _authSessionServiceOverride;
  static DeviceSetupServiceContract? _deviceSetupServiceOverride;
  static DeviceRuntimeServiceContract? _deviceRuntimeServiceOverride;
  static TunnelConfigurationServiceContract?
      _tunnelConfigurationServiceOverride;
  static AppCoreCoordinator _coordinator =
      AppCoreCoordinator(
    apiProvider: () => _instance,
    hostGateway: _tunnelHostGateway,
    mqttControlTaskService: _mqttControlTaskService,
    networkEnablePreflightService: _networkEnablePreflightService,
  );
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
    final preferences = await SharedPreferences.getInstance();
    await StartupLog.write('preferences loaded');
    final persistedHost = preferences.getString(_hostPreferenceKey)?.trim();
    if (persistedHost != null && persistedHost.isNotEmpty) {
      _runtimeHostInput = persistedHost;
      _runtimeControlBaseUrl = null;
      _runtimeServerUiUrl = null;
      await StartupLog.write('persisted host restored: $_runtimeHostInput');
    }
    _runtimeClientMachineId = await MachineIdentity.resolve(
      persistedMachineId: preferences.getString(_clientMachineIdKey),
    );
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
      final validationResult =
          await _validatePersistedSession(persistedSession);
      await StartupLog.write(
        'persisted session validation result=${validationResult.isValid} clear=${validationResult.shouldClearPersistedSession}',
      );
      if (validationResult.isValid) {
        await _coordinator.applyExternalSessionInternal(
          validationResult.session ?? persistedSession,
          persistSession: false,
          autoEnableLastNetwork: true,
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
        if (!_isBridgeMode) {
          await _cleanupInactiveTunnelBackend('invalid persisted session');
        }
      }
    } else {
      await StartupLog.write('no persisted session');
      if (!_isBridgeMode) {
        await _cleanupInactiveTunnelBackend('no persisted session');
      }
    }
    await StartupLog.write('app core initialize done');
  }

  static AppCoreApi get instance => _instance;
  static AppSessionStore get sessionStore => _coordinator.sessionStore;
  static AppTunnelStore get tunnelStore => _coordinator.tunnelStore;
  static AppSessionController get sessionController => _sessionController;
  static AppTunnelController get tunnelController => _tunnelController;
  static TunnelHostGateway get tunnelHostGateway => _tunnelHostGateway;
  static String get mode {
    final override = _runtimeModeOverride;
    if (override != null && override.isNotEmpty) {
      return override;
    }
    final configured = _appCoreMode.trim();
    if (configured.isNotEmpty) {
      return configured;
    }
    return Platform.isWindows ? 'bridge' : '';
  }

  static AppHostConfig? get hostConfig =>
      AppHostConfig.tryParse(_runtimeHostInput) ??
      AppHostConfig.tryParse(_runtimeControlBaseUrl) ??
      (Platform.isWindows ? AppHostConfig.tryParse('127.0.0.1') : null);
  static String? get hostInput => _runtimeHostInput ?? hostConfig?.rawInput;
  static String? get controlBaseUrl =>
      hostConfig?.controlBaseUrl ?? _runtimeControlBaseUrl;
  static String? get webConsoleUrl =>
      _runtimeServerUiUrl ??
      hostConfig?.webConsoleUrl ??
      _deriveWebConsoleUrl(_runtimeControlBaseUrl);
  static bool get _isBridgeMode => mode == 'bridge';
  static String get clientMachineId => _runtimeClientMachineId ??=
      'client-${DateTime.now().microsecondsSinceEpoch}';

  static AppCoreApi _buildDefaultInstance() {
    final controlBaseUrl = AppCoreScope.controlBaseUrl;
    return switch (mode) {
      'bridge' => BridgeAppCoreApi(),
      _ => controlBaseUrl == null || controlBaseUrl.isEmpty
          ? MockAppCoreApi()
          : HttpAppCoreApi(baseUrl: controlBaseUrl),
    };
  }

  static void configureHost({
    required String host,
  }) {
    final normalized = host.trim();
    _runtimeHostInput = normalized.isEmpty ? null : normalized;
    _runtimeControlBaseUrl = null;
    _runtimeServerUiUrl = null;
    if (!_isBridgeMode) {
      _instance = _buildDefaultInstance();
      _coordinator.resetForServerSwitch();
    }
    unawaited(_persistHost());
  }

  static void configureServer({
    required String baseUrl,
    String? webBaseUrl,
  }) {
    _runtimeHostInput = null;
    final normalized = baseUrl.trim();
    _runtimeControlBaseUrl = normalized.isEmpty ? null : normalized;
    final webNormalized = webBaseUrl?.trim() ?? '';
    _runtimeServerUiUrl = webNormalized.isEmpty ? null : webNormalized;
    if (!_isBridgeMode) {
      _instance = _buildDefaultInstance();
      _coordinator.resetForServerSwitch();
    }
    unawaited(_persistHost(clear: true));
  }

  @visibleForTesting
  static void configureForTest({
    required AppCoreApi appCoreApi,
    TunnelHostGateway? tunnelHostGateway,
    MqttControlTaskService? mqttControlTaskService,
    NetworkEnablePreflightService? networkEnablePreflightService,
    LocalDnsServiceContract? localDnsService,
    AppWorkspaceServiceContract? workspaceService,
    AuthSessionServiceContract? authSessionService,
    DeviceSetupServiceContract? deviceSetupService,
    DeviceRuntimeServiceContract? deviceRuntimeService,
    TunnelConfigurationServiceContract? tunnelConfigurationService,
    String? mode,
  }) {
    _runtimeModeOverride = mode?.trim().isEmpty == true ? null : mode?.trim();
    _instance = appCoreApi;
    _tunnelHostGateway = tunnelHostGateway ?? _buildTunnelHostGateway();
    _mqttControlTaskService =
        mqttControlTaskService ?? const XmlMqttControlTaskService();
    _networkEnablePreflightService =
        networkEnablePreflightService ??
            const DefaultNetworkEnablePreflightService();
    _localDnsServiceOverride = localDnsService;
    _workspaceServiceOverride = workspaceService;
    _authSessionServiceOverride = authSessionService;
    _deviceSetupServiceOverride = deviceSetupService;
    _deviceRuntimeServiceOverride = deviceRuntimeService;
    _tunnelConfigurationServiceOverride = tunnelConfigurationService;
    _resetStoreBindings();
  }

  @visibleForTesting
  static void resetForTest() {
    _runtimeHostInput = _serverHost.isEmpty ? null : _serverHost;
    _runtimeControlBaseUrl = _controlBaseUrl.isEmpty ? null : _controlBaseUrl;
    _runtimeServerUiUrl = _serverUiUrl.isEmpty ? null : _serverUiUrl;
    _runtimeClientMachineId = null;
    _runtimeModeOverride = null;
    _instance = _buildDefaultInstance();
    _tunnelHostGateway = _buildTunnelHostGateway();
    _mqttControlTaskService = const XmlMqttControlTaskService();
    _networkEnablePreflightService =
        const DefaultNetworkEnablePreflightService();
    _localDnsServiceOverride = null;
    _workspaceServiceOverride = null;
    _authSessionServiceOverride = null;
    _deviceSetupServiceOverride = null;
    _deviceRuntimeServiceOverride = null;
    _tunnelConfigurationServiceOverride = null;
    _resetStoreBindings();
  }

  static void _resetStoreBindings() {
    _coordinator.resetState();
    _coordinator = AppCoreCoordinator(
      apiProvider: () => _instance,
      hostGateway: _tunnelHostGateway,
      mqttControlTaskService: _mqttControlTaskService,
      networkEnablePreflightService: _networkEnablePreflightService,
      localDnsService: _localDnsServiceOverride,
      workspaceService: _workspaceServiceOverride,
      authSessionService: _authSessionServiceOverride,
      deviceSetupService: _deviceSetupServiceOverride,
      deviceRuntimeService: _deviceRuntimeServiceOverride,
      tunnelConfigurationService: _tunnelConfigurationServiceOverride,
    );
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
    final serviceHost = _serviceHostAddress.trim();
    if (serviceHost.isNotEmpty) {
      return serviceHost;
    }
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
    final preferences = await SharedPreferences.getInstance();
    await preferences.setString(
      _sessionPreferenceKey,
      jsonEncode(session.toJson()),
    );
  }

  static Future<void> clearPersistedSession() async {
    final preferences = await SharedPreferences.getInstance();
    await preferences.remove(_sessionPreferenceKey);
  }

  static Future<void> hydrateBridgeSession(SessionModel session) async {
    if (!_isBridgeMode) {
      return;
    }
    await _waitForBridgeHelperReady('hydrate bridge session');
    _instance.restoreSession(session);
    await _instance.listDevices();
  }

  static Future<void> persistNetworkUsageState({
    required String userId,
    required bool enabled,
    String? networkId,
  }) async {
    final normalizedUserId = userId.trim();
    if (normalizedUserId.isEmpty) {
      return;
    }
    final preferences = await SharedPreferences.getInstance();
    await preferences.setString(
      _networkUsagePreferenceKey,
      jsonEncode({
        'userId': normalizedUserId,
        'networkId': networkId?.trim(),
        'state': enabled ? 'enabled' : 'disabled',
        'updatedAtMs': DateTime.now().millisecondsSinceEpoch,
      }),
    );
  }

  static Future<PersistedNetworkUsageState?> readNetworkUsageState(
    String userId,
  ) async {
    final normalizedUserId = userId.trim();
    if (normalizedUserId.isEmpty) {
      return null;
    }
    final preferences = await SharedPreferences.getInstance();
    final raw = preferences.getString(_networkUsagePreferenceKey);
    if (raw == null || raw.isEmpty) {
      return null;
    }
    try {
      final decoded = jsonDecode(raw);
      if (decoded is! Map<String, dynamic>) {
        return null;
      }
      final storedUserId = (decoded['userId'] as String? ?? '').trim();
      if (storedUserId != normalizedUserId) {
        return null;
      }
      final state = (decoded['state'] as String? ?? '').trim().toLowerCase();
      return PersistedNetworkUsageState(
        userId: storedUserId,
        networkId: (decoded['networkId'] as String?)?.trim(),
        enabled: state == 'enabled',
        updatedAtMs: (decoded['updatedAtMs'] as num?)?.toInt(),
      );
    } catch (_) {
      return null;
    }
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
      if (_isBridgeMode) {
        final helperReady =
            await _waitForBridgeHelperReady('validate persisted session');
        if (!helperReady) {
          await StartupLog.write(
            'validate persisted bridge session skipped: helper not ready',
          );
          return const _PersistedSessionValidationResult(
            isValid: false,
            shouldClearPersistedSession: false,
          );
        }
        try {
          _instance.restoreSession(session);
          final devices = await _instance.listDevices();
          var validatedSession = session;
          final sessionDeviceId = session.deviceId?.trim() ?? '';
          if (sessionDeviceId.isEmpty && devices.isNotEmpty) {
            validatedSession =
                session.copyWith(deviceId: devices.first.deviceId);
            await persistSession(validatedSession);
            _instance.restoreSession(validatedSession);
          }
          if (_sessionMissingUserLabel(session)) {
            final refreshed =
                await _tryRefreshPersistedSession(validatedSession);
            if (refreshed != null) {
              await StartupLog.write(
                'validate persisted bridge session refreshed missing user label',
              );
              return _persistedSessionValid.withSession(refreshed);
            }
          }
          await StartupLog.write('validate persisted bridge session success');
          return _persistedSessionValid.withSession(validatedSession);
        } catch (error) {
          await StartupLog.write(
            'validate persisted bridge session restore failed: $error',
          );
        }
        final refreshed = await _tryRefreshPersistedSession(session);
        if (refreshed != null) {
          await StartupLog.write(
            'validate persisted bridge session refresh success',
          );
          return _persistedSessionValid.withSession(refreshed);
        }
        await StartupLog.write('validate persisted bridge session skipped');
        return const _PersistedSessionValidationResult(
          isValid: false,
          shouldClearPersistedSession: false,
        );
      }
      _instance.restoreSession(session);
      await _instance.listDevices();
      if (_sessionMissingUserLabel(session)) {
        final refreshed = await _tryRefreshPersistedSession(session);
        if (refreshed != null) {
          await StartupLog.write(
            'validate persisted session refreshed missing user label',
          );
          return _persistedSessionValid.withSession(refreshed);
        }
      }
      await StartupLog.write('validate persisted session success');
      return _persistedSessionValid.withSession(session);
    } catch (error) {
      await StartupLog.write('validate persisted session failed: $error');
      if (_isUnauthorizedSessionError(error)) {
        final refreshed = await _tryRefreshPersistedSession(session);
        if (refreshed != null) {
          return _persistedSessionValid.withSession(refreshed);
        }
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

  static bool _sessionMissingUserLabel(SessionModel session) {
    final label = session.userLabel?.trim() ?? '';
    return label.isEmpty || !label.contains('@');
  }

  static Future<bool> _waitForBridgeHelperReady(String reason) async {
    if (!_isBridgeMode) {
      return true;
    }
    final deadline = DateTime.now().add(_bridgeHelperReadyTimeout);
    Object? lastError;
    var attempt = 0;
    while (true) {
      attempt += 1;
      try {
        final status = await _instance.helperStatus();
        if (status.helperReachable) {
          if (attempt > 1) {
            await StartupLog.write(
              'bridge helper ready reason=$reason attempts=$attempt',
            );
          }
          return true;
        }
        lastError = status.tunnelLastError ?? 'helperReachable=false';
      } catch (error) {
        lastError = error;
        if (!_isTransientBridgeHelperError(error)) {
          await StartupLog.write(
            'bridge helper readiness probe deferred to normal flow reason=$reason error=$error',
          );
          return true;
        }
      }

      if (!DateTime.now().isBefore(deadline)) {
        await StartupLog.write(
          'bridge helper not ready reason=$reason attempts=$attempt lastError=$lastError',
        );
        return false;
      }
      await Future<void>.delayed(_bridgeHelperReadyPollInterval);
    }
  }

  static bool _isTransientBridgeHelperError(Object error) {
    if (error is PlatformException) {
      final code = error.code.trim().toLowerCase();
      final message = (error.message ?? '').toLowerCase();
      if (code != 'app_core_helper_error') {
        return false;
      }
      return message.contains('10061') ||
          message.contains('connection refused') ||
          message.contains('actively refused') ||
          message.contains('无法连接') ||
          message.contains('拒绝');
    }
    final message = error.toString().toLowerCase();
    return message.contains('app_core_helper_error') &&
        (message.contains('10061') ||
            message.contains('connection refused') ||
            message.contains('actively refused') ||
            message.contains('无法连接') ||
            message.contains('拒绝'));
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
    if (uri.host == '127.0.0.1' ||
        uri.host == 'localhost' ||
        uri.host == '::1') {
      return 'http://127.0.0.1:24200';
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

  static Future<SessionModel?> _tryRefreshPersistedSession(
    SessionModel session,
  ) async {
    final refreshToken = session.refreshToken?.trim();
    if (refreshToken == null || refreshToken.isEmpty) {
      await StartupLog.write('persisted session refresh skipped: no token');
      return null;
    }
    try {
      await StartupLog.write('persisted session refresh start');
      final refreshed = await _instance.refreshSession(
        refreshToken: refreshToken,
        deviceId: session.deviceId,
      );
      final merged = refreshed.copyWith(
        deviceId: refreshed.deviceId ?? session.deviceId,
        userLabel: refreshed.userLabel ?? session.userLabel,
        authenticatedAtMs: DateTime.now().millisecondsSinceEpoch,
      );
      await persistSession(merged);
      _instance.restoreSession(merged);
      await _instance.listDevices();
      await StartupLog.write('persisted session refresh success');
      return merged;
    } catch (error) {
      await StartupLog.write('persisted session refresh failed: $error');
      return null;
    }
  }
}

final class PersistedNetworkUsageState {
  const PersistedNetworkUsageState({
    required this.userId,
    required this.enabled,
    this.networkId,
    this.updatedAtMs,
  });

  final String userId;
  final bool enabled;
  final String? networkId;
  final int? updatedAtMs;
}

final class _PersistedSessionValidationResult {
  const _PersistedSessionValidationResult({
    required this.isValid,
    required this.shouldClearPersistedSession,
    this.session,
  });

  final bool isValid;
  final bool shouldClearPersistedSession;
  final SessionModel? session;

  _PersistedSessionValidationResult withSession(SessionModel session) =>
      _PersistedSessionValidationResult(
        isValid: isValid,
        shouldClearPersistedSession: shouldClearPersistedSession,
        session: session,
      );
}
