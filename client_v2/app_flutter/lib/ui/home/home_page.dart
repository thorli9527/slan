import 'dart:async';

import 'package:client_core_plugin/client_core_plugin.dart';
import 'package:flutter/material.dart';

import '../../bridge/android_network_authorization.dart';
import '../../bridge/client_commands.dart';
import '../../bridge/client_core_bridge.dart';
import '../../bridge/client_ui_diagnostics.dart';
import '../../bridge/client_view_state.dart';

class HomePage extends StatefulWidget {
  const HomePage({required this.bridge, super.key});

  final ClientCoreBridge bridge;

  @override
  State<HomePage> createState() => _HomePageState();
}

class _HomePageState extends State<HomePage> {
  String? _lastDiagnosticsSnapshot;
  String? _lastShownError;
  bool _lastSignedIn = false;

  @override
  void initState() {
    super.initState();
    widget.bridge.state.addListener(_logStateChange);
    _logStateChange();
    unawaited(_startBridge());
  }

  @override
  void dispose() {
    widget.bridge.state.removeListener(_logStateChange);
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      body: SafeArea(
        child: ValueListenableBuilder<ClientViewState>(
          valueListenable: widget.bridge.state,
          builder: (context, state, _) {
            return Column(
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                Expanded(
                  child: Padding(
                    padding: const EdgeInsets.fromLTRB(20, 4, 20, 12),
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.stretch,
                      children: [
                        if (state.signedIn)
                          Column(
                            mainAxisSize: MainAxisSize.min,
                            children: [
                              _buildSignedInHeader(state: state),
                              _buildAndroidAuthorizationPanel(),
                            ],
                          )
                        else
                          const _SignedOutStatus(),
                        if (state.signedIn) ...[
                          const Spacer(),
                          _SignedInActions(
                            onOpenConsole: () => widget.bridge.dispatch(
                              const ClientCommand(
                                ClientCommandType.openWebConsole,
                              ),
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
                  ),
                ),
              ],
            );
          },
        ),
      ),
    );
  }

  void _toggleNetwork(bool enabled) {
    _lastShownError = null;
    ClientUiDiagnostics.unawaitedLog(
      'home.switch.tap',
      state: widget.bridge.state.value,
      fields: {'targetEnabled': enabled},
    );
    widget.bridge.dispatch(
      ClientCommand(
        enabled
            ? ClientCommandType.enableNetwork
            : ClientCommandType.disableNetwork,
      ),
    );
  }

  Future<void> _startBridge() async {
    await widget.bridge.start();
    await widget.bridge.prepareAndroidNetworkAuthorization();
  }

  void _logStateChange() {
    final state = widget.bridge.state.value;
    final snapshot = [
      state.signedIn,
      state.networkEnabled,
      state.syncing,
      state.syncReason,
      state.switchEnabled,
      state.virtualIp,
      state.notice,
      state.error,
      state.errorSource,
    ].join('|');
    if (_lastDiagnosticsSnapshot == snapshot) {
      return;
    }
    _lastDiagnosticsSnapshot = snapshot;
    ClientUiDiagnostics.unawaitedLog('home.state.changed', state: state);
    if (state.signedIn && !_lastSignedIn) {
      unawaited(widget.bridge.prepareAndroidNetworkAuthorization());
    }
    _lastSignedIn = state.signedIn;
    _showErrorDialogIfNeeded(state);
  }

  void _showErrorDialogIfNeeded(ClientViewState state) {
    if (state.errorSource != ClientErrorSource.networkSwitch) {
      return;
    }
    final error = state.error?.trim();
    if (error == null || error.isEmpty || error == _lastShownError) {
      return;
    }
    _lastShownError = error;
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (!mounted) {
        return;
      }
      showDialog<void>(
        context: context,
        builder: (context) {
          return Dialog(
            insetPadding: const EdgeInsets.symmetric(
              horizontal: 18,
              vertical: 12,
            ),
            child: ConstrainedBox(
              constraints: const BoxConstraints(
                maxWidth: 320,
                maxHeight: 180,
              ),
              child: Padding(
                padding: const EdgeInsets.fromLTRB(18, 16, 18, 12),
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  crossAxisAlignment: CrossAxisAlignment.stretch,
                  children: [
                    const Text(
                      '操作失败',
                      style: TextStyle(
                        fontSize: 20,
                        fontWeight: FontWeight.w700,
                      ),
                    ),
                    const SizedBox(height: 10),
                    Flexible(
                      child: SingleChildScrollView(
                        child: Text(
                          _friendlyError(error),
                          style: const TextStyle(fontSize: 14, height: 1.35),
                        ),
                      ),
                    ),
                    const SizedBox(height: 8),
                    Align(
                      alignment: Alignment.centerRight,
                      child: TextButton(
                        onPressed: () => Navigator.of(context).pop(),
                        child: const Text('确定'),
                      ),
                    ),
                  ],
                ),
              ),
            ),
          );
        },
      );
    });
  }

  String _friendlyError(String error) {
    final normalized = error.toLowerCase();
    if (normalized.contains('disabled by network admin') ||
        normalized.contains('disabled by network administrator') ||
        normalized.contains('has been disabled') ||
        normalized.contains('current device is disabled') ||
        normalized.contains('suspended') ||
        normalized.contains('revoked') ||
        normalized.contains('blocked')) {
      return '服务端停用，请联系管理员。';
    }
    if (normalized.contains('device unavailable')) {
      return '设备不可用，请联系管理员重新启用。';
    }
    if (normalized.contains('no active network attachment')) {
      return '当前设备没有可用的网络绑定，请先在 Web Console 中绑定设备。';
    }
    if (normalized.contains('session expired')) {
      return '登录状态已失效，请重新登录。';
    }
    if (normalized.contains('local service')) {
      return '本地服务未连接，请稍后重试或重启客户端。';
    }
    return error;
  }

  Widget _buildSignedInHeader({
    required ClientViewState state,
  }) {
    return _SignedInStatusPanel(
      userLabel: _userLabel(state),
      currentIp: _ipText(state),
      state: state,
      onToggle: _toggleNetwork,
    );
  }

  Widget _buildAndroidAuthorizationPanel() {
    return ValueListenableBuilder<AndroidNetworkAuthorizationState>(
      valueListenable: widget.bridge.androidNetworkAuthorization,
      builder: (context, authorization, _) {
        if (!authorization.visible) {
          return const SizedBox.shrink();
        }
        return Padding(
          padding: const EdgeInsets.only(top: 8),
          child: _AndroidAuthorizationPanel(
            authorization: authorization,
            onRefresh: widget.bridge.prepareAndroidNetworkAuthorization,
          ),
        );
      },
    );
  }

  String _userLabel(ClientViewState state) {
    final user = state.userLabel?.trim();
    return user == null || user.isEmpty ? '-' : user;
  }

  String _ipText(ClientViewState state) {
    final virtualIp = state.virtualIp?.trim();
    if (!state.signedIn ||
        !state.networkEnabled ||
        virtualIp == null ||
        virtualIp.isEmpty) {
      return '未启用';
    }
    return virtualIp;
  }
}

