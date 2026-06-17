import 'package:flutter/material.dart';

import 'app/slan_client_v2_app.dart';
import 'bridge/client_core_bridge.dart';

/// Flutter 客户端入口。
///
/// 这里只负责创建真实的 [MethodChannelClientCoreBridge] 并注入根组件；
/// 业务启动、登录、网络开关和平台差异都由 bridge 与页面层处理。
void main() {
  runApp(SlanClientV2App(bridge: MethodChannelClientCoreBridge()));
}
