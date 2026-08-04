# Device Token And Session Design

## Identity

Device 是客户端唯一身份主体。授权 key 用于一次激活或受控重新激活，不直接作为业务 API 的
长期 Bearer token。Customer 和 Operator 均不能替代 Device 访问 App API。

## Exchange

1. Ops 创建授权 key，服务端只保存带 pepper 的 HMAC-SHA256 摘要。
2. 客户端调用 `POST /api/device-auth/token`，提交 `key` 和可选 `deviceId`。
3. 服务端校验摘要、状态、有效期和设备绑定。
4. 首次交换未绑定 key 时，服务端创建设备并将 key 原子绑定到该设备。
5. 返回 device token、refresh token、MQTT 凭据和设备 profile。

## Session Model

- `sessionId`：设备会话唯一标识。
- `deviceId`：会话唯一主体。
- `credentialId`：签发该会话的授权 key 记录。
- `accessToken`：短期 Device Bearer。
- `refreshToken`：用于续期并执行 rotation。
- `expiresAt` / `refreshExpiry`：访问和续期有效期。
- `status` / `revokedAt`：吊销状态。

## Rotation And Revocation

- 每次成功续期生成新的 access/refresh token。
- 前一个 refresh token 只允许幂等重试窗口，不得再次生成不同 token family。
- 吊销授权 key 时撤销其全部活动 Device session。
- 禁用或删除 Device 时撤销该设备全部 session，并停止 MQTT 和网络访问。
- MQTT credential 的 username 和 HMAC 均绑定 `deviceId`、`credentialId` 与到期时间，有效期不得超过 Device session 或授权 key。
- 吊销授权 key 后，关联 HTTP session 和该 key 签发的 MQTT credential 均立即失效。
- Refresh token 每次续期都轮换；数据库使用条件替换保证同一旧 token 只能被消费一次。并发重试在短暂宽限期内返回已生效的同一组新凭据，不会再次轮换。

## Security Requirements

- token 和完整授权 key 不写日志，不进入 URL query。
- 服务端只保存 key 摘要；pepper 由部署密钥管理。
- key 交换按来源 IP、key ID 和失败次数限流并写审计事件。
- 新授权 Key 只使用 `SLAN_DEVICE_CREDENTIAL_PEPPER` 生成摘要。`SLAN_DEVICE_CREDENTIAL_PREVIOUS_PEPPERS` 最多保留 3 个历史 pepper，仅用于校验旧 Key；命中的 pepper 不写入数据库、响应或日志。
- Device Bearer 必须同时校验 session、Device 状态和资源成员关系。
- Device 全局虚拟 IP 由 PostgreSQL 原子计数器分配，非空 `virtual_ip` 受部分唯一索引保护；计数器失败或地址池耗尽时拒绝分配，不回退到固定地址。
- 不迁移或兼容旧账号 token、账号 session 和账号资源归属。
- Operator 新密码使用 bcrypt 存储；旧 `plain:` / `sha256:` 仅保留校验兼容，不再签发。
