# 网络稳定性生产落地计划

本文用于跟踪客户端组网进入生产前必须闭环的稳定性问题。

## 当前基线

- 设备 ID：macOS / Windows / Linux 安装升级已改为 `--ensure-device-id`，不会因覆盖安装重置；Android / iOS embedded `stateDir` 已改为平台持久目录。仍需真机验证卸载、重装、升级、多设备永不串号。
- IP 分配：`server/service-biz/internal/biz/store.go` 内存层有全局 IP 池和重复修复，`NetworkConfig` 会触发 `ensureUniqueGlobalIPsLocked`。生产仍必须加 DB 唯一约束和事务重试，不能只靠内存。
- MQTT 控制：客户端已有 `deliveryId/messageId`、本地控制任务持久化、下行 ACK 队列。还需要明确 retry、expiry、去重窗口、断线重连后补拉最新配置。
- 客户端恢复：已有启动 session 恢复和 MQTT 连接流程。还需要覆盖服务重启、UI 重启、网络断开恢复、登录结果只依赖 MQTT 的集成测试。
- 观测性：relay attach/forward/drop 已有部分日志和指标；登录、consoleLoginKey、MQTT ACK、IP 冲突、客户端网络启停失败还需要统一指标。

## P0 必须落地

### 设备 ID 生命周期

- macOS / Windows / Linux：安装、覆盖安装、升级必须保留 `config.json`。
- Android / iOS：App 升级必须保留平台持久化 `deviceId`；清除 App 数据或卸载重装后允许生成新 ID。
- 服务端必须能识别同一个 `deviceId` 被不同物理设备并发使用的异常：同一时间出现不同公钥、平台指纹或 MQTT client 连接冲突时记录安全事件，并拒绝静默串号。
- 验收：五个平台各跑“首次安装、升级、重启、卸载重装、两台设备并发登录”矩阵，确认设备 ID、用户 session、设备 session 不串。

### IP 分配唯一性

- DB schema 增加唯一约束：同一网络内 `networkId + virtualIP` 不重复；全局设备 IP 池内 `globalIP` 不重复。
- 分配 IP 必须在事务内完成：读取候选、写入占用、唯一约束冲突后自动换 IP 重试。
- 配置生成前继续保留内存修复作为防线，但修复事件必须打日志和指标。
- 验收：并发注册、并发入网、重复数据注入三类测试都不能产生同网络重复 IP。

### MQTT 消息可靠性

- 每条服务端下发控制消息必须包含 `messageId` 或 `deliveryId`、`expiresAt`、`createdAt`、`schemaVersion`。
- 客户端处理成功或失败都必须 ACK，ACK 包含 `deliveryId`、状态、错误码、当前配置版本。
- 服务端未收到 ACK 时按退避重试，超过过期时间停止重试并标记失败。
- 客户端按 `deliveryId` 去重；已成功处理的消息重复到达时只补 ACK，不重复执行网络启停。
- 客户端 MQTT 断线重连后必须补拉最新网络配置、DNS、peers、ACL，并重新上报 runtime 状态。
- 验收：断网、服务端重启、MQTT broker 重启、重复下发、乱序下发、过期消息六类测试都能恢复到最新配置。

### 客户端状态恢复

- 服务进程重启：自动加载本地设备注册信息、MQTT 凭证、用户 session、设备 session，并续命。
- UI 重启：不影响服务面；UI 从本地服务读取当前登录与网络状态。
- 网络断开恢复：MQTT 自动重连，恢复后补拉最新配置，并按当前用户 session 恢复网络。
- 登录完成只依赖服务端 MQTT 通知。
- 验收：杀 UI、杀服务、断 Wi-Fi、切网络、机器重启后，客户端都能回到正确登录页和网络状态。

### 观测性

- 登录：成功、失败、用户 session 创建、设备 session 绑定、续命失败。
- consoleLoginKey：生成、消费、非法、过期、重复消费。
- MQTT：下发、ACK、重试、过期、去重、重连后补拉。
- 设备状态：上线、离线、runtime 更新、设备 ID 冲突。
- relay：attach、forward、drop、peer not attached、relay 地址解析失败。
- IPAM：分配、释放、冲突、DB 唯一约束重试、内存修复。
- 客户端网络：启用失败、关闭失败、utun/address/relay/DNS/ACL 应用失败。

## 下一步执行顺序

1. 先补客户端控制任务 ACK/去重持久化测试，防止 MQTT 下行重复执行。
2. 再补服务端 IP 分配并发和重复注入测试。
3. 设计 DB migration：唯一约束、索引、冲突重试。
4. 增加服务端 MQTT delivery 状态表或 Redis 状态，支持重试、过期、断线补发。
5. 增加五平台恢复矩阵脚本和 2 小时 ping 长测脚本到 CI/发布验收。
