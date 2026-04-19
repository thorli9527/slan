import 'package:flutter/foundation.dart';
import 'package:slan_app_core_plugin/slan_app_core_plugin.dart';

import '../models/models.dart';

mixin AppCoreDemoStoreState on ChangeNotifier {
  SessionModel? session;
  DeviceModel? device;
  NodeModel? node;
  List<NetworkModel> networks = const [];
  BootstrapModel? bootstrap;
  ControlStatusModel? controlStatus;
  RelayTicketModel? relayTicket;
  DataPlaneProbeModel? lastProbe;
  ProbeFailure? lastProbeFailure;
  int? lastSendBytes;
  SendFailure? lastSendFailure;
  WireGuardTunnelRuntimeView? tunnelRuntimeView;
  TunnelActionReport? lastTunnelActionReport;
  ConnectionStateModel connectionState =
      const ConnectionStateModel.disconnected();
  bool busy = false;
  String? error;
  String? tunnelDebugError;

  @protected
  void syncSessionDevice(DeviceModel? nextDevice) {
    device = nextDevice;
    session = session == null || nextDevice == null
        ? session
        : SessionModel(
            userId: session!.userId,
            accessToken: session!.accessToken,
            refreshToken: session!.refreshToken,
            expiresIn: session!.expiresIn,
            deviceId: nextDevice.deviceId,
          );
  }

  @protected
  void clearConnectionArtifacts() {
    connectionState = const ConnectionStateModel.disconnected();
    relayTicket = null;
    controlStatus = null;
    lastProbe = null;
    lastProbeFailure = null;
    lastSendBytes = null;
    lastSendFailure = null;
    tunnelRuntimeView = null;
    lastTunnelActionReport = null;
    tunnelDebugError = null;
  }
}
