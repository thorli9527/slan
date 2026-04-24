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

  Widget _buildPageIntro(
    BuildContext context, {
    Widget? trailing,
    Widget? footer,
  }) {
    return DesktopHeroPanel(
      title: 'Devices Workspace',
      description:
          'Register a local device, attach a node, bootstrap overlay state, drive relay fallback, and inspect ${DesktopPlatform.nativeTunnelLabel} runtime without leaving the desktop client.',
      trailing: trailing,
      footer: DesktopPlatform.supportsNativeTunnel
          ? footer
          : Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                const Text(
                  'Windows client is enabled for login, device, node, bootstrap, and control-plane flows.',
                ),
                const SizedBox(height: 8),
                Text(
                  'Local tunnel runtime actions are routed through the Rust backend on ${DesktopPlatform.nativeTunnelLabel}, so apply, bring-up, and runtime inspection can now run through the desktop bridge.',
                ),
                if (footer != null) ...[
                  const SizedBox(height: 12),
                  footer,
                ],
              ],
            ),
    );
  }

  Widget _buildIdentitySection(
    BuildContext context,
    AppSessionStore sessionStore,
  ) {
    final sessionController = AppCoreScope.sessionController;
    return _DevicesWorkbenchCard(
      title: 'Identity',
      subtitle: 'Device registration, node registration, and bootstrap.',
      child: Column(
        children: [
          _DevicesSubsection(
            title: 'Register Device',
            child: Column(
              children: [
                if (_localClientIp != null) ...[
                  Align(
                    alignment: Alignment.centerLeft,
                    child: DesktopBadge(
                      label: 'current client ip $_localClientIp',
                    ),
                  ),
                  const SizedBox(height: 12),
                ],
                TextField(
                  key: AppTestKeys.devicesNameField,
                  controller: _nameController,
                  decoration: const InputDecoration(labelText: 'Name'),
                ),
                const SizedBox(height: 12),
                TextField(
                  key: AppTestKeys.devicesPlatformField,
                  controller: _platformController,
                  decoration: const InputDecoration(labelText: 'Platform'),
                ),
                const SizedBox(height: 12),
                TextField(
                  key: AppTestKeys.devicesMachineIdField,
                  controller: _machineIdController,
                  decoration: const InputDecoration(labelText: 'Machine ID'),
                ),
                const SizedBox(height: 12),
                TextField(
                  key: AppTestKeys.devicesPublicKeyField,
                  controller: _publicKeyController,
                  decoration: const InputDecoration(labelText: 'Public Key'),
                ),
                const SizedBox(height: 16),
                Align(
                  alignment: Alignment.centerLeft,
                  child: Wrap(
                    spacing: 12,
                    runSpacing: 12,
                    children: [
                      FilledButton(
                        key: AppTestKeys.devicesRegisterDeviceButton,
                        onPressed: sessionStore.busy
                            ? null
                            : _handleDeviceRegistration,
                        child: const Text('Register Device'),
                      ),
                      FilledButton.tonal(
                        onPressed: sessionStore.busy ? null : _handleQuickSetup,
                        child: const Text('Quick Setup Client'),
                      ),
                    ],
                  ),
                ),
              ],
            ),
          ),
          const SizedBox(height: 16),
          _DevicesSubsection(
            title: 'Register Node',
            child: Column(
              children: [
                TextField(
                  key: AppTestKeys.devicesNodeIdField,
                  controller: _nodeIdController,
                  decoration: const InputDecoration(
                    labelText: 'Node ID',
                    hintText: 'Leave empty to auto-generate',
                  ),
                ),
                const SizedBox(height: 12),
                TextField(
                  key: AppTestKeys.devicesNodePublicKeyField,
                  controller: _nodePublicKeyController,
                  decoration:
                      const InputDecoration(labelText: 'Node Public Key'),
                ),
                const SizedBox(height: 16),
                Align(
                  alignment: Alignment.centerLeft,
                  child: OutlinedButton(
                    key: AppTestKeys.devicesRegisterNodeButton,
                    onPressed: sessionStore.busy || sessionStore.device == null
                        ? null
                        : _handleNodeRegistration,
                    child: const Text('Register Node'),
                  ),
                ),
              ],
            ),
          ),
          const SizedBox(height: 16),
          _DevicesSubsection(
            title: 'Bootstrap',
            child: Column(
              children: [
                TextField(
                  key: AppTestKeys.devicesBootstrapNodeIdField,
                  controller: _bootstrapNodeIdController,
                  decoration: const InputDecoration(
                    labelText: 'Node ID',
                    hintText: 'Leave empty to use current node',
                  ),
                ),
                const SizedBox(height: 12),
                TextField(
                  key: AppTestKeys.devicesBootstrapNetworkIdField,
                  controller: _networkIdController,
                  decoration: const InputDecoration(labelText: 'Network ID'),
                ),
                const SizedBox(height: 16),
                Align(
                  alignment: Alignment.centerLeft,
                  child: OutlinedButton(
                    key: AppTestKeys.devicesLoadBootstrapButton,
                    onPressed: sessionStore.busy
                        ? null
                        : () => sessionController.loadBootstrap(
                              nodeId: _bootstrapNodeIdController.text,
                              networkId: _networkIdController.text,
                            ),
                    child: const Text('Load Bootstrap'),
                  ),
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildConnectivitySection(
    BuildContext context,
    AppSessionStore sessionStore,
    AppTunnelStore tunnelStore,
    AppSessionController sessionController,
  ) {
    final peerNodeId = _peerNodeIdController.text.trim();
    final controlPlan = connectPlanForPeer(sessionStore.controlStatus, peerNodeId);
    final recommendationMatch = connectRecommendationMatchLabel(
      controlPlan: controlPlan,
      connectionState: sessionStore.connectionState,
      lastProbe: tunnelStore.lastProbe,
    );
    return _DevicesWorkbenchCard(
      title: 'Connectivity',
      subtitle: 'Drive direct connect, relay fallback, and disconnect.',
      child: Column(
        children: [
          const Align(
            alignment: Alignment.centerLeft,
            child: Text(
              'Use peer node id starting with fail- to simulate P2P failure and trigger relay fallback.',
            ),
          ),
          const SizedBox(height: 12),
          TextField(
            key: AppTestKeys.devicesPeerNodeIdField,
            controller: _peerNodeIdController,
            decoration: const InputDecoration(labelText: 'Peer Node ID'),
          ),
          const SizedBox(height: 12),
          TextField(
            key: AppTestKeys.devicesFailureReasonField,
            controller: _reasonController,
            decoration: const InputDecoration(labelText: 'Failure Reason'),
          ),
          const SizedBox(height: 16),
          Wrap(
            spacing: 12,
            runSpacing: 12,
            children: [
              FilledButton(
                key: AppTestKeys.devicesConnectButton,
                onPressed: sessionStore.busy
                    ? null
                    : () => _handleConnect(sessionController),
                child: const Text('Connect'),
              ),
              OutlinedButton(
                key: AppTestKeys.devicesDisconnectButton,
                onPressed: sessionStore.busy ? null : sessionController.disconnect,
                child: const Text('Disconnect'),
              ),
            ],
          ),
          const SizedBox(height: 16),
          _DevicesSubsection(
            title: 'Connection Guidance',
            child: DesktopKeyValueList(
              entries: [
                DesktopKeyValueEntry(
                  label: 'Control plan',
                  value: controlPlan == null
                      ? 'none'
                      : describeConnectPlan(controlPlan),
                ),
                DesktopKeyValueEntry(
                  label: 'Preferred relay',
                  value: controlPlan == null
                      ? '-'
                      : (controlPlan.preferredDerpNodeIds.isNotEmpty
                          ? controlPlan.preferredDerpNodeIds.first
                          : controlPlan.derpClusterId ?? '-'),
                ),
                DesktopKeyValueEntry(
                  label: 'Latest connect hint',
                  value: _connectionPlanHint ?? 'none',
                ),
                DesktopKeyValueEntry(
                  label: 'Recommendation match',
                  value: recommendationMatch,
                ),
              ],
            ),
          ),
        ],
      ),
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
    final host = Platform.localHostname.replaceAll('.', '-');
    return '$host-${DateTime.now().millisecondsSinceEpoch}';
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

  Widget _buildDiagnosticsSection(
    BuildContext context,
    AppSessionStore sessionStore,
    AppTunnelStore tunnelStore,
    AppTunnelController tunnelController,
  ) {
    return _DevicesWorkbenchCard(
      title: 'Diagnostics',
      subtitle:
          'Exercise send/probe and keep recent data-plane checks close to the tunnel controls.',
      child: Column(
        children: [
          _DevicesSubsection(
            title: 'Send',
            child: Column(
              children: [
                TextField(
                  key: AppTestKeys.devicesSendPayloadField,
                  controller: _sendPayloadController,
                  decoration: const InputDecoration(labelText: 'Payload'),
                ),
                const SizedBox(height: 16),
                Align(
                  alignment: Alignment.centerLeft,
                  child: OutlinedButton(
                    key: AppTestKeys.devicesSendButton,
                    onPressed: sessionStore.busy
                        ? null
                        : () => tunnelController.send(
                              payload: _sendPayloadController.text,
                            ),
                    child: const Text('Send'),
                  ),
                ),
              ],
            ),
          ),
          const SizedBox(height: 16),
          _DevicesSubsection(
            title: 'Probe',
            child: Column(
              children: [
                TextField(
                  key: AppTestKeys.devicesProbePayloadField,
                  controller: _probePayloadController,
                  decoration: const InputDecoration(labelText: 'Payload'),
                ),
                const SizedBox(height: 12),
                TextField(
                  key: AppTestKeys.devicesProbeTimeoutField,
                  controller: _probeTimeoutController,
                  decoration:
                      const InputDecoration(labelText: 'Probe Timeout (ms)'),
                ),
                const SizedBox(height: 16),
                Align(
                  alignment: Alignment.centerLeft,
                  child: OutlinedButton(
                    key: AppTestKeys.devicesProbeButton,
                    onPressed: sessionStore.busy
                        ? null
                        : () => tunnelController.probe(
                              payload: _probePayloadController.text,
                              probeTimeoutMs: int.tryParse(
                                  _probeTimeoutController.text.trim()),
                            ),
                    child: const Text('Probe'),
                  ),
                ),
              ],
            ),
          ),
          const SizedBox(height: 16),
          _DevicesSubsection(
            title: 'Platform',
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Wrap(
                  spacing: 10,
                  runSpacing: 10,
                  children: [
                    OutlinedButton(
                      key: AppTestKeys.devicesPlatformDoctorButton,
                      onPressed: sessionStore.busy
                          ? null
                          : () => tunnelController.refreshPlatformDoctor(),
                      child: const Text('Run Doctor'),
                    ),
                    OutlinedButton(
                      key: AppTestKeys.devicesPlatformInstallPlanButton,
                      onPressed: sessionStore.busy
                          ? null
                          : () =>
                              tunnelController.refreshPlatformInstallPlan(),
                      child: const Text('Install Plan'),
                    ),
                  ],
                ),
                const SizedBox(height: 16),
                DesktopKeyValueList(
                  entries: [
                    DesktopKeyValueEntry(
                      label: 'Platform',
                      value: _formatPlatformSummary(tunnelStore.platformDoctor
                              ?.platform ??
                          tunnelStore.platformInstallPlan?.platform),
                    ),
                    DesktopKeyValueEntry(
                      label: 'Tunnel backend',
                      value: _formatBackendSummary(
                        tunnelStore.platformDoctor?.tunnelBackend,
                      ),
                    ),
                    DesktopKeyValueEntry(
                      label: 'Checks',
                      value: _formatPlatformChecks(
                        tunnelStore.platformDoctor?.checks ?? const [],
                      ),
                    ),
                    DesktopKeyValueEntry(
                      label: 'Packages',
                      value:
                          tunnelStore.platformInstallPlan?.packages.join(', ') ??
                              '-',
                    ),
                    DesktopKeyValueEntry(
                      label: 'Driver modes',
                      value: tunnelStore.platformInstallPlan
                              ?.supportedDriverModes
                              .join(', ') ??
                          '-',
                    ),
                    DesktopKeyValueEntry(
                      label: 'Warnings',
                      value:
                          tunnelStore.platformInstallPlan?.warnings.join('; ') ??
                              '-',
                    ),
                  ],
                ),
              ],
            ),
          ),
          const SizedBox(height: 16),
          _DevicesSubsection(
            title: 'Latest Diagnostics',
            child: DesktopKeyValueList(
              entries: [
                DesktopKeyValueEntry(
                  label: 'Send bytes',
                  value: tunnelStore.lastSendBytes?.toString() ?? '-',
                ),
                DesktopKeyValueEntry(
                  label: 'Send failure',
                  value: tunnelStore.lastSendFailure?.label ?? '-',
                ),
                DesktopKeyValueEntry(
                  label: 'Probe',
                  value: tunnelStore.lastProbe?.probeId ?? 'none',
                ),
                DesktopKeyValueEntry(
                  label: 'Probe RTT',
                  value: tunnelStore.lastProbe?.replyRttMs?.toString() ?? '-',
                ),
                DesktopKeyValueEntry(
                  label: 'Probe failure',
                  value: tunnelStore.lastProbeFailure?.label ?? '-',
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }

  String _formatPlatformSummary(PlatformInfoModel? platform) {
    if (platform == null) {
      return '-';
    }
    final parts = [
      platform.os,
      if (platform.distroId != null && platform.distroId!.isNotEmpty)
        platform.distroId!,
      if (platform.versionId != null && platform.versionId!.isNotEmpty)
        platform.versionId!,
      if (platform.family != null && platform.family!.isNotEmpty)
        'family ${platform.family}',
      if (platform.packageManager != null &&
          platform.packageManager!.isNotEmpty)
        'pkg ${platform.packageManager}',
    ];
    return parts.join(' / ');
  }

  String _formatBackendSummary(TunnelBackendDiagnosticsModel? backend) {
    if (backend == null) {
      return '-';
    }
    final mode = backend.executionMode ?? '-';
    final executor = backend.executionBackend ?? '-';
    final interfaceName = backend.interfaceName ?? '-';
    return '${backend.name} / $mode / $executor / $interfaceName / peers ${backend.plannedPeerCount}';
  }

  String _formatPlatformChecks(List<PlatformCheckModel> checks) {
    if (checks.isEmpty) {
      return '-';
    }
    return checks
        .map((check) => '${check.name}:${check.status}')
        .join(', ');
  }

  Widget _buildTunnelSection(
    BuildContext context,
    AppSessionStore sessionStore,
    AppTunnelStore tunnelStore, {
    required AppSessionController sessionController,
    required AppTunnelController tunnelController,
    bool isDesktop = false,
  }) {
    final actionButtons =
        _buildTunnelActionButtons(sessionStore, tunnelController);
    return _DevicesWorkbenchCard(
      title: 'WireGuard Tunnel Debug',
      subtitle:
          'Prepare PacketTunnel configuration, drive lifecycle actions, and keep the current backend/runtime state in one desktop control surface.',
      child: Column(
        children: [
          if (isDesktop)
            Column(
              children: [
                Row(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Expanded(
                      child: Column(
                        children: [
                          _DevicesSubsection(
                            title: 'Topology',
                            child: Column(
                              children: [
                                TextField(
                                  key: AppTestKeys.devicesTunnelLocalIpField,
                                  controller: _tunnelLocalIpController,
                                  decoration: const InputDecoration(
                                    labelText: 'Local Virtual IP',
                                  ),
                                ),
                                const SizedBox(height: 12),
                                TextField(
                                  key: AppTestKeys.devicesTunnelPeerIpField,
                                  controller: _tunnelPeerIpController,
                                  decoration: const InputDecoration(
                                    labelText: 'Peer Virtual IP',
                                  ),
                                ),
                                const SizedBox(height: 12),
                                TextField(
                                  key: AppTestKeys.devicesTunnelEndpointField,
                                  controller: _tunnelEndpointController,
                                  decoration: const InputDecoration(
                                    labelText: 'Peer Endpoint',
                                  ),
                                ),
                              ],
                            ),
                          ),
                          const SizedBox(height: 16),
                          _DevicesSubsection(
                            title: 'Keys',
                            child: Column(
                              children: [
                                TextField(
                                  key: AppTestKeys.devicesTunnelPrivateKeyField,
                                  controller: _tunnelPrivateKeyController,
                                  decoration: const InputDecoration(
                                    labelText: 'Interface Private Key',
                                  ),
                                ),
                                const SizedBox(height: 12),
                                TextField(
                                  key: AppTestKeys.devicesTunnelPublicKeyField,
                                  controller: _tunnelPublicKeyController,
                                  decoration: const InputDecoration(
                                    labelText: 'Interface Public Key',
                                  ),
                                ),
                                const SizedBox(height: 12),
                                TextField(
                                  key: AppTestKeys
                                      .devicesTunnelPeerPublicKeyField,
                                  controller: _tunnelPeerPublicKeyController,
                                  decoration: const InputDecoration(
                                    labelText: 'Peer Public Key',
                                  ),
                                ),
                              ],
                            ),
                          ),
                        ],
                      ),
                    ),
                    const SizedBox(width: 16),
                    Expanded(
                      child: Column(
                        children: [
                          _DevicesSubsection(
                            title: 'Engine & Actions',
                            child: Column(
                              crossAxisAlignment: CrossAxisAlignment.start,
                              children: [
                                TextField(
                                  key: AppTestKeys
                                      .devicesTunnelDebugEngineModeField,
                                  controller: _tunnelDebugEngineModeController,
                                  decoration: const InputDecoration(
                                    labelText: 'Debug Engine Mode',
                                    hintText: 'noop, loopback, or external',
                                  ),
                                ),
                                const SizedBox(height: 16),
                                actionButtons,
                              ],
                            ),
                          ),
                          const SizedBox(height: 16),
                          _DevicesSubsection(
                            title: 'Operations Desk',
                            child: _TunnelOperationsDesk(
                              tunnelRuntimeView: tunnelStore.tunnelRuntimeView,
                              controlStatus: sessionStore.controlStatus,
                              tunnelDebugError: tunnelStore.tunnelDebugError,
                              error: sessionStore.error,
                              busy: sessionStore.busy,
                              connectionState: sessionStore.connectionState,
                              peerVirtualIp:
                                  _tunnelPeerIpController.text.trim(),
                              activeAction: _activeTunnelAction,
                              recentActions: _recentTunnelActions,
                              recentHealthSnapshots: _recentHealthSnapshots,
                              lastTunnelActionReport:
                                  tunnelStore.lastTunnelActionReport,
                              lastProbe: tunnelStore.lastProbe,
                              lastSendFailure: tunnelStore.lastSendFailure,
                              lastProbeFailure: tunnelStore.lastProbeFailure,
                              onRecover: () =>
                                  _handleRecoverSession(tunnelController),
                              onBootstrap: () =>
                                  _handleBootstrapRefresh(sessionController),
                              onApply: () =>
                                  _handleTunnelApply(tunnelController),
                              onUp: () => _handleTunnelUp(tunnelController),
                              onInspect: () =>
                                  _handleTunnelInspect(tunnelController),
                              onDown: () => _handleTunnelDown(tunnelController),
                              runtimeMonitorEnabled: _runtimeMonitorEnabled,
                              onToggleRuntimeMonitor: (enabled) =>
                                  _setRuntimeMonitorEnabled(enabled),
                            ),
                          ),
                        ],
                      ),
                    ),
                  ],
                ),
                const SizedBox(height: 16),
                _DevicesSubsection(
                  title: 'Runtime Snapshot',
                  child: _TunnelRuntimeSummary(
                    tunnelRuntimeView: tunnelStore.tunnelRuntimeView,
                    tunnelDebugError: tunnelStore.tunnelDebugError,
                    error: sessionStore.error,
                    dense: false,
                  ),
                ),
              ],
            )
          else
            Column(
              children: [
                TextField(
                  key: AppTestKeys.devicesTunnelLocalIpField,
                  controller: _tunnelLocalIpController,
                  decoration:
                      const InputDecoration(labelText: 'Local Virtual IP'),
                ),
                const SizedBox(height: 12),
                TextField(
                  key: AppTestKeys.devicesTunnelPeerIpField,
                  controller: _tunnelPeerIpController,
                  decoration:
                      const InputDecoration(labelText: 'Peer Virtual IP'),
                ),
                const SizedBox(height: 12),
                TextField(
                  key: AppTestKeys.devicesTunnelPrivateKeyField,
                  controller: _tunnelPrivateKeyController,
                  decoration: const InputDecoration(
                    labelText: 'Interface Private Key',
                  ),
                ),
                const SizedBox(height: 12),
                TextField(
                  key: AppTestKeys.devicesTunnelPublicKeyField,
                  controller: _tunnelPublicKeyController,
                  decoration: const InputDecoration(
                    labelText: 'Interface Public Key',
                  ),
                ),
                const SizedBox(height: 12),
                TextField(
                  key: AppTestKeys.devicesTunnelPeerPublicKeyField,
                  controller: _tunnelPeerPublicKeyController,
                  decoration:
                      const InputDecoration(labelText: 'Peer Public Key'),
                ),
                const SizedBox(height: 12),
                TextField(
                  key: AppTestKeys.devicesTunnelEndpointField,
                  controller: _tunnelEndpointController,
                  decoration: const InputDecoration(labelText: 'Peer Endpoint'),
                ),
                const SizedBox(height: 12),
                TextField(
                  key: AppTestKeys.devicesTunnelDebugEngineModeField,
                  controller: _tunnelDebugEngineModeController,
                  decoration: const InputDecoration(
                    labelText: 'Debug Engine Mode',
                    hintText: 'noop, loopback, or external',
                  ),
                ),
                const SizedBox(height: 16),
                actionButtons,
              ],
            ),
        ],
      ),
    );
  }

  Widget _buildTunnelActionButtons(
    AppSessionStore sessionStore,
    AppTunnelController tunnelController,
  ) {
    return Wrap(
      spacing: 12,
      runSpacing: 12,
      children: [
        FilledButton(
          key: AppTestKeys.devicesTunnelApplyButton,
          onPressed: sessionStore.busy
              ? null
              : () => _handleTunnelApply(tunnelController),
          child: const Text('Apply Tunnel'),
        ),
        OutlinedButton(
          key: AppTestKeys.devicesTunnelUpButton,
          onPressed: sessionStore.busy
              ? null
              : () => _handleTunnelUp(tunnelController),
          child: const Text('Bring Up'),
        ),
        OutlinedButton(
          key: AppTestKeys.devicesTunnelViewButton,
          onPressed: sessionStore.busy
              ? null
              : () => _handleTunnelInspect(tunnelController),
          child: const Text('View Runtime'),
        ),
        OutlinedButton(
          key: AppTestKeys.devicesTunnelDownButton,
          onPressed: sessionStore.busy
              ? null
              : () => _handleTunnelDown(tunnelController),
          child: const Text('Bring Down'),
        ),
        OutlinedButton(
          key: AppTestKeys.devicesTunnelRemoveButton,
          onPressed: sessionStore.busy
              ? null
              : () => _handleTunnelRemovePeer(tunnelController),
          child: const Text('Remove Peer'),
        ),
      ],
    );
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
        addresses: ['$localVirtualIp/32'],
        dnsServers: const ['1.1.1.1'],
      ),
      peer: WireGuardTunnelPeerConfiguration(
        publicKey: _tunnelPeerPublicKeyController.text.trim(),
        endpoint: _tunnelEndpointController.text.trim(),
        allowedIps: ['$peerVirtualIp/32'],
      ),
    );
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

_TunnelActionPhaseGuidance _deriveTunnelActionPhaseGuidance(
  TunnelActionReport? report,
) {
  if (report == null) {
    return const _TunnelActionPhaseGuidance();
  }
  final nextStepLabel = switch (report.phase) {
    TunnelActionPhase.accepted => 'Inspect runtime and wait for backend state',
    TunnelActionPhase.configured => 'Bring the tunnel up',
    TunnelActionPhase.started => 'Inspect runtime or leave monitor running',
    TunnelActionPhase.verified => 'Observe runtime or bring the tunnel down',
    TunnelActionPhase.failed => 'Recover session or re-apply configuration',
    TunnelActionPhase.pendingVerification =>
      'Inspect runtime again to confirm backend progress',
  };
  return _TunnelActionPhaseGuidance(
    currentPhaseLabel: _tunnelActionPhaseLabel(report.phase),
    nextStepLabel: nextStepLabel,
    signalSourceLabel: report.sourceLabel,
  );
}

enum _TunnelActionStatus { running, succeeded, failed }

enum _TunnelActionKind {
  setup,
  apply,
  bootstrap,
  recover,
  up,
  inspect,
  down,
  removePeer
}

enum _TunnelSessionHealth { healthy, degraded, failed, idle }

class _TunnelSessionHealthSummary {
  const _TunnelSessionHealthSummary({
    required this.health,
    required this.reason,
    required this.supportingSignal,
    required this.recommendedAction,
    required this.trendLabel,
    required this.trendSignal,
    this.dataPlaneSignal,
    this.recommendationMatch,
    this.primaryAction,
    this.primaryActionLabel,
    this.secondaryAction,
    this.secondaryActionLabel,
  });

  final _TunnelSessionHealth health;
  final String reason;
  final String supportingSignal;
  final String recommendedAction;
  final String trendLabel;
  final String trendSignal;
  final String? dataPlaneSignal;
  final String? recommendationMatch;
  final _TunnelActionKind? primaryAction;
  final String? primaryActionLabel;
  final _TunnelActionKind? secondaryAction;
  final String? secondaryActionLabel;
}

enum _RelayDataPlaneFailureKind { auth, session, protocol }

class _RelayDataPlaneFailureSignal {
  const _RelayDataPlaneFailureSignal({
    required this.kind,
    required this.label,
    required this.summary,
  });

  final _RelayDataPlaneFailureKind kind;
  final String label;
  final String summary;
}

class _TunnelHealthSnapshot {
  const _TunnelHealthSnapshot({
    required this.health,
    required this.occurredAt,
  });

  final _TunnelSessionHealth health;
  final DateTime occurredAt;
}

class _TunnelHealthTrendSummary {
  const _TunnelHealthTrendSummary({
    required this.label,
    required this.signal,
  });

  final String label;
  final String signal;
}

class _TunnelActionPhaseGuidance {
  const _TunnelActionPhaseGuidance({
    this.currentPhaseLabel,
    this.nextStepLabel,
    this.signalSourceLabel,
  });

  final String? currentPhaseLabel;
  final String? nextStepLabel;
  final String? signalSourceLabel;
}

class _TunnelActionEvent {
  const _TunnelActionEvent({
    required this.kind,
    required this.label,
    required this.detail,
    required this.status,
    required this.occurredAt,
    this.progressLabel,
    this.signalSourceLabel,
  });

  final _TunnelActionKind kind;
  final String label;
  final String detail;
  final _TunnelActionStatus status;
  final DateTime occurredAt;
  final String? progressLabel;
  final String? signalSourceLabel;
}

class _TunnelActionBanner extends StatelessWidget {
  const _TunnelActionBanner({
    required this.event,
    required this.nextStepLabel,
  });

  final _TunnelActionEvent event;
  final String nextStepLabel;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final scheme = theme.colorScheme;
    final (backgroundColor, borderColor, foregroundColor) =
        switch (event.status) {
      _TunnelActionStatus.running => (
          scheme.primaryContainer,
          scheme.primary,
          scheme.onPrimaryContainer,
        ),
      _TunnelActionStatus.succeeded => (
          scheme.tertiaryContainer,
          scheme.tertiary,
          scheme.onTertiaryContainer,
        ),
      _TunnelActionStatus.failed => (
          scheme.errorContainer,
          scheme.error,
          scheme.onErrorContainer,
        ),
    };

    return Container(
      width: double.infinity,
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: backgroundColor,
        borderRadius: BorderRadius.circular(20),
        border: Border.all(color: borderColor),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            '${_statusLabel(event.status)} · ${event.label}',
            style: theme.textTheme.titleMedium?.copyWith(
              color: foregroundColor,
              fontWeight: FontWeight.w800,
            ),
          ),
          if (event.progressLabel != null) ...[
            const SizedBox(height: 6),
            Text(
              event.progressLabel!,
              style: theme.textTheme.bodySmall?.copyWith(
                color: foregroundColor,
                fontWeight: FontWeight.w700,
              ),
            ),
          ],
          const SizedBox(height: 6),
          Text(
            event.detail,
            style: theme.textTheme.bodyMedium?.copyWith(
              color: foregroundColor,
              height: 1.4,
            ),
          ),
          if (event.status != _TunnelActionStatus.running) ...[
            const SizedBox(height: 8),
            if (event.signalSourceLabel != null) ...[
              Text(
                'Signal source: ${event.signalSourceLabel!}',
                style: theme.textTheme.bodySmall?.copyWith(
                  color: foregroundColor,
                  fontWeight: FontWeight.w700,
                ),
              ),
              const SizedBox(height: 6),
            ],
            Text(
              'Next recommended step: $nextStepLabel',
              style: theme.textTheme.bodySmall?.copyWith(
                color: foregroundColor,
                fontWeight: FontWeight.w700,
              ),
            ),
          ],
        ],
      ),
    );
  }
}

class _TunnelActionTimelineRow extends StatelessWidget {
  const _TunnelActionTimelineRow({required this.event});

  final _TunnelActionEvent event;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final color = switch (event.status) {
      _TunnelActionStatus.running => theme.colorScheme.primary,
      _TunnelActionStatus.succeeded => theme.colorScheme.tertiary,
      _TunnelActionStatus.failed => theme.colorScheme.error,
    };

    return Row(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Container(
          width: 10,
          height: 10,
          margin: const EdgeInsets.only(top: 4),
          decoration: BoxDecoration(
            color: color,
            borderRadius: BorderRadius.circular(999),
          ),
        ),
        const SizedBox(width: 12),
        Expanded(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(
                '${event.label} · ${_statusLabel(event.status)}',
                style: theme.textTheme.bodyMedium?.copyWith(
                  fontWeight: FontWeight.w800,
                ),
              ),
              const SizedBox(height: 4),
              Text(
                event.detail,
                style: theme.textTheme.bodySmall?.copyWith(
                  color: theme.colorScheme.onSurfaceVariant,
                  height: 1.4,
                ),
              ),
            ],
          ),
        ),
        const SizedBox(width: 12),
        Text(
          _formatActionTime(event.occurredAt),
          style: theme.textTheme.bodySmall?.copyWith(
            color: theme.colorScheme.onSurfaceVariant,
          ),
        ),
      ],
    );
  }
}

class _TunnelHealthSummaryPanel extends StatelessWidget {
  const _TunnelHealthSummaryPanel({
    required this.summary,
    required this.recentSnapshots,
    required this.busy,
    required this.onPrimaryAction,
    required this.onSecondaryAction,
  });

  final _TunnelSessionHealthSummary summary;
  final List<_TunnelHealthSnapshot> recentSnapshots;
  final bool busy;
  final Future<void> Function() onPrimaryAction;
  final Future<void> Function() onSecondaryAction;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Wrap(
          spacing: 10,
          runSpacing: 10,
          children: [
            DesktopMetricPill(
              label: 'Health',
              value: _sessionHealthLabel(summary.health),
              backgroundColor: _sessionHealthBackgroundColor(
                context,
                summary.health,
              ),
              foregroundColor: _sessionHealthForegroundColor(
                context,
                summary.health,
              ),
              borderColor: _sessionHealthBorderColor(
                context,
                summary.health,
              ),
            ),
          ],
        ),
        const SizedBox(height: 14),
        Text(
          summary.reason,
          style: theme.textTheme.bodyMedium?.copyWith(
            color: theme.colorScheme.onSurfaceVariant,
            height: 1.45,
          ),
        ),
        const SizedBox(height: 10),
        Text(
          'Observed: ${summary.supportingSignal}',
          style: theme.textTheme.bodySmall?.copyWith(
            color: theme.colorScheme.onSurfaceVariant,
            height: 1.45,
          ),
        ),
        if (summary.recommendationMatch != null) ...[
          const SizedBox(height: 10),
          Text(
            summary.recommendationMatch!,
            style: theme.textTheme.bodySmall?.copyWith(
              color: theme.colorScheme.onSurfaceVariant,
              height: 1.45,
            ),
          ),
        ],
        const SizedBox(height: 10),
        Text(
          'Trend: ${summary.trendLabel}',
          style: theme.textTheme.bodySmall?.copyWith(
            color: theme.colorScheme.onSurfaceVariant,
            height: 1.45,
            fontWeight: FontWeight.w700,
          ),
        ),
        const SizedBox(height: 6),
        Text(
          summary.trendSignal,
          style: theme.textTheme.bodySmall?.copyWith(
            color: theme.colorScheme.onSurfaceVariant,
            height: 1.45,
          ),
        ),
        const SizedBox(height: 12),
        _TunnelHealthTimeline(snapshots: recentSnapshots),
        const SizedBox(height: 10),
        Text(
          'Recommended action: ${summary.recommendedAction}',
          style: theme.textTheme.bodySmall?.copyWith(
            color: theme.colorScheme.onSurfaceVariant,
            height: 1.45,
            fontWeight: FontWeight.w700,
          ),
        ),
        if (summary.primaryAction != null ||
            summary.secondaryAction != null) ...[
          const SizedBox(height: 14),
          Wrap(
            spacing: 12,
            runSpacing: 12,
            children: [
              if (summary.primaryAction != null)
                FilledButton(
                  onPressed: busy ? null : onPrimaryAction,
                  child: Text(summary.primaryActionLabel!),
                ),
              if (summary.secondaryAction != null)
                OutlinedButton(
                  onPressed: busy ? null : onSecondaryAction,
                  child: Text(summary.secondaryActionLabel!),
                ),
            ],
          ),
        ],
      ],
    );
  }
}

class _TunnelHealthTimeline extends StatelessWidget {
  const _TunnelHealthTimeline({required this.snapshots});

  final List<_TunnelHealthSnapshot> snapshots;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final visibleSnapshots = snapshots.take(4).toList(growable: false);
    if (visibleSnapshots.isEmpty) {
      return Text(
        'Recent checks: waiting for health history.',
        style: theme.textTheme.bodySmall?.copyWith(
          color: theme.colorScheme.onSurfaceVariant,
          height: 1.45,
        ),
      );
    }

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          'Recent checks',
          style: theme.textTheme.bodySmall?.copyWith(
            color: theme.colorScheme.onSurfaceVariant,
            fontWeight: FontWeight.w700,
          ),
        ),
        const SizedBox(height: 10),
        Row(
          children: List.generate(visibleSnapshots.length, (index) {
            final snapshot = visibleSnapshots[index];
            return Expanded(
              child: Padding(
                padding: EdgeInsets.only(
                  right: index == visibleSnapshots.length - 1 ? 0 : 8,
                ),
                child: _TunnelHealthTimelineNode(
                  snapshot: snapshot,
                  isLatest: index == 0,
                ),
              ),
            );
          }),
        ),
      ],
    );
  }
}

