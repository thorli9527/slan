/// ClientViewState 是 Flutter UI 渲染首页、网络开关和流量统计的状态快照。
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
    this.lastControlSyncMessageType,
    this.lastControlSyncReconfigureRequired,
    this.trafficTxBytes,
    this.trafficRxBytes,
    this.trafficTxBytesPerMinute,
    this.trafficRxBytesPerMinute,
    this.trafficUpdatedAtMs,
    this.signalScore,
    this.signalQuality,
    this.signalPath,
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

  /// 最近一次控制面同步事件类型，例如 `network_event`。
  final String? lastControlSyncMessageType;

  /// 最近一次控制面同步事件是否要求客户端重配数据面。
  final bool? lastControlSyncReconfigureRequired;

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

  final int? signalScore;
  final String? signalQuality;
  final String? signalPath;

  /// 构造未登录、网络未启用的默认 UI 状态。
  factory ClientViewState.initial() {
    return const ClientViewState(
      signedIn: false,
      networkEnabled: false,
      syncing: false,
      switchEnabled: true,
    );
  }

  /// 从本地服务或移动端内嵌服务返回的 JSON 解析 UI 状态。
  ///
  /// 服务端在网络未启用时可能仍保留历史虚拟 IP；这里仅在
  /// [networkEnabled] 为 true 时展示虚拟 IP，避免 UI 显示过期地址。
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
      lastControlSyncMessageType: json['lastControlSyncMessageType'] as String?,
      lastControlSyncReconfigureRequired:
          json['lastControlSyncReconfigureRequired'] as bool?,
      trafficTxBytes: _intValue(json['trafficTxBytes']),
      trafficRxBytes: _intValue(json['trafficRxBytes']),
      trafficTxBytesPerMinute: _intValue(json['trafficTxBytesPerMinute']),
      trafficRxBytesPerMinute: _intValue(json['trafficRxBytesPerMinute']),
      trafficUpdatedAtMs: _intValue(json['trafficUpdatedAtMs']),
      signalScore: _intValue(json['signalScore']),
      signalQuality: json['signalQuality'] as String?,
      signalPath: json['signalPath'] as String?,
    );
  }

  /// 基于当前状态创建新状态。
  ///
  /// [clearSyncReason] 和 [clearVirtualIp] 用于显式清空可空字段；其他可空
  /// 字段按业务语义处理，notice/error 传入 null 表示清除当前提示或错误。
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
    String? lastControlSyncMessageType,
    bool? lastControlSyncReconfigureRequired,
    int? trafficTxBytes,
    int? trafficRxBytes,
    int? trafficTxBytesPerMinute,
    int? trafficRxBytesPerMinute,
    int? trafficUpdatedAtMs,
    int? signalScore,
    String? signalQuality,
    String? signalPath,
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
      lastControlSyncMessageType:
          lastControlSyncMessageType ?? this.lastControlSyncMessageType,
      lastControlSyncReconfigureRequired: lastControlSyncReconfigureRequired ??
          this.lastControlSyncReconfigureRequired,
      trafficTxBytes: trafficTxBytes ?? this.trafficTxBytes,
      trafficRxBytes: trafficRxBytes ?? this.trafficRxBytes,
      trafficTxBytesPerMinute:
          trafficTxBytesPerMinute ?? this.trafficTxBytesPerMinute,
      trafficRxBytesPerMinute:
          trafficRxBytesPerMinute ?? this.trafficRxBytesPerMinute,
      trafficUpdatedAtMs: trafficUpdatedAtMs ?? this.trafficUpdatedAtMs,
      signalScore: signalScore ?? this.signalScore,
      signalQuality: signalQuality ?? this.signalQuality,
      signalPath: signalPath ?? this.signalPath,
    );
  }

  /// 状态等价判断用于 [ValueNotifier] 更新时避免重复刷新 UI。
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
            lastControlSyncMessageType == other.lastControlSyncMessageType &&
            lastControlSyncReconfigureRequired ==
                other.lastControlSyncReconfigureRequired &&
            trafficTxBytes == other.trafficTxBytes &&
            trafficRxBytes == other.trafficRxBytes &&
            trafficTxBytesPerMinute == other.trafficTxBytesPerMinute &&
            trafficRxBytesPerMinute == other.trafficRxBytesPerMinute &&
            trafficUpdatedAtMs == other.trafficUpdatedAtMs &&
            signalScore == other.signalScore &&
            signalQuality == other.signalQuality &&
            signalPath == other.signalPath;
  }

  /// 与 [operator ==] 保持字段集合一致。
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
      lastControlSyncMessageType,
      lastControlSyncReconfigureRequired,
      trafficTxBytes,
      trafficRxBytes,
      trafficTxBytesPerMinute,
      trafficRxBytesPerMinute,
      trafficUpdatedAtMs,
      signalScore,
      signalQuality,
      signalPath,
    ]);
  }

  /// 将服务端可能返回的 int/double/string 数字安全转为 int。
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

/// 规约虚拟 IP 字段。
///
/// 服务端历史上可能返回 `10.0.0.2/32` 或空字符串；UI 只展示纯 IPv4。
String? _virtualIp(Object? value) {
  final text = value is String ? value.trim() : '';
  if (text.isEmpty) {
    return null;
  }
  return text.split('/').first.trim();
}

/// UI 错误来源常量。
///
/// 目前主要用于识别网络开关失败，从而弹出更明确的错误对话框。
abstract final class ClientErrorSource {
  static const networkSwitch = 'networkSwitch';
}
