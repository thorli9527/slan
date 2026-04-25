# 主要序列图

本文档集中整理程序当前主链路所需的主要序列图，统一使用当前接口与节点视角命名。

覆盖范围：

- 用户注册与登录
- 设备注册与节点注册
- 网络创建与设备入网
- 节点启动配置获取
- 控制通道建立与网络地图同步
- P2P 直连尝试
- Relay 回退建连
- 断开与状态收敛

## 1. 用户注册与登录

```mermaid
sequenceDiagram
    autonumber
    participant User as 用户
    participant App as App
    participant Biz as Biz

    User->>App: 输入邮箱和密码
    App->>Biz: POST /auth/register
    Biz-->>App: AuthResponse(accessToken, refreshToken)

    User->>App: 再次登录
    App->>Biz: POST /auth/login
    Biz-->>App: AuthResponse(accessToken, refreshToken)
    App-->>User: 展示已登录状态
```

## 2. 设备注册与节点注册

```mermaid
sequenceDiagram
    autonumber
    participant User as 用户
    participant App as App
    participant Biz as Biz

    User->>App: 注册当前设备
    App->>Biz: POST /devices/register
    Biz-->>App: Device(deviceId, publicKey)

    User->>App: 注册当前节点
    App->>Biz: POST /nodes/register
    Note right of App: 带 deviceId、nodeId、nodePublicKey
    Biz-->>App: Node(nodeId, deviceId)
    App-->>User: 展示设备和节点身份
```

## 2.1 用户登录到设备入网（端到端）

```mermaid
sequenceDiagram
    autonumber
    participant User as 用户
    participant App as App(UI/Flutter)
    participant Core as Core(Rust)
    participant Biz as Biz(server-biz)
    participant Redis as Redis(Token/Sync)
    participant PG as Postgres

    User->>App: 输入邮箱/密码
    App->>Biz: POST /auth/login (email, password)
    Biz->>PG: GetUserByEmail(email)
    PG-->>Biz: User(userId, passwordHash)
    Biz->>Redis: StoreAccessToken(accessToken, userId, ttl)
    Biz->>Redis: StoreRefreshToken(refreshToken, userId, ttl)
    Biz-->>App: AuthResponse(userId, accessToken, refreshToken)
    App-->>User: 展示已登录状态

    User->>App: 注册当前设备
    App->>Biz: POST /devices/register (Authorization: Bearer accessToken)
    Biz->>Redis: Authenticate(accessToken)
    Redis-->>Biz: userId
    Biz->>PG: CreateDevice(userId, deviceId, publicKey, ...)
    PG-->>Biz: ok
    Biz-->>App: Device(deviceId, publicKey, status)

    User->>App: 注册当前节点(运行实例)
    App->>Biz: POST /nodes/register (Authorization: Bearer accessToken)
    Note right of App: 带 deviceId、nodePublicKey 等
    Biz->>Redis: Authenticate(accessToken)
    Redis-->>Biz: userId
    Biz->>PG: CreateNode(userId, nodeId, deviceId, publicKey, ...)
    PG-->>Biz: ok
    Biz-->>App: Node(nodeId, deviceId, status)

    alt 用户创建一个新网络
        User->>App: 创建网络
        App->>Biz: POST /networks (Authorization: Bearer accessToken)
        Biz->>Redis: Authenticate(accessToken)
        Redis-->>Biz: userId
        Biz->>PG: CreateNetworkWithDefaultSubnet(owner=userId, cidr)
        PG-->>Biz: Network(networkId, defaultSubnetId)
        Biz-->>App: Network(networkId, defaultSubnetId)
    else 用户加入一个已存在网络
        User->>App: 输入宿主邮箱或 join key，可选设备别名
        alt 宿主邮箱
            App->>Biz: POST /networks/join-by-owner-email
        else join key
            App->>Biz: POST /networks/join-by-key
        end
        Biz-->>App: NetworkJoinResult(member, attachment)
        opt 用户填写设备别名
            App->>Biz: PUT /networks/{networkId}/attachments/{attachmentId}/remark
            Biz-->>App: NetworkAssignment
        end
    end

    User->>App: 让设备加入已知 networkId
    App->>Biz: POST /networks/{networkId}/join (Authorization: Bearer accessToken)
    Note right of App: body: { deviceId }
    Biz->>Redis: Authenticate(accessToken)
    Redis-->>Biz: userId
    Biz->>PG: GetDeviceByID(deviceId) 校验归属
    PG-->>Biz: Device(userId, deviceId)
    Biz->>PG: GetNetworkByID(networkId)
    PG-->>Biz: Network(defaultSubnetId)
    Biz->>PG: GetMemberByNetworkDevice(networkId, deviceId)
    alt 不存在 member
        Biz->>PG: CreateMember(memberId, networkId, deviceId)
        PG-->>Biz: Member
    else 已存在 member
        PG-->>Biz: Member
    end
    Biz->>PG: GetAttachmentBySubnetDevice(defaultSubnetId, deviceId)
    alt 不存在 attachment
        Biz->>PG: ListAttachmentsBySubnet(defaultSubnetId)
        PG-->>Biz: attachments(used IPs)
        Biz->>Biz: allocateIP(subnetRange, used)
        Biz->>PG: CreateAttachment(attachmentId, virtualIP)
        PG-->>Biz: Attachment
    else 已存在 attachment
        PG-->>Biz: Attachment
    end
    Biz-->>App: NetworkJoinResult(member, attachment)
    App-->>User: 展示已入网(虚拟IP/网络信息)

    opt 展示网络详情
        App->>Biz: GET /networks/{networkId} (Authorization: Bearer accessToken)
        Biz->>Redis: Authenticate(accessToken)
        Redis-->>Biz: userId
        Biz->>PG: GetNetworkByID + ListSubnetsByNetwork + ListMembersByNetwork
        PG-->>Biz: NetworkDetail
        Biz-->>App: NetworkDetail(subnets, members)
    end
```

