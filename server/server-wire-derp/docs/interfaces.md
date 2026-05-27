# server-wire-derp 接口文档

客户端协议的完整跨服务合同见 [Wire client protocol](../../../docs/wire-client-protocol.md)。

## 协议形态

`server-wire-derp` 当前实现使用裸 TCP 长连接和 JSON-lines 帧协议。
`derp_tcp_tls_443` 是控制面路径名称；如生产环境需要 443/TLS，应由外部
L4/TLS 终止层转发裸 TCP 到本服务，或后续在所有客户端和服务端统一引入
端到端 TLS transport。

它承担的是最终兜底路径：

- `derp_tcp_tls_443`

## 客户端数据面接口

### 连接建立

客户端连接：

- `derp://<derp-host>:<derp-port>`
- `derp+tcp+tls://<derp-host>:443` 仅在部署层已提供 TLS 终止并转发裸 TCP 时使用

建立连接后，客户端必须先发送 `connect` 帧。

### `connect`

输入：

- `kind=connect`
- `peerId`
- `nodeId`
- `regionId`
- `ticket`

其中 ticket 至少包含：

- `ticketId`
- `peerId`
- `networkId`
- `path=derp_tcp_tls_443`
- `regionId`
- `nodeId`
- `expiresAt`
- `signature`

`signature` 使用 `SLAN_WIRE_TICKET_SECRET` 校验，payload 为
`ticketId|peerId|networkId|path|regionId|nodeId|expiresAt`，算法为 HMAC-SHA256 hex。

输出：

- `kind=connected`
- `sessionId`
- `regionId`
- `nodeId`
- `renewAfterMs`

### `send`

输入：

- `kind=send`
- `sessionId`
- `targetPeerId`
- `payload`

输出：

- `kind=sent`
- `sessionId`
- `bytesForwarded`

### `recv`

服务端向目标 peer 推送：

- `kind=recv`
- `sessionId`
- `sourcePeerId`
- `payload`

### `disconnect`

输入：

- `kind=disconnect`
- `sessionId`

输出：

- `kind=disconnected`
- `sessionId`

### `error`

输出：

- `kind=error`
- `error.code`
- `error.message`

## 管理/观测 HTTP 接口

### `GET /healthz`

返回：

- `ok`
- `service=server-wire-derp`

### `GET /v1/connections`

返回：

- 当前长连接列表
- `peerId`
- `regionId`
- `nodeId`
- `connectedAt`
- `lastSeenAt`

### `GET /v1/sessions`

返回：

- 当前 DERP session 列表
- `sessionId`
- `peerIds`
- `regionId`
- `nodeId`
- `expiresAt`

### `GET /v1/sessions/{sessionId}`

返回单个 DERP session 详情。

### `GET /v1/regions`

返回：

- region 列表
- 每个 region 的 node 列表
- 健康状态
- 当前连接数

### `GET /metrics`

返回：

- `connectionCount`
- `sessionCount`
- `regionHealthyCount`
- `nodeHealthyCount`

## 与 server-wire 的接口边界

`server-wire-derp` 自身不生成 DERP ticket，不直接调用 `server-biz`。

它信任 `server-wire` 输出的：

- `derpMap`
- `derpTicket`
- `regionId`
- `nodeId`

## 输出边界

对客户端输出：

- DERP 长连接接入
- DERP session 建立结果
- DERP 数据转发

对运维输出：

- 连接列表
- session 列表
- region / node 健康
- metrics
