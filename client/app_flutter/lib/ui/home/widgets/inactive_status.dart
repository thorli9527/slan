import 'package:flutter/material.dart';

/// 设备未激活状态提示。
///
/// 保持为独立组件，方便后续补充激活说明而不增大首页文件。
class InactiveStatus extends StatelessWidget {
  const InactiveStatus({this.desktop = false, super.key});

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
                  '激活设备后启用组网',
                  style: theme.textTheme.titleMedium?.copyWith(
                    fontWeight: FontWeight.w800,
                  ),
                ),
              ),
            ],
          ),
          SizedBox(height: desktop ? 7 : 10),
          Text(
            '使用运营后台为本设备签发的授权 Key 完成激活并同步网络配置。',
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
