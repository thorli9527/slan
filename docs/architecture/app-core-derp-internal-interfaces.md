# AppCore 内部需求接口

本文档将前一版 DERP/集群设计进一步收敛到 `client/app_core` 的内部接口层，明确：

- 哪些类型应该进入 `crates/app-core`
- 哪些控制面请求应该进入 `crates/controller-client`
- 哪些连接池与路径管理能力应该进入 `crates/relay-client`
- 哪些能力暂时不进入 Flutter FFI

## 1. 目标

`app_core` 需要承接以下内部能力：

- 从控制面获取 `bootstrap + derp_map + ticket`
- 建立 2~3 个 DERP 热连接
- 维护一个 `derp_pool`
- 对上层只暴露一个 active 发送路径
- 以 RTT、超时、丢包为依据进行切换

这意味着 `app_core` 不能只保留“单个 relay ticket + 单次 connect”接口，必须具备：

- DERP 集群元数据模型
- DERP 健康模型
- DERP 连接池状态模型
- DERP 单连接与池化 trait
- 路径管理 trait

## 2. crate 责任划分

### `crates/app-core`

承载纯模型定义：

- `DerpTransport`
- `DerpNodeMeta`
- `DerpCluster`
- `DerpMap`
- `ProbeSample`
- `DerpHealth`
- `DerpLinkSnapshot`
- `DerpPoolState`
- `ActivePath`
- `SwitchReason`
- `DerpSwitchEvent`

同时扩展：

- `BootstrapConfig.derp_map`
- `RelayTicket.derp_cluster_id`
- `RelayTicket.allowed_derp_node_ids`
- `ConnectionPath::Derp`

### `crates/controller-client`

承载控制面请求与响应接口。

当前最少需要：

- `bootstrap()` 返回带 `derp_map` 的 `BootstrapConfig`
- `issue_relay_ticket()` 扩展支持：
  - `derp_cluster_id`
  - `preferred_derp_node_ids`

后续如果控制面拆出独立 DERP ticket 接口，可再新增：

- `issue_derp_ticket()`

### `crates/relay-client`

承载数据面回退能力抽象。

新增三层接口：

- `DerpClient`
- `DerpPool`
- `PathManager`

保留 `RelayClient` 作为旧抽象或兼容入口，但新实现应优先围绕 `DerpPool` 组织。

## 3. 当前已补齐的接口

### 3.1 `crates/app-core`

当前已定义：

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

### 3.2 `crates/controller-client`

当前 `RelayTicketRequest` 已扩展：

- `derp_cluster_id`
- `preferred_derp_node_ids`

### 3.3 `crates/relay-client`

当前已定义：

- `DerpClient`
- `DerpPool`
- `PathManager`

## 4. 暂不进入 FFI 的接口

以下接口先保留在 `app_core` 内部，不直接暴露给 Flutter：

- `DerpClient`
- `DerpPool`
- `PathManager`
- `DerpLinkSnapshot`
- `DerpPoolState`
- `DerpSwitchEvent`

原因：

- Flutter 当前只需要“连接状态”和“最终生效路径”
- DERP 池内部状态属于诊断和调优数据
- 先在 Rust 核心收敛，再决定是否开放给 UI

## 5. 后续必须补实现的点

当前只是接口定义，后续还需要补：

1. `controller-client` 的真实 HTTP 实现
2. `DerpClient` 的真实连接实现
3. `DerpPool` 的评分、选主、切换逻辑
4. `PathManager` 与 `p2p` / `tunnel` 的打通
5. `ConnectionState` 上报里增加 DERP 维度信息

## 6. 建议的实现顺序

1. 先让 `BootstrapConfig` 真正带回 `derp_map`
2. 再让 `issue_relay_ticket()` 支持 cluster-aware ticket
3. 实现单个 `DerpClient`
4. 实现 `DerpPool.warm_up()` 和 `send_via_active()`
5. 实现 `tick_health_check()` 和 `maybe_switch()`
6. 最后接到 `PathManager`
