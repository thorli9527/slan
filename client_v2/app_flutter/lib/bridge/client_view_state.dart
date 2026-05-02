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
    this.errorSource,
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
  final String? errorSource;

  factory ClientViewState.initial() {
    return const ClientViewState(
      signedIn: false,
      networkEnabled: false,
      syncing: false,
      switchEnabled: true,
    );
  }

  factory ClientViewState.fromJson(Map<String, Object?> json) {
    final networkEnabled = json['networkEnabled'] == true;
    return ClientViewState(
      signedIn: json['signedIn'] == true,
      userLabel: json['userLabel'] as String?,
      deviceId: json['deviceId'] as String?,
      authCallbackId: json['authCallbackId'] as String?,
      virtualIp: networkEnabled ? json['virtualIp'] as String? : null,
      networkEnabled: networkEnabled,
      syncing: json['syncing'] == true,
      syncReason: json['syncReason'] as String?,
      switchEnabled: json['switchEnabled'] != false,
      notice: json['notice'] as String?,
      error: json['error'] as String?,
      errorSource: json['errorSource'] as String?,
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
    String? errorSource,
    bool clearSyncReason = false,
    bool clearVirtualIp = false,
  }) {
    return ClientViewState(
      signedIn: signedIn ?? this.signedIn,
      userLabel: userLabel ?? this.userLabel,
      deviceId: deviceId ?? this.deviceId,
      authCallbackId: authCallbackId ?? this.authCallbackId,
      virtualIp: clearVirtualIp ? null : virtualIp ?? this.virtualIp,
      networkEnabled: networkEnabled ?? this.networkEnabled,
      syncing: syncing ?? this.syncing,
      syncReason: clearSyncReason ? null : syncReason ?? this.syncReason,
      switchEnabled: switchEnabled ?? this.switchEnabled,
      notice: notice,
      error: error,
      errorSource: errorSource,
    );
  }

  @override
  bool operator ==(Object other) {
    return identical(this, other) ||
        other is ClientViewState &&
            signedIn == other.signedIn &&
            userLabel == other.userLabel &&
            deviceId == other.deviceId &&
            authCallbackId == other.authCallbackId &&
            virtualIp == other.virtualIp &&
            networkEnabled == other.networkEnabled &&
            syncing == other.syncing &&
            syncReason == other.syncReason &&
            switchEnabled == other.switchEnabled &&
            notice == other.notice &&
            error == other.error &&
            errorSource == other.errorSource;
  }

  @override
  int get hashCode {
    return Object.hash(
      signedIn,
      userLabel,
      deviceId,
      authCallbackId,
      virtualIp,
      networkEnabled,
      syncing,
      syncReason,
      switchEnabled,
      notice,
      error,
      errorSource,
    );
  }
}

abstract final class ClientErrorSource {
  static const networkSwitch = 'networkSwitch';
}
