# DERP 集群与 `derp_pool` 设计

本文档定义客户端引入 DERP 集群后的整体设计，目标是支持：

- 多连接：客户端同时连接 2~3 个 DERP 节点
- 单路径发送：业务数据只走一个 active 节点
- 动态切换：其余节点作为热备，按健康度自动切换

示例：

```text
derp_pool
├── tokyo(active)
├── singapore(standby)
└── hongkong(standby)
```

运行原则：

- 始终只走 `active` 节点发送业务包
- `standby` 节点维持热连接与探测，不发送业务流
- 每 5 秒执行一次健康探测与评分
- 当 active 节点超时、RTT 恶化或丢包严重时，切换到最优 standby

## 1. 设计目标

### 1.1 业务目标

- P2P 失败后，DERP 作为稳定兜底路径
- 节点切换对上层连接尽量透明
- 切换时不需要重新完整建连
- 控制面可以下发 DERP 集群视图，客户端自主选择最优节点

### 1.2 工程目标

- 将“选哪个 DERP 节点”从单点配置升级为池化调度
- 将“连上 relay 就发流量”升级为“多连接保温 + 单路径发送”
- 将切换策略内聚到客户端核心，不让 UI 或上层业务感知

## 2. 核心概念

### 2.1 DERP 集群

一个 DERP 集群由多个可替换节点组成，客户端可同时接入其中多个节点。

建议定义：

- `cluster_id`: 集群 ID
- `region_id`: 区域 ID，例如 `ap-east`
- `node_id`: DERP 节点 ID，例如 `tokyo-a`
- `priority`: 控制面推荐优先级
- `transport`: 当前先按 UDP/TCP 之一落地，后续可扩为 QUIC

### 2.2 `derp_pool`

`derp_pool` 是客户端维护的 DERP 连接池，负责：

- 建立和维护 2~3 个 DERP 节点连接
- 记录每个连接的实时健康数据
- 选出唯一 active 节点
- 在需要时切换 active

### 2.3 单路径发送

在任意时刻：

- 仅 active 连接承担业务包发送
- standby 连接只承担心跳、探测、接收能力校验
- 不做多发，不做包级复制

这样做的目的：

- 避免重复包和乱序处理复杂度
- 保持路径控制简单
- 先把“快速切换”做好，再考虑多路径冗余

## 3. 整体架构

```text
app_core
├── controller-client
│   └── 拉取 bootstrap / derp map / ticket
├── derp-client
│   └── 单个 DERP 连接、收发、探测
├── derp-pool
│   └── 多连接管理、评分、选主、切换
├── path-manager
│   └── 统一管理 p2p / derp active path
└── tunnel
    └── 始终向当前 active path 发包
```

服务端：

```text
service-biz
├── bootstrap 中返回 derp 集群视图
├── 按节点/集群签发 derp ticket
└── 控制面可返回推荐节点列表

server-wire-relay / server-wire-derp cluster
├── 多个 relay / derp 节点
├── 节点间共享会话认证语义
└── 同一集群内允许客户端快速切换
```

## 4. 控制面设计

## 4.1 Bootstrap 新增 DERP 视图

当前 `bootstrap` 已包含 relay / DERP 集群视图；本节保留字段结构说明，后续重点是生产环境配置、探测和多节点运行时验证。

建议新增结构：

```text
DerpMap
├── clusters[]
│   ├── clusterId
│   ├── regionId
│   ├── regionName
│   ├── nodes[]
│   │   ├── nodeId
│   │   ├── host
│   │   ├── port
│   │   ├── transport
│   │   ├── priority
│   │   └── tags[]
│   └── recommendedFanout = 3
└── probeIntervalSeconds = 5
```

推荐原则：

- 控制面返回一个可选大集合
- 客户端按区域、优先级、历史 RTT 选出 2~3 个实际连接对象

### 4.2 Ticket 设计

如果要支持 active/standby 快切，ticket 不应只绑定一个具体 DERP 节点，建议绑定到“集群范围”。

建议 ticket 字段：

```text
DerpTicket
├── ticketId
├── networkId
├── sessionId
├── srcNodeId
├── dstNodeId
├── derpClusterId
├── allowedDerpNodeIds[]
├── expiresAt
├── sessionKey
└── signature
```

设计要点：

- `derpClusterId` 作为主授权范围
- `allowedDerpNodeIds` 可选，用于精细控制白名单
- 客户端切换节点时，不必重新申请 ticket
- 同集群切换只需重新 attach 新节点

### 4.3 控制面推荐字段

控制面可在 `ConnectPlan` 里下发：

- 推荐集群
- 推荐 active 候选顺序
- 是否允许客户端本地自由切换

例如：

```text
ConnectPlan
├── preferDirect
├── derpClusterId
├── preferredDerpNodeIds = [tokyo-a, singapore-a, hongkong-a]
└── relayTicket
```

## 5. 客户端设计

## 5.1 模块拆分

### `derp-client`

单个 DERP 连接的职责：

