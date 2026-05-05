# server-wire-relay 需求文档

## 系统定位

`server-wire-relay` 是新系统的 `relay_udp` 数据面。

它不参与路径决策，不负责业务授权，只负责基于 ticket 的 UDP 会话转发。

## 核心职责

- 校验 relay ticket 的最小有效性
- 使用 `SLAN_WIRE_TICKET_SECRET` 校验 relay ticket 签名
- attach participant 到 relay session
- forward UDP payload
- detach participant
- 维护最小会话状态
- 暴露最小管理/观测接口

## 只支持的路径

- `relay_udp`

## 不负责的内容

- 用户/设备/网络业务鉴权
- 路径评分
- 路径切换策略
- keepalive 策略
- MTU 规划
- endpoint 探测

## 数据真相

- `RelaySession`
- `ParticipantBinding`
- `SessionExpiry`
- `UDPSourceBinding`

## 输入要求

客户端必须携带由 `server-wire` 签发的 `relay_udp` ticket 才能 attach。

## 非功能要求

- 转发路径必须保持无状态风格的快速处理
- source address 与 participant 绑定必须严格校验
- session 空闲或 participant 全部 detach 后应允许清理
