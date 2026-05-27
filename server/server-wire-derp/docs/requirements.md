# server-wire-derp 需求文档

## 系统定位

`server-wire-derp` 是新系统的最终兜底中继数据面。

它参考 Tailscale 的 DERP 思路，为无法建立 `lan_udp`、`ipv6_udp`、
`direct_udp`、`relay_udp` 的 peer 提供 `derp_tcp_tls_443` 保底通路。

它不参与业务授权，不负责路径排序，也不承担 UDP relay 职责。

## 路径定位

`server-wire-derp` 只承担以下单一路径：

- `derp_tcp_tls_443`

这条路径永远是控制面排序中的最终兜底，不应与 `relay_udp` 混为一层。

## 核心职责

- 通过 TCP 长连接接收客户端数据面连接；生产 443/TLS 由外部终止层或后续统一 TLS transport 提供
- 基于 `server-wire` 签发的 DERP ticket 建立会话
- 使用 `SLAN_WIRE_TICKET_SECRET` 校验 DERP ticket 签名
- 维护 peer 到 DERP 节点的连接绑定
- 在两个 peer 之间转发加密隧道载荷
- 提供 DERP 节点、区域、集群级健康状态
- 暴露管理与观测接口

## 不负责的内容

- 用户、设备、网络业务鉴权
- 路径探测与评分
- active path 决策
- UDP 中继
- 虚拟 IP 分配
- keepalive 策略计算

## 数据真相

`server-wire-derp` 是以下对象的真相源：

- `DerpConnection`
- `DerpSession`
- `PeerConnectionBinding`
- `DerpNodeHealth`
- `DerpRegionHealth`
- `DerpTicketLease`

## 对 server-wire 的依赖

`server-wire-derp` 不自行决定是否允许连接。它只信任并消费
`server-wire` 签发的 DERP ticket。

`server-wire` 需要向客户端输出：

- DERP map
- DERP region / node 候选
- DERP ticket
- DERP 续期建议

## 输入要求

客户端接入 `server-wire-derp` 前必须已经：

1. 完成 `server-biz` 的业务身份与网络授权
2. 向 `server-wire` 注册 runtime
3. 从 `server-wire` 获得 `derp_tcp_tls_443` 作为当前最终兜底路径
4. 从 `server-wire` 获得 DERP ticket 与目标 node 信息

## 非功能要求

- 裸 TCP 监听必须稳定；如生产需要 443/TLS，必须由外部四层/TLS 终止层转发到裸 TCP listener，或后续统一实现内置 TLS transport
- 长连接断开与重连必须可恢复
- 节点健康与 region 可用性必须可观测
- 不允许与 `server-wire-relay` 共用 UDP relay 会话模型
- 单节点异常时，客户端应可在 `server-wire` 指导下切到其他 DERP 节点
