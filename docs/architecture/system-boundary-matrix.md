# 系统边界与接口矩阵

本文档按新系统切割维护。旧 `server/server-relay` Rust 子系统已删除，联网数据面统一拆为 `server-wire-relay` 与 `server-wire-derp`。

## 核心子系统

| 子系统 | 定位 | 不负责 |
| --- | --- | --- |
| `client/app_core` | 客户端运行时编排层，承接控制面配置、WireGuard、路径切换、relay / DERP 兜底与隧道编排 | UI、业务控制面存储、服务端数据面转发 |
| `server/server-biz` | 业务控制面，负责账号、组织、网络资产、设备归属和业务权限 | WireGuard 路径规划、联网票据签发、真实流量转发 |
| `server/server-wire` | 联网控制面，负责 peer 注册、runtime config、路径规划、relay/DERP ticket 签发 | 用户、组织、计费、审计等业务域 |
| `server/server-wire-relay` | UDP relay 数据面，负责 `relay_udp` ticket 消费、session 和 UDP 转发 | 业务域管理、路径评分、票据签发 |
| `server/server-wire-derp` | TCP/TLS 443 兜底数据面，负责 `derp_tcp_tls_443` ticket 消费、连接和转发 | 业务域管理、路径评分、票据签发 |

## 职责矩阵

| 能力 | app_core | server-biz | server-wire | server-wire-relay | server-wire-derp |
| --- | --- | --- | --- | --- | --- |
| 注册 / 登录 | 调用方 | 负责实现 | 不负责 | 不负责 | 不负责 |
| 设备注册 | 调用方 | 负责实现 | 消费已授权设备身份 | 不负责 | 不负责 |
| 网络创建 / 加入 | 调用方 | 负责实现 | 消费网络与 peer 授权 | 不负责 | 不负责 |
| Runtime config | 消费与装配 | 提供业务配置 | 负责联网配置 | 不负责 | 不负责 |
| 路径规划 | 执行与上报 | 不负责 | 负责探测、评分、重选路 | 不负责 | 不负责 |
| Relay / DERP ticket | 消费 | 不签发联网票据 | 负责签发 | 负责校验 relay ticket | 负责校验 DERP ticket |
| Relay / DERP attach | 负责发起 | 不处理数据流 | 不处理数据流 | 负责 UDP relay 接入 | 负责 TCP/TLS 443 接入 |
| 数据转发 | 不负责服务端转发 | 不负责 | 不负责 | 负责 UDP 转发 | 负责 DERP 兜底转发 |

## 协议边界

| 边界 | 协议/模型 | 当前承载 |
| --- | --- | --- |
| app_core -> server-biz | 业务 HTTP DTO | `server-biz` HTTP API |
| server-wire -> server-biz | 业务授权 / peer 授权 / runtime config / topology | `server-biz` `/internal/wire/*` 只读内部 API，`X-Slan-Internal-Token` 鉴权 |
| app_core -> server-wire | peer register、runtime config、path plan、ticket | `server-wire` HTTP API |
| app_core -> server-wire-relay | attach / forward UDP 数据面语义 | `server-wire-relay` UDP JSON 协议 |
| app_core -> server-wire-derp | connect / send TCP 兜底语义 | `server-wire-derp` TCP JSON 协议 |

## 主路径

```text
LAN Direct -> IPv6 Direct -> Direct UDP -> Relay UDP -> DERP TCP/TLS 443
```

`server-wire` 负责路径探测与评分、快速重选路、MTU 探测、IPv6 优先但可回退、LAN 优先直连、endpoint roaming、票据续期和 per-peer keepalive 策略。数据面服务只消费已签发票据，不反向侵入业务控制面。

## 文档入口

- `server/server-biz/docs`
- `server/server-wire/docs`
- `server/server-wire-relay/docs`
- `server/server-wire-derp/docs`
- `docs/wire-client-protocol.md`
