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
              constraints: const BoxConstraints(maxWidth: 760),
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
                        hasActiveNetwork: activeNetwork != null,
                        busy: sessionStore.busy,
                        onEnable: sessionController.enableActiveNetwork,
                        onDisable: sessionController.disableActiveNetwork,
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
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        _ActionTile(
          key: AppTestKeys.homeLoginButton,
          icon: Icons.lock_open_rounded,
          label: 'Login',
          subtitle: 'Open browser login',
          color: const Color(0xFF1E6B52),
          onTap: onLogin,
        ),
      ],
    );
  }
}

class _LoggedInHome extends StatelessWidget {
  const _LoggedInHome({
    required this.userLabel,
    required this.loginTimeLabel,
    required this.virtualIp,
    required this.runtimeState,
    required this.hasActiveNetwork,
    required this.busy,
    required this.onEnable,
    required this.onDisable,
    required this.onLogout,
    required this.onDetails,
  });

  final String userLabel;
  final String loginTimeLabel;
  final String virtualIp;
  final String runtimeState;
  final bool hasActiveNetwork;
  final bool busy;
  final Future<void> Function() onEnable;
  final Future<void> Function() onDisable;
  final Future<void> Function() onLogout;
  final Future<void> Function() onDetails;

  @override
  Widget build(BuildContext context) {
    return DesktopSurfaceCard(
      title: 'Current Session',
      subtitle: 'Minimal client summary after login.',
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          _StatusRow(label: 'Current User', value: userLabel),
          const SizedBox(height: 10),
          _StatusRow(label: 'Login Time', value: loginTimeLabel),
          const SizedBox(height: 10),
          _StatusRow(label: 'Current IP', value: virtualIp),
          const SizedBox(height: 10),
          _StatusRow(label: 'Network State', value: runtimeState),
          const SizedBox(height: 18),
          Wrap(
            spacing: 12,
            runSpacing: 12,
            children: [
              FilledButton.icon(
                key: AppTestKeys.homeEnableNetworkButton,
                onPressed: busy || !hasActiveNetwork ? null : () => onEnable(),
                icon: const Icon(Icons.play_circle_outline_rounded),
                label: const Text('Enable'),
              ),
              OutlinedButton.icon(
                key: AppTestKeys.homeDisableNetworkButton,
                onPressed: busy || !hasActiveNetwork ? null : () => onDisable(),
                icon: const Icon(Icons.pause_circle_outline_rounded),
                label: const Text('Disable'),
              ),
              OutlinedButton.icon(
                key: AppTestKeys.homeDetailsButton,
                onPressed: busy ? null : () => onDetails(),
                icon: const Icon(Icons.open_in_browser_rounded),
                label: const Text('Details'),
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
