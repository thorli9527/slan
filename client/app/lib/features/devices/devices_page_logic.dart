part of 'devices_page.dart';

String _trafficValue(WireGuardTunnelRuntimeView? runtime) {
  if (runtime == null) {
    return '0 / 0';
  }
  return '${runtime.packetRxCount}rx · ${runtime.packetTxCount}tx';
}

_TunnelSessionHealthSummary _deriveTunnelSessionHealthSummary({
  required WireGuardTunnelRuntimeView? runtime,
  required ControlStatusModel? controlStatus,
  required ConnectionStateModel connectionState,
  required DataPlaneProbeModel? lastProbe,
  required String? tunnelDebugError,
  required String? error,
  required bool runtimeMonitorEnabled,
  required List<_TunnelActionEvent> recentActions,
  required List<_TunnelHealthSnapshot> recentHealthSnapshots,
  required SendFailure? lastSendFailure,
  required ProbeFailure? lastProbeFailure,
}) {
  final now = DateTime.now();
  final hasError = (runtime?.backendLastError?.isNotEmpty ?? false) ||
      (runtime?.lastError?.isNotEmpty ?? false) ||
      (tunnelDebugError?.isNotEmpty ?? false) ||
      (error?.isNotEmpty ?? false);
  final state = runtime?.state ?? 'idle';
  final backendState = runtime?.backendState ?? 'unavailable';
  final backendLooksFailed = backendState.toLowerCase().contains('fail');
  final hasEndpoint = (runtime?.selectedEndpoint?.isNotEmpty ?? false);
  final hasTraffic = (runtime?.packetRxCount ?? 0) > 0 ||
      (runtime?.packetTxCount ?? 0) > 0 ||
      (runtime?.lastPacketAtMs ?? 0) > 0;
  final hasApplied = (runtime?.lastAppliedAtMs ?? 0) > 0;
  final hasStarted = (runtime?.backendLastStartedAtMs ?? 0) > 0;
  final applyAge = _timestampAgeLabel(runtime?.lastAppliedAtMs, now);
  final startedAge = _timestampAgeLabel(runtime?.backendLastStartedAtMs, now);
  final packetAge = _timestampAgeLabel(runtime?.lastPacketAtMs, now);
  final recentApplyFailures = _countRecentFailures(
    recentActions,
    _TunnelActionKind.apply,
  );
  final recentUpFailures = _countRecentFailures(
    recentActions,
    _TunnelActionKind.up,
  );
  final trendSummary = _deriveTunnelHealthTrendSummary(
    recentHealthSnapshots,
    now,
  );
  final relayFailure = _latestRelayFailureSignal(
    lastProbeFailure: lastProbeFailure,
    lastSendFailure: lastSendFailure,
  );
  final isPersistentUnhealthy = trendSummary.label == 'sustained degradation' ||
      trendSummary.label == 'degraded again after recovery';
  final isBriefFlap = trendSummary.label == 'short flap';
  final hasControlSession = controlStatus?.sessionTokenPresent == true;
  final controlMapPending =
      hasControlSession && controlStatus?.networkMapPresent != true;
  final preferredControlPath = preferredControlPathLabel(controlStatus);
  final preferredControlRelay = preferredControlRelayLabel(controlStatus);
  final hasPreferredControlPath = preferredControlPath != '-';
  final recommendationMatch = connectRecommendationMatchLabel(
    controlPlan: controlStatus?.connectPlans.isEmpty == false
        ? controlStatus!.connectPlans.first
        : null,
    connectionState: connectionState,
    lastProbe: lastProbe,
  );
  final recommendationSignal = recommendationMatch == 'none'
      ? null
      : 'Control recommendation is currently $recommendationMatch.';

  if (controlMapPending && relayFailure == null && !hasError) {
    return _TunnelSessionHealthSummary(
      health: _TunnelSessionHealth.degraded,
      reason:
          'Control session is configured, but the latest control-plane sync has not produced a usable network map yet.',
      supportingSignal: controlStatus?.wsUrl == null
          ? 'A control session token exists, but no synced peer/runtime view is available yet.'
          : 'Control WS is configured at ${controlStatus!.wsUrl}, but peer state has not been refreshed yet.',
      recommendedAction:
          'Refresh bootstrap or run control sync first so the latest peer map and path recommendation reach the client before retrying tunnel or relay recovery.',
      primaryAction: _TunnelActionKind.bootstrap,
      primaryActionLabel: 'Refresh Bootstrap',
      secondaryAction: _TunnelActionKind.inspect,
      secondaryActionLabel: 'View Runtime',
      trendLabel: trendSummary.label,
      trendSignal: trendSummary.signal,
      dataPlaneSignal: relayFailure?.label,
      recommendationMatch: recommendationSignal,
    );
  }

  if (relayFailure != null) {
    switch (relayFailure.kind) {
      case _RelayDataPlaneFailureKind.auth:
        return _TunnelSessionHealthSummary(
          health: _TunnelSessionHealth.failed,
          reason:
              'Relay authentication failed, so the current relay path can no longer be trusted.',
          supportingSignal:
              'Latest data-plane failure: ${relayFailure.summary}',
          recommendedAction:
              'Refresh bootstrap and relay credentials first, then inspect runtime before retrying relay traffic.',
          primaryAction: _TunnelActionKind.bootstrap,
          primaryActionLabel: 'Refresh Bootstrap',
          secondaryAction: _TunnelActionKind.inspect,
          secondaryActionLabel: 'View Runtime',
          trendLabel: trendSummary.label,
          trendSignal: trendSummary.signal,
          dataPlaneSignal: relayFailure.label,
          recommendationMatch: recommendationSignal,
        );
      case _RelayDataPlaneFailureKind.session:
        return _TunnelSessionHealthSummary(
          health: isPersistentUnhealthy
              ? _TunnelSessionHealth.failed
              : _TunnelSessionHealth.degraded,
          reason:
              'Relay session state is stale or detached, so the transport should be re-established.',
          supportingSignal:
              'Latest data-plane failure: ${relayFailure.summary}',
          recommendedAction: isPersistentUnhealthy
              ? 'Recent checks show the relay session is not recovering. Run a full session recovery flow.'
              : 'Re-establish the relay-backed session by bringing the tunnel down and up again.',
          primaryAction: isPersistentUnhealthy
              ? _TunnelActionKind.recover
              : _TunnelActionKind.down,
          primaryActionLabel:
              isPersistentUnhealthy ? 'Recover Session' : 'Bring Tunnel Down',
          secondaryAction: isPersistentUnhealthy
              ? _TunnelActionKind.inspect
              : _TunnelActionKind.up,
          secondaryActionLabel:
              isPersistentUnhealthy ? 'View Runtime' : 'Bring Tunnel Up',
          trendLabel: trendSummary.label,
          trendSignal: trendSummary.signal,
          dataPlaneSignal: relayFailure.label,
          recommendationMatch: recommendationSignal,
        );
      case _RelayDataPlaneFailureKind.protocol:
        return _TunnelSessionHealthSummary(
          health: _TunnelSessionHealth.degraded,
          reason:
              'Relay protocol handling failed, so the path should be inspected before trusting current traffic.',
          supportingSignal:
              'Latest data-plane failure: ${relayFailure.summary}',
          recommendedAction: isPersistentUnhealthy
              ? 'Protocol errors are repeating. Run a full recovery cycle after inspecting runtime.'
              : 'Inspect runtime and relay state first, then recover the session if protocol errors continue.',
          primaryAction: _TunnelActionKind.inspect,
          primaryActionLabel: 'View Runtime',
          secondaryAction: isPersistentUnhealthy
              ? _TunnelActionKind.recover
              : _TunnelActionKind.apply,
          secondaryActionLabel:
              isPersistentUnhealthy ? 'Recover Session' : 'Apply Tunnel',
          trendLabel: trendSummary.label,
          trendSignal: trendSummary.signal,
          dataPlaneSignal: relayFailure.label,
          recommendationMatch: recommendationSignal,
        );
    }
  }

  if (hasError || backendLooksFailed) {
    return _TunnelSessionHealthSummary(
      health: _TunnelSessionHealth.failed,
      reason: backendLooksFailed
          ? 'The tunnel backend reports a failed state.'
          : 'A runtime or client-side error is blocking the tunnel session.',
      supportingSignal: runtime?.backendLastError ??
          runtime?.lastError ??
          tunnelDebugError ??
          error ??
          'Unknown failure signal.',
      recommendedAction: (recentApplyFailures + recentUpFailures) >= 2 ||
              isPersistentUnhealthy
          ? 'Re-apply configuration and bring the session up again.'
          : 'Inspect runtime first, then re-apply configuration if the backend still looks failed.',
      primaryAction:
          (recentApplyFailures + recentUpFailures) >= 2 || isPersistentUnhealthy
              ? _TunnelActionKind.recover
              : _TunnelActionKind.apply,
      primaryActionLabel:
          (recentApplyFailures + recentUpFailures) >= 2 || isPersistentUnhealthy
              ? 'Recover Session'
              : 'Apply Tunnel Again',
      secondaryAction: _runtimeMonitorEnabledForSummary(
              runtimeMonitorEnabled: runtimeMonitorEnabled)
          ? _TunnelActionKind.inspect
          : _TunnelActionKind.up,
      secondaryActionLabel: _runtimeMonitorEnabledForSummary(
              runtimeMonitorEnabled: runtimeMonitorEnabled)
          ? 'View Runtime'
          : 'Bring Tunnel Up',
      trendLabel: trendSummary.label,
      trendSignal: trendSummary.signal,
      recommendationMatch: recommendationSignal,
    );
  }

  if (!hasApplied) {
    return _TunnelSessionHealthSummary(
      health: _TunnelSessionHealth.idle,
      reason:
          'No tunnel configuration has been applied yet, so there is no active session to evaluate.',
      supportingSignal:
          'The runtime is still idle and no apply timestamp is present.',
      recommendedAction:
          'Apply configuration first, then bring the tunnel up and inspect runtime.',
      primaryAction: _TunnelActionKind.apply,
      primaryActionLabel: 'Apply Tunnel',
      secondaryAction: hasControlSession
          ? _TunnelActionKind.bootstrap
          : _TunnelActionKind.up,
      secondaryActionLabel:
          hasControlSession ? 'Refresh Bootstrap' : 'Bring Tunnel Up',
      trendLabel: trendSummary.label,
      trendSignal: trendSummary.signal,
      recommendationMatch: recommendationSignal,
    );
  }

  final runtimeLooksUp = state == 'up' || state == 'running';
  if (!runtimeLooksUp || !hasStarted) {
    return _TunnelSessionHealthSummary(
      health: _TunnelSessionHealth.degraded,
      reason:
          'Configuration has been staged, but the backend does not look fully started yet.',
      supportingSignal: startedAge == null
          ? 'The runtime shows state=$state backend=$backendState without a verified backend start timestamp.'
          : 'The backend last started about $startedAge ago, but runtime is still state=$state backend=$backendState.',
      recommendedAction: _runtimeMonitorEnabledForSummary(
              runtimeMonitorEnabled: runtimeMonitorEnabled)
          ? 'Watch runtime and refresh if startup does not complete.'
          : 'Bring the tunnel up again, then inspect runtime.',
      primaryAction: _runtimeMonitorEnabledForSummary(
              runtimeMonitorEnabled: runtimeMonitorEnabled)
          ? _TunnelActionKind.inspect
          : _TunnelActionKind.up,
      primaryActionLabel: _runtimeMonitorEnabledForSummary(
              runtimeMonitorEnabled: runtimeMonitorEnabled)
          ? 'View Runtime'
          : 'Bring Tunnel Up',
      secondaryAction: _TunnelActionKind.apply,
      secondaryActionLabel: 'Apply Tunnel Again',
      trendLabel: trendSummary.label,
      trendSignal: trendSummary.signal,
      recommendationMatch: recommendationSignal,
    );
  }

  if (!hasEndpoint) {
    return _TunnelSessionHealthSummary(
      health: _TunnelSessionHealth.degraded,
      reason:
          'The session is running, but runtime has not selected an endpoint yet.',
      supportingSignal: startedAge == null
          ? 'Backend is up, but endpoint selection is still empty.'
          : 'Backend has been up for about $startedAge, but endpoint selection is still empty.',
      recommendedAction: _runtimeMonitorEnabledForSummary(
              runtimeMonitorEnabled: runtimeMonitorEnabled)
          ? 'Let live monitor continue observing runtime or refresh manually until endpoint selection appears.'
          : 'Refresh runtime now to confirm whether endpoint selection is still missing.',
      primaryAction: _TunnelActionKind.inspect,
      primaryActionLabel: 'View Runtime',
      secondaryAction: _runtimeMonitorEnabledForSummary(
              runtimeMonitorEnabled: runtimeMonitorEnabled)
          ? null
          : _TunnelActionKind.recover,
      secondaryActionLabel: _runtimeMonitorEnabledForSummary(
              runtimeMonitorEnabled: runtimeMonitorEnabled)
          ? null
          : 'Recover Session',
      trendLabel: trendSummary.label,
      trendSignal: trendSummary.signal,
      recommendationMatch: recommendationSignal,
    );
  }

  if (!hasTraffic) {
    return _TunnelSessionHealthSummary(
      health: _TunnelSessionHealth.degraded,
      reason:
          'The session is up and an endpoint is selected, but no traffic has been observed yet.',
      supportingSignal: packetAge == null
          ? 'RX/TX counters are still at zero after apply=${applyAge ?? '-'} start=${startedAge ?? '-'}.'
          : 'The last observed packet was about $packetAge ago.',
      recommendedAction: isBriefFlap
          ? 'A short traffic flap just recovered. Keep observing runtime before forcing a reconfiguration.'
          : _runtimeMonitorEnabledForSummary(
                  runtimeMonitorEnabled: runtimeMonitorEnabled)
              ? 'Keep the monitor running or refresh runtime to see whether traffic becomes active.'
              : 'Inspect runtime first. Reconfigure only if traffic stays idle for multiple checks.',
      primaryAction: _TunnelActionKind.inspect,
      primaryActionLabel: 'View Runtime',
      secondaryAction: isPersistentUnhealthy ? _TunnelActionKind.recover : null,
      secondaryActionLabel: isPersistentUnhealthy ? 'Recover Session' : null,
      trendLabel: trendSummary.label,
      trendSignal: trendSummary.signal,
      recommendationMatch: recommendationSignal,
    );
  }

  return _TunnelSessionHealthSummary(
    health: _TunnelSessionHealth.healthy,
    reason:
        'The tunnel backend is running, an endpoint is selected, and traffic has been observed.',
    supportingSignal: packetAge == null
        ? 'Backend=$backendState state=$state traffic=${_trafficValue(runtime)}.'
        : 'Traffic last moved about $packetAge ago with endpoint ${runtime?.selectedEndpoint}.',
    recommendedAction: hasPreferredControlPath
        ? 'The session looks healthy. Keep observing runtime while control plane currently prefers $preferredControlPath${preferredControlRelay == '-' ? '' : ' via $preferredControlRelay'}.'
        : 'The session looks healthy. Continue monitoring or bring it down when you are done.',
    primaryAction: _TunnelActionKind.inspect,
    primaryActionLabel: 'View Runtime',
    secondaryAction: _TunnelActionKind.down,
    secondaryActionLabel: 'Bring Tunnel Down',
    trendLabel: trendSummary.label,
    trendSignal: trendSummary.signal,
    recommendationMatch: recommendationSignal,
  );
}

