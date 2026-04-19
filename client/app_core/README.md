# app_core

SLAN Phase 1 的 Rust 应用核心层。

## 作用

- 对接 `server-biz` 的控制面接口
- 完成设备注册与启动配置拉取
- 执行 NAT 类型探测
- 优先尝试 P2P 连接
- 在直连失败时请求 relay 回退
- 向 Flutter 暴露稳定的门面接口

## crate 划分

- `app-core`：共享领域模型定义
- `app-core-cli`：无界面命令行客户端
- `controller-client`：控制面客户端 trait 与请求模型
- `nat`：NAT 探测抽象
- `p2p`：点对点连接抽象
- `relay-client`：relay 回退连接抽象
- `tunnel`：加密隧道抽象
- `ffi-bridge`：面向 Flutter 的统一门面

## CLI

无界面环境可以直接使用 `app-core-cli`，不依赖 Flutter/plugin：

```bash
cargo run -p app-core-cli -- --help
```

常见用法：

```bash
cargo run -p app-core-cli -- \
  --control-base-url http://127.0.0.1:8080 \
  auth login --email user@example.com --password password123

cargo run -p app-core-cli -- network list

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

cargo run -p app-core-cli -- bootstrap --network-id net-1

cargo run -p app-core-cli -- connect --peer-node-id peer-1
```

说明：

- 默认状态文件在 `~/.slan/app-core-cli-state.json`
- 可用 `--state-file` 覆盖状态文件位置
- 可用 `--json` 输出 JSON，方便脚本消费

## CLI 测试

`app-core-cli` 已有黑盒集成测试，会起一个 fake control server，并用真实 CLI 二进制固化这条链路：

- `auth login`
- `device register`
- `node register`
- `bootstrap`
- `connect`（触发 relay fallback）
- `disconnect`
- `status` / 状态文件持久化

也覆盖这些失败路径：

- 缺 session 时直接 `device register`
- 缺 node 时直接 `bootstrap`
- `bootstrap` 前直接 `connect`
- control plane 返回非 2xx

运行方式：

```bash
cargo test -p app-core-cli
```

## 文档

- [文档索引](./docs/README.md)
- [内部需求](./docs/internal-requirements.md)
- [对外输出功能](./docs/exported-capabilities.md)
- [对外接入接口](./docs/integration-interfaces.md)
