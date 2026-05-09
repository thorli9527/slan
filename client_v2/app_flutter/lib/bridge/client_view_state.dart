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
    this.lastClientMessageId,
    this.lastClientMessageFromDeviceId,
    this.lastClientMessageBody,
    this.lastRelayPolicyId,
    this.lastRelayPolicyUpdatedAtMs,
    this.trafficTxBytes,
    this.trafficRxBytes,
    this.trafficTxBytesPerMinute,
    this.trafficRxBytesPerMinute,
    this.trafficUpdatedAtMs,
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
  final String? lastClientMessageId;
  final String? lastClientMessageFromDeviceId;
  final String? lastClientMessageBody;
  final String? lastRelayPolicyId;
  final int? lastRelayPolicyUpdatedAtMs;
  final int? trafficTxBytes;
  final int? trafficRxBytes;
  final int? trafficTxBytesPerMinute;
  final int? trafficRxBytesPerMinute;
  final int? trafficUpdatedAtMs;

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
      lastClientMessageId: json['lastClientMessageId'] as String?,
      lastClientMessageFromDeviceId:
          json['lastClientMessageFromDeviceId'] as String?,
      lastClientMessageBody: json['lastClientMessageBody'] as String?,
      lastRelayPolicyId: json['lastRelayPolicyId'] as String?,
      lastRelayPolicyUpdatedAtMs: json['lastRelayPolicyUpdatedAtMs'] as int?,
      trafficTxBytes: _intValue(json['trafficTxBytes']),
      trafficRxBytes: _intValue(json['trafficRxBytes']),
      trafficTxBytesPerMinute: _intValue(json['trafficTxBytesPerMinute']),
      trafficRxBytesPerMinute: _intValue(json['trafficRxBytesPerMinute']),
      trafficUpdatedAtMs: _intValue(json['trafficUpdatedAtMs']),
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
    String? lastClientMessageId,
    String? lastClientMessageFromDeviceId,
    String? lastClientMessageBody,
    String? lastRelayPolicyId,
    int? lastRelayPolicyUpdatedAtMs,
    int? trafficTxBytes,
    int? trafficRxBytes,
    int? trafficTxBytesPerMinute,
    int? trafficRxBytesPerMinute,
    int? trafficUpdatedAtMs,
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
      lastClientMessageId: lastClientMessageId ?? this.lastClientMessageId,
      lastClientMessageFromDeviceId:
          lastClientMessageFromDeviceId ?? this.lastClientMessageFromDeviceId,
      lastClientMessageBody:
          lastClientMessageBody ?? this.lastClientMessageBody,
      lastRelayPolicyId: lastRelayPolicyId ?? this.lastRelayPolicyId,
      lastRelayPolicyUpdatedAtMs:
          lastRelayPolicyUpdatedAtMs ?? this.lastRelayPolicyUpdatedAtMs,
      trafficTxBytes: trafficTxBytes ?? this.trafficTxBytes,
      trafficRxBytes: trafficRxBytes ?? this.trafficRxBytes,
      trafficTxBytesPerMinute:
          trafficTxBytesPerMinute ?? this.trafficTxBytesPerMinute,
      trafficRxBytesPerMinute:
          trafficRxBytesPerMinute ?? this.trafficRxBytesPerMinute,
      trafficUpdatedAtMs: trafficUpdatedAtMs ?? this.trafficUpdatedAtMs,
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
            errorSource == other.errorSource &&
            lastClientMessageId == other.lastClientMessageId &&
            lastClientMessageFromDeviceId ==
                other.lastClientMessageFromDeviceId &&
            lastClientMessageBody == other.lastClientMessageBody &&
            lastRelayPolicyId == other.lastRelayPolicyId &&
            lastRelayPolicyUpdatedAtMs == other.lastRelayPolicyUpdatedAtMs &&
            trafficTxBytes == other.trafficTxBytes &&
            trafficRxBytes == other.trafficRxBytes &&
            trafficTxBytesPerMinute == other.trafficTxBytesPerMinute &&
            trafficRxBytesPerMinute == other.trafficRxBytesPerMinute &&
            trafficUpdatedAtMs == other.trafficUpdatedAtMs;
  }

  @override
  int get hashCode {
    return Object.hashAll([
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
      lastClientMessageId,
      lastClientMessageFromDeviceId,
      lastClientMessageBody,
      lastRelayPolicyId,
      lastRelayPolicyUpdatedAtMs,
      trafficTxBytes,
      trafficRxBytes,
      trafficTxBytesPerMinute,
      trafficRxBytesPerMinute,
      trafficUpdatedAtMs,
    ]);
  }

  static int? _intValue(Object? value) {
    if (value is int) {
      return value;
    }
    if (value is num) {
      return value.toInt();
    }
    if (value is String) {
      return int.tryParse(value);
    }
    return null;
  }
}

abstract final class ClientErrorSource {
  static const networkSwitch = 'networkSwitch';
}
