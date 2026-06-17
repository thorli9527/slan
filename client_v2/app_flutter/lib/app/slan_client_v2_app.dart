import 'package:flutter/material.dart';

import '../bridge/client_core_bridge.dart';
import '../ui/home/home_page.dart';

/// SLAN 客户端 Flutter 应用根组件。
///
/// 根组件只配置全局主题和首页依赖注入，避免把业务状态放在全局
/// widget 中，便于桌面端、Android、iOS 共用同一套 UI。
class SlanClientV2App extends StatelessWidget {
  const SlanClientV2App({required this.bridge, super.key});

  /// UI 调用客户端核心能力的门面。
  ///
  /// 生产环境注入 [MethodChannelClientCoreBridge]，测试环境可以替换成
  /// fake bridge，以便验证页面渲染和交互。
  final ClientCoreBridge bridge;

  @override
  Widget build(BuildContext context) {
    return MaterialApp(
      debugShowCheckedModeBanner: false,
      title: 'SLAN Client',
      theme: ThemeData(
        colorScheme: ColorScheme.fromSeed(seedColor: const Color(0xff9b4f2d)),
        useMaterial3: true,
      ),
      home: HomePage(bridge: bridge),
    );
  }
}
