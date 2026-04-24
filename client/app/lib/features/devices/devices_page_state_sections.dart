part of 'devices_page.dart';

class _DeviceStateControlPlaneSection extends StatelessWidget {
  const _DeviceStateControlPlaneSection({
    required this.device,
    required this.node,
    required this.control,
    required this.connectionState,
  });

  final DeviceModel? device;
  final NodeModel? node;
  final ControlStatusModel? control;
  final ConnectionStateModel connectionState;

  @override
  Widget build(BuildContext context) {
    return DesktopKeyValueList(
      entries: [
        DesktopKeyValueEntry(label: 'Device', value: device?.deviceId ?? 'none'),
        DesktopKeyValueEntry(label: 'Platform', value: device?.platform ?? '-'),
        DesktopKeyValueEntry(
          label: 'Virtual IP',
          value: device?.virtualIp ?? '-',
        ),
        DesktopKeyValueEntry(label: 'Node', value: node?.nodeId ?? 'none'),
        DesktopKeyValueEntry(
          label: 'Connection',
          value: connectionState.status,
        ),
        DesktopKeyValueEntry(label: 'Control WS', value: control?.status ?? 'none'),
        DesktopKeyValueEntry(
          label: 'Path',
          value: connectionState.path?.name ?? '-',
        ),
        DesktopKeyValueEntry(
          label: 'Session token',
          value: control?.sessionTokenPresent == true ? 'present' : '-',
        ),
        DesktopKeyValueEntry(
          label: 'Peers',
          value: control?.peerCount.toString() ?? '0',
        ),
      ],
    );
  }
}

class _DeviceStateInventorySection extends StatelessWidget {
  const _DeviceStateInventorySection({
    required this.device,
    required this.devices,
  });

  final DeviceModel? device;
  final List<DeviceModel> devices;

  @override
  Widget build(BuildContext context) {
    final pendingCount =
        devices.where((item) => item.membershipStatus == 'pending').length;
    final visibleDevices = devices.take(4).toList(growable: false);
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        DesktopKeyValueList(
          entries: [
            DesktopKeyValueEntry(
              label: 'Devices',
              value: devices.length.toString(),
            ),
            DesktopKeyValueEntry(
              label: 'Pending joins',
              value: pendingCount.toString(),
            ),
            DesktopKeyValueEntry(
              label: 'Membership',
              value: device?.membershipStatus ?? '-',
            ),
            DesktopKeyValueEntry(
              label: 'Role',
              value: device?.networkRole ?? '-',
            ),
            DesktopKeyValueEntry(
              label: 'Networks',
              value: device?.networkIds.join(', ') ?? '-',
            ),
          ],
        ),
        if (visibleDevices.isNotEmpty) ...[
          const SizedBox(height: 8),
          for (final item in visibleDevices)
            Padding(
              padding: const EdgeInsets.only(top: 4),
              child: Text(_deviceInventoryLine(item)),
            ),
        ],
      ],
    );
  }
}

class _DeviceStateRelayOverlaySection extends StatelessWidget {
  const _DeviceStateRelayOverlaySection({
    required this.relayTicket,
    required this.bootstrap,
    required this.control,
    required this.runtime,
  });

  final RelayTicketModel? relayTicket;
  final BootstrapModel? bootstrap;
  final ControlStatusModel? control;
  final WireGuardTunnelRuntimeView? runtime;

  @override
  Widget build(BuildContext context) {
    return DesktopKeyValueList(
      entries: [
        DesktopKeyValueEntry(
          label: 'Relay ticket',
          value: relayTicket?.ticketId ?? 'none',
        ),
        DesktopKeyValueEntry(
          label: 'Bootstrap',
          value: bootstrap?.relay.defaultClusterId ?? '-',
        ),
        DesktopKeyValueEntry(
          label: 'Networks',
          value: '${bootstrap?.networks.length ?? 0}',
        ),
        DesktopKeyValueEntry(
          label: 'Control plans',
          value: control?.connectPlanCount.toString() ?? '0',
        ),
        DesktopKeyValueEntry(
          label: 'Suggested path',
          value: preferredControlPathLabel(control),
        ),
        DesktopKeyValueEntry(
          label: 'Suggested relay',
          value: preferredControlRelayLabel(control),
        ),
        DesktopKeyValueEntry(
          label: 'Tunnel peer',
          value: runtime?.peerVirtualIp ?? '-',
        ),
        DesktopKeyValueEntry(
          label: 'Endpoint',
          value: runtime?.selectedEndpoint ?? '-',
        ),
      ],
    );
  }
}

String _deviceInventoryLine(DeviceModel device) {
  final status = device.membershipStatus?.isNotEmpty == true
      ? device.membershipStatus!
      : device.status;
  final role = device.networkRole?.isNotEmpty == true
      ? device.networkRole!
      : '-';
  final ip = device.virtualIp?.isNotEmpty == true ? device.virtualIp! : '-';
  return '${device.name} $status/$role $ip';
}

