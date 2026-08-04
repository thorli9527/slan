# 系统边界与接口矩阵

本文档按新系统切割维护。旧 `server/server-relay` Rust 子系统已删除，联网数据面统一拆为 `server-wire-relay` 与 `server-wire-derp`。

## 核心子系统

| 子系统 | 定位 | 不负责 |
| --- | --- | --- |
| `client/rust` | 客户端运行时编排层，承接设备授权、控制面配置、WireGuard、路径切换与隧道编排 | UI、平台资源管理、服务端转发 |
| `server/service-biz` | 业务控制面，负责设备身份、网络资源、授权 key、策略、客户资料和 Ops API | WireGuard 路径规划、真实流量转发 |
| `server/opt-ui` | 运营管理控制台，负责运营用户、客户、授权 Key 和节点运营界面 | 客户端控制面、数据面转发 |
| `server/server-wire` | 联网控制面，负责 peer 注册、runtime config、路径规划、relay/DERP ticket 签发 | 客户资料、授权 key、审计等业务域 |
| `server/server-wire-relay` | UDP relay 数据面，负责 `relay_udp` ticket 消费、session 和 UDP 转发 | 业务域管理、路径评分、票据签发 |
| `server/server-wire-derp` | DERP 兜底数据面，负责 `derp_tcp_tls_443` ticket 消费、连接和转发；当前实现为裸 TCP JSON-lines | 业务域管理、路径评分、票据签发 |

## 职责矩阵

| 能力 | client/rust | service-biz | server-wire | server-wire-relay | server-wire-derp |
| --- | --- | --- | --- | --- | --- |
| 授权 key 换 token | 调用方 | 负责实现 | 不负责 | 不负责 | 不负责 |
| 设备身份与 session | 调用方 | 负责实现 | 消费已授权设备身份 | 不负责 | 不负责 |
| 网络与设备组管理 | 不负责 | Ops API 负责 | 消费网络与 peer 授权 | 不负责 | 不负责 |
| Runtime config | 消费与装配 | 提供业务配置 | 负责联网配置 | 不负责 | 不负责 |
| 路径规划 | 执行与上报 | 不负责 | 负责探测、评分、重选路 | 不负责 | 不负责 |
| Relay / DERP ticket | 消费 | 不签发联网票据 | 负责签发 | 负责校验 relay ticket | 负责校验 DERP ticket |
| Relay / DERP attach | 负责发起 | 不处理数据流 | 不处理数据流 | 负责 UDP relay 接入 | 负责 DERP TCP 接入 |
| 数据转发 | 不负责服务端转发 | 不负责 | 不负责 | 负责 UDP 转发 | 负责 DERP 兜底转发 |

## 协议边界

| 边界 | 协议/模型 | 当前承载 |
| --- | --- | --- |
| client/rust -> service-biz | 设备 HTTP DTO | `service-biz` App API |
| server-wire -> service-biz | 业务授权 / peer 授权 / runtime config / topology | `service-biz` `/internal/wire/*` 只读内部 API，`X-Slan-Internal-Token` 鉴权 |
| client/rust -> server-wire | peer register、runtime config、path plan、ticket | `server-wire` HTTP API |
| client/rust -> server-wire-relay | attach / forward UDP 数据面语义 | `server-wire-relay` UDP JSON 协议 |
| client/rust -> server-wire-derp | connect / send TCP 兜底语义 | `server-wire-derp` TCP JSON 协议 |

## 主路径

```text
LAN Direct -> IPv6 Direct -> Direct UDP -> Relay UDP -> DERP TCP fallback
```

`server-wire` 负责路径探测与评分、快速重选路、MTU 探测、IPv6 优先但可回退、LAN 优先直连、endpoint roaming、票据续期和 per-peer keepalive 策略。数据面服务只消费已签发票据，不反向侵入业务控制面。

## 文档入口

- `server/service-biz/README.md`
- `server/opt-ui`
- `server/server-wire/docs`
- `server/server-wire-relay/docs`
- `server/server-wire-derp/docs`
- `docs/wire-client-protocol.md`
