/// AppCore 依赖注入入口。
///
/// 默认仍挂接 mock；联调时可切到真实控制面 HTTP 或 facade bridge。
library slan_app.infra.app_core.scope;

import 'package:flutter/foundation.dart';

import '../api/app_core_api.dart';
import '../bridge/app_core_bridge.dart';
import '../store/app_core_demo_store.dart';
import '../bridge/bridge_app_core_api.dart';
import '../api/http_app_core_api.dart';
import '../api/mock_app_core_api.dart';

class AppCoreScope {
  AppCoreScope._();

  static const String _appCoreMode =
      String.fromEnvironment('SLAN_APP_CORE_MODE');
  static const String _controlBaseUrl =
      String.fromEnvironment('SLAN_CONTROL_BASE_URL');

  static AppCoreApi _instance = switch (_appCoreMode) {
    'bridge' => BridgeAppCoreApi(bridge: MethodChannelAppCoreBridge()),
    _ => _controlBaseUrl.isEmpty
        ? MockAppCoreApi()
        : HttpAppCoreApi(baseUrl: _controlBaseUrl),
  };
  static AppCoreDemoStore _demo = AppCoreDemoStore();

  static AppCoreApi get instance => _instance;
  static AppCoreDemoStore get demo => _demo;

  @visibleForTesting
  static void configureForTest({
    required AppCoreApi appCoreApi,
  }) {
    _instance = appCoreApi;
    _demo = AppCoreDemoStore();
  }

  @visibleForTesting
  static void resetForTest() {
    _instance = switch (_appCoreMode) {
      'bridge' => BridgeAppCoreApi(bridge: MethodChannelAppCoreBridge()),
      _ => _controlBaseUrl.isEmpty
          ? MockAppCoreApi()
          : HttpAppCoreApi(baseUrl: _controlBaseUrl),
    };
    _demo = AppCoreDemoStore();
  }
}
