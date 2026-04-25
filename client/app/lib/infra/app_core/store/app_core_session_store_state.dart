import 'package:flutter/foundation.dart';

import '../models/bootstrap_models.dart';
import '../models/connection_models.dart';
import '../models/control_models.dart';
import '../models/identity_models.dart';
import '../models/network_models.dart';
import '../models/relay_models.dart';

mixin AppCoreSessionStoreState on ChangeNotifier {
  SessionModel? session;
  DeviceModel? device;
  List<DeviceModel> devices = const [];
  NodeModel? node;
  List<NetworkModel> networks = const [];
  String? selectedNetworkId;
  BootstrapModel? bootstrap;
  ControlStatusModel? controlStatus;
  RelayTicketModel? relayTicket;
  ConnectionStateModel connectionState =
      const ConnectionStateModel.disconnected();
  bool busy = false;
  String? error;
  String? notice;

  @protected
  void resetSessionState() {
    session = null;
    device = null;
    devices = const [];
    node = null;
    networks = const [];
    selectedNetworkId = null;
    bootstrap = null;
    controlStatus = null;
    relayTicket = null;
    connectionState = const ConnectionStateModel.disconnected();
    busy = false;
    error = null;
    notice = null;
  }

  @protected
  void syncSessionDevice(DeviceModel? nextDevice) {
    device = nextDevice;
    if (nextDevice != null) {
      final index =
          devices.indexWhere((item) => item.deviceId == nextDevice.deviceId);
      if (index < 0) {
        devices = List.unmodifiable([nextDevice, ...devices]);
      } else {
        devices = List.unmodifiable([
          for (var i = 0; i < devices.length; i++)
            if (i == index) nextDevice else devices[i],
        ]);
      }
    }
    session = session == null || nextDevice == null
        ? session
        : SessionModel(
            userId: session!.userId,
            accessToken: session!.accessToken,
            refreshToken: session!.refreshToken,
            expiresIn: session!.expiresIn,
            deviceId: nextDevice.deviceId,
            userLabel: session!.userLabel,
            authenticatedAtMs: session!.authenticatedAtMs,
          );
  }

  @protected
  void clearSessionConnectionState() {
    connectionState = const ConnectionStateModel.disconnected();
    relayTicket = null;
    controlStatus = null;
  }

  NetworkModel? get selectedNetwork {
    if (networks.isEmpty) {
      return null;
    }
    final targetId = selectedNetworkId;
    if (targetId != null && targetId.isNotEmpty) {
      for (final network in networks) {
        if (network.networkId == targetId) {
          return network;
        }
      }
    }
    return networks.first;
  }

  void syncSelectedNetworkId({String? preferredNetworkId}) {
    if (networks.isEmpty) {
      selectedNetworkId = null;
      return;
    }
    final preferred = preferredNetworkId?.trim();
    if (preferred != null &&
        preferred.isNotEmpty &&
        networks.any((network) => network.networkId == preferred)) {
      selectedNetworkId = preferred;
      return;
    }
    final current = selectedNetworkId?.trim();
    if (current != null &&
        current.isNotEmpty &&
        networks.any((network) => network.networkId == current)) {
      return;
    }
    selectedNetworkId = networks.first.networkId;
  }
}
