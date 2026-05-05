# local 到 production 配置迁移说明

本文档只描述当前保留的服务。旧 `server/server-relay` 已删除，不再提供 Dockerfile、配置样例或生产迁移路径。

## 本地配置入口

- [`docker-compose.local.yml`](../../docker-compose.local.yml)
- [`.env.local.example`](../../.env.local.example)
- [`server/server-biz/configs/config.docker.yaml`](../../server/server-biz/configs/config.docker.yaml)
- [`deploy/local/Caddyfile`](../../deploy/local/Caddyfile)
- [`deploy/local/bifromq/standalone.yml`](../../deploy/local/bifromq/standalone.yml)

`server-wire`、`server-wire-relay`、`server-wire-derp` 已接入本地 compose。生产化 k8s / 多节点模板仍需单独补齐。

## 必须替换的值

### `server-biz`

- `POSTGRES_PASSWORD`：本地默认值不能直接上线。
- `SLAN_OPS_ACCESS_TOKEN`：仅作为应急运维旁路使用，生产必须换成高强度随机值。
- `SLAN_OPS_DEFAULT_ADMIN_PASSWORD`：仅用于首启 seed，生产必须更换，并在首登后立即改密。
- `SLAN_HTTP_PUBLIC_HOST`：本地是 `slan.localhost:18443`，生产必须改成真实域名。
- `SLAN_HTTP_PUBLIC_SCHEME`：生产固定为 `https`。

### `server-wire`

- `SLAN_WIRE_BIZ_INTERNAL_URL`：指向 `server-biz` 内部 HTTP 地址，用于读取 peer 授权和静态拓扑。
- `SLAN_INTERNAL_WIRE_TOKEN`：`server-wire` 调用 `server-biz /internal/wire/*` 的共享内部令牌，生产必须使用独立高强度随机值。
- `SLAN_WIRE_TICKET_SECRET`：用于签发 relay / DERP ticket，必须使用独立高强度随机值。
- `SLAN_WIRE_TICKET_SECRETS`：逗号分隔的校验密钥环；轮换时新密钥放第一位，旧密钥保留到所有短票据过期。
- `SLAN_WIRE_POSTGRES_DSN`：`server-wire` 运行态持久化 Postgres DSN，生产必须启用，避免多实例和重启丢失 peer/path 状态。
- HTTP 监听地址和公网地址需要在容器化时明确区分内部访问与客户端访问。

### `server-wire-relay`

- `SLAN_WIRE_TICKET_SECRET`：必须与 `server-wire` 保持一致，用于校验 `relay_udp` ticket。
- `SLAN_WIRE_TICKET_SECRETS`：必须与 `server-wire` 的校验密钥环保持一致。
- UDP 监听端口、对外地址、安全组和限流策略必须按 region / node 单独配置。
- 管理和观测接口只允许内网或受信任运维网络访问。

### `server-wire-derp`

- `SLAN_WIRE_TICKET_SECRET`：必须与 `server-wire` 保持一致，用于校验 `derp_tcp_tls_443` ticket。
- `SLAN_WIRE_TICKET_SECRETS`：必须与 `server-wire` 的校验密钥环保持一致。
- 生产必须使用真实 TCP/TLS 443 入口，证书、SNI、反向代理和健康检查需要单独配置。
- region / node 标识要与 `server-wire` 下发的 DERP map 保持一致。

## 生产化缺口

- 为 `server-wire`、`server-wire-relay`、`server-wire-derp` 补 k8s / 多节点部署模板。
- 为票据签名密钥补轮换机制，避免单一长期 secret。
- 为 relay / DERP 数据面补限流、连接配额、指标导出和告警。
- 为 `server-biz -> server-wire` 授权同步补服务间认证和审计。
