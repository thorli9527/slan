import 'package:flutter/material.dart';

/// 移动端账号密码登录表单。
///
/// 桌面端使用浏览器登录同步；Android/iOS 则在 App 内完成登录和服务器地址设置。
class PasswordLoginForm extends StatelessWidget {
  const PasswordLoginForm({
    required this.emailController,
    required this.passwordController,
    required this.serverBaseUrl,
    required this.syncing,
    required this.onSettings,
    required this.onSubmit,
    super.key,
  });

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
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        _ServerSettingsSummary(
          serverBaseUrl: serverBaseUrl,
          syncing: syncing,
          onSettings: onSettings,
        ),
        const SizedBox(height: 10),
        TextField(
          key: const Key('login-email'),
          controller: emailController,
          enabled: !syncing,
          keyboardType: TextInputType.emailAddress,
          textInputAction: TextInputAction.next,
          decoration: const InputDecoration(
            labelText: '账号',
            border: OutlineInputBorder(),
            isDense: true,
          ),
        ),
        const SizedBox(height: 10),
        TextField(
          key: const Key('login-password'),
          controller: passwordController,
          enabled: !syncing,
          obscureText: true,
          textInputAction: TextInputAction.done,
          onSubmitted: (_) => syncing ? null : onSubmit(),
          decoration: const InputDecoration(
            labelText: '密码',
            border: OutlineInputBorder(),
            isDense: true,
          ),
        ),
        const SizedBox(height: 12),
        SizedBox(
          height: 42,
          child: FilledButton(
            key: const Key('login-submit'),
            onPressed: syncing ? null : onSubmit,
            child: Text(syncing ? '登录中' : '登录'),
          ),
        ),
      ],
    );
  }
}

/// 登录表单顶部的服务器地址摘要。
class _ServerSettingsSummary extends StatelessWidget {
  const _ServerSettingsSummary({
    required this.serverBaseUrl,
    required this.syncing,
    required this.onSettings,
  });

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
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 10),
      decoration: BoxDecoration(
        borderRadius: BorderRadius.circular(8),
        border: Border.all(color: theme.colorScheme.outlineVariant),
      ),
      child: Row(
        children: [
          Icon(
            Icons.settings_ethernet_rounded,
            size: 18,
            color: theme.colorScheme.primary,
          ),
          const SizedBox(width: 8),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              mainAxisSize: MainAxisSize.min,
              children: [
                Text(
                  '服务器',
                  style: theme.textTheme.labelSmall?.copyWith(
                    color: theme.colorScheme.onSurfaceVariant,
                    fontWeight: FontWeight.w800,
                  ),
                ),
                Text(
                  value == null || value.isEmpty ? '未设置' : value,
                  key: const Key('server-base-url-value'),
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: theme.textTheme.bodyMedium?.copyWith(
                    fontWeight: FontWeight.w800,
                  ),
                ),
              ],
            ),
          ),
          IconButton(
            key: const Key('server-settings'),
            tooltip: '服务器设置',
            onPressed: syncing ? null : onSettings,
            icon: const Icon(Icons.settings_rounded),
          ),
        ],
      ),
    );
  }
}
