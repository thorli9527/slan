# Relay 功能梳理

## Maintained Status

The current main relay fallback path is wired end to end:

- `POST /bootstrap` returns control-plane config, `NetworkMap`, relay topology,
  and `derp_map`.
- `POST /relay/tickets` issues cluster-aware relay/DERP ticket fields.
- `client/app_core/crates/controller-client` calls real bootstrap and ticket
  HTTP APIs.
- `client/app_core` has tested relay/DERP fallback routing through
  `DerpPool`, `PathManager`, and relay client abstractions.
- Flutter drives bootstrap and relay fallback through the app-core bridge in
  the devices flow.

The remaining work in this area is production hardening: real multi-node relay
deployment, failure injection, observability, and cross-platform tunnel
recovery. It is no longer a missing main-business API flow.

## 目标

当客户端无法通过 P2P 直连时，由控制面签发短时效 `RelayTicket`，客户端再使用该票据接入 `server-relay`，通过中继完成数据转发。

## 角色划分

- `server/server-biz`
  - 负责 `bootstrap` 和 `issueRelayTicket`
  - 负责把中继配置返回给客户端
  - 负责在控制通道里下发带 `relay_ticket` 的 `ConnectDirective`
- `server/server-relay`
  - 负责校验 `RelayTicket`
  - 负责创建/维护 relay session
  - 负责把一个设备发来的 UDP 负载转发给 session 内的另一端
- `client/app` / `client/app_core`
  - 负责调用 biz 的 `bootstrap` 和 `issueRelayTicket`
  - 负责在连接失败时从 P2P 回退到 relay
- `protocol`
  - `openapi/phase1.yaml` 定义 HTTP 接口
  - `protobuf/control.proto` 定义控制通道消息
  - `errors/codes.yaml` 定义错误码

## 当前协议面

### HTTP 接口

- `POST /bootstrap`
  - 返回设备启动配置、网络拓扑、控制通道配置、STUN 列表和 relay 配置
- `POST /relay/tickets`
  - 输入：`deviceId`、`networkId`、`peerDeviceId`、`reason`
  - 输出：`ticketId`、`relayUrl`、`expiresAt`、`sessionKey`、`signature`

定义位置：

- `protocol/openapi/phase1.yaml`
- `server/server-biz/api/dto/types.go`
- `server/server-biz/api/http/routes.go`

### 控制通道消息

- `PeerCandidate`
  - 交换 NAT 穿透候选
- `ConnectDirective`
  - 指示优先使用 P2P 还是 relay
  - relay 回退时携带 `relay_ticket`
- `ConnectionState`
  - 回报 `connecting / connected / failed`

定义位置：

- `protocol/protobuf/control.proto`
- `server/server-biz/internal/ws/messages_handshake.go`
- `server/server-biz/internal/ws/messages_topology.go`
- `server/server-biz/internal/ws/messages_control.go`

## 当前服务端实现

### biz

`server/server-biz` 已实现：

- `bootstrap`
  - 返回 `RelayConfig`
- `issueRelayTicket`
  - 校验设备、网络、对端成员关系
  - 生成带 `relayUrl / expiresAt / signature / sessionKey` 的票据

实现位置：

- `server/server-biz/internal/service/memory.go`

### relay

`server/server-relay` 已实现：

- `relay-core`
  - `RelayTicket`
  - `RelaySession`
  - `RelayError`
  - 票据过期判断与时间解析
- `auth`
  - `StaticTicketValidator`
  - 校验空字段、URL 前缀、过期时间
- `session`
  - `InMemorySessionStore`
  - `create / get / remove`
- `udp-relay`
  - `attach`
  - `attach_with_ticket`
  - `session`
  - `detach`
  - `forward`

实现位置：

- `server/server-relay/crates/relay-core/src/lib.rs`
- `server/server-relay/crates/auth/src/lib.rs`
- `server/server-relay/crates/session/src/lib.rs`
- `server/server-relay/crates/udp-relay/src/lib.rs`

## 当前 app / app_core 对接情况

### 已完成

Flutter 侧已补齐 relay 相关接口契约：

- `AppCoreApi.issueRelayTicket(...)`
- `RelayTicketModel`
- `ControlApi.issueRelayTicket(...)`
- `api_models.dart` 中的 `RelayTicketRequest`

Mock 已支持：

- 返回 mock relay ticket
- `connect()` 根据 peer 标识模拟 `p2p` 或 `relay` 连接路径

实现位置：

- `client/app/lib/infra/app_core/app_core_api.dart`
- `client/app/lib/infra/app_core/models.dart`
- `client/app/lib/infra/app_core/mock_app_core_api.dart`
- `client/app/lib/infra/control_api.dart`
- `client/app/lib/infra/api_models.dart`

### 已有但未落地的 Rust 接口

`client/app_core` 已有：

- `ControllerClient::bootstrap`
- `ControllerClient::issue_relay_ticket`
- `RelayClient::connect`

定义位置：

- `client/app_core/crates/controller-client/src/lib.rs`
- `client/app_core/crates/relay-client/src/lib.rs`

## 端到端调用链

### 启动阶段

1. app 调用 biz 的 `/bootstrap`
2. biz 返回：
   - 控制通道 `wsUrl`
   - `stunServers`
   - `relay.region`
   - `relay.udpEndpoint`
3. 客户端建立控制通道并尝试 P2P

### 回退阶段

1. P2P 失败，客户端上报 `ConnectionState.failed`
2. biz 判断需要 relay，或客户端主动调用 `/relay/tickets`
3. biz 返回 `RelayTicket`
4. 客户端把 `RelayTicket` 传给 `RelayClient`
5. `server-relay` 校验票据，创建 session
6. 双方通过 relay session 转发 UDP 数据

## 当前缺口

当前 `/bootstrap`、`/relay/tickets`、controller-client、relay-client/path
manager 的主流程已经具备测试覆盖。剩余缺口主要是生产化和可观测性：

- Flutter 普通页面仍应只展示稳定连接状态，relay fallback 细节放到诊断视图。
- 真实网络环境下的 relay / DERP 失败注入、恢复和切换测试还需要继续补强。
- 多 relay / DERP 节点集群部署和运维指标还需要完善。
- `ConnectionState` 上报与控制面观测面需要继续细化。

## 建议下一步

1. 固化 direct -> relay / DERP fallback 的回归和 smoke 测试。
2. 补充真实 relay / DERP 节点的故障注入和恢复验证。
3. 把 fallback、active path、最近探测结果整理为诊断视图。
4. 完善多节点 relay / DERP 集群部署和指标。
