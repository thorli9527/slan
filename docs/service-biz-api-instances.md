# Service Biz API Instances

`service-biz` 运行统一业务 API 实例和独立 Ops 实例。原 App/Web 路由已合并到统一业务实例，并提供 `/api/app` 主路径。`internal/api/management` 目前仅是管理 Handler 的内部实现目录，不再发布 `旧 Web API 前缀` 路径或对应独立服务：

- App API: `server/service-biz/internal/biz/app_api_routes.go`
- Web Console API: `server/service-biz/internal/biz/web_console_api_routes.go`
- Ops API: `server/service-biz/internal/biz/ops_server.go`
- MQTT webhook: `server/service-biz/internal/biz/mqtt_api_routes.go`
- Shared route helper: `server/service-biz/internal/biz/api_routes.go`

| 实例 | Route set | 默认端口 | 调用方 | 备注 |
| --- | --- | --- | --- | --- |
| Unified API | `all` | `28080` | 桌面端、移动端、运营资源管理、wire/relay/derp、BifroMQ | `/api/app` 为主入口，仅兼容 `/api/app` |
| Ops API | `ops` | `28082` | 运维/运营控制台 `opt-ui` | 只暴露 `/api/ops/*` 管理接口 |

## Shared

| Method | Path | Auth | 备注 |
| --- | --- | --- | --- |
| `GET` | `/healthz` | 无 | 健康检查；部署脚本用它确认业务和 Ops 实例可用 |

## Unified Business API

统一业务 API 服务桌面端、移动端和业务资源管理。它同时注册 `/internal/wire/*` 和 `/mqtt/*`，并默认启动 MQTT control subscriber / delivery retry worker。

### Auth And Device Session

| Method | Path | Auth | 备注 |
| --- | --- | --- | --- |
| `POST` | `/api/app/auth/login` | 无 | 桌面/移动端账号密码登录；可携带本机 `deviceId` 绑定登录上下文 |
| `POST` | `/api/app/auth/renew` | 用户 token | 刷新用户会话；客户端重新拉取登录态时使用 |
| `POST` | `/api/app/auth/logout` | 用户 token | 用户退出登录；可同时传入 device token 清理设备会话 |
| `POST` | `/api/app/auth/console-login-keys` | 用户 token | App 生成 Web Console 快捷登录 key；不要放到 Ops 实例 |
| `POST` | `/api/app/auth/device-login-devices` | 无 | App 发起设备登录准备流程；Web Console 也需要该接口来配合确认 |
| `GET` | `/api/app/devices` | 可选用户 token/query | App 客户端查询当前用户可见设备列表 |
| `POST` | `/api/app/devices/register` | 用户 token | App 设备注册兼容入口；当前客户端仍可通过该入口补注册设备 |
| `POST` | `/api/app/devices/{deviceId}/renew` | 用户 token | App 设备续期兼容入口；用于兼容仍未切到 device session 的注册流程 |
| `POST` | `/api/app/device/session/bootstrap` | session key | 通过安装/接入码创建设备 session |
| `POST` | `/api/app/device/session/bind` | 用户 token | 将本机设备 session 绑定到当前用户 |
| `POST` | `/api/app/device/session/renew` | 设备 token | 设备心跳续期，同时上报网络启用状态和流量统计 |

### Network Runtime

| Method | Path | Auth | 备注 |
| --- | --- | --- | --- |
| `GET` | `/api/app/devices/{deviceId}/network-configs` | 用户/设备 token | 返回设备参与的网络配置集合；客户端合并/覆盖规则从这里开始 |
| `GET` | `/api/app/devices/{deviceId}/mqtt-credential` | 用户/设备 token | 获取设备 MQTT 凭据；客户端用它订阅控制消息 |
| `GET` | `/api/app/networks/{networkId}/network-config?deviceId=...` | 用户/设备 token | 获取某网络对某设备的最终运行配置 |
| `GET` | `/api/app/networks/{networkId}/relay-candidates?deviceId=...` | 用户/设备 token | 获取当前网络的 relay 候选节点 |
| `POST` | `/api/app/networks/{networkId}/relay-candidates` | 设备签名/token | 设备上报或请求 relay 候选；主要用于路径选择 |
| `POST` | `/api/app/networks/{networkId}/punch/connect-sessions` | 设备签名 header | 创建 P2P punch 协商会话；请求头包含设备/MQTT 签名 |
| `POST` | `/api/app/relay/tickets` | 用户/设备 token | 获取 relay ticket；wire/relay 数据面鉴权使用 |

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

## Management Compatibility Routes

原 Web 管理接口现由统一业务 API 的 `/api/app` 提供。`旧 Web API 前缀` 路径、独立 Web Console 服务和 UI 均已删除。

