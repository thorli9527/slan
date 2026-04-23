# Client Integration Test Runner

这份说明只覆盖 `client/app` 下 **devices 页面 integration test 的结构约定**。

如果你只是想找本地执行命令，先看：

- [Client Local Test Entrypoints](./client-local-test-entrypoints.md)

## 背景

macOS `flutter test` 跑 `integration_test` 时，当前环境会出现两类宿主级问题：

- 残留 `flutter_tools.snapshot test integration_test/devices_*_flow_test.dart`
- 残留 `build/macos/Build/Products/Debug/slan_app.app/Contents/MacOS/slan_app`

这会直接导致：

- Flutter startup lock
- `did not complete`
- `No tests were found`

The same class of startup instability can also show up on Windows desktop when multiple `integration_test/devices_*_flow_test.dart` files are passed to a single `flutter test` invocation. The stable mitigation is the same:

- keep one flow per file
- run files sequentially from an external runner
- perform host-level cleanup between files

因此当前做法不是把所有 devices 用例塞回一个大文件，而是：

- 把 devices integration 拆成单文件单用例
- 在脚本层统一做外部 cleanup

## 执行入口

本地执行命令、脚本入口、日志目录这些运行信息已经统一整理在：

- [Client Local Test Entrypoints](./client-local-test-entrypoints.md)

Current stable local entrypoints:

- macOS / Unix: `./scripts/run_devices_integration.sh`
- Windows PowerShell: `powershell -ExecutionPolicy Bypass -File .\scripts\run_devices_integration.ps1`

Avoid treating `flutter test integration_test/file_a.dart integration_test/file_b.dart` as a stable desktop runner entrypoint on Windows.

这里不再重复维护命令清单，避免同一套入口在两份文档里漂移。

## 用例拆分约定

当前 devices integration 采用：

- `devices_bridge_*_flow_test.dart`
- `devices_tunnel_*_flow_test.dart`

每个文件只承载一条用例或一条生命周期，避免同文件多条 `testWidgets` 互相污染 macOS integration runner。

共享脚手架集中在：

```text
client/app/integration_test/devices_flow_test_support.dart
```

对应本地执行时，runner 仍然会逐文件顺序执行这些单文件 suite；具体命令见上面的本地入口文档。

On Windows, the PowerShell runner also performs external `slan_app.exe` cleanup before and after each file so the next suite starts from a clean desktop host state.

## 后续建议

如果后面要继续补 devices 页面 integration，用同样规则：

- 先抽共享 support
- 再新增单文件用例
- 不回退到“大文件多 testWidgets”
