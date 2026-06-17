import 'package:flutter/material.dart';

import '../../../bridge/client_view_state.dart';
import 'compact_info_row.dart';

/// 已登录首页状态面板。
///
/// 集中展示当前用户、虚拟 IP、流量统计、设备 ID 和最近一条客户端消息。
class SignedInStatusPanel extends StatelessWidget {
  const SignedInStatusPanel({
    required this.userLabel,
    required this.currentIp,
    required this.state,
    required this.onToggle,
    super.key,
  });

  /// 当前用户展示名。
  final String userLabel;

  /// 当前虚拟 IP 文案，未启用网络时由页面层传入“未启用”。
  final String currentIp;

  /// bridge 当前 UI 状态。
  final ClientViewState state;

  /// 网络开关回调。
  final ValueChanged<bool> onToggle;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Container(
      padding: const EdgeInsets.fromLTRB(14, 12, 14, 10),
      decoration: BoxDecoration(
        color: theme.colorScheme.surfaceContainerLowest,
        borderRadius: BorderRadius.circular(10),
        border: Border.all(color: theme.colorScheme.outlineVariant),
      ),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          Row(
            crossAxisAlignment: CrossAxisAlignment.center,
            children: [
              Expanded(child: CompactIdentity(userLabel: userLabel)),
              const SizedBox(width: 12),
              _NetworkControl(state: state, onToggle: onToggle),
            ],
          ),
          const SizedBox(height: 10),
          Divider(height: 1, color: theme.colorScheme.outlineVariant),
          const SizedBox(height: 9),
          CompactInfoRow(
            valueKey: const Key('network-ip-value'),
            icon: Icons.router_rounded,
            label: '当前 IP',
            value: currentIp,
          ),
          if (_hasTraffic(state)) ...[
            const SizedBox(height: 7),
            CompactInfoRow(
              valueKey: const Key('client-traffic-total-value'),
              icon: Icons.speed_rounded,
              label: '已用',
              value: _trafficTotalText(state),
            ),
            const SizedBox(height: 7),
            CompactInfoRow(
              valueKey: const Key('client-traffic-current-value'),
              icon: Icons.swap_vert_rounded,
              label: '当前',
              value: _trafficCurrentText(state),
            ),
          ],
          if (_deviceIdText(state) != null) ...[
            const SizedBox(height: 7),
            CompactInfoRow(
              valueKey: const Key('client-device-id-value'),
              icon: Icons.devices_other_rounded,
              label: '设备 ID',
              value: _deviceIdText(state)!,
            ),
          ],
          if (_lastClientMessageText(state) != null) ...[
            const SizedBox(height: 7),
            CompactInfoRow(
              valueKey: const Key('last-client-message-value'),
              icon: Icons.mark_chat_unread_rounded,
              label: '最近消息',
              value: _lastClientMessageText(state)!,
            ),
          ],
        ],
      ),
    );
  }

  /// 格式化最近一条客户端消息。
  String? _lastClientMessageText(ClientViewState state) {
    final body = state.lastClientMessageBody?.trim();
    if (body == null || body.isEmpty) {
      return null;
    }
    final from = state.lastClientMessageFromDeviceId?.trim();
    if (from == null || from.isEmpty) {
      return body;
    }
    return '$from: $body';
  }

  /// 清理设备 ID，空字符串不展示。
  String? _deviceIdText(ClientViewState state) {
    final deviceId = state.deviceId?.trim();
    if (deviceId == null || deviceId.isEmpty) {
      return null;
    }
    return deviceId;
  }

  /// 判断是否已有流量统计。
  bool _hasTraffic(ClientViewState state) {
    return state.trafficTxBytes != null || state.trafficRxBytes != null;
  }

  /// 格式化累计收发流量。
  String _trafficTotalText(ClientViewState state) {
    final txTotal = _formatBytes(state.trafficTxBytes ?? 0);
    final rxTotal = _formatBytes(state.trafficRxBytes ?? 0);
    return '↑$txTotal ↓$rxTotal';
  }

  /// 格式化当前每分钟流量。
  String _trafficCurrentText(ClientViewState state) {
    final txRate = _formatBytes(state.trafficTxBytesPerMinute ?? 0);
    final rxRate = _formatBytes(state.trafficRxBytesPerMinute ?? 0);
    return '↑$txRate/分 ↓$rxRate/分';
  }

  /// 将字节数压缩为 B/K/M 的短文案，避免卡片过宽。
  String _formatBytes(int bytes) {
    if (bytes >= 1024 * 1024) {
      return '${(bytes / (1024 * 1024)).toStringAsFixed(bytes >= 10 * 1024 * 1024 ? 0 : 1)}M';
    }
    if (bytes >= 1024) {
      return '${(bytes / 1024).toStringAsFixed(bytes >= 10 * 1024 ? 0 : 1)}K';
    }
    return '${bytes}B';
  }
}

/// 网络开关及状态标签。
class _NetworkControl extends StatelessWidget {
  const _NetworkControl({required this.state, required this.onToggle});

  /// 当前 UI 状态，决定开关值、loading 和文案。
  final ClientViewState state;

  /// 用户切换网络时的回调。
  final ValueChanged<bool> onToggle;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final enabled = state.networkEnabled;
    final label = state.syncing
        ? '同步中'
        : enabled
            ? '网络已启用'
            : '网络未启用';
    return SizedBox(
      width: 112,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.end,
        mainAxisSize: MainAxisSize.min,
        children: [
          _NetworkSwitch(state: state, onToggle: onToggle),
          const SizedBox(height: 3),
          Text(
            label,
            maxLines: 1,
            overflow: TextOverflow.ellipsis,
            style: theme.textTheme.labelSmall?.copyWith(
              color: enabled
                  ? theme.colorScheme.primary
                  : theme.colorScheme.onSurfaceVariant,
              fontWeight: FontWeight.w800,
            ),
          ),
        ],
      ),
    );
  }
}

/// 固定尺寸的网络开关，避免 loading 指示器出现时布局跳动。
class _NetworkSwitch extends StatelessWidget {
  const _NetworkSwitch({required this.state, required this.onToggle});

  /// 当前 UI 状态，决定开关是否可点。
  final ClientViewState state;

  /// 用户切换网络时的回调。
  final ValueChanged<bool> onToggle;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final switchBusy = state.syncing && !state.switchEnabled;
    return Align(
      alignment: Alignment.centerRight,
      child: Row(
        mainAxisAlignment: MainAxisAlignment.end,
        mainAxisSize: MainAxisSize.min,
        children: [
          SizedBox(
            width: 20,
            height: 16,
            child: switchBusy
                ? CircularProgressIndicator(
                    strokeWidth: 2,
                    color: theme.colorScheme.primary,
                  )
                : null,
          ),
          const SizedBox(width: 4),
          SizedBox(
            height: 36,
            child: Switch(
              key: const Key('network-switch'),
              value: state.networkEnabled,
              onChanged:
                  state.switchEnabled && !state.syncing ? onToggle : null,
            ),
          ),
        ],
      ),
    );
  }
}