## 3. 网络创建与设备加入网络

```mermaid
sequenceDiagram
    autonumber
    participant User as 用户
    participant App as App
    participant Biz as Biz

    User->>App: 创建网络
    App->>Biz: POST /networks
    Biz-->>App: Network(networkId, defaultSubnetId)

    User->>App: 加入网络
    alt 已知 networkId
        App->>Biz: POST /networks/{networkId}/join
    else 宿主邮箱
        App->>Biz: POST /networks/join-by-owner-email
    else join key
        App->>Biz: POST /networks/join-by-key
    end
    Note right of App: 带 deviceId，可选别名后续写入 attachment remark
    Biz->>Biz: 建立成员关系
    Biz->>Biz: 分配默认子网虚拟 IP
    Biz-->>App: NetworkJoinResult(member, attachment)

    App->>Biz: POST /networks/{networkId}/switch
    Biz-->>App: NetworkJoinResult(member, attachment)
    App->>Biz: POST /networks/{networkId}/activate
    Biz-->>App: NetworkJoinResult(member, attachment)

    App->>Biz: GET /networks/{networkId}
    Biz-->>App: NetworkDetail(subnets, members)
```

## 4. 节点获取启动配置

```mermaid
sequenceDiagram
    autonumber
    participant App as App
    participant Core as Core
    participant Biz as Biz

    App->>Core: bootstrap(nodeId, networkId)
    Core->>Biz: POST /bootstrap
    Note right of Core: 带 nodeId、networkId
    Biz->>Biz: 校验 node 所属用户与网络成员关系
    Biz-->>Core: BootstrapResponse
    Note left of Biz: 返回 device、networks、controlPlane、stunServers、relay、networkMap
    Core-->>App: BootstrapModel
```

## 5. 控制通道建立与网络地图同步

```mermaid
sequenceDiagram
    autonumber
    participant Core as Core
    participant Biz as Biz

    alt 标准客户端启动
        Core->>Biz: POST /bootstrap
        Note right of Core: 带 nodeId、networkId
        Biz-->>Core: BootstrapResponse(sessionToken, wsUrl, networkMap)
    else 仅刷新控制会话
        Core->>Biz: POST /control/sessions
        Note right of Core: 已有 device/node/network 上下文
        Biz-->>Core: ControlSessionResponse(sessionToken, wsUrl, networkMap)
    end

    Core->>Biz: WebSocket connect wsUrl
    Core->>Biz: Envelope(NodeHello)
    Biz-->>Core: Envelope(NodeHelloAck)

    opt 客户端主动拉全量地图
        Core->>Biz: Envelope(NetworkMapRequest)
        Biz-->>Core: Envelope(NetworkMapResponse)
    end

    loop 运行期间
        Core->>Biz: Envelope(Ping)
        Biz-->>Core: Envelope(Pong)
        Biz-->>Core: Envelope(PeerUpdate/PeerRemove)
        Core->>Biz: Envelope(EndpointReport)
    end
```

## 6. P2P 直连尝试

```mermaid
sequenceDiagram
    autonumber
    participant NodeA as Node A
    participant Biz as Biz
    participant NodeB as Node B

    NodeA->>Biz: EndpointReport(natType, endpoints)
    NodeB->>Biz: EndpointReport(natType, endpoints)

    Biz-->>NodeA: PeerCandidate(peerNodeId, endpoint...)
    Biz-->>NodeB: PeerCandidate(peerNodeId, endpoint...)
    Biz-->>NodeA: ConnectPlan(preferDirect=true, paths)
    Biz-->>NodeB: ConnectPlan(preferDirect=true, paths)

    NodeA->>NodeB: 尝试 UDP 打洞 / 握手
    NodeB->>NodeA: 回应握手

    NodeA->>Biz: ConnectionState(path=p2p, state=connected)
    NodeB->>Biz: ConnectionState(path=p2p, state=connected)
```