- 建立连接
- 完成 ticket attach
- 发送与接收数据包
- 发送 heartbeat / probe
- 上报 RTT、超时、丢包等健康指标

### `derp-pool`

池化管理职责：

- 维护 2~3 个 `derp-client`
- 选出唯一 active
- 周期性打分
- 触发切换
- 对外暴露统一 `send()` 接口

### `path-manager`

统一路径管理职责：

- 管理 `p2p` 和 `derp_pool`
- 对 tunnel 暴露当前 active path
- 当 p2p 失败时切换到 derp
- 当 p2p 恢复时可选择切回

## 5.2 客户端内部数据结构

建议新增核心结构：

```rust
pub struct DerpNodeMeta {
    pub cluster_id: String,
    pub region_id: String,
    pub node_id: String,
    pub host: String,
    pub port: u16,
    pub transport: DerpTransport,
    pub priority: u32,
}

pub enum DerpLinkState {
    Connecting,
    Ready,
    Suspect,
    Failed,
    Closed,
}

pub struct DerpHealth {
    pub rtt_ms_ewma: u32,
    pub loss_ppm: u32,
    pub timeout_count: u32,
    pub consecutive_failures: u32,
    pub last_probe_at_ms: u64,
    pub last_recv_at_ms: u64,
    pub score: u32,
}

pub struct DerpLinkSnapshot {
    pub meta: DerpNodeMeta,
    pub state: DerpLinkState,
    pub health: DerpHealth,
    pub is_active: bool,
}
```

`derp_pool` 本体：

```rust
pub struct DerpPoolState {
    pub cluster_id: String,
    pub session_id: String,
    pub active_node_id: Option<String>,
    pub links: Vec<DerpLinkSnapshot>,
    pub switch_epoch: u64,
}
```

## 5.3 内部 trait 设计

### 单连接接口

```rust
pub trait DerpClient: Send + Sync {
    fn connect(&self, meta: &DerpNodeMeta) -> Result<(), DerpError>;
    fn attach(&self, ticket: &DerpTicket) -> Result<(), DerpError>;
    fn send(&self, packet: &[u8]) -> Result<(), DerpError>;
    fn recv_next(&self) -> Result<Option<Vec<u8>>, DerpError>;
    fn probe(&self) -> Result<ProbeSample, DerpError>;
    fn snapshot(&self) -> DerpLinkSnapshot;
    fn close(&self) -> Result<(), DerpError>;
}
```

### 连接池接口

```rust
pub trait DerpPool: Send + Sync {
    fn install_cluster(
        &self,
        cluster_id: &str,
        nodes: Vec<DerpNodeMeta>,
        ticket: DerpTicket,
    ) -> Result<(), DerpError>;

    fn warm_up(&self, fanout: usize) -> Result<(), DerpError>;
    fn active_link(&self) -> Option<DerpLinkSnapshot>;
    fn send_via_active(&self, packet: &[u8]) -> Result<(), DerpError>;
    fn tick_health_check(&self) -> Result<(), DerpError>;
    fn maybe_switch(&self) -> Result<Option<SwitchEvent>, DerpError>;
    fn snapshots(&self) -> Vec<DerpLinkSnapshot>;
    fn close(&self) -> Result<(), DerpError>;
}
```

### 路径管理接口

```rust
pub trait PathManager: Send + Sync {
    fn on_p2p_failed(&self, peer_node_id: &str, reason: &str) -> Result<(), PathError>;
    fn on_p2p_recovered(&self, peer_node_id: &str) -> Result<(), PathError>;
    fn current_path(&self) -> ActivePath;
    fn send(&self, packet: &[u8]) -> Result<(), PathError>;
}
```

## 6. 连接建立流程

## 6.1 DERP 进入流程

```text
P2P 失败
-> controller-client 获取 derp ticket + derp map
-> derp-pool 选择 2~3 个候选节点
-> 并行建立连接
-> attach ticket
-> 计算初始 RTT / 丢包评分
-> 选出 active
-> tunnel 开始只向 active 发流量
```

## 6.2 初始选点策略

建议分两步：

1. 控制面先返回候选集和推荐顺序
2. 客户端在本地再做一次筛选

客户端筛选规则建议：

- 优先同区域或低时延区域
- 按 `priority` 初步排序
- 从前 N 个里取 2~3 个建立热连接

例如：

```text
候选: tokyo, singapore, hongkong, frankfurt
实际 warm up: tokyo, singapore, hongkong
```

## 7. 健康探测与评分

## 7.1 探测周期

固定每 5 秒执行一次：

- RTT 探测
- 超时统计
- 丢包估计
- 最近数据接收时间检查

standby 也要执行探测，否则切换时没有实时判断依据。

## 7.2 指标定义

### RTT

- 使用 EWMA 平滑
- 避免瞬时抖动导致频繁切换

### 超时

- 连续 probe timeout 计数
- 超过阈值直接判定为 `Suspect` 或 `Failed`

### 丢包

建议用近 30 秒滑动窗口估计：

