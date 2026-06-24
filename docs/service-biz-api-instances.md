# Service Biz API Instances

`service-biz` 使用同一镜像运行三个实例，通过 `SLAN_BIZ_ROUTE_SET` 分离 App、Web Console 和 Ops 接口。新增接口时必须先确认调用方归属，再加入对应路由文件：

- App API: `server/service-biz/internal/biz/app_api_routes.go`
- Web Console API: `server/service-biz/internal/biz/web_console_api_routes.go`
- Ops API: `server/service-biz/internal/biz/ops_server.go`
- MQTT webhook: `server/service-biz/internal/biz/mqtt_api_routes.go`
- Shared route helper: `server/service-biz/internal/biz/api_routes.go`

| 实例 | Route set | 默认端口 | 调用方 | 备注 |
| --- | --- | --- | --- | --- |
| App API | `app` | `28080` | 桌面端、移动端、wire/relay/derp、BifroMQ | 唯一启动 MQTT subscriber / retry worker 的业务实例 |
| Web Console API | `web` | `28081` | `/opt/web-console` / `server-ui-web` | 面向普通用户控制台，不暴露 ops、MQTT、内部 wire |
| Ops API | `ops` | `28082` | 运维/运营控制台 `opt-ui` | 只暴露 `/api/ops/*` 管理接口 |

## Shared

| Method | Path | Auth | 备注 |
| --- | --- | --- | --- |
| `GET` | `/healthz` | 无 | 健康检查；部署脚本用它确认三个实例都可用 |
| `GET` | `/api/client-downloads` | 无 | 公共客户端下载列表；App/Web/Ops 都可能展示下载入口 |
| `GET` | `/downloads/clients/{fileName}` | 无 | 客户端安装包下载；文件名必须来自已登记下载记录 |

## App API

App API 服务桌面端和移动端控制面。它同时注册 `/internal/wire/*` 和 `/mqtt/*`，并默认启动 MQTT control subscriber / delivery retry worker。

### Auth And Device Session

| Method | Path | Auth | 备注 |
| --- | --- | --- | --- |
| `POST` | `/api/auth/register` | 无 | 移动端集成测试和首次账号注册入口；创建用户会初始化默认资源 |
| `POST` | `/api/auth/login` | 无 | 桌面/移动端账号密码登录；可携带本机 `deviceId` 绑定登录上下文 |
| `POST` | `/api/auth/renew` | 用户 token | 刷新用户会话；客户端重新拉取登录态时使用 |
| `POST` | `/api/auth/logout` | 用户 token | 用户退出登录；可同时传入 device token 清理设备会话 |
| `POST` | `/api/auth/console-login-keys` | 用户 token | App 生成 Web Console 快捷登录 key；不要放到 Ops 实例 |
| `POST` | `/api/auth/device-login-devices` | 无 | App 发起设备登录准备流程；Web Console 也需要该接口来配合确认 |
| `GET` | `/api/devices` | 可选用户 token/query | App 侧列出可见设备；历史客户端仍使用该入口 |
| `POST` | `/api/devices/register` | 用户 token | 用户登录态下注册设备；普通 Web Console 也支持手动注册 |
| `POST` | `/api/devices/{deviceId}/renew` | 用户 token | 老设备注册续期接口；保留给兼容客户端 |
| `POST` | `/api/device/session/bootstrap` | session key | 通过安装/接入码创建设备 session |
| `POST` | `/api/device/session/bind` | 用户 token | 将本机设备 session 绑定到当前用户 |
| `POST` | `/api/device/session/renew` | 设备 token | 设备心跳续期，同时上报网络启用状态和流量统计 |

### Network Runtime

