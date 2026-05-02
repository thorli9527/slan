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
  从 JSON 配置文件加载 `udp_bind`、`relay_url_prefix`、`ticket_signing_secret` 和 `mqtt`
- `--udp-bind <addr>`
  默认 `0.0.0.0:9000`
- `--relay-url-prefix <prefix>`
  默认 `udp://`
- `--no-relay-url-prefix`
  关闭 relay URL 前缀校验
- `--mqtt-enabled` / `--mqtt-disabled`
  手工启用或关闭 relay MQTT 心跳
- `--mqtt-broker-url <url>`
  relay 可访问的 MQTT broker，例如 `mqtt://127.0.0.1:1883`
- `--mqtt-topic-prefix <topic>`
  MQTT topic 前缀，默认 `slan/devices`
- `--mqtt-username-prefix <prefix>`
  MQTT 用户名前缀，默认 `slan`
- `--mqtt-password-secret <secret>`
  必须与 server-biz 的 `mqtt.password_secret` 一致
- `--relay-node-id <id>`
  当前 relay 节点 ID，必须与控制面拓扑一致
- `--relay-cluster-id <id>`
  当前 relay 所属集群 ID
- `--relay-country-code <code>`
  当前 relay 所属国家编码
- `--relay-city-code <code>`
  当前 relay 所属城市编码
- `--relay-transport <transport>`
  当前 relay 上报的传输类型
- `--relay-address <addr>`
  当前 relay 对客户端公布的地址
- `--mqtt-interval-seconds <seconds>`
  心跳上报周期
- `--mqtt-credential-ttl-seconds <seconds>`
  relay MQTT 凭据有效期

## MQTT 心跳

relay daemon 可以通过 MQTT 定时上报自身健康，用于控制面排序和运营界面展示。
JSON 配置和环境变量都支持；环境变量优先级最高。

常用环境变量：

- `SLAN_RELAY_MQTT_ENABLED`
- `SLAN_RELAY_MQTT_BROKER_URL`
- `SLAN_RELAY_MQTT_TOPIC_PREFIX`
- `SLAN_RELAY_MQTT_USERNAME_PREFIX`
- `SLAN_RELAY_MQTT_PASSWORD_SECRET`
- `SLAN_RELAY_NODE_ID`
- `SLAN_RELAY_CLUSTER_ID`
- `SLAN_RELAY_COUNTRY_CODE`
- `SLAN_RELAY_CITY_CODE`
- `SLAN_RELAY_TRANSPORT`
- `SLAN_RELAY_ADDRESS`
- `SLAN_RELAY_MQTT_INTERVAL_SECONDS`

`SLAN_RELAY_NODE_ID` 必须与控制面 relay 拓扑中的节点 ID 一致。

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
