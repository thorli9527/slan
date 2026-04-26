# 小规模上线检查清单

## Maintained Main-Flow Status

Before rollout, the repository-level main flow should be treated as available
when these checks pass:

- protocol contract checks, including `http-routes`
- `server/server-biz` unit tests
- `server/tests/go/server-biz-test` HTTP flow tests
- `server/tests/rust/app-core-tests`
- Flutter app tests
- web console build

The uncovered items near the end of this document are rollout hardening tasks,
not missing create/join/switch/bootstrap APIs.

本文档面向当前仓库的可运行形态，目标是把 `Mac 客户端 + server-biz + server-relay` 推到“小规模上线可控”的状态。

这里不讨论大规模集群化、复杂高可用和正式多环境发布平台，只关注当前代码已经具备、并且已经在本地 Docker 栈里验证过的最小闭环。

## 当前已验证通过

- [x] `server-biz` Docker 化
- [x] `server-relay` Docker 化
- [x] `docker-compose.local.yml` 可拉起：
  - `postgres`
  - `redis`
  - `server-biz`
  - `server-relay`
  - `caddy`
- [x] `server-biz` public / ops 健康检查可用
- [x] `caddy` 本地 `https/wss` 代理可用
- [x] 默认管理员自动 seed
- [x] 默认管理员登录、改密、告警清零可用
- [x] 控制面主链路可用：
  - register
  - refresh token
  - device register
  - device list
  - network create
  - join by owner email
  - join by key
  - device alias via attachment remark
  - switch network
  - activate / deactivate network
  - node register
  - control session
  - bootstrap
  - relay ticket
- [x] relay 数据面最小链路可用：
  - attach
  - forward
  - packet delivery
  - detach

## P0 必做

### 1. 替换默认密钥和默认密码

- [ ] 替换 `POSTGRES_PASSWORD`
- [ ] 替换 `SLAN_OPS_ACCESS_TOKEN`
- [ ] 替换 `SLAN_OPS_DEFAULT_ADMIN_PASSWORD`
- [ ] 替换 `SLAN_RELAY_TICKET_SIGNING_SECRET`
- [ ] 首次启动后立即修改默认管理员密码
- [ ] 确认 `/overview.securityWarnings` 为空

### 2. 换成真实公网配置

- [ ] `SLAN_HTTP_PUBLIC_HOST` 使用真实域名
- [ ] `SLAN_HTTP_PUBLIC_SCHEME` 设为 `https`
- [ ] `server-biz/config.docker.yaml` 中 relay 节点地址换成真实可达地址
- [ ] STUN 地址换成你自己的可控地址，至少不要长期依赖默认公共地址

### 3. 部署层 TLS

- [ ] `caddy` 仅作为本地 / 小规模入口示例
- [ ] 生产环境必须使用真实证书和真实域名
- [ ] MQTT broker 必须通过受控端口暴露并启用鉴权
- [ ] `ops` 面不要直接暴露公网，至少放到内网或白名单入口后

### 4. 数据持久化

- [ ] PostgreSQL volume 持久化已启用，确认生产路径和备份策略
- [ ] Redis 的角色要明确：
  - 仅缓存
  - 还是必须保留登录态 / token
- [ ] 至少有一套数据库导出和恢复流程

### 5. 运行时验证

- [ ] 真实域名下再次验证：
  - `/healthz`
  - `/login`
  - `/overview`
- [ ] 再跑一次控制面 smoke
- [ ] 再跑一次 relay 数据面 smoke

## P1 强烈建议

### 1. 运维面

- [ ] 给 `server-biz` 增加最少监控面板：
  - HTTP 健康
  - Redis / PostgreSQL 可用性
  - `control_mqtt` 最近错误和热点 peer
- [ ] 收集容器日志
- [ ] 明确崩溃后自动重启策略

### 2. 安全面

- [ ] 默认管理员首登强制改密
- [ ] 管理员密码策略至少包含长度下限
- [ ] 生产环境禁用静态 `ops.access_token` 对公网开放
- [ ] 管理员操作审计要落日志

### 3. Mac 客户端

- [ ] release 配置不再指向本地 mock/dev 默认地址
- [ ] 完成正式签名
- [ ] 完成 notarization
- [ ] 使用真实控制面和 relay 地址联调

## 建议的上线顺序

1. 准备生产环境变量和真实域名
2. 启动 PostgreSQL / Redis / server-biz / server-relay / TLS 入口
3. 登录默认管理员并立即改密
4. 检查 `/overview`
5. 跑 `scripts/local_stack_smoke.sh` 对应的生产环境版本 smoke
6. 跑 `scripts/local_relay_e2e.go` 对应的生产环境版本 smoke
7. 再接入 Mac 客户端做真实链路联调

## 当前仍未覆盖的事项

- 多节点 relay 集群
- 正式证书自动化续期
- 多环境配置编排
- 大规模用户 / 设备并发压测
- 数据迁移回滚流程
