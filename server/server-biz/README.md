# server-biz

Phase 1 MVP 的 Go 控制面服务。

## 当前职责

- 用户注册与登录
- 设备注册
- 网络创建、加入与成员管理
- 子网与虚拟 IP 分配
- 控制通道 bootstrap 配置下发
- relay 票据签发
- 控制通道 WebSocket、路径健康与 connect-plan
- 内嵌 `/ops` 运营管理入口

## 目录说明

- `cmd/biz-server`：进程入口
- `api/http`：HTTP 路由层
- `api/dto`：请求响应模型
- `internal/*`：领域服务边界与实现

当前代码结构已经按边界做过一轮收口：

- `api/dto`
  - `types_business_access.go`：注册/登录 DTO
  - `types_business_registration.go`：设备/节点注册 DTO
  - `types_business_network.go`：网络/子网/挂载 DTO
  - `types_business_control.go`：bootstrap / control / relay DTO
  - `types_system.go`：系统/运维 DTO
- `api/http`
  - `routes_business*.go`：客户业务 HTTP 接口
  - `routes_ops.go`：运营管理接口
  - `routes_common.go`：HTTP 公共鉴权与错误处理
  - `control_ws*.go`：控制通道相关实现
- `internal/service`
  - `access.go`：身份与 token 接口
  - `registration.go`：设备/节点注册接口
  - `network.go`：网络编排与 IPAM 接口
  - `control.go`：bootstrap / control channel / control sync 接口
  - `ops.go`：运营管理接口
  - `impl/*`：各接口的数据库实现

## 文档

- [文档索引](./docs/README.md)
- [内部需求](./docs/internal-requirements.md)
- [对外输出功能](./docs/exported-capabilities.md)
- [对外接入接口](./docs/integration-interfaces.md)

## 本地 Docker 运行

仓库根目录提供了 `docker-compose.local.yml`，可直接拉起：

- `postgres`
- `redis`
- `server-biz`
- `server-relay`
- `caddy`

启动：

```bash
docker compose -f docker-compose.local.yml up --build server-biz
```

默认端口：

- `server-biz public (direct debug)`: `28080` by default, configurable via `SLAN_BIZ_PUBLIC_PORT`
- `server-biz ops`: `28081` by default, configurable via `SLAN_BIZ_OPS_PORT`
- `public https / wss`: `18443`
- `server-relay udp`: `19000/udp`
- `postgres`: `15432`
- `redis`: `16379`

MQTT/BifroMQ notes:

- MQTT is disabled by default in local compose (`SLAN_MQTT_ENABLED=false`).
- Local compose starts BifroMQ as the MQTT broker. Port `1883` is exposed by
  default and can be changed with `BIFROMQ_MQTT_PORT`.
- When enabling SLAN MQTT, provide a BifroMQ endpoint reachable from
  `server-biz` as `bifromq:1883`, or override `SLAN_MQTT_BROKER_URL`.
- `SLAN_MQTT_PUBLIC_BROKER_URL` is the broker URL returned to the desktop app.
- Local BifroMQ uses the built-in WebHook demo Auth Provider against
  `server-biz` endpoints under `/mqtt/bifromq/*`. This validates SLAN-generated
  credentials and enforces device/server topic permissions for the local stack.
  For production, replace the demo provider with a dedicated BifroMQ Auth
  Provider plugin that calls `POST /mqtt/bifromq/auth` for authentication and
  `POST /mqtt/bifromq/check` for authorization.
  A successful auth check marks only `controlReachable=true`; virtual network
  online state still comes from the client heartbeat after the local tunnel is
  up.
- When MQTT is enabled, `server-biz` subscribes to
  `{topic_prefix}/{deviceId}/networks/{networkId}/state` and persists the same
  `DeviceNetworkState` record as the HTTP fallback endpoint.
- To verify the local BifroMQ path, start compose with MQTT enabled:
  `SLAN_MQTT_ENABLED=true docker compose -f docker-compose.local.yml up --build -d`
  on Unix shells, or
  `$env:SLAN_MQTT_ENABLED='true'; docker compose -f docker-compose.local.yml up --build -d`
  in PowerShell,
  and then run:
  `powershell -ExecutionPolicy Bypass -File scripts/verify-local-server.ps1 -ExpectMqttCredential -VerifyMqttBroker`.

相关文件：

- `Dockerfile`
- `configs/config.docker.yaml`
- `../../docker-compose.local.yml`

说明：

- `server-biz` 容器会在启动时自动做 PostgreSQL `AutoMigrate`
- `config.docker.yaml` 里的 relay 地址会返回宿主机公开端口 `127.0.0.1:19000`
- `caddy` 提供本地 `https://slan.localhost:18443` 和 `wss://slan.localhost:18443/control/ws`
- compose 使用环境变量覆盖敏感配置，建议先复制根目录 `.env.local.example` 再启动

## 运营入口

`server-biz` 当前拆成两个独立 HTTP 实例：

- public：对外客户接口 + control WS
- ops：运营管理接口

不再依赖独立 `server-ops` 项目。

鉴权方式：

- 使用配置项 `ops.access_token`
- 访问时传 `Authorization: Bearer <ops token>`
- 不复用普通业务用户的 access token

当前已提供的接口：

- `POST /login`
- `GET /overview`
- `GET /users`
- `GET /devices`
- `GET /relays`
- `GET /admins`
- `POST /admins`
- `PUT /admins/:adminId/password`
- `POST /admins/:adminId/unlock`
- `GET /roles`
- `POST /roles`
- `PUT /users/:userId/roles`
- `GET /menus`
- `POST /menus`
- `PUT /roles/:roleId/menus`

这套接口当前覆盖：

- 管理员信息 `admin_info`
- 角色 `roles`
- 用户绑定角色 `user_roles`
- 功能菜单 `menus`
- 角色绑定菜单 `role_menus`

动态管理员 token 访问 `/ops/*` 时，会继续按菜单码做路由鉴权：

- `ops.overview`
- `ops.users`
- `ops.devices`
- `ops.admins`
- `ops.roles`
- `ops.menus`
- `ops.relays`

静态 `ops.access_token` 仍保留全量访问能力，适合内网运维和应急使用。

服务启动时会自动写入一组内置 RBAC 数据：

- 菜单码：
  - `ops.overview`
  - `ops.users`
  - `ops.devices`
  - `ops.admins`
  - `ops.roles`
  - `ops.menus`
  - `ops.relays`
- 内置角色：
  - `ops-super-admin`
  - `ops-observer`

同时会按配置自动灌入一个默认管理员：

- 配置位置：`ops.default_admin`
- 默认登录名：`admin`
- 默认角色：`ops-super-admin`
- 默认密码来自配置文件，应在生产环境显式替换

默认管理员只会在首次缺失时创建，不会在每次启动时覆盖你后来手工修改的密码和角色。

如果默认管理员仍在使用 seed 密码：

- `GET /overview` 会在 `securityWarnings` 返回告警
- `GET /overview` 还会直接返回：
  - `defaultAdminSeeded`
  - `defaultAdminLoginName`
  - `defaultAdminRoleBound`
- `GET /admins` 会对对应管理员标记 `usingSeedPassword=true`

示例：

管理员登录：

```bash
curl -s \
  -X POST \
  -H 'Content-Type: application/json' \
  -d '{"loginName":"admin","password":"change-me-admin-password"}' \
  http://127.0.0.1:28081/login
```

静态 token 访问：

```bash
curl -s \
  -H 'Authorization: Bearer docker-ops-token' \
  http://127.0.0.1:28081/overview
```