class _DeviceStatePipelineStatusSection extends StatelessWidget {
  const _DeviceStatePipelineStatusSection({
    required this.phaseGuidance,
    required this.stagePlan,
    required this.latestAction,
    required this.healthSummary,
    required this.runtimeMonitorEnabled,
    required this.control,
  });

  final _TunnelActionPhaseGuidance phaseGuidance;
  final _TunnelStagePlan stagePlan;
  final _TunnelActionEvent? latestAction;
  final _TunnelSessionHealthSummary healthSummary;
  final bool runtimeMonitorEnabled;
  final ControlStatusModel? control;

  @override
  Widget build(BuildContext context) {
    return DesktopKeyValueList(
      entries: [
        DesktopKeyValueEntry(
          label: 'Current phase',
          value: phaseGuidance.currentPhaseLabel ?? _fallbackCurrentPhaseTitle(),
        ),
        DesktopKeyValueEntry(
          label: 'Next step',
          value: phaseGuidance.nextStepLabel ?? stagePlan.nextStepLabel,
        ),
        DesktopKeyValueEntry(
          label: 'Signal source',
          value: phaseGuidance.signalSourceLabel ?? '-',
        ),
        DesktopKeyValueEntry(
          label: 'Last action',
          value: latestAction?.label ?? 'none',
        ),
        DesktopKeyValueEntry(
          label: 'Last outcome',
          value: latestAction == null
              ? 'none'
              : '${_statusLabel(latestAction!.status)} at ${_formatActionTime(latestAction!.occurredAt)}',
        ),
        DesktopKeyValueEntry(
          label: 'Session health',
          value: _sessionHealthLabel(healthSummary.health),
        ),
        DesktopKeyValueEntry(label: 'Why', value: healthSummary.reason),
        DesktopKeyValueEntry(
          label: 'Action',
          value: healthSummary.recommendedAction,
        ),
        DesktopKeyValueEntry(
          label: 'Data plane signal',
          value: healthSummary.dataPlaneSignal ?? '-',
        ),
        DesktopKeyValueEntry(
          label: 'Recommendation',
          value: healthSummary.recommendationMatch ?? '-',
        ),
        DesktopKeyValueEntry(
          label: 'Monitor',
          value: runtimeMonitorEnabled ? 'auto-refresh on' : 'manual',
        ),
        DesktopKeyValueEntry(
          label: 'Control sync',
          value: control?.networkMapPresent == true
              ? 'map active'
              : (control?.status ?? 'none'),
        ),
      ],
    );
  }

  String _fallbackCurrentPhaseTitle() {
    return stagePlan.steps
        .firstWhere(
          (step) => step.state == _TunnelStageState.current,
          orElse: () => stagePlan.steps.lastWhere(
            (step) => step.state == _TunnelStageState.complete,
            orElse: () => stagePlan.steps.first,
          ),
        )
        .title;
  }
}

class _DeviceStateTrafficDiagnosticsSection extends StatelessWidget {
  const _DeviceStateTrafficDiagnosticsSection({
    required this.lastSendBytes,
    required this.lastSendFailure,
    required this.lastProbe,
    required this.lastProbeFailure,
  });

  final int? lastSendBytes;
  final SendFailure? lastSendFailure;
  final DataPlaneProbeModel? lastProbe;
  final ProbeFailure? lastProbeFailure;

  @override
  Widget build(BuildContext context) {
    return DesktopKeyValueList(
      entries: [
        DesktopKeyValueEntry(
          label: 'Send bytes',
          value: lastSendBytes?.toString() ?? '-',
        ),
        DesktopKeyValueEntry(
          label: 'Send failure',
          value: lastSendFailure?.summary ?? '-',
        ),
        DesktopKeyValueEntry(
          label: 'Probe',
          value: lastProbe?.probeId ?? 'none',
        ),
        DesktopKeyValueEntry(
          label: 'Probe RTT',
          value: lastProbe?.replyRttMs?.toString() ?? '-',
        ),
        DesktopKeyValueEntry(
          label: 'Probe failure',
          value: lastProbeFailure?.summary ?? '-',
        ),
      ],
    );
  }
}

class _DeviceStateEventsErrorsSection extends StatelessWidget {
  const _DeviceStateEventsErrorsSection({
    required this.runtime,
    required this.tunnelDebugError,
    required this.error,
  });

  final WireGuardTunnelRuntimeView? runtime;
  final String? tunnelDebugError;
  final String? error;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return DefaultTextStyle(
      style: theme.textTheme.bodyMedium?.copyWith(
            color: theme.colorScheme.onSurfaceVariant,
            height: 1.45,
          ) ??
          const TextStyle(),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text('Backend error: ${runtime?.backendLastError ?? '-'}'),
          const SizedBox(height: 8),
          Text('Tunnel error: ${runtime?.lastError ?? tunnelDebugError ?? '-'}'),
          const SizedBox(height: 8),
          Text('Client error: ${error ?? '-'}'),
        ],
      ),
    );
  }
}
