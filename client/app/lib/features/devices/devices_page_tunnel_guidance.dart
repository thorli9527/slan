part of 'devices_page.dart';

_TunnelActionPhaseGuidance _deriveTunnelActionPhaseGuidance(
  TunnelActionReport? report,
) {
  if (report == null) {
    return const _TunnelActionPhaseGuidance();
  }
  final nextStepLabel = switch (report.phase) {
    TunnelActionPhase.accepted => 'Inspect runtime and wait for backend state',
    TunnelActionPhase.configured => 'Bring the tunnel up',
    TunnelActionPhase.started => 'Inspect runtime or leave monitor running',
    TunnelActionPhase.verified => 'Observe runtime or bring the tunnel down',
    TunnelActionPhase.failed => 'Recover session or re-apply configuration',
    TunnelActionPhase.pendingVerification =>
      'Inspect runtime again to confirm backend progress',
  };
  return _TunnelActionPhaseGuidance(
    currentPhaseLabel: _tunnelActionPhaseLabel(report.phase),
    nextStepLabel: nextStepLabel,
    signalSourceLabel: report.sourceLabel,
  );
}

enum _TunnelActionStatus { running, succeeded, failed }

enum _TunnelActionKind {
  setup,
  apply,
  bootstrap,
  recover,
  up,
  inspect,
  down,
  removePeer
}

enum _TunnelSessionHealth { healthy, degraded, failed, idle }

class _TunnelSessionHealthSummary {
  const _TunnelSessionHealthSummary({
    required this.health,
    required this.reason,
    required this.supportingSignal,
    required this.recommendedAction,
    required this.trendLabel,
    required this.trendSignal,
    this.dataPlaneSignal,
    this.recommendationMatch,
    this.primaryAction,
    this.primaryActionLabel,
    this.secondaryAction,
    this.secondaryActionLabel,
  });

  final _TunnelSessionHealth health;
  final String reason;
  final String supportingSignal;
  final String recommendedAction;
  final String trendLabel;
  final String trendSignal;
  final String? dataPlaneSignal;
  final String? recommendationMatch;
  final _TunnelActionKind? primaryAction;
  final String? primaryActionLabel;
  final _TunnelActionKind? secondaryAction;
  final String? secondaryActionLabel;
}

enum _RelayDataPlaneFailureKind { auth, session, protocol }

class _RelayDataPlaneFailureSignal {
  const _RelayDataPlaneFailureSignal({
    required this.kind,
    required this.label,
    required this.summary,
  });

  final _RelayDataPlaneFailureKind kind;
  final String label;
  final String summary;
}

class _TunnelHealthSnapshot {
  const _TunnelHealthSnapshot({
    required this.health,
    required this.occurredAt,
  });

  final _TunnelSessionHealth health;
  final DateTime occurredAt;
}

class _TunnelHealthTrendSummary {
  const _TunnelHealthTrendSummary({
    required this.label,
    required this.signal,
  });

  final String label;
  final String signal;
}

class _TunnelActionPhaseGuidance {
  const _TunnelActionPhaseGuidance({
    this.currentPhaseLabel,
    this.nextStepLabel,
    this.signalSourceLabel,
  });

  final String? currentPhaseLabel;
  final String? nextStepLabel;
  final String? signalSourceLabel;
}

class _TunnelActionEvent {
  const _TunnelActionEvent({
    required this.kind,
    required this.label,
    required this.detail,
    required this.status,
    required this.occurredAt,
    this.progressLabel,
    this.signalSourceLabel,
  });

  final _TunnelActionKind kind;
  final String label;
  final String detail;
  final _TunnelActionStatus status;
  final DateTime occurredAt;
  final String? progressLabel;
  final String? signalSourceLabel;
}

class _TunnelActionBanner extends StatelessWidget {
  const _TunnelActionBanner({
    required this.event,
    required this.nextStepLabel,
  });

