# server-relay 内部需求

本文档描述 `server/server-relay` 作为数据面的内部职责、crate 划分和后续必须承接的能力。

## 1. 定位

`server-relay` 是数据面中继服务，负责：

- 校验控制面签发的 ticket
- 创建和维护 relay session
- 转发会话双方的数据包
- 保活与过期控制

它不负责用户、网络、设备和成员管理。

## 2. 内部核心职责

### 2.1 ticket 校验

- 校验 ticket 必填字段
- 校验 relay URL 前缀
- 校验过期时间
- 后续校验 cluster-aware DERP ticket

### 2.2 session 生命周期

- 创建 session
- 查询 session
- 删除 session
- 处理 session 已存在/不存在错误

### 2.3 数据转发

- 接收客户端上行数据
- 通过 session 找到对端
- 转发给对端
- 拒绝非参与方发包

### 2.4 集群化扩展

后续 DERP 集群场景下还需要：

- 同集群多节点 ticket 语义一致
- 客户端切换节点后重新 attach
- 多节点统一 session 语义

## 3. 当前 crate 划分

- `crates/relay-core`
  核心模型与错误定义
- `crates/auth`
  ticket 校验
- `crates/session`
  session store
- `crates/udp-relay`
  UDP attach / detach / forward

## 4. 当前内部模型

当前关键模型包括：

- `RelayTicket`
- `RelaySession`
- `RelayError`
- `RelayPacket`
- `ForwardedPacket`

## 5. 当前内部 trait

### `relay-core`

- `TicketValidator`

### `session`

- `SessionStore`

### `udp-relay`

虽然不是 trait，但 `UdpRelay` 当前承载核心能力：

- `attach`
- `attach_with_ticket`
- `session`
- `detach`
- `forward`

## 6. DERP/集群相关新增内部需求

为了支持多节点 DERP 集群，后续内部还需要承接：

- `DerpClusterId`
- `DerpNodeId`
- cluster-aware ticket 字段
- 多节点 attach 语义
- session 漫游或重建语义

最低落地要求：

1. 同集群 ticket 可在多个节点复用
2. 客户端切换 active 节点时能重新 attach
3. 不要求第一阶段就做跨节点状态复制

## 7. 当前缺口

当前实现还缺：

1. 真实网络监听与 runtime
2. 多节点集群实现
3. DERP 探测响应与健康反馈
4. 集群内共享 session 方案
5. 更丰富的鉴权策略

## 8. 建议实现顺序

1. 先完成单节点 relay 数据面运行时
2. 再扩 cluster-aware ticket
3. 再实现多节点 attach 语义
4. 最后再考虑集群内 session 同步或重建
