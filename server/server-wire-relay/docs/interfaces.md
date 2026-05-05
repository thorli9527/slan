# server-wire-relay 接口文档

客户端协议的完整跨服务合同见 [Wire client protocol](../../../docs/wire-client-protocol.md)。

## 协议形态

`server-wire-relay` 使用 UDP JSON 控制消息。

## 管理/观测 HTTP 接口

- `GET /healthz`
- `GET /v1/sessions`
- `GET /v1/sessions/{sessionId}`
- `GET /metrics`

## 客户端输入消息

### `ping`

输入：

- `kind=ping`

### `attach`

输入：

- `kind=attach`
- `participantId`
- `transport=udp|relay_udp`
- `ticket`

其中 ticket 至少包含：

- `ticketId`
- `peerId`
- `sessionId`
- `path=relay_udp`
- `expiresAt`
- `signature`

`signature` 使用 `SLAN_WIRE_TICKET_SECRET` 校验，payload 为
`ticketId|peerId|sessionId|path|expiresAt`，算法为 HMAC-SHA256 hex。

### `forward`

输入：

- `kind=forward`
- `sessionId`
- `participantId`
- `payload`

### `detach`

输入：

- `kind=detach`
- `sessionId`
- `participantId`

## 服务端输出消息

### `pong`

- `kind=pong`

### `attached`

- `kind=attached`
- `sessionId`
- `participantId`
- `peerParticipantId`

### `forwarded`

- `kind=forwarded`
- `sessionId`
- `participantId`
- `bytesForwarded`

### `packet`

- `kind=packet`
- `sessionId`
- `participantId`
- `payload`

### `detached`

- `kind=detached`
- `sessionId`
- `participantId`

### `error`

- `kind=error`
- `error.code`
- `error.message`
