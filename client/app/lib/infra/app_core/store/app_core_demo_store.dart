import 'package:flutter/foundation.dart';
import 'package:slan_app_core_plugin/slan_app_core_plugin.dart';

import '../scope/app_core_scope.dart';
import '../models/models.dart';
import 'app_core_demo_store_async.dart';
import 'app_core_demo_store_state.dart';

class AppCoreDemoStore extends ChangeNotifier
    with AppCoreDemoStoreState, AppCoreDemoStoreAsync {
  Future<void> register({
    required String email,
    required String password,
  }) async {
    await runAction(() async {
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
    await runAction(() async {
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
    await runAction(() async {
      final nextDevice = await AppCoreScope.instance.registerDevice(
        name: name,
        platform: platform,
        machineId: machineId,
        publicKey: publicKey,
      );
      syncSessionDevice(nextDevice);
    });
  }

  Future<void> refreshNetworks() async {
    await runAction(() async {
      networks = await AppCoreScope.instance.listNetworks();
    });
  }

  Future<void> createNetwork({
    required String name,
    required String cidr,
  }) async {
    await runAction(() async {
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
    await runAction(() async {
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
        nodeId?.trim().isNotEmpty == true ? nodeId!.trim() : node?.nodeId;
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
    await runAction(() async {
      bootstrap = await AppCoreScope.instance.bootstrap(
        nodeId: targetNodeId,
        networkId: targetNetworkId,
      );
      controlStatus = await AppCoreScope.instance.controlStatus();
      device = bootstrap!.device;
      networks = bootstrap!.networks;
    });
  }

  Future<void> syncControlPlane({
    required String nodeId,
    required String networkId,
  }) async {
    await runAction(() async {
      bootstrap = await AppCoreScope.instance.controlSync(
        nodeId: nodeId,
        networkId: networkId,
      );
      controlStatus = await AppCoreScope.instance.controlStatus();
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
    await runAction(() async {
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
    await runAction(() async {
      await AppCoreScope.instance.disconnect();
      clearConnectionArtifacts();
    });
  }

  Future<TunnelActionReport> applyTunnelConfiguration({
    required WireGuardTunnelConfiguration configuration,
    String? verifyPeerVirtualIp,
  }) async {
    final report = await runTunnelAction<TunnelActionReport>(() async {
      final nativeResult =
          await SlanAppCorePluginPlatform.instance.applyTunnelConfiguration(
        configuration,
      );
      tunnelRuntimeView = null;
      final acceptedReport = TunnelActionReport(
        succeeded: nativeResult.accepted,
        detail: nativeResult.detail,
        source: TunnelActionReportSource.pluginHost,
        phase: _mapNativeTunnelActionPhase(nativeResult.phase),
        errorMessage: nativeResult.runtimeLastError,
      );
      if (!nativeResult.accepted) {
        return acceptedReport;
      }
      final peerVirtualIp = verifyPeerVirtualIp?.trim();
      if (peerVirtualIp == null || peerVirtualIp.isEmpty) {
        return acceptedReport;
      }

      final runtime =
          await SlanAppCorePluginPlatform.instance.tunnelRuntimeView(
        peerVirtualIp,
      );
      tunnelRuntimeView = runtime;
      return _buildApplyVerificationReport(configuration, runtime);
    });
    lastTunnelActionReport = report ?? _failedTunnelActionReport();
    return lastTunnelActionReport!;
  }

  Future<TunnelActionReport> bringTunnelUp({
    String? verifyPeerVirtualIp,
  }) async {
    final report = await runTunnelAction<TunnelActionReport>(() async {
      final nativeResult =
          await SlanAppCorePluginPlatform.instance.bringTunnelUp();
      final acceptedReport = TunnelActionReport(
        succeeded: nativeResult.accepted,
        detail: nativeResult.detail,
        source: TunnelActionReportSource.pluginHost,
        phase: _mapNativeTunnelActionPhase(nativeResult.phase),
        errorMessage: nativeResult.runtimeLastError,
      );
      if (!nativeResult.accepted) {
        return acceptedReport;
      }
      final peerVirtualIp = verifyPeerVirtualIp?.trim();
      if (peerVirtualIp == null || peerVirtualIp.isEmpty) {
        return acceptedReport;
      }

      final runtime =
          await SlanAppCorePluginPlatform.instance.tunnelRuntimeView(
        peerVirtualIp,
      );
      tunnelRuntimeView = runtime;
      return _buildBringUpVerificationReport(runtime);
    });
    lastTunnelActionReport = report ?? _failedTunnelActionReport();
    return lastTunnelActionReport!;
  }

  Future<TunnelActionReport> bringTunnelDown() async {
    final report = await runTunnelAction<TunnelActionReport>(() async {
      final nativeResult =
          await SlanAppCorePluginPlatform.instance.bringTunnelDown();
      tunnelRuntimeView = null;
      return TunnelActionReport(
        succeeded: nativeResult.accepted,
        detail: nativeResult.detail,
        source: TunnelActionReportSource.pluginHost,
        phase: _mapNativeTunnelActionPhase(nativeResult.phase),
        errorMessage: nativeResult.runtimeLastError,
      );
    });
    lastTunnelActionReport = report ?? _failedTunnelActionReport();
    return lastTunnelActionReport!;
  }

  Future<TunnelActionReport> removeTunnelPeer({
    required String peerVirtualIp,
  }) async {
    final report = await runTunnelAction<TunnelActionReport>(() async {
      final nativeResult = await SlanAppCorePluginPlatform.instance
          .removeTunnelPeer(peerVirtualIp);
      tunnelRuntimeView = null;
      return TunnelActionReport(
        succeeded: nativeResult.accepted,
        detail: nativeResult.detail,
        source: TunnelActionReportSource.pluginHost,
        phase: _mapNativeTunnelActionPhase(nativeResult.phase),
        errorMessage: nativeResult.runtimeLastError,
      );
    });
    lastTunnelActionReport = report ?? _failedTunnelActionReport();
    return lastTunnelActionReport!;
  }

  Future<TunnelActionReport> refreshTunnelRuntime({
    required String peerVirtualIp,
  }) async {
    final report = await runTunnelAction<TunnelActionReport>(() async {
      final runtime = await SlanAppCorePluginPlatform.instance
          .tunnelRuntimeView(peerVirtualIp);
      tunnelRuntimeView = runtime;
      final errorMessage = runtime?.backendLastError ?? runtime?.lastError;
      if (runtime == null) {
        return const TunnelActionReport(
          succeeded: false,
          detail: 'Runtime snapshot is unavailable for the selected peer.',
          source: TunnelActionReportSource.runtimeSnapshot,
          phase: TunnelActionPhase.failed,
          errorMessage: 'No runtime snapshot returned from native backend.',
        );
      }
      final phase = _deriveRuntimeInspectionPhase(runtime, errorMessage);
      return TunnelActionReport(
        succeeded: (errorMessage?.isEmpty ?? true),
        detail:
            'Runtime reports tunnel ${runtime.state} and backend ${runtime.backendState}.',
        source: TunnelActionReportSource.runtimeSnapshot,
        phase: phase,
        errorMessage: errorMessage,
        runtimeSnapshot: runtime,
      );
    });
    lastTunnelActionReport = report ?? _failedTunnelActionReport();
    return lastTunnelActionReport!;
  }

  TunnelActionReport _failedTunnelActionReport() {
    return TunnelActionReport(
      succeeded: false,
      detail:
          'Native tunnel action failed before a verified backend signal arrived.',
      source: TunnelActionReportSource.pluginHost,
      phase: TunnelActionPhase.failed,
      errorMessage:
          tunnelDebugError ?? error ?? 'unknown tunnel action failure',
      runtimeSnapshot: tunnelRuntimeView,
    );
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

    final state = (runtime.state).toLowerCase();
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
            'Native plugin accepted the tunnel configuration, but runtime verification is still pending.',
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
            'Native plugin accepted the tunnel configuration; runtime shape matches, but apply verification is still pending.',
        source: TunnelActionReportSource.runtimeSnapshot,
        phase: TunnelActionPhase.pendingVerification,
      );
    }

    return const TunnelActionReport(
      succeeded: true,
      detail:
          'Native plugin accepted the tunnel configuration, but runtime verification has not caught up yet.',
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

  Future<void> probe({
    required String payload,
    int? probeTimeoutMs,
  }) async {
    await runProbeAction(() {
      return AppCoreScope.instance.probe(
        payload: payload,
        probeTimeoutMs: probeTimeoutMs,
      );
    });
  }

  Future<void> send({
    required String payload,
  }) async {
    await runSendAction(() => AppCoreScope.instance.send(payload: payload));
  }
}
