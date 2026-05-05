# client-v2 device_id

`device_id` 是客户端设备的稳定标识。它不是安全凭证，不能替代设备密钥、登录 token 或 WireGuard key。

## 格式

- 不使用业务前缀。
- 当前客户端生成 16 位小写十六进制字符串。
- 示例：`4f2a91c0e8b7d613`

## 稳定来源

桌面端优先使用系统/硬件稳定锚点派生：

- Windows：`MachineGuid`
- macOS：`IOPlatformUUID`
- Linux：`/etc/machine-id` 或 `/var/lib/dbus/machine-id`

客户端会把派生结果写入本地 `client-v2-device-id.txt`。如果本地文件与当前硬件派生结果不一致，以硬件派生结果为准并覆盖本地文件。

## 重装语义

- 删除软件后重装，硬件/系统锚点不变时，`device_id` 不变。
- 重装系统、换主板、虚拟机克隆或系统锚点变化时，`device_id` 可能变化。
- 移动端通常不能读取硬件唯一 ID，后续应使用平台安全存储尽量保持稳定，但无法保证卸载重装后不变。

## 安全边界

- `device_id` 只用于控制面识别“同一设备”。
- 设备身份认证应依赖设备密钥对、公钥注册和服务端授权。
- 服务端只校验 `device_id` 可用性和归属，不应从格式前缀推断平台或环境。
