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

## 双 iOS 最小回归

双 iOS 模拟器的稳定最小闭环，直接使用：

```bash
bash scripts/ios_dual_fast_check.sh
```

默认行为：

- 启动或复用两个 iOS Simulator
- 运行双端 DNS / ACL / MQTT quick 校验
- 运行双端 Flutter app 双向消息收发校验

只验证 DNS / ACL / MQTT quick：

```bash
bash scripts/ios_dual_fast_check.sh --quick-only
```

只验证双 iOS Flutter 双向消息：

```bash
bash scripts/ios_dual_fast_check.sh --message-only
```

常用环境变量：

- `SLAN_RUN_IOS_START_SIMS=0`
  复用当前已启动的模拟器，减少启动时间。
- `SLAN_IOS_SIM_A_NAME`
  指定第一个 iOS 模拟器名称。
- `SLAN_IOS_SIM_B_NAME`
  指定第二个 iOS 模拟器名称。
- `SLAN_TEST_EMAIL`
  复用固定测试账号；不设置时脚本会自动生成。
- `SLAN_TEST_PASSWORD`
  指定测试密码。

示例：

```bash
SLAN_RUN_IOS_START_SIMS=0 \
bash scripts/ios_dual_fast_check.sh --message-only
```

补充说明：

- `scripts/ios_full_business_check.sh` 现在默认也只跑 iOS 相关链路。
- 如需额外带上 `Mac+iOS` 或 `iOS+Android`，再显式打开对应环境变量。
- iOS Simulator 目前不能覆盖真实 PacketTunnel UDP/TCP 数据面，只能验证控制面、DNS/ACL 和消息面。

## 双 Android 最小回归

双 Android 模拟器的稳定最小闭环，直接使用：

```bash
bash scripts/android_dual_fast_check.sh
```

默认行为：

- 复用远程 Docker 业务栈默认地址
- 使用已验证通过的双 Android 模拟器默认序列号
- 运行双端消息、DNS、ACL、UDP/TCP 真链路稳定回归

常用环境变量：

- `SLAN_ANDROID_DEVICE_A`
  指定第一个 Android 模拟器 adb serial。
- `SLAN_ANDROID_DEVICE_B`
  指定第二个 Android 模拟器 adb serial。
- `SLAN_ANDROID_AVD_A`
  指定第一个推荐 AVD 名称。
- `SLAN_ANDROID_AVD_B`
  指定第二个推荐 AVD 名称。

示例：

```bash
SLAN_ANDROID_DEVICE_A=emulator-5554 \
SLAN_ANDROID_DEVICE_B=emulator-5556 \
bash scripts/android_dual_fast_check.sh
```

## Mac 与移动端快速回归

Mac 与 Android：

```bash
bash scripts/mac_android_fast_check.sh
```

Mac 与 iOS：

```bash
bash scripts/mac_ios_fast_check.sh
```

Mac 与 iOS 真机 UDP/TCP：

```bash
SLAN_IOS_FLUTTER_DEVICE="<real ios device id or name>" \
bash scripts/mac_ios_real_device_socket_check.sh
```

两条链路一起跑：

```bash
bash scripts/mac_android_ios_stable_check.sh
```

四条稳定链路一起跑：

```bash
bash scripts/mac_android_ios_stable_check.sh
```

补充说明：

- 日常回归统一使用新的 `*_fast_check.sh` 入口。
- `scripts/mac_android_ios_stable_check.sh` 现在会统一串行执行：
  - 双 Android
  - 双 iOS
  - Mac + Android
  - Mac + iOS

## 当前已验证结果

截至当前仓库状态，已经实跑通过的稳定链路如下：

- 双 Android：
  - 双向 client message
  - DNS / ACL
  - Android A <-> B UDP echo
  - Android A <-> B TCP echo
- 双 iOS：
  - DNS / ACL quick
  - MQTT client_message quick
  - Flutter 双向消息
- Mac + Android：
  - Mac client-core-service 启网
  - Android -> Mac UDP echo
  - Android -> Mac TCP echo
- Mac + iOS：
  - Mac client-core-service 启网
  - iOS DNS / ACL quick
  - iOS <-> Mac 双向消息

尚未在当前环境实跑通过的链路：

- Mac + iOS 真机 UDP/TCP 数据面

## 双 Docker Linux 全业务回归

两个 Docker Linux 客户端的完整业务回归，直接使用：

```bash
bash scripts/linux_dual_docker_packet_smoke.sh
```

默认行为：

- 启动两个 Linux Docker 容器客户端
- 为两个容器生成一次性 bootstrap key 并完成首次安装接入
- 校验双端 network module 是否收到 DNS / ACL 配置
- 校验双向 `client_message`
- 校验双向 `UDP` / `TCP` echo

常用环境变量：

- `SLAN_LINUX_DUAL_IMAGE`
  指定容器镜像，默认 `ubuntu:24.04`。
- `SLAN_LINUX_DUAL_KEEP_CONTAINERS=1`
  失败后保留容器，便于进入容器排查日志。
- `SLAN_LINUX_DUAL_RESULT_DIR`
  指定结果目录。
- `SLAN_LINUX_DUAL_PACKET_TESTS=1`
  开启数据面测试；`linux_dual_docker_packet_smoke.sh` 默认已开启。

补充说明：

- 脚本当前默认业务地址：
  - `SLAN_BIZ_URL=http://47.245.40.231:28080`
  - `SLAN_WEB_BASE_URL=http://47.245.40.231:28081`
- 这条链路适合验证 Docker Linux 客户端首次安装、控制面和数据面是否一起正常。

## 当前新增验证结果

截至 `2026-07-01`，已新增实跑通过：

- 双 Docker Linux：
  - 双端首次安装与 bootstrap
  - DNS / ACL 下发
  - 双向 `client_message`
  - 双向 `UDP` echo
  - 双向 `TCP` echo
  说明：脚本已补齐，但当前没有真实 iPhone / iPad，因此未执行。

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
