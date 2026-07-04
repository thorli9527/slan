import 'package:flutter/material.dart';

/// 紧凑用户身份展示。
class CompactIdentity extends StatelessWidget {
  const CompactIdentity({required this.userLabel, super.key});

  /// 当前用户展示名。
  final String userLabel;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Row(
      children: [
        Icon(
          Icons.account_circle_outlined,
          size: 24,
          color: theme.colorScheme.primary,
        ),
        const SizedBox(width: 9),
        Expanded(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            mainAxisSize: MainAxisSize.min,
            children: [
              Text(
                '当前用户邮箱',
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
                style: theme.textTheme.labelMedium?.copyWith(
                  color: theme.colorScheme.onSurfaceVariant,
                  fontWeight: FontWeight.w800,
                ),
              ),
              const SizedBox(height: 2),
              Text(
                key: const Key('current-user-email-value'),
                userLabel,
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
                style: theme.textTheme.bodyMedium?.copyWith(
                  fontWeight: FontWeight.w900,
                ),
              ),
            ],
          ),
        ),
      ],
    );
  }
}

/// 图标、标签和值组成的紧凑信息行。
class CompactInfoRow extends StatelessWidget {
  const CompactInfoRow({
    this.valueKey,
    required this.icon,
    required this.label,
    required this.value,
    super.key,
  });

  /// 值文本的 key，供自动化测试定位。
  final Key? valueKey;

  /// 左侧图标。
  final IconData icon;

  /// 信息标签。
  final String label;

  /// 信息值。
  final String value;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Row(
      children: [
        Icon(icon, size: 18, color: theme.colorScheme.primary),
        const SizedBox(width: 9),
        Text(
          label,
          maxLines: 1,
          overflow: TextOverflow.ellipsis,
          style: theme.textTheme.labelMedium?.copyWith(
            color: theme.colorScheme.onSurfaceVariant,
            fontWeight: FontWeight.w800,
          ),
        ),
        const SizedBox(width: 12),
        Expanded(
          child: Text(
            key: valueKey,
            value,
            maxLines: 1,
            overflow: TextOverflow.ellipsis,
            textAlign: TextAlign.right,
            style: theme.textTheme.bodyMedium?.copyWith(
              fontWeight: FontWeight.w900,
            ),
          ),
        ),
      ],
    );
  }
}