  final _TunnelActionEvent event;
  final String nextStepLabel;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final scheme = theme.colorScheme;
    final (backgroundColor, borderColor, foregroundColor) =
        switch (event.status) {
      _TunnelActionStatus.running => (
          scheme.primaryContainer,
          scheme.primary,
          scheme.onPrimaryContainer,
        ),
      _TunnelActionStatus.succeeded => (
          scheme.tertiaryContainer,
          scheme.tertiary,
          scheme.onTertiaryContainer,
        ),
      _TunnelActionStatus.failed => (
          scheme.errorContainer,
          scheme.error,
          scheme.onErrorContainer,
        ),
    };

    return Container(
      width: double.infinity,
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: backgroundColor,
        borderRadius: BorderRadius.circular(20),
        border: Border.all(color: borderColor),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            '${_statusLabel(event.status)} 路 ${event.label}',
            style: theme.textTheme.titleMedium?.copyWith(
              color: foregroundColor,
              fontWeight: FontWeight.w800,
            ),
          ),
          if (event.progressLabel != null) ...[
            const SizedBox(height: 6),
            Text(
              event.progressLabel!,
              style: theme.textTheme.bodySmall?.copyWith(
                color: foregroundColor,
                fontWeight: FontWeight.w700,
              ),
            ),
          ],
          const SizedBox(height: 6),
          Text(
            event.detail,
            style: theme.textTheme.bodyMedium?.copyWith(
              color: foregroundColor,
              height: 1.4,
            ),
          ),
          if (event.status != _TunnelActionStatus.running) ...[
            const SizedBox(height: 8),
            if (event.signalSourceLabel != null) ...[
              Text(
                'Signal source: ${event.signalSourceLabel!}',
                style: theme.textTheme.bodySmall?.copyWith(
                  color: foregroundColor,
                  fontWeight: FontWeight.w700,
                ),
              ),
              const SizedBox(height: 6),
            ],
            Text(
              'Next recommended step: $nextStepLabel',
              style: theme.textTheme.bodySmall?.copyWith(
                color: foregroundColor,
                fontWeight: FontWeight.w700,
              ),
            ),
          ],
        ],
      ),
    );
  }
}

class _TunnelActionTimelineRow extends StatelessWidget {
  const _TunnelActionTimelineRow({required this.event});

  final _TunnelActionEvent event;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final color = switch (event.status) {
      _TunnelActionStatus.running => theme.colorScheme.primary,
      _TunnelActionStatus.succeeded => theme.colorScheme.tertiary,
      _TunnelActionStatus.failed => theme.colorScheme.error,
    };

    return Row(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Container(
          width: 10,
          height: 10,
          margin: const EdgeInsets.only(top: 4),
          decoration: BoxDecoration(
            color: color,
            borderRadius: BorderRadius.circular(999),
          ),
        ),
        const SizedBox(width: 12),
        Expanded(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(
                '${event.label} 路 ${_statusLabel(event.status)}',
                style: theme.textTheme.bodyMedium?.copyWith(
                  fontWeight: FontWeight.w800,
                ),
              ),
              const SizedBox(height: 4),
              Text(
                event.detail,
                style: theme.textTheme.bodySmall?.copyWith(
                  color: theme.colorScheme.onSurfaceVariant,
                  height: 1.4,
                ),
              ),
            ],
          ),
        ),
        const SizedBox(width: 12),
        Text(
          _formatActionTime(event.occurredAt),
          style: theme.textTheme.bodySmall?.copyWith(
            color: theme.colorScheme.onSurfaceVariant,
          ),
        ),
      ],
    );
  }
}

class _TunnelHealthSummaryPanel extends StatelessWidget {
  const _TunnelHealthSummaryPanel({
    required this.summary,
    required this.recentSnapshots,
    required this.busy,
    required this.onPrimaryAction,
    required this.onSecondaryAction,
  });

