# app_core 对外接入接口

本文档描述 `app_core` 从外部系统接入什么接口，以及这些接口在内部分别落到哪个 crate。

这里的“对外接入”主要指：

- 控制面 `server-biz`
- 数据面 `server-relay / DERP cluster`
- Flutter FFI 调用方

## 1. 外部依赖总览

`app_core` 不是孤立模块，它依赖三类外部接口：

### 1.1 控制面接口

来自 `server-biz`：

- `/auth/register`
- `/auth/login`
- `/auth/refresh`
- `/auth/callback-status/{callbackId}`
- `/devices/register`
- `/devices`
- `/devices/{deviceId}/networks/{networkId}/state`
- `/nodes/register`
- `/networks/home`
- `/networks`
- `/networks/join-by-key`
- `/networks/{networkId}/switch`
- `/networks/{networkId}/join`
- `/networks/{networkId}/activate`
- `/networks/{networkId}/deactivate`
- `/networks/{networkId}/attachments/{attachmentId}/remark`
- `/bootstrap`
- `/relay/tickets`
- `/control/sessions`
- MQTT control topic `{topicPrefix}/control/up` and `{topicPrefix}/control/down`

`/devices/register` now may return a device-scoped MQTT credential (`mqtt`) for
BifroMQ access. The desktop app connects MQTT only after that successful
device registration. Browser login callback delivery remains HTTP polling via
`/auth/callback-status/{callbackId}` before the device exists.

MQTT reachability and virtual network availability are not the same state. The
app reports `controlReachable`, `networkOnline`, `tunnelUp`, and `lastProbeOk`
through MQTT topic `{topicPrefix}/networks/{networkId}/state` after MQTT is
connected, with `/devices/{deviceId}/networks/{networkId}/state` kept as the
HTTP fallback. The app sends the report every 15 seconds. Before the local
tunnel is enabled it reports `networkOnline=false`; after tunnel up it reports
`networkOnline=true`. The server treats reports older than 45 seconds as
offline.

在 `app_core` 内部主要由 `crates/controller-client` 承接。

### 1.2 数据面接口

来自 `server-relay / DERP`：

- relay / DERP attach
- relay / DERP 收发
- heartbeat / probe
- 集群节点切换

在 `app_core` 内部主要由 `crates/relay-client` 承接。

### 1.3 上层调用接口

来自 Flutter：

- FFI 门面调用
- 用户动作触发的业务请求
- 上层读取运行结果和状态

在 `app_core` 内部主要由 `crates/ffi-bridge` 承接。

## 2. 控制面接入接口

### 2.1 账户与身份

由 `ControllerClient` 接入：

- `register`
- `login`
- `restore_session`
- `refresh_session`
- `register_device`
- `list_devices`
- `register_node`

### 2.2 网络与配置

由 `ControllerClient` 接入：

- `list_networks`
- `create_network`
- `join_network`
- `join_network_by_key`
- `switch_network`
- `activate_network`
- `deactivate_network`
- `set_device_network_state`
- `update_attachment_remark`
- `bootstrap`

### 2.3 relay / DERP 授权

由 `ControllerClient` 接入：

- `issue_relay_ticket`

DERP 场景下，这个接口还需要承接：

- `derp_cluster_id`
- `preferred_derp_node_ids`

## 3. 数据面接入接口

### 3.1 P2P

由 `crates/p2p` 接入：

- `PeerCandidate`
- `P2PConnector::connect`

### 3.2 Relay

由 `crates/relay-client` 接入：

- `RelayClient::connect`

### 3.3 DERP 集群

由 `crates/relay-client` 接入：

- `DerpClient`
- `DerpPool`
- `PathManager`

这些接口负责把外部 DERP 集群能力转成内部统一路径语义。

## 4. Flutter 接入接口

Flutter 当前不直接接入底层 crate，而是通过 `AppCoreFacade`。

### 4.1 Flutter 可直接调用的门面

- `register`
- `login`
- `refresh_session`
- `register_device`
- `register_node`
- `list_networks`
- `create_network`
- `join_network`
- `join_network_by_key`
- `switch_network`
- `activate_network`
- `deactivate_network`
- `update_attachment_remark`
- `bootstrap`
- `issue_relay_ticket`
- `connect`
- `disconnect`
- `control_sync`
- `control_status`
- `enable_local_network`
- `disable_local_network`
- `report_device_network_state`

### 4.1.1 Maintained startup and service boundary

In bridge/service mode, Flutter is only the UI and action trigger. Runtime
ownership sits in `app-core-service` and `app-core-helper`:

- Startup recovery calls `restore_session` to inject the persisted access token
  into the native snapshot before validation. If that token is stale, the
  client falls back to `refresh_session` when a refresh token is available.
- `enable_local_network` performs network activation, node/bootstrap reuse,
  local tunnel replacement, local DNS start, and online state reporting.
- `disable_local_network` tears down local tunnel/DNS, reports
  `networkOnline=false`, and deactivates the current device attachment.
- `app-core-service` runs periodic jobs for control sync and device network
  state reporting. Flutter does not own the MQTT heartbeat or control-sync
  timer in bridge mode.
- Flutter may keep HTTP/mock fallback paths for development, but bridge mode
  must not silently fall back to Flutter-side tunnel or DNS mutation when a
  service call fails.
- Bridge-mode UI must not expose local WireGuard IP/key/endpoint inputs as
  runtime controls. Those values are derived and applied inside the
  service/helper runtime; Flutter should show service actions and status only.

### 4.2 Flutter 暂不直接接入的接口

- `DerpClient`
- `DerpPool`
- `PathManager`
- `P2PConnector`
- `TunnelManager`

这些都属于内核内部依赖，不应直接暴露给页面层。

## 5. 各 crate 接口边界

### `crates/app-core`

- 不直接发网络请求
- 不直接建立连接
- 只定义共享模型

### `crates/controller-client`

- 接 HTTP / MQTT 控制面
- 不负责真实数据面流量

### `crates/p2p`

- 接 P2P 候选与直连逻辑
- 不负责控制面账户和网络管理

### `crates/relay-client`

- 接 relay / DERP 数据面
- 不负责 UI 和页面逻辑

### `crates/tunnel`

- 接隧道建立与关闭
- 不直接决定选哪条底层路径

### `crates/ffi-bridge`

- 接 Flutter
- 不应该自己承载复杂网络逻辑

## 6. 接口接入原则

1. 控制面接入只放在 `controller-client`
2. 数据面接入只放在 `relay-client / p2p / tunnel`
3. Flutter 只能接 `ffi-bridge`
4. 共享模型只能放在 `app-core`
