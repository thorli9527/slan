# Client 重构清单

本文档用于把当前 `client/` 侧已经铺开的 Flutter、Rust app_core、plugin、多平台 tunnel 接入，重新收口成稳定的模块边界。

目标不是一次性“重写所有客户端代码”，而是先把结构稳住，避免继续把 UI、bridge、runtime、平台实现揉在一起。

## 当前 client 结构

### Flutter app

- `client/app`

关键点：

- 页面集中在 `lib/features`
- 控制面 / bridge / store 混合在 `lib/infra`
- 已经存在 devices 页的 bridge/tunnel debug 和集成测试

### Rust app_core

- `client/app_core`

关键点：

- `app-core` 已经引入 WireGuard-first 共享模型
- `tunnel` 已经有 `TunnelBackend` 抽象和平台 skeleton
- `ffi-bridge` 已经承担 façade、状态持久化、typed error、tunnel key provider
- `p2p` / `relay-client` / `PathManager` 已经切到 transport packet 语义

### Flutter plugin / platform interface

- `client/app_core_plugin`
- `client/app_core_plugin_platform_interface`

关键点：

- 已经有 typed tunnel backend model
- method channel 与 platform interface 仍有进一步收口空间

### 平台插件

- `client/app_core_plugin_macos`
- `client/app_core_plugin_android`
- `client/app_core_plugin_linux`
- `client/app_core_plugin_windows`

关键点：

- macOS 侧已经有最小 tunnel debug 调用链
- 真正的 Packet Tunnel Provider target 还没建
- 其他平台目前主要还是骨架

## 当前主要问题

### 1. Flutter app 的 infra 层过厚

现在 `client/app/lib/infra/app_core` 同时包含：

- API 抽象
- bridge 实现
- mock 实现
- store
- scope
- 数据模型

这让 UI、桥接、测试替身、状态编排混在一个目录里，后续继续扩 tunnel/path/status 会越来越难拆。

### 2. `ffi-bridge` 过于中心化

它现在已经承担：

- façade
- snapshot/state
- data-plane typed error
- tunnel key provider
- probe/send 诊断语义

这层继续增长的话，后面会把 tunnel、path、key management、JSON bridge 再次揉回一个大 crate。

### 3. tunnel 与平台接入尚未真正分层完成

Rust `tunnel` crate 已经做了第一轮抽象，但：

- macOS Packet Tunnel Provider 还没建 target
- platform plugin 还没有形成统一“控制链”
- 当前 plugin 侧仍保留内存态 tunnel backend 逻辑

### 4. 多平台 plugin 骨架不对齐

当前 macOS 侧演进最快，Android/Linux/Windows 还主要是空壳。继续往前推前，需要先把统一职责写清楚，不然每个平台都会各自发明一套 bridge 语义。

## 目标边界

## A. Flutter app 只保留三种职责

1. 页面与交互
2. store / scope 状态拼装
3. 调用统一 `AppCoreApi`

不再在 app 里扩：

- 原生平台 tunnel 安装细节
- Packet Tunnel / Network Extension 控制逻辑
- WireGuard 具体配置拼接规则

## B. `ffi-bridge` 只保留统一 façade

它应该负责：

- `connect / disconnect / status / probe`
- typed runtime model
- snapshot persistence

不应继续增长成：

- 平台 tunnel 安装器
- 原生 Network Extension lifecycle 管理器
- 平台特定 key store 实现集合

## C. `tunnel` crate 只保留统一 tunnel/backend 模型

它应该负责：

- `TunnelConfig`
- `TunnelBackend`
- `TunnelRuntimeView`
- `TunnelManager`

不应直接承载：

- Xcode target
- Android `VpnService`
- Windows service lifecycle
- Linux 内核接口细节实现散落在主 crate

## D. 平台 plugin 只负责原生控制链

平台层应该统一为：

- 接受 typed tunnel config
- 控制原生 backend lifecycle
- 返回 runtime view