bool _runtimeMonitorEnabledForSummary({required bool runtimeMonitorEnabled}) {
  return runtimeMonitorEnabled;
}

_TunnelHealthTrendSummary _deriveTunnelHealthTrendSummary(
  List<_TunnelHealthSnapshot> snapshots,
  DateTime now,
) {
  if (snapshots.isEmpty) {
    return const _TunnelHealthTrendSummary(
      label: 'no history yet',
      signal:
          'Trend memory will appear after a few runtime checks or lifecycle actions.',
    );
  }

  final recent = snapshots.take(4).toList(growable: false);
  final latest = recent.first;
  final degradedCount = recent
      .where(
        (snapshot) =>
            snapshot.health == _TunnelSessionHealth.degraded ||
            snapshot.health == _TunnelSessionHealth.failed,
      )
      .length;
  final healthyCount = recent
      .where((snapshot) => snapshot.health == _TunnelSessionHealth.healthy)
      .length;

  if (recent.length >= 3 &&
      (latest.health == _TunnelSessionHealth.degraded ||
          latest.health == _TunnelSessionHealth.failed) &&
      recent[1].health == _TunnelSessionHealth.healthy &&
      (recent[2].health == _TunnelSessionHealth.degraded ||
          recent[2].health == _TunnelSessionHealth.failed)) {
    final age = _timestampAgeLabel(
      latest.occurredAt.millisecondsSinceEpoch,
      now,
    );
    return _TunnelHealthTrendSummary(
      label: 'degraded again after recovery',
      signal: age == null
          ? 'The tunnel recovered briefly, then fell back into an unhealthy state.'
          : 'The tunnel recovered briefly, then degraded again about $age ago.',
    );
  }

  if (recent.length >= 3 && degradedCount == recent.length) {
    final age = _timestampAgeLabel(
      recent.last.occurredAt.millisecondsSinceEpoch,
      now,
    );
    return _TunnelHealthTrendSummary(
      label: 'sustained degradation',
      signal: age == null
          ? 'Recent health samples have stayed degraded without a healthy interval.'
          : 'Recent health samples have stayed degraded for about $age.',
    );
  }

  if (recent.length >= 3 &&
      latest.health == _TunnelSessionHealth.healthy &&
      degradedCount >= 1 &&
      healthyCount >= 2) {
    final age = _timestampAgeLabel(
      recent[1].occurredAt.millisecondsSinceEpoch,
      now,
    );
    return _TunnelHealthTrendSummary(
      label: 'short flap',
      signal: age == null
          ? 'A recent degraded sample recovered quickly and now looks stable again.'
          : 'A recent degraded sample recovered quickly after appearing about $age ago.',
    );
  }

  final age = _timestampAgeLabel(
    latest.occurredAt.millisecondsSinceEpoch,
    now,
  );
  return _TunnelHealthTrendSummary(
    label: 'steady',
    signal: age == null
        ? 'Recent health samples have stayed broadly steady enough that no stronger trend stands out yet.'
        : 'Recent health samples have stayed broadly steady through the last $age.',
  );
}

