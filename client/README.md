# Client Layout

客户端代码当前分成四层：

## 1. Flutter app

- `client/app`

职责：

- 页面与交互
- store / scope
- 控制面 API 调用
- AppCore bridge 接入

当前主要目录：

- `lib/features`
- `lib/infra`
- `integration_test`
- `macos`

## 2. Rust app_core

- `client/app_core`

职责：

- 控制面 DTO / facade
- P2P / relay / derp / tunnel runtime
- WireGuard-first 共享模型
- CLI / helper

当前 crate：

- `app-core`
- `controller-client`
- `ffi-bridge`
- `p2p`
- `relay-client`
- `tunnel`
- `app-core-cli`
- `app-core-helper`

## 3. Flutter plugin surface

- `client/app_core_plugin`
- `client/app_core_plugin_platform_interface`

职责：

- Dart 侧平台抽象
- typed tunnel / bridge API 暴露

## 4. 平台插件实现

- `client/app_core_plugin_macos`
- `client/app_core_plugin_android`
- `client/app_core_plugin_linux`
- `client/app_core_plugin_windows`

职责：

- 各平台原生桥接
- tunnel backend / helper / method channel 接入

## 当前重构方向

近期 client 侧的重点不是继续堆页面功能，而是收口这些边界：

1. Flutter app 只保留 UI、store、bridge 组合，不继续下沉平台细节
2. `ffi-bridge` 只保留统一 façade 和 typed runtime/status 语义
3. `tunnel` crate 只定义统一 tunnel/backend 模型，不夹带平台安装细节
4. 各平台插件只负责原生控制链，不在 plugin 里重复实现业务状态机

更完整的清单见：

- [docs/architecture/client-refactor-plan.md](../docs/architecture/client-refactor-plan.md)
- [docs/architecture/macos-packet-tunnel-provider-plan.md](../docs/architecture/macos-packet-tunnel-provider-plan.md)

## 本地测试入口

客户端本地测试/构建入口已经单独整理在：

- [docs/architecture/client-local-test-entrypoints.md](../docs/architecture/client-local-test-entrypoints.md)