## 7. Relay 回退建连

```mermaid
sequenceDiagram
    autonumber
    participant NodeA as Node A
    participant Biz as Biz
    participant Relay as Relay
    participant NodeB as Node B

    NodeA->>Biz: ConnectionState(path=p2p, state=failed, reason=timeout)
    NodeB->>Biz: ConnectionState(path=p2p, state=failed, reason=timeout)

    alt 控制面主动下发 relay 计划
        Biz-->>NodeA: ConnectPlan(paths=[relay], relayTicket)
        Biz-->>NodeB: ConnectPlan(paths=[relay], relayTicket)
    else 客户端主动申请 relay ticket
        NodeA->>Biz: POST /relay/tickets
        Note right of NodeA: networkId, srcNodeId, dstNodeId, reason
        Biz-->>NodeA: RelayTicket
        NodeB->>Biz: POST /relay/tickets
        Biz-->>NodeB: RelayTicket
    end

    NodeA->>Relay: connect(ticketA)
    Relay->>Relay: 校验票据并创建/绑定 session
    Relay-->>NodeA: attach ok

    NodeB->>Relay: connect(ticketB)
    Relay->>Relay: 校验票据并加入同一 session
    Relay-->>NodeB: attach ok

    NodeA->>Relay: UDP payload
    Relay->>NodeB: forward payload
    NodeB->>Relay: UDP payload
    Relay->>NodeA: forward payload

    NodeA->>Biz: ConnectionState(path=relay, state=connected)
    NodeB->>Biz: ConnectionState(path=relay, state=connected)
```

## 8. 隧道建立与业务数据收发

```mermaid
sequenceDiagram
    autonumber
    participant AppA as App A
    participant CoreA as Core A
    participant CoreB as Core B
    participant AppB as App B

    AppA->>CoreA: connect(peerNodeId)
    Note over CoreA,CoreB: 底层可走 p2p 或 relay
    CoreA->>CoreB: 建立加密隧道握手
    CoreB-->>CoreA: 握手确认

    CoreA-->>AppA: ConnectionState.connected
    CoreB-->>AppB: ConnectionState.connected

    AppA->>CoreA: 发送业务流量
    CoreA->>CoreB: 加密隧道数据包
    CoreB->>AppB: 解密后交付
```

## 9. 断开与状态收敛

```mermaid
sequenceDiagram
    autonumber
    participant User as 用户
    participant App as App
    participant Core as Core
    participant Biz as Biz
    participant Relay as Relay

    User->>App: 主动断开连接
    App->>Core: disconnect()
    Core->>Biz: ConnectionState(state=closed)
    opt 当前正在走 relay
        Core->>Relay: detach / close session endpoint
        Relay->>Relay: 清理 endpoint 绑定
    end
    Biz-->>Core: 可选 DisconnectNotice / 状态确认
    Core-->>App: ConnectionState.disconnected
```

## 10. 主链路总览

```mermaid
sequenceDiagram
    autonumber
    participant User as 用户
    participant App as App
    participant Core as Core
    participant Biz as Biz
    participant Relay as Relay
    participant Peer as 对端节点

    User->>App: 注册 / 登录
    App->>Biz: auth register/login
    Biz-->>App: token

    App->>Biz: register device
    Biz-->>App: deviceId
    App->>Biz: register node
    Biz-->>App: nodeId

    App->>Biz: create network / join-by-owner-email / join-by-key
    Biz-->>App: network and attachment
    opt 用户设置设备别名
        App->>Biz: update attachment remark
    end
    App->>Biz: switch network
    Biz-->>App: NetworkJoinResult(member, attachment)
    App->>Biz: activate network
    Biz-->>App: NetworkJoinResult(member, attachment)

    App->>Core: bootstrap(nodeId, networkId)
    Core->>Biz: POST /bootstrap
    Biz-->>Core: bootstrap config

    Core->>Biz: ws hello with bootstrap session token
    Biz-->>Core: network map / connect plan

    Core->>Peer: try p2p
    alt P2P 成功
        Core->>Biz: ConnectionState(path=p2p, connected)
    else P2P 失败
        Core->>Biz: request relay ticket
        Biz-->>Core: relay ticket
        Core->>Relay: attach with ticket
        Relay->>Peer: relay forward
        Core->>Biz: ConnectionState(path=relay, connected)
    end

    App-->>User: 展示连接状态
```

## 11. 建议阅读顺序

1. 先看“主链路总览”
2. 再看“设备注册与节点注册”以及“网络创建与设备加入网络”
3. 然后看“节点获取启动配置”和“控制通道建立与网络地图同步”
4. 最后看“P2P 直连尝试”和“Relay 回退建连”
