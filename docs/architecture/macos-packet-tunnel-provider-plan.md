# macOS Packet Tunnel Provider 接入清单

本文档把 macOS `Network Extension + NEPacketTunnelProvider + utun` 接入，落成一份直接对应当前仓库结构的工程清单。

当前前提：

- Flutter macOS 宿主工程位于 `client/app/macos`
- 现有宿主 target 为 `Runner`
- 当前 `Runner` bundle id 为 `com.example.slanApp`
- 当前 tunnel 调试入口已经打到 `client/app_core_plugin_macos`
- 当前 Rust `app_core/tunnel` 已经具备 WireGuard-first 模型和 `TunnelBackend` 抽象

## 结论

对这个项目，macOS 端应按以下路线接入：

- 宿主 app：`Runner`
- Network Extension 形态：`Packet Tunnel Provider`
- 打包形态：
  - Mac App Store 路线可用 `app extension`
  - Developer ID / 非 App Store 路线应优先 `system extension`
- tunnel 虚拟接口：由 `NEPacketTunnelProvider` 提供的 `utun`
- 插件层职责：控制 tunnel lifecycle、下发配置、读取 runtime view
- Rust 职责：只提供 WireGuard config、path/tunnel runtime model，不直接管理 Apple extension 生命周期

参考：

- Apple TN3134: <https://developer.apple.com/documentation/technotes/tn3134-network-extension-provider-deployment>
- Packet tunnel provider: <https://developer.apple.com/documentation/networkextension/packet-tunnel-provider>
- `NEPacketTunnelProvider`: <https://developer.apple.com/documentation/networkextension/nepackettunnelprovider>
- Network Extensions entitlement: <https://developer.apple.com/documentation/bundleresources/entitlements/com.apple.developer.networking.networkextension>
- Configuring network extensions: <https://developer.apple.com/documentation/xcode/configuring-network-extensions>
- System Extensions: <https://developer.apple.com/system-extensions/>

## 推荐打包形态

按 Apple 当前文档，packet tunnel provider 在 macOS 上支持：

- `app extension`，最低 macOS 10.11，但 `App Store only`
- `system extension`，最低 macOS 10.15

当前工程明显不是只面向 Mac App Store，因此推荐从一开始就按 `system extension` 设计，避免后续从 app extension 返工到 system extension。

## 当前本地开发前提

当前仓库已经给 `Runner` 和 `PacketTunnel` 都加上了 Network Extension
entitlements。这带来一个直接结果：

- 原生 target-only build check 可以继续不签名运行：
  - `make macos-packet-tunnel-build-check`
- 但完整的 Flutter macOS app build / integration test 现在需要本地 development signing

仓库里已经提供了一个明确的前置检查入口：

```bash
make macos-packet-tunnel-signing-check
```

这条检查会：

- 校验 `Runner` / `PacketTunnel` entitlements 里确实包含 `packet-tunnel-provider`
- 校验本机 keychain 里是否至少存在一个可用的 code-signing identity

如果失败，就不建议继续直接跑 `flutter test integration_test/...`，而是先在 Xcode
里完成 Team / certificate 配置。

## 当前工程需要新增的 target

在 `client/app/macos/Runner.xcodeproj` 里，至少新增一个 target：

1. `PacketTunnel`
- 类型：Packet Tunnel Provider
- 主类：`PacketTunnelProvider`
- 建议 bundle id：
  - `com.example.slanApp.PacketTunnel`
- 如果最终走 Developer ID/system extension，entitlement 里应使用：
  - `packet-tunnel-provider-systemextension`

建议保留现有 `Runner` target 不改名，只新增 provider target。

## 当前已落地的最小验证面

仓库里已经补了一条独立的 `TunnelControl` 原生验证链：

- 本地入口：`make macos-tunnel-control-test`
- 脚本入口：`./scripts/test_macos_tunnel_control.sh`

它当前覆盖的是：

- `WireGuardTunnelConfigMapper`
- `WireGuardTunnelRuntimeViewMapper`

这层还不是完整的 PacketTunnel/Runner/Xcode target 集成测试，但已经能独立回归 `client/app_core_plugin_macos/macos/Classes/TunnelControl/`。

## 建议新增的目录结构

### 1. `client/app/macos/Runner`

新增这些 Swift 文件：

- `NetworkExtensionManager.swift`
- `TunnelProviderSessionController.swift`
- `TunnelConfigurationStore.swift`
- `TunnelRuntimeBridge.swift`