| Method | Path | Auth | 备注 |
| --- | --- | --- | --- |
| `GET` | `/api/devices/{deviceId}/network-configs` | 用户/设备 token | 返回设备参与的网络配置集合；客户端合并/覆盖规则从这里开始 |
| `GET` | `/api/devices/{deviceId}/mqtt-credential` | 用户/设备 token | 获取设备 MQTT 凭据；客户端用它订阅控制消息 |
| `GET` | `/api/networks/{networkId}/network-config?deviceId=...` | 用户/设备 token | 获取某网络对某设备的最终运行配置 |
| `GET` | `/api/networks/{networkId}/relay-candidates?deviceId=...` | 用户/设备 token | 获取当前网络的 relay 候选节点 |
| `POST` | `/api/networks/{networkId}/relay-candidates` | 设备签名/token | 设备上报或请求 relay 候选；主要用于路径选择 |
| `POST` | `/api/networks/{networkId}/punch/connect-sessions` | 设备签名 header | 创建 P2P punch 协商会话；请求头包含设备/MQTT 签名 |
| `POST` | `/api/relay/tickets` | 用户/设备 token | 获取 relay ticket；wire/relay 数据面鉴权使用 |

### MQTT Webhook

| Method | Path | Auth | 备注 |
| --- | --- | --- | --- |
| `POST` | `/mqtt/bifromq/auth` | BifroMQ webhook | Broker 连接认证回调；只注册在 App API 实例 |
| `POST` | `/mqtt/bifromq/check` | BifroMQ webhook | Broker 发布/订阅权限检查回调；只注册在 App API 实例 |

### Internal Wire

`/internal/wire/*` 只注册在 App API 实例，所有接口必须携带 `X-Slan-Internal-Token: $SLAN_INTERNAL_WIRE_TOKEN`。

| Method | Path | 备注 |
| --- | --- | --- |
| `GET` | `/internal/wire/peers/{peerId}/authz` | wire 节点校验 peer 访问权限 |
| `GET` | `/internal/wire/peers/{peerId}/runtime-config` | wire 节点获取 peer 运行配置 |
| `GET` | `/internal/wire/networks/{networkId}/topology` | wire 节点获取网络拓扑 |
| `GET` | `/internal/wire/derp-map` | wire/客户端获取 DERP map |
| `GET` | `/internal/wire/admin/relay-nodes` | relay 管理列表 |
| `PUT` | `/internal/wire/admin/relay-nodes` | relay 节点注册/更新 |
| `POST` | `/internal/wire/admin/relay-nodes/{regionId}/{nodeId}/heartbeat` | relay 心跳 |
| `PATCH` | `/internal/wire/admin/relay-nodes/{regionId}/{nodeId}/status` | relay 状态更新 |
| `DELETE` | `/internal/wire/admin/relay-nodes/{regionId}/{nodeId}` | relay 删除 |
| `GET` | `/internal/wire/admin/derp-nodes` | DERP 管理列表 |
| `PUT` | `/internal/wire/admin/derp-nodes` | DERP 节点注册/更新 |
| `POST` | `/internal/wire/admin/derp-nodes/{regionId}/{nodeId}/heartbeat` | DERP 心跳 |
| `PATCH` | `/internal/wire/admin/derp-nodes/{regionId}/{nodeId}/status` | DERP 状态更新 |
| `DELETE` | `/internal/wire/admin/derp-nodes/{regionId}/{nodeId}` | DERP 删除 |

## Web Console API

Web Console API 服务 `/opt/web-console`。它不注册 `/api/ops/*`、`/api/device/session/*`、`/mqtt/*`、`/internal/wire/*`。

### Auth

| Method | Path | Auth | 备注 |
| --- | --- | --- | --- |
| `POST` | `/api/auth/register` | 无 | Web Console 注册账号 |
| `POST` | `/api/auth/login` | 无 | Web Console 账号密码登录 |
| `POST` | `/api/auth/renew` | 用户 token | 刷新 Web Console 会话 |
| `POST` | `/api/auth/logout` | 用户 token | 退出 Web Console |
| `POST` | `/api/auth/console-login-keys` | 用户 token | 生成一次性控制台登录 key |
| `POST` | `/api/auth/console-login` | login key | App 跳转 Web Console 时使用 key 换取 Web 会话 |
| `POST` | `/api/auth/device-login-devices` | 无 | 准备设备登录确认流程 |
| `POST` | `/api/auth/device-login-devices/{deviceId}/complete` | 用户 token | 用户在 Web Console 确认某设备登录 |