class _TunnelHealthTimelineNode extends StatelessWidget {
  const _TunnelHealthTimelineNode({
    required this.snapshot,
    required this.isLatest,
  });

  final _TunnelHealthSnapshot snapshot;
  final bool isLatest;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Container(
      padding: const EdgeInsets.all(10),
      decoration: BoxDecoration(
        color: _sessionHealthBackgroundColor(context, snapshot.health),
        borderRadius: BorderRadius.circular(16),
        border: Border.all(
          color: _sessionHealthBorderColor(context, snapshot.health),
          width: isLatest ? 1.4 : 1,
        ),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            isLatest ? 'now' : _formatActionTime(snapshot.occurredAt),
            style: theme.textTheme.bodySmall?.copyWith(
              color: _sessionHealthForegroundColor(context, snapshot.health),
              fontWeight: FontWeight.w800,
            ),
          ),
          const SizedBox(height: 6),
          Text(
            _sessionHealthLabel(snapshot.health),
            style: theme.textTheme.bodySmall?.copyWith(
              color: _sessionHealthForegroundColor(context, snapshot.health),
              height: 1.3,
            ),
          ),
        ],
      ),
    );
  }
}

Color _sessionHealthBackgroundColor(
  BuildContext context,
  _TunnelSessionHealth health,
) {
  final scheme = Theme.of(context).colorScheme;
  return switch (health) {
    _TunnelSessionHealth.healthy => scheme.tertiaryContainer,
    _TunnelSessionHealth.degraded => scheme.secondaryContainer,
    _TunnelSessionHealth.failed => scheme.errorContainer,
    _TunnelSessionHealth.idle => scheme.surfaceContainerLowest,
  };
}

