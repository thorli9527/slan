# service-biz-new

`service-biz-new` 是 SLAN 新业务控制面原型，目标是从旧的网络/设备绑定模型迁移到：

- 全局设备 IP 池：所有设备从 `10.0.0.0/8` 统一分配唯一 SLAN IP。
- 全局域名服务：每台设备自动获得 `{deviceId}.slan`。
- 工作组模型：用户和设备加入工作组，默认工作组为 `default`。
- 单工作组启用：客户端请求某个工作组的最终网络配置。
- 工作组私有域：每个工作组有自己的私有域，例如 `default.slan`。
- 私有转全局映射：DNS 记录可选择是否暴露到全局域名映射。
- 工作组 ACL：默认 deny，按工作组规则生成最终 peers/ACL 配置。

## Run

```bash
go run ./cmd/service-biz-new
```

默认监听 `:38080`，可通过 `SLAN_BIZ_NEW_ADDR` 修改。

## Core APIs

- `POST /api/users/register`
- `POST /api/devices/register`
- `GET /api/dns/global`
- `GET /api/workspaces`
- `POST /api/workspaces`
- `POST /api/workspaces/{workspaceId}/members`
- `POST /api/workspaces/{workspaceId}/devices`
- `POST /api/workspaces/{workspaceId}/acl`
- `POST /api/workspaces/{workspaceId}/dns/records`
- `GET /api/workspaces/{workspaceId}/network-config?deviceId=...`

当前实现为内存存储，用于先固定接口和 UI。后续落地 Postgres/Redis、认证、审计和策略编译。
