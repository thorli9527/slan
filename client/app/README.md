# slan_app

SLAN 的 Flutter 桌面客户端。

## AppCore Modes

默认模式仍然是本地 mock。

如果要切到真实控制面 HTTP：

```bash
flutter run -d macos \
  --dart-define=SLAN_CONTROL_BASE_URL=http://127.0.0.1:8080
```

如果要切到 Rust facade bridge：

1. 先构建 Rust helper：

```bash
cd ../app_core
cargo build -p app-core-helper
```

2. 再启动 macOS 客户端：

```bash
cd ../app
SLAN_APP_CORE_HELPER=/Users/thorli/workspace/slan/client/app_core/target/debug/app-core-helper \
SLAN_CONTROL_BASE_URL=http://127.0.0.1:8080 \
flutter run -d macos --dart-define=SLAN_APP_CORE_MODE=bridge
```

说明：

- `SLAN_APP_CORE_MODE=bridge` 让 Dart 侧走 `MethodChannel('slan/app_core')`
- `SLAN_APP_CORE_HELPER` 指向常驻 Rust helper 可执行文件
- `SLAN_CONTROL_BASE_URL` 会透传给 helper，由 Rust `HttpControllerClient` 访问控制面

## 本地测试入口

客户端本地测试/构建命令已经统一整理在：

- [docs/architecture/client-local-test-entrypoints.md](../../docs/architecture/client-local-test-entrypoints.md)

如果只是想快速回归当前 Flutter 桌面客户端，最常用的是：

```bash
make client-desktop-ui-test
```

如果要跑 Devices 页面 integration：

```bash
make devices-integration
```