Color _sessionHealthForegroundColor(
  BuildContext context,
  _TunnelSessionHealth health,
) {
  final scheme = Theme.of(context).colorScheme;
  return switch (health) {
    _TunnelSessionHealth.healthy => scheme.onTertiaryContainer,
    _TunnelSessionHealth.degraded => scheme.onSecondaryContainer,
    _TunnelSessionHealth.failed => scheme.onErrorContainer,
    _TunnelSessionHealth.idle => scheme.onSurface,
  };
}

Color _sessionHealthBorderColor(
  BuildContext context,
  _TunnelSessionHealth health,
) {
  final scheme = Theme.of(context).colorScheme;
  return switch (health) {
    _TunnelSessionHealth.healthy => scheme.tertiary,
    _TunnelSessionHealth.degraded => scheme.secondary,
    _TunnelSessionHealth.failed => scheme.error,
    _TunnelSessionHealth.idle => scheme.outlineVariant,
  };
}

class _TunnelStagePlan {
  const _TunnelStagePlan({
    required this.nextStepLabel,
    required this.summary,
    required this.recoveryTitle,
    required this.recoverySteps,
    required this.steps,
    required this.primaryAction,
    required this.primaryActionLabel,
    this.primaryRecoveryAction,
    this.primaryRecoveryLabel,
    this.secondaryRecoveryAction,
    this.secondaryRecoveryLabel,
  });

