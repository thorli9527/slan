# Client Local Test Entrypoints

这份说明只覆盖 **本地开发时** 客户端相关测试/检查入口，不涉及 CI。

## 总览

仓库根目录当前统一通过 `make` 暴露客户端本地验证命令：

```bash
make help
```

当前客户端相关入口有：

- `make client-desktop-ui-test`
- `make devices-integration`
- `make cleanup-devices-integration`
- `make macos-tunnel-control-test`
- `make macos-packet-tunnel-build-check`
- `make macos-packet-tunnel-signing-check`

Related notes:

- [local-auth-callback-smoke.md](./local-auth-callback-smoke.md)
- [m2-patch-summary.md](./m2-patch-summary.md)
- [server-biz-local-test-notes.md](./server-biz-local-test-notes.md)

## 1. Flutter 桌面 UI Widget Tests

入口：

```bash
make client-desktop-ui-test
```

实际调用：

```text
./scripts/test_client_desktop_ui.sh
```

当前覆盖：

- `SlanApp` smoke test
- 共享 `desktop_client_widgets` widget tests
- `AuthPage` desktop layout test
- `NetworksPage` desktop layout test
- `HomePage` desktop shell test
- `DevicesPage` desktop workspace test

默认日志位置：

```text
artifacts/client-desktop-ui/flutter-test.log
```

可选覆盖：

```bash
CLIENT_DESKTOP_UI_LOG_DIR=/path/to/logs make client-desktop-ui-test
```

## 2. Devices Integration Tests

入口：

```bash
make devices-integration
```

实际调用：

```text
./scripts/run_devices_integration.sh
```

特点：

- 会先做外部 cleanup
- 会先做 macOS PacketTunnel signing precheck
- 会逐文件顺序执行 `devices_*_flow_test.dart`

默认日志目录：

```text
artifacts/devices-integration/
```

如果上一轮 integration 没收干净，先执行：

```bash
make cleanup-devices-integration
```

## 3. macOS TunnelControl 原生 Swift 测试

入口：

```bash
make macos-tunnel-control-test
```

实际调用：

```text
./scripts/test_macos_tunnel_control.sh
```

用途：

- 验证 `client/app_core_plugin_macos/macos/Classes/TunnelControl/`
- 不依赖完整 Flutter macOS app target
- 走 SwiftPM harness

默认日志位置：

```text
artifacts/macos-tunnel-control/swift-test.log
```

## 4. macOS PacketTunnel Target Build Check

入口：

```bash
make macos-packet-tunnel-build-check
```

实际调用：

```text
./scripts/test_macos_packet_tunnel_target.sh
```

用途：

- 只检查 `PacketTunnel` app extension target 能否构建
- 不拉起完整 `Runner` / Flutter Assemble
- 关闭 code signing，适合快速本地 native build loop

默认日志位置：

```text
artifacts/macos-packet-tunnel/xcodebuild.log
```

## 5. macOS PacketTunnel Signing Precheck

入口：

```bash
make macos-packet-tunnel-signing-check
```

实际调用：

```text
./scripts/check_macos_packet_tunnel_signing.sh
```

用途：

- 检查 `Runner` / `PacketTunnel` entitlement 是否具备
- 检查本机 keychain 是否存在可用 development signing identity
- 在真正跑完整 Flutter macOS integration 之前先挡掉签名问题

## 建议顺序

如果只是改 Flutter 桌面界面：

1. `make client-desktop-ui-test`

如果改了 Devices 页面与集成链路：

1. `make cleanup-devices-integration`
2. `make devices-integration`

如果改了 macOS native tunnel / PacketTunnel：

1. `make macos-tunnel-control-test`
2. `make macos-packet-tunnel-build-check`
3. 如需完整 integration，再执行 `make macos-packet-tunnel-signing-check`
