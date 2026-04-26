import 'package:flutter/material.dart';

import '../../infra/app_core/scope/app_core_scope.dart';
import '../../infra/app_core/scope/app_host_config.dart';
import '../../shared/desktop_url_launcher.dart';
import '../../testing/app_test_keys.dart';
import '../shared/desktop_client_widgets.dart';
import 'auth_callback_service.dart';

class AuthPage extends StatefulWidget {
  const AuthPage({super.key});

  @override
  State<AuthPage> createState() => _AuthPageState();
}

class _AuthPageState extends State<AuthPage> {
  late final TextEditingController _hostController;

  @override
  void initState() {
    super.initState();
    _hostController = TextEditingController(
      text: AppCoreScope.hostInput ?? '127.0.0.1',
    );
  }

  @override
  void dispose() {
    _hostController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final sessionStore = AppCoreScope.sessionStore;
    return AnimatedBuilder(
      animation: sessionStore,
      builder: (context, _) {
        final hostConfig = AppHostConfig.tryParse(_hostController.text) ??
            AppCoreScope.hostConfig;
        return Center(
          child: ConstrainedBox(
            constraints: const BoxConstraints(maxWidth: 720),
            child: SingleChildScrollView(
              padding: const EdgeInsets.all(16),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.stretch,
                children: [
                  Align(
                    alignment: Alignment.topRight,
                    child: IconButton.filledTonal(
                      key: AppTestKeys.authServerConfigButton,
                      tooltip: 'Server Config',
                      onPressed: () => _showServerConfigDialog(context),
                      icon: const Icon(Icons.settings_ethernet),
                    ),
                  ),
                  const SizedBox(height: 12),
                  _AuthHero(
                    error: sessionStore.error,
                    hostConfig: hostConfig,
                  ),
                  const SizedBox(height: 16),
                  _AuthFormCard(
                    busy: sessionStore.busy,
                    appCoreMode: AppCoreScope.mode,
                    onOpenLogin: () => _openWebAuth(loginOnly: true),
                    onOpenConsole: () => _openWebAuth(loginOnly: false),
                  ),
                ],
              ),
            ),
          ),
        );
      },
    );
  }

  Future<void> _openWebAuth({required bool loginOnly}) async {
    final hostConfig =
        AppHostConfig.tryParse(_hostController.text) ?? AppCoreScope.hostConfig;
    final configured = loginOnly
        ? hostConfig?.authLoginUrl ??
            (AppCoreScope.webConsoleUrl == null
                ? null
                : AppHostConfig.tryParse(AppCoreScope.webConsoleUrl)
                    ?.authLoginUrl)
        : hostConfig?.webConsoleUrl ?? AppCoreScope.webConsoleUrl;
    final sessionStore = AppCoreScope.sessionStore;
    final currentDeviceId = sessionStore.device?.deviceId?.trim();
    final loginTargetDeviceId =
        currentDeviceId != null && currentDeviceId.isNotEmpty
            ? currentDeviceId
            : AppCoreScope.clientMachineId;
    if (configured == null || configured.isEmpty) {
      return;
    }
    final uri = Uri.tryParse(configured);
    if (uri == null) {
      return;
    }
    await AuthCallbackService.preparePendingServerCallback(
      preferredKey: loginTargetDeviceId,
    );
    final target = uri.replace(
      path: '/',
      queryParameters: {
        ...uri.queryParameters,
        'deviceId': loginTargetDeviceId,
      },
    );
    await DesktopUrlLauncher.open(target.toString());
  }

  void _applyHostConfig() {
    AppCoreScope.configureHost(host: _hostController.text);
    setState(() {});
  }

  void _setHostPreset(String host) {
    _hostController.text = host;
    _applyHostConfig();
  }

  Future<void> _showServerConfigDialog(BuildContext context) async {
    await showDialog<void>(
      context: context,
      builder: (dialogContext) {
        return StatefulBuilder(
          builder: (context, setDialogState) {
            final hostConfig = AppHostConfig.tryParse(_hostController.text) ??
                AppCoreScope.hostConfig;
            return AlertDialog(
              key: AppTestKeys.authServerConfigDialog,
              title: const Text('Server Config'),
              content: SizedBox(
                width: 560,
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    TextField(
                      key: AppTestKeys.authHostField,
                      controller: _hostController,
                      onChanged: (_) => setDialogState(() {}),
                      decoration: const InputDecoration(
                        labelText: 'Server Host',
                        hintText: '127.0.0.1 / slan.localhost / your-host',
                        prefixIcon: Icon(Icons.hub_outlined),
                      ),
                    ),
                    const SizedBox(height: 12),
                    Wrap(
                      spacing: 12,
                      runSpacing: 12,
                      children: [
                        FilledButton.icon(
                          key: AppTestKeys.authApplyHostButton,
                          onPressed: () {
                            _applyHostConfig();
                            setDialogState(() {});
                          },
                          icon: const Icon(Icons.sync_alt),
                          label: const Text('Apply Host'),
                        ),
                        OutlinedButton(
                          onPressed: () {
                            _setHostPreset('127.0.0.1');
                            setDialogState(() {});
                          },
                          child: const Text('Use Local HTTP'),
                        ),
                        OutlinedButton(
                          onPressed: () {
                            _setHostPreset('slan.localhost');
                            setDialogState(() {});
                          },
                          child: const Text('Use Local HTTPS'),
                        ),
                      ],
                    ),
                    const SizedBox(height: 16),
                    DesktopInsetBlock(
                      title: 'Resolved Endpoints',
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          _ResolvedRow(
                            label: 'Control API',
                            value: hostConfig?.controlBaseUrl ?? 'pending',
                          ),
                          _ResolvedRow(
                            label: 'Web Console',
                            value: hostConfig?.webConsoleUrl ?? 'pending',
                          ),
                          _ResolvedRow(
                            label: 'Login URL',
                            value: hostConfig?.authLoginUrl ?? 'pending',
                          ),
                        ],
                      ),
                    ),
                  ],
                ),
              ),
              actions: [
                TextButton(
                  onPressed: () => Navigator.of(dialogContext).pop(),
                  child: const Text('Close'),
                ),
              ],
            );
          },
        );
      },
    );
    if (mounted) {
      setState(() {});
    }
  }
}

