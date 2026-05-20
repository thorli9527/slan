# service-biz

`service-biz` 是 SLAN 当前业务控制面，承接客户 Web、运营管理、设备注册、网络配置和 wire 内部授权：

- 全局设备 IP 池：所有设备从 `10.0.0.0/8` 统一分配唯一 SLAN IP。
- 用户短码域名：注册用户自动获得 `{userSlug}.slan.com`。
- 网络模型：网络是一组设备和访问策略，默认网络为 `default`。
- 设备授权：生成 32 位一次性接入码，设备 owner 确认后授权邀请方可见。
- 内网域名：每个网络维护自己的 DNS Zone 和解析记录。
- 公网访问：按 `{alias}.{networkCode}.{userSlug}.pub.staticlss.com` 映射到设备端口。
- 安全组：默认 deny，按网络规则生成最终 peers/ACL 配置。

## Run

```bash
go run ./cmd/service-biz
```

默认监听 `:38080`，可通过 `SLAN_BIZ_ADDR` 修改。

配置 Postgres 后，服务启动会自动执行 `migrations/*.sql`：

```bash
SLAN_BIZ_POSTGRES_DSN=postgres://postgres:change-me-postgres-password@postgres:5432/slan?sslmode=disable
SLAN_BIZ_MIGRATIONS_DIR=/app/migrations
```

当前阶段 Postgres 已接入 schema 初始化；业务读写仍在逐步从内存 Store 迁移到 Postgres Store。

生产环境需要配置业务状态目录，至少用于 MQTT control delivery/ACK 状态恢复：

```bash
SLAN_BIZ_STATE_DIR=/var/lib/slan/service-biz
```

该目录必须由 `service-biz` 进程可读写。完整生产持久化仍应接入 Postgres Store，不能只依赖内存 Store。

`server-wire`、`server-wire-relay`、`server-wire-punch`、`server-wire-derp` 接入 `service-biz` 时需要配置同一个内部 token：

```bash
SLAN_INTERNAL_WIRE_TOKEN=change-me-wire-internal-token
```

`service-biz` 代理 punch connect-session 时从 ops 管理的 punch 节点中选择可用节点。punch 节点在 biz 上用 `publicUdpIp/publicUdpPort` 管理，不再配置 punch 域名或内网地址。可用多节点环境变量做启动种子，格式为 `name=ip:udpPort`：

```bash
SLAN_WIRE_PUNCH_NODES=local=47.245.40.231:29130,backup=47.245.40.232:29130
```

内置运营管理员用于本地联调：

- 邮箱：`admin1`
- 密码：`admin1`

## Core APIs

- `POST /api/auth/register`
- `POST /api/auth/login`
- `POST /api/auth/device-login-devices`
- `POST /api/auth/device-login-devices/{deviceId}/complete`
- `GET /api/users`
- `GET /api/users/{userId}/entitlement`
- `PATCH /api/users/{userId}/password`
- `GET /api/user-aliases?ownerUserId=...`
- `PATCH /api/user-aliases`
- `GET /api/devices`
- `GET /api/devices/visible?userId=...`
- `POST /api/devices/register`
- `POST /api/devices/{deviceId}/renew`
- `GET /api/devices/{deviceId}/network-configs`
- `GET /api/devices/{deviceId}/mqtt-credential`
- `PATCH /api/devices/{deviceId}`
- `DELETE /api/devices/{deviceId}`
- `POST /api/device-invites`
- `GET /api/device-invites?userId=...`
- `POST /api/device-invites/accept`
- `GET /api/networks`
- `POST /api/networks`
- `PATCH /api/networks/{networkId}`
- `GET /api/networks/{networkId}/devices`
- `POST /api/networks/{networkId}/devices`
- `PATCH /api/networks/{networkId}/devices/{deviceId}`
- `DELETE /api/networks/{networkId}/devices/{deviceId}`
- `GET /api/networks/{networkId}/dns/zones`
- `POST /api/networks/{networkId}/dns/zones`
- `PATCH /api/networks/{networkId}/dns/zones/{zoneId}`
- `DELETE /api/networks/{networkId}/dns/zones/{zoneId}`
- `GET /api/networks/{networkId}/dns/records`
- `POST /api/networks/{networkId}/dns/records`
- `PATCH /api/networks/{networkId}/dns/records/{recordId}`
- `DELETE /api/networks/{networkId}/dns/records/{recordId}`
- `GET /api/networks/{networkId}/public-mappings`
- `POST /api/networks/{networkId}/public-mappings`
- `PATCH /api/networks/{networkId}/public-mappings/{mappingId}`
- `DELETE /api/networks/{networkId}/public-mappings/{mappingId}`
- `GET /api/networks/{networkId}/security-groups`
- `POST /api/networks/{networkId}/security-groups`
- `DELETE /api/networks/{networkId}/security-groups/{securityGroupId}`
- `GET /api/security-groups/{securityGroupId}/rules`
- `POST /api/security-groups/{securityGroupId}/rules`
- `PATCH /api/security-groups/rules/{ruleId}`
- `DELETE /api/security-groups/rules/{ruleId}`
- `GET /api/networks/{networkId}/network-config?deviceId=...`
- `GET /api/networks/{networkId}/relay-candidates?deviceId=...`
- `POST /api/networks/{networkId}/relay-candidates`
- `POST /api/relay/tickets`

## Ops APIs

- `POST /api/ops/auth/login`
- `PATCH /api/ops/auth/password`
- `GET /api/ops/dashboard`
- `GET /api/ops/operators`
- `POST /api/ops/operators`
- `PATCH /api/ops/operators/{operatorId}`
- `POST /api/ops/operators/{operatorId}/password`
- `GET /api/ops/relay-nodes`
- `POST /api/ops/relay-nodes`
- `PATCH /api/ops/relay-nodes/{nodeId}`
- `GET /api/ops/customers`
- `POST /api/ops/customers/{customerId}/assign-plan`
- `GET /api/ops/plans`
- `POST /api/ops/plans`
- `PATCH /api/ops/plans/{planCode}`
- `GET /api/ops/products`
- `POST /api/ops/products`
- `PATCH /api/ops/products/{productId}`
- `GET /api/ops/orders`
- `POST /api/ops/orders`
- `GET /api/ops/renewals`

## Internal Wire APIs

这些接口仅供 wire 控制面、DERP、Relay 节点调用，必须携带 `X-Slan-Internal-Token: $SLAN_INTERNAL_WIRE_TOKEN`。

- `GET /internal/wire/peers/{peerId}/authz`
- `GET /internal/wire/peers/{peerId}/runtime-config`
- `GET /internal/wire/networks/{networkId}/topology`
- `GET /internal/wire/derp-map`
- `GET /internal/wire/admin/relay-nodes`
- `PUT /internal/wire/admin/relay-nodes`
- `POST /internal/wire/admin/relay-nodes/{regionId}/{nodeId}/heartbeat`
- `PATCH /internal/wire/admin/relay-nodes/{regionId}/{nodeId}/status`
- `GET /internal/wire/admin/derp-nodes`
- `PUT /internal/wire/admin/derp-nodes`
- `POST /internal/wire/admin/derp-nodes/{regionId}/{nodeId}/heartbeat`
- `PATCH /internal/wire/admin/derp-nodes/{regionId}/{nodeId}/status`

开发与部署统一使用远程 Docker context `slan-remote`，详见仓库根目录
`docs/remote-docker-only.md`。`scripts/local_docker_*.sh` 仅保留为显式
本机调试/清理入口，默认拒绝运行。
