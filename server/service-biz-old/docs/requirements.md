# server-biz 需求文档

## 系统定位

`server-biz` 是业务控制面，只负责身份、资源归属和网络业务配置。

它不参与实时路径选择，不维护 endpoint/path/runtime 真相，不承担 relay
数据转发。

## 核心职责

- 用户注册、登录、刷新令牌、管理员鉴权
- 设备注册与节点注册
- 网络创建、加入、成员审批、切换
- 虚拟 IP 分配与 attachment 生命周期
- DNS、CIDR、Allowed IPs、业务配额等静态网络配置
- 面向 `server-wire` 输出已授权的联网视图
- RBAC、审计、运营后台能力
- Wire 控制面/数据面节点健康观测，包括票据密钥环一致性漂移检测

## 不负责的内容

- endpoint 上报
- 路径探测与评分
- active path 真相
- MTU probing
- endpoint roaming
- relay ticket 运行时决策
- UDP relay session 与数据转发
- 生成或持有 Wire ticket 签名密钥的真实密钥值

## 数据真相

`server-biz` 是以下对象的唯一真相源：

- `User`
- `Session`
- `Device`
- `Node`
- `Network`
- `Membership`
- `Attachment`
- `VirtualIP`
- `AllowedIPs`
- `DNSConfig`
- `PlanQuota`
- `AdminRole`

## 对 server-wire 的输出要求

`server-biz` 必须稳定输出以下授权结果：

- peer 所属的 `deviceId/nodeId/networkId`
- peer 是否允许联网
- peer 的 `virtualIps`
- peer 的 `allowedIps`
- peer 的 `dns`
- peer 可见的对端范围
- 套餐与配额上限

## Wire 运维健康要求

`server-biz` 必须能在 ops 视图里汇总当前可调度的 `server-wire`、
`server-wire-relay`、`server-wire-derp` 实例票据密钥状态：

- `keyRingId` 必须跨 `wire / relay / derp` 一致
- `rotationReady` 必须能反映是否具备滚动轮换条件
- 任一活跃实例 `keyRingId` 不一致时必须输出 `drifted=true`
- 任一活跃实例无法提供密钥状态时必须计入 `unavailableCount`
- 历史 stale、disabled、unhealthy 节点不得污染当前健康汇总

## 非功能要求

- 所有业务资源写入必须幂等
- 设备、节点、网络、挂载关系必须可审计
- 对内接口允许 `server-wire` 读取授权与拓扑，但不允许其直接写业务资源
