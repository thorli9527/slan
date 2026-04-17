import 'package:flutter/foundation.dart';

import 'app_core_scope.dart';
import 'models.dart';

class AppCoreDemoStore extends ChangeNotifier {
  SessionModel? session;
  DeviceModel? device;
  NodeModel? node;
  List<NetworkModel> networks = const [];
  BootstrapModel? bootstrap;
  RelayTicketModel? relayTicket;
  ConnectionStateModel connectionState =
      const ConnectionStateModel.disconnected();
  bool busy = false;
  String? error;

  Future<void> register({
    required String email,
    required String password,
  }) async {
    await _run(() async {
      session = await AppCoreScope.instance.register(
        email: email,
        password: password,
      );
    });
  }

  Future<void> login({
    required String email,
    required String password,
  }) async {
    await _run(() async {
      session = await AppCoreScope.instance.login(
        email: email,
        password: password,
      );
    });
  }

  Future<void> registerDevice({
    required String name,
    required String platform,
    required String machineId,
    required String publicKey,
  }) async {
    await _run(() async {
      device = await AppCoreScope.instance.registerDevice(
        name: name,
        platform: platform,
        machineId: machineId,
        publicKey: publicKey,
      );
      session = session == null
          ? null
          : SessionModel(
              userId: session!.userId,
              accessToken: session!.accessToken,
              refreshToken: session!.refreshToken,
              expiresIn: session!.expiresIn,
              deviceId: device!.deviceId,
            );
    });
  }

  Future<void> refreshNetworks() async {
    await _run(() async {
      networks = await AppCoreScope.instance.listNetworks();
    });
  }

  Future<void> createNetwork({
    required String name,
    required String cidr,
  }) async {
    await _run(() async {
      await AppCoreScope.instance.createNetwork(name: name, cidr: cidr);
      networks = await AppCoreScope.instance.listNetworks();
    });
  }

  Future<void> registerNode({
    required String deviceId,
    required String nodeId,
    required String nodePublicKey,
    List<String> capabilities = const [],
  }) async {
    await _run(() async {
      node = await AppCoreScope.instance.registerNode(
        deviceId: deviceId,
        nodeId: nodeId,
        nodePublicKey: nodePublicKey,
        capabilities: capabilities,
      );
    });
  }

  Future<void> loadBootstrap({
    String? nodeId,
    String? networkId,
  }) async {
    final targetNodeId =
        nodeId?.trim().isNotEmpty == true ? nodeId!.trim() : this.node?.nodeId;
    final targetNetworkId = networkId?.trim().isNotEmpty == true
        ? networkId!.trim()
        : networks.isNotEmpty
            ? networks.first.networkId
            : null;
    if (targetNodeId == null ||
        targetNodeId.isEmpty ||
        targetNetworkId == null) {
      error = 'nodeId and networkId are required';
      notifyListeners();
      return;
    }
    await _run(() async {
      bootstrap = await AppCoreScope.instance.bootstrap(
        nodeId: targetNodeId,
        networkId: targetNetworkId,
      );
      device = bootstrap!.device;
      networks = bootstrap!.networks;
    });
  }

  Future<void> connectWithFallback({
    required String networkId,
    required String peerNodeId,
    required String reason,
  }) async {
    final localNodeId = node?.nodeId;
    if (localNodeId == null || localNodeId.isEmpty) {
      error = 'register a node first';
      notifyListeners();
      return;
    }
    await _run(() async {
      relayTicket = null;
      connectionState = const ConnectionStateModel.connecting();
      notifyListeners();

      final direct = await AppCoreScope.instance.connect(
        networkId: networkId,
        peerNodeId: peerNodeId,
      );
      if (direct.status == 'connected') {
        connectionState = direct;
        return;
      }

      if (direct.status != 'failed') {
        connectionState = direct;
        return;
      }

      relayTicket = await AppCoreScope.instance.issueRelayTicket(
        networkId: networkId,
        srcNodeId: localNodeId,
        dstNodeId: peerNodeId,
        reason: reason,
      );
      connectionState = await AppCoreScope.instance.connect(
        networkId: networkId,
        peerNodeId: 'relay-$peerNodeId',
      );
    });
  }

  Future<void> disconnect() async {
    await _run(() async {
      await AppCoreScope.instance.disconnect();
      connectionState = const ConnectionStateModel.disconnected();
      relayTicket = null;
    });
  }

  Future<void> _run(Future<void> Function() action) async {
    busy = true;
    error = null;
    notifyListeners();
    try {
      await action();
    } catch (err) {
      error = err.toString();
    } finally {
      busy = false;
      notifyListeners();
    }
  }
}
