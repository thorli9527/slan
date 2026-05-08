# server-biz 接口文档

## 对客户端接口

### Auth

- `POST /auth/register`
- `POST /auth/login`
- `POST /auth/refresh`
- `PUT /auth/password`

### Device / Node

- `POST /devices/install-register`
- `POST /devices/register`
- `GET /devices`
- `POST /nodes/register`

### Network

- `POST /networks`
- `POST /networks/{networkId}/join`
- `POST /networks/{networkId}/switch`
- `GET /networks/{networkId}`
- `PUT /networks/{networkId}/attachments/{attachmentId}/ip`
- `PUT /networks/{networkId}/attachments/{attachmentId}/remark`

### Ops

- `/ops/*`

## 对 server-wire 的内部接口

这些接口只服务新联网控制面，不对客户端公开。

鉴权：

- Header: `X-Slan-Internal-Token`
- Token 来源：`internal.wire_token` / `SLAN_INTERNAL_WIRE_TOKEN`

### `GET /internal/wire/peers/{peerId}/authz`

返回：

- `peerId`
- `deviceId`
- `nodeId`
- `networkId`
- `enabled`
- `quota`
- `virtualIps`
- `allowedIps`

### `GET /internal/wire/networks/{networkId}/topology`

返回：

- `networkId`
- `peers[]`
- `virtualIps`
- `allowedIps`
- `dns`
- `networkEnabled`

### `GET /internal/wire/peers/{peerId}/runtime-config`

返回：

- `peerId`
- `virtualIps`
- `allowedIps`
- `dns`
- `defaultKeepaliveSecs`
- `networkEnabled`

### `GET /internal/wire/derp-map`

返回健康 DERP 节点调度视图：

- `preferredRegionId`
- `regions[]`
- `regions[].nodes[]`

过滤规则：

- 只返回 `enabled=true && healthy=true && stale=false` 的节点
- 节点超过心跳新鲜度窗口后会被清理为 `healthy=false`，调度查询也会直接排除过期 `updatedAtMs`

## 对 server-wire 数据面的内部管理接口

这些接口归 `server-biz` 管理，供数据面节点注册、心跳和运维调度使用，不对客户端公开。

鉴权：

- Header: `X-Slan-Internal-Token`
- Token 来源：`internal.wire_token` / `SLAN_INTERNAL_WIRE_TOKEN`

### `GET /internal/wire/admin/derp-nodes`

返回 DERP 节点列表。

节点记录包含：

- `enabled`
- `healthy`
- `stale`
- `updatedAtMs`
- `ticketKeyRotation`

### `PUT /internal/wire/admin/derp-nodes`

输入：

- `regionId`
- `nodeId`
- `name`
- `host`
- `port`
- `enabled`
- `healthy`
- `priority`
- `ticketKeyRotation`

### `POST /internal/wire/admin/derp-nodes/{regionId}/{nodeId}/heartbeat`

输入：

- `healthy`
- `ticketKeyRotation`

### `PATCH /internal/wire/admin/derp-nodes/{regionId}/{nodeId}/status`

输入：

- `enabled`
- `healthy`

用于运维禁用/启用节点，或强制标记健康状态；不要求重新提交 `host/port`。

### `GET /internal/wire/admin/relay-nodes`

返回 relay 节点列表。

节点记录包含：

- `enabled`
- `healthy`
- `stale`
- `updatedAtMs`
- `ticketKeyRotation`

## Ops Wire 节点视图

### `GET /wire-nodes`

返回：

- `derpNodes`
- `relayNodes`
- `recentEvents`
- `derpMap`
- `schedulableDerpCount`
- `schedulableRelayCount`
- `staleCount`
- `ticketKeyRotation`
- `ticketKeyHealth`

`ticketKeyHealth` 汇总当前可调度的 `wire / relay / derp` 实例：

- `baselineKeyRingId`
- `rotationReady`
- `drifted`
- `unavailableCount`
- `instances`

`instances` 字段包含：

- `kind`: `wire` / `relay` / `derp`
- `regionId`
- `nodeId`
- `url`
- `status`
- `drifted`

`status` 字段包含：

- `source`
- `keyRingId`
- `signingConfigured`
- `keyRingConfigured`
- `effectiveKeyCount`
- `rotationReady`
- `acceptsDevFallback`
- `available`
- `error`
- `observedAtMs`

### `GET /wire-node-events`

返回分页节点事件，用于 `server-main` Angular 运维页面筛选：

- `nodeKind`
- `regionId`
- `nodeId`
- `eventType`
- `createdFromMs`
- `createdToMs`
- `page`
- `pageSize`

说明：

- `server-biz` 不再生成 Wire HTML 页面。
- Wire 节点运维页面统一由 `server-main` Angular 调用 `/ops-api/wire-nodes` 和 `/ops-api/wire-node-events` 渲染。

## Wire 节点调度配置

YAML:

- `wire.control_plane_urls`，server-wire 控制面实例 URL 列表，用于 ops 主动探测 `/internal/wire/ticket-key-status`
- `wire.node_heartbeat_freshness_seconds`，默认 `120`
- `wire.node_cleanup_interval_seconds`，默认 `60`
- `wire.node_event_retention_seconds`，默认 `604800`

环境变量：

- `SLAN_WIRE_CONTROL_PLANE_URLS`，逗号分隔
- `SLAN_WIRE_NODE_HEARTBEAT_FRESHNESS_SECONDS`
- `SLAN_WIRE_NODE_CLEANUP_INTERVAL_SECONDS`
- `SLAN_WIRE_NODE_EVENT_RETENTION_SECONDS`

超过 `node_heartbeat_freshness_seconds` 的节点会被视为 `stale=true`，调度查询直接排除；后台 janitor 按 `node_cleanup_interval_seconds` 周期把 stale 节点标记为 `healthy=false`。
`node_event_retention_seconds <= 0` 时不清理事件。

## Wire 节点事件

`wire_node_events` 记录 DERP/relay 节点状态时间线，`/wire-nodes` 的 `recentEvents` 返回最近事件。

### `GET /wire-node-events`

查询参数：

- `nodeKind`: `derp` / `relay`
- `regionId`
- `nodeId`
- `eventType`
- `createdFromMs`
- `createdToMs`
- `page`
- `pageSize`，最大 `200`

返回：

- `items`
- `page`
- `pageSize`
- `total`

事件类型：

- `registered`
- `heartbeat`
- `status_changed`
- `stale_marked`

事件字段：

- `nodeKind`: `derp` / `relay`
- `regionId`
- `nodeId`
- `fromEnabled` / `toEnabled`
- `fromHealthy` / `toHealthy`
- `reason`
- `createdAtMs`

### `PUT /internal/wire/admin/relay-nodes`

输入：

- `regionId`
- `nodeId`
- `host`
- `udpPort`
- `adminPort`
- `enabled`
- `healthy`
- `priority`
- `ticketKeyRotation`

### `POST /internal/wire/admin/relay-nodes/{regionId}/{nodeId}/heartbeat`

输入：

- `healthy`
- `ticketKeyRotation`

### `PATCH /internal/wire/admin/relay-nodes/{regionId}/{nodeId}/status`

输入：

- `enabled`
- `healthy`

用于运维禁用/启用节点，或强制标记健康状态；不要求重新提交 `host/udpPort/adminPort`。

## 输入输出边界

输入：

- 用户身份凭证
- 设备/节点注册请求
- 网络资源变更请求

输出：

- 业务 token
- 设备/节点/网络资源视图
- 给 `server-wire` 的授权与静态拓扑视图
