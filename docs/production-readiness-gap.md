# SLAN 生产化缺口整理

更新时间：2026-05-17

本文用于统一整理 SLAN 从当前开发/联调状态进入生产环境前还需要补齐的事项。范围覆盖 Web Console、`service-biz`、MQTT 控制通道、Wire/Relay 组网服务、客户端 macOS / Windows / Linux / iOS / Android，以及发布、观测和验收流程。

## 当前结论

当前系统已经具备核心链路雏形：客户端设备注册、MQTT 控制通道、Web Console 登录通知客户端、设备 session / 用户 session、基础组网、relay 转发、Postgres schema 和部分业务持久化都已经在推进。

但还不建议直接上生产。主要风险集中在：

- 身份与会话：用户 session、设备 session、consoleLoginKey、MQTT 凭证的续命、吊销、重放防护还需要完整闭环。
- 数据一致性：IP 分配、设备绑定、网络成员关系必须以 Postgres 事务和唯一约束兜底，不能依赖内存判断。
- MQTT 可靠性：已引入 delivery/ACK 思路，但还需要服务端持久化 delivery 状态、重试、过期、去重和断线补拉。
- 多平台客户端：macOS 已反复暴露安装、设备 ID、登录跳转、网络启停问题，Windows / Linux / iOS / Android 也必须跑同一套验收矩阵。
- 观测与运维：关键链路缺少统一指标、审计日志、告警和发布回滚策略。

## 当前落地状态

| 分级 | 模块 | 当前状态 | 仍需闭环 |
| --- | --- | --- | --- |
| P0 | 设备 ID | 已收紧服务端防串号：普通注册不允许跨用户抢占同一 `deviceId`；设备 session 跨用户绑定必须公钥一致；Web Console 完成登录不能抢占其他用户设备。 | 五平台真机验证；记录更完整的设备安装指纹；异常重复 `deviceId` 安全事件与告警。 |
| P0 | IP 唯一性 | schema 已有全局 IP、设备 active IP、同网络 virtual IP 唯一约束；Postgres 写路径已改为先持久化 IPAM 池，再只允许 claim `available` 或同设备 IP。 | 并发 integration test；冲突后自动换 IP 重试；IP 池耗尽错误码和运维工具。 |
| P0 | MQTT 可靠性 | 控制消息已有 `deliveryId/messageId/schemaVersion/createdAt/expiresAt`；客户端已有 ACK/去重测试；服务端已记录 pending/retrying/published/succeeded/failed/expired 状态，重试复用同一 `deliveryId`；delivery payload 已持久化；后台 worker 已按退避重试并过期终止；客户端 MQTT 重连后会立即同步控制面，已启网时重建本地网络配置以补拉 DNS、peers、ACL。 | 多实例下 delivery worker 的分布式锁或队列归属；五平台断网/重连真机矩阵验证。 |
| P0 | 登录跳转 | 主流程已收口到 `auth=login&deviceId=...`，服务端通过 MQTT 下发 `device_user_login_succeeded`。 | Web Console/Angular 已登录态、注册成功、刷新恢复、非法 deviceId 的 E2E 验证。 |
| P0 | session 续命 | 用户 session、设备 session 已有 TTL/renew 基础逻辑；客户端会在 MQTT credential 进入续命窗口时刷新设备 session；用户 logout 会清理用户 session 并撤销该用户 active 设备 session；ops 禁用/启用设备会持久化 device/runtime/membership 并通过 MQTT 下发成员状态和配置变更；ops 禁用用户会撤销该用户 user/device session，禁用其设备和网络成员，并通知相关设备。 | 续命退避策略细化；MQTT 权限吊销或刷新；五平台长时间续命验证。 |
| P0 | 客户端恢复 | 客户端已有本地服务状态和 MQTT worker 基础；UI widget test 已恢复设备消息入口。 | 服务重启、UI 重启、断网、系统重启、DNS/peers/ACL 内存下发的五平台矩阵。 |
| P1 | Postgres 权威存储 | `service-biz` 已接入 Postgres schema、启动 migration、核心 store 多数写路径已事务化。 | 移除剩余内存权威依赖；Postgres integration test 纳入 CI；migration 回滚/补偿。 |
| P1 | Web Console/Angular | 登录需求已文档化；旧登录完成 HTTP 等待分支已从协议和当前代码整理掉，登录完成统一由 MQTT 通知客户端。 | 统一 Angular 登录恢复逻辑；补 E2E。 |
| P1 | 权限安全 | 部分 session、device、MQTT 鉴权已有雏形；新密码已使用 bcrypt，旧 SHA-256 哈希保留兼容校验并在改密时升级；用户登录和 ops 登录已有账号/IP 维度的基础失败限流；登录、logout、consoleLoginKey、device login complete、设备注册/绑定/续命、网络成员、DNS、security group/rule、public mapping、主要 ops 管理操作、客户端网络控制 ACK 与 runtime 状态变化已有脱敏审计事件并可落 Postgres；ops 后台已有审计查询 API；普通日志已移除登录邮箱输出，MQTT auth 请求 key 会脱敏敏感字段名。 | Redis/网关级分布式限流、TLS、密钥管理、审计告警规则、集中日志接入。 |
| P1 | 多平台发布 | macOS 打包/安装脚本已有基础。 | Windows/Linux/iOS/Android 同协议同验收；签名、公证、版本兼容。 |
| P1 | 自动化验收 | 后端 Go、客户端 Rust/Flutter 测试已有。 | Android emulator、macOS 安装、跨设备 ping 长测、发布前一键验收。 |
| P2 | 运维能力 | relay 和部分服务日志已有。 | 指标、日志、告警、发布灰度、回滚演练、面板。 |

