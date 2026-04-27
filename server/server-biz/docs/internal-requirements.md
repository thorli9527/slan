# server-biz 内部需求

本文档描述 `server/server-biz` 作为控制面的内部职责、模块划分和后续必须承接的能力。

## 1. 定位

`server-biz` 是 SLAN 的控制面服务，负责：

- 用户和访问令牌
- 设备和节点身份
- 网络、成员、子网和挂载关系
- 启动配置编排
- relay / DERP 票据签发
- 控制通道会话与网络地图编排

它不负责真实业务流量转发。

## 2. 内部核心职责

### 2.1 身份与鉴权

- 用户注册
- 用户登录
- Bearer Token 校验
- 设备归属校验
- 节点归属校验

### 2.2 网络编排

- 创建网络
- 查询网络
- 设备加入网络
- 成员查询
- 子网创建
- 设备挂载到子网
- 分配虚拟 IP

### 2.3 启动配置

- 校验 `nodeId + networkId`
- 返回设备、网络、控制通道、STUN、relay / DERP 视图
- 返回 `derp_map`

### 2.4 票据签发

- 签发 relay ticket
- 支持 cluster-aware DERP ticket 字段
- 约束 `srcNodeId / dstNodeId / networkId`

### 2.5 控制通道编排

- 控制会话创建
- MQTT 控制握手
- NetworkMap 下发
- PeerUpdate / PeerRemove
- EndpointReport
- ConnectPlan
- ConnectionState

## 3. 当前模块划分

`server-biz` 目前主要由这些目录承接：

- `cmd/biz-server`
  进程入口
- `api/http`
  HTTP 路由层
- `api/dto`
  请求响应模型
- `internal/controlmsg`
  控制通道消息模型
- `internal/service`
  服务接口与数据库实现
- `internal/repo`
  PostgreSQL / Redis 访问层
- `configs`
  配置结构、样例与运行时初始化

## 4. 当前内部服务边界

当前 `internal/service` 中的核心服务接口为：

- `Auth`
- `Device`
- `Network`
- `Node`
- `Bootstrap`
- `TokenVerifier`
- `ControlChannel`
- `ControlSync`
- `Ops`

这些接口当前由 `internal/service/impl` 下的 PostgreSQL / Redis 实现承接。

## 5. 当前内部需求模型

控制面内部需要维护的关键模型包括：

- `User`
- `AuthResponse`
- `Device`
- `Node`
- `Network`
- `Subnet`
- `NetworkMember`
- `SubnetAttachment`
- `BootstrapRequest`
- `BootstrapResponse`
- `ControlSessionResponse`
- `RelayTicketRequest`
- `RelayTicket`
- `NetworkMap`
- `Peer`
- `Route`
- `RelayRegion`
- `RelayEndpoint`

### 5.1 需求模型脑图

下面这版不再依赖 Mermaid，而是改成“可扩展需求树”。

设计目标：

- 不依赖特定渲染器版本
- 可以持续增量扩展
- 每个主节点有稳定编号，后续新增需求时直接往下挂
- 可以从总树跳到对应章节继续展开

