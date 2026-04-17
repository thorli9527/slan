# server-biz 对外接入接口

本文档描述 `server-biz` 接入哪些外部系统和协议，以及这些接入点分别落在哪一层。

## 1. 外部接入总览

`server-biz` 需要接入三类外部交互：

- 客户端 HTTP 请求
- 客户端控制通道连接
- 配置与基础设施

## 2. HTTP 接入接口

HTTP 接入由 `api/http/routes.go` 承接。

### 2.1 无鉴权接口

- `POST /auth/register`
- `POST /auth/login`
- `GET /healthz`

### 2.2 鉴权接口

- `POST /devices/register`
- `GET /devices`
- `POST /nodes/register`
- `POST /control/sessions`
- `GET /networks`
- `POST /networks`
- `GET /networks/{networkId}`
- `POST /networks/{networkId}/join`
- `GET /networks/{networkId}/members`
- `GET /networks/{networkId}/subnets`
- `POST /networks/{networkId}/subnets`
- `POST /networks/{networkId}/subnets/{subnetId}/attachments`
- `POST /bootstrap`
- `POST /relay/tickets`

## 3. 控制通道接入接口

控制通道消息结构由：

- `protocol/protobuf/control.proto`
- `internal/ws/messages.go`

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
- `Tokens`

调用入口聚合在：

- `internal/service/interfaces.go`

## 5. 配置接入

当前配置主要由：

- `internal/infra/config.go`
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

当前仓库里还没有真正落库实现。

后续接入时应控制在：

- `internal/repo`
- `internal/infra`

而不是直接侵入 `api/http`

## 7. 接入原则

1. 客户端 HTTP 只接到 `api/http`
2. 控制通道只接到协议与消息层
3. 业务编排只落在 `internal/*`
4. 外部依赖只通过 `infra/repo` 收口
