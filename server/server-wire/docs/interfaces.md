# server-wire 接口文档

客户端协议的完整跨服务合同见 [Wire client protocol](../../../docs/wire-client-protocol.md)。

## 对客户端接口

### `POST /v1/peers/register`

输入：

- `peerId`
- `nodeId`
- `publicKey`
- `supportsLanDirect`
- `supportsIpv6Direct`
- `supportsDirectUdp`
- `supportsRelayUdp`
- `supportsDerpTcpTls443`
- `preferLan`
- `preferIpv6`

输出：

- `peer` runtime 记录

### `POST /v1/peers/endpoints`

输入：

- `peerId`
- `endpoints[]`

每个 endpoint 至少包含：

- `kind`
- `address`
- `port`
- `reachable`

### `POST /v1/peers/path-health`

输入：

- `peerId`
- `probes[]`

每个 probe 至少包含：

- `path`
- `reachable`
- `rttMs`
- `lossPpm`
- `jitterMs`
- `mtu`

### `POST /v1/peers/derp-health`

输入：

- `peerId`
- `samples[]`

每个 sample 至少包含：

- `regionId`
- `nodeId`
- `reachable`
- `rttMs`

### `POST /v1/peers/active-path`

输入：

- `peerId`
- `path`

### `POST /v1/path-plan`

输入：

- `peerId`

输出：

- `preferredPath`
- `degradedReason`
- `fallbackOrder`
- `scoredPaths`
- `keepalive`
- `mtu`
- `roaming`
- `relayTicket`
- `relayCandidates`
- `derpCandidates`
- `derpTicket`
- `fastReselection`

### `POST /v1/relay/tickets`

输入：

- `peerId`
- `ttlSeconds`
- `renewAfterMs`

输出：

- `ticketId`
- `peerId`
- `sessionId`
- `path=relay_udp`
- `regionId`
- `nodeId`
- `host`
- `udpPort`
- `expiresAt`
- `signature`

### `GET /v1/derp/map`

输出：

- `regions[]`
- `nodes[]`
- `preferredRegionId`

### `POST /v1/derp/tickets`

输入：

- `peerId`
- `regionId`
- `nodeId`
- `ttlSeconds`

输出：

- `ticketId`
- `peerId`
- `networkId`
- `path=derp_tcp_tls_443`
- `regionId`
- `nodeId`
- `expiresAt`
- `signature`

票据签名：

- relay payload: `ticketId|peerId|sessionId|path|expiresAt`
- DERP payload: `ticketId|peerId|networkId|path|regionId|nodeId|expiresAt`
- 算法：HMAC-SHA256 hex
- 当前签发密钥：`SLAN_WIRE_TICKET_SECRET`
- 校验密钥环：`SLAN_WIRE_TICKET_SECRETS`，逗号分隔；relay / DERP 数据面接受任一密钥命中的票据签名，用于短票据窗口内滚动轮换。

### `GET /internal/wire/ticket-key-status`

输出：

- `source`: `key_ring` / `signing_secret` / `dev_default`
- `keyRingId`: 密钥环一致性 ID，用于跨实例比较；不包含原始密钥值
- `signingConfigured`
- `keyRingConfigured`
- `effectiveKeyCount`
- `rotationReady`
- `acceptsDevFallback`

该接口只输出配置状态和一致性 ID，不输出密钥值。用于检查 `server-wire` 多实例与 relay/DERP 数据面验签密钥环是否一致。

## 从 service-biz 读取的内部接口

`server-wire` 依赖以下内部读接口：

- `GET /internal/wire/peers/{peerId}/authz`
- `GET /internal/wire/networks/{networkId}/topology`
- `GET /internal/wire/peers/{peerId}/runtime-config`
- `GET /internal/wire/derp-map`
- `GET /internal/wire/admin/relay-nodes`

当配置 `SLAN_WIRE_BIZ_INTERNAL_URL` 后，`server-wire` 会在 peer register 时调用
`service-biz` 的 `authz` 接口，并以业务控制面返回的 `networkId`、`nodeId`、
`virtualIps`、`allowedIps` 覆盖客户端自报字段。调用时必须携带
`X-Slan-Internal-Token: $SLAN_INTERNAL_WIRE_TOKEN`。

调度约束：

- DERP ticket 与 `derpCandidates` 严格基于 `service-biz /internal/wire/derp-map`。
- relay ticket 与 `relayCandidates` 严格基于 `service-biz /internal/wire/admin/relay-nodes`。
- biz 模式下只使用 `enabled && healthy && !stale` 的 relay/DERP 节点；没有可调度节点时不落回本地静态节点。
- path plan 在 relay/DERP 控制面不可用或无健康节点时允许降级为 direct-only，并通过 `degradedReason` 输出原因。

`degradedReason` 取值：

- `relay_control_unavailable`
- `relay_no_healthy_nodes`
- `derp_control_unavailable`
- `derp_no_healthy_nodes`

## 输出边界

对客户端输出：

- path plan
- relay ticket
- runtime policy

不输出：

- 用户 token
- 业务 RBAC 信息
- IPAM 写接口