  final _TunnelSessionHealthSummary summary;
  final List<_TunnelHealthSnapshot> recentSnapshots;
  final bool busy;
  final Future<void> Function() onPrimaryAction;
  final Future<void> Function() onSecondaryAction;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Wrap(
          spacing: 10,
          runSpacing: 10,
          children: [
            DesktopMetricPill(
              label: 'Health',
              value: _sessionHealthLabel(summary.health),
              backgroundColor: _sessionHealthBackgroundColor(
                context,
                summary.health,
              ),
              foregroundColor: _sessionHealthForegroundColor(
                context,
                summary.health,
              ),
              borderColor: _sessionHealthBorderColor(
                context,
                summary.health,
              ),
            ),
          ],
        ),
        const SizedBox(height: 14),
        Text(
          summary.reason,
          style: theme.textTheme.bodyMedium?.copyWith(
            color: theme.colorScheme.onSurfaceVariant,
            height: 1.45,
          ),
        ),
        const SizedBox(height: 10),
        Text(
          'Observed: ${summary.supportingSignal}',
          style: theme.textTheme.bodySmall?.copyWith(
            color: theme.colorScheme.onSurfaceVariant,
            height: 1.45,
          ),
        ),
        if (summary.recommendationMatch != null) ...[
          const SizedBox(height: 10),
          Text(
            summary.recommendationMatch!,
            style: theme.textTheme.bodySmall?.copyWith(
              color: theme.colorScheme.onSurfaceVariant,
              height: 1.45,
            ),
          ),
        ],
        const SizedBox(height: 10),
        Text(
          'Trend: ${summary.trendLabel}',
          style: theme.textTheme.bodySmall?.copyWith(
            color: theme.colorScheme.onSurfaceVariant,
            height: 1.45,
            fontWeight: FontWeight.w700,
          ),
        ),
        const SizedBox(height: 6),
        Text(
          summary.trendSignal,
          style: theme.textTheme.bodySmall?.copyWith(
            color: theme.colorScheme.onSurfaceVariant,
            height: 1.45,
          ),
        ),
        const SizedBox(height: 12),
        _TunnelHealthTimeline(snapshots: recentSnapshots),
        const SizedBox(height: 10),
        Text(
          'Recommended action: ${summary.recommendedAction}',
          style: theme.textTheme.bodySmall?.copyWith(
            color: theme.colorScheme.onSurfaceVariant,
            height: 1.45,
            fontWeight: FontWeight.w700,
          ),
        ),
        if (summary.primaryAction != null ||
            summary.secondaryAction != null) ...[
          const SizedBox(height: 14),
          Wrap(
            spacing: 12,
            runSpacing: 12,
            children: [
              if (summary.primaryAction != null)
                FilledButton(
                  onPressed: busy ? null : onPrimaryAction,
                  child: Text(summary.primaryActionLabel!),
                ),
              if (summary.secondaryAction != null)
                OutlinedButton(
                  onPressed: busy ? null : onSecondaryAction,
                  child: Text(summary.secondaryActionLabel!),
                ),
            ],
          ),
        ],
      ],
    );
  }
}

class _TunnelHealthTimeline extends StatelessWidget {
  const _TunnelHealthTimeline({required this.snapshots});

  final List<_TunnelHealthSnapshot> snapshots;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final visibleSnapshots = snapshots.take(4).toList(growable: false);
    if (visibleSnapshots.isEmpty) {
      return Text(
        'Recent checks: waiting for health history.',
        style: theme.textTheme.bodySmall?.copyWith(
          color: theme.colorScheme.onSurfaceVariant,
          height: 1.45,
        ),
      );
    }

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          'Recent checks',
          style: theme.textTheme.bodySmall?.copyWith(
            color: theme.colorScheme.onSurfaceVariant,
            fontWeight: FontWeight.w700,
          ),
        ),
        const SizedBox(height: 10),
        Row(
          children: List.generate(visibleSnapshots.length, (index) {
            final snapshot = visibleSnapshots[index];
            return Expanded(
              child: Padding(
                padding: EdgeInsets.only(
                  right: index == visibleSnapshots.length - 1 ? 0 : 8,
                ),
                child: _TunnelHealthTimelineNode(
                  snapshot: snapshot,
                  isLatest: index == 0,
                ),
              ),
            );
          }),
        ),
      ],
    );
  }
}

class _TunnelHealthTimelineNode extends StatelessWidget {
  const _TunnelHealthTimelineNode({
    required this.snapshot,
    required this.isLatest,
  });

