enum ClientCommandType {
  loginWithBrowser,
  enableNetwork,
  disableNetwork,
  logout,
  refresh,
  shutdownNetwork,
  openWebConsole,
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
