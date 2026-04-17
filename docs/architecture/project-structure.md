# 项目目录与模块设计

## 1. 目标

在保留 `client/` 和 `server/` 两个一级目录的前提下，建立清晰的业务边界、模块职责和后续可扩展的仓库结构。

核心原则：

- `client/` 只放客户端相关能力
- `server/` 只放服务端相关能力
- 控制面与数据面严格分离
- 协议定义独立管理
- 先按业务边界拆分，再在边界内组织语言实现

## 2. 目标目录结构

```text
slan/
├─ client/
│  ├─ app/                              # Flutter 客户端
│  └─ app_core/                         # Rust 客户端核心
│
├─ server/
│  ├─ server-biz/                       # Go 控制面
│  ├─ server-relay/                     # Rust Relay 数据面
│  └─ server-ops/                       # 运营平台
│
├─ protocol/                            # 协议定义
├─ deploy/                              # 部署资源
├─ scripts/                             # 脚本
├─ docs/                                # 文档
└─ README.md
```

## 3. 详细模块结构

### 3.1 客户端

```text
client/
├─ app/
│  ├─ lib/
│  │  ├─ app/                           # 应用入口、路由、主题、启动流程
│  │  ├─ features/
│  │  │  ├─ auth/                       # 登录、注册
│  │  │  ├─ home/                       # 首页、连接状态
│  │  │  ├─ networks/                   # 网络列表、创建、加入、详情
│  │  │  ├─ devices/                    # 设备列表、重命名、分组
│  │  │  ├─ dns/                        # 私有 DNS、hosts 导入导出
│  │  │  ├─ diagnostics/                # NAT 检测、Ping、Traceroute
│  │  │  └─ settings/                   # 设置、日志、版本
│  │  ├─ shared/                        # 公共组件、常量、工具
│  │  ├─ infra/                         # API、存储、FFI 适配
│  │  └─ main.dart
│  └─ test/
│
└─ app_core/
   ├─ crates/
   │  ├─ core/                          # 核心模型、状态机、错误定义
   │  ├─ controller-client/             # 对接控制面 HTTP/WS
   │  ├─ nat/                           # STUN、NAT 检测、打洞
   │  ├─ p2p/                           # P2P 连接管理
   │  ├─ relay-client/                  # Relay 客户端
   │  ├─ tunnel/                        # Noise 或 WireGuard 封装
   │  ├─ tun/                           # TUN/TAP 适配
   │  ├─ dns/                           # 私有 DNS 与 hosts 处理
   │  ├─ diagnostics/                   # 诊断能力
   │  ├─ platform/                      # 各平台差异封装
   │  └─ ffi-bridge/                    # 对 Flutter 暴露 FFI
   └─ tests/
```

### 3.2 服务端

```text
server/
├─ server-biz/
│  ├─ cmd/
│  │  └─ biz-server/
│  ├─ internal/
│  │  ├─ auth/                          # 注册、登录、JWT、设备认证
│  │  ├─ user/                          # 用户资料
│  │  ├─ network/                       # 网络和成员管理
│  │  ├─ device/                        # 设备注册、状态、分组
│  │  ├─ ipam/                          # 虚拟 IP 分配
│  │  ├─ acl/                           # ACL 策略
│  │  ├─ control/                       # 配置下发与控制逻辑
│  │  ├─ ws/                            # WebSocket 控制信道
│  │  ├─ service/                       # 跨模块编排
│  │  ├─ repo/                          # 数据访问
│  │  └─ infra/                         # DB、Redis、配置、日志、监控
│  ├─ api/
│  │  ├─ http/
│  │  └─ dto/
│  └─ migrations/
│
├─ server-relay/
│  ├─ crates/
│  │  ├─ relay-core/                    # Relay 抽象、路由、会话
│  │  ├─ udp-relay/                     # UDP 中继
│  │  ├─ tcp-relay/                     # TCP 中继
│  │  ├─ auth/                          # 中继票据鉴权
│  │  ├─ session/                       # 会话管理
│  │  └─ metrics/                       # 指标采集
│  └─ tests/
│
└─ server-ops/
   ├─ frontend/                         # 管理后台前端
   ├─ backend/                          # 后台 API
   ├─ jobs/                             # Python 统计任务、报表、计费
   └─ scripts/
```

### 3.3 公共目录

```text
protocol/
├─ openapi/                             # 控制面 API 定义
├─ protobuf/                            # 实时消息与内部协议
└─ errors/                              # 统一错误码

deploy/
├─ docker/
├─ k8s/
└─ local/
```

## 4. 模块职责

### `client/app`

- 负责 UI、交互、页面状态、系统托盘和桌面端应用行为
- 调用 `app_core` 提供的接口，不直接实现组网协议和隧道逻辑
- 只处理展示和用户操作，不承担网络核心职责

### `client/app_core`

- 负责客户端真正的网络核心
- 包括登录后配置拉取、控制信道、NAT 检测、P2P、Relay、隧道、虚拟网卡、DNS 与诊断
- 对上提供统一接口，对下屏蔽不同平台差异

### `server/server-biz`

- 负责控制面业务
- 包括用户、网络、设备、成员权限、IP 分配、ACL、WebSocket 控制信道、配置下发
- 不负责业务流量中继

### `server/server-relay`

- 负责数据面中继
- 处理 P2P 失败后的 UDP 和 TCP 中继
- 关注鉴权、会话管理、转发性能、保活和回收
- 不负责用户系统、网络管理和运营逻辑

### `server/server-ops`

- 负责后台管理、统计分析、报表和计费
- 可以承载网络监控、DAU/MAU、运营工具
- 不进入一期 MVP 主链路

### `protocol`

- 作为唯一协议来源
- 放控制面 API、控制信道消息、Relay 票据、错误码
- 避免 Flutter、Rust、Go 各自维护一套接口

## 5. 当前仓库与目标结构映射

当前仓库已有目录：

```text
client/
├─ app/
└─ app_core/

server/
├─ server-biz/
├─ server-relay/
└─ server-ui/
```

目标命名：

- `server/server-ui` 重命名为 `server/server-ops`

调整原因：

- `server-ops` 比 `server-ui` 更准确表达运营平台职责

## 6. 实施约束

- `client/app` 不实现网络协议与打洞逻辑
- `client/app_core` 不依赖 Flutter 页面结构
- `server/server-biz` 不承担流量中继职责
- `server/server-relay` 不实现用户和网络管理
- `protocol/` 是唯一协议定义来源

## 7. 推荐的工程实践

- `client/app_core` 和 `server/server-relay` 使用 Rust workspace + 多 crate
- `server/server-biz` 使用 Go 标准 `cmd + internal + api` 结构
- 每个服务维护独立配置样例、启动说明和本地开发方式
- 在 `deploy/local/` 中维护本地联调环境
