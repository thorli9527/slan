# server-biz 对外接入接口

本文档描述 `server-biz` 接入哪些外部系统和协议，以及这些接入点分别落在哪一层。

## Current Public Client HTTP Endpoints

This maintained summary is the source to check when wiring client, app-core, or
web-console flows:

- `POST /auth/register`
- `POST /auth/login`
- `POST /auth/refresh`
- `GET /auth/callback-status/{callbackId}`
- `POST /auth/callback-status/{callbackId}/complete`
- `POST /mqtt/auth/check`
- `POST /mqtt/bifromq/auth`
- `POST /mqtt/bifromq/check`
- `POST /devices/register`
- `GET /devices`
- `PUT /devices/{deviceId}/networks/{networkId}/state`
- `POST /nodes/register`
- `GET /networks/home`
- `GET /networks`
- `POST /networks`
- `PUT /networks/{networkId}`
- `PUT /networks/{networkId}/join-key`
- `PUT /networks/{networkId}/dns`
- `POST /networks/join-by-owner-email`
- `POST /networks/join-by-key`
- `POST /networks/{networkId}/switch`
- `GET /networks/{networkId}`
- `POST /networks/{networkId}/join`
- `POST /networks/{networkId}/activate`
- `POST /networks/{networkId}/deactivate`
- `GET /networks/{networkId}/members`
- `PUT /networks/{networkId}/members/{memberId}/status`
- `GET /networks/{networkId}/assignments`
- `GET /networks/{networkId}/subnets`
- `POST /networks/{networkId}/subnets`
- `POST /networks/{networkId}/subnets/{subnetId}/attachments`
- `PUT /networks/{networkId}/attachments/{attachmentId}/ip`
- `PUT /networks/{networkId}/attachments/{attachmentId}/remark`
- `POST /bootstrap`
- `POST /relay/tickets`
- `POST /control/sessions`
- `GET /control/ws`
- `GET /debug/vars`
- `GET /healthz`

The network join and switch flow depends on `GET /networks` returning every
network visible to the authenticated user, including networks joined through
owner email or join key.
The recommended client path is `POST /bootstrap`, which already returns the
control session token and WebSocket config. `POST /control/sessions` remains the
explicit control-session endpoint for callers that need to refresh only the
control-plane session.

Device runtime state is intentionally separated:

- In production, BifroMQ Auth Provider should call `POST /mqtt/bifromq/auth`
  for credential validation and `POST /mqtt/bifromq/check` for topic access
  checks. `POST /mqtt/auth/check` remains as the legacy compatibility
  credential-check endpoint. MQTT authentication success marks only the control
  channel as reachable.
- `PUT /devices/{deviceId}/networks/{networkId}/state` reports whether the
  virtual network is enabled, whether the local tunnel is up, and the latest
  health probe result.
- The client keeps sending that state every 15 seconds after MQTT is connected.
  The preferred path is MQTT topic
  `{topicPrefix}/networks/{networkId}/state`; the HTTP `PUT` endpoint remains a
  fallback and writes the same state record. Before the tunnel is enabled it
  reports `networkOnline=false`; after local tunnel up it reports
  `networkOnline=true`.
- Server cleanup marks stale control/network state offline after the freshness
  window expires, so management views do not treat an old MQTT connection as
  network online.
- Legacy `Device.status` may be `reachable` for compatibility with older
  control-channel views. Management online counts and green online state should
  use `Device.networkState.networkOnline`.

See `client-core-flow.md` for the end-to-end client create, join, alias,
switch, activate, bootstrap, and relay fallback flow.

## 1. 外部接入总览

`server-biz` 需要接入三类外部交互：

- 客户端 HTTP 请求
- 客户端控制通道连接
- 配置与基础设施

## 2. HTTP 接入接口

HTTP 接入由 `api/http/routes.go` 和 `api/http/routes_business*.go` 承接。

### 2.1 无鉴权接口

- `POST /auth/register`
- `POST /auth/login`
- `POST /auth/refresh`
- `GET /auth/callback-status/{callbackId}`
- `POST /auth/callback-status/{callbackId}/complete`
- `POST /mqtt/auth/check`
- `POST /mqtt/bifromq/auth`
- `POST /mqtt/bifromq/check`
- `GET /healthz`
- `GET /debug/vars`

### 2.2 鉴权接口

- `POST /devices/register`
- `GET /devices`
- `PUT /devices/{deviceId}/networks/{networkId}/state`
- `POST /nodes/register`
- `GET /networks/home`
- `GET /networks`
- `POST /networks`
- `PUT /networks/{networkId}`
- `PUT /networks/{networkId}/join-key`
- `PUT /networks/{networkId}/dns`
- `POST /networks/join-by-owner-email`
- `POST /networks/join-by-key`
- `POST /networks/{networkId}/switch`
- `GET /networks/{networkId}`
- `POST /networks/{networkId}/join`
- `POST /networks/{networkId}/activate`
- `POST /networks/{networkId}/deactivate`
- `GET /networks/{networkId}/members`
- `PUT /networks/{networkId}/members/{memberId}/status`
- `GET /networks/{networkId}/assignments`
- `GET /networks/{networkId}/subnets`
- `POST /networks/{networkId}/subnets`
- `POST /networks/{networkId}/subnets/{subnetId}/attachments`
- `PUT /networks/{networkId}/attachments/{attachmentId}/ip`
- `PUT /networks/{networkId}/attachments/{attachmentId}/remark`
- `POST /bootstrap`
- `POST /relay/tickets`
- `POST /control/sessions`
- `GET /control/ws`

## 3. 控制通道接入接口

控制通道消息结构由：

- `protocol/protobuf/control.proto`
- `internal/ws/messages_handshake.go`
- `internal/ws/messages_topology.go`
- `internal/ws/messages_control.go`

承接消息：

- Node 握手
- 心跳
- NetworkMap 请求与推送
- Endpoint 上报
- Peer 候选下发
- ConnectPlan 下发
- ConnectionState 上报

## 4. 内部服务接入边界

HTTP 层并不直接做业务处理，而是接入内部服务：

- `Auth`
- `Device`
- `Network`
- `Node`
- `Bootstrap`
- `TokenVerifier`

调用入口聚合在：

- `internal/service/access.go`
- `internal/service/registration.go`
- `internal/service/network.go`
- `internal/service/control.go`
- `internal/service/ops.go`

## 5. 配置接入

当前配置主要由：

- `configs/config.go`
- `configs/config.example.yaml`

承接。

现阶段重要配置包括：

- HTTP 地址
- 控制通道路径
- relay 基础配置

## 6. 与外部系统的边界

### 6.1 与 `app_core`

通过：

- HTTP
- WebSocket 控制通道

交互。

### 6.2 与 `server-relay`

当前不是直接 RPC 集成，主要通过 ticket 语义间接集成。

未来如果引入 DERP 集群控制面编排，可能还需要：

- 节点拓扑同步
- 集群状态同步

### 6.3 与存储和基础设施

当前仓库里已经接入 PostgreSQL 和 Redis 存储。

这些外部依赖的访问应继续收口在：

- `internal/repo`
- `configs`

而不是直接侵入 `api/http`

## 7. 接入原则

1. 客户端 HTTP 只接到 `api/http`
2. 控制通道只接到协议与消息层
3. 业务编排只落在 `internal/*`
4. 外部依赖只通过 `configs/repo` 收口
