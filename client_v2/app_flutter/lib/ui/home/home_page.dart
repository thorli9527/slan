import 'package:flutter/material.dart';

import '../../bridge/client_commands.dart';
import '../../bridge/client_core_bridge.dart';
import '../../bridge/client_view_state.dart';

class HomePage extends StatefulWidget {
  const HomePage({required this.bridge, super.key});

  final ClientCoreBridge bridge;

  @override
  State<HomePage> createState() => _HomePageState();
}

class _HomePageState extends State<HomePage> {
  @override
  void initState() {
    super.initState();
    widget.bridge.start();
  }

  @override
  void dispose() {
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      body: SafeArea(
        child: ValueListenableBuilder<ClientViewState>(
          valueListenable: widget.bridge.state,
          builder: (context, state, _) {
            return Padding(
              padding: const EdgeInsets.fromLTRB(18, 16, 18, 14),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.stretch,
                children: [
                  if (state.signedIn)
                    _SignedInStatus(
                      state: state,
                      ipText: _ipText(state),
                      onToggle: _toggleNetwork,
                    )
                  else
                    const _SignedOutStatus(),
                  if (state.error != null)
                    _MessageCard(text: _friendlyError(state.error!), error: true),
                  if (state.notice != null) _MessageCard(text: state.notice!),
                  if (state.signedIn) ...[
                    const SizedBox(height: 10),
                    _SignedInActions(
                      syncing: state.syncing,
                      onOpenConsole: () => widget.bridge.dispatch(
                        const ClientCommand(ClientCommandType.openWebConsole),
                      ),
                      onLogout: () => widget.bridge.dispatch(
                        const ClientCommand(ClientCommandType.logout),
                      ),
                    ),
                  ] else ...[
                    const Spacer(),
                    SizedBox(
                      height: 38,
                      child: FilledButton(
                        onPressed: state.syncing
                            ? null
                            : () => widget.bridge.dispatch(
                                  const ClientCommand(
                                    ClientCommandType.loginWithBrowser,
                                  ),
                                ),
                        child: const Text('Login'),
                      ),
                    ),
                  ],
                ],
              ),
            );
          },
        ),
      ),
    );
  }

  void _toggleNetwork(bool enabled) {
    widget.bridge.dispatch(
      ClientCommand(
        enabled
            ? ClientCommandType.enableNetwork
            : ClientCommandType.disableNetwork,
      ),
    );
  }

  String _friendlyError(String error) {
    final normalized = error.toLowerCase();
    if (normalized.contains('device unavailable')) {
      return '设备不可用，请联系管理员重新启用';
    }
    if (normalized.contains('adapter') || normalized.contains('network command')) {
      return '本地网络组件不可用，请重新安装或修复客户端';
    }
    if (normalized.contains('backend is not implemented')) {
      return '当前平台暂未启用本地组网后端';
    }
    if (normalized.contains('local service')) {
      return '本地服务未连接，请稍后重试或重启客户端';
    }
    return error;
  }

  String _ipText(ClientViewState state) {
    if (state.virtualIp != null && state.virtualIp!.trim().isNotEmpty) {
      return state.virtualIp!;
    }
    if (state.syncing) {
      return '同步中';
    }
    if (!state.signedIn) {
      return '未登录';
    }
    if (state.error != null &&
        state.error!.toLowerCase().contains('device unavailable')) {
      return '未分配';
    }
    return '等待分配';
  }
}

class _SignedInStatus extends StatelessWidget {
  const _SignedInStatus({
    required this.state,
    required this.ipText,
    required this.onToggle,
  });

  final ClientViewState state;
  final String ipText;
  final ValueChanged<bool> onToggle;

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        LayoutBuilder(
          builder: (context, constraints) {
            final compact = constraints.maxWidth < 360;
            final userInfo = _CompactIdentity(userLabel: _userLabel());
            final toggle = SizedBox(
              width: compact ? double.infinity : 92,
              height: 40,
              child: _NetworkSwitch(state: state, onToggle: onToggle),
            );

            if (compact) {
              return Column(
                crossAxisAlignment: CrossAxisAlignment.stretch,
                children: [
                  SizedBox(height: 44, child: userInfo),
                  const SizedBox(height: 8),
                  toggle,
                ],
              );
            }

            return Row(
              crossAxisAlignment: CrossAxisAlignment.center,
              children: [
                Expanded(child: userInfo),
                const SizedBox(width: 8),
                Align(alignment: Alignment.centerRight, child: toggle),
              ],
            );
          },
        ),
        const SizedBox(height: 8),
        _CompactInfoPill(
          icon: Icons.router_rounded,
          label: '当前 IP',
          value: ipText,
        ),
      ],
    );
  }

  String _userLabel() {
    final user = state.userLabel?.trim();
    return user == null || user.isEmpty ? '-' : user;
  }
}

class _NetworkSwitch extends StatelessWidget {
  const _NetworkSwitch({required this.state, required this.onToggle});

