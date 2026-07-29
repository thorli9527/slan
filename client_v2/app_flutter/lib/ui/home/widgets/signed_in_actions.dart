import 'package:flutter/material.dart';

/// 已登录后的快捷操作区。
class SignedInActions extends StatelessWidget {
  const SignedInActions({
    required this.desktop,
    required this.showConsole,
    required this.onOpenConsole,
    required this.onLogout,
    this.consoleBusy = false,
    super.key,
  });

  final bool desktop;

  /// 是否显示 Web Console 按钮。移动端不展示该入口。
  final bool showConsole;

  /// 打开 Web Console 的回调；按钮隐藏时可为空。
  final VoidCallback? onOpenConsole;

  /// Web Console 是否正在打开。
  final bool consoleBusy;

  /// 退出登录回调。
  final VoidCallback onLogout;

  @override
  Widget build(BuildContext context) {
    final children = <Widget>[
      if (showConsole)
        Expanded(
          child: SizedBox(
            height: desktop ? 42 : 38,
            child: FilledButton.icon(
              key: const Key('open-web-console'),
              onPressed: onOpenConsole,
              icon: consoleBusy
                  ? const SizedBox.square(
                      dimension: 16,
                      child: CircularProgressIndicator(strokeWidth: 2),
                    )
                  : const Icon(Icons.open_in_browser_rounded, size: 18),
              label: Text(
                desktop ? '打开 Web Console' : 'Web Console',
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
              ),
            ),
          ),
        ),
      if (showConsole) const SizedBox(width: 12),
      Expanded(
        child: SizedBox(
          height: desktop ? 42 : 38,
          child: OutlinedButton.icon(
            onPressed: onLogout,
            icon: const Icon(Icons.logout_rounded, size: 18),
            label: const Text('退出登录'),
          ),
        ),
      ),
    ];
    return Row(
      children: children,
    );
  }
}