职责：

- `NetworkExtensionManager`
  - `NETunnelProviderManager` 的加载、保存、启用、状态查询
- `TunnelProviderSessionController`
  - 启动 / 停止 tunnel session
  - 调 `NETunnelProviderSession`
- `TunnelConfigurationStore`
  - 把 Flutter/plugin 传下来的 tunnel config 转成 provider protocol 配置
- `TunnelRuntimeBridge`
  - 把 native runtime state 映射回 plugin 需要的 `tunnelRuntimeView`

### 2. `client/app/macos/PacketTunnel`

新增 Packet Tunnel Provider target 对应文件：

- `PacketTunnelProvider.swift`
- `PacketTunnelWireGuardAdapter.swift`
- `PacketTunnelConfigMapper.swift`
- `PacketTunnelRuntimeState.swift`
- `Info.plist`
- `PacketTunnel.entitlements`

职责：

- `PacketTunnelProvider`
  - 继承 `NEPacketTunnelProvider`
  - 实现 `startTunnel`, `stopTunnel`, `handleAppMessage`
  - 配置 `NEPacketTunnelNetworkSettings`
  - 管理 `packetFlow`
- `PacketTunnelWireGuardAdapter`
  - 后续真正接 `WireGuardKit` 或 wireguard-apple engine
  - 第一阶段可以先是最小 no-op / staged adapter
- `PacketTunnelConfigMapper`
  - 解析 `NETunnelProviderProtocol.providerConfiguration`
  - 映射到项目已有的 `WireGuardTunnelConfiguration`
- `PacketTunnelRuntimeState`
  - 暴露：
    - `state`
    - `interfaceName`
    - `peerVirtualIp`
    - `selectedEndpoint`
    - `lastHandshakeAt`
    - `transport`

### 3. `client/app_core_plugin_macos/macos/Classes`

建议新增一个子目录：

- `TunnelControl/`
  - `MacOSTunnelManager.swift`
  - `PacketTunnelProviderBridge.swift`
  - `TunnelRuntimeViewMapper.swift`

职责：

- `MacOSTunnelManager`
  - plugin 层入口
  - 对接 `Runner` 宿主侧的 `NetworkExtensionManager`
- `PacketTunnelProviderBridge`
  - 统一调用：
    - `applyTunnelConfiguration`
    - `bringTunnelUp`
    - `bringTunnelDown`
    - `removeTunnelPeer`
    - `tunnelRuntimeView`
- `TunnelRuntimeViewMapper`
  - 把 `NETunnelProviderManager` / provider message 返回的数据映射到 Dart 模型

当前 plugin 里那套 in-memory `WireGuardKitAdapter` 只适合当临时骨架，后续应退场，改成“控制 Packet Tunnel Provider”。

## 需要新增或修改的 plist / entitlement

### 1. `Runner/DebugProfile.entitlements`

当前已有：

- `com.apple.security.app-sandbox`
- `com.apple.security.cs.allow-jit`
- `com.apple.security.network.server`

后续应新增：

- `com.apple.developer.networking.networkextension`

建议值：

- 如果走 system extension / Developer ID：
  - `packet-tunnel-provider-systemextension`
- 如果只走 Mac App Store app extension：
  - `packet-tunnel-provider`

### 2. `Runner/Release.entitlements`

同上，至少补：

- `com.apple.developer.networking.networkextension`

### 3. `PacketTunnel.entitlements`

Packet Tunnel Provider target 也需要自己的 Network Extension entitlement。

建议值与宿主 app 保持一致：

- `packet-tunnel-provider-systemextension`

### 4. `PacketTunnel/Info.plist`

核心字段：

- `CFBundleIdentifier = com.example.slanApp.PacketTunnel`
- `NSExtension`
  - `NSExtensionPointIdentifier = com.apple.networkextension.packet-tunnel`
  - `NSExtensionPrincipalClass = $(PRODUCT_MODULE_NAME).PacketTunnelProvider`

### 5. `Runner/Info.plist`

通常不需要大改 packet tunnel 专属字段，但如果后续 UI 或权限提示需要，可以在这里补本地描述字段。

## Xcode 工程里需要做的改动

在 `client/app/macos/Runner.xcodeproj/project.pbxproj` 里，后续至少要补这些内容：

1. 新增 `PBXNativeTarget`
- 名字：`PacketTunnel`
- product type：Packet Tunnel Provider