```text
server-biz 需求模型
├── AUTH 身份与鉴权
│   ├── AUTH.1 用户注册
│   ├── AUTH.2 用户登录
│   ├── AUTH.3 Bearer Token 校验
│   ├── AUTH.4 Device 归属校验
│   └── AUTH.5 Node 归属校验
├── DN 设备与节点
│   ├── DN.1 Device
│   ├── DN.2 Node
│   ├── DN.3 RegisterDeviceRequest
│   └── DN.4 RegisterNodeRequest
├── NET 网络编排
│   ├── NET.1 Network
│   ├── NET.2 Subnet
│   ├── NET.3 NetworkMember
│   ├── NET.4 SubnetAttachment
│   ├── NET.5 CreateNetworkRequest
│   ├── NET.7 AttachDeviceRequest
│   ├── NET.8 JoinNetworkRequest
│   ├── NET.9 NetworkJoinResult
│   ├── NET.10 NetworkDetail
│   ├── NET.11 虚拟 IP 分配
│   ├── NET.12 默认子网自动挂载
│   ├── NET.13 UpdateNetworkJoinKeyRequest
│   ├── NET.14 UpdateNetworkDNSRequest
│   ├── NET.15 SwitchNetworkRequest
│   ├── NET.16 DeactivateNetworkRequest
│   ├── NET.17 JoinNetworkByOwnerEmailRequest
│   ├── NET.18 UpdateAttachmentIPRequest
│   ├── NET.19 UpdateAttachmentRemarkRequest
│   └── NET.20 NetworkAssignment
├── BOOT 启动配置
│   ├── BOOT.1 BootstrapRequest
│   ├── BOOT.2 BootstrapResponse
│   ├── BOOT.3 DeviceBootstrap
│   ├── BOOT.4 ControlPlaneConfig
│   ├── BOOT.5 RelayConfig
│   ├── BOOT.6 STUNServers
│   ├── BOOT.7 NetworkMap
│   └── BOOT.8 DerpMap
├── CTRL 控制通道会话
│   ├── CTRL.1 CreateControlSessionRequest
│   ├── CTRL.2 ControlSessionResponse
│   ├── CTRL.3 ControlSessionID
│   ├── CTRL.4 SessionToken
│   ├── CTRL.5 初始 NetworkMap
│   └── CTRL.6 MQTT 控制入口
├── MAP 网络地图模型
│   ├── MAP.1 NetworkMap
│   ├── MAP.2 Peer
│   ├── MAP.3 Endpoint
│   ├── MAP.4 Route
│   ├── MAP.5 DNSConfig
│   ├── MAP.6 RelayRegion
│   └── MAP.7 RelayEndpoint
├── RELAY Relay 与 DERP
│   ├── RELAY.1 RelayTicketRequest
│   ├── RELAY.2 RelayTicket
│   ├── RELAY.3 DerpMap
│   ├── RELAY.4 DerpCluster
│   ├── RELAY.5 DerpNode
│   ├── RELAY.6 DerpClusterID
│   ├── RELAY.7 AllowedDerpNodeIDs
│   └── RELAY.8 RelayRegionID
├── SVC 服务边界
│   ├── SVC.1 Auth
│   ├── SVC.2 Device
│   ├── SVC.3 Network
│   ├── SVC.4 Node
│   ├── SVC.5 Bootstrap
│   ├── SVC.6 TokenVerifier
│   ├── SVC.7 ControlChannel
│   ├── SVC.8 ControlSync
│   └── SVC.9 Ops
└── INFRA 基础设施
    ├── INFRA.1 HTTP Address
    ├── INFRA.2 MQTT broker endpoint
    ├── INFRA.3 Relay Region
    ├── INFRA.4 Relay Endpoint
    └── INFRA.5 Bootstrap STUNServers
```

这个脑图可以从三个角度理解：

- 纵向看业务对象：`Device`、`Node`、`Network`、`Subnet`、`NetworkMap`、`RelayTicket`
- 横向看控制面职责：鉴权、编排、启动配置、控制会话、票据签发
- 边界上看服务接口：`Auth`、`Device`、`Network`、`Node`、`Bootstrap`、`TokenVerifier`

### 5.2 动态扩展规则

后续扩展时按下面的规则追加，不要改已有编号：

1. 新增一级领域时，增加新的主前缀。
   例如：`POLICY`、`OBS`、`ACL`
2. 在已有领域下新增能力时，直接追加下一个编号。
   例如：`NET.13`、`RELAY.9`
3. 某个节点需要继续细化时，使用三级编号。
   例如：
   `MAP.1.1 SelfUserID`
   `MAP.1.2 SelfDeviceID`
   `MAP.1.3 SelfNodeID`
4. 文档章节、接口设计、实现任务都优先引用这个编号，而不是只写自然语言标题。

### 5.3 扩展示例

如果后续要补 ACL 和策略，可以这样扩：

```text
├── POLICY 策略与访问控制
│   ├── POLICY.1 ACL 模型
│   ├── POLICY.2 Network 访问策略
│   ├── POLICY.3 Node 能力约束
│   └── POLICY.4 Relay 使用策略
```

这样可以保证需求树一直增量演进，不需要反复重画脑图。

