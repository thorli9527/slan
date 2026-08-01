import 'package:flutter/material.dart';

/// 账号密码登录表单，桌面端使用紧凑横向输入，移动端使用纵向输入。
class PasswordLoginForm extends StatelessWidget {
  const PasswordLoginForm({
    required this.desktop,
    required this.emailController,
    required this.passwordController,
    required this.serverBaseUrl,
    required this.syncing,
    required this.onSettings,
    required this.onSubmit,
    super.key,
  });

  final bool desktop;

  /// 账号输入框控制器。
  final TextEditingController emailController;

  /// 密码输入框控制器。
  final TextEditingController passwordController;

  /// 当前控制面 API 地址。
  final String? serverBaseUrl;

  /// 登录或同步中时禁用输入。
  final bool syncing;

  /// 打开服务器设置弹窗。
  final VoidCallback onSettings;

  /// 提交登录。
  final VoidCallback onSubmit;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Container(
      key: const Key('login-panel'),
      padding: EdgeInsets.all(desktop ? 12 : 16),
      decoration: BoxDecoration(
        color: Colors.white,
        borderRadius: BorderRadius.circular(desktop ? 12 : 16),
        border: Border.all(color: const Color(0xffe2d6cf)),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          _ServerSettingsSummary(
            desktop: desktop,
            serverBaseUrl: serverBaseUrl,
            syncing: syncing,
            onSettings: onSettings,
          ),
          SizedBox(height: desktop ? 8 : 12),
          if (desktop)
            Row(
              children: [
                Expanded(child: _emailField(theme)),
                const SizedBox(width: 8),
                Expanded(child: _passwordField(theme)),
              ],
            )
          else ...[
            _emailField(theme),
            const SizedBox(height: 10),
            _passwordField(theme),
          ],
          SizedBox(height: desktop ? 10 : 14),
          SizedBox(
            height: desktop ? 38 : 44,
            child: FilledButton.icon(
              key: const Key('login-submit'),
              onPressed: syncing ? null : onSubmit,
              icon: syncing
                  ? const SizedBox(
                      width: 14,
                      height: 14,
                      child: CircularProgressIndicator(
                        strokeWidth: 2,
                        color: Colors.white,
                      ),
                    )
                  : const Icon(Icons.login_rounded, size: 18),
              label: Text(syncing ? '登录中' : '登录'),
            ),
          ),
        ],
      ),
    );
  }

  Widget _emailField(ThemeData theme) => SizedBox(
        height: desktop ? 40 : 46,
        child: TextField(
          key: const Key('login-email'),
          controller: emailController,
          enabled: !syncing,
          keyboardType: TextInputType.emailAddress,
          textInputAction: TextInputAction.next,
          style: theme.textTheme.bodyMedium,
          decoration: const InputDecoration(
            hintText: '用户名',
            prefixIcon: Icon(Icons.person_outline_rounded, size: 19),
            prefixIconConstraints: BoxConstraints(minWidth: 38),
            contentPadding: EdgeInsets.symmetric(horizontal: 10, vertical: 10),
            fillColor: Color(0xfffbf9f7),
          ),
        ),
      );

  Widget _passwordField(ThemeData theme) => SizedBox(
        height: desktop ? 40 : 46,
        child: TextField(
          key: const Key('login-password'),
          controller: passwordController,
          enabled: !syncing,
          obscureText: true,
          textInputAction: TextInputAction.done,
          onSubmitted: (_) => syncing ? null : onSubmit(),
          style: theme.textTheme.bodyMedium,
          decoration: const InputDecoration(
            hintText: '密码',
            prefixIcon: Icon(Icons.lock_outline_rounded, size: 18),
            prefixIconConstraints: BoxConstraints(minWidth: 38),
            contentPadding: EdgeInsets.symmetric(horizontal: 10, vertical: 10),
            fillColor: Color(0xfffbf9f7),
          ),
        ),
      );
}

/// 登录表单顶部的服务器地址摘要。
class _ServerSettingsSummary extends StatelessWidget {
  const _ServerSettingsSummary({
    required this.desktop,
    required this.serverBaseUrl,
    required this.syncing,
    required this.onSettings,
  });

  final bool desktop;

  /// 当前控制面 API 地址。
  final String? serverBaseUrl;

  /// 同步中时禁用设置按钮，避免登录过程中切换服务端。
  final bool syncing;

  /// 打开服务器设置弹窗。
  final VoidCallback onSettings;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final value = serverBaseUrl?.trim();
    return SizedBox(
      height: desktop ? 32 : 40,
      child: Container(
        padding: const EdgeInsets.only(left: 10, right: 2),
        decoration: BoxDecoration(
          color: const Color(0xfffff5ef),
          borderRadius: BorderRadius.circular(8),
        ),
        child: Row(
          children: [
            Icon(
              Icons.settings_ethernet_rounded,
              size: 16,
              color: theme.colorScheme.primary,
            ),
            const SizedBox(width: 7),
            Text(
              '服务器',
              style: theme.textTheme.labelMedium?.copyWith(
                color: theme.colorScheme.primary,
                fontWeight: FontWeight.w700,
              ),
            ),
            const SizedBox(width: 8),
            Expanded(
              child: Text(
                value == null || value.isEmpty ? '未设置' : value,
                key: const Key('server-base-url-value'),
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
                style: theme.textTheme.bodySmall?.copyWith(
                  color: theme.colorScheme.onSurfaceVariant,
                ),
              ),
            ),
            IconButton(
              key: const Key('server-settings'),
              tooltip: '服务器设置',
              onPressed: syncing ? null : onSettings,
              padding: EdgeInsets.zero,
              constraints: const BoxConstraints.tightFor(width: 32, height: 32),
              iconSize: 18,
              icon: const Icon(Icons.settings_rounded),
            ),
          ],
        ),
      ),
    );
  }
}