  final String nextStepLabel;
  final String summary;
  final String recoveryTitle;
  final List<String> recoverySteps;
  final List<_TunnelStageStep> steps;
  final _TunnelActionKind? primaryAction;
  final String primaryActionLabel;
  final _TunnelActionKind? primaryRecoveryAction;
  final String? primaryRecoveryLabel;
  final _TunnelActionKind? secondaryRecoveryAction;
  final String? secondaryRecoveryLabel;
}

class _TunnelStageStep {
  const _TunnelStageStep({
    required this.title,
    required this.description,
    required this.state,
  });

  final String title;
  final String description;
  final _TunnelStageState state;
}

enum _TunnelStageState { complete, current, pending, blocked }

class _TunnelStageFlow extends StatelessWidget {
  const _TunnelStageFlow({
    required this.plan,
    required this.busy,
    required this.onRunNext,
  });

  final _TunnelStagePlan plan;
  final bool busy;
  final Future<void> Function() onRunNext;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          plan.summary,
          style: theme.textTheme.bodyMedium?.copyWith(
            color: theme.colorScheme.onSurfaceVariant,
            height: 1.45,
          ),
        ),
        const SizedBox(height: 16),
        for (var index = 0; index < plan.steps.length; index++) ...[
          _TunnelStageRow(
            index: index + 1,
            step: plan.steps[index],
          ),
          if (index != plan.steps.length - 1) const SizedBox(height: 12),
        ],
        const SizedBox(height: 16),
        Align(
          alignment: Alignment.centerLeft,
          child: FilledButton(
            onPressed: busy || plan.primaryAction == null ? null : onRunNext,
            child: Text(plan.primaryActionLabel),
          ),
        ),
      ],
    );
  }
}