  final _TunnelHealthSnapshot snapshot;
  final bool isLatest;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Container(
      padding: const EdgeInsets.all(10),
      decoration: BoxDecoration(
        color: _sessionHealthBackgroundColor(context, snapshot.health),
        borderRadius: BorderRadius.circular(16),
        border: Border.all(
          color: _sessionHealthBorderColor(context, snapshot.health),
          width: isLatest ? 1.4 : 1,
        ),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            isLatest ? 'now' : _formatActionTime(snapshot.occurredAt),
            style: theme.textTheme.bodySmall?.copyWith(
              color: _sessionHealthForegroundColor(context, snapshot.health),
              fontWeight: FontWeight.w800,
            ),
          ),
          const SizedBox(height: 6),
          Text(
            _sessionHealthLabel(snapshot.health),
            style: theme.textTheme.bodySmall?.copyWith(
              color: _sessionHealthForegroundColor(context, snapshot.health),
              height: 1.3,
            ),
          ),
        ],
      ),
    );
  }
}

Color _sessionHealthBackgroundColor(
  BuildContext context,
  _TunnelSessionHealth health,
) {
  final scheme = Theme.of(context).colorScheme;
  return switch (health) {
    _TunnelSessionHealth.healthy => scheme.tertiaryContainer,
    _TunnelSessionHealth.degraded => scheme.secondaryContainer,
    _TunnelSessionHealth.failed => scheme.errorContainer,
    _TunnelSessionHealth.idle => scheme.surfaceContainerLowest,
  };
}

Color _sessionHealthForegroundColor(
  BuildContext context,
  _TunnelSessionHealth health,
) {
  final scheme = Theme.of(context).colorScheme;
  return switch (health) {
    _TunnelSessionHealth.healthy => scheme.onTertiaryContainer,
    _TunnelSessionHealth.degraded => scheme.onSecondaryContainer,
    _TunnelSessionHealth.failed => scheme.onErrorContainer,
    _TunnelSessionHealth.idle => scheme.onSurface,
  };
}

Color _sessionHealthBorderColor(
  BuildContext context,
  _TunnelSessionHealth health,
) {
  final scheme = Theme.of(context).colorScheme;
  return switch (health) {
    _TunnelSessionHealth.healthy => scheme.tertiary,
    _TunnelSessionHealth.degraded => scheme.secondary,
    _TunnelSessionHealth.failed => scheme.error,
    _TunnelSessionHealth.idle => scheme.outlineVariant,
  };
}

class _TunnelStagePlan {
  const _TunnelStagePlan({
    required this.nextStepLabel,
    required this.summary,
    required this.recoveryTitle,
    required this.recoverySteps,
    required this.steps,
    required this.primaryAction,
    required this.primaryActionLabel,
    this.primaryRecoveryAction,
    this.primaryRecoveryLabel,
    this.secondaryRecoveryAction,
    this.secondaryRecoveryLabel,
  });

  final String nextStepLabel;
  final String summary;
  final String recoveryTitle;
  final List<String> recoverySteps;
  final List<_TunnelStageStep> steps;
  final _TunnelActionKind? primaryAction;
  final String primaryActionLabel;
  final _TunnelActionKind? primaryRecoveryAction;
  final String? primaryRecoveryLabel;
  final _TunnelActionKind? secondaryRecoveryAction;
  final String? secondaryRecoveryLabel;
}

class _TunnelStageStep {
  const _TunnelStageStep({
    required this.title,
    required this.description,
    required this.state,
  });

  final String title;
  final String description;
  final _TunnelStageState state;
}

enum _TunnelStageState { complete, current, pending, blocked }

class _TunnelStageFlow extends StatelessWidget {
  const _TunnelStageFlow({
    required this.plan,
    required this.busy,
    required this.onRunNext,
  });

  final _TunnelStagePlan plan;
  final bool busy;
  final Future<void> Function() onRunNext;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          plan.summary,
          style: theme.textTheme.bodyMedium?.copyWith(
            color: theme.colorScheme.onSurfaceVariant,
            height: 1.45,
          ),
        ),
        const SizedBox(height: 16),
        for (var index = 0; index < plan.steps.length; index++) ...[
          _TunnelStageRow(
            index: index + 1,
            step: plan.steps[index],
          ),
          if (index != plan.steps.length - 1) const SizedBox(height: 12),
        ],
        const SizedBox(height: 16),
        Align(
          alignment: Alignment.centerLeft,
          child: FilledButton(
            onPressed: busy || plan.primaryAction == null ? null : onRunNext,
            child: Text(plan.primaryActionLabel),
          ),
        ),
      ],
    );
  }
}

