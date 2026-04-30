library slan_app.application.tunnel_runtime_service;

import 'package:slan_app_core_plugin/slan_app_core_plugin.dart';

import 'tunnel_host_gateway.dart';
import '../infra/app_core/models/tunnel_action_models.dart';

class TunnelRuntimeOperationResult {
  const TunnelRuntimeOperationResult({
    required this.report,
    this.runtimeView,
  });

  final TunnelActionReport report;
  final WireGuardTunnelRuntimeView? runtimeView;
}

abstract class TunnelRuntimeServiceContract {
  Future<TunnelRuntimeOperationResult> applyTunnelConfiguration({
    required WireGuardTunnelConfiguration configuration,
    String? verifyPeerVirtualIp,
  });

  Future<TunnelRuntimeOperationResult> bringTunnelUp({
    String? verifyPeerVirtualIp,
  });

  Future<TunnelRuntimeOperationResult> bringTunnelDown();

  Future<TunnelRuntimeOperationResult> removeTunnelPeer({
    required String peerVirtualIp,
  });

  Future<TunnelRuntimeOperationResult> refreshTunnelRuntime({
    required String peerVirtualIp,
  });
}

class TunnelRuntimeService implements TunnelRuntimeServiceContract {
  const TunnelRuntimeService({
    this.hostGateway = const PluginTunnelHostGateway(),
  });

  final TunnelHostGateway hostGateway;

  @override
  Future<TunnelRuntimeOperationResult> applyTunnelConfiguration({
    required WireGuardTunnelConfiguration configuration,
    String? verifyPeerVirtualIp,
  }) async {
    final nativeResult = await hostGateway.applyTunnelConfiguration(
      configuration,
    );
    final acceptedReport = TunnelActionReport(
      succeeded: nativeResult.accepted,
      detail: nativeResult.detail,
      source: TunnelActionReportSource.pluginHost,
      phase: _mapNativeTunnelActionPhase(nativeResult.phase),
      errorMessage: nativeResult.runtimeLastError,
    );
    if (!nativeResult.accepted) {
      return TunnelRuntimeOperationResult(report: acceptedReport);
    }

    final peerVirtualIp = verifyPeerVirtualIp?.trim();
    if (peerVirtualIp == null || peerVirtualIp.isEmpty) {
      return TunnelRuntimeOperationResult(report: acceptedReport);
    }

    final runtime = await hostGateway.tunnelRuntimeView(
      peerVirtualIp,
    );
    return TunnelRuntimeOperationResult(
      report: _buildApplyVerificationReport(configuration, runtime),
      runtimeView: runtime,
    );
  }

  @override
  Future<TunnelRuntimeOperationResult> bringTunnelUp({
    String? verifyPeerVirtualIp,
  }) async {
    final nativeResult = await hostGateway.bringTunnelUp();
    final acceptedReport = TunnelActionReport(
      succeeded: nativeResult.accepted,
      detail: nativeResult.detail,
      source: TunnelActionReportSource.pluginHost,
      phase: _mapNativeTunnelActionPhase(nativeResult.phase),
      errorMessage: nativeResult.runtimeLastError,
    );
    if (!nativeResult.accepted) {
      return TunnelRuntimeOperationResult(report: acceptedReport);
    }

    final peerVirtualIp = verifyPeerVirtualIp?.trim();
    if (peerVirtualIp == null || peerVirtualIp.isEmpty) {
      return TunnelRuntimeOperationResult(report: acceptedReport);
    }

    final runtime = await hostGateway.tunnelRuntimeView(
      peerVirtualIp,
    );
    return TunnelRuntimeOperationResult(
      report: _buildBringUpVerificationReport(runtime),
      runtimeView: runtime,
    );
  }

  @override
  Future<TunnelRuntimeOperationResult> bringTunnelDown() async {
    final nativeResult = await hostGateway.bringTunnelDown();
    return TunnelRuntimeOperationResult(
      report: TunnelActionReport(
        succeeded: nativeResult.accepted,
        detail: nativeResult.detail,
        source: TunnelActionReportSource.pluginHost,
        phase: _mapNativeTunnelActionPhase(nativeResult.phase),
        errorMessage: nativeResult.runtimeLastError,
      ),
    );
  }

