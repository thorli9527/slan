import 'dart:convert';

import 'package:flutter/services.dart';

import 'android_vpn_contract.dart';

class ClientCorePlugin {
  ClientCorePlugin({MethodChannel? channel})
      : _channel = channel ?? const MethodChannel('dev.slan/client_core_v2');

  final MethodChannel _channel;

  static void registerWith() {}

  Future<Object?> androidVpnPermissionState() {
    return _invokeNativeOnly('androidVpnPermissionState');
  }

  Future<AndroidVpnConsentRequest?> androidRequestVpnPermission() async {
    final result = await _invokeNativeOnly('androidRequestVpnPermission');
    final json = _jsonMap(result);
    return json == null ? null : AndroidVpnConsentRequest.fromJson(json);
  }

  Future<Object?> androidStartVpn(AndroidVpnSessionConfig config) {
    return _invokeNativeOnly('androidStartVpn', config.toJson());
  }

  Future<Object?> androidStopVpn() {
    return _invokeNativeOnly('androidStopVpn');
  }

  Future<Object?> androidProtectSocket(AndroidSocketProtectionRequest request) {
    return _invokeNativeOnly('androidProtectSocket', request.toJson());
  }

  Future<Object?> androidRuntimeState() {
    return _invokeNativeOnly('androidRuntimeState');
  }

  Future<AndroidNetworkEvent?> androidWatchNetworkEvent() async {
    final result = await _invokeNativeOnly('androidWatchNetworkEvent');
    final json = _jsonMap(result);
    return json == null ? null : AndroidNetworkEvent.fromJson(json);
  }

  Future<Map<String, Object?>?> iosPacketTunnelStats() async {
    final result = await _invokeNativeOnly('iosPacketTunnelStats');
    return _jsonMap(result);
  }

  Future<Map<String, Object?>?> iosSharedStoreDiagnostics() async {
    final result = await _invokeNativeOnly('iosSharedStoreDiagnostics');
    return _jsonMap(result);
  }

  Future<Object?> iosStartPacketTunnel(PlatformNetworkConfig config) {
    return _invokeNativeOnly('iosStartPacketTunnel', config.toJson());
  }

  Future<Object?> iosStopPacketTunnel() {
    return _invokeNativeOnly('iosStopPacketTunnel');
  }

  Future<Map<String, Object?>?> embeddedServiceRequest(
      String requestJson) async {
    final result =
        await _invokeNativeOnly('embeddedServiceRequest', requestJson);
    return _jsonMap(result);
  }

  Future<String?> mobileServerBaseUrl() async {
    final result = await _invokeNativeOnly('mobileServerBaseUrl');
    return result is String ? result.trim() : null;
  }

  Future<void> setMobileServerBaseUrl(String serverBaseUrl) {
    return _invokeNativeOnly('setMobileServerBaseUrl', serverBaseUrl);
  }

  Future<Object?> _invokeNativeOnly(String method, [Object? arguments]) {
    return _channel.invokeMethod<Object?>(method, arguments);
  }

  Map<String, Object?>? _jsonMap(Object? result) {
    if (result is Map) {
      return result.cast<String, Object?>();
    }
    if (result is String && result.trim().isNotEmpty) {
      final decoded = jsonDecode(result);
      return decoded is Map ? decoded.cast<String, Object?>() : null;
    }
    return null;
  }
}
