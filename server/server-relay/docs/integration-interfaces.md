# server-relay 对外接入接口

本文档描述 `server-relay` 需要接入哪些外部输入，以及这些接入点分别落在哪个 crate。

## 1. 外部接入总览

`server-relay` 主要接入两类外部输入：

- 客户端的数据面连接
- 控制面定义的 ticket 语义

## 2. ticket 接入

ticket 语义当前由控制面定义，`server-relay` 只负责消费。

当前接入模型：

- `RelayTicket`

当前接入校验：

- `crates/auth`
- `StaticTicketValidator`

## 3. session 接入

当前 session 存储边界在：

- `crates/session`

对外抽象：

- `SessionStore`

当前默认实现：

- `InMemorySessionStore`

## 4. 数据面接入

当前 UDP 中继接入边界在：

- `crates/udp-relay`

主要入口：

- `attach`
- `attach_with_ticket`
- `session`
- `detach`
- `forward`

这代表数据面真正对外接入的核心是“attach + forward”。

## 5. 与客户端的边界

客户端通过以下语义接入：

1. 拿到控制面 ticket
2. 连接到 relay 地址
3. attach session
4. 开始收发数据

## 6. 与控制面的边界

`server-relay` 当前不直接调用 `server-biz`。

两者的边界通过这些协议语义建立：

- `RelayTicket`
- `session_id`
- `network_id`
- `src_node_id / dst_node_id`

在集群化之后，还应增加：

- `derp_cluster_id`
- `allowed_derp_node_ids`

## 7. 接入原则

1. 控制面通过 ticket 定义语义，不直接侵入数据面实现
2. 客户端通过 attach / forward 接入，不直接操作存储
3. session store 可替换，但边界应保持稳定
