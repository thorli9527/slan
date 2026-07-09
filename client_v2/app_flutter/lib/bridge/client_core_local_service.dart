import 'dart:async';
import 'dart:convert';
import 'dart:io';

import 'package:client_core_plugin/client_core_plugin.dart';
import 'package:flutter/services.dart';

/// 桌面端访问本地 client-core-service 的轻量 JSON-line 客户端。
///
/// Rust 服务监听 `127.0.0.1:46392`，Flutter 通过 TCP 发送一行 JSON：
/// `{ "method": "...", "args": {...} }`，服务返回一行 JSON。这个类只负责
/// 协议封装，不持有 UI 状态。
class ClientCoreLocalService {
  ClientCoreLocalService({String? host}) : _host = host;

  /// 编译期指定的本地服务地址，便于测试和定制端口。
  static const _definedServiceHost =
      String.fromEnvironment('SLAN_CLIENT_CORE_SERVICE_HOST');

  /// 构造时显式传入的服务地址，优先级最高。
  final String? _host;

  /// 查询当前 UI 状态快照。
  Future<Map<String, Object?>?> localState() async {
    return requestJson('localState');
  }

  /// 长轮询等待状态变化。
  ///
  /// [lastRevision] 是调用方已看到的状态版本；服务在版本变化或超时后返回。
  Future<Map<String, Object?>?> localStateWatch({
    required int lastRevision,
    int timeoutMs = 30000,
  }) async {
    return requestJson(
      'localStateWatch',
      arguments: {
        'lastRevision': lastRevision,
        'timeoutMs': timeoutMs,
      },
      allowEmptyResponse: true,
    );
  }

  /// 查询本地服务和网络模块的总体状态。
  Future<Map<String, Object?>?> localStatus() async {
    return requestJson('localStatus');
  }

  /// 查询当前持久化 session 摘要。
  Future<Map<String, Object?>?> localSession() async {
    return requestJson('localSession');
  }

  /// 查询当前网络中的 peer 摘要。
  Future<Map<String, Object?>?> localPeers() async {
    return requestJson('localPeers');
  }

  /// 查询当前路径选择计划。
  Future<Map<String, Object?>?> localPathPlan() async {
    return requestJson('localPathPlan');
  }

  /// 查询路径健康诊断结果。
  Future<Map<String, Object?>?> localPathDiagnose() async {
    return requestJson('localPathDiagnose');
  }

  /// 查询 relay 候选节点。
  ///
  /// [refresh] 为 true 时会要求服务端重新拉取/探测候选节点。
  Future<Map<String, Object?>?> localRelayCandidates(
      {bool refresh = false}) async {
    return requestJson(
      refresh ? 'localRefreshRelayCandidates' : 'localRelayCandidates',
    );
  }

  /// 查询 MQTT 控制通道状态。
  Future<Map<String, Object?>?> localControlStatus() async {
    return requestJson('localControlStatus');
  }

  /// 查询控制通道下一步计划。
  Future<Map<String, Object?>?> localControlPlan() async {
    return requestJson('localControlPlan');
  }

  /// 查询控制通道调度节奏。
  Future<Map<String, Object?>?> localControlCadence() async {
    return requestJson('localControlCadence');
  }

  /// 让服务计算一次控制通道 tick 计划，但不直接执行网络 I/O。
  Future<Map<String, Object?>?> localControlTickPlan({
    Object? arguments,
  }) async {
    return requestJson('localControlTickPlan', arguments: arguments);
  }

  /// 获取待发布到 MQTT 的上行控制消息。
  Future<Map<String, Object?>?> localControlOutbox({
    Object? arguments,
  }) async {
    return requestJson('localControlOutbox', arguments: arguments);
  }

  /// 查询等待业务确认的下行控制任务。
  Future<Map<String, Object?>?> localPendingControlAcks() async {
    return requestJson('localPendingControlAcks');
  }

  /// 标记指定下行控制任务已被 UI/客户端处理。
  Future<Map<String, Object?>?> localMarkControlAcked({
    required String taskId,
  }) async {
    return requestJson(
      'localMarkControlAcked',
      arguments: {'taskId': taskId},
    );
  }

  /// 标记一条上行控制消息已成功发布。
  Future<Map<String, Object?>?> localMarkTransportPublished({
    Object? arguments,
  }) async {
    return requestJson('localMarkTransportPublished', arguments: arguments);
  }

  /// 准备 relay 数据面配置。
  Future<Map<String, Object?>?> localRelayPrepare() async {
    return requestJson('localRelayPrepare');
  }

  /// 退出当前账号并清理本地 session。
  Future<Map<String, Object?>?> logout() async {
    return requestJson('localLogout');
  }

  /// 根据最新服务端配置启用虚拟网络。
  Future<Map<String, Object?>?> localNetworkActivate() async {
    return requestJson('localNetworkActivate');
  }

  /// 停用虚拟网络但保留登录状态。
  Future<Map<String, Object?>?> localNetworkDeactivate() async {
    return requestJson('localNetworkDeactivate');
  }

  /// 关闭本地数据面，通常用于应用退出或安装卸载清理。
  Future<Map<String, Object?>?> localNetworkShutdown() async {
    return requestJson('localNetworkShutdown');
  }

  /// 供集成测试使用的测试用户注册入口。
  Future<Map<String, Object?>?> localRegisterTestUser({
    required String email,
    required String password,
  }) async {
    return requestJson(
      'localRegisterTestUser',
      arguments: {
        'email': email,
        'password': password,
      },
    );
  }