## P0 阻塞项

P0 未完成前，不建议对真实用户开放生产环境。

### 1. 设备 ID 生命周期

目标：每台物理设备拥有稳定且不串号的 `deviceId`。

必须补齐：

- macOS / Windows / Linux：安装、覆盖安装、升级不重置 `deviceId`。
- iOS / Android：App 升级保留 `deviceId`；卸载重装或清除 App 数据后允许生成新 `deviceId`。
- 服务端记录设备公钥、平台、安装指纹、首次注册时间、最后登录时间。
- 服务端检测同一 `deviceId` 出现在不同物理指纹或不同 MQTT 连接上的异常，并拒绝静默串号。
- 客户端本地状态目录不能随版本升级变化。

验收：

- 五个平台各跑首次安装、升级、重启、卸载重装、两台设备同时登录。
- 不同用户、不同设备登录后，客户端显示的用户、设备 ID、虚拟 IP 都不串。

### 2. IP 分配唯一性

目标：同一网络内不出现重复虚拟 IP。

必须补齐：

- Postgres 增加唯一约束：同一 `network_id + virtual_ip` 不重复。
- 全局设备 IP 池增加唯一约束，防止多个设备拿到同一个全局 IP。
- IP 分配在事务内完成，唯一约束冲突后自动换 IP 重试。
- 内存修复逻辑只能作为启动修复和防御，不作为唯一一致性来源。
- IP 池耗尽时返回稳定错误码，Web Console 和客户端能明确提示。

验收：

- 并发注册设备、并发加入网络、重复 IP 数据注入测试均不能产生重复 IP。
- 服务端重启后不会因为内存丢失重新分配已占用 IP。

### 3. MQTT 控制消息可靠性

目标：登录成功、配置下发、网络变更都通过 MQTT 可靠送达。

必须补齐：

- 所有服务端下发消息包含 `deliveryId`、`schemaVersion`、`createdAt`、`expiresAt`。
- 客户端处理成功或失败都 ACK，ACK 包含 `deliveryId`、状态、错误码、当前配置版本。
- 服务端持久化 delivery 状态，未 ACK 按退避重试，过期后标记失败。
- 客户端按 `deliveryId` 去重，重复消息只补 ACK，不重复执行网络启停。
- MQTT 断线重连后，客户端主动补拉 DNS、peers、ACL、网络配置和 session 状态。

验收：

- 断网、Broker 重启、服务端重启、重复下发、乱序下发、过期消息六类测试都能恢复到最新状态。

### 4. 登录与 Console 跳转闭环

目标：客户端点击登录后，Web Console 已登录或新登录成功时，客户端都能自动进入登录成功页。

必须补齐：

