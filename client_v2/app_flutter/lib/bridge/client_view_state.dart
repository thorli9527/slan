/// ClientViewState 是 Flutter UI 渲染首页、网络开关、消息和流量统计的状态快照。
class ClientViewState {
  const ClientViewState({
    required this.signedIn,
    required this.networkEnabled,
    required this.syncing,
    required this.switchEnabled,
    this.syncReason,
    this.userLabel,
    this.deviceId,
    this.virtualIp,
    this.notice,
    this.error,
    this.errorSource,
    this.lastClientMessageId,
    this.lastClientMessageFromDeviceId,
    this.lastClientMessageBody,
    this.trafficTxBytes,
    this.trafficRxBytes,
    this.trafficTxBytesPerMinute,
    this.trafficRxBytesPerMinute,
    this.trafficUpdatedAtMs,
  });

  /// 是否已登录。
  final bool signedIn;

  /// 当前用户展示名或邮箱。
  final String? userLabel;

  /// 当前设备 ID。
  final String? deviceId;

  /// 当前虚拟 IP。
  final String? virtualIp;

  /// 虚拟网络是否启用。
  final bool networkEnabled;

  /// 是否正在同步或切换网络。
  final bool syncing;

  /// 当前同步原因。
  final String? syncReason;

  /// UI 网络开关是否可操作。
  final bool switchEnabled;

  /// 最近一次提示消息。
  final String? notice;

  /// 最近一次错误消息。
  final String? error;

  /// 错误来源，用于 UI 区分网络切换、登录等场景。
  final String? errorSource;

  /// 最近收到的客户端消息 ID。
  final String? lastClientMessageId;

  /// 最近收到消息的发送设备 ID。
  final String? lastClientMessageFromDeviceId;

  /// 最近收到的消息正文。
  final String? lastClientMessageBody;

  /// 累计发送字节数。
  final int? trafficTxBytes;

  /// 累计接收字节数。
  final int? trafficRxBytes;

  /// 每分钟发送字节速率。
  final int? trafficTxBytesPerMinute;

  /// 每分钟接收字节速率。
  final int? trafficRxBytesPerMinute;

  /// 流量统计更新时间，Unix 毫秒。
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
      virtualIp: networkEnabled ? _virtualIp(json['virtualIp']) : null,
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
      virtualIp: clearVirtualIp
          ? null
          : _virtualIp(virtualIp) ?? _virtualIp(this.virtualIp),
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

String? _virtualIp(Object? value) {
  final text = value is String ? value.trim() : '';
  if (text.isEmpty) {
    return null;
  }
  return text.split('/').first.trim();
}

abstract final class ClientErrorSource {
  static const networkSwitch = 'networkSwitch';
}
