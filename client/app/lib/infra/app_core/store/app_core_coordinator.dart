import 'package:flutter/foundation.dart' show debugPrint;
import 'package:slan_app_core_plugin/slan_app_core_plugin.dart';

import '../../../application/app_workspace_service.dart';
import '../../../application/auth_session_service.dart';
import '../../../application/device_setup_service.dart';
import '../../../application/device_runtime_service.dart';
import '../../../application/tunnel_configuration_service.dart';
import '../../../application/tunnel_host_gateway.dart';
import '../../../application/tunnel_runtime_service.dart';
import '../scope/app_core_scope.dart';
import '../models/diagnostic_models.dart';
import '../models/identity_models.dart';
import '../models/tunnel_action_models.dart';
import 'app_core_coordinator_async.dart';
import 'app_session_store.dart';
import 'app_tunnel_store.dart';

class AppCoreCoordinator with AppCoreCoordinatorAsync {
  static const defaultAutoNetworkCidr = '10.0.0.0/16';
  AppCoreCoordinator({
    TunnelHostGateway hostGateway = const PluginTunnelHostGateway(),
  }) : _tunnelRuntimeService = TunnelRuntimeService(hostGateway: hostGateway);

  final AppSessionStore sessionStore = AppSessionStore();
  final AppTunnelStore tunnelStore = AppTunnelStore();
  final AppWorkspaceService _workspaceService = AppWorkspaceService(
    apiProvider: () => AppCoreScope.instance,
  );
  final AuthSessionService _authSessionService = AuthSessionService(
    apiProvider: () => AppCoreScope.instance,
  );
  final DeviceSetupService _deviceSetupService = DeviceSetupService(
    apiProvider: () => AppCoreScope.instance,
  );
  final DeviceRuntimeService _deviceRuntimeService = DeviceRuntimeService(
    apiProvider: () => AppCoreScope.instance,
  );
  final TunnelRuntimeService _tunnelRuntimeService;
  final TunnelConfigurationService _tunnelConfigurationService =
      const TunnelConfigurationService();

  @override
  bool get busy => sessionStore.busy;
  @override
  set busy(bool value) => sessionStore.busy = value;

  @override
  String? get error => sessionStore.error;
  @override
  set error(String? value) => sessionStore.error = value;

  @override
  DataPlaneProbeModel? get lastProbe => tunnelStore.lastProbe;
  @override
  set lastProbe(DataPlaneProbeModel? value) => tunnelStore.lastProbe = value;

  @override
  ProbeFailure? get lastProbeFailure => tunnelStore.lastProbeFailure;
  @override
  set lastProbeFailure(ProbeFailure? value) =>
      tunnelStore.lastProbeFailure = value;

  @override
  int? get lastSendBytes => tunnelStore.lastSendBytes;
  @override
  set lastSendBytes(int? value) => tunnelStore.lastSendBytes = value;

  @override
  SendFailure? get lastSendFailure => tunnelStore.lastSendFailure;
  @override
  set lastSendFailure(SendFailure? value) =>
      tunnelStore.lastSendFailure = value;

  @override
  String? get tunnelDebugError => tunnelStore.tunnelDebugError;
  @override
  set tunnelDebugError(String? value) => tunnelStore.tunnelDebugError = value;

  @override
  void emitStateChanged() {
    sessionStore.emit();
    tunnelStore.emit();
  }

  void resetState() {
    sessionStore.resetAll();
    tunnelStore.resetAll();
  }

  void resetForServerSwitch() {
    resetState();
    emitStateChanged();
  }

  Future<void> applyExternalSession(SessionModel nextSession) async {
    await applyExternalSessionInternal(nextSession, persistSession: true);
  }

