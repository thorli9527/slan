import 'package:flutter/material.dart';

/// 已登录后的快捷操作区。
class SignedInActions extends StatelessWidget {
  const SignedInActions({
    required this.desktop,
    required this.onLogout,
    super.key,
  });

  final bool desktop;

  /// 退出登录回调。
  final VoidCallback onLogout;

  @override
  Widget build(BuildContext context) {
    return SizedBox(
      height: desktop ? 42 : 38,
      child: OutlinedButton.icon(
        key: const Key('logout'),
        onPressed: onLogout,
        icon: const Icon(Icons.logout_rounded, size: 18),
        label: const Text('退出登录'),
      ),
    );
  }
}