### Users And Aliases

| Method | Path | Auth | 备注 |
| --- | --- | --- | --- |
| `GET` | `/api/users` | 用户 token | 管理可见用户列表；当前主要用于控制台初始化 |
| `GET` | `/api/users/{userId}/entitlement` | 用户 token | 获取套餐/配额信息 |
| `PATCH` | `/api/users/{userId}/password` | 用户 token | 修改账号密码 |
| `GET` | `/api/users/{userId}/user-aliases` | 用户 token | 获取指定用户别名 |
| `GET` | `/api/user-aliases` | 用户 token/query | 获取当前用户或指定 owner 的别名 |
| `PATCH` | `/api/user-aliases` | 用户 token | 修改邮箱显示别名和短域名前缀 |

### Devices And Groups

| Method | Path | Auth | 备注 |
| --- | --- | --- | --- |
| `GET` | `/api/devices` | 用户 token/query | 控制台设备列表 |
| `GET` | `/api/devices/visible` | 用户 token/query | 当前用户可见设备列表 |
| `POST` | `/api/devices/register` | 用户 token | 手动添加设备 |
| `PATCH` | `/api/devices/{deviceId}` | 用户 token | 修改设备别名等展示信息 |
| `DELETE` | `/api/devices/{deviceId}` | 用户 token | 删除设备 |
| `GET` | `/api/users/{userId}/devices/visible` | 用户 token | 指定用户可见设备列表 |
| `GET` | `/api/users/{userId}/device-groups` | 用户 token | 设备分组列表和成员关系 |
| `POST` | `/api/users/{userId}/device-groups` | 用户 token | 新建设备分组；分组名不能重复 |
| `PATCH` | `/api/users/{userId}/device-groups/{groupId}` | 用户 token | 修改设备分组名称 |
| `DELETE` | `/api/users/{userId}/device-groups/{groupId}` | 用户 token | 删除设备分组 |
| `PUT` | `/api/users/{userId}/devices/{deviceId}/groups` | 用户 token | 设置设备所属分组；一个设备可属于多个组 |
| `POST` | `/api/device-invites` | 用户 token | 创建设备邀请 |
| `GET` | `/api/device-invites` | 用户 token/query | 查询设备邀请 |
| `POST` | `/api/device-invites/accept` | 用户 token | 接受设备邀请 |
| `GET` | `/api/users/{userId}/device-invites` | 用户 token | 指定用户设备邀请 |
| `POST` | `/api/web/device-bootstrap-keys` | 用户 token | 生成安装/接入 key |
| `GET` | `/api/web/device-bootstrap-keys` | 用户 token/query | 查询接入 key |
| `POST` | `/api/web/device-bootstrap-keys/{keyId}/revoke` | 用户 token | 撤销接入 key |
| `GET` | `/api/users/{userId}/device-bootstrap-keys` | 用户 token | 查询指定用户接入 key |

### Networks

| Method | Path | Auth | 备注 |
| --- | --- | --- | --- |
| `GET` | `/api/networks` | 用户 token/query | 网络列表 |
| `POST` | `/api/networks` | 用户 token | 创建网络 |
| `PATCH` | `/api/networks/{networkId}` | 用户 token | 修改网络名称、组内连通策略等网络属性 |
| `GET` | `/api/users/{userId}/networks` | 用户 token | 指定用户网络列表 |
| `GET` | `/api/networks/{networkId}/devices` | 用户 token | 网络内设备列表 |
| `POST` | `/api/networks/{networkId}/devices` | 用户 token | 添加设备到网络 |
| `PATCH` | `/api/networks/{networkId}/devices/{deviceId}` | 用户 token | 修改网络内设备属性，如别名/IP |
| `DELETE` | `/api/networks/{networkId}/devices/{deviceId}` | 用户 token | 从网络移除设备 |