class _TunnelStageRow extends StatelessWidget {
  const _TunnelStageRow({
    required this.index,
    required this.step,
  });

  final int index;
  final _TunnelStageStep step;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final (fill, foreground) = switch (step.state) {
      _TunnelStageState.complete => (
          theme.colorScheme.tertiary,
          theme.colorScheme.onTertiary,
        ),
      _TunnelStageState.current => (
          theme.colorScheme.primary,
          theme.colorScheme.onPrimary,
        ),
      _TunnelStageState.blocked => (
          theme.colorScheme.error,
          theme.colorScheme.onError,
        ),
      _TunnelStageState.pending => (
          theme.colorScheme.surfaceContainerHighest,
          theme.colorScheme.onSurfaceVariant,
        ),
    };

    return Row(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Container(
          width: 28,
          height: 28,
          alignment: Alignment.center,
          decoration: BoxDecoration(
            color: fill,
            borderRadius: BorderRadius.circular(999),
          ),
          child: Text(
            '$index',
            style: theme.textTheme.labelLarge?.copyWith(
              color: foreground,
              fontWeight: FontWeight.w800,
            ),
          ),
        ),
        const SizedBox(width: 12),
        Expanded(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(
                step.title,
                style: theme.textTheme.bodyMedium?.copyWith(
                  fontWeight: FontWeight.w800,
                ),
              ),
              const SizedBox(height: 4),
              Text(
                step.description,
                style: theme.textTheme.bodySmall?.copyWith(
                  color: theme.colorScheme.onSurfaceVariant,
                  height: 1.4,
                ),
              ),
            ],
          ),
        ),
      ],
    );
  }
}

