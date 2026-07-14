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
    const seed = Color(0xffb85c2f);
    return MaterialApp(
      debugShowCheckedModeBanner: false,
      title: 'SLAN Client',
      theme: ThemeData(
        colorScheme: ColorScheme.fromSeed(seedColor: seed),
        useMaterial3: true,
        scaffoldBackgroundColor: const Color(0xfff6f2ee),
        inputDecorationTheme: const InputDecorationTheme(
          border: OutlineInputBorder(),
          isDense: true,
          filled: true,
          fillColor: Colors.white,
        ),
        cardTheme: CardThemeData(
          color: Colors.white,
          elevation: 0,
          shape: RoundedRectangleBorder(
            borderRadius: BorderRadius.circular(24),
            side: const BorderSide(color: Color(0xffeaded5)),
          ),
        ),
        filledButtonTheme: FilledButtonThemeData(
          style: FilledButton.styleFrom(
            backgroundColor: seed,
            foregroundColor: Colors.white,
            shape: RoundedRectangleBorder(
              borderRadius: BorderRadius.circular(14),
            ),
          ),
        ),
        outlinedButtonTheme: OutlinedButtonThemeData(
          style: OutlinedButton.styleFrom(
            foregroundColor: const Color(0xff5a341f),
            side: const BorderSide(color: Color(0xffd7c2b5)),
            shape: RoundedRectangleBorder(
              borderRadius: BorderRadius.circular(14),
            ),
          ),
        ),
        snackBarTheme: const SnackBarThemeData(
          behavior: SnackBarBehavior.floating,
        ),
      ),
      home: HomePage(bridge: bridge),
    );
  }
}