class _SignedInStatusPanel extends StatelessWidget {
  const _SignedInStatusPanel({
    required this.userLabel,
    required this.currentIp,
    required this.state,
    required this.onToggle,
  });

  final String userLabel;
  final String currentIp;
  final ClientViewState state;
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
              Expanded(child: _CompactIdentity(userLabel: userLabel)),
              const SizedBox(width: 12),
              _NetworkControl(state: state, onToggle: onToggle),
            ],
          ),
          const SizedBox(height: 10),
          Divider(height: 1, color: theme.colorScheme.outlineVariant),
          const SizedBox(height: 9),
          _CompactInfoRow(
            valueKey: const Key('network-ip-value'),
            icon: Icons.router_rounded,
            label: '当前 IP',
            value: currentIp,
          ),
        ],
      ),
    );
  }
}

class _NetworkControl extends StatelessWidget {
  const _NetworkControl({required this.state, required this.onToggle});

  final ClientViewState state;
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

class _AndroidAuthorizationPanel extends StatelessWidget {
  const _AndroidAuthorizationPanel({
    required this.authorization,
    required this.onRefresh,
  });

  final AndroidNetworkAuthorizationState authorization;
  final VoidCallback onRefresh;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final config = authorization.networkConfig;
    final error = authorization.error?.trim();
    final status = _statusText();
    final statusColor = error != null && error.isNotEmpty
        ? theme.colorScheme.error
        : authorization.granted
            ? theme.colorScheme.primary
            : theme.colorScheme.onSurfaceVariant;
    return Container(
      padding: const EdgeInsets.fromLTRB(12, 9, 12, 9),
      decoration: BoxDecoration(
        color: theme.colorScheme.surfaceContainerLowest,
        borderRadius: BorderRadius.circular(8),
        border: Border.all(color: theme.colorScheme.outlineVariant),
      ),
      child: Row(
        children: [
          Icon(
            authorization.granted
                ? Icons.verified_user_outlined
                : Icons.vpn_key_outlined,
            size: 18,
            color: statusColor,
          ),
          const SizedBox(width: 8),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              mainAxisSize: MainAxisSize.min,
              children: [
                Text(
                  status,
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: theme.textTheme.labelMedium?.copyWith(
                    color: statusColor,
                    fontWeight: FontWeight.w900,
                  ),
                ),
                if (config != null || (error != null && error.isNotEmpty)) ...[
                  const SizedBox(height: 2),
                  Text(
                    error != null && error.isNotEmpty
                        ? _compactError(error)
                        : _configText(config!),
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: theme.textTheme.labelSmall?.copyWith(
                      color: theme.colorScheme.onSurfaceVariant,
                      fontWeight: FontWeight.w700,
                    ),
                  ),
                ],
              ],
            ),
          ),
          if (authorization.checking)
            SizedBox(
              width: 18,
              height: 18,
              child: CircularProgressIndicator(
                strokeWidth: 2,
                color: theme.colorScheme.primary,
              ),
            )
          else if (authorization.needsUserConsent)
            SizedBox(
              height: 30,
              child: FilledButton.icon(
                onPressed: onRefresh,
                icon: const Icon(Icons.check_circle_outline, size: 16),
                label: const Text('授权'),
              ),
            )
          else
            IconButton(
              tooltip: '刷新',
              onPressed: onRefresh,
              icon: const Icon(Icons.refresh_rounded, size: 18),
            ),
        ],
      ),
    );
  }

  String _statusText() {
    final error = authorization.error?.trim();
    if (error != null && error.isNotEmpty) {
      return 'Android 网络配置异常';
    }
    if (authorization.checking) {
      return 'Android 网络检查中';
    }
    if (authorization.needsUserConsent) {
      return 'Android 网络待授权';
    }
    if (authorization.granted && authorization.networkConfig != null) {
      return 'Android 网络配置已就绪';
    }
    if (authorization.granted) {
      return 'Android 网络已授权';
    }
    return 'Android 网络状态待确认';
  }

  String _configText(AndroidVpnSessionConfig config) {
    final dns = config.dnsServers.isEmpty ? '-' : config.dnsServers.join(',');
    return '${config.virtualIp}/${config.prefixLen}  DNS $dns';
  }

  String _compactError(String error) {
    return error.replaceAll(RegExp(r'\s+'), ' ');
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
          size: 24,
          color: theme.colorScheme.primary,
        ),
        const SizedBox(width: 9),
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

