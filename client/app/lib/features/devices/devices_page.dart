import 'dart:async';
import 'dart:io';

import 'package:flutter/material.dart';
import 'package:slan_app_core_plugin/slan_app_core_plugin.dart';

import '../../application/control_plan_insights.dart';
import '../../application/tunnel_session_service.dart';
import '../../infra/app_core/api/dev_defaults.dart';
import '../../infra/app_core/models/bootstrap_models.dart';
import '../../infra/app_core/models/connection_models.dart';
import '../../infra/app_core/models/control_models.dart';
import '../../infra/app_core/models/diagnostic_models.dart';
import '../../infra/app_core/models/identity_models.dart';
import '../../infra/app_core/models/relay_models.dart';
import '../../infra/app_core/models/tunnel_action_models.dart';
import '../../infra/app_core/scope/app_core_scope.dart';
import '../../infra/app_core/store/app_session_controller.dart';
import '../../infra/app_core/store/app_session_store.dart';
import '../../infra/app_core/store/app_tunnel_controller.dart';
import '../../infra/app_core/store/app_tunnel_store.dart';
import '../../shared/desktop_platform.dart';
import '../../testing/app_test_keys.dart';
import '../shared/desktop_client_widgets.dart';

part 'devices_page_logic.dart';
part 'devices_page_runtime_panels.dart';
part 'devices_page_sections.dart';
part 'devices_page_tunnel_guidance.dart';
part 'devices_page_state_card.dart';
part 'devices_page_state_sections.dart';

class DevicesPage extends StatefulWidget {
  const DevicesPage({super.key});

  @override
  State<DevicesPage> createState() => _DevicesPageState();
}

class _DevicesPageState extends State<DevicesPage> {
  final TunnelSessionService _tunnelSessionService =
      const TunnelSessionService();
  final _nameController = TextEditingController();
  final _platformController =
      TextEditingController(text: DesktopPlatform.currentId);
  final _machineIdController = TextEditingController();
  final _publicKeyController = TextEditingController();
  final _nodeIdController = TextEditingController();
  final _nodePublicKeyController = TextEditingController();
  final _bootstrapNodeIdController = TextEditingController();
  final _networkIdController = TextEditingController(text: 'net-1');
  final _peerNodeIdController = TextEditingController(text: 'peer-node-1');
  final _reasonController = TextEditingController(text: 'timeout');
  final _sendPayloadController = TextEditingController(text: 'hello');
  final _probePayloadController = TextEditingController(text: 'hello');
  final _probeTimeoutController = TextEditingController(text: '25');
  final _tunnelLocalIpController = TextEditingController(text: '10.0.0.10');
  final _tunnelPeerIpController = TextEditingController(text: '10.0.0.2');
  final _tunnelPrivateKeyController =
      TextEditingController(text: 'debug-private-key');
  final _tunnelPublicKeyController =
      TextEditingController(text: 'debug-public-key');
  final _tunnelPeerPublicKeyController =
      TextEditingController(text: 'peer-debug-public-key');
  final _tunnelEndpointController =
      TextEditingController(text: kDevTunnelEndpoint);
  final _tunnelDebugEngineModeController = TextEditingController();
  _TunnelActionEvent? _activeTunnelAction;
  String? _connectionPlanHint;
  final List<_TunnelActionEvent> _recentTunnelActions = [];
  final List<_TunnelHealthSnapshot> _recentHealthSnapshots = [];
  Timer? _runtimeMonitorTimer;
  bool _runtimeMonitorEnabled = false;
  String? _localClientIp;

  @override
  void initState() {
    super.initState();
    _seedLocalIdentityDefaults();
    unawaited(_loadLocalClientIp());
  }

  @override
  void dispose() {
    _runtimeMonitorTimer?.cancel();
    _nameController.dispose();
    _platformController.dispose();
    _machineIdController.dispose();
    _publicKeyController.dispose();
    _nodeIdController.dispose();
    _nodePublicKeyController.dispose();
    _bootstrapNodeIdController.dispose();
    _networkIdController.dispose();
    _peerNodeIdController.dispose();
    _reasonController.dispose();
    _sendPayloadController.dispose();
    _probePayloadController.dispose();
    _probeTimeoutController.dispose();
    _tunnelLocalIpController.dispose();
    _tunnelPeerIpController.dispose();
    _tunnelPrivateKeyController.dispose();
    _tunnelPublicKeyController.dispose();
    _tunnelPeerPublicKeyController.dispose();
    _tunnelEndpointController.dispose();
    _tunnelDebugEngineModeController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final sessionController = AppCoreScope.sessionController;
    final tunnelController = AppCoreScope.tunnelController;
    final sessionStore = AppCoreScope.sessionStore;
    final tunnelStore = AppCoreScope.tunnelStore;
    return AnimatedBuilder(
      animation: Listenable.merge([sessionStore, tunnelStore]),
      builder: (context, _) {
        return LayoutBuilder(
          builder: (context, constraints) {
            final isDesktop = constraints.maxWidth >= 1180;
            final child = isDesktop
                ? _buildDesktopWorkspace(
                    context,
                    sessionStore,
                    tunnelStore,
                    sessionController,
                    tunnelController,
                  )
                : _buildCompactWorkspace(
                    context,
                    sessionStore,
                    tunnelStore,
                    sessionController,
                    tunnelController,
                  );
            return SingleChildScrollView(
              key: AppTestKeys.devicesScrollView,
              padding: const EdgeInsets.all(16),
              child: child,
            );
          },
        );
      },
    );
  }

