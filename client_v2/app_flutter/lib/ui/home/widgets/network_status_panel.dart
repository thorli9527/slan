import 'package:flutter/material.dart';

import '../../../bridge/client_view_state.dart';
import 'compact_info_row.dart';

/// 已登录首页状态面板。
///
/// 集中展示当前用户、虚拟 IP 和流量统计。
class SignedInStatusPanel extends StatelessWidget {
  const SignedInStatusPanel({
    required this.desktop,
    required this.userLabel,
    required this.currentIp,
    required this.state,
    required this.onToggle,
    this.onAcceptInvite,
    super.key,
  });

  /// 当前用户展示名。
  final String userLabel;

  final bool desktop;

  /// 当前虚拟 IP 文案，未启用网络时由页面层传入“未启用”。
  final String currentIp;

  /// bridge 当前 UI 状态。
  final ClientViewState state;

  /// 网络开关回调。
  final ValueChanged<bool> onToggle;
  final VoidCallback? onAcceptInvite;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Container(
      padding: EdgeInsets.fromLTRB(16, desktop ? 10 : 10, 16, 10),
      decoration: BoxDecoration(
        color: Colors.white,
        borderRadius: BorderRadius.circular(18),
        border: Border.all(color: theme.colorScheme.outlineVariant),
      ),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          Row(
            crossAxisAlignment: CrossAxisAlignment.center,
            children: [
              Expanded(
                child: CompactIdentity(userLabel: userLabel),
              ),
              const SizedBox(width: 12),
              _NetworkControl(
                desktop: desktop,
                state: state,
                onToggle: onToggle,
              ),
            ],
          ),
          if (desktop && onAcceptInvite != null) ...[
            const SizedBox(height: 6),
            Row(
              key: const Key('network-docking-actions'),
              children: [
                CircleAvatar(
                  key: const Key('network-docking-avatar'),
                  radius: 11,
                  backgroundColor: theme.colorScheme.primaryContainer,
                  foregroundColor: theme.colorScheme.onPrimaryContainer,
                  child: const Icon(Icons.link_rounded, size: 14),
                ),
                const SizedBox(width: 10),
                SizedBox(
                  width: 74,
                  child: Text(
                    '网络对接',
                    style: theme.textTheme.labelMedium?.copyWith(
                      fontWeight: FontWeight.w800,
                    ),
                  ),
                ),
                const SizedBox(width: 4),
                TextButton(
                  key: const Key('accept-network-invite'),
                  style: _networkDockingButtonStyle(),
                  onPressed: onAcceptInvite,
                  child: const Text('确认接入'),
                ),
              ],
            ),
          ],
          SizedBox(height: desktop ? 6 : 8),
          CompactInfoRow(
            valueKey: const Key('network-ip-value'),
            icon: Icons.router_rounded,
            label: '当前 IP',
            value: currentIp,
          ),
          if (_hasTraffic(state)) ...[
            const SizedBox(height: 6),
            CompactInfoRow(
              valueKey: const Key('client-traffic-total-value'),
              icon: Icons.speed_rounded,
              label: '已用',
              value: _trafficTotalText(state),
            ),
            const SizedBox(height: 6),
            CompactInfoRow(
              valueKey: const Key('client-traffic-current-value'),
              icon: Icons.swap_vert_rounded,
              label: '当前',
              value: _trafficCurrentText(state),
            ),
          ],
        ],
      ),
    );
  }

  ButtonStyle _networkDockingButtonStyle() {
    return TextButton.styleFrom(
      minimumSize: const Size(0, 28),
      padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 3),
      visualDensity: VisualDensity.compact,
      textStyle: const TextStyle(fontSize: 14, fontWeight: FontWeight.w700),
    );
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
  const _NetworkControl({
    required this.desktop,
    required this.state,
    required this.onToggle,
  });

  final bool desktop;

  /// 当前 UI 状态，决定开关值、loading 和文案。
  final ClientViewState state;

  /// 用户切换网络时的回调。
  final ValueChanged<bool> onToggle;

  @override
  Widget build(BuildContext context) {
    if (desktop) {
      return SizedBox(
        width: 48,
        child: _NetworkSwitch(state: state, onToggle: onToggle),
      );
    }
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
          Container(
            padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 5),
            decoration: BoxDecoration(
              color: enabled
                  ? const Color(0xffebf4ff)
                  : theme.colorScheme.surfaceContainerHigh,
              borderRadius: BorderRadius.circular(999),
            ),
            child: Text(
              label,
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
              style: theme.textTheme.labelMedium?.copyWith(
                color: enabled
                    ? theme.colorScheme.primary
                    : theme.colorScheme.onSurfaceVariant,
                fontWeight: FontWeight.w800,
              ),
            ),
          ),
          const SizedBox(height: 6),
          _NetworkSwitch(state: state, onToggle: onToggle),
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
    final switchBusy = state.syncing;
    final switchInteractive = state.switchEnabled && !switchBusy;
    final enabled = state.networkEnabled;
    return Tooltip(
      message: switchBusy
          ? '正在更新网络'
          : enabled
              ? '停用网络'
              : '启用网络',
      child: SizedBox(
        width: 48,
        height: 30,
        child: Stack(
          alignment: Alignment.centerRight,
          children: [
            if (switchBusy)
              Positioned(
                left: 0,
                child: SizedBox.square(
                  dimension: 12,
                  child: CircularProgressIndicator(
                    strokeWidth: 2,
                    color: theme.colorScheme.primary,
                  ),
                ),
              ),
            Transform.scale(
              scale: 0.68,
              alignment: Alignment.centerRight,
              child: Switch(
                key: const Key('network-switch'),
                value: enabled,
                activeTrackColor: theme.colorScheme.primary,
                activeThumbColor: theme.colorScheme.onPrimary,
                inactiveTrackColor: theme.colorScheme.surfaceContainerHighest,
                inactiveThumbColor: theme.colorScheme.onSurfaceVariant,
                trackOutlineColor: WidgetStateProperty.all(Colors.transparent),
                onChanged: switchInteractive ? onToggle : null,
              ),
            ),
          ],
        ),
      ),
    );
  }
}
