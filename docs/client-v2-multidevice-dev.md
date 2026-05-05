# client-v2 多端联调

目标：在一台 Mac 上同时调试 macOS 桌面、iOS、Android，并让它们连接同一个 `client-core-service`。

## 网络模型

`client-core-service` 默认只监听：

- `127.0.0.1:46392`

多端联调时需要改成开发监听：

- `0.0.0.0:46392`

各端访问地址不同：

- macOS 桌面：`127.0.0.1:46392`
- iOS Simulator：默认使用 Mac 局域网 IP，例如 `192.168.x.x:46392`
- Android Emulator：`10.0.2.2:46392`
- iOS/Android 真机：Mac 局域网 IP，例如 `192.168.x.x:46392`

生产默认不变，仍应使用 loopback。

## 一键启动

```bash
./scripts/client_multidevice_dev.sh
```

默认行为：

- 启动 `client-core-service`，监听 `0.0.0.0:46392`
- 启动 macOS app，连接 `127.0.0.1:46392`
- 启动 iOS app，连接 Mac 局域网 IP
- 启动 Android app，连接 `10.0.2.2:46392`

## 常用参数

只启动部分端：

```bash
SLAN_MULTI_TARGETS=macos,android ./scripts/client_multidevice_dev.sh
```

指定设备：

```bash
SLAN_MULTI_IOS_DEVICE="iPhone 15" \
SLAN_MULTI_ANDROID_DEVICE="emulator-5554" \
./scripts/client_multidevice_dev.sh
```

真机联调：

```bash
SLAN_MULTI_IOS_SERVICE_HOST="192.168.1.10:46392" \
SLAN_MULTI_ANDROID_SERVICE_HOST="192.168.1.10:46392" \
./scripts/client_multidevice_dev.sh
```

复用已启动的服务：

```bash
SLAN_MULTI_START_SERVICE=0 \
SLAN_CLIENT_CORE_SERVICE_BIND=0.0.0.0:46392 \
./scripts/client_multidevice_dev.sh
```

## Flutter 配置入口

Flutter 端优先读取：

- `--dart-define=SLAN_CLIENT_CORE_SERVICE_HOST=host:port`

其次读取：

- `SLAN_CLIENT_CORE_SERVICE_HOST`

最后 fallback：

- `127.0.0.1:46392`

移动端优先使用 `--dart-define`，不要依赖进程环境变量。

## 前置条件

- `flutter devices` 能看到目标设备。
- iOS 需要 Xcode / Simulator 或真机签名配置。
- Android 需要 Android SDK / Emulator 或 USB 调试。
- Mac 防火墙需要允许外部设备访问 `46392`。
- 服务必须监听 `0.0.0.0:46392`，否则 Android 模拟器和真机无法访问。
