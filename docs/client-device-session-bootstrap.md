# Client Device Session Bootstrap

本文档定义客户端设备会话、用户会话、一次性 session key 和 `install.sh`
安装接入流程。

## Goals

- 设备身份和用户身份分离。
- 设备 session 支持自动续期。
- 用户 session 支持自动续期。
- 用户退出或自动失效时，同设备上的设备 session 一起失效。
- Web Console 可以生成一次性 session key，让无用户登录的命令行客户端接入网络。
- `install.sh` 负责下载安装客户端、写入 session key，并启动客户端服务。

## Session Model

### DeviceSession

设备 session 表示“当前设备已被服务端识别并允许保持设备连接”。

字段建议：

- `deviceId`
- `deviceToken`
- `deviceTokenExpiresAt`
- `deviceRefreshToken`
- `registeredAt`
- `lastRenewedAt`
- `activeNetworkIds`
- `mqttCredential`
- `state`: `active / expired / revoked / user_unbound`

### UserSession

用户 session 表示“当前设备上有用户登录态”。

字段建议：

- `userId`
- `accessToken`
- `refreshToken`
- `accessTokenExpiresAt`
- `refreshTokenExpiresAt`
- `loginAt`
- `lastRenewedAt`
- `boundDeviceId`
- `state`: `active / expired / revoked / logout`

### DeviceBootstrapKey

一次性设备初始化 key，由 Web Console 生成，用于脚本化接入设备。

字段建议：

- `id`
- `keyHash`
- `createdByUserId`
- `networkId`
- `deviceAlias`
- `expiresAt`
- `usedAt`
- `usedByDeviceId`
- `revokedAt`
- `status`: `unused / used / expired / revoked`
- `createdAt`

服务端只保存 key hash，不保存明文 key。

## Session Rules

- 设备先注册或 bootstrap，拿到 `DeviceSession`。
- 用户登录后，拿到 `UserSession`，并绑定当前 `deviceId`。
- `DeviceSession` 续期只依赖设备凭据。
- `UserSession` 续期只依赖用户凭据。
- 同一设备上如果用户 session 退出、过期、refresh 失败或被服务端撤销：
  - 当前 `UserSession` 失效。
  - 当前 `DeviceSession` 同时失效。
  - MQTT 断开。
  - 网络停用。
  - 本地清理用户 session 和设备 session。
- 通过 session key 初始化的设备没有 `UserSession`。
- 无用户设备只允许使用 bootstrap key 绑定时授予的网络和策略。
- session key 只能使用一次，成功使用后立即标记 `used`。

## Web Console Flow

入口建议：

```text
网络详情 -> 设备 -> 生成接入命令
```

弹窗显示：

- 一次性 session key
- 完整安装命令
- 过期时间
- 复制按钮

示例：

```bash
curl -fsSL https://staticlss.com/install.sh | sudo bash -s -- \
  --server http://api.dev.staticlss.com \
  --session-key sk_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
```

## Bootstrap Flow

1. 用户在 Web Console 创建一次性 session key。
2. 服务端生成 32 位以上随机 key，只返回一次明文。
3. 用户在目标机器执行 `install.sh` 命令。
4. `install.sh` 下载并安装客户端。
5. `install.sh` 写入 bootstrap 配置。
6. 客户端服务启动。
7. `client-core-service` 读取 `SLAN_SESSION_KEY`。
8. 客户端调用：

```text
POST /api/device/session/bootstrap
```

9. 服务端校验 key：
   - key 存在
   - 未使用
   - 未过期
   - 未撤销
   - 创建人、网络仍有效
10. 服务端完成：
   - 注册设备
   - 创建 `DeviceSession`
   - 将设备加入指定网络
   - 分配设备 IP
   - 生成 MQTT credential
   - 标记 key 为 `used`
11. 客户端保存设备 session。
12. 后续设备依赖 `DeviceSession` 自动续期，不需要用户登录。

## API Design

Web Console:

```text
POST /api/web/device-bootstrap-keys
GET  /api/web/device-bootstrap-keys
POST /api/web/device-bootstrap-keys/{id}/revoke
```

Client Device Session:

```text
POST /api/device/session/bootstrap
POST /api/device/session/register
POST /api/device/session/renew
POST /api/device/session/revoke
```

