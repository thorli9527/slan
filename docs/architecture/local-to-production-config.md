# local 到 production 配置迁移说明

本文档只描述当前仓库里已经存在的配置项，帮助把本地 `docker-compose.local.yml` 迁到小规模生产部署。

## 本地配置入口

当前本地运行主要依赖：

- [docker-compose.local.yml](/Users/thorli/workspace/slan/docker-compose.local.yml)
- [\.env.local.example](/Users/thorli/workspace/slan/.env.local.example)
- [server/server-biz/configs/config.docker.yaml](/Users/thorli/workspace/slan/server/server-biz/configs/config.docker.yaml)
- [server/server-relay/configs/relay-daemon.example.json](/Users/thorli/workspace/slan/server/server-relay/configs/relay-daemon.example.json)
- [deploy/local/Caddyfile](/Users/thorli/workspace/slan/deploy/local/Caddyfile)

## 必须替换的值

### server-biz

- `POSTGRES_PASSWORD`
  本地默认值不能直接上线。

- `SLAN_OPS_ACCESS_TOKEN`
  仅作为应急运维旁路使用，生产必须换成高强度随机值。

- `SLAN_OPS_DEFAULT_ADMIN_PASSWORD`
  仅用于首启 seed，生产必须更换，并在首登后立即改密。

- `SLAN_RELAY_TICKET_SIGNING_SECRET`
  必须改成独立高强度随机值。

- `SLAN_HTTP_PUBLIC_HOST`
  本地是 `slan.localhost:18443`，生产必须改成真实域名。

- `SLAN_HTTP_PUBLIC_SCHEME`
  生产固定为 `https`。

- `SLAN_BIZ_PUBLIC_PORT`
- `SLAN_BIZ_OPS_PORT`
  这两个只影响本地宿主机调试映射。生产可以不暴露宿主机端口，而走反向代理或容器网络。

### server-relay

- `udp_bind`
  本地可以是 `0.0.0.0:9000`，生产需要确认实际监听端口和安全组。

- `relay_url_prefix`
  当前是 `udp://`，如果后续有其他传输协议，需要和控制面返回值保持一致。

- `ticket_signing_secret`
  必须与 `server-biz` 的 relay ticket 签名密钥保持一致。

## 本地值与生产值对照

### public host

- local:
  - `slan.localhost:18443`
- production:
  - 例如 `control.example.com`

### public scheme

- local:
  - `https`
- production:
  - `https`

### relay UDP

- local:
  - `127.0.0.1:19000`
- production:
  - 真实公网 `ip:port`
  - 或真实域名解析到公网入口

### ops 入口

- local:
  - `127.0.0.1:28081`
  - `ops.slan.localhost:18443`
- production:
  - 建议只保留内网访问
  - 或放到受限反向代理后

## 建议的生产部署差异

### 1. 反向代理

本地使用 `caddy` 只为快速验证：

- `https://slan.localhost:18443`
- `https://ops.slan.localhost:18443`

生产建议：

- 使用真实域名
- 使用真实证书
- 让 public 和 ops 拥有独立入口策略

### 2. 端口暴露

本地暴露：

- `28080`
- `28081`
- `18443`
- `19000/udp`
- `15432`
- `16379`

生产建议：

- public 只暴露必要端口
- PostgreSQL / Redis 不直接暴露公网
- ops 不直接暴露公网

### 3. 默认管理员

本地保留默认管理员是为了联调方便。

生产建议：

- 首启时允许 seed
- 首次登录后立即改密
- 确认 `/overview.securityWarnings` 清零

## 最低迁移动作

1. 复制 `.env.local.example`
2. 替换所有密码、token、secret
3. 把 `SLAN_HTTP_PUBLIC_HOST` 改成真实域名
4. 把 relay 地址改成真实公网地址
5. 部署真实 TLS
6. 首启后修改默认管理员密码
7. 跑一轮控制面和 relay 数据面 smoke
