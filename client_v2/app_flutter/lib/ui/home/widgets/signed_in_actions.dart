import 'package:flutter/material.dart';

/// 已登录后的快捷操作区。
class SignedInActions extends StatelessWidget {
  const SignedInActions({
    required this.showConsole,
    required this.onOpenConsole,
    required this.onLogout,
    super.key,
  });

  /// 是否显示 Web Console 按钮。移动端不展示该入口。
  final bool showConsole;

  /// 打开 Web Console 的回调；按钮隐藏时可为空。
  final VoidCallback? onOpenConsole;

  /// 退出登录回调。
  final VoidCallback onLogout;

  @override
  Widget build(BuildContext context) {
    return Row(
      children: [
        if (showConsole) ...[
          Expanded(
            child: SizedBox(
              height: 38,
              child: OutlinedButton.icon(
                onPressed: onOpenConsole,
                icon: const Icon(Icons.open_in_browser_rounded, size: 17),
                label: const Text('Web Console'),
              ),
            ),
          ),
          const SizedBox(width: 8),
        ],
        Expanded(
          child: SizedBox(
            height: 38,
            child: OutlinedButton.icon(
              onPressed: onLogout,
              icon: const Icon(Icons.logout_rounded, size: 17),
              label: const Text('Logout'),
            ),
          ),
        ),
      ],
    );
  }
}