- 客户端登录 URL 只传 `auth=login&deviceId=...`，不再传临时完成标识、`clientPlatform`、`clientName`。
- Web Console 如果已有登录态，直接请求服务端完成当前 `deviceId` 登录。
- Web Console 如果无登录态，用户登录或注册成功后再请求服务端完成当前 `deviceId` 登录。
- 服务端校验 deviceId、用户 session、设备状态后，通过 MQTT 给目标设备下发 `device_user_login_succeeded`。
- 客户端收到 MQTT 登录成功后，保存用户 session，生成或续命设备 session，并跳到默认功能页。
- 浏览器刷新 Web Console 不应导致客户端或 Web Console 登录态立即丢失。

验收：

- 已登录浏览器、未登录浏览器、注册后登录、两个不同用户两个不同设备并发登录都必须通过。
- 客户端不得出现 A 设备收到 B 用户登录消息的情况。

### 5. 会话续命与吊销

目标：用户 session、设备 session、MQTT 凭证都可续命、可过期、可吊销。

必须补齐：

- 用户 session TTL、设备 session TTL、MQTT credential TTL 配置化。
- 客户端周期续命，失败后进入明确的重新登录状态。
- 用户退出时，服务端清理用户 session、设备 session，并吊销或刷新 MQTT 权限。
- 管理员禁用用户、设备、网络成员时，已在线客户端能被下线或停止网络；用户禁用和设备禁用路径已持久化并下发 MQTT 变更。
- refresh/renew 失败不能无限重试刷日志，需要退避和最终失败状态。

验收：

- session 过期、服务端吊销、客户端离线后重连、用户退出、管理员禁用五类场景都能进入正确状态。

### 6. 客户端状态恢复

目标：服务、UI、系统网络任何一层重启后能恢复到正确状态。

必须补齐：

- 服务进程重启后自动加载设备注册信息、MQTT 凭证、用户 session、设备 session。
- UI 重启后从本地服务读取当前状态，不重新触发错误登录流程。
- 网络断开恢复后 MQTT 自动重连，并补拉最新配置。
- DNS、peers、ACL、网络配置放内存运行；服务启动或配置变更由 MQTT 下放，不写回客户端配置文件。
- 网络启停失败要返回可定位的错误码和上下文，例如 utun 分配、地址配置、relay 解析、DNS 应用失败。

验收：

- 杀 UI、杀本地服务、断 Wi-Fi、切换网络、系统重启后，客户端状态一致。

## P1 上线前必备

### 1. Postgres 全量权威存储

目标：业务状态以 Postgres 为权威，不依赖进程内存。

必须补齐：

- 用户、session、consoleLoginKey、设备、网络、成员、IPAM、DNS、ACL、MQTT delivery 全部落 Postgres。
- 启动时从 Postgres 恢复内存工作集。
- 所有写路径要么直接事务写 DB 后更新内存，要么有可靠 outbox，不允许只写内存。
- migration 可重复执行，有版本记录和回滚/补偿策略。

验收：

- 服务进程重启、容器重建、多实例启动后数据一致。
- `go test` 覆盖关键 store 写路径，Postgres integration test 能在 CI 跑通。

### 2. Web Console 与 Angular 整理

目标：Web Console 登录、注册、设备绑定、默认页跳转逻辑清晰且可测试。

必须补齐：

- 登录入口统一处理 `auth=login&deviceId=...`。
- 已登录态恢复、注册成功、密码登录成功、刷新页面都走同一套完成设备登录逻辑。
- 非法或过期 deviceId / consoleLoginKey 有明确错误页。
- 不再保留旧浏览器完成分支或 HTTP 等待登录结果逻辑；客户端只接收 MQTT 登录通知。
- E2E 测试覆盖已登录、未登录、注册、非法 deviceId。

### 3. 安全基线

必须补齐：

- 密码使用 bcrypt 强哈希；旧 SHA-256 哈希仅作为兼容登录，改密后升级为 bcrypt。
- 登录限流：服务端已按账号/IP 做基础失败限流；生产多实例还需要 Redis/网关级限流，并补设备维度策略。
- HTTP/MQTT 生产环境必须启用 TLS 或由可信反向代理终止 TLS。
- 所有密钥、数据库密码、MQTT 凭证从环境或密钥系统注入。
- 日志禁止输出 password、token、session、MQTT password；MQTT auth 请求字段名和审计 details 已统一脱敏 password/token/secret/key/authorization。
- 登录成功、失败、限流、退出、consoleLoginKey 生成/消费、device login complete、设备注册、设备 session bootstrap/bind/renew、网络成员、DNS、security group/rule、public mapping、客户/设备/套餐/产品/订单/续费等 ops 操作、客户端网络控制 ACK、runtime 网络状态变化已有审计日志，details 会脱敏 password/token/secret/key。
- ops 后台已有审计查询 API，支持按 actor/action/resource/status 过滤。
- 关键写操作还需要扩展审计日志：审计告警规则和集中日志接入。

