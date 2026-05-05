# server-wire 需求文档

## 系统定位

`server-wire` 是 WireGuard 风格的联网控制面。

它只负责联网运行时决策，不负责身份、网络资源归属和 IP 分配。

## 主路径模型

支持以下 5 条主路径：

- `lan_udp`
- `ipv6_udp`
- `direct_udp`
- `relay_udp`
- `derp_tcp_tls_443`

其中 `derp_tcp_tls_443` 是最终兜底路径，永远排在 `relay_udp` 之后。

## 必备行为能力

- 路径探测与评分
- 快速重选路
- MTU 探测
- IPv6 优先但可回退
- LAN 优先直连
- endpoint roaming
- 中继票据续期
- per-peer keepalive 策略

## 核心职责

- peer runtime 注册
- endpoint 接收与归一化
- path health 样本接收
- path score 计算
- active path 决策
- path plan 输出
- relay ticket 签发
- DERP map / region / node 候选输出
- DERP ticket 签发
- ticket 使用 `SLAN_WIRE_TICKET_SECRET` 做 HMAC-SHA256 签名
- 向客户端输出 peer runtime config
- 从 `server-biz` 拉取授权与静态拓扑
- 从 `server-biz` 拉取 DERP/relay 节点调度视图

## 不负责的内容

- 用户登录
- 设备/节点/网络创建
- 虚拟 IP 分配
- 业务 RBAC
- UDP relay 数据转发

## 数据真相

`server-wire` 是以下对象的真相源：

- `PeerRuntime`
- `EndpointSet`
- `PathProbeSamples`
- `PathScores`
- `ActivePath`
- `KeepalivePolicy`
- `MtuPlan`
- `RoamingState`
- `RelayTicket`

本地和生产推荐使用 Postgres 持久化上述运行态：

- `wire_peers`
- `wire_path_probes`
- `wire_derp_health`
- `wire_active_path`

当 `SLAN_WIRE_POSTGRES_DSN` 未配置或启动连接失败时，允许回退到内存存储，仅用于开发与降级，不作为真实控制面边界。

## 与其他系统的边界

从 `server-biz` 读取：

- peer 授权状态
- peer 对应的 `deviceId/nodeId/networkId`
- `virtualIps`
- `allowedIps`
- `dns`
- 配额与默认策略
- `enabled && healthy` 的 DERP 节点
- `enabled && healthy` 的 relay 节点

调度边界：

- biz 模式下，DERP ticket 必须命中 `server-biz /internal/wire/derp-map` 返回的节点。
- biz 模式下，relay ticket 与 path plan 必须命中 `server-biz /internal/wire/admin/relay-nodes` 返回的可调度节点。
- biz 模式下，没有健康可用节点时不允许回退到 `server-wire` 本地静态节点。

向 `server-wire-relay` 间接输出：

- `relay_udp` ticket

向 `server-wire-derp` 间接输出：

- `derp_tcp_tls_443` ticket
- DERP map / region / node 信息

## 非功能要求

- 所有 runtime 更新都应支持快速覆盖
- path plan 计算必须可重放、可解释
- 票据续期应早于过期窗口
- 同一 peer 的 active path 变更必须可追踪
