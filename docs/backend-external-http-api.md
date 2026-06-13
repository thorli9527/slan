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
- `/mqtt/bifromq/*`：BifroMQ broker 鉴权/ACL 回调。
- `server-wire`、`server-wire-relay`、`server-wire-derp` 的管理 HTTP：数据面或运维面接口，默认不作为 App/Web 业务 API。
- `server-wire-punch` 的 `/v1/*`：打洞服务控制面，业务入口由 `service-biz` 代理。

## Auth

| Auth Type | Header | 用途 |
| --- | --- | --- |
| User Bearer | `Authorization: Bearer <userToken>` | Web 控制台、用户态 API、设备绑定 |
| Device Bearer | `Authorization: Bearer <deviceToken>` | 设备 session 续期、设备态控制面请求 |
| Operator Bearer | `Authorization: Bearer <opsToken>` | 运营后台 `/api/ops/*` |
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
| Go DTO / Interface | `server/service-biz/internal/biz/http_api_contracts.go` | 后端代码中的命名 request/response DTO 与 `ExternalHTTPAPI` 接口 |
| Implementation | `server/service-biz/internal/biz/server.go`, `punch.go`, `ops_server.go`, `download_server.go` | HTTP handler、store 调用、审计、MQTT 通知等实现 |

新增或修改外部业务接口时，必须先更新 OpenAPI 和 Go DTO，再更新 handler 实现。handler 中不应继续新增匿名 request struct；应使用 `*Request`/`*Response` 命名类型。

## System And Downloads

| Method | Path | Auth | Request | Response | 用途 |
| --- | --- | --- | --- | --- | --- |
| GET | `/healthz` | None | - | `{status}` | API 健康检查 |
| GET | `/api/client-downloads` | None | - | `{items: ClientDownload[]}` | 客户端下载列表 |
| GET | `/downloads/clients/{fileName}` | None | - | file/script | 下载客户端安装包或 `install.sh` |

## Auth And User Session

| Method | Path | Auth | Request | Response | 用途 |
| --- | --- | --- | --- | --- | --- |
| POST | `/api/auth/register` | None | `{email,password,name}` | `{auth, defaultNetwork}` | 注册用户并创建默认网络 |
| POST | `/api/auth/login` | None | `{email,password}` | `{auth}` | 用户登录 |
| POST | `/api/auth/renew` | User Bearer | - | `{auth}` | 续期用户 session |
| POST | `/api/auth/logout` | User Bearer optional body | `{deviceToken?}` | `{status}` | 注销用户和可选设备 session |
| PATCH | `/api/users/{userId}/password` | User context | `{oldPassword,newPassword}` | `{status}` | 修改用户密码 |
| GET | `/api/users` | User/Web | - | `{items: User[]}` | 用户列表 |
| GET | `/api/users/{userId}/entitlement` | User/Web | - | `DeviceQuota` | 用户套餐/设备额度 |
| GET | `/api/user-aliases?ownerUserId=...` | User/Web | - | `{items: UserAlias[]}` | 用户别名列表 |
| PATCH | `/api/user-aliases` | User/Web | `{ownerUserId,email,alias}` | `UserAlias` | 设置用户别名 |

## Device Login And Bootstrap

| Method | Path | Auth | Request | Response | 用途 |
| --- | --- | --- | --- | --- | --- |
| POST | `/api/auth/console-login-keys` | User Bearer | `{deviceId}` | `ConsoleLoginKey` | Web 创建设备登录确认 key |
| POST | `/api/auth/console-login` | None | `{loginKey}` | `{auth}` | Web 使用 login key 登录 |
| POST | `/api/auth/device-login-devices` | None | `{deviceId,name,platform,osName,osVersion,alias,publicKey,deviceVersion?}` | `{deviceId,loginUrl,mqtt}` | 设备发起扫码/控制台登录 |
| POST | `/api/auth/device-login-devices/{deviceId}/complete` | User token in body | `{accessToken|token,action}` | `{status,deviceId,deliveryId}` | 用户确认设备登录并通过 MQTT 通知设备 |
| POST | `/api/web/device-bootstrap-keys` | User Bearer or `userId` | `{userId?,networkId,deviceAlias?,ttlSeconds?}` | `DeviceBootstrapKey` | 创建无人值守设备引导 key |
| GET | `/api/web/device-bootstrap-keys?userId=...` | User Bearer or query | - | `{items}` | 查询引导 key |
| POST | `/api/web/device-bootstrap-keys/{keyId}/revoke` | User Bearer or body | `{userId?}` | `DeviceBootstrapKey` | 撤销引导 key |

