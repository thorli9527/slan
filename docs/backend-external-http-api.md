# Backend External HTTP API

本文档定义 SLAN 后端对外业务 HTTP 接口，用于梳理客户端、Web 控制台、运营后台与服务端之间的边界。

## Scope

主业务入口是 `service-biz`：

- 远程 Docker 入口示例：`http://47.245.40.231:28080`
- 本地 Docker 端口示例：`http://127.0.0.1:28080`
- 响应格式：默认 JSON
- 错误格式：`{"code":"...","message":"..."}`

不属于外部业务 API 的接口：

- `/internal/wire/*`：wire/relay/derp 内部服务调用。
- `/mqtt/emqx/*`、`/mqtt/bifromq/*`：MQTT broker（EMQX）鉴权/ACL 回调。
- `server-wire`、`server-wire-relay`、`server-wire-derp` 的管理 HTTP：数据面或运维面接口，默认不作为 App/Web 业务 API。
- `server-wire-punch` 的 `/v1/*`：打洞服务控制面，业务入口由 `service-biz` 代理。

## Auth

| Auth Type | Header | 用途 |
| --- | --- | --- |
| Device Bearer | `Authorization: Bearer <deviceToken>` | 设备 session 续期、设备态控制面请求 |
| Operator Bearer | `Authorization: Bearer <opsToken>` | 运营后台 `/api/ops/*` |

Device 与 Operator token 仅允许通过 `Authorization: Bearer` 传输；URL query 和 `X-Access-Token` 不被接受。
| Punch MQTT Signature | `X-Slan-Device-ID`, `X-Slan-MQTT-Username`, `X-Slan-Punch-Signature` | 设备请求 P2P punch connect-session |

Punch 签名约定：

```text
X-Slan-Device-ID: <deviceId>
X-Slan-MQTT-Username: <mqtt username issued by biz>
X-Slan-Punch-Signature: md5(deviceId + mqttPassword)
```

其中 `mqttPassword` 是 biz 下发给设备的 MQTT 密码。biz 根据设备 ID、MQTT username 和服务端保存的 MQTT secret 复算 MQTT 密码，再校验 MD5。

## Interface And Implementation Separation

外部 HTTP 业务接口需要按下面三层维护，避免接口定义继续散落在 handler 实现里：

| Layer | File | Responsibility |
| --- | --- | --- |
| API Index | `docs/backend-external-http-api.md` | 面向人阅读的业务接口目录、认证方式、用途说明 |
| OpenAPI Contract | `protocol/openapi/service-biz-external.yaml` | 面向工具的 HTTP path、method、request、response 定义 |
| Go DTO / Interface | `server/service-biz/internal/service` | 后端业务输入、视图和 use-case 接口 |
| Implementation | `server/service-biz/internal/api` | HTTP 路由、请求解析和响应适配 |

新增或修改外部业务接口时，必须先更新 OpenAPI 和 Go DTO，再更新 handler 实现。handler 中不应继续新增匿名 request struct；应使用 `*Request`/`*Response` 命名类型。

## System

| Method | Path | Auth | Request | Response | 用途 |
| --- | --- | --- | --- | --- | --- |
| GET | `/healthz` | None | - | `{status}` | API 健康检查 |

## Device Authorization And Session

| Method | Path | Auth | Request | Response | 用途 |
| --- | --- | --- | --- | --- | --- |
| POST | `/api/device-auth/token` | Authorization key | `{key,deviceId?}` | `{device,deviceSession,mqtt}` | 授权 key 换设备 token；未绑定 key 首次交换时创建设备 |
| POST | `/api/app/device/session/renew` | Device Bearer | `{refreshToken,networkEnabled,rxBytesTotal,txBytesTotal}` | `{device,deviceSession,mqtt,networkConfigs}` | 设备 session 续期和运行态上报 |
| POST | `/api/app/devices/{deviceId}/renew` | Device Bearer | `{networkEnabled,rxBytesTotal,txBytesTotal}` | `{device,mqtt,networkConfigs,leaseExpiresAt}` | 设备运行租约续期 |
| GET | `/api/app/devices/{deviceId}/network-configs` | Device Bearer | - | `{deviceId,items}` | 设备网络配置列表 |
| GET | `/api/app/devices/{deviceId}/mqtt-credential` | Device Bearer | - | `{mqtt}` | 获取 MQTT 凭据 |

