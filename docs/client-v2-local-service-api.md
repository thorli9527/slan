# client-v2 local service API

`client-core-service` 当前对 UI 暴露的是本地 JSON line RPC：

- 默认地址：`127.0.0.1:46392`
- 请求格式：`{"method":"localStatus","args":{}}\n`
- 响应格式：单行 JSON

后续如果增加 HTTP facade，可以一一映射为 `GET /local/*` 与 `POST /local/*`。

## 第一批稳定接口

### `GET /local/status`

RPC method: `localStatus`

用途：只读本地服务总览，不要求 UI 解析完整 `state`。

返回字段：

- `service`
- `version`
- `signedIn`
- `deviceId`
- `selfNodeId`
- `activeNetworkId`
- `virtualIp`
- `networkEnabled`
- `switchEnabled`
- `syncing`
- `syncReason`
- `activePath`
- `peerCount`
- `relayCandidateCount`
- `connectPlanCount`
- `error`
- `runtimeError`

### `GET /local/peers`

RPC method: `localPeers`

用途：只读本地路径运行态，面向 UI、诊断和后续自动 fallback 可视化。

返回字段：

- `items[].peerNodeId`
- `items[].peerVirtualIps`
- `items[].activePath`
- `items[].candidates`

`candidates` 直接复用 core 的 `PathCandidate` JSON：

- `kind`
- `state`
- `endpointId`
- `address`
- `sessionId`
- `transport`
- `rttMs`
- `pathScore`
- `lastOkAtMs`
- `lastError`

### `GET /local/path-plan`

RPC method: `localPathPlan`

用途：读取最近有效 connect plan，用于展示服务端/控制面下发后的本地路径计划。

返回字段：

- `items[].peerNodeId`
- `items[].preferDirect`
- `items[].paths`
- `items[].relayTicket`
- `items[].updatedAtMs`

### `GET /local/session`

RPC method: `localSession`

用途：读取脱敏后的本地登录与网络会话摘要，给 UI 判断登录态、设备绑定、过期状态，不返回任何 credential。

返回字段：

- `signedIn`
- `expired`
- `userId`
- `userLabel`
- `deviceId`
- `selfNodeId`
- `activeNetworkId`
- `virtualIp`
- `relayCandidateCount`
- `mqttConfigured`
- `expiresIn`
- `authenticatedAtMs`
- `expiresAtMs`

明确不返回：

- `accessToken`
- `refreshToken`
- `mqtt.username`
- `mqtt.password`

### `GET /local/path-diagnose`

RPC method: `localPathDiagnose`

用途：读取本地路径诊断视图，聚合 direct、relay、MTU、DNS 和健康原因。

### `GET /local/relay-candidates`

RPC method: `localRelayCandidates`

用途：读取本地 relay 候选与选择结果。

### `POST /local/relay-candidates/refresh`

RPC method: `localRefreshRelayCandidates`

用途：触发一次 relay candidate 重新选择后返回结果。

### `GET /local/control/status`

RPC method: `localControlStatus`

用途：读取本地控制面同步状态。

### `GET /local/control/plan`

RPC method: `localControlPlan`

用途：读取本地控制面 transport 计划，用于排查 MQTT/HTTP fallback 选择。

## 可以继续抽出来的接口

### 状态与会话

- `GET /local/status` -> `localStatus`
- `GET /local/session` -> `localSession`
- `POST /local/logout` -> 当前 `logout`

### Peer 与路径

- `GET /local/peers` -> `localPeers`
- `GET /local/path-plan` -> `localPathPlan`
- `GET /local/path-diagnose` -> `localPathDiagnose`
- `POST /local/path/probe` -> 后续触发一次主动路径探测
- `POST /local/path/select` -> 后续手动切换 active path，调试用，默认不开放给普通 UI

### Relay / DERP

- `GET /local/relay-candidates` -> `localRelayCandidates`
- `POST /local/relay-candidates/refresh` -> `localRefreshRelayCandidates`
- `POST /local/relay/prepare` -> 当前 `prepareRelayDataPlane`
- `GET /local/derp` -> 后续暴露 DERP map / candidate / ticket 状态

### Control sync

- `GET /local/control/status` -> `localControlStatus`
- `GET /local/control/plan` -> `localControlPlan`
- `GET /local/control/outbox` -> 当前 `controlTransportOutbox`
- `POST /local/control/ack` -> 当前 `markControlAcked`

### 网络开关

- `POST /local/network/activate` -> 当前 `activateNetwork`
- `POST /local/network/deactivate` -> 当前 `deactivateNetwork`
- `POST /local/network/shutdown` -> 当前 `shutdownNetwork`
- `GET /local/android/network-config` -> 当前 `androidNetworkConfig`

### 观测与事件

- `GET /local/events/watch` -> 当前 `watchBusinessEvent`
- `GET /local/state/watch` -> 当前 `watchState`
- `GET /local/diagnostics/export` -> 当前 `exportDiagnostics`

## 接口化原则

- UI 新代码优先调用 `local*` 稳定接口，不再直接依赖完整 `state`。
- `state` 保留为兼容接口，只用于老 UI 和兜底。
- 本地服务返回的只读接口不携带 token、ticket secret、refresh token。
- 控制类接口必须继续走明确 method，不要通过泛化 `dispatch` 暴露给 UI。
