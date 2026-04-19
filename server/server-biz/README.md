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
  - `types_business.go`：业务 DTO
  - `types_system.go`：系统/运维 DTO
- `api/http`
  - `routes_business.go`：业务 HTTP 接口
  - `routes_ops.go`：运营管理接口
  - `control_ws*.go`：控制通道相关实现
- `internal/service`
  - `interfaces_business.go`：业务服务聚合
  - `interfaces_system.go`：token / control / ops 等系统接口

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

启动：

```bash
docker compose -f docker-compose.local.yml up --build server-biz
```

默认端口：

- `server-biz public`: `18080`
- `server-biz ops`: `18081`
- `postgres`: `15432`
- `redis`: `16379`

相关文件：

- `Dockerfile`
- `configs/config.docker.yaml`
- `../../docker-compose.local.yml`

说明：

- `server-biz` 容器会在启动时自动做 PostgreSQL `AutoMigrate`
- `config.docker.yaml` 里的 relay 地址先保留为本地占位地址
- `Dockerfile` 仍支持通过 `APP_TARGET` 构建别的 `cmd/*` 入口，但当前 compose 只接入了可编译、可运行的 `biz-server`
- `server-relay` 当前还没有独立可执行 binary，所以这次 compose 没有把它伪装成可运行服务

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

- `GET /overview`
- `GET /users`
- `GET /devices`
- `GET /relays`
- `GET /admins`
- `POST /admins`
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

示例：

```bash
curl -s \
  -H 'Authorization: Bearer docker-ops-token' \
  http://127.0.0.1:18081/overview
```
