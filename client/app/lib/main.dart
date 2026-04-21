/// 应用入口文件。
///
/// 当前阶段只负责启动 Flutter 应用并挂载 [SlanApp]。
library slan_app.main;

import 'dart:async';

import 'package:flutter/material.dart';

import 'app/app.dart';
import 'features/auth/auth_callback_service.dart';
import 'infra/app_core/scope/app_core_scope.dart';
import 'infra/logging/startup_log.dart';

Future<void> main() async {
  WidgetsFlutterBinding.ensureInitialized();
  await StartupLog.reset();
  await StartupLog.write('main start');
  final initialization = AppCoreScope.initialize();
  await StartupLog.write('runApp');
  FlutterError.onError = (details) {
    unawaited(StartupLog.write('flutter error: ${details.exceptionAsString()}'));
    FlutterError.presentError(details);
  };
  runApp(SlanApp(initialization: initialization));
  unawaited(
    AuthCallbackService.ensureInitialized().then(
      (_) => StartupLog.write('auth callback init done'),
      onError: (Object error, StackTrace _) =>
          StartupLog.write('auth callback init failed: $error'),
    ),
  );
}