而不是：

- 重写 Rust 的 path/tunnel 状态机
- 在原生层复刻业务规则

## 分阶段重构建议

### P0 结构收口

这一阶段先稳结构，不大动运行语义。

1. 给 `client/` 增统一目录说明
2. 明确 `docs/architecture` 下的 client 文档入口
3. 统一 generated artifact 忽略规则
4. 保持现有测试入口不变

这一轮已经覆盖到：

- `client/README.md`
- `docs/architecture/client-refactor-plan.md`
- `docs/architecture/macos-packet-tunnel-provider-plan.md`

### P1 Flutter app 分层

目标：

- 把 `client/app/lib/infra/app_core` 拆成更清晰的子层

建议结构：

- `lib/infra/app_core/api/`
- `lib/infra/app_core/bridge/`
- `lib/infra/app_core/models/`
- `lib/infra/app_core/store/`
- `lib/infra/app_core/testing/`

其中：

- `AppCoreDemoStore` 不应继续和 bridge/api 实现同层平铺
- `models.dart` 后续应拆成：
  - facade/runtime models
  - probe/send failure models
  - tunnel debug models

### P2 Rust app_core 分层

目标：

- 压缩 `ffi-bridge` 的中心化程度

建议拆分方向：

- `ffi-bridge`
  - façade 与 JSON bridge
- `app-core`
  - 纯共享模型
- `tunnel`
  - tunnel config / backend / runtime
- 后续可新增：
  - `app-core-state`
  - `app-core-diagnostics`
  - `app-core-keys`

这一步不要求马上拆 crate，但至少要先停止把新职责继续堆进 `default_facade.rs`。

### P3 平台 plugin 统一 contract

目标：

- 先写清统一 contract，再扩平台实现

统一平台 contract 至少包含：

1. `applyTunnelConfiguration`
2. `removeTunnelPeer`
3. `bringTunnelUp`
4. `bringTunnelDown`
5. `tunnelRuntimeView`

macOS 先按 Packet Tunnel Provider 方案继续推进，其他平台跟随同一 contract，不再各自定义 method 名称和状态字段。

### P4 真正的平台 backend

这一阶段才进入：

- macOS `NEPacketTunnelProvider`
- Linux kernel WireGuard
- Windows `embeddable-dll-service`
- Android `com.wireguard.android:tunnel`

前提是 P0-P3 先把统一边界站住。

## 代码热点与约束

### Flutter

重点文件：

- `client/app/lib/infra/app_core/app_core_demo_store.dart`
- `client/app/lib/infra/app_core/models.dart`
- `client/app/lib/infra/app_core/bridge_app_core_api.dart`
- `client/app/lib/features/devices/devices_page.dart`

要求：

- 页面不要直接绑定 method channel 细节
- failure taxonomy 保持 send/probe/tunnel 一致命名

### Rust

重点文件：

- `client/app_core/crates/ffi-bridge/src/default_facade.rs`
- `client/app_core/crates/tunnel/src/*`
- `client/app_core/crates/relay-client/src/*`

要求：

- 新职责优先拆模块，不继续往一个 runtime 文件里堆
- 平台实现细节不要回流到 `ffi-bridge`

### macOS plugin

重点文件：

- `client/app_core_plugin_macos/macos/Classes/SlanAppCorePluginMacosPlugin.swift`

要求：

- 后续改成 `MacOSTunnelManager -> PacketTunnelProvider`
- 当前 in-memory tunnel backend 只能继续作为过渡态

## 当前建议

如果下一步继续做代码，不建议“全量重构 client 所有代码”。更稳的顺序是：

1. 先做 macOS Packet Tunnel Provider target 骨架
2. 同时把 Flutter `infra/app_core` 做一次目录拆分
3. 再把 `ffi-bridge` 的 key/state/diagnostics 逐步从 `default_facade.rs` 分出去

这三步能最大化降低后面接 WireGuard 真正后端时的返工。
