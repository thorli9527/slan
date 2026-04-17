# server-relay 对外输出功能

本文档描述 `server-relay` 对客户端和外部系统输出什么能力。

## 1. 输出目标

`server-relay` 对外输出的是：

- 可消费的中继接入点
- 可 attach 的 session 语义
- 可用的数据转发能力

它不对外输出用户管理、网络管理或控制面编排能力。

## 2. 当前对外输出能力

### 2.1 ticket 消费能力

客户端拿到控制面签发的 `RelayTicket` 后，可以接入 `server-relay`。

### 2.2 session 建立能力

客户端通过 ticket attach 后，可以进入某个 relay session。

### 2.3 UDP 转发能力

当前主要输出：

- UDP 中继转发
- 会话另一端寻址
- 非参与方拒绝

### 2.4 错误语义

当前主要错误输出：

- `InvalidTicket`
- `TicketExpired`
- `SessionNotFound`
- `SessionAlreadyExists`
- `UnauthorizedPeer`
- `EmptyPayload`
- `ClockSkew`
- `Store`

## 3. 输出给谁

### 3.1 输出给客户端

主要是 `client/app_core` 的 relay / DERP 客户端。

客户端依赖 `server-relay` 获取：

- attach 成功
- session 可用
- 数据可被转发

### 3.2 输出给控制面语义

虽然 `server-relay` 不直接服务控制面业务，但它必须遵守控制面定义的 ticket 和 session 语义。

## 4. 后续建议扩展的输出

在 DERP 集群场景下，后续应扩展：

- cluster-aware ticket 消费能力
- 多节点 attach 结果
- 探测和保活响应
- 更丰富的节点状态输出

## 5. 对外输出原则

1. 客户端只消费接入和转发能力
2. 控制面只通过 ticket 语义约束数据面
3. 数据面不暴露内部存储与实现细节
