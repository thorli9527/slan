# 上线准备清单（server-biz 控制面）

本文档用于评估与推进 `server/server-biz` 上生产所需的最小条件与工程化完善项。

## P0 阻塞项（未完成不建议上线）

### 账号密码安全

- [ ] 密码使用强哈希（bcrypt/argon2）+ 每用户随机盐；禁止等值/弱 hash
- [ ] 登录校验使用恒定时间比较
- [ ] 密码策略与校验：长度、复杂度、常见弱口令拦截（至少可配置）
- [ ] 登录失败次数限制与风控（按 IP/账号维度），必要时接入验证码/二次验证
- [ ] 敏感信息禁止出现在日志（password、token、密钥）

落点参考：
- Auth 实现：[access.go](file:///Users/thorli/workspace/slan/server/server-biz/internal/service/impl/access.go)

### Token / 会话体系可控

- [ ] access/refresh TTL 明确且可配置
- [ ] refresh token 轮换（rotation）与复用检测（reuse detection）
- [ ] 支持吊销：单用户/单设备/单会话强制下线立即生效
- [ ] token 与设备/客户端信息绑定（至少用于风控与审计）
- [ ] Redis 故障策略明确（拒绝/降级）并设置超时与重试上限

落点参考：
- token 签发与写入：[access.go](file:///Users/thorli/workspace/slan/server/server-biz/internal/service/impl/access.go)
- Redis token store：[redis.go](file:///Users/thorli/workspace/slan/server/server-biz/internal/repo/redis.go)
- 设计建议：[token-session-design.md](file:///Users/thorli/workspace/slan/docs/architecture/token-session-design.md)

### 传输安全与配置安全

- [ ] HTTP/WS 必须在 TLS 下运行（或由反向代理终止 TLS），并确保 WS 使用 WSS
- [ ] 正确处理反向代理场景的真实 IP（X-Forwarded-For / X-Real-IP）与信任边界
- [ ] 默认配置仅用于开发环境；生产环境禁止默认弱口令与默认连接串
- [ ] 所有密钥/凭证从环境或密钥系统注入（而不是写在配置样例里）

落点参考：
- 配置结构与默认值：[config.go](file:///Users/thorli/workspace/slan/server/server-biz/configs/config.go)

### 数据一致性与约束（IPAM/入网）

- [ ] 为虚拟 IP 分配增加 DB 唯一约束（subnetId + virtualIP），并在冲突时自动重试
- [ ] 入网（member/attachment）关键写操作保证事务一致性（并发下无重复/脏数据）
- [ ] IP 池耗尽时的错误与引导策略明确（错误码、提示、运维手段）
- [ ] 为高频查询补齐索引（networkId/deviceId/nodeId/member/attachment）

落点参考：
- IP 分配与校验：[network_support.go](file:///Users/thorli/workspace/slan/server/server-biz/internal/service/impl/network_support.go)
- 网络编排：[network.go](file:///Users/thorli/workspace/slan/server/server-biz/internal/service/impl/network.go)
- 网络模型：[network_models.go](file:///Users/thorli/workspace/slan/server/server-biz/internal/repo/network_models.go)

### 控制通道（WebSocket）稳定性与防滥用

- [ ] WS 握手严格鉴权（control session token）且可吊销
- [ ] 心跳/超时与断线收敛策略明确
- [ ] 最大消息大小限制、反序列化防护、输入校验
- [ ] 写入背压/发送队列：广播不阻塞业务线程；必要时丢弃或降级
- [ ] 连接数/速率限制（按 IP/用户/网络）

落点参考：
- WS 接入与扇出：[control_ws.go](file:///Users/thorli/workspace/slan/server/server-biz/api/http/control_ws.go)

## P1 上线前必备（稳定上线）

### 迁移与 schema 管理

- [ ] 有可重复执行的 migrations（支持新环境一键初始化）
- [ ] schema 变更有回滚/补偿策略（至少能安全回退应用版本）
- [ ] 生产不允许手工建表，发布流程可审计

### 权限模型与审计

- [ ] 明确并实现 RBAC/ACL：谁能创建网络/拉成员/发票据/创建 session
- [ ] 所有关键写操作记录审计日志（userId/ip/ua/对象/结果/时间）
- [ ] 安全事件（异常登录/暴力尝试/异常流量）可追溯

### 错误码与协议兼容

- [ ] HTTP/WS 错误码稳定且文档化（客户端可据此做 UI/重试）
- [ ] DTO/WS 消息具备版本字段或能力协商，避免升级不兼容

落点参考：
- HTTP 错误映射：[routes.go](file:///Users/thorli/workspace/slan/server/server-biz/api/http/routes.go)
- WS 消息定义：
  - [messages_handshake.go](file:///Users/thorli/workspace/slan/server/server-biz/internal/ws/messages_handshake.go)
  - [messages_topology.go](file:///Users/thorli/workspace/slan/server/server-biz/internal/ws/messages_topology.go)
  - [messages_control.go](file:///Users/thorli/workspace/slan/server/server-biz/internal/ws/messages_control.go)

### 配置与密钥管理

- [ ] 生产配置分层：dev/staging/prod，且有最小必填校验
- [ ] 密钥轮换流程（token/票据签名/数据库密码）可执行

## P2 运营与可观测（上线后能管得住）

### 指标（Metrics）

- [ ] HTTP：QPS、P95/P99、按错误码聚合
- [ ] WS：在线数、握手成功率、广播量、队列长度、丢弃数
- [ ] PG/Redis：连接池、延迟、超时、错误率

### 日志与追踪

- [ ] 结构化日志：requestId、userId、nodeId、networkId、remoteIP
- [ ] 分布式追踪：login → register device/node → join-by-owner-email/join-by-key → alias remark → switch → activate/bootstrap → ws handshake → fanout

### 告警

- [ ] Redis/PG 不可用告警
- [ ] 错误率/延迟/WS 在线数异常告警
- [ ] 入网失败率高与 IP 池耗尽告警

## P3 可靠性与扩展（规模上来不崩）

### 多实例一致性策略

- [ ] Redis pubsub 丢消息时有补偿机制（revision + 拉全量 network map）
- [ ] 大网络广播风暴控制（分片/批量/降采样/速率上限）

### 限流与防刷

- [ ] /auth、/devices/register、/nodes/register、/networks join/switch/activate、/bootstrap、/control/sessions、WS node_hello 全部限流
- [ ] 防枚举（网络 ID、设备 ID）与异常访问封禁

## 最小验收标准（建议）

- [ ] 安全：无明文/弱哈希；TLS 生效；token 可吊销并可强制下线
- [ ] 一致性：并发 join 不产生重复 virtualIP；失败可重试并最终一致
- [ ] 可运维：有健康检查、指标面板、告警；关键操作有审计记录
- [ ] 回滚：应用回滚不破坏数据；迁移具备回滚/补偿方案