  @override
  Future<TunnelRuntimeOperationResult> removeTunnelPeer({
    required String peerVirtualIp,
  }) async {
    final nativeResult = await hostGateway.removeTunnelPeer(peerVirtualIp);
    return TunnelRuntimeOperationResult(
      report: TunnelActionReport(
        succeeded: nativeResult.accepted,
        detail: nativeResult.detail,
        source: TunnelActionReportSource.pluginHost,
        phase: _mapNativeTunnelActionPhase(nativeResult.phase),
        errorMessage: nativeResult.runtimeLastError,
      ),
    );
  }

  @override
  Future<TunnelRuntimeOperationResult> refreshTunnelRuntime({
    required String peerVirtualIp,
  }) async {
    final runtime = await hostGateway.tunnelRuntimeView(
      peerVirtualIp,
    );
    final errorMessage = runtime?.backendLastError ?? runtime?.lastError;
    if (runtime == null) {
      return const TunnelRuntimeOperationResult(
        report: TunnelActionReport(
          succeeded: false,
          detail: 'Mesh runtime snapshot is unavailable for the selected peer.',
          source: TunnelActionReportSource.runtimeSnapshot,
          phase: TunnelActionPhase.failed,
          errorMessage: 'No runtime snapshot returned from native backend.',
        ),
      );
    }
    return TunnelRuntimeOperationResult(
      report: TunnelActionReport(
        succeeded: (errorMessage?.isEmpty ?? true),
          detail:
            'Mesh runtime reports ${runtime.state} and backend ${runtime.backendState}.',
        source: TunnelActionReportSource.runtimeSnapshot,
        phase: _deriveRuntimeInspectionPhase(runtime, errorMessage),
        errorMessage: errorMessage,
        runtimeSnapshot: runtime,
      ),
      runtimeView: runtime,
    );
  }
}

TunnelActionReport _buildBringUpVerificationReport(
  WireGuardTunnelRuntimeView? runtime,
) {
  if (runtime == null) {
    return const TunnelActionReport(
      succeeded: false,
      detail:
          'Native plugin accepted the bring-up request, but runtime verification returned no session snapshot.',
      source: TunnelActionReportSource.runtimeSnapshot,
      phase: TunnelActionPhase.pendingVerification,
      errorMessage: 'Bring-up could not be verified from runtime.',
    );
  }

  final backendError = runtime.backendLastError;
  final tunnelError = runtime.lastError;
  final errorMessage =
      (backendError?.isNotEmpty ?? false) ? backendError : tunnelError;
  if (errorMessage?.isNotEmpty ?? false) {
    return TunnelActionReport(
      succeeded: false,
      detail:
          'Native plugin accepted the bring-up request, but runtime reported ${runtime.state}/${runtime.backendState}.',
      source: TunnelActionReportSource.runtimeSnapshot,
      phase: TunnelActionPhase.failed,
      errorMessage: errorMessage,
      runtimeSnapshot: runtime,
    );
  }

  final state = runtime.state.toLowerCase();
  final backendState = (runtime.backendState ?? '').toLowerCase();
  final looksStarted = (state == 'up' || state == 'running') &&
      (backendState == 'started' || backendState == 'running');
  if (looksStarted) {
    return TunnelActionReport(
      succeeded: true,
      detail:
          'Bring-up verified: runtime now reports tunnel ${runtime.state} and backend ${runtime.backendState}.',
      source: TunnelActionReportSource.runtimeSnapshot,
      phase: TunnelActionPhase.started,
      runtimeSnapshot: runtime,
    );
  }

  return TunnelActionReport(
    succeeded: false,
    detail:
        'Native plugin accepted the bring-up request, but runtime still reports tunnel ${runtime.state} and backend ${runtime.backendState}.',
    source: TunnelActionReportSource.runtimeSnapshot,
    phase: TunnelActionPhase.pendingVerification,
    errorMessage: 'Backend start is not verified yet.',
    runtimeSnapshot: runtime,
  );
}