class _TunnelStageOverview extends StatelessWidget {
  const _TunnelStageOverview({
    required this.plan,
    required this.latestAction,
    required this.sessionHealth,
  });

  final _TunnelStagePlan plan;
  final _TunnelActionEvent? latestAction;
  final _TunnelSessionHealth sessionHealth;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final blocked = plan.steps.any(
      (step) => step.state == _TunnelStageState.blocked,
    );
    final completed = plan.steps
        .where((step) => step.state == _TunnelStageState.complete)
        .length;

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Wrap(
          spacing: 10,
          runSpacing: 10,
          children: [
            DesktopMetricPill(
              label: 'Progress',
              value: blocked ? 'blocked' : '$completed/3 complete',
              backgroundColor: blocked
                  ? theme.colorScheme.errorContainer
                  : theme.colorScheme.surfaceContainerLowest,
              foregroundColor:
                  blocked ? theme.colorScheme.onErrorContainer : null,
              borderColor: blocked ? theme.colorScheme.error : null,
            ),
            DesktopMetricPill(
              label: 'Next',
              value: plan.nextStepLabel,
            ),
            DesktopMetricPill(
              label: 'Health',
              value: _sessionHealthLabel(sessionHealth),
              backgroundColor: _sessionHealthBackgroundColor(
                context,
                sessionHealth,
              ),
              foregroundColor: _sessionHealthForegroundColor(
                context,
                sessionHealth,
              ),
              borderColor: _sessionHealthBorderColor(
                context,
                sessionHealth,
              ),
            ),
          ],
        ),
        const SizedBox(height: 14),
        for (var index = 0; index < plan.steps.length; index++) ...[
          _TunnelStageOverviewCard(
            index: index + 1,
            step: plan.steps[index],
          ),
          if (index != plan.steps.length - 1) const SizedBox(height: 10),
        ],
        if (latestAction != null) ...[
          const SizedBox(height: 14),
          Text(
            'Latest action: ${latestAction!.label} · ${_statusLabel(latestAction!.status)}',
            style: theme.textTheme.bodySmall?.copyWith(
              color: theme.colorScheme.onSurfaceVariant,
              fontWeight: FontWeight.w700,
            ),
          ),
        ],
      ],
    );
  }
}

