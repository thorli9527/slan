# server-biz 对外输出功能

本文档描述 `server-biz` 对客户端和其他外部系统输出什么能力。

## Current Public Capabilities

The main client-facing capabilities currently exported by `server-biz` are:

- Auth lifecycle: register, login, refresh, browser-to-client callback status.
- Device and node lifecycle: register/list devices, register nodes.
- Device runtime state: report control reachability, virtual network enabled
  state, tunnel state, health probe state, and heartbeat freshness.
- Network lifecycle: create, list visible networks, get active/owned home
  summary, get network detail, update network metadata, join key, and DNS.
- Join and switch: join by owner email, join by key, explicit join, switch,
  activate, deactivate.
- Membership and assignment management: list/update members, list assignments,
  update attachment IP, update attachment remark.
- Runtime bootstrap: bootstrap control-plane config, NetworkMap, relay/DERP
  topology, and relay fallback tickets.
- Explicit control session creation for clients that need to refresh only the
  control-plane session after bootstrap.
- Control WebSocket: node hello, network map updates, peer updates, connect
  plans, path health, connection state, device IP reassignment, active network
  notifications.

See `client-core-flow.md` for how these capabilities compose into the public
desktop app, app-core, and web-console client flow.

## 1. 输出目标

`server-biz` 对外输出的不是底层存储或内部状态，而是：

- 可调用的控制面 API
- 控制通道协议
- 编排后的身份与网络视图
- 启动配置和 ticket

## 2. 当前对外输出能力

### 2.1 HTTP 管理面

当前主要输出：

- `/auth/register`
- `/auth/login`
- `/auth/refresh`
- `/auth/callback-status/{callbackId}`
- `/auth/callback-status/{callbackId}/complete`
- `/mqtt/auth/check`
- `/mqtt/bifromq/auth`
- `/mqtt/bifromq/check`
- `/auth/ws/{callbackId}` (deprecated compatibility path)
- `/devices/register`
- `/devices`
- `/devices/{deviceId}/networks/{networkId}/state`
- `/nodes/register`
- `/networks/home`
- `/networks`
- `/networks/{networkId}`
- `/networks/{networkId}/join-key`
- `/networks/{networkId}/dns`
- `/networks/join-by-owner-email`
- `/networks/join-by-key`
- `/networks/{networkId}/switch`
- `/networks/{networkId}/join`
- `/networks/{networkId}/activate`
- `/networks/{networkId}/deactivate`
- `/networks/{networkId}/members`
- `/networks/{networkId}/members/{memberId}/status`
- `/networks/{networkId}/assignments`
- `/networks/{networkId}/subnets`
- `/networks/{networkId}/subnets/{subnetId}/attachments`
- `/networks/{networkId}/attachments/{attachmentId}/ip`
- `/networks/{networkId}/attachments/{attachmentId}/remark`
- `/bootstrap`
- `/relay/tickets`
- `/control/sessions`
- `/control/ws`
- `/debug/vars`
- `/healthz`

### 2.2 控制通道协议

控制通道当前对外输出这些消息语义：

- `NodeHello`
- `NodeHelloAck`
- `Ping / Pong`
- `NetworkMapRequest / Response`
- `PeerUpdate / PeerRemove`
- `EndpointReport`
- `PeerCandidate`
- `ConnectPlan`
- `RelayTicket`
- `ConnectionState`
- `DisconnectNotice`
- `ErrorMessage`

### 2.3 配置与视图

对客户端输出的关键编排结果：

- `BootstrapResponse`
- `ControlSessionResponse`
- `NetworkMap`
- `RelayTicket`
- `Device.networkState`, which separates control reachability from virtual
  network online state and tunnel health. MQTT auth success only sets
  the control reachability side; the app heartbeat promotes the network side
  after the local tunnel is up.

## 3. 输出给谁

### 3.1 输出给客户端

主要是：

- `client/app_core`
- `client/app`

客户端依赖 `server-biz` 获取身份、网络配置、控制通道配置和回退 ticket。

### 3.2 输出给数据面

`server-relay` 不直接依赖 `server-biz` 的内部实现，但依赖它签发的 ticket 语义。

也就是说，`server-biz` 对数据面的主要输出是：

- `RelayTicket`
- 后续的 `DerpTicket`

## 4. 对外输出结果的特点

控制面的输出是“编排结果”，而不是底层原始数据。

例如：

- 客户端拿到的是 `BootstrapResponse`
  不是内部存储表
- 客户端拿到的是 `NetworkMap`
  不是所有仓储对象
- relay 拿到的是 `RelayTicket`
  不是控制面数据库访问权

## 5. 当前不应直接对外输出的内容

这些内容不应直接暴露给外部调用方：

- 内部内存状态
- `internal/service` 具体实现
- 存储层结构
- 鉴权与编排内部中间状态

对外应始终通过：

- HTTP API
- WebSocket 控制通道
- 标准 DTO / protobuf 消息

## 6. DERP / 集群输出状态

当前协议已经具备这些 DERP / 集群相关输出：

- `bootstrap.derp_map`
- `ConnectPlan.derpClusterId`
- `ConnectPlan.preferredDerpNodeIds`
- cluster-aware `RelayTicket / DerpTicket`

后续重点是生产配置、真实多节点运行时、观测指标和故障恢复验证。

## 7. 对外输出原则

1. 客户端只看编排结果，不看控制面内部细节
2. 数据面只消费 ticket，不依赖控制面内部实现
3. 所有外部输出必须通过协议模型定义
