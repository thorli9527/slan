import 'package:flutter/material.dart';

class DesktopHeroPanel extends StatelessWidget {
  const DesktopHeroPanel({
    super.key,
    required this.title,
    required this.description,
    this.backgroundColor,
    this.foregroundColor,
    this.trailing,
    this.footer,
  });

  final String title;
  final String description;
  final Color? backgroundColor;
  final Color? foregroundColor;
  final Widget? trailing;
  final Widget? footer;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final textColor = foregroundColor ?? theme.colorScheme.onSurface;
    final secondaryTextColor = foregroundColor == null
        ? theme.colorScheme.onSurfaceVariant
        : foregroundColor!.withValues(alpha: 0.78);
    return Container(
      padding: const EdgeInsets.all(20),
      decoration: BoxDecoration(
        color: backgroundColor ?? theme.colorScheme.surfaceContainerLowest,
        borderRadius: BorderRadius.circular(24),
        border: Border.all(color: theme.colorScheme.outlineVariant),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          if (trailing != null) ...[
            Align(alignment: Alignment.topRight, child: trailing!),
            const SizedBox(height: 4),
          ],
          Text(
            title,
            style: theme.textTheme.headlineSmall?.copyWith(
              fontWeight: FontWeight.w800,
              color: textColor,
            ),
          ),
          const SizedBox(height: 8),
          Text(
            description,
            style: theme.textTheme.bodyLarge?.copyWith(
              height: 1.45,
              color: secondaryTextColor,
            ),
          ),
          if (footer != null) ...[
            const SizedBox(height: 14),
            footer!,
          ],
        ],
      ),
    );
  }
}

class DesktopSurfaceCard extends StatelessWidget {
  const DesktopSurfaceCard({
    super.key,
    required this.title,
    this.subtitle,
    this.backgroundColor,
    this.foregroundColor,
    required this.child,
  });

  final String title;
  final String? subtitle;
  final Color? backgroundColor;
  final Color? foregroundColor;
  final Widget child;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final textColor = foregroundColor ?? theme.colorScheme.onSurface;
    final secondaryTextColor = foregroundColor == null
        ? theme.colorScheme.onSurfaceVariant
        : foregroundColor!.withValues(alpha: 0.8);
    return Card(
      elevation: 0,
      color: backgroundColor ?? theme.colorScheme.surface,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(24),
        side: BorderSide(color: theme.colorScheme.outlineVariant),
      ),
      child: Padding(
        padding: const EdgeInsets.all(20),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(
              title,
              style: theme.textTheme.titleLarge?.copyWith(
                fontWeight: FontWeight.w800,
                color: textColor,
              ),
            ),
            if (subtitle != null) ...[
              const SizedBox(height: 6),
              Text(
                subtitle!,
                style: theme.textTheme.bodyMedium?.copyWith(
                  color: secondaryTextColor,
                  height: 1.4,
                ),
              ),
            ],
            const SizedBox(height: 18),
            child,
          ],
        ),
      ),
    );
  }
}

class DesktopMetricPill extends StatelessWidget {
  const DesktopMetricPill({
    super.key,
    required this.label,
    required this.value,
    this.backgroundColor,
    this.foregroundColor,
    this.borderColor,
  });

  final String label;
  final String value;
  final Color? backgroundColor;
  final Color? foregroundColor;
  final Color? borderColor;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final textColor = foregroundColor ?? theme.colorScheme.onSurface;
    final secondaryTextColor = foregroundColor == null
        ? theme.colorScheme.onSurfaceVariant
        : foregroundColor!.withValues(alpha: 0.78);
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 12),
      decoration: BoxDecoration(
        color: backgroundColor ?? theme.colorScheme.surfaceContainerLowest,
        borderRadius: BorderRadius.circular(18),
        border: Border.all(
          color: borderColor ?? theme.colorScheme.outlineVariant,
        ),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(label, style: TextStyle(color: secondaryTextColor)),
          const SizedBox(height: 2),
          Text(
            value,
            style: TextStyle(
              color: textColor,
              fontWeight: FontWeight.w700,
            ),
          ),
        ],
      ),
    );
  }
}

class DesktopInsetBlock extends StatelessWidget {
  const DesktopInsetBlock({
    super.key,
    required this.title,
    required this.child,
  });

  final String title;
  final Widget child;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Container(
      width: double.infinity,
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: theme.colorScheme.surfaceContainerLowest,
        borderRadius: BorderRadius.circular(20),
        border: Border.all(color: theme.colorScheme.outlineVariant),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            title,
            style: theme.textTheme.titleMedium?.copyWith(
              fontWeight: FontWeight.w700,
            ),
          ),
          const SizedBox(height: 14),
          child,
        ],
      ),
    );
  }
}

class DesktopBadge extends StatelessWidget {
  const DesktopBadge({
    super.key,
    required this.label,
  });

  final String label;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 10),
      decoration: BoxDecoration(
        color: Colors.white,
        borderRadius: BorderRadius.circular(18),
        border: Border.all(color: theme.colorScheme.outlineVariant),
      ),
      child: Text(
        label,
        style: const TextStyle(fontWeight: FontWeight.w700),
      ),
    );
  }
}