  Future<void> applyExternalSessionInternal(
    SessionModel nextSession, {
    required bool persistSession,
  }) async {
    await runAction(() async {
      final hydratedSession = nextSession.authenticatedAtMs == null
          ? nextSession.copyWith(
              authenticatedAtMs: DateTime.now().millisecondsSinceEpoch,
            )
          : nextSession;
      debugPrint(
        '[auth-callback] store applyExternalSessionInternal start userId=${hydratedSession.userId} deviceId=${hydratedSession.deviceId}',
      );
      resetState();
      sessionStore.session = hydratedSession;
      if (persistSession) {
        await AppCoreScope.persistSession(hydratedSession);
      }
      sessionStore.notice = '已收到浏览器登录回调，正在恢复客户端会话。';
      emitStateChanged();
      debugPrint(
          '[auth-callback] store session assigned and listeners notified');
      final hydrated =
          await _authSessionService.hydrateExternalSession(hydratedSession);
      if (hydrated.device != null) {
        sessionStore.syncDevice(hydrated.device);
        emitStateChanged();
        debugPrint(
          '[auth-callback] store matched device=${hydrated.device!.deviceId}',
        );
      }
      sessionStore.networks = hydrated.networks;
      debugPrint(
          '[auth-callback] store loaded networks count=${sessionStore.networks.length}');
      sessionStore.notice = hydrated.notice;
      debugPrint('[auth-callback] store final notice=${sessionStore.notice}');
    });
  }

  Future<void> signOut() async {
    await runAction(() async {
      try {
        await bringTunnelDown();
      } catch (_) {
        // Best effort. Local state still needs to be cleared even if the
        // tunnel runtime is already gone or not initialized.
      }
      await AppCoreScope.clearPersistedSession();
      resetState();
    });
  }

  Future<void> registerDevice({
    required String name,
    required String platform,
    required String machineId,
    required String publicKey,
  }) async {
    await runAction(() async {
      final result = await _deviceSetupService.registerDevice(
        DeviceRegistrationInput(
          name: name,
          platform: platform,
          machineId: machineId,
          publicKey: publicKey,
        ),
      );
      sessionStore.syncDevice(result.device);
    });
  }

  Future<String?> ensureHomeWorkspaceReady({
    required String deviceName,
    required String platform,
    required String machineId,
    required String devicePublicKey,
  }) async {
    String? statusMessage;
    await runAction(() async {
      final result = await _workspaceService.ensureWorkspaceReady(
        session: sessionStore.session,
        currentDevice: sessionStore.device,
        deviceName: deviceName,
        platform: platform,
        machineId: machineId,
        devicePublicKey: devicePublicKey,
      );
      sessionStore.syncDevice(result.device);
      sessionStore.networks = result.networks;
      statusMessage = result.statusMessage;
    });
    return statusMessage;
  }

  Future<void> registerNode({
    required String deviceId,
    required String nodeId,
    required String nodePublicKey,
    List<String> capabilities = const [],
    String? bootstrapNetworkId,
  }) async {
    await runAction(() async {
      final result = await _deviceSetupService.registerNodeAndBootstrap(
        NodeRegistrationInput(
          deviceId: deviceId,
          nodeId: nodeId,
          nodePublicKey: nodePublicKey,
          bootstrapNetworkId: bootstrapNetworkId,
          capabilities: capabilities.isEmpty ? const ['desktop'] : capabilities,
        ),
        currentNetworks: sessionStore.networks,
      );
      sessionStore.node = result.node;
      if (result.bootstrap != null) {
        sessionStore.bootstrap = result.bootstrap;
        sessionStore.controlStatus = result.controlStatus;
        sessionStore.device = result.bootstrap!.device;
      }
      sessionStore.networks = result.networks;
    });
  }

  Future<String> ensureNetworkAvailableAndJoined({
    required DeviceModel currentDevice,
    required String preferredNetworkId,
    required String fallbackNetworkName,
    String fallbackCidr = defaultAutoNetworkCidr,
  }) async {
    late String targetNetworkId;
    await runAction(() async {
      final result = await _deviceSetupService.ensureNetworkAvailableAndJoined(
        device: currentDevice,
        currentNetworks: sessionStore.networks,
        preferredNetworkId: preferredNetworkId,
        fallbackNetworkName: fallbackNetworkName,
        fallbackCidr: fallbackCidr,
      );
      sessionStore.networks = result.networks;
      targetNetworkId = result.networkId;
    });
    return targetNetworkId;
  }

