library slan_app.application.tunnel_session_service;

import '../infra/app_core/models/tunnel_action_models.dart';

class TunnelProgressUpdate {
  const TunnelProgressUpdate({
    required this.detail,
    required this.progressLabel,
  });

  final String detail;
  final String progressLabel;
}

typedef TunnelProgressReporter = void Function(TunnelProgressUpdate update);
typedef TunnelFailureReader = String? Function();

abstract class TunnelSessionServiceContract {
  Future<TunnelActionReport?> recoverSession({
    required TunnelProgressReporter onProgress,
    required Future<TunnelActionReport> Function() applyConfiguration,
    required Future<TunnelActionReport> Function() bringTunnelUp,
    required Future<TunnelActionReport> Function() inspectRuntime,
  });

  Future<TunnelActionReport?> quickSetup({
    required TunnelProgressReporter onProgress,
    required Future<void> Function() ensureDeviceRegistered,
    required Future<void> Function() ensureNetworkAvailableAndJoined,
    required Future<void> Function() ensureNodeRegisteredAndBootstrapped,
    required Future<TunnelActionReport> Function() applyConfiguration,
    required Future<TunnelActionReport> Function() bringTunnelUp,
    required TunnelFailureReader readCurrentFailure,
  });
}

class TunnelSessionService implements TunnelSessionServiceContract {
  const TunnelSessionService();

  @override
  Future<TunnelActionReport?> recoverSession({
    required TunnelProgressReporter onProgress,
    required Future<TunnelActionReport> Function() applyConfiguration,
    required Future<TunnelActionReport> Function() bringTunnelUp,
    required Future<TunnelActionReport> Function() inspectRuntime,
  }) async {
    onProgress(
      const TunnelProgressUpdate(
        detail: 'Applying configuration before recovery',
        progressLabel: '1/3 Apply configuration',
      ),
    );
    final applyReport = await applyConfiguration();
    if (!applyReport.succeeded) {
      return applyReport;
    }

    onProgress(
      TunnelProgressUpdate(
        detail: _formatTunnelActionReportDetail(applyReport),
        progressLabel: '2/3 Bring tunnel up',
      ),
    );
    final upReport = await bringTunnelUp();
    if (!upReport.succeeded) {
      return upReport;
    }

    onProgress(
      TunnelProgressUpdate(
        detail: _formatTunnelActionReportDetail(upReport),
        progressLabel: '3/3 Inspect runtime',
      ),
    );
    return inspectRuntime();
  }

  @override
  Future<TunnelActionReport?> quickSetup({
    required TunnelProgressReporter onProgress,
    required Future<void> Function() ensureDeviceRegistered,
    required Future<void> Function() ensureNetworkAvailableAndJoined,
    required Future<void> Function() ensureNodeRegisteredAndBootstrapped,
    required Future<TunnelActionReport> Function() applyConfiguration,
    required Future<TunnelActionReport> Function() bringTunnelUp,
    required TunnelFailureReader readCurrentFailure,
  }) async {
    onProgress(
      const TunnelProgressUpdate(
        detail: 'Preparing local device identity',
        progressLabel: '1/5 Register device',
      ),
    );
    await ensureDeviceRegistered();
    final deviceFailure = readCurrentFailure();
    if (deviceFailure != null && deviceFailure.isNotEmpty) {
      return null;
    }

    onProgress(
      const TunnelProgressUpdate(
        detail: 'Ensuring a usable overlay network exists',
        progressLabel: '2/5 Join network',
      ),
    );
    await ensureNetworkAvailableAndJoined();
    final networkFailure = readCurrentFailure();
    if (networkFailure != null && networkFailure.isNotEmpty) {
      return null;
    }

    onProgress(
      const TunnelProgressUpdate(
        detail: 'Registering a node and loading bootstrap',
        progressLabel: '3/5 Register node',
      ),
    );
    await ensureNodeRegisteredAndBootstrapped();
    final nodeFailure = readCurrentFailure();
    if (nodeFailure != null && nodeFailure.isNotEmpty) {
      return null;
    }

    onProgress(
      const TunnelProgressUpdate(
        detail: 'Staging tunnel configuration from current bootstrap state',
        progressLabel: '4/5 Apply tunnel',
      ),
    );
    final applyReport = await applyConfiguration();
    if (!applyReport.succeeded) {
      return applyReport;
    }

    onProgress(
      const TunnelProgressUpdate(
        detail: 'Bringing PacketTunnel session up',
        progressLabel: '5/5 Tunnel up',
      ),
    );
    return bringTunnelUp();
  }
}

String _formatTunnelActionReportDetail(TunnelActionReport? report) {
  if (report == null) {
    return 'Tunnel action completed without a structured backend report.';
  }
  return '${report.detail} Verified by ${report.sourceLabel}.';
}
