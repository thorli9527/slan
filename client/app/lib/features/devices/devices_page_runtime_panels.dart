part of 'devices_page.dart';

class _TunnelRuntimeSummary extends StatelessWidget {
  const _TunnelRuntimeSummary({
    required this.tunnelRuntimeView,
    required this.tunnelDebugError,
    required this.error,
    this.dense = false,
  });

  final WireGuardTunnelRuntimeView? tunnelRuntimeView;
  final String? tunnelDebugError;
  final String? error;
  final bool dense;

  @override
  Widget build(BuildContext context) {
    final runtime = tunnelRuntimeView;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Wrap(
          spacing: 10,
          runSpacing: 10,
          children: [
            DesktopBadge(label: 'state ${runtime?.state ?? 'idle'}'),
            DesktopBadge(
              label: 'backend ${runtime?.backendState ?? 'unavailable'}',
            ),
            DesktopBadge(label: 'engine ${runtime?.debugEngineMode ?? '-'}'),
          ],
        ),
        const SizedBox(height: 14),
        DesktopKeyValueList(
          entries: [
            DesktopKeyValueEntry(
              label: 'Transport',
              value: runtime?.transport ?? '-',
            ),
            DesktopKeyValueEntry(label: 'Peer', value: runtime?.peerVirtualIp ?? '-'),
            DesktopKeyValueEntry(
              label: 'Endpoint',
              value: runtime?.selectedEndpoint ?? '-',
            ),
            DesktopKeyValueEntry(
              label: 'Interface',
              value: runtime?.interfaceName ?? '-',
            ),
            DesktopKeyValueEntry(
              label: 'Backend',
              value:
                  '${runtime?.backendName ?? '-'} / ${runtime?.backendState ?? '-'}',
            ),
            DesktopKeyValueEntry(
              label: 'RX',
              value:
                  '${runtime?.packetRxCount ?? 0} pkt / ${runtime?.packetRxBytes ?? 0} B',
            ),
            DesktopKeyValueEntry(
              label: 'TX',
              value:
                  '${runtime?.packetTxCount ?? 0} pkt / ${runtime?.packetTxBytes ?? 0} B',
            ),
            DesktopKeyValueEntry(
              label: 'Last packet',
              value: runtime?.lastPacketAtMs?.toString() ?? '-',
            ),
            DesktopKeyValueEntry(
              label: 'Last apply',
              value: runtime?.lastAppliedAtMs?.toString() ?? '-',
            ),
            if (!dense)
              DesktopKeyValueEntry(
                label: 'Last start',
                value: runtime?.backendLastStartedAtMs?.toString() ?? '-',
              ),
            if (!dense)
              DesktopKeyValueEntry(
                label: 'Peer IP',
                value: runtime?.backendPeerVirtualIp ?? '-',
              ),
            DesktopKeyValueEntry(
              label: 'Error',
              value: runtime?.backendLastError ??
                  runtime?.lastError ??
                  tunnelDebugError ??
                  error ??
                  '-',
            ),
          ],
        ),
      ],
    );
  }
}

class _TunnelOperationsDesk extends StatelessWidget {
  const _TunnelOperationsDesk({
    required this.tunnelRuntimeView,
    required this.controlStatus,
    required this.tunnelDebugError,
    required this.error,
    required this.busy,
    required this.connectionState,
    required this.peerVirtualIp,
    required this.activeAction,
    required this.recentActions,
    required this.recentHealthSnapshots,
    required this.lastTunnelActionReport,
    required this.lastProbe,
    required this.lastSendFailure,
    required this.lastProbeFailure,
    required this.onBootstrap,
    required this.onRecover,
    required this.onApply,
    required this.onUp,
    required this.onInspect,
    required this.onDown,
    required this.runtimeMonitorEnabled,
    required this.onToggleRuntimeMonitor,
  });

  final WireGuardTunnelRuntimeView? tunnelRuntimeView;
  final ControlStatusModel? controlStatus;
  final String? tunnelDebugError;
  final String? error;
  final bool busy;
  final ConnectionStateModel connectionState;
  final String peerVirtualIp;
  final _TunnelActionEvent? activeAction;
  final List<_TunnelActionEvent> recentActions;
  final List<_TunnelHealthSnapshot> recentHealthSnapshots;
  final TunnelActionReport? lastTunnelActionReport;
  final DataPlaneProbeModel? lastProbe;
  final SendFailure? lastSendFailure;
  final ProbeFailure? lastProbeFailure;
  final Future<void> Function() onBootstrap;
  final Future<void> Function() onRecover;
  final Future<void> Function() onApply;
  final Future<void> Function() onUp;
  final Future<void> Function() onInspect;
  final Future<void> Function() onDown;
  final bool runtimeMonitorEnabled;
  final ValueChanged<bool> onToggleRuntimeMonitor;

