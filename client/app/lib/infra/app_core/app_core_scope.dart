/// AppCore 依赖注入入口。
///
/// 当前阶段默认挂接 mock 实现，后续可以替换为真实 FFI/Rust 实现。
import 'app_core_api.dart';
import 'app_core_demo_store.dart';
import 'mock_app_core_api.dart';

class AppCoreScope {
  AppCoreScope._();

  static final AppCoreApi instance = MockAppCoreApi();
  static final AppCoreDemoStore demo = AppCoreDemoStore();
}
