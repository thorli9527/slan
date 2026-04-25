# app_core 内部需求

## Maintained Main-Flow Status

`client/app_core` now has real control-plane access for the main client flow:

- auth/session refresh
- device and node registration
- network create, join-by-owner-email, join-by-key, alias remark, switch,
  activate, and deactivate
- bootstrap/control sync
- relay ticket issue and relay/DERP fallback routing tests

Remaining requirements in this document are production runtime, diagnostics,
and cross-platform tunnel hardening unless they explicitly call out a missing
API.

本文档描述 `client/app_core` 在系统中的内部职责、模块拆分和后续必须实现的核心能力。

## 1. 定位

`app_core` 是 Flutter 应用与底层组网核心之间的 Rust 中间层，负责：

- 承接控制面身份与配置
- 承接 NAT / P2P / relay / DERP 等连接逻辑
- 向上提供稳定门面
- 向下隔离网络实现细节

它本身不是 UI，也不是完整控制面客户端，而是“客户端运行时编排层”。

## 2. 内部核心职责

### 2.1 账户与身份

- 注册与登录
- 设备注册
- 节点注册
- 保存当前运行会话所需的身份信息

### 2.2 网络配置获取

- 查询可见网络
- 创建网络
- 让设备加入网络
- 按宿主邮箱或 join key 加入网络
- 激活 / 停用当前选中的网络
- 维护当前设备 attachment 备注，用于保存加入网络时的别名
- 拉取 `bootstrap`

### 2.3 运行时连接管理

- NAT 探测
- 候选路径收集
- P2P 直连尝试
- relay / DERP 回退
- 隧道建立与关闭

### 2.4 DERP/集群能力

- 解析控制面返回的 `derp_map`
- 构建 `derp_pool`
- 建立 2~3 个 DERP 热连接
- 只选择一个 active 节点发送
- 每 5 秒执行 RTT / 超时 / 丢包探测
- 基于评分触发切换

## 3. crate 划分

当前工作区：

- `crates/app-core`
  共享模型定义
- `crates/controller-client`
  控制面客户端 trait 与请求模型
- `crates/nat`
  NAT 探测抽象
- `crates/p2p`
  P2P 候选与连接抽象
- `crates/relay-client`
  relay / DERP 回退连接与路径管理抽象
- `crates/tunnel`
  隧道抽象
- `crates/ffi-bridge`
  对 Flutter 暴露统一门面

## 4. 当前内部需求模型

`crates/app-core` 当前应承载这些内部模型：

- `Session`
- `Device`
- `Node`
- `Network`
- `BootstrapConfig`
- `RelayTicket`
- `ConnectionState`
- `DerpTransport`
- `DerpNodeMeta`
- `DerpCluster`
- `DerpMap`
- `ProbeSample`
- `DerpLinkState`
- `DerpHealth`
- `DerpLinkSnapshot`
- `DerpPoolState`
- `ActivePath`
- `SwitchReason`
- `DerpSwitchEvent`

这些模型属于运行时内部语义，UI 不一定全部可见。

## 5. 当前内部 trait 需求

### `controller-client`

必须支持：

- `register`
- `login`
- `refresh_session`
- `register_device`
- `list_devices`
- `register_node`
- `list_networks`
- `create_network`
- `join_network`
- `join_network_by_owner_email`
- `join_network_by_key`
- `switch_network`
- `activate_network`
- `deactivate_network`
- `update_attachment_remark`
- `bootstrap`
- `issue_relay_ticket`

DERP 场景下还要求：

- `bootstrap()` 能返回 `derp_map`
- `issue_relay_ticket()` 能携带 `derp_cluster_id`
- `issue_relay_ticket()` 能携带 `preferred_derp_node_ids`

### `relay-client`

当前内部最少应包含四层能力：

- `RelayClient`
  单次 relay 回退入口
- `DerpClient`
  单个 DERP 连接
- `DerpPool`
  多连接热备、评分、切换
- `PathManager`
  统一管理 `p2p / relay / derp`

### `p2p`

必须提供：

- `PeerCandidate`
- `P2PConnector::connect`

后续应与 `PathManager` 联动，而不是单独对外暴露连接结论。

### `tunnel`

必须提供：

- `TunnelConfig`
- `TunnelManager::establish`
- `TunnelManager::close`

后续应由 `PathManager` 决定“通过哪条路径承载隧道数据”。

## 6. 暂不暴露给 Flutter 的内部能力

以下内容先定义在 Rust 内部，不直接暴露给 Flutter：

- `DerpClient`
- `DerpPool`
- `PathManager`
- `DerpLinkSnapshot`
- `DerpPoolState`
- `DerpSwitchEvent`

原因：

- 它们是运行时调度与诊断语义
- UI 当前只需要稳定的连接状态与结果
- 先在内核层收敛再开放，能避免 FFI 过早固化

## 7. 当前缺口

当前主业务接口已经有真实控制面接入和测试覆盖；剩余缺口主要集中在更完整的数据面运行时与诊断能力：

1. 更完整的 `DerpClient` / `DerpPool` 生产环境连接与诊断覆盖
2. `PathManager` 与 `p2p` / `relay-client` / `tunnel` 的更多真实场景打通
3. 上报控制面的连接状态扩展
4. 跨平台隧道运行时的安装、权限和故障恢复路径

## 8. 建议实现顺序

1. 固化当前 join / switch / activate / bootstrap 主流程的回归测试
2. 扩展真实 DERP 连接与健康检查覆盖
3. 完成更多 `PathManager` 与隧道联动场景
4. 完善连接状态上报与诊断接口
5. 最后再考虑是否把 DERP 诊断状态开放给 Flutter