class _AuthHero extends StatelessWidget {
  const _AuthHero({
    required this.error,
    required this.hostConfig,
  });

  final String? error;
  final AppHostConfig? hostConfig;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return DesktopHeroPanel(
      title: 'Desktop Access Gateway',
      description:
          'Use a single server host for browser login, console access, and desktop callback forwarding.',
      backgroundColor: const Color(0xFFEAF4EE),
      trailing: Wrap(
        spacing: 10,
        runSpacing: 10,
        children: [
          const DesktopMetricPill(
            label: 'Auth Mode',
            value: 'Server Forward',
            backgroundColor: Colors.white,
          ),
          DesktopMetricPill(
            label: 'Transport',
            value: hostConfig?.secure == true ? 'HTTPS / MQTT TLS' : 'HTTP / MQTT',
            backgroundColor: Colors.white,
          ),
        ],
      ),
      footer: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          if (hostConfig case final config?) ...[
            Wrap(
              spacing: 10,
              runSpacing: 10,
              children: [
                DesktopBadge(label: 'control ${config.controlBaseUrl}'),
                DesktopBadge(label: 'web ${config.webConsoleUrl}'),
              ],
            ),
            const SizedBox(height: 10),
          ],
          if (error != null)
            Text(
              error!,
              style: TextStyle(color: theme.colorScheme.error),
            ),
        ],
      ),
    );
  }
}

class _AuthFormCard extends StatelessWidget {
  const _AuthFormCard({
    required this.busy,
    required this.appCoreMode,
    required this.onOpenLogin,
    required this.onOpenConsole,
  });

  final bool busy;
  final String appCoreMode;
  final VoidCallback onOpenLogin;
  final VoidCallback onOpenConsole;

  @override
  Widget build(BuildContext context) {
    return DesktopSurfaceCard(
      title: 'Sign In',
      subtitle:
          'Use browser login to authenticate this desktop client, then return here to continue.',
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Wrap(
            spacing: 10,
            runSpacing: 10,
            children: [
              DesktopBadge(label: 'mode $appCoreMode'),
              const DesktopBadge(label: 'entry browser-auth'),
            ],
          ),
          const SizedBox(height: 16),
          Text(
            'If you need to change the target server, use the config button in the top-right corner before opening browser login.',
            style: Theme.of(context).textTheme.bodyMedium,
          ),
          const SizedBox(height: 12),
          Wrap(
            spacing: 12,
            runSpacing: 12,
            children: [
              FilledButton(
                key: AppTestKeys.authOpenLoginButton,
                onPressed: busy ? null : onOpenLogin,
                child: const Text('Open Browser Login'),
              ),
              OutlinedButton(
                key: AppTestKeys.authOpenConsoleButton,
                onPressed: busy ? null : onOpenConsole,
                child: const Text('Open Web Console'),
              ),
            ],
          ),
        ],
      ),
    );
  }
}

class _ResolvedRow extends StatelessWidget {
  const _ResolvedRow({
    required this.label,
    required this.value,
  });

  final String label;
  final String value;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.only(bottom: 10),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          SizedBox(
            width: 120,
            child: Text(
              label,
              style: Theme.of(context).textTheme.bodyMedium?.copyWith(
                    color: Theme.of(context).colorScheme.onSurfaceVariant,
                  ),
            ),
          ),
          Expanded(
            child: SelectableText(
              value,
              style: Theme.of(context).textTheme.bodyMedium?.copyWith(
                    fontWeight: FontWeight.w600,
                  ),
            ),
          ),
        ],
      ),
    );
  }
}