### 5.4 关键模型属性细化

这一节把高频核心模型继续展开到字段级，作为需求评审、协议设计和实现落地时的统一语义基线。

#### AUTH.1 `RegisterRequest`

- `email`
  中文备注：用户唯一登录标识，输入时应做去空格、大小写归一和格式校验。
- `password`
  中文备注：注册时提交的原始密码，只允许进入鉴权服务，不应在日志、事件或响应中回显。

#### AUTH.2 `LoginRequest`

- `email`
  中文备注：登录账号标识，语义上必须和注册时归一后的邮箱一致。
- `password`
  中文备注：登录校验口令，服务端只用于比对，不应向其他模块传播明文。

#### AUTH.3 `AuthResponse`

- `userId`
  中文备注：控制面内部用户主键，后续所有受保护资源都围绕它做归属判断。
- `accessToken`
  中文备注：HTTP 管理面 Bearer Token，生命周期较短，用于调用受保护接口。
- `refreshToken`
  中文备注：预留的续期令牌，当前可为空，但模型上已经预留扩展位。
- `expiresIn`
  中文备注：`accessToken` 的剩余有效期，单位秒，主要用于客户端刷新本地会话状态。

#### DN.1 `Device`

- `deviceId`
  中文备注：业务设备主键，用于表示一个客户端安装实例或终端实体。
- `name`
  中文备注：展示给用户和运维看的设备名称，不要求全局唯一。
- `platform`
  中文备注：设备所属平台，例如 `macos`、`linux`、`windows`。
- `status`
  中文备注：兼容字段，最多表示控制可达快照；网络在线应以 `networkState.networkOnline` 为准。
- `publicKey`
  中文备注：设备层公钥，用于隧道或后续身份扩展，允许为空表示尚未完成密钥初始化。
- `networkIds`
  中文备注：设备当前已加入或已挂载到的网络列表，是派生视图，不是独立事实源。

#### DN.2 `Node`

- `nodeId`
  中文备注：通信节点主键，表示设备在组网控制面和数据面上的逻辑身份。
- `deviceId`
  中文备注：节点所属业务设备 ID，用于把通信节点和业务设备绑定起来。
- `nodePublicKey`
  中文备注：节点级公钥，供对等发现、控制通道和数据面建立信任。
- `networkIds`
  中文备注：节点当前可见或已加入的网络列表，通常由设备成员关系派生。
- `capabilities`
  中文备注：节点能力声明，例如普通 peer、relay 能力、subnet-router 能力等。

#### NET.1 `Network`

- `networkId`
  中文备注：逻辑网络主键，是设备、节点、子网和票据编排的顶层作用域。
- `name`
  中文备注：网络展示名称，面向用户和管理后台。
- `description`
  中文备注：运维说明文本，用于补充网络用途、场景和管理备注。
- `defaultSubnetId`
  中文备注：创建网络时自动生成的默认子网 ID，Join 流程默认挂这个子网。
- `defaultSubnetCidr`
  中文备注：默认子网地址段，主要用于前端和调试视图快速展示。

#### NET.2 `Subnet`

- `subnetId`
  中文备注：子网主键，是地址空间和 IP 分配的直接作用对象。
- `networkId`
  中文备注：所属父网络 ID，用于保证子网不会脱离网络单独存在。
- `name`
  中文备注：网络内的子网名称，应该在同一网络下唯一。
- `cidr`
  中文备注：子网地址段定义，是 IP 分配和路由编排的根约束。
- `gatewayIp`
  中文备注：该子网的网关或保留路由地址，可自动生成也可显式指定。
- `allocationStartIp`
  中文备注：允许分配的起始地址，用于排除网关和保留地址。
- `allocationEndIp`
  中文备注：允许分配的结束地址，和起始地址一起限制地址池范围。
- `isDefault`
  中文备注：标识该子网是否为网络创建时自动生成的默认子网。
- `status`
  中文备注：子网生命周期状态，便于后续支持停用、迁移或软删除。

#### NET.3 `NetworkMember`

- `memberId`
  中文备注：网络成员关系主键，表示某个设备进入某个网络这一事实。