class _TunnelStageOverviewCard extends StatelessWidget {
  const _TunnelStageOverviewCard({
    required this.index,
    required this.step,
  });

  final int index;
  final _TunnelStageStep step;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final (backgroundColor, borderColor, accentColor) = switch (step.state) {
      _TunnelStageState.complete => (
          theme.colorScheme.tertiaryContainer,
          theme.colorScheme.tertiary,
          theme.colorScheme.tertiary,
        ),
      _TunnelStageState.current => (
          theme.colorScheme.primaryContainer,
          theme.colorScheme.primary,
          theme.colorScheme.primary,
        ),
      _TunnelStageState.blocked => (
          theme.colorScheme.errorContainer,
          theme.colorScheme.error,
          theme.colorScheme.error,
        ),
      _TunnelStageState.pending => (
          theme.colorScheme.surfaceContainerLowest,
          theme.colorScheme.outlineVariant,
          theme.colorScheme.onSurfaceVariant,
        ),
    };

    return AnimatedContainer(
      duration: const Duration(milliseconds: 220),
      curve: Curves.easeOutCubic,
      padding: const EdgeInsets.all(14),
      decoration: BoxDecoration(
        color: backgroundColor,
        borderRadius: BorderRadius.circular(18),
        border: Border.all(color: borderColor),
      ),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Container(
            width: 26,
            height: 26,
            alignment: Alignment.center,
            decoration: BoxDecoration(
              color: accentColor.withValues(alpha: 0.16),
              borderRadius: BorderRadius.circular(999),
            ),
            child: Text(
              '$index',
              style: theme.textTheme.labelLarge?.copyWith(
                color: accentColor,
                fontWeight: FontWeight.w800,
              ),
            ),
          ),
          const SizedBox(width: 12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  step.title,
                  style: theme.textTheme.bodyMedium?.copyWith(
                    fontWeight: FontWeight.w800,
                  ),
                ),
                const SizedBox(height: 4),
                Text(
                  step.description,
                  style: theme.textTheme.bodySmall?.copyWith(
                    color: theme.colorScheme.onSurfaceVariant,
                    height: 1.35,
                  ),
                ),
              ],
            ),
          ),
          const SizedBox(width: 12),
          Text(
            _stageStateLabel(step.state),
            style: theme.textTheme.labelMedium?.copyWith(
              color: accentColor,
              fontWeight: FontWeight.w800,
            ),
          ),
        ],
      ),
    );
  }
}