extension on String {
  String ifEmpty(String fallback) => trim().isEmpty ? fallback : this;
}

_RelayDataPlaneFailureSignal? _latestRelayFailureSignal({
  required ProbeFailure? lastProbeFailure,
  required SendFailure? lastSendFailure,
}) {
  if (lastProbeFailure != null) {
    switch (lastProbeFailure.kind) {
      case ProbeFailureKind.relayAuth:
        return _RelayDataPlaneFailureSignal(
          kind: _RelayDataPlaneFailureKind.auth,
          label: lastProbeFailure.label,
          summary: lastProbeFailure.summary,
        );
      case ProbeFailureKind.relaySession:
        return _RelayDataPlaneFailureSignal(
          kind: _RelayDataPlaneFailureKind.session,
          label: lastProbeFailure.label,
          summary: lastProbeFailure.summary,
        );
      case ProbeFailureKind.relayProtocol:
        return _RelayDataPlaneFailureSignal(
          kind: _RelayDataPlaneFailureKind.protocol,
          label: lastProbeFailure.label,
          summary: lastProbeFailure.summary,
        );
      case ProbeFailureKind.timeout:
      case ProbeFailureKind.transport:
      case ProbeFailureKind.unsupported:
      case ProbeFailureKind.unknown:
        break;
    }
  }
  if (lastSendFailure != null) {
    switch (lastSendFailure.kind) {
      case SendFailureKind.relayAuth:
        return _RelayDataPlaneFailureSignal(
          kind: _RelayDataPlaneFailureKind.auth,
          label: lastSendFailure.label,
          summary: lastSendFailure.summary,
        );
      case SendFailureKind.relaySession:
        return _RelayDataPlaneFailureSignal(
          kind: _RelayDataPlaneFailureKind.session,
          label: lastSendFailure.label,
          summary: lastSendFailure.summary,
        );
      case SendFailureKind.relayProtocol:
        return _RelayDataPlaneFailureSignal(
          kind: _RelayDataPlaneFailureKind.protocol,
          label: lastSendFailure.label,
          summary: lastSendFailure.summary,
        );
      case SendFailureKind.timeout:
      case SendFailureKind.transport:
      case SendFailureKind.unsupported:
      case SendFailureKind.unknown:
        break;
    }
  }
  return null;
}

