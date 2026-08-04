import 'dart:async';
import 'dart:ui';

import 'package:flutter/material.dart';

import 'app/slan_client_v2_app.dart';
import 'bridge/client_core_bridge.dart';
import 'bridge/client_ui_diagnostics.dart';

/// Flutter 客户端入口。
///
/// 这里只负责创建真实的 [MethodChannelClientCoreBridge] 并注入根组件；
/// 业务启动、登录、网络开关和平台差异都由 bridge 与页面层处理。
void main() {
  WidgetsFlutterBinding.ensureInitialized();
  final previousFlutterErrorHandler = FlutterError.onError;
  FlutterError.onError = (details) {
    if (previousFlutterErrorHandler != null) {
      previousFlutterErrorHandler(details);
    } else {
      FlutterError.presentError(details);
    }
    ClientUiDiagnostics.unawaitedCriticalLog(
      'flutter.uncaught_error',
      fields: {
        'error': details.exceptionAsString(),
        if (details.stack != null) 'stack': details.stack.toString(),
        if (details.context != null) 'context': details.context.toString(),
      },
    );
  };
  PlatformDispatcher.instance.onError = (error, stack) {
    ClientUiDiagnostics.unawaitedCriticalLog(
      'platform.uncaught_error',
      fields: {'error': error.toString(), 'stack': stack.toString()},
    );
    return true;
  };
  runZonedGuarded(
    () => runApp(SlanClientV2App(bridge: MethodChannelClientCoreBridge())),
    (error, stack) => ClientUiDiagnostics.unawaitedCriticalLog(
      'zone.uncaught_error',
      fields: {'error': error.toString(), 'stack': stack.toString()},
    ),
  );
}
