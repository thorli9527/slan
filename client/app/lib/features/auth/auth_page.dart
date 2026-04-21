import 'dart:io';

import 'package:flutter/material.dart';

import '../../infra/app_core/scope/app_host_config.dart';
import '../../infra/app_core/scope/app_core_scope.dart';
import '../../testing/app_test_keys.dart';
import 'auth_callback_service.dart';
import '../shared/desktop_client_widgets.dart';

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
        return Center(
          child: ConstrainedBox(
            constraints: const BoxConstraints(maxWidth: 720),
            child: SingleChildScrollView(
              padding: const EdgeInsets.all(16),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.stretch,
                children: [
                  _AuthHero(
                    error: sessionStore.error,
                    hostConfig: AppHostConfig.tryParse(_hostController.text) ??
                        AppCoreScope.hostConfig,
                  ),
                  const SizedBox(height: 16),
                  _AuthFormCard(
                    hostController: _hostController,
                    busy: sessionStore.busy,
                    hostConfig: AppHostConfig.tryParse(_hostController.text) ??
                        AppCoreScope.hostConfig,
                    appCoreMode: AppCoreScope.mode,
                    onApplyServer: _applyHostConfig,
                    onOpenLogin: () => _openWebAuth(loginOnly: true),
                    onOpenConsole: () => _openWebAuth(loginOnly: false),
                    onUseLocalHost: () => _setHostPreset('127.0.0.1'),
                    onUseSecureLocalHost: () =>
                        _setHostPreset('slan.localhost'),
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
    final configured =
        hostConfig?.webConsoleUrl ?? AppCoreScope.webConsoleUrl ?? '';
    if (configured.isEmpty) {
      return;
    }
    final uri = Uri.tryParse(configured);
    if (uri == null) {
      return;
    }
    final callbackId = await AuthCallbackService.preparePendingServerCallback();
    final target = uri.replace(
      queryParameters: {
        ...uri.queryParameters,
        if (loginOnly) 'auth': 'login',
        'deviceId': AppCoreScope.clientMachineId,
        'callbackId': callbackId,
      },
    );
    final url = target.toString();
    if (Platform.isMacOS) {
      await Process.start('open', [url]);
      return;
    }
    if (Platform.isLinux) {
      await Process.start('xdg-open', [url]);
      return;
    }
    if (Platform.isWindows) {
      await Process.start('cmd', ['/c', 'start', '', url]);
      return;
    }
  }

  void _applyHostConfig() {
    AppCoreScope.configureHost(host: _hostController.text);
    setState(() {});
  }

  void _setHostPreset(String host) {
    _hostController.text = host;
    _applyHostConfig();
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
      description: '客户端现在只维护一个统一 host 配置。控制面、网页控制台和网页登录结果转交都从这里自动推导，避免多个地址分开维护。',
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
            value: hostConfig?.secure == true ? 'HTTPS / WSS' : 'HTTP / WS',
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
    required this.hostController,
    required this.busy,
    required this.hostConfig,
    required this.appCoreMode,
    required this.onApplyServer,
    required this.onOpenLogin,
    required this.onOpenConsole,
    required this.onUseLocalHost,
    required this.onUseSecureLocalHost,
  });

  final TextEditingController hostController;
  final bool busy;
  final AppHostConfig? hostConfig;
  final String appCoreMode;
  final VoidCallback onApplyServer;
  final VoidCallback onOpenLogin;
  final VoidCallback onOpenConsole;
  final VoidCallback onUseLocalHost;
  final VoidCallback onUseSecureLocalHost;

  @override
  Widget build(BuildContext context) {
    return DesktopSurfaceCard(
      title: 'Connection Profile',
      subtitle: '输入一个 host，客户端自动推导控制面和网页控制台地址。',
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Wrap(
            spacing: 10,
            runSpacing: 10,
            children: [
              DesktopBadge(label: 'mode $appCoreMode'),
              DesktopBadge(
                label:
                    hostConfig == null ? 'state mock' : 'state connected-host',
              ),
            ],
          ),
          const SizedBox(height: 14),
          TextField(
            key: AppTestKeys.authHostField,
            controller: hostController,
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
                onPressed: busy ? null : onApplyServer,
                icon: const Icon(Icons.sync_alt),
                label: const Text('Apply Host'),
              ),
              OutlinedButton(
                onPressed: busy ? null : onUseLocalHost,
                child: const Text('Use Local HTTP'),
              ),
              OutlinedButton(
                onPressed: busy ? null : onUseSecureLocalHost,
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