class _CompactInfoRow extends StatelessWidget {
  const _CompactInfoRow({
    this.valueKey,
    required this.icon,
    required this.label,
    required this.value,
  });

  final Key? valueKey;
  final IconData icon;
  final String label;
  final String value;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Row(
      children: [
        Icon(icon, size: 18, color: theme.colorScheme.primary),
        const SizedBox(width: 9),
        Text(
          label,
          maxLines: 1,
          overflow: TextOverflow.ellipsis,
          style: theme.textTheme.labelMedium?.copyWith(
            color: theme.colorScheme.onSurfaceVariant,
            fontWeight: FontWeight.w800,
          ),
        ),
        const SizedBox(width: 12),
        Expanded(
          child: Text(
            key: valueKey,
            value,
            maxLines: 1,
            overflow: TextOverflow.ellipsis,
            textAlign: TextAlign.right,
            style: theme.textTheme.bodyMedium?.copyWith(
              fontWeight: FontWeight.w900,
            ),
          ),
        ),
      ],
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
    required this.onOpenConsole,
    required this.onLogout,
  });

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
              onPressed: onOpenConsole,
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
              onPressed: onLogout,
              icon: const Icon(Icons.logout_rounded, size: 17),
              label: const Text('Logout'),
            ),
          ),
        ),
      ],
    );
  }
}
