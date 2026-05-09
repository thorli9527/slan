# service-biz-new

`service-biz-new` 是 SLAN 新业务控制面原型，目标是从旧的网络/设备绑定模型迁移到：

- 全局设备 IP 池：所有设备从 `10.0.0.0/8` 统一分配唯一 SLAN IP。
- 用户短码域名：注册用户自动获得 `{userSlug}.slan.com`。
- 网络模型：网络是一组设备和访问策略，默认网络为 `default`。
- 设备授权：生成 32 位一次性接入码，设备 owner 确认后授权邀请方可见。
- 内网域名：每个网络维护自己的 DNS Zone 和解析记录。
- 公网访问：按 `{alias}.{networkCode}.{userSlug}.pub.slan.com` 映射到设备端口。
- 安全组：默认 deny，按网络规则生成最终 peers/ACL 配置。

## Run

```bash
go run ./cmd/service-biz-new
```

默认监听 `:38080`，可通过 `SLAN_BIZ_NEW_ADDR` 修改。

## Core APIs

- `POST /api/auth/register`
- `POST /api/auth/login`
- `PATCH /api/users/{userId}/password`
- `GET /api/user-aliases?ownerUserId=...`
- `PATCH /api/user-aliases`
- `GET /api/devices/visible?userId=...`
- `POST /api/devices/register`
- `PATCH /api/devices/{deviceId}`
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
- `POST /api/networks/{networkId}/dns/records`
- `GET /api/networks/{networkId}/public-mappings`
- `POST /api/networks/{networkId}/public-mappings`
- `GET /api/networks/{networkId}/security-groups`
- `POST /api/networks/{networkId}/security-groups`
- `GET /api/security-groups/{securityGroupId}/rules`
- `POST /api/security-groups/{securityGroupId}/rules`
- `GET /api/networks/{networkId}/network-config?deviceId=...`

当前实现为内存存储，用于先固定接口和 UI。数据库/API 收敛说明见 `docs/client-web-db-api-alignment.md`。
