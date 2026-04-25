# app_core 对外输出功能

本文档描述 `app_core` 向上层应用输出什么能力。

这里的“对外”主要指：

- Flutter UI
- 未来桌面壳层或命令行壳层
- 任何通过 FFI 使用 `app_core` 的上层调用方

## 1. 输出目标

`app_core` 对上层的输出应该是：

- 稳定的业务动作接口
- 稳定的结果模型
- 尽量少暴露底层网络实现细节

上层应该关心：

- 能不能登录
- 设备/节点是否创建成功
- 网络是否准备好
- 当前连接状态是什么
- 当前走的是哪条路径

上层不应该直接关心：

- NAT 打洞细节
- DERP 连接池内部评分
- 具体切换算法
- 单个探测样本

## 2. 当前对外输出功能

### 2.1 身份与网络管理

- 注册账号
- 登录账号
- 注册设备
- 注册节点
- 查询网络列表
- 创建网络
- 按网络 ID、宿主邮箱或 join key 加入网络
- 激活 / 停用选中网络
- 更新当前设备 attachment 备注，用于加入网络时保存别名

### 2.2 配置获取

- 获取指定节点和网络的 `bootstrap`
- 获取控制通道配置
- 获取 STUN 列表
- 获取 relay / DERP 基础配置

### 2.3 连接控制

- 发起连接
- 断开连接
- 申请 relay 回退票据

### 2.4 状态输出

- `ConnectionState`
  - `Disconnected`
  - `Connecting`
  - `Connected`
  - `Failed`
- `ConnectionPath`
  - `P2P`
  - `Relay`
  - `Derp`

## 3. 当前对外门面

当前对 Flutter 的主要门面是 `crates/ffi-bridge` 中的 `AppCoreFacade`。

它当前提供：

- `register`
- `login`
- `refresh_session`
- `register_device`
- `register_node`
- `list_networks`
- `create_network`
- `join_network`
- `join_network_by_owner_email`
- `join_network_by_key`
- `switch_network`
- `activate_network`
- `deactivate_network`
- `update_attachment_remark`
- `bootstrap`
- `issue_relay_ticket`
- `connect`
- `disconnect`

这意味着上层看到的是“动作型接口”，而不是直接操作底层模块。

## 4. 对外输出模型

当前主要对外模型包括：

- `Session`
- `Device`
- `Node`
- `Network`
- `NetworkJoinResult`
- `BootstrapConfig`
- `RelayTicket`
- `ConnectionState`

其中：

- `BootstrapConfig` 是上层进入运行态的核心输入
- `ConnectionState` 是上层展示状态的核心输出

## 5. 对外不应直接输出的内容

以下模型不建议直接开放给上层：

- `DerpPoolState`
- `DerpLinkSnapshot`
- `DerpHealth`
- `ProbeSample`
- `SwitchReason`

这些更适合：

- 内部调度
- 调试页
- 诊断模式

而不是普通业务页面。

## 6. 后续建议的对外扩展

如果后续需要更丰富的状态展示，可以新增只读接口，而不是直接暴露内部 trait：

- `get_connection_diagnostics()`
- `get_active_path()`
- `list_derp_links()`

建议策略：

- 默认门面保持简洁
- 调试与诊断能力通过单独查询接口补充

## 7. 对外输出原则

1. 上层看结果，不看调度过程
2. 上层看状态，不看连接池内部细节
3. 上层通过门面调用，不直接依赖底层 crate
