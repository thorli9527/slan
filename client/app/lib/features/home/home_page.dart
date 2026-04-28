library slan_app.features.home;

import 'dart:async';
import 'dart:io';

import 'package:flutter/material.dart';

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
  Timer? _networkPollingTimer;

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
          body: Align(
            alignment: loggedIn ? Alignment.topCenter : Alignment.center,
            child: ConstrainedBox(
              constraints: BoxConstraints(maxWidth: loggedIn ? 460 : 520),
              child: SingleChildScrollView(
                padding: EdgeInsets.fromLTRB(
                  loggedIn ? 14 : 18,
                  loggedIn ? 12 : 18,
                  loggedIn ? 14 : 18,
                  loggedIn ? 12 : 18,
                ),
                child: loggedIn
                    ? _LoggedInHomeV4(
                        userLabel: _currentUserLabel(
                          sessionStore,
                          virtualIp: virtualIp,
                        ),
                        virtualIp: virtualIp,
                        runtimeState: runtimeState,
                        hasActiveNetwork: activeNetwork != null,
                        busy: sessionStore.busy,
                        onEnable: () => _enableActiveNetworkWithPrompt(
                          currentMember,
                        ),
                        onDisable: sessionController.disableActiveNetwork,
                        onLogout: _logoutFromClient,
                        onDetails: _openWebDetails,
                      )
                    : _LoggedOutHome(
                        onLogin: _openBrowserLogin,
                        onSettings: () => _showServerSettingsDialog(context),
                        serverLabel:
                            AppCoreScope.hostConfig?.displayHost ?? 'mock',
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
    required this.onSettings,
    required this.serverLabel,
  });

  final VoidCallback onLogin;
  final VoidCallback onSettings;
  final String serverLabel;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return _RouterPanel(
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(
                'Network Console',
                style: theme.textTheme.headlineSmall?.copyWith(
                  fontWeight: FontWeight.w900,
                ),
              ),
              const SizedBox(height: 8),
              Text(
                'Manage access, local tunnel state, and the web console from one compact desktop client.',
                style: theme.textTheme.bodyMedium?.copyWith(
                  color: theme.colorScheme.onSurfaceVariant,
                  height: 1.4,
                ),
              ),
            ],
          ),
          const SizedBox(height: 18),
          Row(
            children: [
              Expanded(
                child: SizedBox(
                  height: 42,
                  child: FilledButton.icon(
                    key: AppTestKeys.homeLoginButton,
                    onPressed: onLogin,
                    icon: const Icon(Icons.lock_open_rounded, size: 18),
                    label: const Text('Login'),
                  ),
                ),
              ),
              const SizedBox(width: 10),
              Tooltip(
                message: 'Server Config: $serverLabel',
                child: SizedBox(
                  width: 46,
                  height: 42,
                  child: OutlinedButton(
                    key: AppTestKeys.homeSettingsButton,
                    onPressed: onSettings,
                    style: OutlinedButton.styleFrom(
                      padding: EdgeInsets.zero,
                    ),
                    child: const Icon(Icons.settings_suggest_rounded, size: 20),
                  ),
                ),
              ),
            ],
          ),
        ],
      ),
    );
  }
}

// ignore: unused_element
class _LoggedInHome extends StatelessWidget {
  const _LoggedInHome({
    required this.userLabel,
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
    final normalizedState = runtimeState.toLowerCase().trim();
    final networkEnabled = hasActiveNetwork &&
        normalizedState != 'idle' &&
        normalizedState != 'inactive' &&
        normalizedState != 'disabled';

    return _RouterPanel(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          const Row(
            children: [
              _BrandMark(),
              SizedBox(width: 16),
              Expanded(
                child: _SectionHeader(
                  title: '当前用户邮箱',
                  subtitle: '当前账户、本机 IP 和隧道控制。',
                ),
              ),
            ],
          ),
          const SizedBox(height: 18),
          GridView.count(
            crossAxisCount: 3,
            shrinkWrap: true,
            physics: const NeverScrollableScrollPhysics(),
            crossAxisSpacing: 12,
            mainAxisSpacing: 12,
            childAspectRatio: 2.7,
            children: [
              _InfoPill(
                icon: Icons.account_circle_outlined,
                label: '当前用户邮箱',
                value: userLabel,
              ),
              _InfoPill(
                icon: Icons.router_rounded,
                label: '当前IP',
                value: virtualIp,
              ),
              _InfoPill(
                icon: Icons.network_check_rounded,
                label: 'Network State',
                value: networkEnabled ? 'enable' : 'disable',
              ),
            ],
          ),
          const SizedBox(height: 18),
          Row(
            children: [
              Expanded(
                child: networkEnabled
                    ? OutlinedButton.icon(
                        key: AppTestKeys.homeNetworkSwitchButton,
                        onPressed: busy || !hasActiveNetwork
                            ? null
                            : () => onDisable(),
                        icon: const Icon(Icons.toggle_on_rounded),
                        label: const Text('Disable'),
                      )
                    : FilledButton.icon(
                        key: AppTestKeys.homeNetworkSwitchButton,
                        onPressed:
                            busy || !hasActiveNetwork ? null : () => onEnable(),
                        icon: const Icon(Icons.toggle_off_rounded),
                        label: const Text('Enable'),
                      ),
              ),
              const SizedBox(width: 10),
              Expanded(
                child: OutlinedButton.icon(
                  key: AppTestKeys.homeDetailsButton,
                  onPressed: busy ? null : () => onDetails(),
                  icon: const Icon(Icons.open_in_browser_rounded),
                  label: const Text('Web Console'),
                ),
              ),
              const SizedBox(width: 10),
              Expanded(
                child: OutlinedButton.icon(
                  key: AppTestKeys.homeLogoutButton,
                  onPressed: busy ? null : () => onLogout(),
                  icon: const Icon(Icons.logout_rounded),
                  label: const Text('Logout'),
                ),
              ),
            ],
          ),
        ],
      ),
    );
  }
}

