# Service Biz API Instances

`service-biz` 使用同一镜像运行 App、Web 和 Ops 三种 route set。业务实现位于
`internal/service`，HTTP 适配位于 `internal/api`，路由组合位于 `internal/app`。

| 实例 | Route set | 默认端口 | 调用方 | 职责 |
| --- | --- | --- | --- | --- |
| App API | `app` | `28080` | 客户端、wire、BifroMQ | 设备授权、设备运行控制、MQTT 和内部 wire |
| Web API | `web` | `28081` | 客户端下载页 | 只提供公开客户端下载信息 |
| Ops API | `ops` | `28082` | `opt-ui` | 平台资源、客户、授权 key 和运营管理 |

## Identity Boundary

- 客户端没有账号、密码或用户 session。
- 客户端使用设备授权 key 调用 `POST /api/device-auth/token`，换取设备 token、refresh token 和 MQTT 凭据。
- Device、Network 和 DeviceGroup 是平台级资源，不挂在客户或客户端账号下。
- Customer 只用于运营资料，不参与设备鉴权或资源归属。
- Operator 账号只用于 `/api/ops/*` 后台管理。

## Shared Routes

| Method | Path | Auth | 用途 |
| --- | --- | --- | --- |
| `GET` | `/healthz` | 无 | 健康检查 |

## App API

| Method | Path | Auth | 用途 |
| --- | --- | --- | --- |
| `POST` | `/api/device-auth/token` | 授权 key | 换取设备 token 和 MQTT 凭据 |
| `POST` | `/api/app/device/session/renew` | Device Bearer | 续期设备 session 并上报运行指标 |
| `POST` | `/api/app/devices/{deviceId}/renew` | Device Bearer | 续期设备运行租约 |
| `GET` | `/api/app/devices/{deviceId}/network-configs` | Device Bearer | 获取设备参与的网络配置 |
| `GET` | `/api/app/devices/{deviceId}/mqtt-credential` | Device Bearer | 获取 MQTT 凭据 |
| `GET` | `/api/app/devices/{deviceId}/mqtt-profile` | Device Bearer | 获取 MQTT profile |
| `POST` | `/api/app/devices/{deviceId}/runtime` | Device Bearer | 上报设备运行状态 |
| `POST` | `/api/app/devices/{deviceId}/logs` | Device Bearer | 上传客户端日志 |
| `POST` | `/api/app/client/messages` | Device Bearer | 发送客户端控制消息 |
| `GET` | `/api/app/runtime/endpoints` | Device Bearer | 获取运行端点 |
| `GET` | `/api/app/networks/{networkId}/snapshot` | Device Bearer | 获取设备可见网络快照 |
| `GET/POST` | `/api/app/networks/{networkId}/relay-candidates` | Device Bearer | 获取 relay 候选 |
| `POST` | `/api/app/relay/tickets` | Device Bearer | 获取 relay ticket |
| `POST` | `/api/app/networks/{networkId}/punch/connect-sessions` | 设备 MQTT 签名 | 创建 P2P 打洞会话 |

App route set 同时承载 `/mqtt/*` BifroMQ webhook 和 `/internal/wire/*` 内部接口。MQTT webhook
使用 `X-Slan-MQTT-Webhook-Token`，内部 wire 接口使用 `X-Slan-Internal-Token`；两者都是服务间接口，不能作为客户端业务 API 使用。

## Web API

Web route set 只暴露客户端下载列表和下载文件。不提供注册、登录、账号、用户 session、
网络或设备管理接口。

## Ops API

除 `POST /api/ops/auth/login` 外，所有 Ops API 使用 Operator Bearer。

| 领域 | 主要路径 | 能力 |
| --- | --- | --- |
| 操作员 | `/api/ops/operators` | 创建、更新、重置密码和停用操作员 |
| 客户 | `/api/ops/customers` | 维护客户基础资料 |
| 设备 | `/api/ops/devices` | 创建、查询、更新和删除平台设备 |
| 授权 key | `/api/ops/device-credentials` | 创建、查询和吊销设备授权 key |
| 网络 | `/api/ops/networks` | 创建、更新、删除网络并绑定设备或设备组 |
| 设备组 | `/api/ops/device-groups` | 管理设备组和组成员 |
| DNS/ACL | `/api/ops/networks/{networkId}/dns`, `/api/ops/security-*` | 管理网络 DNS 和安全策略 |
| 节点 | `/api/ops/relay-nodes`, `/api/ops/punch-nodes` | 管理 Relay 和 Punch 节点 |

授权 key 完整值只在创建响应中返回一次。服务端保存 HMAC-SHA256 摘要；吊销 key 时同时
撤销其关联的活动设备 session。
