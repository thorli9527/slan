import 'package:flutter/foundation.dart';
import 'package:slan_app_core_plugin/slan_app_core_plugin.dart';

import '../models/diagnostic_models.dart';
import '../models/tunnel_action_models.dart';

mixin AppCoreTunnelStoreState on ChangeNotifier {
  DataPlaneProbeModel? lastProbe;
  ProbeFailure? lastProbeFailure;
  int? lastSendBytes;
  SendFailure? lastSendFailure;
  WireGuardTunnelRuntimeView? tunnelRuntimeView;
  TunnelActionReport? lastTunnelActionReport;
  String? tunnelDebugError;

  @protected
  void resetTunnelState() {
    lastProbe = null;
    lastProbeFailure = null;
    lastSendBytes = null;
    lastSendFailure = null;
    tunnelRuntimeView = null;
    lastTunnelActionReport = null;
    tunnelDebugError = null;
  }

  @protected
  void clearTunnelConnectionArtifacts() {
    lastProbe = null;
    lastProbeFailure = null;
    lastSendBytes = null;
    lastSendFailure = null;
    tunnelRuntimeView = null;
    lastTunnelActionReport = null;
    tunnelDebugError = null;
  }
}
