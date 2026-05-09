enum ClientCommandType {
  loginWithBrowser,
  loginWithPassword,
  enableNetwork,
  disableNetwork,
  logout,
  refresh,
  localNetworkShutdown,
  openWebConsole,
  sendClientMessage,
}

class ClientCommand {
  const ClientCommand(this.type, [this.payload]);

  final ClientCommandType type;
  final Map<String, Object?>? payload;

  Map<String, Object?> toJson() {
    return {
      'type': type.name,
      if (payload != null) 'payload': payload,
    };
  }
}