  Future<void> enableActiveNetwork() async {
    await runAction(() async {
      if (sessionStore.session == null) {
        throw StateError('login required');
      }
      if (sessionStore.device == null) {
        throw StateError('当前设备尚未注册完成。');
      }
      if (sessionStore.networks.isEmpty) {
        sessionStore.notice = '当前账号还没有可启用的网络，请先在网页端创建或接入网络。';
        return;
      }
      final runtime = await _workspaceService.prepareActiveNetworkRuntime(
        session: sessionStore.session,
        currentDevice: sessionStore.device,
        currentNode: sessionStore.node,
        currentNetworks: sessionStore.networks,
      );
      debugPrint(
        '[tunnel-enable] runtime ready activeNetwork=${runtime.activeNetwork.networkId} node=${runtime.node?.nodeId} device=${runtime.device?.deviceId}',
      );
      sessionStore.node = runtime.node;
      sessionStore.bootstrap = runtime.bootstrap;
      sessionStore.controlStatus = runtime.controlStatus;
      sessionStore.device = runtime.device;
      sessionStore.networks = runtime.networks;

      final config =
          _tunnelConfigurationService.buildActiveNetworkConfiguration(
        network: runtime.activeNetwork,
        deviceId: sessionStore.device!.deviceId,
        devicePublicKey: sessionStore.device!.publicKey,
      );
      final peerVirtualIp = config.peer.allowedIps.first.split('/').first;
      debugPrint(
        '[tunnel-enable] apply start peerVirtualIp=$peerVirtualIp listenPort=${config.interface.listenPort}',
      );
      final applyReport = await applyTunnelConfiguration(
        configuration: config,
        verifyPeerVirtualIp: peerVirtualIp,
      );
      debugPrint(
        '[tunnel-enable] apply done succeeded=${applyReport.succeeded} phase=${applyReport.phase} error=${applyReport.errorMessage}',
      );
      if (!applyReport.succeeded) {
        return;
      }
      debugPrint('[tunnel-enable] bring-up start peerVirtualIp=$peerVirtualIp');
      await bringTunnelUp(verifyPeerVirtualIp: peerVirtualIp);
      debugPrint('[tunnel-enable] bring-up done peerVirtualIp=$peerVirtualIp');
    });
  }

  Future<void> disableActiveNetwork() async {
    await runAction(() async {
      if (sessionStore.networks.isEmpty) {
        sessionStore.notice = '当前没有已接入的活动网络。';
        return;
      }
      await bringTunnelDown();
      final result = await _workspaceService.deactivateActiveNetwork(
        session: sessionStore.session,
        currentDevice: sessionStore.device,
        currentNetworks: sessionStore.networks,
      );
      final currentDevice = result.device;
      sessionStore.networks = result.networks;
      if (currentDevice != null) {
        sessionStore.device = DeviceModel(
          deviceId: currentDevice.deviceId,
          name: currentDevice.name,
          platform: currentDevice.platform,
          status: currentDevice.status,
          publicKey: currentDevice.publicKey,
        );
      }
      sessionStore.notice = '当前网络已停用，本地隧道和虚拟 IP 已释放。';
      sessionStore.bootstrap = null;
      sessionStore.controlStatus = null;
      sessionStore.clearConnection();
      tunnelStore.clearConnection();
      tunnelStore.lastTunnelActionReport = const TunnelActionReport(
        succeeded: true,
        detail: '当前网络已停用，本地隧道运行态已清空。',
        source: TunnelActionReportSource.runtimeSnapshot,
        phase: TunnelActionPhase.verified,
      );
    });
  }

  Future<void> loadBootstrap({
    String? nodeId,
    String? networkId,
  }) async {
    final targetNodeId =
        nodeId?.trim().isNotEmpty == true
            ? nodeId!.trim()
            : sessionStore.node?.nodeId;
    final targetNetworkId = networkId?.trim().isNotEmpty == true
        ? networkId!.trim()
        : sessionStore.networks.isNotEmpty
            ? sessionStore.networks.first.networkId
            : null;
    if (targetNodeId == null ||
        targetNodeId.isEmpty ||
        targetNetworkId == null) {
      error = 'nodeId and networkId are required';
      emitStateChanged();
      return;
    }
    await runAction(() async {
      sessionStore.bootstrap = await AppCoreScope.instance.bootstrap(
        nodeId: targetNodeId,
        networkId: targetNetworkId,
      );
      sessionStore.controlStatus = await AppCoreScope.instance.controlStatus();
      sessionStore.device = sessionStore.bootstrap!.device;
      sessionStore.networks = sessionStore.bootstrap!.networks;
    });
  }

