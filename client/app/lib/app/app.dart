/// Flutter 应用根组件。
///
/// 这里负责统一配置 MaterialApp、主题以及首页路由。
import 'package:flutter/material.dart';

import '../features/home/home_page.dart';

class SlanApp extends StatelessWidget {
  const SlanApp({super.key});

  @override
  Widget build(BuildContext context) {
    return MaterialApp(
      title: 'SLAN',
      theme: ThemeData(useMaterial3: true, colorSchemeSeed: Colors.blue),
      home: const HomePage(),
    );
  }
}
