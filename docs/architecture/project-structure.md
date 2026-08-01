# 项目目录与模块设计

## 目标

仓库按客户端、业务控制面、联网控制面、数据面兜底服务拆分。旧 `server/server-relay` 已删除，不再作为目标结构或实现入口。

## 目标目录结构

```text
slan/
├─ client/
│  ├─ app/                              # Flutter 客户端
│  └─ app_core/                         # 客户端核心
│
├─ server/
│  ├─ service-biz/                  # 业务控制面
│  ├─ opt-ui/                       # 统一运营/运维管理控制台
│  ├─ server-wire/                      # 联网控制面
│  ├─ server-wire-relay/                # UDP relay 数据面
│  └─ server-wire-derp/                 # DERP TCP 兜底数据面
│
├─ protocol/                            # 共享协议定义
├─ deploy/                              # 部署资源
├─ scripts/                             # 脚本
├─ docs/                                # 文档
└─ README.md
```

## 模块职责

### `client/app`

- 负责 UI、交互、页面状态、系统托盘和桌面端应用行为。
- 调用 `app_core` 提供的接口，不直接实现组网协议和隧道逻辑。

### `client/app_core`

- 负责客户端网络核心。
- 包括登录后配置拉取、WireGuard endpoint 管理、路径探测、LAN/IPv6/direct/relay/DERP 切换、TUN/TAP、DNS 与诊断。

### `server/service-biz`

- 负责业务控制面。
- 包括用户、网络、设备、成员权限、IP 分配、ACL、业务审计和业务 bootstrap。
- 不负责 WireGuard 路径规划、联网票据签发和业务流量中继。

### `server/opt-ui`

- 负责统一运营/运维管理控制台。
- 包括用户、设备、设备分组、网络、安全组、DNS 和中继/打洞节点配置。

### `server/server-wire`

- 负责联网控制面。
- 包括 peer 注册、endpoint 上报、路径探测与评分、runtime config、relay/DERP ticket 签发、DERP map 和内部拓扑接口。
- 不负责用户、组织、审计等业务域；商业计费已从系统退役。

### `server/server-wire-relay`

- 负责 UDP relay 数据面。
- 消费 `server-wire` 签发的 `relay_udp` ticket，维护 session，转发 UDP payload，提供管理和观测接口。
- 不负责路径规划、票据签发和业务域管理。

### `server/server-wire-derp`

- 负责 DERP 最终兜底数据面；当前实现为裸 TCP JSON-lines，生产 443/TLS 需要外部终止层或后续内置 TLS transport。
- 消费 `server-wire` 签发的 `derp_tcp_tls_443` ticket，维护连接和 session，提供 region/connection/session 观测接口。
- 不负责路径规划、票据签发和业务域管理。

## 实施约束

- `client/app` 不实现网络协议与打洞逻辑。
- `client/app_core` 不依赖 Flutter 页面结构。
- `server/service-biz` 不承担联网控制面和数据面职责。
- `server/server-wire` 不承担业务域职责。
- `server/server-wire-relay` 和 `server/server-wire-derp` 不签发票据，只校验票据。
- `protocol/` 和各子项目 `docs/` 保持接口语义一致。