class _TunnelStageRow extends StatelessWidget {
  const _TunnelStageRow({
    required this.index,
    required this.step,
  });

  final int index;
  final _TunnelStageStep step;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final (fill, foreground) = switch (step.state) {
      _TunnelStageState.complete => (
          theme.colorScheme.tertiary,
          theme.colorScheme.onTertiary,
        ),
      _TunnelStageState.current => (
          theme.colorScheme.primary,
          theme.colorScheme.onPrimary,
        ),
      _TunnelStageState.blocked => (
          theme.colorScheme.error,
          theme.colorScheme.onError,
        ),
      _TunnelStageState.pending => (
          theme.colorScheme.surfaceContainerHighest,
          theme.colorScheme.onSurfaceVariant,
        ),
    };

    return Row(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Container(
          width: 28,
          height: 28,
          alignment: Alignment.center,
          decoration: BoxDecoration(
            color: fill,
            borderRadius: BorderRadius.circular(999),
          ),
          child: Text(
            '$index',
            style: theme.textTheme.labelLarge?.copyWith(
              color: foreground,
              fontWeight: FontWeight.w800,
            ),
          ),
        ),
        const SizedBox(width: 12),
        Expanded(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(
                step.title,
                style: theme.textTheme.bodyMedium?.copyWith(
                  fontWeight: FontWeight.w800,
                ),
              ),
              const SizedBox(height: 4),
              Text(
                step.description,
                style: theme.textTheme.bodySmall?.copyWith(
                  color: theme.colorScheme.onSurfaceVariant,
                  height: 1.4,
                ),
              ),
            ],
          ),
        ),
      ],
    );
  }
}

class _TunnelStageOverview extends StatelessWidget {
  const _TunnelStageOverview({
    required this.plan,
    required this.latestAction,
    required this.sessionHealth,
  });

  final _TunnelStagePlan plan;
  final _TunnelActionEvent? latestAction;
  final _TunnelSessionHealth sessionHealth;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final blocked = plan.steps.any(
      (step) => step.state == _TunnelStageState.blocked,
    );
    final completed = plan.steps
        .where((step) => step.state == _TunnelStageState.complete)
        .length;

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Wrap(
          spacing: 10,
          runSpacing: 10,
          children: [
            DesktopMetricPill(
              label: 'Progress',
              value: blocked ? 'blocked' : '$completed/3 complete',
              backgroundColor: blocked
                  ? theme.colorScheme.errorContainer
                  : theme.colorScheme.surfaceContainerLowest,
              foregroundColor:
                  blocked ? theme.colorScheme.onErrorContainer : null,
              borderColor: blocked ? theme.colorScheme.error : null,
            ),
            DesktopMetricPill(
              label: 'Next',
              value: plan.nextStepLabel,
            ),
            DesktopMetricPill(
              label: 'Health',
              value: _sessionHealthLabel(sessionHealth),
              backgroundColor: _sessionHealthBackgroundColor(
                context,
                sessionHealth,
              ),
              foregroundColor: _sessionHealthForegroundColor(
                context,
                sessionHealth,
              ),
              borderColor: _sessionHealthBorderColor(
                context,
                sessionHealth,
              ),
            ),
          ],
        ),
        const SizedBox(height: 14),
        for (var index = 0; index < plan.steps.length; index++) ...[
          _TunnelStageOverviewCard(
            index: index + 1,
            step: plan.steps[index],
          ),
          if (index != plan.steps.length - 1) const SizedBox(height: 10),
        ],
        if (latestAction != null) ...[
          const SizedBox(height: 14),
          Text(
            'Latest action: ${latestAction!.label} 路 ${_statusLabel(latestAction!.status)}',
            style: theme.textTheme.bodySmall?.copyWith(
              color: theme.colorScheme.onSurfaceVariant,
              fontWeight: FontWeight.w700,
            ),
          ),
        ],
      ],
    );
  }
}

