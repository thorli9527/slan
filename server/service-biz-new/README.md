# service-biz-new

`service-biz-new` 是 SLAN 当前业务控制面，承接客户 Web、运营管理、设备注册、网络配置和 wire 内部授权：

- 全局设备 IP 池：所有设备从 `10.0.0.0/8` 统一分配唯一 SLAN IP。
- 用户短码域名：注册用户自动获得 `{userSlug}.slan.com`。
- 网络模型：网络是一组设备和访问策略，默认网络为 `default`。
- 设备授权：生成 32 位一次性接入码，设备 owner 确认后授权邀请方可见。
- 内网域名：每个网络维护自己的 DNS Zone 和解析记录。
- 公网访问：按 `{alias}.{networkCode}.{userSlug}.pub.staticlss.com` 映射到设备端口。
- 安全组：默认 deny，按网络规则生成最终 peers/ACL 配置。

## Run

```bash
go run ./cmd/service-biz-new
```

默认监听 `:38080`，可通过 `SLAN_BIZ_NEW_ADDR` 修改。

`server-wire`、`server-wire-relay`、`server-wire-derp` 接入 `service-biz-new` 时需要配置同一个内部 token：

```bash
SLAN_INTERNAL_WIRE_TOKEN=change-me-wire-internal-token
```

内置运营管理员用于本地联调：

- 邮箱：`admin@slan.local`
- 密码：`admin123456`

## Core APIs

- `POST /api/auth/register`
- `POST /api/auth/login`
- `POST /api/auth/device-login-callbacks`
- `GET /api/auth/device-login-callbacks/{callbackId}`
- `POST /api/auth/device-login-callbacks/{callbackId}/complete`
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

本地开发栈见仓库根目录 `docker-compose.local.yml` 与 `scripts/local_docker_up.sh`。