2. 新增这些文件引用和 build phase
- `PacketTunnelProvider.swift`
- `PacketTunnelWireGuardAdapter.swift`
- `PacketTunnelConfigMapper.swift`
- `PacketTunnelRuntimeState.swift`
- `PacketTunnel/Info.plist`
- `PacketTunnel/PacketTunnel.entitlements`

3. 新增 target build settings
- `INFOPLIST_FILE = PacketTunnel/Info.plist`
- `CODE_SIGN_ENTITLEMENTS = PacketTunnel/PacketTunnel.entitlements`
- `PRODUCT_BUNDLE_IDENTIFIER = com.example.slanApp.PacketTunnel`
- `MACOSX_DEPLOYMENT_TARGET = 10.15`

4. 宿主 `Runner` target 里新增 embed / dependency
- 让 `Runner` 能安装和持有该 provider target

## plugin 调用链应该如何改

当前：

- Dart `applyTunnelConfiguration(...)`
- `SlanAppCorePluginMacosPlugin.swift`
- in-memory `WireGuardKitAdapter`

后续应改成：

- Dart `applyTunnelConfiguration(...)`
- `SlanAppCorePluginMacosPlugin.swift`
- `MacOSTunnelManager`
- `NETunnelProviderManager`
- `PacketTunnelProvider`

也就是说，plugin 层不再直接“模拟 tunnel backend”，而是控制真实的 Packet Tunnel Provider lifecycle。

## PacketTunnelProvider 第一阶段最小实现

第一阶段不要急着接真正的 WireGuard 数据面，先把系统 tunnel 生命线打通：

1. `startTunnel(options:)`
- 从 `providerConfiguration` 取：
  - local virtual IP
  - peer virtual IP
  - selected endpoint
  - MTU
  - DNS
- 调 `setTunnelNetworkSettings(...)`
- 记录 runtime state

2. `stopTunnel(with:)`
- 清理 runtime state

3. `handleAppMessage(_:)`
- 支持至少两个命令：
  - `runtimeView`
  - `removePeer`

4. 这阶段先不做：
- 真实 `packetFlow` -> WireGuard engine -> UDP path 收发
- 真正 `WireGuardKit` 或 wireguard-apple 接入

目标是先确认：

- target 可编译
- tunnel 可启动
- utun/network settings 可下发
- plugin 可读到 runtime state

## 第二阶段才接 WireGuard engine

当 Packet Tunnel Provider 最小生命周期跑通后，再接：

- `WireGuardKit`
- 或 wireguard-apple 对应 engine

接入点建议固定在：

- `PacketTunnelWireGuardAdapter.swift`

这样不会污染：

- `PacketTunnelProvider.swift`
- plugin 层
- Rust `app_core`

## 和现有 Rust 代码的边界

Rust 侧继续保留：

- `WireGuardInterfaceConfig`
- `WireGuardPeerConfig`
- `TunnelBackend`
- `TunnelRuntimeView`
- `TunnelKeyProvider`

Rust 侧不负责：

- `NETunnelProviderManager`
- `NEPacketTunnelProvider`
- Xcode target / plist / entitlement
- System Extension / Network Extension 安装与签名

## 实施顺序

### P0

- 新增 `PacketTunnel` target 骨架
- 补 `Info.plist`
- 补 `PacketTunnel.entitlements`
- 补 `Runner` 的 Network Extension entitlement

### P1

- 新增 `NetworkExtensionManager.swift`
- 新增 `TunnelProviderSessionController.swift`
- 让 plugin 的 tunnel 方法走 `MacOSTunnelManager`

### P2

- `PacketTunnelProvider` 最小 `startTunnel / stopTunnel / handleAppMessage`
- 能返回 `tunnelRuntimeView`

### P3

- 接 `WireGuardKit` / wireguard-apple
- 真正开始读写 `packetFlow`

## 当前工程里最值的下一步

如果按当前仓库状态继续推进，最值的不是改 Rust，而是：

1. 在 `client/app/macos` 新增 `PacketTunnel` target 骨架
2. 给 `Runner` 补 Network Extension entitlement
3. 把 `client/app_core_plugin_macos` 的 tunnel 分支改成调用一个新的 `MacOSTunnelManager`

做到这一步后，macOS 侧就从“plugin 内存态假 tunnel”推进成“真实 Packet Tunnel Provider 控制链”了。
