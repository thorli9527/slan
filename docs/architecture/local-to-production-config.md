# remote-dev 到 production 配置迁移说明

本文档只描述当前保留的服务。旧 `server/server-relay` 已删除，不再提供 Dockerfile、配置样例或生产迁移路径。

## 远程开发 Docker 入口

- [`docs/remote-docker-only.md`](../remote-docker-only.md)
- [`.tmp/remote-deploy/deploy_to_47.245.40.231.sh`](../../.tmp/remote-deploy/deploy_to_47.245.40.231.sh)
- [`docker-compose.local.yml`](../../docker-compose.local.yml) 仅作为远程开发 compose 文件使用
- [`server/service-biz/configs/config.docker.yaml`](../../server/service-biz/configs/config.docker.yaml)
- [`deploy/local/Caddyfile`](../../deploy/local/Caddyfile)
- [`deploy/local/bifromq/standalone.yml`](../../deploy/local/bifromq/standalone.yml)

`server-wire`、`server-wire-relay`、`server-wire-derp` 已接入远程开发 compose。
本机 Docker Compose 栈不再作为默认开发入口。生产化 k8s / 多节点模板仍需单独补齐。

## 生产启动强校验

当 `SLAN_ENV=prod` 或 `SLAN_ENV=production` 时，服务会拒绝使用本地默认配置启动。

`service-biz` 会强制检查：

- `SLAN_DEVICE_CREDENTIAL_PEPPER`、`SLAN_MQTT_PASSWORD_SECRET`、`SLAN_MQTT_WEBHOOK_TOKEN` 和 `SLAN_INTERNAL_WIRE_TOKEN` 必须显式配置，长度不少于 32 个字符。
- 上述密钥不能使用开发默认值或包含 `change-me`。校验错误只输出环境变量名，不回显密钥值。
- PostgreSQL 密码必须显式配置且长度不少于 16 个字符；使用 `SLAN_SERVICE_BIZ_DSN` 时会解析 DSN 内的密码进行同样校验。
- `SLAN_MQTT_PUBLIC_BROKER_URL` 必须使用 `mqtts://` 或 `wss://`，且不能指向 loopback 主机。
- 空库首启必须显式设置 `SLAN_OPS_DEFAULT_ADMIN_PASSWORD`；长度为 12 至 72 字节且不能包含常见弱口令片段

`server-wire`、`server-wire-relay`、`server-wire-derp` 会强制检查：

- `SLAN_WIRE_TICKET_SECRET` 和 `SLAN_WIRE_TICKET_SECRETS` 必须显式配置，不能使用 `dev-wire-ticket-secret` 或 `change-me-*`
- `SLAN_WIRE_TICKET_SECRET` 必须等于 `SLAN_WIRE_TICKET_SECRETS` 的第一个 key，保证签发 key 和校验 keyring 不漂移
- `SLAN_INTERNAL_WIRE_TOKEN` 必须显式配置为生产令牌
- `server-wire` 必须配置 `SLAN_WIRE_BIZ_INTERNAL_URL` 和 `SLAN_WIRE_POSTGRES_DSN`
- relay / DERP 必须配置 `SLAN_BIZ_URL`，且对外 host 不能是 loopback

## 必须替换的值

### `service-biz`

- `SLAN_SERVICE_BIZ_DB_PASSWORD` / `SLAN_SERVICE_BIZ_DSN`：必须包含非默认的强 PostgreSQL 密码。
- `SLAN_OPS_DEFAULT_ADMIN_EMAIL`：空库首启管理员邮箱，默认 `admin@slan.local`。
- `SLAN_OPS_DEFAULT_ADMIN_PASSWORD`：仅用于空库首启 seed，必须显式配置 12 至 72 字节强密码；已有 Operator 数据时不会读取或改写。
- `SLAN_DEVICE_CREDENTIAL_PEPPER`：用于授权 Key 摘要，必须使用独立高强度随机值。
- `SLAN_DEVICE_CREDENTIAL_PREVIOUS_PEPPERS`：逗号分隔的历史 pepper，最多 3 个，每个不少于 32 字符，仅在授权 Key 轮换窗口内保留。
- `SLAN_MQTT_PASSWORD_SECRET`：用于签发和校验 MQTT 凭据，必须使用独立高强度随机值。
- `SLAN_MQTT_WEBHOOK_TOKEN`：用于 BifroMQ Auth Provider 调用 `service-biz /mqtt/*` 的服务间鉴权，不得与 MQTT 凭据签名密钥复用。
- `SLAN_MQTT_PUBLIC_BROKER_URL`：生产客户端入口，必须是非 loopback 的 TLS URL。
- `SLAN_INTERNAL_WIRE_TOKEN`：`server-wire` 调用 `service-biz /internal/wire/*` 的共享内部令牌，生产必须使用独立高强度随机值。
- `SLAN_HTTP_PUBLIC_HOST`：本地是 `slan.localhost:18443`，生产必须改成真实域名。
- `SLAN_HTTP_PUBLIC_SCHEME`：生产固定为 `https`。

### 授权 Key pepper 轮换

1. 将当前 pepper 加入 `SLAN_DEVICE_CREDENTIAL_PREVIOUS_PEPPERS`，同时将新随机值设为 `SLAN_DEVICE_CREDENTIAL_PEPPER`。
2. 部署所有 `service-biz` 实例，确认新旧授权 Key 均能正常交换，新建 Key 只使用新 pepper。
3. 等待旧 Key 全部过期或在 Ops 吊销后，从历史列表移除旧 pepper 并再次部署。

轮换不修改已存储的摘要，不执行数据迁移。移除历史 pepper 后，所有仍使用该 pepper 摘要的旧 Key 立即失效。

### `server-wire`

- `SLAN_WIRE_BIZ_INTERNAL_URL`：指向 `service-biz` 内部 HTTP 地址，用于读取 peer 授权和静态拓扑。
- `SLAN_INTERNAL_WIRE_TOKEN`：`server-wire` 调用 `service-biz /internal/wire/*` 的共享内部令牌，生产必须使用独立高强度随机值。
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
- 当前 DERP listener 是裸 TCP JSON-lines；生产如需 443/TLS，证书、SNI、四层/TLS 终止和健康检查需要在外部入口单独配置，或后续统一实现内置 TLS transport。
- region / node 标识要与 `server-wire` 下发的 DERP map 保持一致。

## 生产化缺口

- 为 `server-wire`、`server-wire-relay`、`server-wire-derp` 补 k8s / 多节点部署模板。
- 将票据密钥轮换从环境变量密钥环推进到集中密钥管理；当前生命周期是“新密钥放第一位签发，旧密钥保留到所有短票据过期，过期后从 `SLAN_WIRE_TICKET_SECRETS` 删除”。
- 为 relay / DERP 数据面补限流、连接配额、指标导出和告警。
- 为 `service-biz -> server-wire` 授权同步补服务间认证和审计。
