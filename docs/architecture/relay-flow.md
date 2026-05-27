# Relay / DERP 回退流程

旧 `server/server-relay` 已删除。当前回退链路由 `server-wire` 签发票据，由 `server-wire-relay` 和 `server-wire-derp` 分别承载 UDP relay 与 DERP TCP 兜底数据面。

## 目标

当客户端无法通过 LAN Direct、IPv6 Direct 或 Direct UDP 直连时，客户端向 `server-wire` 请求短时效票据，再接入对应数据面服务完成转发。

```text
LAN Direct -> IPv6 Direct -> Direct UDP -> Relay UDP -> DERP TCP fallback
```

## 角色划分

- `server/server-wire` 负责 peer runtime config、路径规划、`RelayTicket` / `DerpTicket` 签发、DERP map。
- `server/server-wire-relay` 负责校验 `relay_udp` ticket、创建 session、转发 UDP payload。
- `server/server-wire-derp` 负责校验 `derp_tcp_tls_443` ticket、维护 DERP TCP 连接、转发最终兜底 payload。
- `client/app_core` 负责路径探测、active path 切换、ticket 消费、数据面接入和失败回退。
- `server/service-biz` 负责业务身份和网络授权，不直接签发联网票据、不处理数据面。

## 当前协议面

### `server-wire`

- `POST /v1/path-plan` 返回路径候选和评分。
- `POST /v1/relay/tickets` 签发 `relay_udp` ticket。
- `POST /v1/derp/tickets` 签发 `derp_tcp_tls_443` ticket。
- `GET /v1/derp/map` 返回 DERP region / node 列表。
- `POST /v1/peers/path-health` 和 `POST /v1/peers/derp-health` 接收客户端探测结果。

### `server-wire-relay`

- UDP `attach` 消费 `RelayTicket`。
- UDP `forward` 在 session 内转发 payload。
- HTTP `GET /v1/sessions` 和 `GET /metrics` 提供观测。

### `server-wire-derp`

- TCP JSON `connect` 消费 `DerpTicket`。
- TCP JSON `send` 在 DERP session 内转发 payload。
- HTTP `GET /v1/connections`、`GET /v1/sessions`、`GET /v1/regions` 和 `GET /metrics` 提供观测。

## 端到端调用链

1. 客户端注册 peer 并拉取 `server-wire` runtime config。
2. 客户端按 LAN、IPv6、Direct UDP、Relay UDP、DERP TCP fallback 顺序探测并上报 health。
3. `server-wire` 基于探测结果、endpoint、MTU 和 keepalive 策略生成 path plan。
4. 需要 relay 时，客户端请求 `server-wire` 签发 `RelayTicket`，再接入 `server-wire-relay`。
5. 需要最终兜底时，客户端请求 `server-wire` 签发 `DerpTicket`，再接入 `server-wire-derp`。
6. 客户端 active path 变化后回报 `server-wire`，用于快速重选路和后续评分。

## 当前缺口

- `server-wire` 需要补业务授权同步、票据密钥轮换、持久化存储。
- `server-wire-relay` 需要补生产 UDP runtime 限流、集群注册和跨节点 session 策略。
- `server-wire-derp` 当前是裸 TCP JSON-lines；生产 443/TLS 需要外部四层/TLS 终止或后续内置 TLS transport，并继续补 region 多节点调度和连接限流。
- `client/app_core` 需要按新协议补齐真实数据面客户端实现。