class DesktopNavigationSidebar extends StatelessWidget {
  const DesktopNavigationSidebar({
    super.key,
    required this.title,
    required this.subtitle,
    required this.children,
    this.footer,
    this.width = 248,
  });

  final String title;
  final String subtitle;
  final List<Widget> children;
  final Widget? footer;
  final double width;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Container(
      width: width,
      padding: const EdgeInsets.fromLTRB(18, 24, 18, 18),
      decoration: BoxDecoration(
        color: const Color(0xFF0E1E18),
        borderRadius: BorderRadius.circular(32),
        boxShadow: [
          BoxShadow(
            color: Colors.black.withValues(alpha: 0.08),
            blurRadius: 32,
            offset: const Offset(0, 14),
          ),
        ],
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            title,
            style: theme.textTheme.headlineSmall?.copyWith(
              color: Colors.white,
              fontWeight: FontWeight.w800,
              letterSpacing: 0.6,
            ),
          ),
          const SizedBox(height: 8),
          Text(
            subtitle,
            style: theme.textTheme.bodyMedium?.copyWith(
              color: Colors.white70,
            ),
          ),
          const SizedBox(height: 24),
          ...children,
          if (footer != null) ...[
            const Spacer(),
            footer!,
          ],
        ],
      ),
    );
  }
}

class DesktopNavigationItem extends StatelessWidget {
  const DesktopNavigationItem({
    super.key,
    required this.icon,
    required this.label,
    required this.selected,
    required this.onTap,
  });

  final IconData icon;
  final String label;
  final bool selected;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    return Material(
      color: Colors.transparent,
      child: InkWell(
        borderRadius: BorderRadius.circular(18),
        onTap: onTap,
        child: AnimatedContainer(
          duration: const Duration(milliseconds: 180),
          padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 14),
          decoration: BoxDecoration(
            color: selected ? const Color(0xFFB8FF7A) : Colors.transparent,
            borderRadius: BorderRadius.circular(18),
          ),
          child: Row(
            children: [
              Icon(
                icon,
                color: selected ? const Color(0xFF0E1E18) : Colors.white70,
              ),
              const SizedBox(width: 12),
              Text(
                label,
                style: TextStyle(
                  color: selected ? const Color(0xFF0E1E18) : Colors.white,
                  fontWeight: FontWeight.w700,
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

class DesktopOverviewPanel extends StatelessWidget {
  const DesktopOverviewPanel({
    super.key,
    required this.title,
    required this.child,
    this.backgroundColor,
    this.foregroundColor,
  });

  final String title;
  final Widget child;
  final Color? backgroundColor;
  final Color? foregroundColor;

  @override
  Widget build(BuildContext context) {
    return DesktopSurfaceCard(
      title: title,
      backgroundColor: backgroundColor,
      foregroundColor: foregroundColor,
      child: DefaultTextStyle(
        style: TextStyle(
          color: foregroundColor,
        ),
        child: child,
      ),
    );
  }
}

class DesktopKeyValueList extends StatelessWidget {
  const DesktopKeyValueList({
    super.key,
    required this.entries,
  });

  final List<DesktopKeyValueEntry> entries;

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        for (var index = 0; index < entries.length; index++) ...[
          DesktopKeyValueRow(
            label: entries[index].label,
            value: entries[index].value,
          ),
          if (index != entries.length - 1) const SizedBox(height: 10),
        ],
      ],
    );
  }
}

class DesktopKeyValueEntry {
  const DesktopKeyValueEntry({
    required this.label,
    required this.value,
  });

  final String label;
  final String value;
}

class DesktopKeyValueRow extends StatelessWidget {
  const DesktopKeyValueRow({
    super.key,
    required this.label,
    required this.value,
    this.labelWidth = 108,
  });

  final String label;
  final String value;
  final double labelWidth;

  @override
  Widget build(BuildContext context) {
    return Row(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        SizedBox(
          width: labelWidth,
          child: Text(
            label,
            style: TextStyle(color: DefaultTextStyle.of(context).style.color),
          ),
        ),
        Expanded(
          child: Text(
            value,
            style: const TextStyle(fontWeight: FontWeight.w700),
          ),
        ),
      ],
    );
  }
}

class DesktopWorkspaceFrame extends StatelessWidget {
  const DesktopWorkspaceFrame({
    super.key,
    required this.child,
    this.borderRadius = 28,
    this.backgroundColor,
  });

  final Widget child;
  final double borderRadius;
  final Color? backgroundColor;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return DecoratedBox(
      decoration: BoxDecoration(
        color: backgroundColor ?? theme.colorScheme.surface.withValues(alpha: 0.94),
        borderRadius: BorderRadius.circular(borderRadius),
        border: Border.all(color: theme.colorScheme.outlineVariant),
      ),
      child: ClipRRect(
        borderRadius: BorderRadius.circular(borderRadius),
        child: child,
      ),
    );
  }
}
