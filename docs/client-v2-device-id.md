# client-v2 device_id

`device_id` 是客户端设备的稳定标识。它不是安全凭证，不能替代设备密钥、登录 token 或 WireGuard key。

## 格式

- 安装阶段使用 UUID v4 算法生成。
- 示例：`f47ac10b-58cc-4372-a567-0e02b2c3d479`
- 不从硬件 SN、`MachineGuid`、`IOPlatformUUID` 或 `machine-id` 派生。

## 生成时机

桌面端 macOS / Linux / Windows 的安装脚本在服务启动前执行：

```sh
client-core-service --ensure-device-id
```

该命令只在本地 `config.json` 不存在，或其中 `deviceId` 不是 UUID v4 时生成新的 UUID v4。正常安装、升级、覆盖安装都不会重置已有 `device_id`。

服务启动和设备激活流程只读取这个统一配置文件。如果文件不存在，或 `deviceId` 不是 UUID v4，服务会补生成一个 UUID v4 并持久化。

客户端通过 Ops 签发的授权 key 激活设备。授权 key、Device token、refresh token 和 MQTT 凭据与设备状态一起加密保存，不得使用 `deviceId` 派生或替代任何授权凭据。

移动端 iOS / Android 在插件首次初始化时生成 UUID v4，并写入平台持久化存储：

- iOS：`UserDefaults` key `dev.slan.client.v2.ios.deviceId`
- Android：`SharedPreferences` key `dev.slan.client.v2.android.deviceId`

移动端后续启动和激活只读取该持久化值。如果 App 数据被清除、卸载重装，或旧值不是 UUID v4，则会生成新的 UUID。

## 重装语义

- 桌面端升级安装和覆盖安装不会重新生成 `device_id`，继续复用本地 `config.json`。
- 同一设备上服务重启、UI 重启不会重新生成。
- 只有完整清理本地状态目录、文件损坏、文件丢失，或运维显式执行 `client-core-service --reset-device-id` 时才会生成新的 UUID。
- 设备凭证跟随状态目录保存。升级和覆盖安装必须保留；完整清理状态目录后会重新生成，服务端会把它视为新的设备身份。
- 移动端保留 App 数据时不会重新生成；清除 App 数据或卸载重装后会重新生成。
- 克隆系统或复制安装目录后，如果连同状态目录一起复制，可能带来重复 `device_id`。生产验收必须覆盖多设备同时安装、升级、卸载重装矩阵，并由服务端检测重复设备身份和异常 session 迁移。

## 安全边界

- `device_id` 只用于控制面识别“同一客户端安装实例”。
- `config.json` 只明文保存 `deviceId` 和加密格式元数据；设备公钥、Device token、refresh token、MQTT 凭据和网络会话均位于 AES-256-GCM 密文中。加密密钥由 `deviceId` 前 20 位经 SHA-256 派生，因此主要防止配置内容被直接读取，不等同于操作系统密钥链提供的本机强隔离。
- 设备身份认证应依赖设备密钥对、公钥注册和服务端授权。
- 服务端必须通过授权 key 摘要、绑定状态和 Device session 验证设备身份，不能接受 `deviceId` 本身作为凭据。
- 服务端只校验 `device_id` 可用性，不应从格式推断平台、客户或环境。
