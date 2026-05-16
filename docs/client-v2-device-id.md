# client-v2 device_id

`device_id` 是客户端设备的稳定标识。它不是安全凭证，不能替代设备密钥、登录 token 或 WireGuard key。

## 格式

- 安装阶段使用 UUID v4 算法生成。
- 示例：`f47ac10b-58cc-4372-a567-0e02b2c3d479`
- 不从硬件 SN、`MachineGuid`、`IOPlatformUUID` 或 `machine-id` 派生。

## 生成时机

桌面端 macOS / Linux / Windows 的安装脚本在服务启动前执行：

```sh
client-core-service --reset-device-id
```

该命令会生成新的 UUID v4，并写入本地 `client-v2-device-id.txt`。

服务启动和客户端登录流程只读取这个本地文件。如果文件不存在，或文件内容不是 UUID v4，服务会补生成一个 UUID v4 并持久化。

移动端 iOS / Android 在插件首次初始化时生成 UUID v4，并写入平台持久化存储：

- iOS：`UserDefaults` key `dev.slan.client.v2.ios.deviceId`
- Android：`SharedPreferences` key `dev.slan.client.v2.android.deviceId`

移动端后续启动和登录只读取该持久化值。如果 App 数据被清除、卸载重装，或旧值不是 UUID v4，则会生成新的 UUID。

## 重装语义

- 桌面端重新安装客户端会重新生成 `device_id`。
- 同一设备上服务重启不会重新生成，继续复用本地 `client-v2-device-id.txt`。
- 移动端保留 App 数据时不会重新生成；清除 App 数据或卸载重装后会重新生成。
- 克隆系统或复制安装目录后，只要重新安装一次客户端，就会得到新的 UUID，避免多个设备共用同一个 `device_id`。

## 安全边界

- `device_id` 只用于控制面识别“同一客户端安装实例”。
- 设备身份认证应依赖设备密钥对、公钥注册和服务端授权。
- 服务端只校验 `device_id` 可用性和归属，不应从格式推断平台或环境。