  final ClientViewState state;
  final ValueChanged<bool> onToggle;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final switchBusy = state.syncing &&
        (state.syncReason == 'enableNetwork' ||
            state.syncReason == 'disableNetwork' ||
            state.syncReason == 'shutdownNetwork');
    return Align(
      alignment: Alignment.centerRight,
      child: AnimatedSize(
        duration: const Duration(milliseconds: 180),
        curve: Curves.easeOutCubic,
        alignment: Alignment.centerRight,
        child: Row(
          mainAxisAlignment: MainAxisAlignment.end,
          mainAxisSize: MainAxisSize.min,
          children: [
            AnimatedSwitcher(
              duration: const Duration(milliseconds: 160),
              switchInCurve: Curves.easeOutCubic,
              switchOutCurve: Curves.easeInCubic,
              child: switchBusy
                  ? Padding(
                      key: const ValueKey('network-toggle-progress'),
                      padding: const EdgeInsets.only(right: 4),
                      child: SizedBox(
                        width: 16,
                        height: 16,
                        child: CircularProgressIndicator(
                          strokeWidth: 2,
                          color: theme.colorScheme.primary,
                        ),
                      ),
                    )
                  : const SizedBox(
                      key: ValueKey('network-toggle-idle'),
                      width: 0,
                      height: 16,
                    ),
            ),
            AnimatedScale(
              duration: const Duration(milliseconds: 160),
              curve: Curves.easeOutCubic,
              scale: switchBusy ? 0.96 : 1,
              child: SizedBox(
                height: 36,
                child: Switch(
                  value: state.networkEnabled,
                  onChanged:
                      state.switchEnabled && !state.syncing ? onToggle : null,
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }
}

class _CompactIdentity extends StatelessWidget {
  const _CompactIdentity({required this.userLabel});

  final String userLabel;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Row(
      children: [
        Icon(
          Icons.account_circle_outlined,
          size: 22,
          color: theme.colorScheme.primary,
        ),
        const SizedBox(width: 8),
        Expanded(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            mainAxisSize: MainAxisSize.min,
            children: [
              Text(
                '当前用户邮箱',
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
                style: theme.textTheme.labelMedium?.copyWith(
                  color: theme.colorScheme.onSurfaceVariant,
                  fontWeight: FontWeight.w800,
                ),
              ),
              const SizedBox(height: 2),
              Text(
                userLabel,
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
                style: theme.textTheme.bodyMedium?.copyWith(
                  fontWeight: FontWeight.w900,
                ),
              ),
            ],
          ),
        ),
      ],
    );
  }
}

class _CompactInfoPill extends StatelessWidget {
  const _CompactInfoPill({
    required this.icon,
    required this.label,
    required this.value,
  });

  final IconData icon;
  final String label;
  final String value;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 9),
      decoration: BoxDecoration(
        color: theme.colorScheme.surfaceContainerLowest,
        borderRadius: BorderRadius.circular(10),
        border: Border.all(color: theme.colorScheme.outlineVariant),
      ),
      child: Row(
        children: [
          Icon(icon, size: 18, color: theme.colorScheme.primary),
          const SizedBox(width: 8),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              mainAxisSize: MainAxisSize.min,
              children: [
                Text(
                  label,
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: theme.textTheme.labelMedium?.copyWith(
                    color: theme.colorScheme.onSurfaceVariant,
                    fontWeight: FontWeight.w800,
                  ),
                ),
                const SizedBox(height: 2),
                Text(
                  value,
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: theme.textTheme.bodyMedium?.copyWith(
                    fontWeight: FontWeight.w900,
                  ),
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }
}

class _SignedOutStatus extends StatelessWidget {
  const _SignedOutStatus();

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 12),
      decoration: BoxDecoration(
        border: Border.all(color: const Color(0xffe2d6cf)),
        borderRadius: BorderRadius.circular(8),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            '登录后启用组网',
            style: Theme.of(context).textTheme.titleMedium?.copyWith(
                  fontWeight: FontWeight.w700,
                ),
          ),
        ],
      ),
    );
  }
}

class _SignedInActions extends StatelessWidget {
  const _SignedInActions({
    required this.syncing,
    required this.onOpenConsole,
    required this.onLogout,
  });

  final bool syncing;
  final VoidCallback onOpenConsole;
  final VoidCallback onLogout;

  @override
  Widget build(BuildContext context) {
    return Row(
      children: [
        Expanded(
          child: SizedBox(
            height: 38,
            child: OutlinedButton.icon(
              onPressed: syncing ? null : onOpenConsole,
              icon: const Icon(Icons.open_in_browser_rounded, size: 17),
              label: const Text('Web Console'),
            ),
          ),
        ),
        const SizedBox(width: 8),
        Expanded(
          child: SizedBox(
            height: 38,
            child: OutlinedButton.icon(
              onPressed: syncing ? null : onLogout,
              icon: const Icon(Icons.logout_rounded, size: 17),
              label: const Text('Logout'),
            ),
          ),
        ),
      ],
    );
  }
}

class _MessageCard extends StatelessWidget {
  const _MessageCard({required this.text, this.error = false});

  final String text;
  final bool error;

  @override
  Widget build(BuildContext context) {
    return Container(
      margin: const EdgeInsets.only(top: 6),
      padding: const EdgeInsets.symmetric(horizontal: 11, vertical: 9),
      decoration: BoxDecoration(
        color: error ? const Color(0xffffebee) : const Color(0xfff5efe9),
        borderRadius: BorderRadius.circular(8),
      ),
      child: Text(
        text,
        maxLines: 2,
        overflow: TextOverflow.ellipsis,
      ),
    );
  }
}