## Device Session And Devices

| Method | Path | Auth | Request | Response | 用途 |
| --- | --- | --- | --- | --- | --- |
| POST | `/api/device/session/bootstrap` | Session key | `{sessionKey,deviceId,name,platform,osName,osVersion,alias,publicKey}` | `{device,deviceSession,mqtt,networkConfigs}` | 设备用 bootstrap key 换设备 session |
| POST | `/api/device/session/bind` | User Bearer | `{deviceId,name,platform,osName,osVersion,alias,publicKey}` | `{device,deviceSession,mqtt,networkConfigs}` | 用户登录态绑定本机设备 |
| POST | `/api/device/session/renew` | Device Bearer | `{networkEnabled,rxBytesTotal,txBytesTotal}` | `{device,deviceSession,mqtt,networkConfigs}` | 设备 session 续期和运行态上报 |
| GET | `/api/devices?userId=...` | User/Web | - | `{items: Device[]}` | 查询用户设备 |
| GET | `/api/devices/visible?userId=...` | User/Web | - | `{items: Device[]}` | 查询用户可见设备 |
| POST | `/api/devices/register` | User/Web | `{userId,deviceId,name,platform,osName,osVersion,alias,publicKey}` | `{device,defaultNetworkDevice,mqtt}` | 旧版设备注册 |
| POST | `/api/devices/{deviceId}/renew` | Device/User legacy | `{userId,networkEnabled,rxBytesTotal,txBytesTotal}` | `{device,mqtt,networkConfigs,leaseExpiresAt}` | 旧版设备续约 |
| GET | `/api/devices/{deviceId}/network-configs` | Device/User | - | `{deviceId,items}` | 设备网络配置列表 |
| GET | `/api/devices/{deviceId}/mqtt-credential` | Device/User | - | `{mqtt}` | 获取 MQTT 凭据 |
| PATCH | `/api/devices/{deviceId}` | User/Web | `{actorUserId,alias}` | `Device` | 修改设备别名 |
| DELETE | `/api/devices/{deviceId}` | User/Web | query/body `{actorUserId}` | `204` | 删除/解绑可见设备 |

## Device Invites

| Method | Path | Auth | Request | Response | 用途 |
| --- | --- | --- | --- | --- | --- |
| POST | `/api/device-invites` | User/Web | `{ownerUserId,inviteeEmail,deviceId,alias}` | `DeviceInvite` | 邀请其他用户访问设备 |
| GET | `/api/device-invites?userId=...` | User/Web | - | `{items}` | 查询设备邀请 |
| POST | `/api/device-invites/accept` | User/Web | `{inviteId,userId,alias}` | `{device,owner}` | 接受设备邀请 |

## Networks And Members

| Method | Path | Auth | Request | Response | 用途 |
| --- | --- | --- | --- | --- | --- |
| GET | `/api/networks?userId=...` | User/Web | - | `{items: Network[]}` | 网络列表 |
| POST | `/api/networks` | User/Web | `{ownerUserId,name,code,templateKey}` | `{network,defaultSecurityGroup,defaultDNSZone}` | 创建网络 |
| PATCH | `/api/networks/{networkId}` | User/Web | `{name,code,status}` | `Network` | 更新网络 |
| GET | `/api/networks/{networkId}/devices` | User/Web | - | `{items: NetworkDevice[]}` | 网络成员设备 |
| POST | `/api/networks/{networkId}/devices` | User/Web | `{deviceId,ownerUserId,alias,role,enabled}` | `NetworkDevice` | 添加设备到网络 |
| PATCH | `/api/networks/{networkId}/devices/{deviceId}` | User/Web | `{alias,role,enabled}` | `NetworkDevice` | 更新网络成员 |
| DELETE | `/api/networks/{networkId}/devices/{deviceId}` | User/Web | - | `{status}` | 移除网络成员 |
| GET | `/api/networks/{networkId}/network-config?deviceId=...` | Device/User | - | `NetworkConfig` | 单网络设备配置 |