String _statusLabel(_TunnelActionStatus status) => switch (status) {
      _TunnelActionStatus.running => 'Running',
      _TunnelActionStatus.succeeded => 'Completed',
      _TunnelActionStatus.failed => 'Failed',
    };

String _sessionHealthLabel(_TunnelSessionHealth health) => switch (health) {
      _TunnelSessionHealth.healthy => 'healthy',
      _TunnelSessionHealth.degraded => 'degraded',
      _TunnelSessionHealth.failed => 'failed',
      _TunnelSessionHealth.idle => 'idle',
    };

String _tunnelActionPhaseLabel(TunnelActionPhase phase) => switch (phase) {
      TunnelActionPhase.accepted => 'accepted',
      TunnelActionPhase.configured => 'configured',
      TunnelActionPhase.started => 'started',
      TunnelActionPhase.verified => 'verified',
      TunnelActionPhase.failed => 'failed',
      TunnelActionPhase.pendingVerification => 'pending verify',
    };

String _stageStateLabel(_TunnelStageState state) => switch (state) {
      _TunnelStageState.complete => 'done',
      _TunnelStageState.current => 'now',
      _TunnelStageState.pending => 'next',
      _TunnelStageState.blocked => 'blocked',
    };

String _formatActionTime(DateTime value) {
  final hour = value.hour.toString().padLeft(2, '0');
  final minute = value.minute.toString().padLeft(2, '0');
  final second = value.second.toString().padLeft(2, '0');
  return '$hour:$minute:$second';
}