TunnelActionReport _buildApplyVerificationReport(
  WireGuardTunnelConfiguration configuration,
  WireGuardTunnelRuntimeView? runtime,
) {
  if (runtime == null) {
    return const TunnelActionReport(
      succeeded: true,
      detail:
          'Native plugin accepted the overlay configuration, but runtime verification is still pending.',
      source: TunnelActionReportSource.pluginHost,
      phase: TunnelActionPhase.accepted,
    );
  }

  final backendError = runtime.backendLastError;
  final tunnelError = runtime.lastError;
  final errorMessage =
      (backendError?.isNotEmpty ?? false) ? backendError : tunnelError;
  if (errorMessage?.isNotEmpty ?? false) {
    return TunnelActionReport(
      succeeded: false,
      detail:
          'Native plugin accepted the configuration, but runtime reported ${runtime.state}/${runtime.backendState ?? 'unknown'}.',
      source: TunnelActionReportSource.runtimeSnapshot,
      phase: TunnelActionPhase.failed,
      errorMessage: errorMessage,
      runtimeSnapshot: runtime,
    );
  }

  final peerMatches = runtime.peerVirtualIp == configuration.peerVirtualIp;
  final endpointMatches = configuration.peer.endpoint == null ||
      runtime.selectedEndpoint == configuration.peer.endpoint ||
      runtime.backendSelectedEndpoint == configuration.peer.endpoint;
  final appliedRecorded = (runtime.lastAppliedAtMs ?? 0) > 0;
  if (peerMatches && endpointMatches && appliedRecorded) {
    return TunnelActionReport(
      succeeded: true,
      detail:
          'Configuration staged and runtime reflects peer ${runtime.peerVirtualIp}${runtime.selectedEndpoint == null ? '' : ' via ${runtime.selectedEndpoint}'}'
              .trim(),
      source: TunnelActionReportSource.runtimeSnapshot,
      phase: TunnelActionPhase.configured,
      runtimeSnapshot: runtime,
    );
  }

  if (peerMatches && endpointMatches) {
    return const TunnelActionReport(
          succeeded: true,
          detail:
          'Native plugin accepted the overlay configuration; runtime shape matches, but apply verification is still pending.',
      source: TunnelActionReportSource.runtimeSnapshot,
      phase: TunnelActionPhase.pendingVerification,
    );
  }

  return const TunnelActionReport(
    succeeded: true,
    detail:
        'Native plugin accepted the overlay configuration, but runtime verification has not caught up yet.',
    source: TunnelActionReportSource.pluginHost,
    phase: TunnelActionPhase.accepted,
  );
}

TunnelActionPhase _mapNativeTunnelActionPhase(
  WireGuardTunnelActionPhase phase,
) {
  return switch (phase) {
    WireGuardTunnelActionPhase.accepted => TunnelActionPhase.accepted,
    WireGuardTunnelActionPhase.configured => TunnelActionPhase.configured,
    WireGuardTunnelActionPhase.started => TunnelActionPhase.started,
    WireGuardTunnelActionPhase.verified => TunnelActionPhase.verified,
    WireGuardTunnelActionPhase.failed => TunnelActionPhase.failed,
    WireGuardTunnelActionPhase.pendingVerification =>
      TunnelActionPhase.pendingVerification,
  };
}

TunnelActionPhase _deriveRuntimeInspectionPhase(
  WireGuardTunnelRuntimeView runtime,
  String? errorMessage,
) {
  if (errorMessage?.isNotEmpty ?? false) {
    return TunnelActionPhase.failed;
  }
  final state = runtime.state.toLowerCase();
  final backendState = (runtime.backendState ?? '').toLowerCase();
  if ((state == 'up' || state == 'running') &&
      (backendState == 'started' || backendState == 'running')) {
    return TunnelActionPhase.verified;
  }
  if ((runtime.lastAppliedAtMs ?? 0) > 0) {
    return TunnelActionPhase.configured;
  }
  return TunnelActionPhase.accepted;
}
