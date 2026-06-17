import 'package:flutter/material.dart';

/// 未登录状态提示。
///
/// 保持为独立组件，方便后续补充注册引导或登录状态说明而不增大首页文件。
class SignedOutStatus extends StatelessWidget {
  const SignedOutStatus({super.key});

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 12),
      decoration: BoxDecoration(
        border: Border.all(color: const Color(0xffe2d6cf)),
        borderRadius: BorderRadius.circular(8),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            '登录后启用组网',
            style: Theme.of(context).textTheme.titleMedium?.copyWith(
                  fontWeight: FontWeight.w700,
                ),
          ),
        ],
      ),
    );
  }
}
