class ClientViewState {
  const ClientViewState({
    required this.signedIn,
    required this.networkEnabled,
    required this.syncing,
    required this.switchEnabled,
    this.syncReason,
    this.userLabel,
    this.deviceId,
    this.authCallbackId,
    this.virtualIp,
    this.notice,
    this.error,
  });

  final bool signedIn;
  final String? userLabel;
  final String? deviceId;
  final String? authCallbackId;
  final String? virtualIp;
  final bool networkEnabled;
  final bool syncing;
  final String? syncReason;
  final bool switchEnabled;
  final String? notice;
  final String? error;

  factory ClientViewState.initial() {
    return const ClientViewState(
      signedIn: false,
      networkEnabled: false,
      syncing: false,
      switchEnabled: true,
    );
  }

  factory ClientViewState.fromJson(Map<String, Object?> json) {
    return ClientViewState(
      signedIn: json['signedIn'] == true,
      userLabel: json['userLabel'] as String?,
      deviceId: json['deviceId'] as String?,
      authCallbackId: json['authCallbackId'] as String?,
      virtualIp: json['virtualIp'] as String?,
      networkEnabled: json['networkEnabled'] == true,
      syncing: json['syncing'] == true,
      syncReason: json['syncReason'] as String?,
      switchEnabled: json['switchEnabled'] != false,
      notice: json['notice'] as String?,
      error: json['error'] as String?,
    );
  }

  ClientViewState copyWith({
    bool? signedIn,
    String? userLabel,
    String? deviceId,
    String? authCallbackId,
    String? virtualIp,
    bool? networkEnabled,
    bool? syncing,
    String? syncReason,
    bool? switchEnabled,
    String? notice,
    String? error,
    bool clearSyncReason = false,
  }) {
    return ClientViewState(
      signedIn: signedIn ?? this.signedIn,
      userLabel: userLabel ?? this.userLabel,
      deviceId: deviceId ?? this.deviceId,
      authCallbackId: authCallbackId ?? this.authCallbackId,
      virtualIp: virtualIp ?? this.virtualIp,
      networkEnabled: networkEnabled ?? this.networkEnabled,
      syncing: syncing ?? this.syncing,
      syncReason: clearSyncReason ? null : syncReason ?? this.syncReason,
      switchEnabled: switchEnabled ?? this.switchEnabled,
      notice: notice,
      error: error,
    );
  }
}