- `networkId`
  中文备注：所属网络范围。
- `deviceId`
  中文备注：加入该网络的设备。
- `role`
  中文备注：成员角色，例如 `owner`、`member`，后续可扩展 `admin`、`viewer`。
- `status`
  中文备注：成员关系状态，例如有效、冻结、待确认。

#### NET.4 `SubnetAttachment`

- `attachmentId`
  中文备注：设备挂载到子网的关系主键。
- `networkId`
  中文备注：冗余保存父网络 ID，便于快速过滤和权限校验。
- `subnetId`
  中文备注：设备当前落入的具体子网。
- `deviceId`
  中文备注：被挂载的设备。
- `virtualIp`
  中文备注：在该子网中为设备分配的虚拟地址，是后续 NetworkMap 的重要来源。
- `status`
  中文备注：挂载关系状态，后续可用于表示迁移中、失效、禁用等过程态。

#### BOOT.2 `BootstrapResponse`

- `controlSessionId`
  中文备注：启动阶段同时分配的控制会话 ID，用于后续控制通道关联。
- `device`
  中文备注：设备启动视图，除了设备本体，还要带当前所有子网挂载信息。
- `networks`
  中文备注：当前设备可见的网络拓扑详情，便于客户端本地建立初始缓存。
- `controlPlane`
  中文备注：控制通道接入配置，至少包含 MQTT broker 地址和心跳参数。
- `stunServers`
  中文备注：NAT 探测使用的 STUN 服务器列表。
- `relay`
  中文备注：普通 relay 回退配置，用于非 DERP 或兜底场景。
- `derpMap`
  中文备注：DERP 集群视图，用于客户端 `derp_pool` 建立多连接热备。
- `networkMap`
  中文备注：当前节点的初始网络地图，是控制通道建立前的第一份拓扑快照。

#### CTRL.2 `ControlSessionResponse`

- `controlSessionId`
  中文备注：控制面分配的控制通道会话主键。
- `sessionToken`
  中文备注：控制通道握手令牌，用于 MQTT 控制通道鉴权。
- `controlPlane`
  中文备注：控制面连接配置，和 `bootstrap` 中的同类字段保持一致语义。
- `networkMap`
  中文备注：创建控制会话时下发的初始网络地图，避免控制连接后还要再拉一次全量。

#### MAP.1 `NetworkMap`

- `selfUserId`
  中文备注：当前节点所属用户 ID。
- `selfDeviceId`
  中文备注：当前节点对应的设备 ID。
- `selfNodeId`
  中文备注：当前节点自身 ID，是控制面下发任何 peer 关系的锚点。
- `networkId`
  中文备注：该网络地图所属网络范围。
- `revision`
  中文备注：网络地图版本号，用于全量/增量同步和顺序一致性控制。
- `heartbeatSeconds`
  中文备注：控制通道期望心跳周期。
- `stunServers`
  中文备注：该网络会话建议使用的 STUN 列表。
- `peers`
  中文备注：当前网络内所有可见对等节点快照。
- `routes`
  中文备注：当前网络内的可见路由集合。
- `relayRegions`
  中文备注：普通 relay 区域视图。
- `dns`
  中文备注：网络级 DNS 配置。
- `mtu`
  中文备注：建议的网络 MTU，用于客户端隧道层配置。

#### MAP.2 `Peer`

- `nodeId`
  中文备注：对等节点 ID。
- `deviceId`
  中文备注：该 peer 所属设备 ID。
- `publicKey`
  中文备注：对等节点公钥，用于建立直连或中继会话。
- `status`
  中文备注：节点控制面可达状态，是拓扑规划视角下的当前快照。
- `relayAllowed`
  中文备注：是否允许对该 peer 走 relay/DERP 回退。
- `virtualIps`
  中文备注：该 peer 当前持有的虚拟 IP 列表。
- `endpoints`
  中文备注：控制面已知的候选端点列表，供 P2P 尝试排序。
- `allowedRoutes`
  中文备注：该 peer 可通告或允许访问的路由集合。

#### RELAY.2 `RelayTicket`

- `ticketId`
  中文备注：票据主键，是一次中继授权的唯一标识。
