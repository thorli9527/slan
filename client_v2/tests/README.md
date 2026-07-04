# client_v2 Test Engineering

`client_v2/` 下的业务工程继续保持原有语言生态结构：

- Flutter 业务代码：
  [`app_flutter/lib`](/Users/thorli/workspace/slan/slan/client_v2/app_flutter/lib)
- Flutter 单元测试：
  [`tests/app_flutter/unit`](/Users/thorli/workspace/slan/slan/client_v2/tests/app_flutter/unit)
- Flutter 集成测试：
  [`tests/app_flutter/integration`](/Users/thorli/workspace/slan/slan/client_v2/tests/app_flutter/integration)
- 插件源码：
  [`tests/plugin/source`](/Users/thorli/workspace/slan/slan/client_v2/tests/plugin/source)
- Rust 客户端核心源码：
  [`tests/rust/crates`](/Users/thorli/workspace/slan/slan/client_v2/tests/rust/crates)

外部测试工程与多平台联调入口统一收敛到：

- [`tests/multiplatform/android`](/Users/thorli/workspace/slan/slan/client_v2/tests/multiplatform/android)
- [`tests/multiplatform/ios`](/Users/thorli/workspace/slan/slan/client_v2/tests/multiplatform/ios)
- [`tests/multiplatform/linux`](/Users/thorli/workspace/slan/slan/client_v2/tests/multiplatform/linux)
- [`tests/multiplatform/macos`](/Users/thorli/workspace/slan/slan/client_v2/tests/multiplatform/macos)
- [`tests/multiplatform/matrix`](/Users/thorli/workspace/slan/slan/client_v2/tests/multiplatform/matrix)
- [`tests/multiplatform/shared`](/Users/thorli/workspace/slan/slan/client_v2/tests/multiplatform/shared)

这层拆分的目标是：

- 单元测试仍贴近业务模块
- 平台回归 / 安装验证 / 双端联调 / 多端矩阵转到独立测试工程视图
- `client_v2/` 目录内可以直接看到“业务实现”和“测试工程入口”的边界