  Future<String> refreshBootstrapOrControlSync() async {
    late String detail;
    await runAction(() async {
      final result = await _deviceRuntimeService.refreshBootstrap(
        node: sessionStore.node,
        currentBootstrap: sessionStore.bootstrap,
        currentNetworks: sessionStore.networks,
      );
      sessionStore.bootstrap = result.bootstrap;
      sessionStore.controlStatus = result.controlStatus;
      sessionStore.device = result.bootstrap.device;
      sessionStore.networks = result.bootstrap.networks;
      detail = result.detail;
    });
    return detail;
  }

  Future<ConnectAttemptResult> connectUsingControlPlan({
    required String networkId,
    required String peerNodeId,
    required String reason,
  }) async {
    late ConnectAttemptResult result;
    await runAction(() async {
      result = await _deviceRuntimeService.connectWithFallback(
        networkId: networkId,
        peerNodeId: peerNodeId,
        reason: reason,
        currentNode: sessionStore.node,
        controlStatus: sessionStore.controlStatus,
        lastProbe: lastProbe,
      );
      sessionStore.relayTicket = result.relayTicket;
      sessionStore.connectionState = result.connectionState;
    });
    return result;
  }

  Future<void> disconnect() async {
    await runAction(() async {
      await AppCoreScope.instance.disconnect();
      sessionStore.clearConnection();
      tunnelStore.clearConnection();
    });
  }

  Future<TunnelActionReport> applyTunnelConfiguration({
    required WireGuardTunnelConfiguration configuration,
    String? verifyPeerVirtualIp,
  }) async {
    final report = await runTunnelAction<TunnelActionReport>(() async {
      final result = await _tunnelRuntimeService.applyTunnelConfiguration(
        configuration: configuration,
        verifyPeerVirtualIp: verifyPeerVirtualIp,
      );
      tunnelStore.tunnelRuntimeView = result.runtimeView;
      return result.report;
    });
    tunnelStore.lastTunnelActionReport = report ?? _failedTunnelActionReport();
    return tunnelStore.lastTunnelActionReport!;
  }

  Future<TunnelActionReport> bringTunnelUp({
    String? verifyPeerVirtualIp,
  }) async {
    final report = await runTunnelAction<TunnelActionReport>(() async {
      final result = await _tunnelRuntimeService.bringTunnelUp(
        verifyPeerVirtualIp: verifyPeerVirtualIp,
      );
      tunnelStore.tunnelRuntimeView = result.runtimeView;
      return result.report;
    });
    tunnelStore.lastTunnelActionReport = report ?? _failedTunnelActionReport();
    return tunnelStore.lastTunnelActionReport!;
  }

  Future<TunnelActionReport> bringTunnelDown() async {
    final report = await runTunnelAction<TunnelActionReport>(() async {
      final result = await _tunnelRuntimeService.bringTunnelDown();
      tunnelStore.tunnelRuntimeView = result.runtimeView;
      return result.report;
    });
    tunnelStore.lastTunnelActionReport = report ?? _failedTunnelActionReport();
    return tunnelStore.lastTunnelActionReport!;
  }

  Future<TunnelActionReport> removeTunnelPeer({
    required String peerVirtualIp,
  }) async {
    final report = await runTunnelAction<TunnelActionReport>(() async {
      final result = await _tunnelRuntimeService.removeTunnelPeer(
        peerVirtualIp: peerVirtualIp,
      );
      tunnelStore.tunnelRuntimeView = result.runtimeView;
      return result.report;
    });
    tunnelStore.lastTunnelActionReport = report ?? _failedTunnelActionReport();
    return tunnelStore.lastTunnelActionReport!;
  }

  Future<TunnelActionReport> refreshTunnelRuntime({
    required String peerVirtualIp,
  }) async {
    final report = await runTunnelAction<TunnelActionReport>(() async {
      final result = await _tunnelRuntimeService.refreshTunnelRuntime(
        peerVirtualIp: peerVirtualIp,
      );
      tunnelStore.tunnelRuntimeView = result.runtimeView;
      return result.report;
    });
    tunnelStore.lastTunnelActionReport = report ?? _failedTunnelActionReport();
    return tunnelStore.lastTunnelActionReport!;
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
      runtimeSnapshot: tunnelStore.tunnelRuntimeView,
    );
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
