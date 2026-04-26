library slan_app.features.home;

import 'dart:async';
import 'dart:io';

import 'package:flutter/material.dart';

import '../../features/shared/desktop_client_widgets.dart';
import '../../infra/app_core/models/network_models.dart';
import '../../infra/app_core/scope/app_core_scope.dart';
import '../../infra/app_core/store/app_session_store.dart';
import '../../infra/logging/startup_log.dart';
import '../../shared/desktop_platform.dart';
import '../../shared/desktop_url_launcher.dart';
import '../../testing/app_test_keys.dart';
import '../auth/auth_callback_service.dart';

part 'home_page_logic.dart';

class HomePage extends StatefulWidget {
  const HomePage({
    super.key,
    this.enableAutoSetup = true,
  });

  final bool enableAutoSetup;

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
    if (widget.enableAutoSetup &&
        sessionStore.session != null &&
        !_autoSetupStarted) {
      _autoSetupStarted = true;
      WidgetsBinding.instance.addPostFrameCallback((_) {
        _ensureWorkspaceReady(sessionStore);
      });
    }

    return AnimatedBuilder(
      animation: Listenable.merge([sessionStore, tunnelStore]),
      builder: (context, _) {
        final loggedIn = sessionStore.session != null;
        final activeNetwork = sessionStore.selectedNetwork;
        final runtime = tunnelStore.tunnelRuntimeView;
        final currentMember = _memberForCurrentDevice(
            sessionStore.device?.deviceId, activeNetwork);
        final virtualIp = switch (activeNetwork) {
          null => 'No network',
          _
              when currentMember?.virtualIp != null &&
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
              if (!loggedIn)
                Padding(
                  padding: const EdgeInsets.only(right: 12),
                  child: Tooltip(
                    message:
                        'Server Config: ${AppCoreScope.hostConfig?.displayHost ?? 'mock'}',
                    child: IconButton.filledTonal(
                      key: AppTestKeys.homeSettingsButton,
                      onPressed: () => _showServerSettingsDialog(context),
                      icon: const Icon(Icons.settings_suggest_rounded),
                    ),
                  ),
                ),
            ],
          ),
          body: Center(
            child: ConstrainedBox(
              constraints: const BoxConstraints(maxWidth: 1120),
              child: SingleChildScrollView(
                padding: const EdgeInsets.all(24),
                child: loggedIn
                    ? _LoggedInHome(
                        userLabel: sessionStore.session?.userLabel ??
                            sessionStore.session?.userId ??
                            'Unknown user',
                        loginTimeLabel: _formatLoginTime(
                          sessionStore.session?.authenticatedAtMs,
                        ),
                        virtualIp: virtualIp,
                        runtimeState: runtimeState,
                        activeNetwork: activeNetwork,
                        currentDeviceId: sessionStore.device?.deviceId ??
                            sessionStore.session?.deviceId ??
                            '',
                        notice: sessionStore.notice,
                        error: sessionStore.error,
                        hasActiveNetwork: activeNetwork != null,
                        busy: sessionStore.busy,
                        onEnable: sessionController.enableActiveNetwork,
                        onDisable: sessionController.disableActiveNetwork,
                        onRefresh: sessionController.refreshNetworks,
                        onLogout: () => _logoutFromClient(
                          hasActiveNetwork: activeNetwork != null,
                        ),
                        onDetails: _openWebDetails,
                      )
                    : _LoggedOutHome(
                        onLogin: _openBrowserLogin,
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
    required this.onLogin,
  });

  final VoidCallback onLogin;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return _RouterPanel(
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.center,
        children: [
          const _BrandMark(),
          const SizedBox(width: 18),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  'SLAN Console',
                  style: theme.textTheme.labelLarge?.copyWith(
                    color: theme.colorScheme.onSurfaceVariant,
                    fontWeight: FontWeight.w800,
                  ),
                ),
                const SizedBox(height: 6),
                Text(
                  'Network Console',
                  style: theme.textTheme.headlineMedium?.copyWith(
                    fontWeight: FontWeight.w900,
                  ),
                ),
                const SizedBox(height: 8),
                Text(
                  'View the current network, device status, and access state in one router-style dashboard.',
                  style: theme.textTheme.bodyLarge?.copyWith(
                    color: theme.colorScheme.onSurfaceVariant,
                    height: 1.45,
                  ),
                ),
              ],
            ),
          ),
          const SizedBox(width: 24),
          FilledButton.icon(
            key: AppTestKeys.homeLoginButton,
            onPressed: onLogin,
            icon: const Icon(Icons.lock_open_rounded),
            label: const Text('Login'),
          ),
        ],
      ),
    );
  }
}