class _TunnelRecoveryGuide extends StatelessWidget {
  const _TunnelRecoveryGuide({
    required this.plan,
    required this.busy,
    required this.onPrimaryAction,
    required this.onSecondaryAction,
  });

  final _TunnelStagePlan plan;
  final bool busy;
  final Future<void> Function() onPrimaryAction;
  final Future<void> Function() onSecondaryAction;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          plan.recoveryTitle,
          style: theme.textTheme.bodyMedium?.copyWith(
            fontWeight: FontWeight.w800,
          ),
        ),
        const SizedBox(height: 12),
        for (var index = 0; index < plan.recoverySteps.length; index++) ...[
          Text(
            '${index + 1}. ${plan.recoverySteps[index]}',
            style: theme.textTheme.bodySmall?.copyWith(
              color: theme.colorScheme.onSurfaceVariant,
              height: 1.45,
            ),
          ),
          if (index != plan.recoverySteps.length - 1) const SizedBox(height: 8),
        ],
        if (plan.primaryRecoveryAction != null ||
            plan.secondaryRecoveryAction != null) ...[
          const SizedBox(height: 16),
          Wrap(
            spacing: 12,
            runSpacing: 12,
            children: [
              if (plan.primaryRecoveryAction != null)
                FilledButton(
                  onPressed: busy ? null : onPrimaryAction,
                  child: Text(plan.primaryRecoveryLabel!),
                ),
              if (plan.secondaryRecoveryAction != null)
                OutlinedButton(
                  onPressed: busy ? null : onSecondaryAction,
                  child: Text(plan.secondaryRecoveryLabel!),
                ),
            ],
          ),
        ],
      ],
    );
  }
}