// ignore: unused_element
class _LoggedInHomeV2 extends StatelessWidget {
  const _LoggedInHomeV2({
    required this.userLabel,
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
    final normalizedState = runtimeState.toLowerCase().trim();
    final networkEnabled = hasActiveNetwork &&
        normalizedState != 'idle' &&
        normalizedState != 'inactive' &&
        normalizedState != 'disabled';

    final networkToggle = _NetworkEnableSwitch(
      enabled: networkEnabled,
      disabled: busy || !hasActiveNetwork,
      onEnable: onEnable,
      onDisable: onDisable,
    );

    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      mainAxisSize: MainAxisSize.min,
      children: [
        LayoutBuilder(
          builder: (context, constraints) {
            final compact = constraints.maxWidth < 520;
            final userInfo = _UserIdentitySummary(userLabel: userLabel);
            final toggle = SizedBox(
              width: compact ? double.infinity : 86,
              height: 58,
              child: networkToggle,
            );

            if (compact) {
              return Column(
                crossAxisAlignment: CrossAxisAlignment.stretch,
                children: [
                  userInfo,
                  const SizedBox(height: 12),
                  toggle,
                ],
              );
            }

            return IntrinsicHeight(
              child: Row(
                crossAxisAlignment: CrossAxisAlignment.stretch,
                children: [
                  Expanded(child: userInfo),
                  const SizedBox(width: 12),
                  toggle,
                ],
              ),
            );
          },
        ),
        const SizedBox(height: 12),
        GridView.count(
          crossAxisCount: 2,
          shrinkWrap: true,
          physics: const NeverScrollableScrollPhysics(),
          crossAxisSpacing: 12,
          mainAxisSpacing: 12,
          childAspectRatio: 2.7,
          children: [
            _InfoPill(
              icon: Icons.router_rounded,
              label: '当前 IP',
              value: virtualIp,
            ),
            _InfoPill(
              icon: Icons.network_check_rounded,
              label: 'Network State',
              value: networkEnabled ? 'enable' : 'disable',
            ),
          ],
        ),
        const SizedBox(height: 18),
        Row(
          children: [
            Expanded(
              child: OutlinedButton.icon(
                key: AppTestKeys.homeDetailsButton,
                onPressed: busy ? null : () => onDetails(),
                icon: const Icon(Icons.open_in_browser_rounded),
                label: const Text('Web Console'),
              ),
            ),
            const SizedBox(width: 10),
            Expanded(
              child: OutlinedButton.icon(
                key: AppTestKeys.homeLogoutButton,
                onPressed: busy ? null : () => onLogout(),
                icon: const Icon(Icons.logout_rounded),
                label: const Text('Logout'),
              ),
            ),
          ],
        ),
      ],
    );
  }
}

class _RouterPanel extends StatelessWidget {
  const _RouterPanel({required this.child});