Device Session 续期按凭据摘要限制为每分钟 30 次，设备运行态上报按 Device token 摘要限制为每分钟 120 次；超限返回 `429 Too Many Requests`。服务端限流键不保存原始 token。当前为单进程有界内存限流，多实例生产环境仍需在受信任网关或共享存储上实现集群级限流。

授权 Key 兑换按来源 IP 及来源 IP + Key 摘要双维度限制失败次数。1 分钟内任一维度达到 5 次失败后返回 `429 Too Many Requests`；成功兑换或窗口到期后恢复。服务端不会为限流保存完整 Key。

## Device Credentials

完整授权 Key 仅在创建响应中返回一次。服务端只保存使用 `SLAN_DEVICE_CREDENTIAL_PEPPER` 计算的 HMAC-SHA256 摘要。

| Method | Path | Auth | Input | Output | Description |
|---|---|---|---|---|---|
| GET | `/api/ops/device-credentials?deviceId=...` | Operator Bearer | - | `{items}` | 查询授权 Key，响应不含完整 Key |
| POST | `/api/ops/device-credentials` | Operator Bearer | `{deviceId,name,scopes,expiresAt}` | `DeviceCredential` with one-time `key` | 创建授权 Key |
| GET | `/api/ops/devices/{deviceId}/credentials` | Operator Bearer | - | `{items}` | 查询指定设备的授权 Key |
| POST | `/api/ops/devices/{deviceId}/credentials` | Operator Bearer | `{name,scopes,expiresAt}` | `DeviceCredential` with one-time `key` | 为指定设备创建授权 Key |

当前仅支持 `standard_device` 权限模板。未实现端点授权规则的 scope 会被拒绝，避免有限权限 Key 实际获得完整设备权限。
| POST | `/api/ops/device-credentials/{credentialId}/revoke` | Operator Bearer | - | `DeviceCredential` | 吊销 Key，并阻断关联 Device Session 和 MQTT 凭证 |
| POST | `/api/device-auth/token` | Device authorization key | `{key,deviceId?}` | `DeviceSessionBoundView` | 使用授权 Key 换取 Device Token 和 MQTT 配置 |

## Operator Managed Resources

Network、Device 和 DeviceGroup 均为平台资源，由 Operator 管理，不存在客户端账号归属。

| Method | Path | Description |
|---|---|---|
| GET/POST | `/api/ops/networks` | 查询或创建网络 |
| PATCH/DELETE | `/api/ops/networks/{networkId}` | 更新或删除网络 |
| POST | `/api/ops/networks/{networkId}/devices` | 添加网络设备 |
| DELETE | `/api/ops/networks/{networkId}/devices/{deviceId}` | 移除网络设备 |
| POST | `/api/ops/networks/{networkId}/device-groups` | 引用设备组 |
| DELETE | `/api/ops/networks/{networkId}/device-groups/{groupId}` | 移除设备组引用 |
| GET/POST | `/api/ops/devices` | 查询或创建独立设备 |
| PATCH/DELETE | `/api/ops/devices/{deviceId}` | 更新或删除设备 |
| GET/POST | `/api/ops/device-groups` | 查询或创建设备组 |
| PATCH/DELETE | `/api/ops/device-groups/{groupId}` | 更新或删除设备组 |
| POST | `/api/ops/device-groups/{groupId}/devices` | 添加设备组成员 |
| DELETE | `/api/ops/device-groups/{groupId}/devices/{deviceId}` | 移除设备组成员 |

## Networks And Members

网络及其设备、设备组关联只通过上面的 Operator Managed Resources 接口管理。`/api/app/networks/{networkId}/network-config` 仅允许设备 token 读取运行配置。

## DNS

DNS 管理能力只挂在 Opt 下。

## Public Mappings

公网映射尚未纳入当前外部业务接口。

## Security Groups

安全组管理能力只挂在 Opt 下。ACL peer 仅支持设备和设备组。

## Relay And P2P Punch

