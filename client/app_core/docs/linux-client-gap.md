# Linux 客户端缺口梳理

## 已具备

- `app-core-cli` 已覆盖控制台接入端主流程：认证、设备注册、节点注册、网络创建/加入/切换/启停、bootstrap、connect、relay ticket、send/probe、状态持久化。
- `app-core-helper` 已提供 JSON-line helper 门面，可被 Flutter Linux plugin 或脚本调用。
- `app-core-service` 已提供 TCP helper service 入口。
- `tunnel` crate 已有 Linux kernel backend，依赖 `iproute2`、`wireguard-tools` 和 `/dev/net/tun`。
- `client/app_core/Dockerfile.linux-client` 可在 Docker 内生成 Linux CLI 客户端镜像。
- `client/app_core/scripts/linux-helper-smoke.sh` 可在具备 `NET_ADMIN` 和 `/dev/net/tun` 的容器里验证 Linux helper 能创建 WireGuard 接口。

## 仍缺少

- Flutter Linux GUI 还没有完整 Docker 化构建链路；目前 Docker 镜像定位为控制台/接入端，不是桌面 GUI 包。
- Linux 桌面 plugin 的端到端 UI 自动化还未接到 Docker smoke；现有 smoke 验证 helper/CLI，不启动 Flutter 窗口。
- 真机级网络联通测试还缺一组双容器/双节点场景：当前 smoke 验证接口创建和 WireGuard 配置，不验证跨节点真实收发包。
- MQTT gateway 默认未启用；Docker Linux 客户端可使用 HTTP 主链路，MQTT 设备状态上报需要启用 `rocketmq-mqtt` profile 并提供可用镜像。
- 容器内启用真实隧道需要运行参数：`--cap-add NET_ADMIN --device /dev/net/tun`。无这些权限时只能跑 CLI/doctor/dry-run。

## Docker 验证入口

构建：

```bash
docker build -f client/app_core/Dockerfile.linux-client -t slan-linux-client:local client/app_core
```

CLI smoke：

```bash
docker run --rm slan-linux-client:local --json doctor
docker run --rm slan-linux-client:local --json platform detect
docker run --rm slan-linux-client:local --json platform plan
```

Linux helper 隧道 smoke：

```bash
docker run --rm --cap-add NET_ADMIN --device /dev/net/tun \
  --entrypoint /app/scripts/linux-helper-smoke.sh \
  slan-linux-client:local
```
