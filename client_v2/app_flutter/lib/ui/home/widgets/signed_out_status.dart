import 'package:flutter/material.dart';

/// 未登录状态提示。
///
/// 保持为独立组件，方便后续补充注册引导或登录状态说明而不增大首页文件。
class SignedOutStatus extends StatelessWidget {
  const SignedOutStatus({
    this.desktop = false,
    super.key,
  });

  final bool desktop;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Container(
      padding: EdgeInsets.symmetric(
        horizontal: 18,
        vertical: desktop ? 12 : 18,
      ),
      decoration: BoxDecoration(
        color: Colors.white,
        border: Border.all(color: const Color(0xffe2d6cf)),
        borderRadius: BorderRadius.circular(18),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Container(
                width: desktop ? 38 : 42,
                height: desktop ? 38 : 42,
                decoration: const BoxDecoration(
                  color: Color(0xfffff1e9),
                  borderRadius: BorderRadius.all(Radius.circular(14)),
                ),
                child: const Icon(
                  Icons.lock_open_rounded,
                  color: Color(0xffb85c2f),
                ),
              ),
              SizedBox(width: desktop ? 10 : 12),
              Expanded(
                child: Text(
                  '登录后启用组网',
                  style: theme.textTheme.titleMedium?.copyWith(
                    fontWeight: FontWeight.w800,
                  ),
                ),
              ),
            ],
          ),
          SizedBox(height: desktop ? 7 : 10),
          Text(
            '登录成功后即可启用虚拟网络，并接收当前设备的网络配置。',
            style: theme.textTheme.bodyMedium?.copyWith(
              color: theme.colorScheme.onSurfaceVariant,
              height: 1.35,
            ),
          ),
        ],
      ),
    );
  }
}