DateTime? _latestSuccessfulActionAt(
  List<_TunnelActionEvent> actions,
  _TunnelActionKind kind,
) {
  for (final event in actions) {
    if (event.kind == kind && event.status == _TunnelActionStatus.succeeded) {
      return event.occurredAt;
    }
  }
  return null;
}

int _countRecentFailures(
  List<_TunnelActionEvent> actions,
  _TunnelActionKind kind,
) {
  var count = 0;
  for (final event in actions.take(6)) {
    if (event.kind == kind && event.status == _TunnelActionStatus.failed) {
      count += 1;
    }
  }
  return count;
}

String? _timestampAgeLabel(int? timestampMs, DateTime now) {
  if (timestampMs == null || timestampMs <= 0) {
    return null;
  }
  final timestamp = DateTime.fromMillisecondsSinceEpoch(timestampMs);
  final diff = now.difference(timestamp);
  if (diff.isNegative) {
    return null;
  }
  if (diff.inSeconds < 60) {
    return '${diff.inSeconds}s';
  }
  if (diff.inMinutes < 60) {
    return '${diff.inMinutes}m';
  }
  return '${diff.inHours}h';
}

_TunnelStagePlan _deriveTunnelStagePlan({
  required WireGuardTunnelRuntimeView? runtime,
  required ControlStatusModel? controlStatus,
  required _TunnelActionEvent? activeAction,
  required List<_TunnelActionEvent> recentActions,
  required String? tunnelDebugError,
  required String? error,
}) {
  final latest =
      activeAction ?? (recentActions.isEmpty ? null : recentActions.first);
  final latestError =
      latest?.status == _TunnelActionStatus.failed ? latest!.detail : null;
  final effectiveError = latestError ??
      runtime?.backendLastError ??
      runtime?.lastError ??
      tunnelDebugError ??
      error;

  final latestAppliedAt = _latestSuccessfulActionAt(
    recentActions,
    _TunnelActionKind.apply,
  );
  final latestUpAt = _latestSuccessfulActionAt(
    recentActions,
    _TunnelActionKind.up,
  );
  final applyNeedsBringUp = latestAppliedAt != null &&
      (latestUpAt == null || latestAppliedAt.isAfter(latestUpAt));

  final hasApplied = runtime?.lastAppliedAtMs != null ||
      recentActions.any((event) =>
          event.kind == _TunnelActionKind.apply &&
          event.status == _TunnelActionStatus.succeeded);
  final runtimeLooksUp = runtime?.state == 'up' || runtime?.state == 'running';
  final isUp = runtimeLooksUp && !applyNeedsBringUp;
  final hasRecentRuntimeObservation = runtime != null &&
      ((runtime.lastPacketAtMs ?? 0) > 0 ||
          (runtime.backendLastStartedAtMs ?? 0) > 0 ||
          (runtime.packetRxCount > 0) ||
          (runtime.packetTxCount > 0));
  final hasInspection = hasRecentRuntimeObservation ||
      recentActions.any((event) =>
          event.kind == _TunnelActionKind.inspect &&
          event.status == _TunnelActionStatus.succeeded);
  final blocked = effectiveError != null;
  final hasControlSession = controlStatus?.sessionTokenPresent == true;
  final controlMapPending =
      hasControlSession && controlStatus?.networkMapPresent != true;
  final preferredControlPath = preferredControlPathLabel(controlStatus);
  final preferredControlRelay = preferredControlRelayLabel(controlStatus);
  final hasPreferredControlPath = preferredControlPath != '-';

  final nextStepLabel = blocked
      ? 'Fix the failure, then re-apply configuration'
      : controlMapPending
          ? 'Refresh control sync'
          : !hasApplied
              ? 'Apply configuration'
              : !isUp
                  ? 'Bring the tunnel up'
                  : !hasInspection
                      ? 'Refresh runtime'
                      : hasPreferredControlPath
                          ? 'Observe runtime or follow control recommendation $preferredControlPath'
                          : 'Observe runtime or bring the tunnel down';

  final summary = blocked
      ? 'The latest tunnel action failed. Recover configuration or backend state before moving forward.'
      : controlMapPending
          ? 'Control plane is configured but has not synced a fresh network map yet. Refresh bootstrap/control sync before trusting local path decisions.'
          : !hasApplied
              ? 'Start the recommended sequence by applying configuration so the host has a staged interface and peer definition.'
              : applyNeedsBringUp
                  ? 'A newer configuration was applied after the last successful bring-up. Bring the tunnel up again so runtime reflects the staged settings.'
                  : !isUp
                      ? 'Configuration is staged. The next meaningful step is to bring the PacketTunnel session up.'
                      : !hasInspection
                          ? hasPreferredControlPath
                              ? 'The tunnel looks active. Refresh runtime now, or let live monitor observe backend state while control plane recommends $preferredControlPath${preferredControlRelay == '-' ? '' : ' via $preferredControlRelay'}.'
                              : 'The tunnel looks active. Refresh runtime now, or let live monitor observe backend state and traffic changes for you.'
                          : hasPreferredControlPath
                              ? 'The recommended flow has completed. Control plane currently recommends $preferredControlPath${preferredControlRelay == '-' ? '' : ' via $preferredControlRelay'}.'
                              : 'The recommended flow has completed. You can keep observing runtime, re-apply configuration, or shut the tunnel down cleanly.';

  final recoveryTitle = effectiveError == null
      ? 'If the next step fails'
      : 'Recovery path for the current failure';
  final recoverySteps = effectiveError == null
      ? <String>[
          if (controlMapPending)
            'Refresh bootstrap/control sync first so the latest peer map and path recommendation are present before tunnel actions.',
          'Verify local IP, peer IP, endpoint, and debug engine mode before pressing Apply Tunnel.',
          'If Bring Up does not move the tunnel out of idle, run View Runtime immediately or enable live monitor to capture backend state and activity.',
          if (hasPreferredControlPath)
            'Control plane currently suggests $preferredControlPath${preferredControlRelay == '-' ? '' : ' via $preferredControlRelay'}. Use that as the expected path while validating runtime.',
          'If runtime still looks stale, bring the tunnel down, apply configuration again, then repeat the flow.',
        ]
      : <String>[
          'Review the latest failure first: $effectiveError',
          if (controlMapPending)
            'Refresh bootstrap/control sync before retrying so local runtime and control-plane state are aligned.',
          'Re-check endpoint, peer addressing, and debug engine mode. Use noop, loopback, or external only when you intend to test those paths.',
          'Repeat the recommended sequence in order: Apply Tunnel, Bring Up, then View Runtime.',
        ];

  final primaryAction = blocked
      ? _TunnelActionKind.apply
      : controlMapPending
          ? _TunnelActionKind.bootstrap
          : !hasApplied
              ? _TunnelActionKind.apply
              : !isUp
                  ? _TunnelActionKind.up
                  : !hasInspection
                      ? _TunnelActionKind.inspect
                      : _TunnelActionKind.down;
  final primaryActionLabel = blocked
      ? 'Re-apply configuration'
      : controlMapPending
          ? 'Run Refresh Bootstrap'
          : !hasApplied
              ? 'Run Apply Tunnel'
              : !isUp
                  ? 'Run Bring Up'
                  : !hasInspection
                      ? 'Run View Runtime'
                      : 'Run Bring Down';

  final primaryRecoveryAction =
      blocked ? _TunnelActionKind.apply : _TunnelActionKind.inspect;
  final primaryRecoveryLabel = blocked ? 'Apply Tunnel Again' : 'View Runtime';
  final secondaryRecoveryAction =
      blocked ? _TunnelActionKind.inspect : _TunnelActionKind.down;
  final secondaryRecoveryLabel =
      blocked ? 'Refresh Runtime' : 'Bring Tunnel Down';

  return _TunnelStagePlan(
    nextStepLabel: nextStepLabel,
    summary: summary,
    recoveryTitle: recoveryTitle,
    recoverySteps: recoverySteps,
    primaryAction: primaryAction,
    primaryActionLabel: primaryActionLabel,
    primaryRecoveryAction: primaryRecoveryAction,
    primaryRecoveryLabel: primaryRecoveryLabel,
    secondaryRecoveryAction: secondaryRecoveryAction,
    secondaryRecoveryLabel: secondaryRecoveryLabel,
    steps: [
      _TunnelStageStep(
        title: 'Apply configuration',
        description:
            'Stage interface, peer, endpoint, and engine settings in the plugin host.',
        state: hasApplied
            ? _TunnelStageState.complete
            : blocked
                ? _TunnelStageState.blocked
                : _TunnelStageState.current,
      ),
      _TunnelStageStep(
        title: 'Bring tunnel up',
        description:
            'Start PacketTunnel and let the selected backend move into an active state.',
        state: isUp
            ? _TunnelStageState.complete
            : blocked && hasApplied
                ? _TunnelStageState.blocked
                : hasApplied
                    ? _TunnelStageState.current
                    : _TunnelStageState.pending,
      ),
      _TunnelStageStep(
        title: 'Inspect runtime',
        description:
            'Refresh runtime to confirm endpoint selection, backend state, and RX/TX counters.',
        state: hasInspection
            ? _TunnelStageState.complete
            : blocked && (hasApplied || isUp)
                ? _TunnelStageState.blocked
                : isUp
                    ? _TunnelStageState.current
                    : _TunnelStageState.pending,
      ),
    ],
  );
}
