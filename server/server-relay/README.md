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
cargo run -p relay-daemon --bin server-relay -- --udp-bind 0.0.0.0:9000 --tcp-bind 0.0.0.0:9001
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
- `--tcp-bind <addr>`
  可选 TCP relay 监听地址；启用后 TCP 使用 `4-byte length + payload` 承载同一套 JSON 控制包和二进制 data frame
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
  当前 relay 上报的传输类型；本 daemon 目前实际数据面支持 `udp` 和 `tcp`，`tls` / `http3` 已保留在控制面协议中但不会被此 daemon 伪装成可用 listener
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
- `SLAN_RELAY_TCP_BIND`

`SLAN_RELAY_NODE_ID` 必须与控制面 relay 拓扑中的节点 ID 一致。
如果同一个 daemon 同时开放 UDP/TCP listener，推荐在 JSON 配置的 `mqtt.nodes` 中为每个
真实 listener 配置一个独立节点；未填写节点级 `cluster_id` / `country_code` / `city_code`
时会继承 MQTT 顶层区域信息。环境变量仍保持旧的单节点模式，适合只上报一个 listener。

## 当前协议

daemon 使用 UDP JSON 协议处理控制面；启用 `tcp_bind` 后，TCP 也支持同一套控制面，
外层使用 4 字节大端长度前缀：

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

JSON 控制面负责建立会话绑定：客户端必须先 `attach`，daemon 会把
`session_id + participant_id` 绑定到当前 relay endpoint。UDP endpoint 是源地址，
TCP endpoint 是当前连接，连接断开时会自动清理绑定。

数据面使用最小二进制 frame。Windows / Android / Linux / iOS / macOS 后续都应该复用
这个 frame，只把各平台的 TUN/VPN 读写封装在本地适配层：

| 字段 | 长度 | 说明 |
| --- | ---: | --- |
| magic | 4 | 固定 `SLAN` |
| version | 1 | 当前 `1` |
| type | 1 | `1` 表示 data |
| header_len | 2 | 当前 `32`，大端 |
| seq | 8 | 客户端递增序号，大端 |
| config_hash | 8 | 客户端网络配置 hash，大端 |
| payload_len | 4 | 原始 IP 包长度，大端 |
| reserved | 4 | 保留 |
| payload | N | 原始 IP 包 |

收到二进制 frame 时，daemon 不再解 JSON / base64，而是根据之前 `attach` 建立的
endpoint 索引找到会话和对端，把原始 IP 包重新封装成同版本二进制 frame 发给 peer。
这使控制面保持可读、可调试，数据面走低开销二进制路径。

当前数据面真实可用传输：

- `relay_udp`
- `relay_tcp`

控制面和服务端统计已经预留：

- `direct_udp`
- `relay_udp`
- `relay_tcp`
- `relay_http3`
- `relay_tls`

其中 `relay_http3` / `relay_tls` 需要独立 listener、握手探测和证书/QUIC 配置后才能在
daemon 心跳中声明为健康可用。

## 仍未完成

虽然现在已经是可运行进程，但下面这些还没做：

- 多节点 / 集群 runtime
- DERP 健康探测与反馈
- TLS listener / HTTP3 listener
- 数据面 frame 的认证、重放保护、加密和压缩能力协商
