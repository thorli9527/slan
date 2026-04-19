/// 连接路径枚举，表示当前走 P2P 还是 Relay。
enum ConnectionPathModel {
  p2p,
  relay,
}

/// 连接状态模型。
class ConnectionStateModel {
  const ConnectionStateModel.disconnected()
      : status = 'disconnected',
        path = null,
        reason = null;

  const ConnectionStateModel.connecting()
      : status = 'connecting',
        path = null,
        reason = null;

  const ConnectionStateModel.connected(this.path)
      : status = 'connected',
        reason = null;

  const ConnectionStateModel.failed(this.reason)
      : status = 'failed',
        path = null;

  final String status;
  final ConnectionPathModel? path;
  final String? reason;
}
