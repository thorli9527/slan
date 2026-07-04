import 'dart:convert';

import 'package:flutter/services.dart';

import 'android_vpn_contract.dart';

/// ClientCorePlugin 是 Flutter 到原生平台插件的 MethodChannel 封装。
///
/// 它只负责跨平台方法调用和 JSON 转换；业务状态聚合在 app 层 bridge 中完成。
class ClientCorePlugin {
  ClientCorePlugin({MethodChannel? channel})
      : _channel = channel ?? const MethodChannel('dev.slan/client_core_v2');

  final MethodChannel _channel;

  static void registerWith() {}

  /// 查询 Android VpnService 授权状态。
  Future<Object?> androidVpnPermissionState() {
    return _invokeNativeOnly('androidVpnPermissionState');
  }

  /// 请求 Android VPN 授权，必要时返回系统授权请求信息。
  Future<AndroidVpnConsentRequest?> androidRequestVpnPermission() async {
    final result = await _invokeNativeOnly('androidRequestVpnPermission');
    final json = _jsonMap(result);
    return json == null ? null : AndroidVpnConsentRequest.fromJson(json);
  }

  /// 启动 Android VPN 数据面。
  Future<Object?> androidStartVpn(AndroidVpnSessionConfig config) {
    return _invokeNativeOnly('androidStartVpn', config.toJson());
  }

  /// 停止 Android VPN 数据面。
  Future<Object?> androidStopVpn() {
    return _invokeNativeOnly('androidStopVpn');
  }

  /// 请求 Android 原生层 protect 指定 socket，避免控制面流量被 VPN 捕获。
  Future<Object?> androidProtectSocket(AndroidSocketProtectionRequest request) {
    return _invokeNativeOnly('androidProtectSocket', request.toJson());
  }

  /// 查询 Android 原生层运行状态。
  Future<Object?> androidRuntimeState() {
    return _invokeNativeOnly('androidRuntimeState');
  }

  /// 等待 Android 原生层网络事件。
  Future<AndroidNetworkEvent?> androidWatchNetworkEvent() async {
    final result = await _invokeNativeOnly('androidWatchNetworkEvent');
    final json = _jsonMap(result);
    return json == null ? null : AndroidNetworkEvent.fromJson(json);
  }

  /// 查询 iOS PacketTunnel 数据面统计。
  Future<Map<String, Object?>?> iosPacketTunnelStats() async {
    final result = await _invokeNativeOnly('iosPacketTunnelStats');
    return _jsonMap(result);
  }

  /// 查询 iOS App Group 共享存储诊断信息。
  Future<Map<String, Object?>?> iosSharedStoreDiagnostics() async {
    final result = await _invokeNativeOnly('iosSharedStoreDiagnostics');
    return _jsonMap(result);
  }

  /// 查询 iOS 原生层运行状态。
  Future<Object?> iosRuntimeState() {
    return _invokeNativeOnly('iosRuntimeState');
  }

  /// 等待 iOS 原生层 PacketTunnel 网络事件。
  Future<AndroidNetworkEvent?> iosWatchNetworkEvent() async {
    final result = await _invokeNativeOnly('iosWatchNetworkEvent');
    final json = _jsonMap(result);
    return json == null ? null : AndroidNetworkEvent.fromJson(json);
  }

  /// 启动 iOS PacketTunnel 数据面。
  Future<Object?> iosStartPacketTunnel(PlatformNetworkConfig config) {
    return _invokeNativeOnly('iosStartPacketTunnel', config.toJson());
  }

  /// 停止 iOS PacketTunnel 数据面。
  Future<Object?> iosStopPacketTunnel() {
    return _invokeNativeOnly('iosStopPacketTunnel');
  }

  /// 调用移动端内嵌 client-core-service。
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

  Future<void> setAndroidDebugEmulatorVpnBypass(bool enabled) {
    return _invokeNativeOnly('setAndroidDebugEmulatorVpnBypass', enabled);
  }

  /// 派发桌面端通用控制命令。
  ///
  /// macOS/Windows/Linux 原生插件会负责自动启动本地 service，并在
  /// openClientLogin/openWebConsole 这类命令中打开系统默认浏览器。
  Future<Object?> dispatch(Map<String, Object?> command) {
    return _invokeNativeOnly('dispatch', command);
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
