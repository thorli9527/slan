/// Flutter UI 可派发给 client-core-service 或移动端内嵌服务的命令类型。
///
/// 命令枚举保持和 Rust local service 的 JSON API 语义对齐，页面层只关心
/// 用户动作，不直接拼接底层 method 名。
enum ClientCommandType {
  /// 启用当前设备的虚拟网络。
  enableNetwork,

  /// 停用当前设备的虚拟网络。
  disableNetwork,

  /// 重新拉取本地状态。
  refresh,

  /// 关闭本地网络数据面，常用于应用退出或系统清理。
  localNetworkShutdown,
}

/// UI 层传递给 bridge 的统一命令对象。
///
/// [payload] 只承载命令特有参数。
/// 无参数命令保持 payload 为空，序列化时不会输出 payload 字段。
class ClientCommand {
  const ClientCommand(this.type, [this.payload]);

  /// 命令类型。
  final ClientCommandType type;

  /// 命令附加参数，最终会转成 JSON 发送给本地服务或内嵌服务。
  final Map<String, Object?>? payload;

  /// 转换成 bridge/service 约定的 JSON 结构。
  Map<String, Object?> toJson() {
    return {
      'type': type.name,
      if (payload != null) 'payload': payload,
    };
  }
}