class _LoggedInHome extends StatelessWidget {
  const _LoggedInHome({
    required this.userLabel,
    required this.loginTimeLabel,
    required this.virtualIp,
    required this.runtimeState,
    required this.activeNetwork,
    required this.currentDeviceId,
    required this.notice,
    required this.error,
    required this.hasActiveNetwork,
    required this.busy,
    required this.onEnable,
    required this.onDisable,
    required this.onRefresh,
    required this.onLogout,
    required this.onDetails,
  });

  final String userLabel;
  final String loginTimeLabel;
  final String virtualIp;
  final String runtimeState;
  final NetworkModel? activeNetwork;
  final String currentDeviceId;
  final String? notice;
  final String? error;
  final bool hasActiveNetwork;
  final bool busy;
  final Future<void> Function() onEnable;
  final Future<void> Function() onDisable;
  final Future<void> Function() onRefresh;
  final Future<void> Function() onLogout;
  final Future<void> Function() onDetails;

  @override
  Widget build(BuildContext context) {
    final member = _networkMemberForCurrentDevice(
      currentDeviceId,
      activeNetwork,
    );
    final subnetLabel = activeNetwork == null
        ? '-'
        : ' - ';
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        _RouterPanel(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              _SectionHeader(
                title: 'Network Overview',
                subtitle: 'Like a router console, start with the active network, online device, and access status.',
                trailing: OutlinedButton(
                  key: AppTestKeys.networksRefreshButton,
                  onPressed: busy ? null : () => onRefresh(),
                  child: const Text('Refresh Status'),
                ),
              ),
              const SizedBox(height: 28),
              LayoutBuilder(
                builder: (context, constraints) {
                  final wide = constraints.maxWidth >= 820;
                  final cards = [
                    _OverviewTile(
                      tone: _OverviewTone.success,
                      label: 'Runtime Status',
                      value: activeNetwork == null ? 'Not joined' : 'Joined',
                      detail: activeNetwork?.networkId ?? 'No active network',
                    ),
                    _OverviewTile(
                      label: 'Default Network',
                      value: activeNetwork?.cidr ?? '-',
                      detail: subnetLabel,
                    ),
                    _OverviewTile(
                      label: 'Current Device',
                      value: currentDeviceId.isEmpty
                          ? 'No registered device'
                          : 'SLAN Client - ',
                      detail: member?.virtualIp ?? virtualIp,
                    ),
                  ];
                  return GridView.count(
                    crossAxisCount: wide ? 3 : 1,
                    shrinkWrap: true,
                    physics: const NeverScrollableScrollPhysics(),
                    crossAxisSpacing: 12,
                    mainAxisSpacing: 12,
                    childAspectRatio: wide ? 2.9 : 4.8,
                    children: cards,
                  );
                },
              ),
              if (notice != null || error != null) ...[
                const SizedBox(height: 16),
                _InlineNotice(
                  message: error ?? notice!,
                  isError: error != null,
                ),
              ],
            ],
          ),
        ),
        const SizedBox(height: 16),
        _RouterPanel(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              _SectionHeader(
                title: 'Current Session',
                subtitle: 'Current account, network runtime, and local client actions.',
              ),
              const SizedBox(height: 16),
              Wrap(
                spacing: 10,
                runSpacing: 10,
                children: [
                  _InfoPill(label: 'Current User', value: userLabel),
                  _InfoPill(label: 'Login Time', value: loginTimeLabel),
                  _InfoPill(label: 'Current IP', value: virtualIp),
                  _InfoPill(label: 'Network State', value: runtimeState),
                ],
              ),
              const SizedBox(height: 18),
              Wrap(
                spacing: 12,
                runSpacing: 12,
                children: [
                  FilledButton.icon(
                    key: AppTestKeys.homeEnableNetworkButton,
                    onPressed:
                        busy || !hasActiveNetwork ? null : () => onEnable(),
                    icon: const Icon(Icons.play_circle_outline_rounded),
                    label: const Text('Enable'),
                  ),
                  OutlinedButton.icon(
                    key: AppTestKeys.homeDisableNetworkButton,
                    onPressed:
                        busy || !hasActiveNetwork ? null : () => onDisable(),
                    icon: const Icon(Icons.pause_circle_outline_rounded),
                    label: const Text('Disable'),
                  ),
                  OutlinedButton.icon(
                    key: AppTestKeys.homeDetailsButton,
                    onPressed: busy ? null : () => onDetails(),
                    icon: const Icon(Icons.open_in_browser_rounded),
                    label: const Text('Web Console'),
                  ),
                  OutlinedButton.icon(
                    key: AppTestKeys.homeLogoutButton,
                    onPressed: busy ? null : () => onLogout(),
                    icon: const Icon(Icons.logout_rounded),
                    label: const Text('Logout'),
                  ),
                ],
              ),
            ],
          ),
        ),
      ],
    );
  }
}

NetworkMemberModel? _networkMemberForCurrentDevice(
  String? deviceId,
  NetworkModel? network,
) {
  if (deviceId == null || network == null) {
    return null;
  }
  for (final member in network.members) {
    if (member.deviceId == deviceId) {
      return member;
    }
  }
  return null;
}