  final Widget child;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Container(
      width: double.infinity,
      padding: const EdgeInsets.all(22),
      decoration: BoxDecoration(
        color: theme.colorScheme.surface,
        borderRadius: BorderRadius.circular(18),
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

class _LoggedInHomeV4 extends StatelessWidget {
  const _LoggedInHomeV4({
    required this.userLabel,
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
    final normalizedState = runtimeState.toLowerCase().trim();
    final networkEnabled = hasActiveNetwork &&
        normalizedState != 'idle' &&
        normalizedState != 'inactive' &&
        normalizedState != 'disabled';

    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      mainAxisSize: MainAxisSize.min,
      children: [
        LayoutBuilder(
          builder: (context, constraints) {
            final compact = constraints.maxWidth < 360;
            final userInfo = _CompactIdentity(userLabel: userLabel);
            final toggle = SizedBox(
              width: compact ? double.infinity : 72,
              height: 40,
              child: _NetworkEnableSwitch(
                enabled: networkEnabled,
                disabled: busy || !hasActiveNetwork,
                onEnable: onEnable,
                onDisable: onDisable,
              ),
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
          value: virtualIp,
        ),
        const SizedBox(height: 10),
        Row(
          children: [
            Expanded(
              child: SizedBox(
                height: 38,
                child: OutlinedButton.icon(
                  key: AppTestKeys.homeDetailsButton,
                  onPressed: busy ? null : () => onDetails(),
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
                  key: AppTestKeys.homeLogoutButton,
                  onPressed: busy ? null : () => onLogout(),
                  icon: const Icon(Icons.logout_rounded, size: 17),
                  label: const Text('Logout'),
                ),
              ),
            ),
          ],
        ),
      ],
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

// ignore: unused_element
class _LoggedInHomeV3 extends StatelessWidget {
  const _LoggedInHomeV3({
    required this.userLabel,
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
    final normalizedState = runtimeState.toLowerCase().trim();
    final networkEnabled = hasActiveNetwork &&
        normalizedState != 'idle' &&
        normalizedState != 'inactive' &&
        normalizedState != 'disabled';

    return _RouterPanel(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          LayoutBuilder(
            builder: (context, constraints) {
              final compact = constraints.maxWidth < 520;
              final userInfo = _UserIdentitySummary(userLabel: userLabel);
              final toggle = SizedBox(
                width: compact ? double.infinity : 72,
                height: 42,
                child: _NetworkEnableSwitch(
                  enabled: networkEnabled,
                  disabled: busy || !hasActiveNetwork,
                  onEnable: onEnable,
                  onDisable: onDisable,
                ),
              );

              if (compact) {
                return Column(
                  crossAxisAlignment: CrossAxisAlignment.stretch,
                  children: [
                    userInfo,
                    const SizedBox(height: 8),
                    toggle,
                  ],
                );
              }

              return Row(
                crossAxisAlignment: CrossAxisAlignment.center,
                children: [
                  Expanded(child: SizedBox(height: 44, child: userInfo)),
                  const SizedBox(width: 8),
                  toggle,
                ],
              );
            },
          ),
          const SizedBox(height: 8),
          _InfoPill(
            icon: Icons.router_rounded,
            label: '当前 IP',
            value: virtualIp,
          ),
          const SizedBox(height: 10),
          Row(
            children: [
              Expanded(
                child: OutlinedButton.icon(
                  key: AppTestKeys.homeDetailsButton,
                  onPressed: busy ? null : () => onDetails(),
                  icon: const Icon(Icons.open_in_browser_rounded),
                  label: const Text('Web Console'),
                ),
              ),
              const SizedBox(width: 8),
              Expanded(
                child: OutlinedButton.icon(
                  key: AppTestKeys.homeLogoutButton,
                  onPressed: busy ? null : () => onLogout(),
                  icon: const Icon(Icons.logout_rounded),
                  label: const Text('Logout'),
                ),
              ),
            ],
          ),
        ],
      ),
    );
  }
}

class _NetworkEnableSwitch extends StatelessWidget {
  const _NetworkEnableSwitch({
    required this.enabled,
    required this.disabled,
    required this.onEnable,
    required this.onDisable,
  });

  final bool enabled;
  final bool disabled;
  final Future<void> Function() onEnable;
  final Future<void> Function() onDisable;

  @override
  Widget build(BuildContext context) {
    return Align(
      alignment: Alignment.centerRight,
      child: Switch(
        key: AppTestKeys.homeNetworkSwitchButton,
        value: enabled,
        onChanged: disabled
            ? null
            : (value) {
                if (value) {
                  onEnable();
                } else {
                  onDisable();
                }
              },
      ),
    );
  }
}

class _UserIdentitySummary extends StatelessWidget {
  const _UserIdentitySummary({required this.userLabel});

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
        const SizedBox(width: 12),
        Expanded(
          child: Column(
            mainAxisAlignment: MainAxisAlignment.center,
            crossAxisAlignment: CrossAxisAlignment.start,
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
              const SizedBox(height: 5),
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

class _BrandMark extends StatelessWidget {
  const _BrandMark();

  @override
  Widget build(BuildContext context) {
    return Container(
      width: 58,
      height: 58,
      alignment: Alignment.center,
      decoration: BoxDecoration(
        color: Theme.of(context).colorScheme.primary,
        borderRadius: BorderRadius.circular(16),
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
    // ignore: unused_element_parameter
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

class _InfoPill extends StatelessWidget {
  const _InfoPill({
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
