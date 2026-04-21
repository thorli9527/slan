# server-relay

Phase 1 MVP 的 Rust 中继数据面服务。

## 当前职责

- 校验控制面签发的 relay ticket
- 创建并维护 relay session
- 转发双方设备之间的 UDP 负载
- 维护会话保活与过期控制

## 文档

- [文档索引](./docs/README.md)
- [内部需求](./docs/internal-requirements.md)
- [对外输出功能](./docs/exported-capabilities.md)
- [对外接入接口](./docs/integration-interfaces.md)

## 本地运行

现在已经补了单节点 relay daemon，可直接运行：

```bash
cargo run -p relay-daemon --bin server-relay -- --udp-bind 0.0.0.0:9000
```

也可以通过配置文件运行：

```bash
cargo run -p relay-daemon --bin server-relay -- \
  --config ./configs/relay-daemon.example.json
```

可选参数：

- `--config <path>`
  从 JSON 配置文件加载 `udp_bind`、`relay_url_prefix`、`ticket_signing_secret`
- `--udp-bind <addr>`
  默认 `0.0.0.0:9000`
- `--relay-url-prefix <prefix>`
  默认 `udp://`
- `--no-relay-url-prefix`
  关闭 relay URL 前缀校验

## 当前协议

daemon 目前使用最小 UDP JSON 协议：

- `ping`
- `attach`
- `forward`
- `detach`

响应语义：

- `pong`
- `attached`
- `forwarded`
- `packet`
- `detached`
- `error`

这一版先解决“单节点可运行”，还没有引入更正式的二进制 framing。

## 仍未完成

虽然现在已经是可运行进程，但下面这些还没做：

- 多节点 / 集群 runtime
- DERP 健康探测与反馈
