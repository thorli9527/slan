# Phase 1 MVP

## Goal

验证最短设备组网闭环：

1. Operator 创建设备授权 key。
2. 客户端使用 key 换取 device token、refresh token 和 MQTT 凭据。
3. Operator 创建 Network，并直接加入 Device 或引用 DeviceGroup。
4. 客户端拉取自身网络配置和虚拟 IP。
5. 客户端建立 MQTT 控制通道。
6. 客户端优先 P2P，失败后回退 Relay/DERP。
7. 客户端展示设备和网络运行状态。

## Scope

### `service-biz`

- 设备授权 key 的创建、交换、过期和吊销。
- Device session 的签发、续期和撤销。
- Ops 管理 Network、Device、DeviceGroup、DNS 和 ACL。
- IP 分配、MQTT 控制、内部 wire 授权和运行配置。

### Client

- 持久化设备身份和加密凭据。
- 拉取网络配置并维持 MQTT 控制通道。
- NAT 探测、P2P、Relay/DERP fallback 和加密隧道。
- 授权、连接状态和基础错误反馈。

## Acceptance Criteria

1. 有效 key 可激活设备，无效、过期或已吊销 key 被拒绝。
2. Device token 只能访问自身及已加入网络的运行资源。
3. Operator 可独立管理网络、设备和设备组关系。
4. 客户端可续期、重连并恢复最新网络配置。
5. P2P 不可用时可自动回退 Relay/DERP。
6. 整个客户端流程不需要账号、密码或账号会话。
