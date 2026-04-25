# app_core

SLAN Phase 1 的 Rust 应用核心层。它负责把桌面客户端、CLI、Flutter
plugin bridge 和控制面 `server-biz` 串起来。

## 职责

- 对接 `server-biz` 的认证、设备、节点、网络、bootstrap、relay ticket 接口。
- 维护本机 session、device、node、当前网络、bootstrap、连接路径和隧道快照。
- 支持创建网络、加入网络、设备别名、切换网络、激活/停用网络。
- 执行 NAT/P2P/DERP/relay fallback 和 tunnel runtime 编排。
- 通过 `ffi-bridge` 和 JSON-line helper 向 Flutter 暴露稳定门面。

## Crates

- `app-core`: 共享领域模型。
- `app-core-cli`: 无界面命令行客户端。
- `app-core-helper`: Flutter/native plugin 可启动的 helper 进程。
- `app-core-service`: helper TCP service 入口。
- `controller-client`: 控制面 HTTP client trait、DTO 和 transport。
- `control-ws-client`: 控制 WebSocket client。
- `ffi-bridge`: 面向 Flutter/JSON bridge 的统一 facade。
- `nat`, `p2p`, `relay-client`, `tunnel`: 连接路径与本地隧道 runtime。

## CLI 主流程

无界面环境可以直接使用 `app-core-cli`：

```bash
cargo run -p app-core-cli -- --help
```

常见端到端流程：

```bash
cargo run -p app-core-cli -- \
  --control-base-url http://127.0.0.1:8080 \
  auth login --email user@example.com --password password123

cargo run -p app-core-cli -- \
  device register \
  --name thor-mac \
  --platform macos \
  --machine-id machine-1 \
  --public-key pubkey-1

cargo run -p app-core-cli -- \
  node register \
  --node-id node-1 \
  --node-public-key node-pubkey-1

cargo run -p app-core-cli -- \
  network create --name home --cidr 100.64.0.0/24

cargo run -p app-core-cli -- \
  network join-by-owner-email \
  --owner-email owner@example.com \
  --alias "Thor laptop"

cargo run -p app-core-cli -- \
  network join-by-key \
  --join-key team-alpha-join-key \
  --alias "Thor laptop"

cargo run -p app-core-cli -- \
  network switch --network-id net-1

cargo run -p app-core-cli -- \
  bootstrap --network-id net-1

cargo run -p app-core-cli -- \
  connect --peer-node-id peer-1
```

可用的网络子命令：

- `network list`
- `network create`
- `network join`
- `network join-by-owner-email`
- `network join-by-key`
- `network remark`
- `network activate`
- `network switch`
- `network deactivate`

说明：

- 默认状态文件在 `~/.slan/app-core-cli-state.json`。
- 可用 `--state-file` 覆盖状态文件位置。
- 可用 `--json` 输出 JSON，方便脚本消费。
- `network switch` 调用控制面的 `/networks/{networkId}/switch`，并更新本地当前网络。
- `network activate` 调用 `/networks/{networkId}/activate`，适合显式恢复当前设备在目标网络的接入。
- `network join-by-owner-email` 和 `network join-by-key` 的 `--alias` 会写入 attachment remark。

## 测试

`app-core-cli` 黑盒测试会启动 fake control server，并用真实 CLI 二进制固化主链路：

- `auth login`
- `device register`
- `node register`
- `network create`
- `network join-by-owner-email`
- `network join-by-key`
- `network remark`
- `network activate`
- `network switch`
- `network deactivate`
- `bootstrap`
- `connect` / relay fallback
- `send` / `probe`
- `disconnect`
- `status` 和状态文件持久化

运行方式：

```bash
cargo test -p app-core-cli
```

全量 Rust 验证：

```bash
cargo test
```

## 文档

- [文档索引](./docs/README.md)
- [内部需求](./docs/internal-requirements.md)
- [对外输出能力](./docs/exported-capabilities.md)
- [对外接入接口](./docs/integration-interfaces.md)
