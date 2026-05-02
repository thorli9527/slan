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

## 5. 后续必须补强的点

当前控制面消费、relay / DERP 票据字段、`DerpPool` / `PathManager` 基础路径已经打通。后续还需要补强：

1. 真实网络环境下的 `DerpClient` 故障注入与恢复验证
2. `DerpPool` 评分、选主、切换逻辑的长期运行与诊断覆盖
3. `PathManager` 与 `p2p` / `tunnel` 的更多平台场景打通
4. `ConnectionState` 上报里增加更完整的 DERP 维度信息
5. 将 active path、最近 probe、fallback 原因整理成只读诊断接口

## 6. 建议的实现顺序

1. 固化 `BootstrapConfig.derp_map` 与 cluster-aware relay ticket 的协议回归
2. 扩展 `DerpClient` / `DerpPool` 的真实网络和故障注入能力
3. 完善 `tick_health_check()`、`maybe_switch()` 和 active path 诊断输出
4. 扩展 `PathManager` 与 `p2p` / `tunnel` 联动场景
5. 最后按需把 DERP 诊断状态开放给 Flutter
