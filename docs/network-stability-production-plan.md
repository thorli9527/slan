# 网络稳定性生产落地计划

## Current Baseline

- 客户端以稳定 `deviceId` 作为唯一主体。
- Ops 创建授权 key，客户端交换 Device token 和 MQTT 凭据。
- Network、Device 和 DeviceGroup 均由 Ops 管理。
- MQTT 控制消息具备 delivery ID、ACK、持久化和重试基础。
- Postgres 保存设备、session、网络关系、IPAM 和控制消息状态。

## P0

### Device Identity

- 五个平台验证首次安装、升级、重启和卸载重装的 `deviceId` 生命周期。
- 检测同一 `deviceId` 在不同安装指纹或 MQTT 连接并发使用。
- 本地凭据必须加密保存，安装升级不得静默覆盖。

### Authorization And Session

- 覆盖 key 创建、首次绑定、重复交换、过期、吊销和错误 key。
- refresh rotation 必须支持安全的幂等重试并检测重放。
- 吊销 key、禁用设备后，HTTP token 和 MQTT 权限必须及时失效。
- key 和 token 不得进入日志、URL、崩溃报告或 UI 明文持久化。

### IP And Resource Consistency

- 同一网络 `networkId + virtualIp` 和全局设备 IP 使用数据库唯一约束。
- IP 分配在事务内完成，唯一冲突后换候选地址重试。
- Device、DeviceGroup 与 Network 关系更新必须原子写入并触发配置版本变更。

### MQTT Reliability

- 下行消息包含 `deliveryId`、`schemaVersion`、`createdAt` 和 `expiresAt`。
- 客户端处理后 ACK，并按 delivery ID 去重。
- 服务端按退避重试，过期后终止并告警。
- 客户端断线重连后补拉最新网络、DNS、peer 和 ACL 配置。

### Client Recovery

- 服务进程重启后恢复 Device session、MQTT 和已启用网络。
- UI 重启不影响后台网络服务。
- 断网、切网和系统重启后自动续期、重连并收敛到服务端最新版本。

## Verification Matrix

- 五个平台：首次安装、升级、重启、卸载重装、错误 key、吊销 key。
- 两台及以上设备：并发激活、并发入网、跨设备 token 访问拒绝。
- 网络故障：Broker 重启、服务端重启、乱序/重复/过期 MQTT 消息。
- 数据一致性：并发 IP 分配、事务回滚、重复关系写入和数据库重启。
- 长测：P2P/Relay/DERP 路径切换与至少两小时持续 ping/流量。