- `networkId`
  中文备注：票据所属网络范围，防止跨网误用。
- `sessionId`
  中文备注：中继数据面的会话 ID。
- `srcNodeId`
  中文备注：源节点 ID。
- `dstNodeId`
  中文备注：目标节点 ID。
- `derpClusterId`
  中文备注：允许使用的 DERP 集群 ID，可为空表示普通 relay 票据。
- `allowedDerpNodeIds`
  中文备注：集群内允许 attach 的 DERP 节点白名单。
- `relayUrl`
  中文备注：普通 relay 或 DERP attach 的接入地址。
- `expiresAt`
  中文备注：票据失效时间，超过该时间后数据面必须拒绝使用。
- `sessionKey`
  中文备注：可选的不透明会话密钥，用于给数据面传递会话级附加授权材料。
- `signature`
  中文备注：控制面签名，供 `server-relay` 校验票据真实性和完整性。

#### RELAY.3 `DerpMap`

- `probeIntervalSeconds`
  中文备注：客户端做 RTT、丢包、超时探测的建议周期。
- `clusters`
  中文备注：可选 DERP 集群列表，客户端在其上建立 `derp_pool`。

#### RELAY.4 `DerpCluster`

- `clusterId`
  中文备注：DERP 集群主键。
- `regionId`
  中文备注：区域 ID，通常用于归类和路由展示。
- `regionName`
  中文备注：人类可读区域名，例如“Asia Pacific East”。
- `recommendedFanout`
  中文备注：建议同时建立的热备连接数，例如 2 或 3。
- `nodes`
  中文备注：该集群下的可用 DERP 节点列表。

#### RELAY.5 `DerpNode`

- `nodeId`
  中文备注：DERP 节点主键。
- `host`
  中文备注：节点域名或 IP。
- `port`
  中文备注：接入端口。
- `transport`
  中文备注：接入协议，例如 `udp`、`tcp`、`quic`。
- `priority`
  中文备注：控制面建议优先级，数值越小越优先。
- `tags`
  中文备注：附加标签，用于机房、运营商、链路类型等扩展筛选。

### 5.5 模型细化原则

- 同一个字段在不同响应里语义必须一致。
  例如：`networkId`、`nodeId`、`deviceId` 不能在不同接口里表示不同层级对象。
- 派生字段和事实字段要分清。
  例如：`Device.networkIds` 更接近派生视图，而 `SubnetAttachment` 才是网络归属事实。
- 面向客户端输出的模型必须是编排结果，而不是内部存储结构。
- 任何新增字段都应补中文备注，并优先挂到上面的编号树里。

## 6. DERP/集群相关内部需求

为了支持 DERP 集群与客户端 `derp_pool`，`server-biz` 当前已经承接：

- `DerpMap`
- `DerpCluster`
- `DerpNode`
- 推荐 active 候选顺序
- cluster-aware ticket 签发

具体要求：

1. `bootstrap` 返回 `derp_map`
2. `ConnectPlan` 返回推荐 `derpClusterId`
3. `RelayTicket` 支持：
   - `derpClusterId`
   - `allowedDerpNodeIds`

后续重点是生产配置、真实多节点运行时、观测指标和故障恢复验证。

## 7. 当前缺口

当前主业务接口已经基本拉通：

1. `bootstrap` 能返回控制会话、设备 attachment、`NetworkMap` 和 `derp_map`
2. `control/sessions` 保留为显式控制会话刷新入口
3. join-by-owner-email / join-by-key / switch / activate / deactivate 已成为公开客户端流程
4. attachment remark 支持网络 owner 维护所有备注，也支持设备所有者维护自己的别名
5. DERP / relay ticket 已具备 cluster-aware 字段

## 8. 建议实现顺序

后续重点从“打通主流程”转为“补强生产约束”：

1. 固化 create / join / alias / switch / activate / bootstrap / relay fallback 的回归测试矩阵
2. 补强策略、ACL、成员审批和错误码一致性
3. 扩展控制通道事件覆盖和连接状态上报
4. 补充生产配置校验、限流和观测指标
5. 与 app_core / server-relay 继续联调 DERP 集群和真实数据面路径