### 4. 多平台客户端发布一致性

必须补齐：

- macOS 安装包安装前停旧 UI 和服务，删除旧服务，再安装新服务。
- Windows / Linux 安装升级逻辑与 macOS 对齐。
- Android / iOS 嵌入式 Rust service 生命周期与前后台限制验证。
- 五个平台使用同一协议字段、同一登录流程、同一 MQTT ACK/去重规则。
- 发布包包含版本号、commit hash、协议版本，便于排查。

### 5. 自动化验收

必须补齐：

- 服务端单测、Postgres integration test、MQTT delivery test。
- 客户端 Rust `cargo test`。
- Flutter `analyze`、Flutter widget test。
- Android emulator 登录启网测试。
- macOS 客户端安装、登录、启网、ping 测试。
- 至少 2 小时、5 分钟一轮的跨设备 ping 长测。

## P2 生产运维能力

### 1. 指标

必须有：

- HTTP：QPS、P95/P99、错误码、登录成功率。
- MQTT：在线设备数、下发量、ACK 延迟、重试数、过期数、去重数。
- IPAM：分配成功、冲突重试、池耗尽、重复修复。
- Relay：attach、forward、drop、peer not attached、relay 解析失败。
- 客户端：登录状态、MQTT 状态、网络启停结果、session 续命结果。

### 2. 日志与追踪

必须有：

- requestId / traceId 贯穿 Web Console、service-biz、MQTT、客户端。
- 关键日志字段：userId、deviceId、networkId、deliveryId、sessionId、remoteIP。
- 客户端本地日志可导出，且能按登录、MQTT、网络启停、relay、DNS 分类。

### 3. 告警

必须有：

- Postgres / Redis / MQTT broker 不可用。
- MQTT ACK 超时率异常。
- 登录失败率异常。
- IP 冲突或 IP 池耗尽。
- Relay drop 率异常。
- 客户端网络启用失败率异常。

### 4. 发布与回滚

必须有：

- 服务端灰度发布和快速回滚。
- DB migration 发布前备份和回滚预案。
- 客户端安装包签名、公证、版本兼容策略。
- 协议版本兼容窗口：旧客户端至少能收到强制升级或兼容错误。

## 推荐执行顺序

1. 先完成 Postgres 权威存储和唯一约束，尤其是 IPAM、设备、session、MQTT delivery。
2. 再完成 MQTT delivery 持久化、ACK、重试、过期、去重、断线补拉。
3. 收口登录流程：只保留 `auth=login&deviceId=...`，Web Console 已登录/新登录都通过 MQTT 通知客户端。
4. 跑五平台设备 ID 生命周期和登录串号矩阵。
5. 跑组网稳定性矩阵：启网、关网、断网、重启、relay、DNS、ACL。
6. 补齐安全基线：密码哈希、限流、TLS、审计、敏感日志清理。
7. 建立观测面板和告警。
8. 最后做发布演练：服务端重发、客户端重新打包、安装升级、回滚。

## 最小生产验收标准

满足以下条件后，才建议进入小流量生产灰度：

- 两个不同用户、两台不同设备并发登录不串号。
- 同一网络内不会出现重复虚拟 IP。
- 客户端登录结果只依赖 MQTT，不再通过 HTTP 等待登录完成。
- MQTT 下发具备 ACK、重试、过期、去重和断线补拉。
- 服务端重启、客户端服务重启、UI 重启后状态能恢复。
- 用户退出或管理员禁用后，用户 session 和设备 session 都被清理或失效。
- macOS / Windows / Linux / iOS / Android 全部通过安装升级、登录、启网、ping、重启恢复测试。
- 2 小时跨设备 ping 长测无真实丢包，异常有日志可解释。
- 关键指标和告警可用，发布和回滚流程已经演练。
