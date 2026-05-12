# Client Web 数据库与接口对齐

本文档按当前客户 Web UI 收敛 `service-biz-new` 的数据模型和接口。术语以 UI 为准：

- 设备：当前用户可管理或可访问的设备。
- 生成接入码：生成一次性设备授权码，让设备 owner 确认后授权给当前用户访问。
- 网络：一组设备和访问策略。
- 内网域名：网络内 DNS Zone 与解析记录。
- 公网访问：把某台设备的某个端口发布成公网域名。
- 安全组：网络级访问控制，默认作用于网络内全部设备。

## 保留模型

### users

注册和登录主体。注册成功后自动创建默认网络。

关键字段：`email`、`password_hash`、`display_name`、`status`。

### user_aliases

当前用户给可见用户邮箱设置显示别名。其它界面展示用户时优先使用别名。

唯一约束：`owner_user_id + email`。

### devices

设备注册后直接绑定 owner，并分配全局 IP。

关键字段：`owner_user_id`、`device_id`、`platform`、`os_version`、`alias`、`global_ip`、`status`。

### device_invites

设备访问授权邀请码。邀请码不绑定网络，只表达“邀请方想获得某台设备的访问权”。

约束：

- 邀请码 32 位。
- 30 分钟有效。
- 只能使用一次。
- 推荐 Redis 存储，服务内存保存展示记录。

### device_access_grants

设备 owner 确认邀请码后生成授权关系，使邀请方可以在“设备”列表看到该设备，并可把它加入自己的网络。

唯一约束：`device_id + user_id`。

### networks

UI 中的“网络”。同一 owner 下 `code` 不可重复。

### network_devices

网络和设备的一对多关系。同一网络不能重复添加同一设备。

### network_dns_zones / network_dns_records

内网域名和解析记录。解析值绑定设备和端口。

### public_domain_mappings

公网访问映射，格式为：

`{alias}.{networkCode}.{userSlug}.pub.staticlss.com`

绑定来源设备、协议和端口。

### security_groups / security_group_rules

安全组属于网络。当前阶段默认作用于网络内全部设备，不做设备级安全组绑定。

入方向对象：来源设备 / 用户 / 网络 / CIDR / 全部。

出方向对象：目标设备 / 网络 / CIDR / 域名 / 全部。

## 删除或不再暴露

- `network_members`
- `network_invites`
- `security_group_devices`
- 网络成员相关 API
- 网络邀请码相关 API
- 独立 IPAM 管理 UI API
- 全局 DNS 管理 UI API

IPAM 仍作为内部能力存在：注册设备时自动分配全局 IP，UI 不单独管理。

## 当前 UI API

### Auth

- `POST /api/auth/register`
- `POST /api/auth/login`
- `PATCH /api/users/{userId}/password`

### 用户别名

- `GET /api/user-aliases?ownerUserId={userId}`
- `PATCH /api/user-aliases`

### 设备

- `GET /api/devices/visible?userId={userId}`
- `POST /api/devices/register`
- `PATCH /api/devices/{deviceId}`

### 接入码

- `POST /api/device-invites`
- `GET /api/device-invites?userId={userId}`
- `POST /api/device-invites/accept`

### 网络

- `GET /api/networks?userId={userId}`
- `POST /api/networks`
- `PATCH /api/networks/{networkId}`

### 网络设备

- `GET /api/networks/{networkId}/devices`
- `POST /api/networks/{networkId}/devices`
- `PATCH /api/networks/{networkId}/devices/{deviceId}`
- `DELETE /api/networks/{networkId}/devices/{deviceId}`

### 内网域名

- `GET /api/networks/{networkId}/dns/zones`
- `POST /api/networks/{networkId}/dns/zones`
- `PATCH /api/networks/{networkId}/dns/zones/{zoneId}`
- `DELETE /api/networks/{networkId}/dns/zones/{zoneId}`
- `GET /api/networks/{networkId}/dns/records`
- `POST /api/networks/{networkId}/dns/records`
- `PATCH /api/networks/{networkId}/dns/records/{recordId}`
- `DELETE /api/networks/{networkId}/dns/records/{recordId}`

### 公网访问

- `GET /api/networks/{networkId}/public-mappings`
- `POST /api/networks/{networkId}/public-mappings`
- `PATCH /api/networks/{networkId}/public-mappings/{mappingId}`
- `DELETE /api/networks/{networkId}/public-mappings/{mappingId}`

### 安全组

- `GET /api/networks/{networkId}/security-groups`
- `POST /api/networks/{networkId}/security-groups`
- `DELETE /api/networks/{networkId}/security-groups/{securityGroupId}`
- `GET /api/security-groups/{securityGroupId}/rules`
- `POST /api/security-groups/{securityGroupId}/rules`
- `PATCH /api/security-groups/rules/{ruleId}`
- `DELETE /api/security-groups/rules/{ruleId}`

### 网络配置

- `GET /api/networks/{networkId}/network-config?deviceId={deviceId}`

客户端启用网络时读取最终配置。
