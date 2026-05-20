# .gitignore 约定

本文档说明本仓库 `.gitignore` 的设计目标、覆盖范围与维护方式。

## 目标

- 避免提交构建产物、缓存、临时文件与本机/IDE 相关文件
- 让团队在不同平台（macOS/Linux/Windows）下获得一致的版本库状态
- 通过“分层 + 通配”覆盖多语言/多模块工程（Go / Rust / Flutter / Web）

## 当前仓库结构对应

- 客户端 Flutter：`client/app`
- 客户端 Rust workspace：`client/app_core`
- 服务端 Go：`server/service-biz`（以及后续可能增加的其他 `server/*` 模块）
- 协议与文档：`protocol/`、`docs/`

因此 `.gitignore` 采用 `**/` 与 `server/**/` 的通配规则，避免未来新增模块时反复改规则。

## 规则说明（按类型）

### IDE/编辑器

- `.idea/`、`*.iml`：JetBrains 系列 IDE 工程文件
- `.vscode/`：VS Code 工程配置（如需共享推荐插件/调试配置，可改为精确忽略局部文件）

### 通用临时文件

- `*.log`：运行日志
- `coverage/`：覆盖率输出目录（不同语言/工具可能生成该目录）

### Go（服务端）

- `server/**/bin/`：自定义输出目录
- `server/**/*.exe`、`server/**/*.out`：Windows 二进制与 profiling 输出

说明：只对 `server/` 下做约束，避免误伤其他目录中同名文件。

### Rust（客户端数据面与库）

- `**/target/`：Cargo 构建缓存与产物
- `**/*.rs.bk`：Rustfmt/工具链可能生成的备份文件

注意：当前仓库里已存在 `client/app_core/target/` 产物，建议从版本库中移除后交由 `.gitignore` 管理。

### Flutter/Dart（客户端 UI）

- `**/.dart_tool/`：Dart 工具缓存
- `**/build/`：构建输出
- `**/.flutter-plugins`、`**/.flutter-plugins-dependencies`、`**/.metadata`：Flutter 工程生成文件

### Web/Node（如后续引入）

- `node_modules/`：依赖安装目录
- `dist/`：前端构建输出

### 环境变量文件

- `.env`、`.env.local`、`.env.*.local`：本机配置与敏感信息承载文件（避免被提交）

## 维护原则

- 新增规则时优先选择“最小范围 + 通配”的组合，避免误忽略业务源码
- 当某类文件确实需要提交时，采用“白名单”方式：在规则中添加更精确的 `!path` 反向规则

