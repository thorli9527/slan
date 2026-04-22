library slan_app.features.home;

import 'dart:async';
import 'dart:io';

import 'package:flutter/material.dart';

import '../auth/auth_callback_service.dart';
import '../../features/shared/desktop_client_widgets.dart';
import '../../infra/app_core/models/network_models.dart';
import '../../infra/logging/startup_log.dart';
import '../../infra/app_core/scope/app_core_scope.dart';
import '../../infra/app_core/store/app_session_store.dart';
import '../../shared/desktop_platform.dart';
import '../../shared/desktop_url_launcher.dart';
import '../../testing/app_test_keys.dart';

part 'home_page_logic.dart';

class HomePage extends StatefulWidget {
  const HomePage({super.key});

  @override
  State<HomePage> createState() => _HomePageState();
}

class _HomePageState extends State<HomePage> {
  bool _autoSetupStarted = false;
  bool _openedNetworkConsole = false;
  String? _networkConsoleStatus;
  Timer? _networkPollingTimer;

  void _setNetworkConsoleStatus(String message) {
    if (!mounted) {
      return;
    }
    setState(() {
      _networkConsoleStatus = message;
    });
  }

  @override
  Widget build(BuildContext context) {
    final sessionController = AppCoreScope.sessionController;
    final sessionStore = AppCoreScope.sessionStore;
    final tunnelStore = AppCoreScope.tunnelStore;
    if (sessionStore.session == null) {
      _autoSetupStarted = false;
    }
    if (sessionStore.session != null && !_autoSetupStarted) {
      _autoSetupStarted = true;
      WidgetsBinding.instance.addPostFrameCallback((_) {
        _ensureWorkspaceReady(sessionStore);
      });
    }

    return AnimatedBuilder(
      animation: Listenable.merge([sessionStore, tunnelStore]),
      builder: (context, _) {
        final loggedIn = sessionStore.session != null;
        final activeNetwork =
            sessionStore.networks.isNotEmpty ? sessionStore.networks.first : null;
        final runtime = tunnelStore.tunnelRuntimeView;
        final currentMember =
            _memberForCurrentDevice(sessionStore.device?.deviceId, activeNetwork);
        final virtualIp = switch (activeNetwork) {
          null => 'No network',
          _ when currentMember?.virtualIp != null &&
              currentMember!.virtualIp!.trim().isNotEmpty =>
            currentMember.virtualIp!.trim(),
          _ => 'Pending allocation',
        };
        final runtimeState =
            activeNetwork == null ? 'inactive' : runtime?.state ?? 'idle';
        return Scaffold(
          appBar: AppBar(
            title: const Text('SLAN'),
            actions: [
              IconButton(
                tooltip: 'Settings',
                onPressed: () => _showServerSettingsDialog(context),
                icon: const Icon(Icons.tune),
              ),
            ],
          ),
          body: Center(
            child: ConstrainedBox(
              constraints: const BoxConstraints(maxWidth: 760),
              child: SingleChildScrollView(
                padding: const EdgeInsets.all(24),
                child: loggedIn
                    ? _LoggedInHome(
                        userLabel: sessionStore.session?.userLabel ??
                            sessionStore.session?.userId ??
                            'Unknown user',
                        virtualIp: virtualIp,
                        runtimeState: runtimeState,
                        hasActiveNetwork: activeNetwork != null,
                        busy: sessionStore.busy,
                        statusMessage: _networkConsoleStatus ??
                            sessionStore.notice ??
                            tunnelStore.lastTunnelActionReport?.detail,
                        error: sessionStore.error,
                        onEnable: sessionController.enableActiveNetwork,
                        onDisable: sessionController.disableActiveNetwork,
                        onLogout: sessionController.signOut,
                      )
                    : _LoggedOutHome(
                        hostLabel:
                            AppCoreScope.hostConfig?.displayHost ?? 'mock',
                        onLogin: () => _openBrowserLogin(),
                        onSettings: () => _showServerSettingsDialog(context),
                      ),
              ),
            ),
          ),
        );
      },
    );
  }

  @override
  void dispose() {
    _networkPollingTimer?.cancel();
    super.dispose();
  }
}

class _LoggedOutHome extends StatelessWidget {
  const _LoggedOutHome({
    required this.hostLabel,
    required this.onLogin,
    required this.onSettings,
  });

  final String hostLabel;
  final VoidCallback onLogin;
  final VoidCallback onSettings;

  @override
  Widget build(BuildContext context) {
    return Row(
      children: [
        Expanded(
          child: _ActionTile(
            key: AppTestKeys.homeLoginButton,
            icon: Icons.lock_open_rounded,
            label: '登录',
            subtitle: '打开浏览器完成登录',
            color: const Color(0xFF1E6B52),
            onTap: onLogin,
          ),
        ),
        const SizedBox(width: 16),
        Expanded(
          child: _ActionTile(
            key: AppTestKeys.homeSettingsButton,
            icon: Icons.settings_suggest_rounded,
            label: '设置',
            subtitle: hostLabel,
            color: const Color(0xFF355C7D),
            onTap: onSettings,
          ),
        ),
      ],
    );
  }
}

class _LoggedInHome extends StatelessWidget {
  const _LoggedInHome({
    required this.userLabel,
    required this.virtualIp,
    required this.runtimeState,
    required this.hasActiveNetwork,
    required this.busy,
    required this.statusMessage,
    required this.error,
    required this.onEnable,
    required this.onDisable,
    required this.onLogout,
  });