### DNS And Public Mappings

| Method | Path | Auth | 备注 |
| --- | --- | --- | --- |
| `GET` | `/api/networks/{networkId}/dns/zones` | 用户 token | 内网域名 Zone 列表 |
| `POST` | `/api/networks/{networkId}/dns/zones` | 用户 token | 创建内网域名 Zone |
| `PATCH` | `/api/networks/{networkId}/dns/zones/{zoneId}` | 用户 token | 修改 Zone |
| `DELETE` | `/api/networks/{networkId}/dns/zones/{zoneId}` | 用户 token | 删除 Zone |
| `GET` | `/api/networks/{networkId}/dns/records` | 用户 token | DNS 记录列表 |
| `POST` | `/api/networks/{networkId}/dns/records` | 用户 token | 新增 DNS 记录 |
| `PATCH` | `/api/networks/{networkId}/dns/records/{recordId}` | 用户 token | 修改 DNS 记录 |
| `DELETE` | `/api/networks/{networkId}/dns/records/{recordId}` | 用户 token | 删除 DNS 记录 |
| `GET` | `/api/networks/{networkId}/public-mappings` | 用户 token | 公网映射列表 |
| `POST` | `/api/networks/{networkId}/public-mappings` | 用户 token | 创建公网映射 |
| `PATCH` | `/api/networks/{networkId}/public-mappings/{mappingId}` | 用户 token | 修改公网映射 |
| `DELETE` | `/api/networks/{networkId}/public-mappings/{mappingId}` | 用户 token | 删除公网映射 |

### Security Groups

| Method | Path | Auth | 备注 |
| --- | --- | --- | --- |
| `GET` | `/api/networks/{networkId}/security-groups` | 用户 token | 安全组列表 |
| `POST` | `/api/networks/{networkId}/security-groups` | 用户 token | 创建安全组；创建/改名不推送客户端 |
| `PATCH` | `/api/networks/{networkId}/security-groups/{securityGroupId}` | 用户 token | 修改安全组名称等元数据 |
| `DELETE` | `/api/networks/{networkId}/security-groups/{securityGroupId}` | 用户 token | 删除安全组 |
| `GET` | `/api/security-groups/{securityGroupId}/rules` | 用户 token | 安全组规则列表 |
| `POST` | `/api/security-groups/{securityGroupId}/rules` | 用户 token | 新增规则；规则变化会推送客户端网络配置 |
| `PATCH` | `/api/security-groups/rules/{ruleId}` | 用户 token | 修改规则；规则变化会推送客户端网络配置 |
| `DELETE` | `/api/security-groups/rules/{ruleId}` | 用户 token | 删除规则；规则变化会推送客户端网络配置 |

## Ops API

Ops API 服务运维/运营控制台。它不注册 Web Console 普通业务接口、App 设备 session、MQTT webhook、内部 wire 路由。

### Auth And Audit

| Method | Path | Auth | 备注 |
| --- | --- | --- | --- |
| `POST` | `/api/ops/auth/login` | 无 | 运维账号登录 |
| `PATCH` | `/api/ops/auth/password` | Operator token | 修改当前运维账号密码 |
| `GET` | `/api/ops/dashboard` | Operator token | 运维概览数据 |
| `GET` | `/api/ops/audit-events` | Operator token | 审计日志列表，支持查询参数过滤 |

### Operators

| Method | Path | Auth | 备注 |
| --- | --- | --- | --- |
| `GET` | `/api/ops/operators` | Operator token | 运维账号列表 |
| `POST` | `/api/ops/operators` | Operator token | 创建运维账号 |
| `PATCH` | `/api/ops/operators/{operatorId}` | Operator token | 修改运维账号状态、角色等 |
| `POST` | `/api/ops/operators/{operatorId}/password` | Operator token | 重置指定运维账号密码 |

