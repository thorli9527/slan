# remote-dev 到 production 配置迁移说明

本文档只描述当前保留的服务。旧 `server/server-relay` 已删除，不再提供 Dockerfile、配置样例或生产迁移路径。

## 远程开发 Docker 入口

- [`docs/remote-docker-only.md`](../remote-docker-only.md)
- [`.tmp/remote-deploy/deploy_to_47.245.40.231.sh`](../../.tmp/remote-deploy/deploy_to_47.245.40.231.sh)
- [`docker-compose.local.yml`](../../docker-compose.local.yml) 仅作为远程开发 compose 文件使用
- [`server/server-biz/configs/config.docker.yaml`](../../server/server-biz/configs/config.docker.yaml)
- [`deploy/local/Caddyfile`](../../deploy/local/Caddyfile)
- [`deploy/local/bifromq/standalone.yml`](../../deploy/local/bifromq/standalone.yml)

`server-wire`、`server-wire-relay`、`server-wire-derp` 已接入远程开发 compose。
本机 Docker Compose 栈不再作为默认开发入口。生产化 k8s / 多节点模板仍需单独补齐。

## 生产启动强校验

当 `SLAN_ENV=prod` 或 `SLAN_ENV=production` 时，服务会拒绝使用本地默认配置启动。

`server-biz` 会强制检查：

- `http.public_scheme=https`
- `http.public_host` 不能是 `localhost / 127.0.0.1 / ::1`
- relay ticket、MQTT、ops token、Postgres password、`internal.wire_token` 不能是空值、本地默认值或 `change-me-*`
- 默认管理员如果启用，密码不能保持 `admin` 或示例值
- `wire.control_plane_urls` 至少包含一个非 loopback 的 `server-wire` 实例

`server-wire`、`server-wire-relay`、`server-wire-derp` 会强制检查：

- `SLAN_WIRE_TICKET_SECRET` 和 `SLAN_WIRE_TICKET_SECRETS` 必须显式配置，不能使用 `dev-wire-ticket-secret` 或 `change-me-*`
- `SLAN_WIRE_TICKET_SECRET` 必须等于 `SLAN_WIRE_TICKET_SECRETS` 的第一个 key，保证签发 key 和校验 keyring 不漂移
- `SLAN_INTERNAL_WIRE_TOKEN` 必须显式配置为生产令牌
- `server-wire` 必须配置 `SLAN_WIRE_BIZ_INTERNAL_URL` 和 `SLAN_WIRE_POSTGRES_DSN`
- relay / DERP 必须配置 `SLAN_BIZ_URL`，且对外 host 不能是 loopback

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
- 将票据密钥轮换从环境变量密钥环推进到集中密钥管理；当前生命周期是“新密钥放第一位签发，旧密钥保留到所有短票据过期，过期后从 `SLAN_WIRE_TICKET_SECRETS` 删除”。
- 为 relay / DERP 数据面补限流、连接配额、指标导出和告警。
- 为 `server-biz -> server-wire` 授权同步补服务间认证和审计。