  final String userLabel;
  final String virtualIp;
  final String runtimeState;
  final bool hasActiveNetwork;
  final bool busy;
  final String? statusMessage;
  final String? error;
  final Future<void> Function() onEnable;
  final Future<void> Function() onDisable;
  final Future<void> Function() onLogout;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        DesktopHeroPanel(
          title: 'Client',
          description: '登录成功后默认进入这张极简页面，只保留当前用户、当前 IP 和网络启停。',
          backgroundColor: const Color(0xFFF0FBF6),
          trailing: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            mainAxisSize: MainAxisSize.min,
            children: [
              DesktopMetricPill(label: 'User', value: userLabel),
              const SizedBox(height: 10),
              DesktopMetricPill(label: 'IP', value: virtualIp),
              const SizedBox(height: 10),
              DesktopMetricPill(label: 'Runtime', value: runtimeState),
            ],
          ),
        ),
        const SizedBox(height: 20),
        Row(
          children: [
            Expanded(
              child: FilledButton.icon(
                key: AppTestKeys.homeEnableNetworkButton,
                onPressed: busy || !hasActiveNetwork ? null : () => onEnable(),
                icon: const Icon(Icons.play_circle_outline_rounded),
                label: const Text('启用网络'),
              ),
            ),
            const SizedBox(width: 12),
            Expanded(
              child: OutlinedButton.icon(
                key: AppTestKeys.homeDisableNetworkButton,
                onPressed: busy || !hasActiveNetwork ? null : () => onDisable(),
                icon: const Icon(Icons.pause_circle_outline_rounded),
                label: const Text('停用网络'),
              ),
            ),
            const SizedBox(width: 12),
            Expanded(
              child: OutlinedButton.icon(
                key: AppTestKeys.homeLogoutButton,
                onPressed: busy ? null : () => onLogout(),
                icon: const Icon(Icons.logout_rounded),
                label: const Text('退出'),
              ),
            ),
          ],
        ),
        const SizedBox(height: 20),
        DesktopSurfaceCard(
          title: 'Current Session',
          subtitle: '这里只显示当前会话和网络运行反馈。',
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              _StatusRow(label: '当前用户', value: userLabel),
              const SizedBox(height: 10),
              _StatusRow(label: '当前 IP', value: virtualIp),
              const SizedBox(height: 10),
              _StatusRow(label: '网络状态', value: runtimeState),
              const SizedBox(height: 16),
              if (busy) const Text('Working on the current node operation...'),
              if (!busy && statusMessage != null)
                Text(
                  statusMessage!,
                  style: TextStyle(color: theme.colorScheme.primary),
                ),
              if (error != null) ...[
                if (statusMessage != null || busy) const SizedBox(height: 8),
                Text(
                  error!,
                  style: TextStyle(color: theme.colorScheme.error),
                ),
              ],
              if (!busy && statusMessage == null && error == null)
                Text(
                  hasActiveNetwork
                      ? 'Ready. You can enable, disable, or sign out.'
                      : '当前账号还没有活动网络，请先在网页端创建或接入网络。',
                ),
            ],
          ),
        ),
      ],
    );
  }
}

class _StatusRow extends StatelessWidget {
  const _StatusRow({
    required this.label,
    required this.value,
  });

  final String label;
  final String value;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Row(
      children: [
        SizedBox(
          width: 88,
          child: Text(
            label,
            style: theme.textTheme.bodyMedium?.copyWith(
              color: theme.colorScheme.onSurfaceVariant,
            ),
          ),
        ),
        Expanded(
          child: Text(
            value,
            style: theme.textTheme.titleMedium?.copyWith(
              fontWeight: FontWeight.w700,
            ),
          ),
        ),
      ],
    );
  }
}

class _ActionTile extends StatelessWidget {
  const _ActionTile({
    super.key,
    required this.icon,
    required this.label,
    required this.subtitle,
    required this.color,
    required this.onTap,
  });

  final IconData icon;
  final String label;
  final String subtitle;
  final Color color;
  final VoidCallback? onTap;

  @override
  Widget build(BuildContext context) {
    return InkWell(
      borderRadius: BorderRadius.circular(28),
      onTap: onTap,
      child: Ink(
        padding: const EdgeInsets.all(22),
        decoration: BoxDecoration(
          color: Colors.white,
          borderRadius: BorderRadius.circular(28),
          border: Border.all(
            color: color.withValues(alpha: 0.18),
          ),
          boxShadow: [
            BoxShadow(
              color: color.withValues(alpha: 0.08),
              blurRadius: 28,
              offset: const Offset(0, 12),
            ),
          ],
        ),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Container(
              width: 64,
              height: 64,
              decoration: BoxDecoration(
                color: color.withValues(alpha: 0.12),
                borderRadius: BorderRadius.circular(20),
              ),
              child: Icon(icon, color: color, size: 32),
            ),
            const SizedBox(height: 18),
            Text(
              label,
              style: Theme.of(context).textTheme.headlineSmall?.copyWith(
                    fontWeight: FontWeight.w800,
                  ),
            ),
            const SizedBox(height: 6),
            Text(
              subtitle,
              style: Theme.of(context).textTheme.bodyMedium?.copyWith(
                    color: Theme.of(context).colorScheme.onSurfaceVariant,
                    height: 1.45,
                  ),
            ),
          ],
        ),
      ),
    );
  }
}
