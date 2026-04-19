/// Flutter 应用根组件。
///
/// 这里负责统一配置 MaterialApp、主题以及首页路由。
library slan_app.app;

import 'package:flutter/material.dart';

import '../features/home/home_page.dart';

class SlanApp extends StatelessWidget {
  const SlanApp({super.key});

  @override
  Widget build(BuildContext context) {
    final colorScheme = ColorScheme.fromSeed(
      seedColor: const Color(0xFF1E6B52),
      brightness: Brightness.light,
    );
    return MaterialApp(
      title: 'SLAN',
      debugShowCheckedModeBanner: false,
      theme: ThemeData(
        useMaterial3: true,
        colorScheme: colorScheme,
        scaffoldBackgroundColor: const Color(0xFFF4F7F1),
        appBarTheme: const AppBarTheme(
          backgroundColor: Color(0xFFF4F7F1),
          surfaceTintColor: Colors.transparent,
        ),
        inputDecorationTheme: InputDecorationTheme(
          filled: true,
          fillColor: Colors.white,
          border: OutlineInputBorder(
            borderRadius: BorderRadius.circular(16),
            borderSide: BorderSide(color: colorScheme.outlineVariant),
          ),
          enabledBorder: OutlineInputBorder(
            borderRadius: BorderRadius.circular(16),
            borderSide: BorderSide(color: colorScheme.outlineVariant),
          ),
          focusedBorder: OutlineInputBorder(
            borderRadius: BorderRadius.circular(16),
            borderSide: BorderSide(
              color: colorScheme.primary,
              width: 1.4,
            ),
          ),
        ),
      ),
      home: const HomePage(),
    );
  }
}