## DNS

| Method | Path | Auth | Request | Response | 用途 |
| --- | --- | --- | --- | --- | --- |
| GET | `/api/networks/{networkId}/dns/zones` | User/Web | - | `{items}` | DNS zone 列表 |
| POST | `/api/networks/{networkId}/dns/zones` | User/Web | `{zoneName,exposeGlobal}` | `DNSZone` | 创建 DNS zone |
| PATCH | `/api/networks/{networkId}/dns/zones/{zoneId}` | User/Web | `{zoneName,exposeGlobal}` | `DNSZone` | 更新 DNS zone |
| DELETE | `/api/networks/{networkId}/dns/zones/{zoneId}` | User/Web | - | `{status}` | 删除 DNS zone |
| GET | `/api/networks/{networkId}/dns/records` | User/Web | - | `{items}` | DNS record 列表 |
| POST | `/api/networks/{networkId}/dns/records` | User/Web | `{zoneId,name,recordType,targetDeviceId,targetIp,cname,port,ttl}` | `DNSRecord` | 新增 DNS record |
| PATCH | `/api/networks/{networkId}/dns/records/{recordId}` | User/Web | `{name,recordType,targetDeviceId,targetIp,cname,port,ttl}` | `DNSRecord` | 更新 DNS record |
| DELETE | `/api/networks/{networkId}/dns/records/{recordId}` | User/Web | - | `{status}` | 删除 DNS record |

## Public Mappings

| Method | Path | Auth | Request | Response | 用途 |
| --- | --- | --- | --- | --- | --- |
| GET | `/api/networks/{networkId}/public-mappings` | User/Web | - | `{items}` | 公网映射列表 |
| POST | `/api/networks/{networkId}/public-mappings` | User/Web | `{alias,publicDomain,sourceRecord,deviceId,protocol,port,externalPort,status}` | `PublicDomainMapping` | 创建公网映射 |
| PATCH | `/api/networks/{networkId}/public-mappings/{mappingId}` | User/Web | same as create | `PublicDomainMapping` | 更新公网映射 |
| DELETE | `/api/networks/{networkId}/public-mappings/{mappingId}` | User/Web | - | `{status}` | 删除公网映射 |

## Security Groups

| Method | Path | Auth | Request | Response | 用途 |
| --- | --- | --- | --- | --- | --- |
| GET | `/api/networks/{networkId}/security-groups` | User/Web | - | `{items}` | 安全组列表 |
| POST | `/api/networks/{networkId}/security-groups` | User/Web | `{name,description}` | `SecurityGroup` | 创建安全组 |
| DELETE | `/api/networks/{networkId}/security-groups/{securityGroupId}` | User/Web | - | `204` | 删除安全组 |
| GET | `/api/security-groups/{securityGroupId}/rules` | User/Web | - | `{items}` | 安全组规则列表 |
| POST | `/api/security-groups/{securityGroupId}/rules` | User/Web | `{direction,priority,action,protocol,portFrom,portTo,peerType,peerValue,description,enabled}` | `SecurityRule` | 新增规则 |
| PATCH | `/api/security-groups/rules/{ruleId}` | User/Web | same as create | `SecurityRule` | 更新规则 |
| DELETE | `/api/security-groups/rules/{ruleId}` | User/Web | - | `{status}` | 删除规则 |

## Relay And P2P Punch

| Method | Path | Auth | Request | Response | 用途 |
| --- | --- | --- | --- | --- | --- |
| GET | `/api/networks/{networkId}/relay-candidates?deviceId=...` | Device/User | - | `{items: RelayCandidate[]}` | 获取 relay 候选 |
| POST | `/api/networks/{networkId}/relay-candidates` | Device/User | `{deviceId}` | `{items: RelayCandidate[]}` | 获取 relay 候选 |
| POST | `/api/relay/tickets` | Device/User | `{networkId,srcNodeId,dstNodeId,derpClusterId?,preferredDerpNodeIds?,preferredRelayEndpointIds?,reason?,relayRegionId?}` | `RelayTicket` | 申请 relay ticket |
| POST | `/api/networks/{networkId}/punch/connect-sessions` | Punch MQTT Signature | `{requesterNodeId,peerNodeId,ttlSeconds?}` | `PunchConnectSession` | 申请 P2P 打洞会话，biz 代理到 punch-service |

