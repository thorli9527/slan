import 'package:flutter/material.dart';

import '../../infra/app_core/app_core_scope.dart';
import '../../infra/app_core/models.dart';

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
        return ListView(
          padding: const EdgeInsets.all(16),
          children: [
            TextField(
              controller: _emailController,
              decoration: const InputDecoration(labelText: 'Email'),
            ),
            const SizedBox(height: 12),
            TextField(
              controller: _passwordController,
              decoration: const InputDecoration(labelText: 'Password'),
              obscureText: true,
            ),
            const SizedBox(height: 16),
            Wrap(
              spacing: 12,
              runSpacing: 12,
              children: [
                FilledButton(
                  onPressed: store.busy
                      ? null
                      : () => store.register(
                            email: _emailController.text.trim(),
                            password: _passwordController.text,
                          ),
                  child: const Text('Register'),
                ),
                OutlinedButton(
                  onPressed: store.busy
                      ? null
                      : () => store.login(
                            email: _emailController.text.trim(),
                            password: _passwordController.text,
                          ),
                  child: const Text('Login'),
                ),
              ],
            ),
            const SizedBox(height: 24),
            _SessionCard(session: store.session),
            if (store.error != null) ...[
              const SizedBox(height: 12),
              Text(
                store.error!,
                style: TextStyle(color: Theme.of(context).colorScheme.error),
              ),
            ],
          ],
        );
      },
    );
  }
}

class _SessionCard extends StatelessWidget {
  const _SessionCard({required this.session});

  final SessionModel? session;

  @override
  Widget build(BuildContext context) {
    if (session == null) {
      return const Card(
        child: ListTile(
          title: Text('No session'),
          subtitle:
              Text('Use register or login to create a local mock session.'),
        ),
      );
    }
    return Card(
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text('User: ${session!.userId}'),
            const SizedBox(height: 8),
            Text('Access Token: ${session!.accessToken}'),
            const SizedBox(height: 8),
            Text('Device ID: ${session!.deviceId ?? 'not registered'}'),
          ],
        ),
      ),
    );
  }
}