  @override
  Widget build(BuildContext context) {
    final runtime = tunnelRuntimeView;
    final stagePlan = _deriveTunnelStagePlan(
      runtime: runtime,
      controlStatus: controlStatus,
      activeAction: activeAction,
      recentActions: recentActions,
      tunnelDebugError: tunnelDebugError,
      error: error,
    );
    final healthSummary = _deriveTunnelSessionHealthSummary(
      runtime: runtime,
      controlStatus: controlStatus,
      connectionState: connectionState,
      lastProbe: lastProbe,
      tunnelDebugError: tunnelDebugError,
      error: error,
      runtimeMonitorEnabled: runtimeMonitorEnabled,
      recentActions: recentActions,
      recentHealthSnapshots: recentHealthSnapshots,
      lastSendFailure: lastSendFailure,
      lastProbeFailure: lastProbeFailure,
    );
    final phaseGuidance = _deriveTunnelActionPhaseGuidance(
      lastTunnelActionReport,
    );

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        if (activeAction != null) ...[
          _TunnelActionBanner(
            event: activeAction!,
            nextStepLabel: stagePlan.nextStepLabel,
          ),
          const SizedBox(height: 16),
        ],
        DesktopInsetBlock(
          title: 'Recommended Flow',
          child: _TunnelStageFlow(
            plan: stagePlan,
            busy: busy,
            onRunNext: () => _runStageAction(stagePlan.primaryAction),
          ),
        ),
        const SizedBox(height: 16),
        if (activeAction == null) const SizedBox(height: 16),
        _TunnelOperationsMetrics(
          runtime: runtime,
          runtimeMonitorEnabled: runtimeMonitorEnabled,
          healthSummary: healthSummary,
          phaseGuidance: phaseGuidance,
        ),
        const SizedBox(height: 16),
        DesktopInsetBlock(
          title: 'Health Guidance',
          child: _TunnelHealthSummaryPanel(
            summary: healthSummary,
            recentSnapshots: recentHealthSnapshots,
            busy: busy,
            onPrimaryAction: () => _runStageAction(healthSummary.primaryAction),
            onSecondaryAction: () =>
                _runStageAction(healthSummary.secondaryAction),
          ),
        ),
        const SizedBox(height: 16),
        DesktopInsetBlock(
          title: 'Live Runtime Monitor',
          child: _TunnelLiveRuntimeMonitorPanel(
            runtimeMonitorEnabled: runtimeMonitorEnabled,
            busy: busy,
            onToggleRuntimeMonitor: onToggleRuntimeMonitor,
          ),
        ),
        const SizedBox(height: 16),
        _TunnelConnectionFacts(
          runtime: runtime,
          connectionState: connectionState,
          peerVirtualIp: peerVirtualIp,
          tunnelDebugError: tunnelDebugError,
          error: error,
        ),
        const SizedBox(height: 16),
        DesktopInsetBlock(
          title: 'Recovery Guidance',
          child: _TunnelRecoveryGuide(
            plan: stagePlan,
            busy: busy,
            onPrimaryAction: () =>
                _runStageAction(stagePlan.primaryRecoveryAction),
            onSecondaryAction: () =>
                _runStageAction(stagePlan.secondaryRecoveryAction),
          ),
        ),
        const SizedBox(height: 16),
        DesktopInsetBlock(
          title: 'Recent Actions',
          child: _TunnelRecentActionsPanel(recentActions: recentActions),
        ),
      ],
    );
  }

  Future<void> _runStageAction(_TunnelActionKind? kind) {
    if (kind == null) {
      return Future<void>.value();
    }
    return switch (kind) {
      _TunnelActionKind.setup => onRecover(),
      _TunnelActionKind.apply => onApply(),
      _TunnelActionKind.bootstrap => onBootstrap(),
      _TunnelActionKind.recover => onRecover(),
      _TunnelActionKind.up => onUp(),
      _TunnelActionKind.inspect => onInspect(),
      _TunnelActionKind.down => onDown(),
      _TunnelActionKind.removePeer => onDown(),
    };
  }
}

class _TunnelOperationsMetrics extends StatelessWidget {
  const _TunnelOperationsMetrics({
    required this.runtime,
    required this.runtimeMonitorEnabled,
    required this.healthSummary,
    required this.phaseGuidance,
  });

  final WireGuardTunnelRuntimeView? runtime;
  final bool runtimeMonitorEnabled;
  final _TunnelSessionHealthSummary healthSummary;
  final _TunnelActionPhaseGuidance phaseGuidance;

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Wrap(
          spacing: 10,
          runSpacing: 10,
          children: [
            DesktopMetricPill(label: 'Runtime', value: runtime?.state ?? 'idle'),
            DesktopMetricPill(
              label: 'Backend',
              value: runtime?.backendState ?? 'unavailable',
            ),
            DesktopMetricPill(
              label: 'Health',
              value: _sessionHealthLabel(healthSummary.health),
              backgroundColor: _sessionHealthBackgroundColor(
                context,
                healthSummary.health,
              ),
              foregroundColor: _sessionHealthForegroundColor(
                context,
                healthSummary.health,
              ),
              borderColor: _sessionHealthBorderColor(
                context,
                healthSummary.health,
              ),
            ),
            DesktopMetricPill(label: 'Traffic', value: _trafficValue(runtime)),
            DesktopMetricPill(
              label: 'Monitor',
              value: runtimeMonitorEnabled ? 'live' : 'off',
            ),
            if (phaseGuidance.currentPhaseLabel != null)
              DesktopMetricPill(
                label: 'Phase',
                value: phaseGuidance.currentPhaseLabel!,
              ),
          ],
        ),
        if (phaseGuidance.currentPhaseLabel != null) ...[
          const SizedBox(height: 12),
          _TunnelPhaseGuidanceSummary(phaseGuidance: phaseGuidance),
        ],
      ],
    );
  }
}

