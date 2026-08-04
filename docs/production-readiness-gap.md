# SLAN Production Readiness Gaps

更新时间：2026-08-02

## Current State

核心设备链路已具备：Ops 资源管理、设备授权 key、Device session、MQTT 控制、Postgres
持久化、P2P 和 Relay/DERP fallback。客户端不再依赖账号或浏览器登录。

## Priority Gaps

| Priority | Area | Remaining Work |
| --- | --- | --- |
| P0 | 授权 key | 分布式失败限流、异常来源审计的告警通知消费端、pepper 轮换演练、五平台 E2E |
| P0 | Device session | 长时间续期、禁用/吊销即时生效的多实例验证 |
| P0 | IPAM | PostgreSQL 并发分配测试纳入 CI、计数器/地址一致性检查与运维修复工具 |
| P0 | MQTT | 多实例 worker 归属、断线补拉、乱序/过期消息矩阵 |
| P0 | 客户端恢复 | 服务/UI/系统重启和网络切换的五平台验证 |
| P1 | PostgreSQL | 集成测试纳入 CI、备份恢复、schema 变更回滚策略 |
| P1 | Security | TLS、集中密钥管理、网关限流、审计告警和 CI 敏感字段扫描 |
| P1 | Release | Windows/macOS 签名，移动端发布，版本兼容和自动回滚 |
| P1 | Observability | 指标、结构化日志、trace、SLO 和故障面板 |

## Release Gates

1. 有效、无效、过期、已吊销授权 key 的行为全部通过自动化验证。
2. Device token 不能访问其他设备或未加入网络的资源。
3. 禁用设备或吊销 key 后，HTTP、MQTT 和网络访问在目标时间内失效。
4. 并发激活、IP 分配和网络关系更新不产生重复或悬挂数据。
5. Broker、biz、wire 和客户端任一重启后均能自动收敛。
6. 五个平台通过安装、升级、恢复和长时间网络稳定性矩阵。
7. 生产密钥不使用默认值，日志中不出现授权 key、token 或 MQTT 密码。

## Data Policy

本次业务重构不迁移旧账号、账号 session、用户归属、邀请或别名数据。部署前应使用新 schema
和明确的全新数据库/命名空间，避免旧表被误认为兼容数据源。