class _TunnelStageOverviewCard extends StatelessWidget {
  const _TunnelStageOverviewCard({
    required this.index,
    required this.step,
  });

  final int index;
  final _TunnelStageStep step;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final (backgroundColor, borderColor, accentColor) = switch (step.state) {
      _TunnelStageState.complete => (
          theme.colorScheme.tertiaryContainer,
          theme.colorScheme.tertiary,
          theme.colorScheme.tertiary,
        ),
      _TunnelStageState.current => (
          theme.colorScheme.primaryContainer,
          theme.colorScheme.primary,
          theme.colorScheme.primary,
        ),
      _TunnelStageState.blocked => (
          theme.colorScheme.errorContainer,
          theme.colorScheme.error,
          theme.colorScheme.error,
        ),
      _TunnelStageState.pending => (
          theme.colorScheme.surfaceContainerLowest,
          theme.colorScheme.outlineVariant,
          theme.colorScheme.onSurfaceVariant,
        ),
    };

    return AnimatedContainer(
      duration: const Duration(milliseconds: 220),
      curve: Curves.easeOutCubic,
      padding: const EdgeInsets.all(14),
      decoration: BoxDecoration(
        color: backgroundColor,
        borderRadius: BorderRadius.circular(18),
        border: Border.all(color: borderColor),
      ),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Container(
            width: 26,
            height: 26,
            alignment: Alignment.center,
            decoration: BoxDecoration(
              color: accentColor.withValues(alpha: 0.16),
              borderRadius: BorderRadius.circular(999),
            ),
            child: Text(
              '$index',
              style: theme.textTheme.labelLarge?.copyWith(
                color: accentColor,
                fontWeight: FontWeight.w800,
              ),
            ),
          ),
          const SizedBox(width: 12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  step.title,
                  style: theme.textTheme.bodyMedium?.copyWith(
                    fontWeight: FontWeight.w800,
                  ),
                ),
                const SizedBox(height: 4),
                Text(
                  step.description,
                  style: theme.textTheme.bodySmall?.copyWith(
                    color: theme.colorScheme.onSurfaceVariant,
                    height: 1.35,
                  ),
                ),
              ],
            ),
          ),
          const SizedBox(width: 12),
          Text(
            _stageStateLabel(step.state),
            style: theme.textTheme.labelMedium?.copyWith(
              color: accentColor,
              fontWeight: FontWeight.w800,
            ),
          ),
        ],
      ),
    );
  }
}

class _TunnelRecoveryGuide extends StatelessWidget {
  const _TunnelRecoveryGuide({
    required this.plan,
    required this.busy,
    required this.onPrimaryAction,
    required this.onSecondaryAction,
  });

  final _TunnelStagePlan plan;
  final bool busy;
  final Future<void> Function() onPrimaryAction;
  final Future<void> Function() onSecondaryAction;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          plan.recoveryTitle,
          style: theme.textTheme.bodyMedium?.copyWith(
            fontWeight: FontWeight.w800,
          ),
        ),
        const SizedBox(height: 12),
        for (var index = 0; index < plan.recoverySteps.length; index++) ...[
          Text(
            '${index + 1}. ${plan.recoverySteps[index]}',
            style: theme.textTheme.bodySmall?.copyWith(
              color: theme.colorScheme.onSurfaceVariant,
              height: 1.45,
            ),
          ),
          if (index != plan.recoverySteps.length - 1) const SizedBox(height: 8),
        ],
        if (plan.primaryRecoveryAction != null ||
            plan.secondaryRecoveryAction != null) ...[
          const SizedBox(height: 16),
          Wrap(
            spacing: 12,
            runSpacing: 12,
            children: [
              if (plan.primaryRecoveryAction != null)
                FilledButton(
                  onPressed: busy ? null : onPrimaryAction,
                  child: Text(plan.primaryRecoveryLabel!),
                ),
              if (plan.secondaryRecoveryAction != null)
                OutlinedButton(
                  onPressed: busy ? null : onSecondaryAction,
                  child: Text(plan.secondaryRecoveryLabel!),
                ),
            ],
          ),
        ],
      ],
    );
  }
}