  Widget _buildCompactWorkspace(
    BuildContext context,
    AppSessionStore sessionStore,
    AppTunnelStore tunnelStore,
    AppSessionController sessionController,
    AppTunnelController tunnelController,
  ) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        _buildPageIntro(context),
        const SizedBox(height: 16),
        _buildIdentitySection(context, sessionStore),
        const SizedBox(height: 16),
        _buildConnectivitySection(
          context,
          sessionStore,
          tunnelStore,
          sessionController,
        ),
        const SizedBox(height: 16),
        _buildDiagnosticsSection(
          context,
          sessionStore,
          tunnelStore,
          tunnelController,
        ),
        const SizedBox(height: 16),
        _buildTunnelSection(
          context,
          sessionStore,
          tunnelStore,
          sessionController: sessionController,
          tunnelController: tunnelController,
        ),
        const SizedBox(height: 16),
        _DeviceStateCard(
          device: sessionStore.device,
          devices: sessionStore.devices,
          node: sessionStore.node,
          bootstrap: sessionStore.bootstrap,
          controlStatus: sessionStore.controlStatus,
          relayTicket: sessionStore.relayTicket,
          lastSendBytes: tunnelStore.lastSendBytes,
          lastSendFailure: tunnelStore.lastSendFailure,
          lastProbe: tunnelStore.lastProbe,
          lastProbeFailure: tunnelStore.lastProbeFailure,
          connectionState: sessionStore.connectionState,
          tunnelRuntimeView: tunnelStore.tunnelRuntimeView,
          tunnelDebugError: tunnelStore.tunnelDebugError,
          error: sessionStore.error,
          activeAction: _activeTunnelAction,
          recentActions: _recentTunnelActions,
          recentHealthSnapshots: _recentHealthSnapshots,
          lastTunnelActionReport: tunnelStore.lastTunnelActionReport,
          runtimeMonitorEnabled: _runtimeMonitorEnabled,
          onBootstrap: () => _handleBootstrapRefresh(sessionController),
          onApply: () => _handleTunnelApply(tunnelController),
          onRecover: () => _handleRecoverSession(tunnelController),
          onUp: () => _handleTunnelUp(tunnelController),
          onInspect: () => _handleTunnelInspect(tunnelController),
          onDown: () => _handleTunnelDown(tunnelController),
          busy: sessionStore.busy,
        ),
      ],
    );
  }

  Widget _buildDesktopWorkspace(
    BuildContext context,
    AppSessionStore sessionStore,
    AppTunnelStore tunnelStore,
    AppSessionController sessionController,
    AppTunnelController tunnelController,
  ) {
    final runtime = tunnelStore.tunnelRuntimeView;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        _buildPageIntro(
          context,
          trailing: Wrap(
            spacing: 12,
            runSpacing: 12,
            children: [
              DesktopMetricPill(
                label: 'Connection',
                value: sessionStore.connectionState.status,
              ),
              DesktopMetricPill(
                label: 'Tunnel',
                value: runtime?.state ?? 'idle',
              ),
              DesktopMetricPill(
                label: 'Backend',
                value: runtime?.backendState ?? 'unavailable',
              ),
              DesktopMetricPill(
                label: 'Flow',
                value: _trafficValue(runtime),
              ),
            ],
          ),
          footer: Wrap(
            spacing: 10,
            runSpacing: 10,
            children: [
              DesktopBadge(
                label: sessionStore.busy ? 'pipeline busy' : 'pipeline idle',
              ),
              DesktopBadge(
                label:
                    'path ${sessionStore.connectionState.path?.name ?? 'unknown'}',
              ),
              DesktopBadge(
                label:
                    'peer ${runtime?.peerVirtualIp ?? _tunnelPeerIpController.text.trim()}',
              ),
            ],
          ),
        ),
        const SizedBox(height: 16),
        DesktopWorkspaceFrame(
          child: Padding(
            padding: const EdgeInsets.all(16),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                Row(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Expanded(
                      flex: 4,
                      child: Column(
                        children: [
                          _buildIdentitySection(context, sessionStore),
                          const SizedBox(height: 16),
                          _buildConnectivitySection(
                            context,
                            sessionStore,
                            tunnelStore,
                            sessionController,
                          ),
                        ],
                      ),
                    ),
                    const SizedBox(width: 16),
                    Expanded(
                      flex: 6,
                      child: Column(
                        children: [
                          _buildTunnelSection(
                            context,
                            sessionStore,
                            tunnelStore,
                            sessionController: sessionController,
                            tunnelController: tunnelController,
                            isDesktop: true,
                          ),
                          const SizedBox(height: 16),
                          _buildDiagnosticsSection(
                            context,
                            sessionStore,
                            tunnelStore,
                            tunnelController,
                          ),
                        ],
                      ),
                    ),
                    const SizedBox(width: 16),
                    SizedBox(
                      width: 380,
                      child: _DeviceStateCard(
                        device: sessionStore.device,
                        devices: sessionStore.devices,
                        node: sessionStore.node,
                        bootstrap: sessionStore.bootstrap,
                        controlStatus: sessionStore.controlStatus,
                        relayTicket: sessionStore.relayTicket,
                        lastSendBytes: tunnelStore.lastSendBytes,
                        lastSendFailure: tunnelStore.lastSendFailure,
                        lastProbe: tunnelStore.lastProbe,
                        lastProbeFailure: tunnelStore.lastProbeFailure,
                        connectionState: sessionStore.connectionState,
                        tunnelRuntimeView: tunnelStore.tunnelRuntimeView,
                        tunnelDebugError: tunnelStore.tunnelDebugError,
                        error: sessionStore.error,
                        activeAction: _activeTunnelAction,
                        recentActions: _recentTunnelActions,
                        recentHealthSnapshots: _recentHealthSnapshots,
                        lastTunnelActionReport: tunnelStore.lastTunnelActionReport,
                        runtimeMonitorEnabled: _runtimeMonitorEnabled,
                        onBootstrap: () =>
                            _handleBootstrapRefresh(sessionController),
                        onApply: () => _handleTunnelApply(tunnelController),
                        onRecover: () => _handleRecoverSession(tunnelController),
                        onUp: () => _handleTunnelUp(tunnelController),
                        onInspect: () => _handleTunnelInspect(tunnelController),
                        onDown: () => _handleTunnelDown(tunnelController),
                        busy: sessionStore.busy,
                      ),
                    ),
                  ],
                ),
              ],
            ),
          ),
        ),
      ],
    );
  }

  Future<void> _handleDeviceRegistration() async {
    final sessionController = AppCoreScope.sessionController;
    final sessionStore = AppCoreScope.sessionStore;
    final machineId = _machineIdController.text.trim().isEmpty
        ? _generatedMachineId()
        : _machineIdController.text.trim();
    final publicKey = _publicKeyController.text.trim().isEmpty
        ? _generatedPublicKey('device')
        : _publicKeyController.text.trim();
    _machineIdController.text = machineId;
    _publicKeyController.text = publicKey;
    await sessionController.registerDevice(
      name: _nameController.text.trim(),
      platform: _platformController.text.trim(),
      deviceVersion: DesktopPlatform.currentVersion,
      machineId: machineId,
      publicKey: publicKey,
    );
    if (!mounted || sessionStore.device == null) {
      return;
    }
    _nodeIdController.text = _generatedNodeId(sessionStore.device!.deviceId);
    _nodePublicKeyController.text = _generatedPublicKey('node');
  }

  Future<void> _handleNodeRegistration() async {
    final sessionController = AppCoreScope.sessionController;
    final sessionStore = AppCoreScope.sessionStore;
    final generatedNodeId = _nodeIdController.text.trim().isEmpty
        ? _generatedNodeId(sessionStore.device!.deviceId)
        : _nodeIdController.text.trim();
    final generatedNodePublicKey = _nodePublicKeyController.text.trim().isEmpty
        ? _generatedPublicKey('node')
        : _nodePublicKeyController.text.trim();
    _nodeIdController.text = generatedNodeId;
    _nodePublicKeyController.text = generatedNodePublicKey;
    final bootstrapNetworkId = _networkIdController.text.trim().isNotEmpty
        ? _networkIdController.text.trim()
        : sessionStore.networks.isNotEmpty
            ? sessionStore.networks.first.networkId
            : null;
    await sessionController.registerNode(
      deviceId: sessionStore.device!.deviceId,
      nodeId: generatedNodeId,
      nodePublicKey: generatedNodePublicKey,
      bootstrapNetworkId: bootstrapNetworkId,
    );
    if (!mounted) {
      return;
    }
    if (sessionStore.networks.isNotEmpty) {
      _networkIdController.text = sessionStore.networks.first.networkId;
    }
    _bootstrapNodeIdController.text =
        sessionStore.node?.nodeId ?? generatedNodeId;
    _syncTunnelDefaultsFromState();
  }

  Future<void> _handleQuickSetup() async {
    final sessionController = AppCoreScope.sessionController;
    final tunnelController = AppCoreScope.tunnelController;
    final sessionStore = AppCoreScope.sessionStore;
    final tunnelStore = AppCoreScope.tunnelStore;
    final startedAt = DateTime.now();

    void setSetupProgress({
      required String detail,
      required String progressLabel,
    }) {
      setState(() {
        _activeTunnelAction = _TunnelActionEvent(
          kind: _TunnelActionKind.setup,
          label: 'Quick setup client',
          detail: detail,
          status: _TunnelActionStatus.running,
          occurredAt: startedAt,
          progressLabel: progressLabel,
        );
      });
    }

    Future<void> finishSetup({
      String? failureMessage,
      String? successDetail,
    }) async {
      final completedEvent = _TunnelActionEvent(
        kind: _TunnelActionKind.setup,
        label: 'Quick setup client',
        detail: failureMessage ??
            successDetail ??
            'Device, node, bootstrap, and tunnel flow completed.',
        status: failureMessage == null
            ? _TunnelActionStatus.succeeded
            : _TunnelActionStatus.failed,
        occurredAt: DateTime.now(),
        progressLabel: failureMessage == null
            ? '5/5 complete'
            : 'Setup stopped before completion',
      );
      setState(() {
        _activeTunnelAction = completedEvent;
        _recentTunnelActions.insert(0, completedEvent);
        if (_recentTunnelActions.length > 6) {
          _recentTunnelActions.removeRange(6, _recentTunnelActions.length);
        }
        _captureHealthSnapshot();
      });
    }

    final finalReport = await _tunnelSessionService.quickSetup(
      onProgress: (update) {
        setSetupProgress(
          detail: update.detail,
          progressLabel: update.progressLabel,
        );
      },
      ensureDeviceRegistered: () async {
        if (sessionStore.device != null) {
          return;
        }
        await _handleDeviceRegistration();
      },
      ensureNetworkAvailableAndJoined: () async {
        final currentDevice = sessionStore.device;
        if (currentDevice == null) {
          return;
        }
        final targetNetworkId =
            await sessionController.ensureNetworkAvailableAndJoined(
          currentDevice: currentDevice,
          preferredNetworkId: _networkIdController.text.trim(),
          fallbackNetworkName: '${_nameController.text.trim()}-network',
        );
        _networkIdController.text = targetNetworkId;
      },
      ensureNodeRegisteredAndBootstrapped: () async {
        await _handleNodeRegistration();
      },
      applyConfiguration: () async {
        _syncTunnelDefaultsFromState();
        return tunnelController.applyTunnelConfiguration(
          configuration: _buildTunnelConfiguration(),
          verifyPeerVirtualIp: _tunnelPeerIpController.text.trim(),
        );
      },
      bringTunnelUp: () => tunnelController.bringTunnelUp(
        verifyPeerVirtualIp: _tunnelPeerIpController.text.trim(),
      ),
      readCurrentFailure: () => sessionStore.error ?? tunnelStore.tunnelDebugError,
    );

    final failureMessage = sessionStore.error ?? tunnelStore.tunnelDebugError;
    if (failureMessage != null && failureMessage.isNotEmpty) {
      await finishSetup(failureMessage: failureMessage);
      return;
    }
    if (finalReport != null && !finalReport.succeeded) {
      await finishSetup(
        failureMessage: finalReport.errorMessage ??
            _formatTunnelActionReportDetail(finalReport),
      );
      return;
    }

    await finishSetup(
      successDetail:
          'Client is ready. Device is registered, node is bootstrapped, and tunnel bring-up completed.',
    );
  }

  void _seedLocalIdentityDefaults() {
    final host = Platform.localHostname.replaceAll('.', '-');
    _nameController.text = host;
    _machineIdController.text = _generatedMachineId();
    _publicKeyController.text = _generatedPublicKey('device');
    _nodeIdController.text = _generatedNodeId(_machineIdController.text);
    _nodePublicKeyController.text = _generatedPublicKey('node');
  }

  Future<void> _loadLocalClientIp() async {
    try {
      final interfaces = await NetworkInterface.list(
        includeLoopback: false,
        type: InternetAddressType.IPv4,
      );
      for (final interface in interfaces) {
        for (final address in interface.addresses) {
          if (address.address.isNotEmpty) {
            if (!mounted) {
              return;
            }
            setState(() {
              _localClientIp = address.address;
            });
            return;
          }
        }
      }
    } catch (_) {
      return;
    }
  }

  String _generatedMachineId() {
    return AppCoreScope.clientMachineId;
  }

  String _generatedNodeId(String seed) {
    final normalized = seed.replaceAll(RegExp(r'[^a-zA-Z0-9-]'), '-');
    return 'node-$normalized';
  }

  String _generatedPublicKey(String prefix) {
    return '$prefix-key-${DateTime.now().microsecondsSinceEpoch}';
  }

  void _syncTunnelDefaultsFromState() {
    final sessionStore = AppCoreScope.sessionStore;
    final currentNetworkId = _networkIdController.text.trim();
    final network = sessionStore.networks
            .where((item) => item.networkId == currentNetworkId)
            .isNotEmpty
        ? sessionStore.networks
            .firstWhere((item) => item.networkId == currentNetworkId)
        : (sessionStore.networks.isNotEmpty ? sessionStore.networks.first : null);
    final selfDeviceId = sessionStore.device?.deviceId;
    String? localVirtualIp;
    if (selfDeviceId != null && network != null) {
      for (final member in network.members) {
        if (member.deviceId == selfDeviceId &&
            member.virtualIp != null &&
            member.virtualIp!.isNotEmpty) {
          localVirtualIp = member.virtualIp;
          break;
        }
      }
    }
    if (localVirtualIp != null && localVirtualIp.isNotEmpty) {
      _tunnelLocalIpController.text = localVirtualIp;
      _tunnelPeerIpController.text = _derivedPeerVirtualIp(localVirtualIp);
    }
    if (_localClientIp != null && _localClientIp!.isNotEmpty) {
      _tunnelEndpointController.text = '$_localClientIp:51820';
    }
  }

  String _derivedPeerVirtualIp(String localVirtualIp) {
    final parts = localVirtualIp.split('.');
    if (parts.length != 4) {
      return _tunnelPeerIpController.text.trim().isEmpty
          ? '10.0.0.2'
          : _tunnelPeerIpController.text.trim();
    }
    final last = int.tryParse(parts.last);
    if (last == null) {
      return _tunnelPeerIpController.text.trim().isEmpty
          ? '10.0.0.2'
          : _tunnelPeerIpController.text.trim();
    }
    final peerLast = last == 2 ? 3 : 2;
    return '${parts[0]}.${parts[1]}.${parts[2]}.$peerLast';
  }

  Future<void> _handleTunnelApply(
    AppTunnelController tunnelController,
  ) {
    return _runTunnelWorkbenchAction(
      kind: _TunnelActionKind.apply,
      label: 'Apply configuration',
      detail:
          'Stage ${_tunnelLocalIpController.text.trim()} -> ${_tunnelPeerIpController.text.trim()}',
      action: () => tunnelController.applyTunnelConfiguration(
        configuration: _buildTunnelConfiguration(),
        verifyPeerVirtualIp: _tunnelPeerIpController.text.trim(),
      ),
    );
  }

  Future<void> _handleTunnelUp(
    AppTunnelController tunnelController,
  ) {
    return _runTunnelWorkbenchAction(
      kind: _TunnelActionKind.up,
      label: 'Bring tunnel up',
      detail: 'Start PacketTunnel session',
      action: () => tunnelController.bringTunnelUp(
        verifyPeerVirtualIp: _tunnelPeerIpController.text.trim(),
      ),
    );
  }

  Future<void> _handleTunnelInspect(
    AppTunnelController tunnelController,
  ) {
    return _runTunnelWorkbenchAction(
      kind: _TunnelActionKind.inspect,
      label: 'Refresh runtime',
      detail: 'Inspect ${_tunnelPeerIpController.text.trim()} runtime view',
      action: () => tunnelController.refreshTunnelRuntime(
        peerVirtualIp: _tunnelPeerIpController.text.trim(),
      ),
    );
  }

  Future<void> _handleTunnelDown(
    AppTunnelController tunnelController,
  ) {
    return _runTunnelWorkbenchAction(
      kind: _TunnelActionKind.down,
      label: 'Bring tunnel down',
      detail: 'Stop PacketTunnel session',
      action: tunnelController.bringTunnelDown,
    );
  }

  Future<void> _handleTunnelRemovePeer(
    AppTunnelController tunnelController,
  ) {
    return _runTunnelWorkbenchAction(
      kind: _TunnelActionKind.removePeer,
      label: 'Remove peer',
      detail:
          'Delete peer ${_tunnelPeerIpController.text.trim()} from tunnel view',
      action: () => tunnelController.removeTunnelPeer(
        peerVirtualIp: _tunnelPeerIpController.text.trim(),
      ),
    );
  }

  Future<void> _handleBootstrapRefresh(
    AppSessionController sessionController,
  ) async {
    final sessionStore = AppCoreScope.sessionStore;
    final runningEvent = _TunnelActionEvent(
      kind: _TunnelActionKind.bootstrap,
      label: 'Refresh bootstrap',
      detail: 'Reload control-plane and relay bootstrap state',
      status: _TunnelActionStatus.running,
      occurredAt: DateTime.now(),
    );
    setState(() {
      _activeTunnelAction = runningEvent;
    });

    await sessionController.refreshBootstrapOrControlSync();

    final failureMessage = sessionStore.error;
    final completedEvent = _TunnelActionEvent(
      kind: _TunnelActionKind.bootstrap,
      label: 'Refresh bootstrap',
      detail: failureMessage ??
          'Bootstrap and relay metadata were refreshed from the control plane.',
      status: failureMessage == null
          ? _TunnelActionStatus.succeeded
          : _TunnelActionStatus.failed,
      occurredAt: DateTime.now(),
    );

    setState(() {
      _activeTunnelAction = completedEvent;
        _recentTunnelActions.insert(0, completedEvent);
        if (_recentTunnelActions.length > 6) {
          _recentTunnelActions.removeRange(6, _recentTunnelActions.length);
        }
        _captureHealthSnapshot();
      });
  }

  Future<void> _handleConnect(
    AppSessionController sessionController,
  ) async {
    final sessionStore = AppCoreScope.sessionStore;
    final peerNodeId = _peerNodeIdController.text.trim();
    final networkId = _networkIdController.text.trim();
    if (networkId.isEmpty) {
      setState(() {
        _connectionPlanHint = 'No target network is selected.';
      });
      return;
    }

    setState(() {
      _connectionPlanHint =
          'Resolving control-plane connect plan for $peerNodeId.';
    });

    final result = await sessionController.connectUsingControlPlan(
      networkId: networkId,
      peerNodeId: peerNodeId,
      reason: _reasonController.text.trim(),
    );
    if (!mounted) {
      return;
    }
    setState(() {
      _connectionPlanHint = sessionStore.error ?? result.resultHint;
    });
  }

  Future<void> _handleRecoverSession(
    AppTunnelController tunnelController,
  ) {
    final sessionStore = AppCoreScope.sessionStore;
    final tunnelStore = AppCoreScope.tunnelStore;
    final startedAt = DateTime.now();

    void setRecoveryProgress({
      required String detail,
      required String progressLabel,
    }) {
      setState(() {
        _activeTunnelAction = _TunnelActionEvent(
          kind: _TunnelActionKind.recover,
          label: 'Recover session',
          detail: detail,
          status: _TunnelActionStatus.running,
          occurredAt: startedAt,
          progressLabel: progressLabel,
        );
      });
    }

    Future<void> finishRecovery(TunnelActionReport? finalReport) async {
      final failureMessage =
          finalReport?.errorMessage ??
              tunnelStore.tunnelDebugError ??
              sessionStore.error;
      final automationNote = failureMessage == null
          ? await _applyTunnelLifecycleAutomation(_TunnelActionKind.recover)
          : null;
      final completedEvent = _TunnelActionEvent(
        kind: _TunnelActionKind.recover,
        label: 'Recover session',
        detail: failureMessage ??
            '${_formatTunnelActionReportDetail(finalReport)}${automationNote == null ? '' : ' $automationNote'}',
        status: failureMessage == null
            ? _TunnelActionStatus.succeeded
            : _TunnelActionStatus.failed,
        occurredAt: DateTime.now(),
        progressLabel: failureMessage == null
            ? '3/3 complete'
            : 'Recovery stopped before completion',
      );
      setState(() {
        _activeTunnelAction = completedEvent;
        _recentTunnelActions.insert(0, completedEvent);
        if (_recentTunnelActions.length > 6) {
          _recentTunnelActions.removeRange(6, _recentTunnelActions.length);
        }
        _captureHealthSnapshot();
      });
    }

    return Future<void>(() async {
      final finalReport = await _tunnelSessionService.recoverSession(
        onProgress: (update) {
          setRecoveryProgress(
            detail: update.detail,
            progressLabel: update.progressLabel,
          );
        },
        applyConfiguration: () => tunnelController.applyTunnelConfiguration(
          configuration: _buildTunnelConfiguration(),
          verifyPeerVirtualIp: _tunnelPeerIpController.text.trim(),
        ),
        bringTunnelUp: () => tunnelController.bringTunnelUp(
          verifyPeerVirtualIp: _tunnelPeerIpController.text.trim(),
        ),
        inspectRuntime: () => tunnelController.refreshTunnelRuntime(
          peerVirtualIp: _tunnelPeerIpController.text.trim(),
        ),
      );
      await finishRecovery(finalReport);
    });
  }

  void _setRuntimeMonitorEnabled(bool enabled) {
    if (_runtimeMonitorEnabled == enabled) {
      return;
    }

    setState(() {
      _runtimeMonitorEnabled = enabled;
    });

    _runtimeMonitorTimer?.cancel();
    _runtimeMonitorTimer = null;

    if (!enabled) {
      return;
    }

    _refreshRuntimeFromMonitor();
    _runtimeMonitorTimer = Timer.periodic(
      const Duration(seconds: 4),
      (_) => _refreshRuntimeFromMonitor(),
    );
  }

  Future<void> _refreshRuntimeFromMonitor() async {
    final tunnelController = AppCoreScope.tunnelController;
    final sessionStore = AppCoreScope.sessionStore;
    final peerVirtualIp = _tunnelPeerIpController.text.trim();
    if (!_runtimeMonitorEnabled ||
        peerVirtualIp.isEmpty ||
        sessionStore.busy ||
        !mounted) {
      return;
    }

    await tunnelController.refreshTunnelRuntime(peerVirtualIp: peerVirtualIp);
    if (!mounted) {
      return;
    }
    setState(() {
      _captureHealthSnapshot();
    });
  }

  Future<void> _runTunnelWorkbenchAction(
   {
    required _TunnelActionKind kind,
    required String label,
    required String detail,
    required Future<TunnelActionReport> Function() action,
  }) async {
    final sessionStore = AppCoreScope.sessionStore;
    final tunnelStore = AppCoreScope.tunnelStore;
    final runningEvent = _TunnelActionEvent(
      kind: kind,
      label: label,
      detail: detail,
      status: _TunnelActionStatus.running,
      occurredAt: DateTime.now(),
    );
    setState(() {
      _activeTunnelAction = runningEvent;
    });

    final report = await action();

    final failureMessage =
        report.errorMessage ?? tunnelStore.tunnelDebugError ?? sessionStore.error;
    final automationNote = failureMessage == null
        ? await _applyTunnelLifecycleAutomation(kind)
        : null;
    final completedEvent = _TunnelActionEvent(
      kind: kind,
      label: label,
      detail: failureMessage ??
          '${_formatTunnelActionReportDetail(report)}${automationNote == null ? '' : ' $automationNote'}',
      status: failureMessage == null
          ? _TunnelActionStatus.succeeded
          : _TunnelActionStatus.failed,
      occurredAt: DateTime.now(),
      signalSourceLabel: report.sourceLabel,
    );

    setState(() {
      _activeTunnelAction = completedEvent;
      _recentTunnelActions.insert(0, completedEvent);
      if (_recentTunnelActions.length > 6) {
        _recentTunnelActions.removeRange(6, _recentTunnelActions.length);
      }
      _captureHealthSnapshot();
    });
  }

  String _formatTunnelActionReportDetail(TunnelActionReport? report) {
    if (report == null) {
      return 'Tunnel action completed without a structured backend report.';
    }
    return '${report.detail} Verified by ${report.sourceLabel}.';
  }

  void _captureHealthSnapshot() {
    final sessionStore = AppCoreScope.sessionStore;
    final tunnelStore = AppCoreScope.tunnelStore;
    final summary = _deriveTunnelSessionHealthSummary(
      runtime: tunnelStore.tunnelRuntimeView,
      controlStatus: sessionStore.controlStatus,
      connectionState: sessionStore.connectionState,
      lastProbe: tunnelStore.lastProbe,
      tunnelDebugError: tunnelStore.tunnelDebugError,
      error: sessionStore.error,
      runtimeMonitorEnabled: _runtimeMonitorEnabled,
      recentActions: _recentTunnelActions,
      recentHealthSnapshots: _recentHealthSnapshots,
      lastSendFailure: tunnelStore.lastSendFailure,
      lastProbeFailure: tunnelStore.lastProbeFailure,
    );
    final snapshot = _TunnelHealthSnapshot(
      health: summary.health,
      occurredAt: DateTime.now(),
    );
    if (_recentHealthSnapshots.isNotEmpty &&
        _recentHealthSnapshots.first.health == snapshot.health) {
      _recentHealthSnapshots[0] = snapshot;
    } else {
      _recentHealthSnapshots.insert(0, snapshot);
    }
    if (_recentHealthSnapshots.length > 8) {
      _recentHealthSnapshots.removeRange(8, _recentHealthSnapshots.length);
    }
  }

  Future<String?> _applyTunnelLifecycleAutomation(
    _TunnelActionKind kind,
  ) async {
    switch (kind) {
      case _TunnelActionKind.setup:
        return 'Configuration changed. Continue observing dashboard and runtime state.';
      case _TunnelActionKind.apply:
        return _runtimeMonitorEnabled
            ? 'Configuration changed. Bring the tunnel up next, then re-observe runtime because live monitor is still running.'
            : 'Configuration changed. Bring the tunnel up next to activate the new settings.';
      case _TunnelActionKind.bootstrap:
        return 'Bootstrap and relay metadata refreshed from the control plane.';
      case _TunnelActionKind.up:
        final shouldStartMonitor = !_runtimeMonitorEnabled;
        _setRuntimeMonitorEnabled(true);
        await _refreshRuntimeFromMonitor();
        return shouldStartMonitor
            ? 'Live runtime monitor started automatically.'
            : 'Live runtime monitor kept running.';
      case _TunnelActionKind.down:
      case _TunnelActionKind.removePeer:
        final wasMonitoring = _runtimeMonitorEnabled;
        _setRuntimeMonitorEnabled(false);
        return wasMonitoring
            ? 'Live runtime monitor stopped automatically.'
            : null;
      case _TunnelActionKind.inspect:
        return null;
      case _TunnelActionKind.recover:
        final shouldStartMonitor = !_runtimeMonitorEnabled;
        _setRuntimeMonitorEnabled(true);
        return shouldStartMonitor
            ? 'Recovery finished and live runtime monitor started automatically.'
            : 'Recovery finished while live runtime monitor stayed active.';
    }
  }

  WireGuardTunnelConfiguration _buildTunnelConfiguration() {
    final localVirtualIp = _tunnelLocalIpController.text.trim();
    final peerVirtualIp = _tunnelPeerIpController.text.trim();
    final debugEngineMode = _tunnelDebugEngineModeController.text.trim();
    final interfacePrefix = _activeNetworkPrefixLength();
    return WireGuardTunnelConfiguration(
      transport: 'relay',
      localVirtualIp: localVirtualIp,
      peerVirtualIp: peerVirtualIp,
      debugEngineMode: debugEngineMode.isEmpty ? null : debugEngineMode,
      interface: WireGuardTunnelInterfaceConfiguration(
        keyPair: WireGuardTunnelKeyPair(
          privateKey: _tunnelPrivateKeyController.text.trim(),
          publicKey: _tunnelPublicKeyController.text.trim(),
        ),
        listenPort: 51820,
        mtu: 1280,
        addresses: ['$localVirtualIp/$interfacePrefix'],
        dnsServers: const ['1.1.1.1'],
      ),
      peer: WireGuardTunnelPeerConfiguration(
        publicKey: _tunnelPeerPublicKeyController.text.trim(),
        endpoint: _tunnelEndpointController.text.trim(),
        allowedIps: ['$peerVirtualIp/32'],
      ),
    );
  }

  int _activeNetworkPrefixLength() {
    final sessionStore = AppCoreScope.sessionStore;
    final currentNetworkId = _networkIdController.text.trim();
    final network = sessionStore.networks
            .where((item) => item.networkId == currentNetworkId)
            .isNotEmpty
        ? sessionStore.networks.firstWhere(
            (item) => item.networkId == currentNetworkId,
          )
        : (sessionStore.networks.isNotEmpty
            ? sessionStore.networks.first
            : null);
    final cidr = network?.cidr.trim() ?? '';
    final slash = cidr.lastIndexOf('/');
    if (slash <= 0 || slash == cidr.length - 1) {
      return 32;
    }
    final parsed = int.tryParse(cidr.substring(slash + 1).trim());
    if (parsed == null || parsed < 0 || parsed > 32) {
      return 32;
    }
    return parsed;
  }
}

class _DevicesWorkbenchCard extends StatelessWidget {
  const _DevicesWorkbenchCard({
    required this.title,
    required this.subtitle,
    required this.child,
  });

  final String title;
  final String subtitle;
  final Widget child;

  @override
  Widget build(BuildContext context) {
    return DesktopSurfaceCard(
      title: title,
      subtitle: subtitle,
      child: child,
    );
  }
}

class _DevicesSubsection extends StatelessWidget {
  const _DevicesSubsection({
    required this.title,
    required this.child,
  });

  final String title;
  final Widget child;

  @override
  Widget build(BuildContext context) {
    return DesktopInsetBlock(
      title: title,
      child: child,
    );
  }
}