class _TunnelPhaseGuidanceSummary extends StatelessWidget {
  const _TunnelPhaseGuidanceSummary({required this.phaseGuidance});

  final _TunnelActionPhaseGuidance phaseGuidance;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          'Native phase: ${phaseGuidance.currentPhaseLabel!}${phaseGuidance.signalSourceLabel == null ? '' : ' via ${phaseGuidance.signalSourceLabel!}'}',
          style: theme.textTheme.bodySmall?.copyWith(
            color: theme.colorScheme.onSurfaceVariant,
            fontWeight: FontWeight.w700,
          ),
        ),
        if (phaseGuidance.nextStepLabel != null) ...[
          const SizedBox(height: 6),
          Text(
            'Phase-driven next step: ${phaseGuidance.nextStepLabel!}',
            style: theme.textTheme.bodySmall?.copyWith(
              color: theme.colorScheme.onSurfaceVariant,
              height: 1.4,
            ),
          ),
        ],
      ],
    );
  }
}

class _TunnelLiveRuntimeMonitorPanel extends StatelessWidget {
  const _TunnelLiveRuntimeMonitorPanel({
    required this.runtimeMonitorEnabled,
    required this.busy,
    required this.onToggleRuntimeMonitor,
  });

  final bool runtimeMonitorEnabled;
  final bool busy;
  final ValueChanged<bool> onToggleRuntimeMonitor;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        SwitchListTile(
          contentPadding: EdgeInsets.zero,
          title: const Text('Auto-refresh runtime every 4 seconds'),
          subtitle: Text(
            runtimeMonitorEnabled
                ? 'The workbench will keep polling runtime while the page is open.'
                : 'Leave this off to inspect runtime manually after each step.',
          ),
          value: runtimeMonitorEnabled,
          onChanged: busy ? null : onToggleRuntimeMonitor,
        ),
        const SizedBox(height: 8),
        Text(
          runtimeMonitorEnabled
              ? 'Live monitor is active. Runtime cards will follow backend and traffic changes automatically.'
              : 'Manual mode is active. Use View Runtime or the recommended flow button to refresh state.',
          style: theme.textTheme.bodySmall?.copyWith(
            color: theme.colorScheme.onSurfaceVariant,
            height: 1.45,
          ),
        ),
      ],
    );
  }
}

class _TunnelConnectionFacts extends StatelessWidget {
  const _TunnelConnectionFacts({
    required this.runtime,
    required this.connectionState,
    required this.peerVirtualIp,
    required this.tunnelDebugError,
    required this.error,
  });

  final WireGuardTunnelRuntimeView? runtime;
  final ConnectionStateModel connectionState;
  final String peerVirtualIp;
  final String? tunnelDebugError;
  final String? error;

  @override
  Widget build(BuildContext context) {
    return DesktopKeyValueList(
      entries: [
        DesktopKeyValueEntry(
          label: 'Connection',
          value: connectionState.status,
        ),
        DesktopKeyValueEntry(
          label: 'Peer',
          value: runtime?.peerVirtualIp ?? peerVirtualIp.ifEmpty('-'),
        ),
        DesktopKeyValueEntry(
          label: 'Engine',
          value: runtime?.debugEngineMode ?? '-',
        ),
        DesktopKeyValueEntry(
          label: 'Backend',
          value: runtime?.backendName ?? '-',
        ),
        DesktopKeyValueEntry(
          label: 'Last packet',
          value: runtime?.lastPacketAtMs?.toString() ?? '-',
        ),
        DesktopKeyValueEntry(
          label: 'Error',
          value: runtime?.backendLastError ??
              runtime?.lastError ??
              tunnelDebugError ??
              error ??
              '-',
        ),
      ],
    );
  }
}

class _TunnelRecentActionsPanel extends StatelessWidget {
  const _TunnelRecentActionsPanel({required this.recentActions});

  final List<_TunnelActionEvent> recentActions;

  @override
  Widget build(BuildContext context) {
    if (recentActions.isEmpty) {
      return const Text('No tunnel actions yet.');
    }

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        for (var index = 0; index < recentActions.length; index++) ...[
          _TunnelActionTimelineRow(event: recentActions[index]),
          if (index != recentActions.length - 1) const SizedBox(height: 12),
        ],
      ],
    );
  }
}
