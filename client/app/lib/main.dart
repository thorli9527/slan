/// 应用入口文件。
///
/// 当前阶段只负责启动 Flutter 应用并挂载 [SlanApp]。
library slan_app.main;

import 'package:flutter/material.dart';

import 'app/app.dart';

void main() {
  runApp(const SlanApp());
}