- `sent_probe`
- `acked_probe`
- `loss_rate = 1 - acked / sent`

## 7.3 评分公式

可先采用简单线性评分：

```text
score =
  rtt_ms_ewma * 1
  + loss_percent * 20
  + timeout_count * 200
```

分值越低越好。

解释：

- RTT 是基础项
- 丢包惩罚高于 RTT
- 超时惩罚远高于 RTT

## 8. 切换策略

## 8.1 切换触发条件

满足任一条件即可触发切换评估：

- active 连续超时
- active RTT 超过阈值
- active 丢包率超过阈值
- active 进入 `Suspect` 或 `Failed`

建议阈值：

- 连续超时 >= 2
- RTT EWMA > 300ms
- 丢包率 > 10%

## 8.2 切换决策

从 standby 中选择：

- `state == Ready`
- `score` 最低
- 最近一次 probe 成功

切换条件建议加入迟滞，防止抖动：

```text
standby.score + switch_margin < active.score
```

其中：

- `switch_margin = 30 ~ 50`

## 8.3 冷却时间

每次切换后进入 cooldown：

- 10~15 秒内不允许再次因轻微抖动切回

仍然允许在严重故障时强切。

## 8.4 切换动作

切换不是“先断旧再连新”，而是：

1. standby 预先就是 ready
2. 将 `active_node_id` 原子切到新节点
3. 新发出的包只走新 active
4. 旧 active 降为 standby
5. 上报一次 path switch 事件

## 9. 发送模型

## 9.1 单路径发送

隧道层只调用：

```rust
derp_pool.send_via_active(packet)
```

`derp_pool` 内部负责：

- 找当前 active
- 发送
- 如果发送失败则快速降级到 `maybe_switch()`

## 9.2 接收模型

所有 ready 连接都允许接收控制帧与必要的数据面响应。

但业务发送只认 active，避免：

- 双发
- 重复 ack
- 路径乱序

## 10. 状态机

### 单连接状态机

```text
Connecting -> Ready -> Suspect -> Failed
      ^         |         |         |
      |         +---------+---------+
      +---------------- Reconnect ---+
```

### 池状态机

```text
Empty
-> WarmingUp
-> Ready(active selected)
-> Switching
-> Ready
-> Degraded(all links suspect/failed)
```

## 11. 控制面与客户端事件

建议新增内部事件：

```rust
pub enum DerpPoolEvent {
    LinkConnected { node_id: String },
    LinkSuspect { node_id: String, reason: String },
    LinkFailed { node_id: String, reason: String },
    ActiveSwitched {
        from_node_id: String,
        to_node_id: String,
        reason: SwitchReason,
    },
}
```

上报给控制面的连接状态可扩展为：

- `path = derp`
- `activeDerpNodeId`
- `derpClusterId`
- `switchReason`

## 12. 服务端集群要求

为了让客户端能在一个 DERP 集群内平滑切换，服务端需要满足：

- 同一集群节点采用统一 ticket 校验规则
- 同一 session 可在多个节点独立 attach
- 节点故障不影响 ticket 语义
- 会话路由可由上层共享存储或无状态重建机制支持

最低可落地方案：

- 先允许“客户端切换节点后重新 attach session”
- 不要求节点间复制连接状态
- 切换期间允许极短暂抖动

## 13. 推荐落地顺序

1. 固化 `bootstrap.derp_map` 与 cluster-aware relay ticket 的协议回归。
2. 扩展客户端 `derp-client` / `derp-pool` 的真实网络和故障注入能力。
3. 完善多连接、单路径发送、probe 评分和切换的诊断输出。
4. 补齐 server-wire-relay / server-wire-derp 多节点部署、attach、转发和指标。
5. 最后完善动态切换与状态上报的生产观测。

## 14. 与当前代码结构的对应关系

建议新增或调整：

```text
client/app_core/crates/
├── controller-client/
│   └── 消费 derp map / cluster-aware relay ticket
├── relay-client/
│   └── derp-client / derp-pool / path-manager
├── app-core/
│   └── DerpNodeMeta / DerpHealth / ActivePath
└── tunnel/
    └── send path 改为通过 path-manager 分发

server/service-biz/
├── api/dto/
│   └── 扩展 bootstrap / relay ticket 结构
└── internal/service/impl/
    └── 补充 derp cluster 编排

server/server-wire-relay/
└── internal/
    └── 引入 cluster-aware relay ticket / session 语义

server/server-wire-derp/
└── internal/
    └── 引入 derp region / node / connection 语义
```

## 15. 关键结论

- 客户端应同时持有 2~3 个 DERP 热连接
- 业务流量只走一个 active 节点
- standby 节点只保活和探测，不做双发
- 切换依据是 RTT、超时、丢包综合评分
- 切换要有迟滞和 cooldown，防止抖动
- ticket 应按集群授权，而不是绑死到单节点
- `derp_pool` 应作为客户端核心一级模块，而不是附着在 UI 或临时连接逻辑上
