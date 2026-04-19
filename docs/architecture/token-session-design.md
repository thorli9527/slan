# Token / 会话设计（生产建议）

本文档给出 `server-biz` 在生产环境的 token 与会话管理建议，用于落地以下目标：

- access/refresh TTL 可配置
- refresh rotation + reuse detection
- 会话吊销：单用户/单设备/单会话立即生效
- token 与设备/客户端信息绑定（风控与审计）
- Redis 故障策略明确且可控

## 当前实现概览（便于对照）

- token 形态：opaque token（随机串）写入 Redis
- access/refresh：以 key-value 形式保存 `token -> userId`，并依赖 TTL 过期
- 认证：仅校验 `access_token:{token}` 是否存在并取回 userId

落点参考：
- token 签发与写入：[db_access.go](file:///Users/thorli/workspace/slan/server/server-biz/internal/service/db_access.go)
- Redis token store：[redis.go](file:///Users/thorli/workspace/slan/server/server-biz/internal/repo/redis.go)

## 1. TTL 配置化

建议在配置中增加 token 相关项（示例）：

- accessTTLSeconds
- refreshTTLSeconds
- controlSessionTTLSeconds

并统一由 service 层读取配置注入到签发逻辑，避免散落 `time.Hour` 常量。

## 2. 会话模型（Session-first，而不是 Token-first）

建议把“会话”作为一等实体，token 只是会话的凭证：

- sessionId：一次登录/一次设备会话的唯一标识
- accessToken：短期，频繁使用
- refreshToken：长期，仅用于换取新的 access/refresh

建议 Redis key 结构（示例）：

- `sess:{sessionId}` -> JSON(sessionMeta)（TTL=refreshTTL）
- `access:{accessToken}` -> sessionId（TTL=accessTTL）
- `refresh:{refreshToken}` -> sessionId（TTL=refreshTTL）

其中 sessionMeta 建议包含：

- userId
- deviceId（可选但强烈建议）
- clientId / appVersion / platform（用于审计与风控）
- uaHash / ipHash（可选，用于风控）
- createdAt / lastSeenAt
- revokedAt（或单独 revoke set）

## 3. Refresh rotation + reuse detection

### rotation（每次刷新都换 refresh）

刷新接口（建议新增）：`POST /auth/refresh`

- 输入：refreshToken（建议放在 httpOnly cookie 或请求体）
- 输出：新的 accessToken + 新的 refreshToken

刷新流程（关键点）：

1. 查 `refresh:{oldToken}` 得到 sessionId
2. 校验 session 未被吊销
3. 生成 `newAccess`、`newRefresh`
4. 写入 `access:{newAccess}` 与 `refresh:{newRefresh}`
5. 删除 `refresh:{oldToken}`（或标记旧 token 已用）

### reuse detection（旧 refresh 被再次使用）

生产环境必须处理“旧 refresh 泄露被重放”的场景。推荐两种方式：

**方式 A：一次性 refresh（推荐）**

- 刷新成功后删除 `refresh:{oldToken}`
- 若再次使用同一个 oldToken，则查不到 key，直接判定为非法刷新
- 同时对该 session 触发吊销（防止已泄露）

**方式 B：refresh family 版本号**

- `sess:{sessionId}` 中维护 `refreshVersion`
- 每次刷新 `refreshVersion++` 并把 version 写入 refresh token 对应的记录
- 若请求带来的 refreshVersion < 当前版本，判定 reuse → 吊销 session

## 4. 吊销策略（单用户/单设备/单会话）

建议支持三类吊销能力：

- 单会话吊销：删除 `sess:{sessionId}`，并删除会话关联的 token key
- 单设备吊销：枚举该 deviceId 下所有 sessionId 并吊销
- 单用户吊销：枚举该 userId 下所有 sessionId 并吊销

为避免枚举成本过高，建议维护索引集合（示例）：

- `user_sessions:{userId}` -> set(sessionId)
- `device_sessions:{deviceId}` -> set(sessionId)

吊销动作要求“立即生效”：

- access token 校验必须能反查 session 并确认未 revoked
- 或在吊销时立即删除所有 `access:{token}` 映射（需要维护 token 列表）

## 5. Token 绑定设备/客户端信息

目标：降低 token 被窃取后跨设备滥用的风险，并为审计提供依据。

建议最小绑定集合：

- deviceId（强绑定，适用于多端同设备会话）
- platform/appVersion（弱绑定，用于异常检测）

实施方式：

- 登录/刷新时都把绑定信息写入 `sess:{sessionId}`
- 认证中间件在解析 accessToken 后取出 sessionMeta，校验：
  - userId 匹配
  - deviceId（如果请求头/客户端上报带 deviceId）一致
  - 可选：uaHash/ipHash 在允许窗口内（风控策略）

## 6. Redis 故障策略（拒绝/降级/超时）

token 与控制会话强依赖 Redis 时，建议默认策略为：

- Redis 不可用：鉴权相关接口返回 503（拒绝），避免出现“无鉴权放行”的灾难性降级

同时要求：

- Redis 调用必须配置超时（context deadline）
- 失败重试有上限，并带抖动（避免雪崩）
- 对关键路径（鉴权/刷新）与非关键路径（lastSeenAt）区别对待

## 7. 接口清单（建议补齐）

- `POST /auth/login`：支持可选 deviceId/clientInfo 字段写入 session
- `POST /auth/refresh`：refresh rotation
- `POST /auth/logout`：吊销当前 session
- `POST /admin/users/{userId}/sessions/revoke`：吊销用户全部会话（运营后台/管理 API）
- `POST /admin/devices/{deviceId}/sessions/revoke`：吊销设备全部会话

