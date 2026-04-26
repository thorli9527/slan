# 一期 MVP 方案

## 1. 一期目标

一期目标不是覆盖完整 PRD，而是打通一个可联调、可演示、可继续扩展的最短闭环。

闭环定义：

1. 用户注册或登录
2. 创建设备
3. 创建网络
4. 设备加入网络
5. 客户端获取网络配置
6. 建立控制信道
7. 尝试 P2P 建连
8. P2P 失败后走 Relay
9. 建立加密隧道
10. UI 展示连接状态和设备状态

## 2. 一期范围

### 2.1 必做范围

#### 控制面 `server-biz`

- 用户注册
- 用户登录
- JWT 鉴权
- 创建设备
- 创建设备与账号绑定关系
- 创建网络
- 设备加入网络
- 查询网络成员
- 虚拟 IP 分配
- MQTT 控制信道
- 下发客户端连接配置

#### 数据面 `server-relay`

- Relay 票据鉴权
- Relay 会话建立
- UDP Relay
- 基础保活
- 超时回收

#### 客户端核心 `app_core`

- 登录并保存凭证
- 获取网络与设备配置
- 建立 MQTT 控制信道
- NAT 检测
- P2P 尝试
- P2P 失败切换 Relay
- 建立基础加密隧道
- 维护设备在线状态

#### 客户端应用 `app`

- 登录页
- 网络列表页
- 创建网络入口
- 连接状态页
- 设备列表页
- 基础错误提示和状态展示

### 2.2 预留目录但不在一期完成

- 私有 DNS 全功能
- hosts 导入导出
- Traceroute
- 故障分析
- ACL 细粒度策略
- TCP Relay
- 计费系统
- DAU/MAU 大盘
- 复杂运营后台

## 3. 一期不做重的原因

- DNS、ACL、计费、运营报表都不是“能不能连通”的前置条件
- 如果最短建连链路没有打通，先做这些能力会形成高返工
- MVP 的目标是先验证产品闭环、协议设计和联调方式

## 4. 开发顺序

### 阶段 0：冻结协议

先完成 `protocol/`，避免多端并行时各自定义接口。

最少需要冻结以下内容：

- 登录和注册 API
- 网络创建、加入、查询 API
- 设备注册、设备上线 API
- 客户端启动后拉取配置 API
- MQTT 控制消息
- Relay 鉴权票据结构
- 通用错误码

### 阶段 1：控制面最小可用

优先完成 `server/server-biz`：

- 用户系统
- 网络管理
- 设备管理
- IP 分配
- MQTT 控制信道
- 连接配置下发

原因：

- 客户端与 Relay 都依赖控制面给出身份、成员关系和连接参数

### 阶段 2：Relay 最小可用

完成 `server/server-relay`：

- Relay 鉴权
- UDP 中继
- 会话生命周期
- 保活与回收

原因：

- 即使 P2P 无法立即成功，也需要可靠兜底路径保证 MVP 可演示

### 阶段 3：客户端核心打通

完成 `client/app_core`：

- 登录
- 拉取配置
- 控制信道
- NAT 检测
- P2P 尝试
- Relay 回退
- 隧道建立

原因：

- 这是完整建连链路的核心

### 阶段 4：接入客户端 UI

完成 `client/app`：

- 页面入口
- 基础状态展示
- 用户操作入口
- 错误反馈

原因：

- UI 应建立在核心链路已经能工作的基础上

### 阶段 5：预留扩展

后续再补：

- DNS
- 诊断
- ACL
- TCP Relay
- 运营平台

## 5. 模块依赖关系

```text
protocol
  ├─> server-biz
  ├─> server-relay
  └─> app_core
          └─> client/app
```

说明：

- `protocol` 必须最先建立
- `client/app` 依赖 `app_core`
- `app_core` 依赖 `server-biz` 和 `server-relay`

## 6. 一期主流程时序

```text
app
 -> app_core
 -> server-biz 登录
 -> server-biz 返回 token 和设备初始化信息
 -> app_core 获取网络配置
 -> app_core 建立 MQTT 控制信道
 -> app_core 执行 NAT 检测
 -> app_core 尝试 P2P
 -> 若失败则申请 Relay 票据
 -> app_core 连接 server-relay
 -> 建立加密隧道
 -> app 展示连接成功和设备状态
```

## 7. 一期建议目录焦点

一期只需要优先充实以下目录：

```text
client/
├─ app/lib/features/
│  ├─ auth/
│  ├─ home/
│  ├─ networks/
│  └─ devices/
└─ app_core/crates/
   ├─ core/
   ├─ controller-client/
   ├─ nat/
   ├─ p2p/
   ├─ relay-client/
   ├─ tunnel/
   └─ ffi-bridge/

server/
├─ server-biz/internal/
│  ├─ auth/
│  ├─ network/
│  ├─ device/
│  ├─ ipam/
│  ├─ control/
│  └─ ws/
└─ server-relay/crates/
   ├─ relay-core/
   ├─ udp-relay/
   └─ session/

protocol/
└─ openapi/
```

## 8. 一期验收标准

满足以下条件即可视为一期闭环完成：

1. 用户可以注册并登录
2. 用户可以创建网络
3. 设备可以注册并加入网络
4. 客户端可以获取网络成员和分配 IP
5. 客户端可以建立控制信道
6. 客户端可以优先尝试 P2P
7. 在 P2P 失败时可以自动切换到 Relay
8. 设备之间可以建立加密连接
9. UI 可以展示连接状态、设备状态和基础错误信息

## 9. 落地建议

### 文档先行

在正式写代码前，至少补齐以下文档：

- `docs/architecture/system-overview.md`
- `docs/architecture/connection-flow.md`
- `protocol/openapi/`

### 协议优先

- API 与消息格式先冻结，再允许客户端和服务端并行开发
- 不允许由前后端各自临时维护私有字段

### 先打通主链路

- 先实现“能连上”
- 再逐步补 DNS、诊断、ACL 和运营能力

### 工程边界要硬

- UI 不做网络核心
- 控制面不做中继
- Relay 不做业务管理
- 协议不分散维护
