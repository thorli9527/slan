import 'package:slan_app_core_plugin/slan_app_core_plugin.dart';

enum TunnelActionReportSource { pluginHost, runtimeSnapshot }

enum TunnelActionPhase {
  accepted,
  configured,
  started,
  verified,
  failed,
  pendingVerification,
}

class TunnelActionReport {
  const TunnelActionReport({
    required this.succeeded,
    required this.detail,
    required this.source,
    required this.phase,
    this.errorMessage,
    this.runtimeSnapshot,
  });

  final bool succeeded;
  final String detail;
  final TunnelActionReportSource source;
  final TunnelActionPhase phase;
  final String? errorMessage;
  final WireGuardTunnelRuntimeView? runtimeSnapshot;

  String get sourceLabel => switch (source) {
        TunnelActionReportSource.pluginHost => 'plugin host',
        TunnelActionReportSource.runtimeSnapshot => 'runtime snapshot',
      };
}