## Ops Auth

所有 `/api/ops/*` 除登录外都使用 `Operator Bearer`。

| Method | Path | Auth | Request | Response | 用途 |
| --- | --- | --- | --- | --- | --- |
| POST | `/api/ops/auth/login` | None | `{email,password}` | `{auth}` | 运营登录 |
| PATCH | `/api/ops/auth/password` | Operator Bearer | `{oldPassword,newPassword}` | `204` | 修改当前运营密码 |
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
| PATCH | `/api/ops/customers/{customerId}` | `CustomerProfile` | `CustomerProfile` | 更新客户状态/资料 |
| POST | `/api/ops/customers/{customerId}/assign-plan` | `{planCode,expiresAt,amount,period}` | `{customer,renewal}` | 分配套餐 |
| GET | `/api/ops/devices` | - | `{items}` | 设备运营视图 |
| PATCH | `/api/ops/devices/{deviceId}` | `{alias,status,enabled}` | `OpsDeviceView` | 更新设备状态 |
| DELETE | `/api/ops/devices/{deviceId}` | - | `204` | 删除设备 |

## Ops Network Nodes

| Method | Path | Request | Response | 用途 |
| --- | --- | --- | --- | --- |
| GET | `/api/ops/relay-nodes` | - | `{items}` | relay 节点列表 |
| POST | `/api/ops/relay-nodes` | `OpsRelayNode` | `OpsRelayNode` | 创建 relay 节点 |
| PATCH | `/api/ops/relay-nodes/{nodeId}` | `OpsRelayNode` | `OpsRelayNode` | 更新 relay 节点 |
| DELETE | `/api/ops/relay-nodes/{nodeId}` | - | `204 No Content` | 删除 relay/DERP 节点 |
| GET | `/api/ops/punch-nodes` | - | `{items}` | punch 节点列表 |
| POST | `/api/ops/punch-nodes` | `{name,region,publicUdpIp,publicUdpPort,maxSessions,status,health,priority}` | `OpsPunchNode` | 创建 punch 节点 |
| PATCH | `/api/ops/punch-nodes/{nodeId}` | same as create | `OpsPunchNode` | 更新 punch 节点 |
| DELETE | `/api/ops/punch-nodes/{nodeId}` | - | `204 No Content` | 删除 punch 节点 |

## Ops Billing And Downloads

| Method | Path | Request | Response | 用途 |
| --- | --- | --- | --- | --- |
| GET | `/api/ops/plans` | - | `{items}` | 套餐列表 |
| POST | `/api/ops/plans` | `OpsPlan` | `OpsPlan` | 创建/更新套餐 |
| PATCH | `/api/ops/plans/{planCode}` | `OpsPlan` | `OpsPlan` | 更新套餐 |
| GET | `/api/ops/products` | - | `{items}` | 商品列表 |
| POST | `/api/ops/products` | `Product` | `Product` | 创建商品 |
| PATCH | `/api/ops/products/{productId}` | `Product` | `Product` | 更新商品 |
| GET | `/api/ops/orders` | - | `{items}` | 订单列表 |
| POST | `/api/ops/orders` | `Order` | `Order` | 创建订单 |
| PATCH | `/api/ops/orders/{orderId}` | `Order` | `Order` | 更新订单 |
| GET | `/api/ops/renewals` | - | `{items}` | 续费记录列表 |
| PATCH | `/api/ops/renewals/{renewalId}` | `Renewal` | `Renewal` | 更新续费记录 |
| GET | `/api/ops/client-downloads` | - | `{items}` | 客户端下载包运营列表 |
| POST | `/api/ops/client-downloads` | multipart `{file,platform,version,channel,arch,releaseNotes,status}` | `ClientDownload` | 上传客户端包 |
| DELETE | `/api/ops/client-downloads/{downloadId}` | - | `204` | 删除客户端包 |

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
