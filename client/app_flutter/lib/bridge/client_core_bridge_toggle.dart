import 'client_commands.dart';
import 'client_view_state.dart';

/// NetworkToggleOperation 记录一次网络开关操作的上下文，用于异步回调回来时
/// 判断结果是否仍属于当前最新操作。
class NetworkToggleOperation {
  const NetworkToggleOperation({
    required this.epoch,
    required this.command,
    required this.method,
    required this.targetEnabled,
    required this.previousState,
  });

  /// 操作序号。新操作会递增 epoch，旧异步回调不能覆盖新状态。
  final int epoch;

  /// 用户触发的原始命令。
  final ClientCommandType command;

  /// 本次操作对应的底层 service method，便于日志和诊断。
  final String method;

  /// 本次操作期望的网络状态。
  final bool targetEnabled;

  /// 操作开始前的 UI 状态，失败时用于回滚可见状态。
  final ClientViewState previousState;
}