Client User Session:

```text
POST /api/user/session/login
POST /api/user/session/renew
POST /api/user/session/logout
POST /api/session/bind-device
POST /api/session/logout-current-device
```

`logout-current-device` 必须联动：

- revoke 当前 user session
- revoke 当前 device session
- 清理或作废 MQTT credential
- 广播设备下线或停用事件

## Local Client Storage

本地 session 文件拆分：

```text
client-v2-device-session.json
client-v2-user-session.json
```

通过 session key 初始化的设备只创建：

```text
client-v2-device-session.json
```

不创建：

```text
client-v2-user-session.json
```

## Client Startup Flow

1. 加载 `DeviceSession`。
2. 如果不存在或已过期：
   - 若存在 `SLAN_SESSION_KEY`，执行 bootstrap。
   - 否则执行普通设备注册或等待用户登录。
3. 加载 `UserSession`。
4. 如果用户 session 存在，执行用户 session 续期。
5. 如果设备 session 有效：
   - 续期设备 session。
   - 获取 MQTT credential。
   - 拉取设备所在网络配置。
6. 如果用户 session 也有效：
   - 允许访问用户态 API。
7. 网络启用前必须确认：
   - `DeviceSession active`
   - 网络绑定有效
   - 设备未被禁用

## Renewal Rules

- 设备 session 提前 5 分钟自动续期。
- 用户 access token 提前 1 分钟自动续期。
- 用户 refresh token 快过期时提示重新登录。
- 设备续期失败：
  - 停止 MQTT。
  - 停用网络。
  - 标记设备不可用。
- 用户续期失败：
  - 用户 session 失效。
  - 当前设备 session 同步失效。
  - 停止 MQTT。
  - 停用网络。

## install.sh Design

`install.sh` 是无用户登录设备接入入口，职责包括：

- 识别操作系统。
- 下载对应客户端安装包。
- 安装客户端。
- 写入 `SLAN_CONTROL_BASE_URL` 和 `SLAN_SESSION_KEY`。
- 启动客户端服务。
- 让 `client-core-service` 在首次启动时消费 session key。

推荐命令：

```bash
curl -fsSL https://staticlss.com/install.sh | sudo bash -s -- \
  --server http://api.dev.staticlss.com \
  --session-key sk_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
```

macOS 示例逻辑：

```bash
PKG_URL="$SERVER/downloads/slan-client/macos/latest/SLAN-Client-V2-macos.pkg"
curl -fsSL "$PKG_URL" -o /tmp/slan-client.pkg
installer -pkg /tmp/slan-client.pkg -target /
mkdir -p "/Library/Application Support/SLAN"
cat > "/Library/Application Support/SLAN/bootstrap.env" <<EOF
SLAN_CONTROL_BASE_URL=$SERVER
SLAN_SESSION_KEY=$SESSION_KEY
EOF
launchctl kickstart -k system/com.slan.client.v2 || true
```

Linux 示例逻辑：

```bash
TAR_URL="$SERVER/downloads/slan-client/linux/latest/slan-client-linux.tar.gz"
curl -fsSL "$TAR_URL" -o /tmp/slan-client-linux.tar.gz
mkdir -p /opt/slan-client
tar -xzf /tmp/slan-client-linux.tar.gz -C /opt/slan-client
mkdir -p /etc/slan
cat > /etc/slan/bootstrap.env <<EOF
SLAN_CONTROL_BASE_URL=$SERVER
SLAN_SESSION_KEY=$SESSION_KEY
EOF
/opt/slan-client/install-service.sh
systemctl enable --now slan-client-v2
```

Windows 使用独立 PowerShell 脚本：

```powershell
powershell -ExecutionPolicy Bypass -File install.ps1 `
  -Server http://api.dev.staticlss.com `
  -SessionKey sk_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
```

## Security Notes

- session key 必须是高强度随机值，建议至少 32 字节随机数再编码。
- session key 只返回一次明文。
- session key 只能使用一次。
- session key 建议默认 30 分钟有效。
- 服务端保存 hash，禁止保存明文。
- 使用成功后立即标记 used。
- 被撤销或过期的 key 不允许 bootstrap。
- bootstrap 成功后，后续认证只依赖设备 session。
- 用户态 API 不能只信 device session。