### Nodes

| Method | Path | Auth | 备注 |
| --- | --- | --- | --- |
| `GET` | `/api/ops/relay-nodes` | Operator token | Relay 节点列表 |
| `POST` | `/api/ops/relay-nodes` | Operator token | 新增 Relay 节点 |
| `PATCH` | `/api/ops/relay-nodes/{nodeId}` | Operator token | 修改 Relay 节点配置 |
| `PATCH` | `/api/ops/relay-nodes/{nodeId}/status` | Operator token | 修改 Relay 节点状态 |
| `DELETE` | `/api/ops/relay-nodes/{nodeId}` | Operator token | 删除 Relay 节点 |
| `GET` | `/api/ops/punch-nodes` | Operator token | Punch 节点列表 |
| `POST` | `/api/ops/punch-nodes` | Operator token | 新增 Punch 节点 |
| `PATCH` | `/api/ops/punch-nodes/{nodeId}` | Operator token | 修改 Punch 节点配置 |
| `PATCH` | `/api/ops/punch-nodes/{nodeId}/status` | Operator token | 修改 Punch 节点状态 |
| `DELETE` | `/api/ops/punch-nodes/{nodeId}` | Operator token | 删除 Punch 节点 |

### Customers And Devices

| Method | Path | Auth | 备注 |
| --- | --- | --- | --- |
| `GET` | `/api/ops/customers` | Operator token | 客户列表 |
| `PATCH` | `/api/ops/customers/{customerId}` | Operator token | 修改客户状态、名称等 |
| `POST` | `/api/ops/customers/{customerId}/assign-plan` | Operator token | 给客户分配套餐 |
| `GET` | `/api/ops/devices` | Operator token | 全局设备列表 |
| `PATCH` | `/api/ops/devices/{deviceId}` | Operator token | 运维侧修改设备状态等 |
| `DELETE` | `/api/ops/devices/{deviceId}` | Operator token | 运维侧删除设备 |

### Plans Products Orders Renewals

| Method | Path | Auth | 备注 |
| --- | --- | --- | --- |
| `GET` | `/api/ops/plans` | Operator token | 套餐列表 |
| `POST` | `/api/ops/plans` | Operator token | 创建/更新套餐 |
| `PATCH` | `/api/ops/plans/{planCode}` | Operator token | 修改套餐 |
| `GET` | `/api/ops/products` | Operator token | 商品列表 |
| `POST` | `/api/ops/products` | Operator token | 创建商品 |
| `PATCH` | `/api/ops/products/{productId}` | Operator token | 修改商品 |
| `GET` | `/api/ops/orders` | Operator token | 订单列表 |
| `POST` | `/api/ops/orders` | Operator token | 创建订单 |
| `PATCH` | `/api/ops/orders/{orderId}` | Operator token | 修改订单 |
| `GET` | `/api/ops/renewals` | Operator token | 续费记录列表 |
| `PATCH` | `/api/ops/renewals/{renewalId}` | Operator token | 修改续费记录 |

### Client Downloads

| Method | Path | Auth | 备注 |
| --- | --- | --- | --- |
| `GET` | `/api/ops/client-downloads` | Operator token | 管理客户端下载记录 |
| `POST` | `/api/ops/client-downloads` | Operator token | 上传/登记客户端安装包 |
| `DELETE` | `/api/ops/client-downloads/{downloadId}` | Operator token | 删除客户端安装包记录 |

## Isolation rules

- App API should not expose `/api/ops/*`.
- Web Console API should not expose `/api/ops/*`, `/api/device/session/*`, `/mqtt/*`, or `/internal/wire/*`.
- Ops API should not expose Web Console APIs, App device session APIs, `/mqtt/*`, or `/internal/wire/*`.