用户创建以及网络、设备分组、DNS 和安全组管理只通过 Ops API。统一业务 API 仅提供用户登录、设备运行时和只读配置接口。

### Auth

| Method | Path | Auth | 备注 |
| --- | --- | --- | --- |
| `POST` | `/api/app/auth/login` | 无 | 用户名密码登录 |
| `POST` | `/api/app/auth/renew` | 用户 token | 刷新用户会话 |
| `POST` | `/api/app/auth/logout` | 用户 token | 退出登录 |
| `POST` | `/api/app/auth/console-login-keys` | 用户 token | 旧客户端控制台快捷登录兼容接口 |
| `POST` | `/api/app/auth/console-login` | login key | 旧客户端控制台快捷登录兼容接口 |
| `POST` | `/api/app/auth/device-login-devices` | 无 | 旧浏览器登录准备流程兼容接口 |
| `POST` | `/api/app/auth/device-login-devices/{deviceId}/complete` | 用户 token | 旧浏览器登录完成流程兼容接口 |

### Users And Aliases

| Method | Path | Auth | 备注 |
| --- | --- | --- | --- |
| `GET` | `/api/users` | 用户 token | 管理可见用户列表；当前主要用于控制台初始化 |
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
| `POST` | `/api/app/device-bootstrap-keys` | 用户 token | 生成安装/接入 key |
| `GET` | `/api/app/device-bootstrap-keys` | 用户 token/query | 查询接入 key |
| `POST` | `/api/app/device-bootstrap-keys/{keyId}/revoke` | 用户 token | 撤销接入 key |
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

Ops API 服务运维/运营控制台。它不注册客户端设备 session、MQTT webhook 或内部 wire 路由。

除 `POST /api/ops/auth/login` 外，所有 Ops 路由必须携带有效 Operator Bearer token；服务端校验会话有效期和运营账号 active 状态。资源写操作成功后统一记录 operator、动作、资源类型和资源 ID 到审计事件表。

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

### Users And Devices

| Method | Path | Auth | 备注 |
| --- | --- | --- | --- |
| `GET` | `/api/ops/users` | Operator token | 全局用户列表 |
| `POST` | `/api/ops/users` | Operator token | 新增用户 |
| `PATCH` | `/api/ops/users/{userId}` | Operator token | 修改用户状态、名称等 |
| `PATCH` | `/api/ops/users/{userId}/password` | Operator token | 设置用户密码 |
| `GET` | `/api/ops/devices` | Operator token | 全局设备列表 |
| `PATCH` | `/api/ops/devices/{deviceId}` | Operator token | 运维侧修改设备状态等 |
| `DELETE` | `/api/ops/devices/{deviceId}` | Operator token | 运维侧删除设备 |

### Global Networks And Notifications

| Method | Path | Auth | 备注 |
| --- | --- | --- | --- |
| `GET` | `/api/ops/networks` | Operator token | 全局网络列表 |
| `GET` | `/api/ops/networks/{networkId}/devices` | Operator token | 网络设备列表 |
| `POST` | `/api/ops/networks/{networkId}/devices/{deviceId}` | Operator token | 单设备加入网络并通知网络成员及目标客户端 |
| `DELETE` | `/api/ops/networks/{networkId}/devices/{deviceId}` | Operator token | 单设备离开网络并通知网络成员及目标客户端 |
| `POST/PATCH/DELETE` | `/api/ops/networks/{networkId}/dns/*`、`/api/ops/dns/*` | Operator token | DNS 变更并广播 resolver、版本和全量快照事件 |
| `POST/PATCH/DELETE` | `/api/ops/networks/{networkId}/security-groups/*`、`/api/ops/security-*` | Operator token | 安全组变更并广播 ACL、版本和全量快照事件 |

单设备入网/离网会更新网络配置版本，向网络广播 `member_added` 或 `member_removed` 和 `network_snapshot`，并向目标设备发布 `device_network_membership_changed`。重复加入已存在的有效成员不会重复推送。离网使用持久化的 `excluded` 成员来源标记，防止设备被后续分组同步自动重新加入；再次由运营端加入时才恢复 active 状态。数据库启动迁移会为旧成员回填 `direct` 或 `device_group` 来源。

用户套餐、商品、订单、续费管理和客户端发布模块已退役，不再注册相关路由。服务启动迁移会幂等删除对应历史表；客户端安装包由外部制品渠道交付，业务服务不再提供上传、存储或下载接口。

## Isolation rules

- Unified business API must not expose `/api/ops/*`.
- Ops API must not expose business session APIs, `/mqtt/*`, or `/internal/wire/*`.
