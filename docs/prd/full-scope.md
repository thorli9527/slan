# SLAN Product Scope

## Positioning

SLAN 是跨平台安全组网平台。平台管理员集中管理网络、设备、设备组和访问策略；客户端只负责
设备激活、运行状态和安全网络连接。

## Architecture

- Client：Rust 网络核心与 Flutter UI，使用设备授权 key 激活。
- Business control plane：Go `service-biz`，管理设备身份、网络资源、策略、MQTT 和 Ops API。
- Network control plane：`server-wire`，负责 peer 配置、路径规划和数据面票据。
- Data plane：P2P、Relay UDP 和 DERP fallback。
- Operations：`opt-ui` 管理平台资源、授权 key、客户和节点。

## Identity Model

- 客户端没有账号、密码或账号会话。
- Device 是唯一客户端主体。
- 授权 key 由 Ops 创建，只在创建时显示完整值。
- 客户端用授权 key 换取短期 device token、refresh token 和 MQTT 凭据。
- Customer 仅保存运营资料，不拥有 Device、Network 或 DeviceGroup。
- Operator 是后台管理主体，不可作为客户端身份使用。

## Resource Model

- Network、Device 和 DeviceGroup 是 Ops 管理的平台资源。
- Device 可直接加入 Network，也可通过 DeviceGroup 被 Network 引用。
- DNS、ACL、安全组和网络成员变更只通过 Ops 管理。
- 客户端只能读取自身参与网络的运行配置，不能创建、修改或删除平台资源。

## Client Scope

- 输入授权 key 并完成设备激活。
- 安全保存 device token、refresh token 和 MQTT 凭据。
- 自动续期、重连和恢复本地网络状态。
- 展示设备、网络连接、路径、延迟、流量和诊断信息。
- 执行 P2P 优先、Relay/DERP fallback 和 TUN/DNS/ACL 配置。

## Operations Scope

- 管理操作员、客户和审计日志。
- 管理网络、设备、设备组、DNS、ACL 和授权 key。
- 管理 Relay、DERP 和 Punch 节点。

## Current Decisions

- 不迁移旧用户、用户 session、用户资源归属和邀请数据。
- 不提供客户端注册、登录、用户列表、用户别名或改密接口。
- 不提供用户态网络管理 Web Console。
- 授权 key 不是长期 API token；它只用于交换可续期、可吊销的设备 session。
