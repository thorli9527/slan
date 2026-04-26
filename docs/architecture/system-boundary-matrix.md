# 系统边界与接口矩阵

## Maintained Main-Flow Status

The create / join / alias / switch / activate / bootstrap / control-session /
relay-ticket main flow is now wired across:

- Flutter app UI and app workspace services
- app-core bridge, JSON facade, controller-client, and relay/path abstractions
- server-biz HTTP routes, DTOs, service layer, OpenAPI, and protocol contracts
- web console network lifecycle helpers

Remaining gaps in this matrix should be read as production and real-network
hardening unless a row explicitly says an API is missing.

本文档把当前主链路中的三个核心子系统统一放到一张边界矩阵里：

- `client/app_core`
- `server/server-biz`
- `server/server-relay`

目标：

- 明确每个子系统“自己负责什么”
- 明确“对外输出什么”
- 明确“对外接入什么”
- 明确三者之间的协议边界

## 1. 三个核心子系统

| 子系统 | 定位 | 不负责 |
| --- | --- | --- |
| `client/app_core` | 客户端运行时编排层，承接控制面配置、P2P、relay / DERP、路径切换与隧道编排 | UI、控制面业务存储、数据面服务端转发 |
| `server/server-biz` | 控制面，负责身份、网络、配置、控制通道、ticket 编排 | 真实业务流量转发 |
| `server/server-relay` | 数据面，负责 ticket 消费、session、数据转发 | 用户、网络、设备和成员管理 |

## 2. 职责矩阵

| 能力 | app_core | server-biz | server-relay |
| --- | --- | --- | --- |
| 注册 / 登录 | 调用方 | 负责实现 | 不负责 |
| 设备注册 | 调用方 | 负责实现 | 不负责 |
| 节点注册 | 调用方 | 负责实现 | 不负责 |
| 网络创建 / 加入 | 调用方 | 负责实现 | 不负责 |
| Bootstrap | 消费与运行时装配 | 负责编排与返回 | 不负责 |
| 控制通道 | 建连、消费、上报 | 负责会话与消息编排 | 不负责 |
| NAT / P2P | 负责实现 | 提供地图与计划 | 不负责 |
| Relay / DERP ticket | 消费 | 负责签发 | 负责校验 |
| Relay / DERP attach | 负责发起 | 不直接处理数据流 | 负责接入 |
| 数据转发 | 不负责服务端转发 | 不负责 | 负责 |
| DERP 池化与切换 | 负责实现 | 提供集群视图和授权 | 提供集群消费能力 |

## 3. 输入与输出矩阵

### 3.1 `client/app_core`

| 维度 | 内容 |
| --- | --- |
| 输入 | Flutter FFI 调用、控制面 HTTP/MQTT、relay/DERP 数据面 |
| 输出 | `Session`、`Device`、`Node`、`Network`、`BootstrapConfig`、`RelayTicket`、`ConnectionState` |
| 内部核心 | `controller-client`、`p2p`、`relay-client`、`tunnel`、`ffi-bridge` |
| 内部新增重点 | `DerpMap`、`DerpPool`、`PathManager` |

### 3.2 `server/server-biz`

| 维度 | 内容 |
| --- | --- |
| 输入 | HTTP 请求、控制通道消息、配置 |
| 输出 | HTTP DTO、`BootstrapResponse`、`ControlSessionResponse`、`NetworkMap`、`RelayTicket` |
| 内部核心 | `auth`、`device`、`network`、`node`、`control`、`ws` |
| 内部新增重点 | `derp_map` 编排、cluster-aware ticket、DERP 候选排序 |

### 3.3 `server/server-relay`

| 维度 | 内容 |
| --- | --- |
| 输入 | `RelayTicket`、客户端 attach、客户端 UDP 包 |
| 输出 | attach 结果、转发结果、错误语义 |
| 内部核心 | `relay-core`、`auth`、`session`、`udp-relay` |
| 内部新增重点 | cluster-aware ticket 校验、多节点 attach、DERP 集群语义 |

## 4. 协议边界矩阵

| 边界 | 协议/模型 | 当前承载 |
| --- | --- | --- |
| app_core -> server-biz | HTTP DTO | `protocol/openapi/phase1.yaml` |
| app_core <-> server-biz | 控制通道消息 | `protocol/protobuf/control.proto` |
| server-biz -> server-relay | `RelayTicket` / `DerpTicket` 语义 | `api/dto` + `protocol` |
| app_core -> server-relay | attach / forward 数据面语义 | `relay-client` + `server-relay` crate |

## 5. 主链路边界

### 5.1 身份与网络

```text
Flutter
-> app_core
-> server-biz(auth/device/node/network)
```

### 5.2 启动与控制

```text
Flutter
-> app_core(controller-client)
-> server-biz(bootstrap/MQTT control)
```

### 5.3 回退与数据面

```text
app_core(path-manager / relay-client)
-> server-biz(issue ticket)
-> server-relay(attach / forward)
```

## 6. DERP 集群扩展矩阵

| 能力 | app_core | server-biz | server-relay |
| --- | --- | --- | --- |
| DERP 节点列表 | 消费 `derp_map` | 编排并下发 | 不负责生成 |
| 热连接 2~3 个节点 | 负责 | 不负责 | 被动接入 |
| 单路径 active 发送 | 负责 | 不负责 | 被动承载 |
| 评分与切换 | 负责 | 可给推荐顺序 | 不负责调度 |
| ticket 集群授权 | 消费 | 负责签发 | 负责校验 |
| 集群内节点切换 | 发起切换 | 可下发建议 | 负责 accept/attach |

## 7. 当前缺口总览

### `app_core`

- 已具备真实 `controller-client` HTTP 接入、join / switch / activate / bootstrap / ticket 主流程。
- 仍需扩展生产级 `DerpClient` / `DerpPool` / `PathManager` 诊断与真实网络覆盖。
- 仍需继续完善跨平台 `tunnel` 安装、权限和故障恢复路径。

### `server-biz`

- 已具备 `bootstrap`、显式 `control/sessions`、`NetworkMap`、`derp_map` 与 cluster-aware ticket 主流程。
- 仍需继续补强策略 / ACL、生产配置校验和更完整的控制通道事件覆盖。

### `server-relay`

- 缺少真实监听 runtime
- 缺少集群化语义
- 缺少 DERP 多节点支持

## 8. 推荐联动实现顺序

1. 固化当前 client create / join / alias / switch / activate / bootstrap / relay 主流程回归测试。
2. 扩展 `app_core.derp_pool` 的真实网络诊断和生产路径覆盖。
3. 补强 `server-biz` 策略 / ACL 与控制通道事件覆盖。
4. 补 `server-relay` 的集群接入能力和多节点运行时。
5. 最后补跨平台隧道安装、权限和恢复体验。

## 9. 文档入口

- `client/app_core/docs`
- `server/server-biz/docs`
- `server/server-relay/docs`

本文件用于横向看三边边界，上述目录用于纵向看各自内部设计。
