import 'dart:async';
import 'dart:convert';
import 'dart:io';

import 'package:flutter/foundation.dart';
import 'package:shared_preferences/shared_preferences.dart';

import '../../infra/app_core/models/identity_models.dart';
import '../../infra/app_core/scope/app_core_scope.dart';
import '../../shared/desktop_platform.dart';

class AuthCallbackService {
  AuthCallbackService._();

  static const _pendingCallbackIdKey = 'slan.pending_auth_callback_id';
  static bool _initialized = false;
  static Timer? _pollTimer;
  static String? _lastAppliedCallbackId;
  static WebSocket? _socket;
  static String? _socketCallbackId;

  static Future<String> preparePendingServerCallback() async {
    final callbackId = 'cb-${DateTime.now().microsecondsSinceEpoch}';
    final preferences = await SharedPreferences.getInstance();
    await preferences.setString(_pendingCallbackIdKey, callbackId);
    return callbackId;
  }

  static Future<void> ensureInitialized() async {
    if (_initialized) {
      debugPrint('[auth-callback] ensureInitialized skipped: already initialized');
      return;
    }
    _initialized = true;
    debugPrint('[auth-callback] ensureInitialized start');
    try {
      await _ensurePendingServerCallbackSocket();
    } catch (_) {
      // Keep startup resilient; the polling loop will retry on the next tick.
    }
    _pollTimer ??= Timer.periodic(
      const Duration(seconds: 1),
      (_) => _ensurePendingServerCallbackSocket(),
    );
  }

  static Future<void> _applyServerCallback({
    required String callbackId,
    required Map<String, dynamic> payload,
  }) async {
    if (_lastAppliedCallbackId == callbackId) {
      debugPrint('[auth-callback] duplicate callbackId ignored: $callbackId');
      return;
    }
    final accessToken = (payload['accessToken'] as String? ?? '').trim();
    final userId = (payload['userId'] as String? ?? '').trim();
    final refreshToken = (payload['refreshToken'] as String?)?.trim();
    final deviceId = (payload['deviceId'] as String?)?.trim();
    final userLabel = (payload['userLabel'] as String?)?.trim();
    final action = (payload['action'] as String?)?.trim();
    final expiresIn = (payload['expiresIn'] as num?)?.toInt() ?? 3600;
    if (accessToken.isEmpty || userId.isEmpty) {
      debugPrint('[auth-callback] server payload missing accessToken or userId');
      return;
    }
    await AppCoreScope.sessionController.applyExternalSession(
      SessionModel(
        userId: userId,
        accessToken: accessToken,
        refreshToken: refreshToken?.isEmpty == true ? null : refreshToken,
        expiresIn: expiresIn,
        deviceId: deviceId?.isEmpty == true ? null : deviceId,
        userLabel: userLabel?.isEmpty == true ? null : userLabel,
      ),
    );
    if (action == 'activate_active_network') {
      await _activateActiveNetworkAfterCallback();
    }
    final preferences = await SharedPreferences.getInstance();
    await preferences.remove(_pendingCallbackIdKey);
    _lastAppliedCallbackId = callbackId;
    debugPrint('[auth-callback] server callback applied callbackId=$callbackId');
  }

  static Future<void> _ensurePendingServerCallbackSocket() async {
    final preferences = await SharedPreferences.getInstance();
    await preferences.reload();
    final callbackId = preferences.getString(_pendingCallbackIdKey)?.trim() ?? '';
    if (callbackId.isEmpty) {
      await _closeSocket();
      return;
    }
    if (_socket != null && _socketCallbackId == callbackId) {
      return;
    }
    await _closeSocket();
    try {
      final socket = await WebSocket.connect(
        _buildAuthWebSocketUrl(callbackId),
        headers: {
          'Origin': _buildAuthWebSocketOrigin(),
        },
      );
      _socket = socket;
      _socketCallbackId = callbackId;
      socket.listen(
        (message) async {
          if (message is! String || message.isEmpty) {
            return;
          }
          final decoded = jsonDecode(message);
          if (decoded is! Map<String, dynamic>) {
            return;
          }
          final type = (decoded['type'] as String? ?? '').trim();
          if (type == 'acknowledged') {
            await _closeSocket();
            return;
          }
          if (type != 'auth_callback_ready') {
            return;
          }
          final payload = decoded['payload'];
          if (payload is! Map<String, dynamic>) {
            return;
          }
          await _applyServerCallback(callbackId: callbackId, payload: payload);
          socket.add(jsonEncode({'type': 'ack'}));
        },
        onDone: () {
          if (_socket == socket) {
            _socket = null;
            _socketCallbackId = null;
          }
        },
        onError: (_) {
          if (_socket == socket) {
            _socket = null;
            _socketCallbackId = null;
          }
        },
        cancelOnError: true,
      );
    } catch (_) {
      debugPrint('[auth-callback] auth websocket connect failed');
    }
  }

  static String _buildAuthWebSocketUrl(String callbackId) {
    final baseUrl = AppCoreScope.controlBaseUrl?.trim() ?? '';
    final base = Uri.tryParse(baseUrl);
    if (base == null) {
      throw StateError('missing control base url');
    }
    final host = base.host.trim().toLowerCase();
    final isLocal = host == '127.0.0.1' ||
        host == 'localhost' ||
        host == '::1' ||
        host == 'slan.localhost' ||
        host == 'web.slan.localhost';
    if (isLocal) {
      return 'ws://127.0.0.1:28080/auth/ws/$callbackId';
    }
    final scheme = base.scheme == 'https' ? 'wss' : 'ws';
    return base
        .replace(
          scheme: scheme,
          path: '/auth/ws/$callbackId',
          query: null,
          fragment: null,
        )
        .toString();
  }

  static String _buildAuthWebSocketOrigin() {
    final baseUrl = AppCoreScope.controlBaseUrl?.trim() ?? '';
    final base = Uri.tryParse(baseUrl);
    if (base == null || base.host.isEmpty) {
      return 'http://127.0.0.1:28080';
    }
    if (base.host == '127.0.0.1' ||
        base.host == 'localhost' ||
        base.host == '::1' ||
        base.host == 'slan.localhost' ||
        base.host == 'web.slan.localhost') {
      return 'http://127.0.0.1:28080';
    }
    return base.replace(path: '', query: null, fragment: null).toString();
  }

  static Future<void> _closeSocket() async {
    final socket = _socket;
    _socket = null;
    _socketCallbackId = null;
    if (socket == null) {
      return;
    }
    try {
      await socket.close();
    } catch (_) {
      debugPrint('[auth-callback] close auth websocket failed');
    }
  }

  static Future<void> _activateActiveNetworkAfterCallback() async {
    try {
      final host = Platform.localHostname.replaceAll('.', '-');
      await AppCoreScope.sessionController.ensureHomeWorkspaceReady(
        deviceName: host,
        platform: DesktopPlatform.currentId,
        machineId: AppCoreScope.clientMachineId,
        devicePublicKey: 'device-key-${DateTime.now().microsecondsSinceEpoch}',
      );
      if (AppCoreScope.sessionStore.networks.isEmpty) {
        debugPrint('[auth-callback] skip activate_active_network: no active network');
        return;
      }
      await AppCoreScope.sessionController.enableActiveNetwork();
      debugPrint('[auth-callback] activate_active_network completed');
    } catch (error) {
      debugPrint('[auth-callback] activate_active_network failed: $error');
    }
  }
}