| Method | Path | Auth | Request | Response | 用途 |
| --- | --- | --- | --- | --- | --- |
| GET | `/api/app/networks/{networkId}/relay-candidates?deviceId=...` | Device Bearer | - | `{items: RelayCandidate[]}` | 获取 relay 候选 |
| POST | `/api/app/networks/{networkId}/relay-candidates` | Device Bearer | `{deviceId}` | `{items: RelayCandidate[]}` | 获取 relay 候选 |
| POST | `/api/app/relay/tickets` | Device Bearer | `{networkId,srcNodeId,dstNodeId,derpClusterId?,preferredDerpNodeIds?,preferredRelayEndpointIds?,reason?,relayRegionId?}` | `RelayTicket` | 申请 relay ticket |
| POST | `/api/app/networks/{networkId}/punch/connect-sessions` | Punch MQTT Signature | `{requesterNodeId,peerNodeId,ttlSeconds?}` | `PunchConnectSession` | 申请 P2P 打洞会话，biz 代理到 punch-service |

## Ops Auth

所有 `/api/ops/*` 除登录外都使用 `Operator Bearer`。

后台仅支持 `admin` 和 `operator` 两种角色。普通 `operator` 可管理业务资源和设备授权 Key；运营账号目录、角色、状态以及他人密码重置仅允许 `admin`。系统始终要求至少保留一个 active admin，禁止停用或降级最后一个管理员。自助改密始终作用于当前登录账号，必须提交当前密码；改密、管理员重置密码或停用账号都会撤销该账号全部 Operator Session。

运营登录按来源 IP 和规范化邮箱双维度限制失败次数。1 分钟内任一维度达到 5 次失败后返回 `429 Too Many Requests`，成功登录或窗口到期后恢复。

登录成功时会清理已过期的 Operator Session。登录成功、登录失败和登出结果均写入审计日志；审计记录不包含密码或 token。

同一授权 Key 在成功交换时如果来源 IP 与上次成功使用不同，会额外写入 `exchange_source_changed` / `warning` 审计事件。事件包含 `remoteIp` 和不含凭据的 `detail`，用于 Ops 筛选和后续告警消费。

| Method | Path | Auth | Request | Response | 用途 |
| --- | --- | --- | --- | --- | --- |
| POST | `/api/ops/auth/login` | None | `{email,password}` | `{auth}` | 运营登录 |
| PATCH | `/api/ops/auth/password` | Operator Bearer | `{oldPassword,newPassword}` | `OperatorUser` | 修改当前运营密码并撤销全部 session |
| POST | `/api/ops/auth/logout` | Operator Bearer | - | `204` | 撤销当前 Operator Session |
| GET | `/api/ops/dashboard` | Operator Bearer | - | `OpsDashboard` | 运营首页数据 |
| GET | `/api/ops/audit-events?actorType=&actorId=&action=&resourceType=&resourceId=&status=&limit=` | Operator Bearer | - | `{items}` | 审计日志查询 |

## Ops Resources

| Method | Path | Request | Response | 用途 |
| --- | --- | --- | --- | --- |
| GET | `/api/ops/operators` | - | `{items}` | 运营账号列表 |
| POST | `/api/ops/operators` | `OperatorUser` | `OperatorUser` | 创建运营账号 |
| PATCH | `/api/ops/operators/{operatorId}` | `OperatorUser` | `OperatorUser` | 更新运营账号 |
| POST | `/api/ops/operators/{operatorId}/password` | `{newPassword}` | `204` | 重置运营密码 |
| GET | `/api/ops/customers` | - | `{items}` | 客户列表 |
| POST | `/api/ops/customers` | `CustomerProfile` | `CustomerProfile` | 创建独立客户 |
| PATCH | `/api/ops/customers/{customerId}` | `CustomerProfile` | `CustomerProfile` | 更新客户状态/资料 |
| GET | `/api/ops/devices` | - | `{items}` | 设备运营视图 |
| PATCH | `/api/ops/devices/{deviceId}` | `{alias,status,enabled}` | `OpsDeviceView` | 更新设备状态 |
| DELETE | `/api/ops/devices/{deviceId}` | - | `204` | 删除设备 |

## Ops Server Nodes

| Method | Path | Request | Response | 用途 |
| --- | --- | --- | --- | --- |
| GET | `/api/ops/server-nodes` | - | `{items}` | 服务器节点列表 |
| POST | `/api/ops/server-nodes` | 服务器、SSH 和服务端口配置 | `OpsServerNode` | 创建服务器节点 |
| PATCH | `/api/ops/server-nodes/{nodeId}` | same as create | `OpsServerNode` | 更新服务器节点及启用的服务 |
| POST | `/api/ops/server-nodes/{nodeId}/ssh-host-key` | `{}` | `{fingerprint}` | 首次部署前探测 SSH 主机指纹 |
| POST | `/api/ops/server-nodes/{nodeId}/deploy` | `{sshHostKeyFingerprint}` | `OpsServerNode` | 确认首次主机指纹后，通过 SSH 部署中继、打洞和代理服务 |
| DELETE | `/api/ops/server-nodes/{nodeId}` | - | `{ok:true}` | 删除服务器节点及其运行时注册记录 |

