import 'package:flutter/material.dart';

import '../../../bridge/client_view_state.dart';
import 'compact_info_row.dart';

/// 已登录首页状态面板。
///
/// 集中展示虚拟 IP、网络开关和流量统计。
class ActivatedStatusPanel extends StatelessWidget {
  const ActivatedStatusPanel({
    required this.desktop,
    required this.currentIp,
    required this.state,
    required this.onToggle,
    this.viewportHeight,
    super.key,
  });

  final bool desktop;

  /// 当前设备专属虚拟 IP 文案，与网络开关状态独立。
  final String currentIp;

  /// bridge 当前 UI 状态。
  final ClientViewState state;

  /// 网络开关回调。
  final ValueChanged<bool> onToggle;

  /// 桌面端可视区域高度；非空时面板在其中垂直居中，避免顶部钉住留下大片空白。
  final double? viewportHeight;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final panel = Container(
      padding: EdgeInsets.fromLTRB(16, desktop ? 10 : 10, 16, 10),
      decoration: BoxDecoration(
        color: Colors.white,
        borderRadius: BorderRadius.circular(18),
        border: Border.all(color: theme.colorScheme.outlineVariant),
      ),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          if (desktop)
            Row(
              children: [
                Expanded(child: _buildIpRow()),
                const SizedBox(width: 14),
                _NetworkControl(
                  desktop: true,
                  state: state,
                  onToggle: onToggle,
                ),
              ],
            )
          else ...[
            Row(
              children: [
                Icon(
                  Icons.hub_outlined,
                  size: 22,
                  color: theme.colorScheme.primary,
                ),
                const SizedBox(width: 9),
                Expanded(
                  child: Text(
                    '虚拟网络',
                    style: theme.textTheme.titleSmall?.copyWith(
                      fontWeight: FontWeight.w800,
                    ),
                  ),
                ),
                _NetworkControl(
                  desktop: false,
                  state: state,
                  onToggle: onToggle,
                ),
              ],
            ),
            const SizedBox(height: 8),
            _buildIpRow(),
          ],
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
          if (desktop) ...[
            const SizedBox(height: 8),
            const Divider(height: 1),
            const SizedBox(height: 8),
            _StatusFooter(state: state),
          ],
        ],
      ),
    );
    final viewportHeight = this.viewportHeight;
    if (!desktop || viewportHeight == null) {
      return panel;
    }
    return LayoutBuilder(
      builder: (context, _) {
        return Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            SizedBox(
              height: _verticalFiller(viewportHeight),
            ),
            panel,
          ],
        );
      },
    );
  }

  /// 估算面板自身高度，计算垂直居中所需的顶部填充。
  double _verticalFiller(double viewportHeight) {
    final contentHeight =
        20 + // 面板上下 padding
        30 + // IP + 开关行
        17 + // 分隔线 + 状态栏行（含间距）
        (_hasTraffic(state) ? 2 * 30 : 0);
    final remainder = viewportHeight - contentHeight;
    return remainder > 0 ? remainder / 2 : 0;
  }

  Widget _buildIpRow() {
    return Row(
      children: [
        Expanded(
          child: CompactInfoRow(
            valueKey: const Key('network-ip-value'),
            icon: Icons.router_rounded,
            label: '当前 IP',
            value: currentIp,
          ),
        ),
        const SizedBox(width: 8),
        _TrafficIndicator(state: state),
      ],
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

/// 面板底部状态栏，展示网络状态胶囊与信号/流量概要。
class _StatusFooter extends StatelessWidget {
  const _StatusFooter({required this.state});

  final ClientViewState state;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final syncing = state.syncing;
    final enabled = state.networkEnabled;
    final pillColor = syncing
        ? theme.colorScheme.primary
        : enabled
            ? const Color(0xff2e9e5b)
            : theme.colorScheme.onSurfaceVariant;
    final pillBackground = syncing
        ? theme.colorScheme.primaryContainer
        : enabled
            ? const Color(0xffe3f3ea)
            : theme.colorScheme.surfaceContainerHigh;
    final label = syncing ? '同步中' : enabled ? '已连接' : '未连接';
    final trailing = _footerTrailing(theme);
    return Row(
      children: [
        Container(
          padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 4),
          decoration: BoxDecoration(
            color: pillBackground,
            borderRadius: BorderRadius.circular(999),
          ),
          child: Row(
            mainAxisSize: MainAxisSize.min,
            children: [
              if (syncing)
                Padding(
                  padding: const EdgeInsets.only(right: 6),
                  child: SizedBox.square(
                    dimension: 10,
                    child: CircularProgressIndicator(
                      strokeWidth: 1.6,
                      color: theme.colorScheme.primary,
                    ),
                  ),
                )
              else
                Container(
                  width: 7,
                  height: 7,
                  margin: const EdgeInsets.only(right: 6),
                  decoration: BoxDecoration(
                    color: pillColor,
                    shape: BoxShape.circle,
                  ),
                ),
              Text(
                label,
                style: theme.textTheme.labelSmall?.copyWith(
                  color: pillColor,
                  fontWeight: FontWeight.w800,
                ),
              ),
            ],
          ),
        ),
        if (trailing != null) ...[
          const Spacer(),
          trailing,
        ],
      ],
    );
  }

  /// 右侧概要：优先信号质量，其次当前速率。
  Widget? _footerTrailing(ThemeData theme) {
    final score = state.signalScore;
    if (score != null) {
      return Text(
        '信号 $score',
        style: theme.textTheme.labelSmall?.copyWith(
          color: theme.colorScheme.onSurfaceVariant,
        ),
      );
    }
    final tx = state.trafficTxBytesPerMinute;
    final rx = state.trafficRxBytesPerMinute;
    if (tx == null && rx == null) {
      return null;
    }
    return Text(
      '↑${_formatBytes(tx ?? 0)}/分 ↓${_formatBytes(rx ?? 0)}/分',
      style: theme.textTheme.labelSmall?.copyWith(
        color: theme.colorScheme.onSurfaceVariant,
      ),
    );
  }

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

/// IP 行右侧的流量/同步指示点，同步中显示转圈，有流量时高亮。
class _TrafficIndicator extends StatelessWidget {
  const _TrafficIndicator({required this.state});

  final ClientViewState state;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    if (state.syncing) {
      return SizedBox.square(
        dimension: 14,
        child: CircularProgressIndicator(
          strokeWidth: 2,
          color: theme.colorScheme.primary,
        ),
      );
    }
    final active = state.networkEnabled &&
        ((state.trafficTxBytesPerMinute ?? 0) > 0 ||
            (state.trafficRxBytesPerMinute ?? 0) > 0);
    return Tooltip(
      message: active ? '数据传输中' : '空闲',
      child: Container(
        width: 8,
        height: 8,
        decoration: BoxDecoration(
          color: active
              ? const Color(0xff2e9e5b)
              : theme.colorScheme.surfaceContainerHighest,
          shape: BoxShape.circle,
        ),
      ),
    );
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