class _RouterPanel extends StatelessWidget {
  const _RouterPanel({required this.child});

  final Widget child;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Container(
      width: double.infinity,
      padding: const EdgeInsets.all(20),
      decoration: BoxDecoration(
        color: theme.colorScheme.surface,
        borderRadius: BorderRadius.circular(22),
        border: Border.all(color: theme.colorScheme.outlineVariant),
        boxShadow: [
          BoxShadow(
            color: Colors.black.withValues(alpha: 0.04),
            blurRadius: 28,
            offset: const Offset(0, 10),
          ),
        ],
      ),
      child: child,
    );
  }
}

class _BrandMark extends StatelessWidget {
  const _BrandMark();

  @override
  Widget build(BuildContext context) {
    return Container(
      width: 56,
      height: 56,
      alignment: Alignment.center,
      decoration: BoxDecoration(
        color: Theme.of(context).colorScheme.primary,
        borderRadius: BorderRadius.circular(18),
      ),
      child: const Text(
        'S',
        style: TextStyle(
          color: Colors.white,
          fontSize: 24,
          fontWeight: FontWeight.w900,
        ),
      ),
    );
  }
}

class _SectionHeader extends StatelessWidget {
  const _SectionHeader({
    required this.title,
    required this.subtitle,
    this.trailing,
  });

  final String title;
  final String subtitle;
  final Widget? trailing;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Row(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Expanded(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(
                title,
                style: theme.textTheme.titleLarge?.copyWith(
                  fontWeight: FontWeight.w900,
                ),
              ),
              const SizedBox(height: 8),
              Text(
                subtitle,
                style: theme.textTheme.bodyMedium?.copyWith(
                  color: theme.colorScheme.onSurfaceVariant,
                  height: 1.45,
                ),
              ),
            ],
          ),
        ),
        if (trailing != null) ...[
          const SizedBox(width: 16),
          trailing!,
        ],
      ],
    );
  }
}

enum _OverviewTone { normal, success }

class _OverviewTile extends StatelessWidget {
  const _OverviewTile({
    required this.label,
    required this.value,
    required this.detail,
    this.tone = _OverviewTone.normal,
  });

  final String label;
  final String value;
  final String detail;
  final _OverviewTone tone;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final success = tone == _OverviewTone.success;
    return Container(
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: success
            ? const Color(0xFFEFFAF4)
            : theme.colorScheme.surfaceContainerLowest,
        borderRadius: BorderRadius.circular(16),
        border: Border.all(
          color: success
              ? const Color(0xFFB9E8CB)
              : theme.colorScheme.outlineVariant,
        ),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        mainAxisAlignment: MainAxisAlignment.center,
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
          const SizedBox(height: 10),
          Text(
            value,
            maxLines: 1,
            overflow: TextOverflow.ellipsis,
            style: theme.textTheme.titleLarge?.copyWith(
              fontWeight: FontWeight.w900,
            ),
          ),
          const SizedBox(height: 6),
          Text(
            detail,
            maxLines: 1,
            overflow: TextOverflow.ellipsis,
            style: theme.textTheme.bodyMedium?.copyWith(
              color: theme.colorScheme.onSurfaceVariant,
            ),
          ),
        ],
      ),
    );
  }
}

class _InlineNotice extends StatelessWidget {
  const _InlineNotice({
    required this.message,
    required this.isError,
  });

  final String message;
  final bool isError;

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return Container(
      width: double.infinity,
      padding: const EdgeInsets.all(14),
      decoration: BoxDecoration(
        color: isError ? scheme.errorContainer : const Color(0xFFEFFAF4),
        borderRadius: BorderRadius.circular(14),
        border: Border.all(
          color: isError ? scheme.error : const Color(0xFFB9E8CB),
        ),
      ),
      child: Text(
        message,
        style: TextStyle(
          color: isError ? scheme.onErrorContainer : const Color(0xFF14532D),
          fontWeight: FontWeight.w700,
        ),
      ),
    );
  }
}

class _InfoPill extends StatelessWidget {
  const _InfoPill({
    required this.label,
    required this.value,
  });

  final String label;
  final String value;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 12),
      decoration: BoxDecoration(
        color: theme.colorScheme.surfaceContainerLowest,
        borderRadius: BorderRadius.circular(14),
        border: Border.all(color: theme.colorScheme.outlineVariant),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        mainAxisSize: MainAxisSize.min,
        children: [
          Text(
            label,
            style: theme.textTheme.labelMedium?.copyWith(
              color: theme.colorScheme.onSurfaceVariant,
              fontWeight: FontWeight.w800,
            ),
          ),
          const SizedBox(height: 4),
          Text(
            value,
            style: theme.textTheme.bodyMedium?.copyWith(
              fontWeight: FontWeight.w800,
            ),
          ),
        ],
      ),
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
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        SizedBox(
          width: 104,
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
