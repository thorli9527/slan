import 'dart:async';
import 'dart:convert';
import 'dart:io';

import 'package:flutter/foundation.dart';
import 'package:shared_preferences/shared_preferences.dart';

import '../../infra/app_core/models/identity_models.dart';
import '../../infra/app_core/scope/app_core_scope.dart';
import '../../shared/desktop_platform.dart';

abstract class AuthCallbackGateway {
  Future<String> preparePendingServerCallback({String? preferredKey});

  Future<void> clearPendingServerCallback();
}

final class DefaultAuthCallbackGateway implements AuthCallbackGateway {
  const DefaultAuthCallbackGateway();

  static const instance = DefaultAuthCallbackGateway();

  @override
  Future<String> preparePendingServerCallback({String? preferredKey}) {
    return AuthCallbackService.preparePendingServerCallback(
      preferredKey: preferredKey,
    );
  }

  @override
  Future<void> clearPendingServerCallback() {
    return AuthCallbackService.clearPendingServerCallback();
  }
}

class AuthCallbackService {
  AuthCallbackService._();

  static const _pendingCallbackIdKey = 'slan.pending_auth_callback_id';
  static bool _initialized = false;
  static Timer? _pollTimer;
  static String? _lastAppliedCallbackId;

  static Future<String> preparePendingServerCallback(
      {String? preferredKey}) async {
    final callbackId = (preferredKey?.trim().isNotEmpty == true)
        ? preferredKey!.trim()
        : 'cb-${DateTime.now().microsecondsSinceEpoch}';
    // Allow the same device-scoped callback id to be reused across fresh login attempts.
    _lastAppliedCallbackId = null;
    final preferences = await SharedPreferences.getInstance();
    await preferences.setString(_pendingCallbackIdKey, callbackId);
    return callbackId;
  }

  static Future<void> clearPendingServerCallback() async {
    _lastAppliedCallbackId = null;
    final preferences = await SharedPreferences.getInstance();
    await preferences.remove(_pendingCallbackIdKey);
  }

  static Future<void> ensureInitialized() async {
    if (_initialized) {
      debugPrint(
          '[auth-callback] ensureInitialized skipped: already initialized');
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
      debugPrint(
          '[auth-callback] server payload missing accessToken or userId');
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
    debugPrint(
        '[auth-callback] server callback applied callbackId=$callbackId');
  }

  static Future<void> _ensurePendingServerCallbackSocket() async {
    final preferences = await SharedPreferences.getInstance();
    await preferences.reload();
    final callbackId =
        preferences.getString(_pendingCallbackIdKey)?.trim() ?? '';
    if (callbackId.isEmpty) {
      return;
    }
    try {
      await _pollServerCallbackStatus(callbackId);
    } catch (_) {
      debugPrint('[auth-callback] callback status polling failed');
    }
  }

  static Future<void> _pollServerCallbackStatus(String callbackId) async {
    final uri = _buildControlUri('/auth/callback-status/$callbackId');
    final client = HttpClient();
    try {
      final request = await client.getUrl(uri);
      final response = await request.close();
      if (response.statusCode < 200 || response.statusCode >= 300) {
        return;
      }
      final body = await utf8.decodeStream(response);
      final decoded = jsonDecode(body);
      if (decoded is! Map<String, dynamic> || decoded['ready'] != true) {
        return;
      }
      final payload = decoded['payload'];
      if (payload is! Map<String, dynamic>) {
        return;
      }
      await _applyServerCallback(callbackId: callbackId, payload: payload);
    } finally {
      client.close(force: true);
    }
  }

  static Uri _buildControlUri(String path) {
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
      return Uri.parse('http://127.0.0.1:28080$path');
    }
    return base.replace(
      path: path,
      query: null,
      fragment: null,
    );
  }

  static Future<void> _activateActiveNetworkAfterCallback() async {
    try {
      final host = Platform.localHostname.replaceAll('.', '-');
      await AppCoreScope.sessionController.ensureHomeWorkspaceReady(
        deviceName: host,
        platform: DesktopPlatform.currentId,
        deviceVersion: DesktopPlatform.currentVersion,
        machineId: AppCoreScope.clientMachineId,
        devicePublicKey: 'device-key-${DateTime.now().microsecondsSinceEpoch}',
      );
      if (AppCoreScope.sessionStore.networks.isEmpty) {
        debugPrint(
            '[auth-callback] skip activate_active_network: no active network');
        return;
      }
      await AppCoreScope.sessionController.enableActiveNetwork();
      debugPrint('[auth-callback] activate_active_network completed');
    } catch (error) {
      debugPrint('[auth-callback] activate_active_network failed: $error');
    }
  }
}