  /// 通过 Rust local API 统一上报设备 runtime 到控制面。
  Future<Map<String, Object?>?> localReportDeviceRuntime({
    required String deviceId,
    required Map<String, Object?> body,
  }) async {
    return requestJson(
      'localReportDeviceRuntime',
      arguments: {
        'deviceId': deviceId,
        'body': body,
      },
    );
  }

  /// 长轮询等待业务事件。
  ///
  /// 业务事件用于驱动 UI 增量刷新，例如登录成功、网络配置变化、消息到达。
  Future<Map<String, Object?>?> localBusinessEventWatch({
    required int lastRevision,
    int timeoutMs = 30000,
  }) async {
    return requestJson(
      'localBusinessEventWatch',
      arguments: {
        'lastRevision': lastRevision,
        'timeoutMs': timeoutMs,
      },
      allowEmptyResponse: true,
    );
  }

  /// 获取平台网络配置，并转换成 Android/iOS 共用的 VPN 配置模型。
  Future<AndroidVpnSessionConfig?> localPlatformNetworkConfig() async {
    final json = await requestJson('localPlatformNetworkConfig');
    return json == null ? null : AndroidVpnSessionConfig.fromJson(json);
  }

  /// 把原生平台数据面的运行状态回报给本地服务。
  ///
  /// 移动端 VPN/PacketTunnel 运行在原生层，服务需要这些数据来刷新 UI、
  /// 上报路径健康和流量统计。
  Future<Map<String, Object?>?> ingestPlatformRuntimeState({
    required Map<String, Object?> runtimeState,
    String? platform,
    Map<String, Object?>? traffic,
    String? error,
  }) {
    return requestJson(
      'ingestPlatformRuntimeState',
      arguments: {
        if (platform != null) 'platform': platform,
        'runtimeState': runtimeState,
        if (traffic != null) 'traffic': traffic,
        if (error != null) 'error': error,
        'reportedAtMs': DateTime.now().millisecondsSinceEpoch,
      },
    );
  }

  /// 导出本地诊断文件。
  Future<Map<String, Object?>?> localDiagnosticsExport() async {
    return requestJson('localDiagnosticsExport');
  }

  /// 发送请求并把返回值解析成 JSON map。
  Future<Map<String, Object?>?> requestJson(
    String method, {
    Object? arguments,
    bool allowEmptyResponse = false,
  }) async {
    final result = await request(
      method,
      arguments: arguments,
      allowEmptyResponse: allowEmptyResponse,
    );
    return jsonMapFromResult(result);
  }

  /// 发送原始 JSON-line 请求。
  ///
  /// 这个方法是所有桌面端本地服务调用的唯一出口，负责校验 host、建立
  /// TCP 连接、发送请求、读取第一行响应和关闭 socket。
  Future<Object?> request(
    String method, {
    Object? arguments,
    bool allowEmptyResponse = false,
  }) async {
    final host = _serviceHost();
    final separator = host.lastIndexOf(':');
    if (separator <= 0 || separator == host.length - 1) {
      throw PlatformException(
        code: 'invalid_service_host',
        message: 'invalid SLAN_CLIENT_CORE_SERVICE_HOST: $host',
      );
    }
    final hostname = host.substring(0, separator);
    final port = int.tryParse(host.substring(separator + 1));
    if (port == null || port <= 0 || port > 65535) {
      throw PlatformException(
        code: 'invalid_service_port',
        message: 'invalid SLAN client service port: $host',
      );
    }

    final socket = await Socket.connect(
      hostname,
      port,
      timeout: const Duration(seconds: 2),
    );
    try {
      final payload = jsonEncode({
        'method': method,
        'args': arguments ?? <String, Object?>{},
      });
      socket.write('$payload\n');
      await socket.flush();
      try {
        return await socket
            .cast<List<int>>()
            .transform(utf8.decoder)
            .transform(const LineSplitter())
            .first
            .timeout(const Duration(seconds: 90));
      } on StateError {
        if (allowEmptyResponse) {
          return '';
        }
        rethrow;
      }
    } finally {
      await socket.close();
    }
  }

  /// 解析本地服务地址。
  ///
  /// 优先级：构造参数 > 编译期 dart-define > 进程环境变量 > 默认端口。
  String _serviceHost() {
    return _host ??
        (_definedServiceHost.isEmpty ? null : _definedServiceHost) ??
        Platform.environment['SLAN_CLIENT_CORE_SERVICE_HOST'] ??
        '127.0.0.1:46392';
  }

  /// 把本地服务返回值规约成 JSON map。
  static Map<String, Object?>? jsonMapFromResult(Object? result) {
    if (result is Map) {
      return result.cast<String, Object?>();
    }
    if (result is String && result.trim().isNotEmpty) {
      return jsonDecode(result) as Map<String, Object?>;
    }
    return null;
  }

  /// 把插件或服务返回值规约成字符串。
  ///
  /// Android VPN 权限状态可能以字符串或 JSON map 返回，这里统一处理。
  static String stringResult(Object? result) {
    if (result is String) {
      final value = result.trim();
      if (value.startsWith('{')) {
        final decoded = jsonDecode(value);
        if (decoded is Map && decoded['permissionState'] is String) {
          return decoded['permissionState'] as String;
        }
      }
      return value;
    }
    if (result is Map && result['permissionState'] is String) {
      return result['permissionState'] as String;
    }
    return '';
  }
}
