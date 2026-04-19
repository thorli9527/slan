import 'package:flutter/material.dart';

import '../../infra/app_core/models/models.dart';
import '../../infra/app_core/scope/app_core_scope.dart';
import '../../testing/app_test_keys.dart';
import '../shared/desktop_client_widgets.dart';

class AuthPage extends StatefulWidget {
  const AuthPage({super.key});

  @override
  State<AuthPage> createState() => _AuthPageState();
}

class _AuthPageState extends State<AuthPage> {
  final _emailController = TextEditingController(text: 'user@example.com');
  final _passwordController = TextEditingController(text: 'password123');

  @override
  void dispose() {
    _emailController.dispose();
    _passwordController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final store = AppCoreScope.demo;
    return AnimatedBuilder(
      animation: store,
      builder: (context, _) {
        return LayoutBuilder(
          builder: (context, constraints) {
            final isDesktop = constraints.maxWidth >= 980;
            final child = isDesktop
                ? _buildDesktopWorkspace(context, store)
                : _buildCompactWorkspace(context, store);
            return SingleChildScrollView(
              padding: const EdgeInsets.all(16),
              child: child,
            );
          },
        );
      },
    );
  }

  Widget _buildCompactWorkspace(BuildContext context, dynamic store) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        _AuthHero(error: store.error),
        const SizedBox(height: 16),
        _AuthFormCard(
          emailController: _emailController,
          passwordController: _passwordController,
          busy: store.busy,
          onRegister: () => store.register(
                email: _emailController.text.trim(),
                password: _passwordController.text,
              ),
          onLogin: () => store.login(
                email: _emailController.text.trim(),
                password: _passwordController.text,
              ),
        ),
        const SizedBox(height: 16),
        _SessionCard(session: store.session),
      ],
    );
  }

  Widget _buildDesktopWorkspace(BuildContext context, dynamic store) {
    return Row(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Expanded(
          child: Column(
            children: [
              _AuthHero(error: store.error),
              const SizedBox(height: 16),
              _AuthFormCard(
                emailController: _emailController,
                passwordController: _passwordController,
                busy: store.busy,
                onRegister: () => store.register(
                      email: _emailController.text.trim(),
                      password: _passwordController.text,
                    ),
                onLogin: () => store.login(
                      email: _emailController.text.trim(),
                      password: _passwordController.text,
                    ),
              ),
            ],
          ),
        ),
        const SizedBox(width: 16),
        SizedBox(
          width: 340,
          child: _SessionCard(session: store.session),
        ),
      ],
    );
  }
}

class _AuthHero extends StatelessWidget {
  const _AuthHero({required this.error});

  final String? error;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return DesktopHeroPanel(
      title: 'Authentication',
      description:
          'Create or restore a local session before moving deeper into overlay and tunnel workflows.',
      backgroundColor: const Color(0xFFF0F6EA),
      footer: error == null
          ? null
          : Text(
              error!,
              style: TextStyle(color: theme.colorScheme.error),
            ),
    );
  }
}

class _AuthFormCard extends StatelessWidget {
  const _AuthFormCard({
    required this.emailController,
    required this.passwordController,
    required this.busy,
    required this.onRegister,
    required this.onLogin,
  });

  final TextEditingController emailController;
  final TextEditingController passwordController;
  final bool busy;
  final VoidCallback onRegister;
  final VoidCallback onLogin;

  @override
  Widget build(BuildContext context) {
    return DesktopSurfaceCard(
      title: 'Session Controls',
      subtitle:
          'The current client still uses mock register/login flows, but the desktop shell now treats them as a proper workspace.',
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          TextField(
            key: AppTestKeys.authEmailField,
            controller: emailController,
            decoration: const InputDecoration(labelText: 'Email'),
          ),
          const SizedBox(height: 12),
          TextField(
            key: AppTestKeys.authPasswordField,
            controller: passwordController,
            decoration: const InputDecoration(labelText: 'Password'),
            obscureText: true,
          ),
          const SizedBox(height: 16),
          Wrap(
            spacing: 12,
            runSpacing: 12,
            children: [
              FilledButton(
                key: AppTestKeys.authRegisterButton,
                onPressed: busy ? null : onRegister,
                child: const Text('Register'),
              ),
              OutlinedButton(
                key: AppTestKeys.authLoginButton,
                onPressed: busy ? null : onLogin,
                child: const Text('Login'),
              ),
            ],
          ),
        ],
      ),
    );
  }
}

class _SessionCard extends StatelessWidget {
  const _SessionCard({required this.session});

  final SessionModel? session;

  @override
  Widget build(BuildContext context) {
    return DesktopSurfaceCard(
      title: session == null ? 'No session' : 'Session Snapshot',
      subtitle: session == null
          ? 'Use register or login to create a local mock session.'
          : null,
      child: session == null
          ? const SizedBox.shrink()
          : Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text('User: ${session!.userId}'),
                const SizedBox(height: 8),
                Text('Access Token: ${session!.accessToken}'),
                const SizedBox(height: 8),
                Text('Device ID: ${session!.deviceId ?? 'not registered'}'),
              ],
            ),
    );
  }
}