服务器节点是运营侧唯一的节点管理入口。Relay 与 Punch 进程通过内部 Wire API 自注册并
维持心跳，其运行时记录仅用于路径调度，不提供独立的外部 CRUD API。

## MQTT Broker Webhooks

所有 `/mqtt/*` 回调在配置 `SLAN_MQTT_WEBHOOK_TOKEN` 后都必须携带 `X-Slan-MQTT-Webhook-Token`。生产环境强制配置至少 32 字符的独立随机 token，EMQX 必须在鉴权、ACL 回调中统一发送该 Header（`deploy/local/emqx/emqx.conf`）。未配置 token 的兼容行为仅用于本地开发。

`POST /mqtt/emqx/auth`（兼容保留 `/mqtt/bifromq/auth`）对同一 `clientId + username` 的失败鉴权限制为每分钟 10 次，对同一 Broker 来源的聚合失败限制为每分钟 1000 次。成功鉴权清除该客户端身份的失败记录，不清除 Broker 来源聚合记录。限流键仅保存 SHA-256 摘要，不保存 MQTT 密码。超限返回 `429 Too Many Requests`。

EMQX 适配响应格式：鉴权允许返回 `200 {"result":"allow"}`（server 主体附带 `"is_superuser":true`），拒绝返回 `403 {"result":"deny"}`；ACL 检查同格式（`/mqtt/emqx/check`）。

当前限流状态为单进程有界内存数据；多实例 Broker webhook 需在网关或共享存储实现集群级失败计数。

## Direct Service HTTP Surfaces

这些服务有 HTTP 端口，但不建议 App/Web 直接调用业务操作。

### server-wire

| Method | Path | 用途 |
| --- | --- | --- |
| GET | `/healthz` | 健康检查 |
| POST | `/v1/peers/register` | wire peer 注册 |
| POST | `/v1/peers/endpoints` | peer endpoint 上报 |
| POST | `/v1/peers/path-health` | path health 上报 |
| POST | `/v1/peers/derp-health` | DERP health 上报 |
| POST | `/v1/peers/active-path` | active path 上报 |
| GET | `/v1/peers/{peerId}/routes` | peer routes |
| POST | `/v1/relay/tickets` | relay ticket |
| GET | `/v1/derp/map` | DERP map |
| POST | `/v1/derp/tickets` | DERP ticket |
| POST | `/v1/path-plan` | path plan |

### server-wire-punch

| Method | Path | Auth | 用途 |
| --- | --- | --- | --- |
| GET | `/healthz` | None | 健康检查 |
| GET | `/v1/stats` | Internal/ops | punch 状态统计 |
| POST | `/v1/endpoints` | Internal + punch signature | endpoint 上报 |
| GET | `/v1/endpoints/{networkId}/{nodeId}` | Internal | endpoint 查询 |
| POST | `/v1/connect-sessions` | Internal + punch signature | 创建打洞会话 |
| GET | `/v1/connect-sessions/{sessionId}` | Internal | 查询打洞会话 |

### server-wire-relay Admin

| Method | Path | 用途 |
| --- | --- | --- |
| GET | `/healthz` | 健康检查 |
| GET | `/v1/sessions` | relay session 列表 |
| GET | `/v1/sessions/{sessionId}` | relay session 详情 |
| GET | `/v1/ticket-key-status` | ticket key 状态 |
| GET | `/metrics` | metrics |

### server-wire-derp Admin

| Method | Path | 用途 |
| --- | --- | --- |
| GET | `/healthz` | 健康检查 |
| GET | `/v1/connections` | DERP 连接列表 |
| GET | `/v1/connections/{connectionId}` | DERP 连接详情 |
| GET | `/v1/sessions` | DERP session 列表 |
| GET | `/v1/sessions/{sessionId}` | DERP session 详情 |
| GET | `/v1/regions` | DERP region 列表 |
| GET | `/v1/ticket-key-status` | ticket key 状态 |
| GET | `/metrics` | metrics |
