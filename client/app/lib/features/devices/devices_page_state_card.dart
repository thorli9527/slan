part of 'devices_page.dart';

class _DeviceStateCard extends StatelessWidget {
  const _DeviceStateCard({
    required this.device,
    required this.node,
    required this.bootstrap,
    required this.controlStatus,
    required this.relayTicket,
    required this.lastSendBytes,
    required this.lastSendFailure,
    required this.lastProbe,
    required this.lastProbeFailure,
    required this.connectionState,
    required this.tunnelRuntimeView,
    required this.tunnelDebugError,
    required this.error,
    required this.activeAction,
    required this.recentActions,
    required this.recentHealthSnapshots,
    required this.lastTunnelActionReport,
    required this.runtimeMonitorEnabled,
    required this.onBootstrap,
    required this.onApply,
    required this.onRecover,
    required this.onUp,
    required this.onInspect,
    required this.onDown,
    required this.busy,
  });

  final DeviceModel? device;
  final NodeModel? node;
  final BootstrapModel? bootstrap;
  final ControlStatusModel? controlStatus;
  final RelayTicketModel? relayTicket;
  final int? lastSendBytes;
  final SendFailure? lastSendFailure;
  final DataPlaneProbeModel? lastProbe;
  final ProbeFailure? lastProbeFailure;
  final ConnectionStateModel connectionState;
  final WireGuardTunnelRuntimeView? tunnelRuntimeView;
  final String? tunnelDebugError;
  final String? error;
  final _TunnelActionEvent? activeAction;
  final List<_TunnelActionEvent> recentActions;
  final List<_TunnelHealthSnapshot> recentHealthSnapshots;
  final TunnelActionReport? lastTunnelActionReport;
  final bool runtimeMonitorEnabled;
  final Future<void> Function() onBootstrap;
  final Future<void> Function() onApply;
  final Future<void> Function() onRecover;
  final Future<void> Function() onUp;
  final Future<void> Function() onInspect;
  final Future<void> Function() onDown;
  final bool busy;

  @override
  Widget build(BuildContext context) {
    final runtime = tunnelRuntimeView;
    final control = controlStatus;
    final theme = Theme.of(context);
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
    final latestAction =
        activeAction ?? (recentActions.isEmpty ? null : recentActions.first);

    return DesktopSurfaceCard(
      key: AppTestKeys.devicesStateCard,
      title: 'Runtime State',
      subtitle:
          'Control plane, relay fallback, send/probe, and PacketTunnel runtime snapshots are summarized here.',
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Wrap(
            spacing: 10,
            runSpacing: 10,
            children: [
              DesktopBadge(label: 'connection ${connectionState.status}'),
              DesktopBadge(label: 'path ${connectionState.path?.name ?? '-'}'),
              DesktopBadge(label: 'tunnel ${runtime?.state ?? 'idle'}'),
            ],
          ),
          const SizedBox(height: 16),
          _DevicesSubsection(
            title: 'Pipeline Overview',
            child: _TunnelStageOverview(
              plan: stagePlan,
              latestAction: latestAction,
              sessionHealth: healthSummary.health,
            ),
          ),
          const SizedBox(height: 16),
          _DevicesSubsection(
            title: 'Control Plane',
            child: _DeviceStateControlPlaneSection(
              device: device,
              node: node,
              control: control,
              connectionState: connectionState,
            ),
          ),
          const SizedBox(height: 16),
          _DevicesSubsection(
            title: 'Relay & Overlay',
            child: _DeviceStateRelayOverlaySection(
              relayTicket: relayTicket,
              bootstrap: bootstrap,
              control: control,
              runtime: runtime,
            ),
          ),
          const SizedBox(height: 16),
          _DevicesSubsection(
            title: 'Pipeline Status',
            child: _DeviceStatePipelineStatusSection(
              phaseGuidance: phaseGuidance,
              stagePlan: stagePlan,
              latestAction: latestAction,
              healthSummary: healthSummary,
              runtimeMonitorEnabled: runtimeMonitorEnabled,
              control: control,
            ),
          ),
          const SizedBox(height: 16),
          _DevicesSubsection(
            title: 'Health Guidance',
            child: _TunnelHealthSummaryPanel(
              summary: healthSummary,
              recentSnapshots: recentHealthSnapshots,
              busy: busy,
              onPrimaryAction: () => _runHealthAction(
                healthSummary.primaryAction,
                onBootstrap: onBootstrap,
                onRecover: onRecover,
              ),
              onSecondaryAction: () => _runHealthAction(
                healthSummary.secondaryAction,
                onBootstrap: onBootstrap,
                onRecover: onRecover,
              ),
            ),
          ),
          const SizedBox(height: 16),
          _DevicesSubsection(
            title: 'Tunnel Runtime',
            child: _TunnelRuntimeSummary(
              tunnelRuntimeView: runtime,
              tunnelDebugError: tunnelDebugError,
              error: error,
              dense: true,
            ),
          ),
          const SizedBox(height: 16),
          _DevicesSubsection(
            title: 'Traffic & Diagnostics',
            child: _DeviceStateTrafficDiagnosticsSection(
              lastSendBytes: lastSendBytes,
              lastSendFailure: lastSendFailure,
              lastProbe: lastProbe,
              lastProbeFailure: lastProbeFailure,
            ),
          ),
          const SizedBox(height: 16),
          _DevicesSubsection(
            title: 'Events & Errors',
            child: _DeviceStateEventsErrorsSection(
              runtime: runtime,
              tunnelDebugError: tunnelDebugError,
              error: error,
            ),
          ),
          if (error != null || runtime?.backendLastError != null) ...[
            const SizedBox(height: 12),
            Text(
              runtime?.backendLastError ?? error!,
              style: TextStyle(color: theme.colorScheme.error),
            ),
          ],
        ],
      ),
    );
  }

  Future<void> _runHealthAction(
    _TunnelActionKind? kind, {
    required Future<void> Function() onBootstrap,
    required Future<void> Function() onRecover,
  }) {
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
