import 'dart:async';

import 'package:flutter/material.dart';
import 'package:slan_app_core_plugin/slan_app_core_plugin.dart';

import '../../infra/app_core/scope/app_core_scope.dart';
import '../../infra/app_core/models/models.dart';
import '../../infra/app_core/store/app_core_demo_store.dart';
import '../../testing/app_test_keys.dart';
import '../shared/desktop_client_widgets.dart';

part 'devices_page_logic.dart';

class DevicesPage extends StatefulWidget {
  const DevicesPage({super.key});

  @override
  State<DevicesPage> createState() => _DevicesPageState();
}

class _DevicesPageState extends State<DevicesPage> {
  final _nameController = TextEditingController(text: 'thor-mac');
  final _platformController = TextEditingController(text: 'macos');
  final _machineIdController = TextEditingController(text: 'machine-1');
  final _publicKeyController = TextEditingController(text: 'pubkey-1');
  final _nodeIdController = TextEditingController(text: 'node-1');
  final _nodePublicKeyController = TextEditingController(text: 'node-pubkey-1');
  final _bootstrapNodeIdController = TextEditingController();
  final _networkIdController = TextEditingController(text: 'net-1');
  final _peerNodeIdController = TextEditingController(text: 'peer-node-1');
  final _reasonController = TextEditingController(text: 'timeout');
  final _sendPayloadController = TextEditingController(text: 'hello');
  final _probePayloadController = TextEditingController(text: 'hello');
  final _probeTimeoutController = TextEditingController(text: '25');
  final _tunnelLocalIpController = TextEditingController(text: '100.64.0.10');
  final _tunnelPeerIpController = TextEditingController(text: '100.64.0.2');
  final _tunnelPrivateKeyController =
      TextEditingController(text: 'debug-private-key');
  final _tunnelPublicKeyController =
      TextEditingController(text: 'debug-public-key');
  final _tunnelPeerPublicKeyController =
      TextEditingController(text: 'peer-debug-public-key');
  final _tunnelEndpointController =
      TextEditingController(text: '203.0.113.10:51820');
  final _tunnelDebugEngineModeController = TextEditingController();
  _TunnelActionEvent? _activeTunnelAction;
  String? _connectionPlanHint;
  final List<_TunnelActionEvent> _recentTunnelActions = [];
  final List<_TunnelHealthSnapshot> _recentHealthSnapshots = [];
  Timer? _runtimeMonitorTimer;
  bool _runtimeMonitorEnabled = false;

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
    final store = AppCoreScope.demo;
    return AnimatedBuilder(
      animation: store,
      builder: (context, _) {
        return LayoutBuilder(
          builder: (context, constraints) {
            final isDesktop = constraints.maxWidth >= 1180;
            final child = isDesktop
                ? _buildDesktopWorkspace(context, store)
                : _buildCompactWorkspace(context, store);
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

  Widget _buildCompactWorkspace(BuildContext context, AppCoreDemoStore store) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        _buildPageIntro(context),
        const SizedBox(height: 16),
        _buildIdentitySection(context, store),
        const SizedBox(height: 16),
        _buildConnectivitySection(context, store),
        const SizedBox(height: 16),
        _buildDiagnosticsSection(context, store),
        const SizedBox(height: 16),
        _buildTunnelSection(context, store),
        const SizedBox(height: 16),
        _DeviceStateCard(
          device: store.device,
          node: store.node,
          bootstrap: store.bootstrap,
          controlStatus: store.controlStatus,
          relayTicket: store.relayTicket,
          lastSendBytes: store.lastSendBytes,
          lastSendFailure: store.lastSendFailure,
          lastProbe: store.lastProbe,
          lastProbeFailure: store.lastProbeFailure,
          connectionState: store.connectionState,
          tunnelRuntimeView: store.tunnelRuntimeView,
          tunnelDebugError: store.tunnelDebugError,
          error: store.error,
          activeAction: _activeTunnelAction,
          recentActions: _recentTunnelActions,
          recentHealthSnapshots: _recentHealthSnapshots,
          lastTunnelActionReport: store.lastTunnelActionReport,
          runtimeMonitorEnabled: _runtimeMonitorEnabled,
          onBootstrap: () => _handleBootstrapRefresh(store),
          onApply: () => _handleTunnelApply(store),
          onRecover: () => _handleRecoverSession(store),
          onUp: () => _handleTunnelUp(store),
          onInspect: () => _handleTunnelInspect(store),
          onDown: () => _handleTunnelDown(store),
          busy: store.busy,
        ),
      ],
    );
  }

  Widget _buildDesktopWorkspace(BuildContext context, AppCoreDemoStore store) {
    final runtime = store.tunnelRuntimeView;
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
                value: store.connectionState.status,
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
                label: store.busy ? 'pipeline busy' : 'pipeline idle',
              ),
              DesktopBadge(
                label: 'path ${store.connectionState.path?.name ?? 'unknown'}',
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
                          _buildIdentitySection(context, store),
                          const SizedBox(height: 16),
                          _buildConnectivitySection(context, store),
                        ],
                      ),
                    ),
                    const SizedBox(width: 16),
                    Expanded(
                      flex: 6,
                      child: Column(
                        children: [
                          _buildTunnelSection(context, store, isDesktop: true),
                          const SizedBox(height: 16),
                          _buildDiagnosticsSection(context, store),
                        ],
                      ),
                    ),
                    const SizedBox(width: 16),
                    SizedBox(
                      width: 380,
                      child: _DeviceStateCard(
                        device: store.device,
                        node: store.node,
                        bootstrap: store.bootstrap,
                        controlStatus: store.controlStatus,
                        relayTicket: store.relayTicket,
                        lastSendBytes: store.lastSendBytes,
                        lastSendFailure: store.lastSendFailure,
                        lastProbe: store.lastProbe,
                        lastProbeFailure: store.lastProbeFailure,
                        connectionState: store.connectionState,
                        tunnelRuntimeView: store.tunnelRuntimeView,
                        tunnelDebugError: store.tunnelDebugError,
                        error: store.error,
                        activeAction: _activeTunnelAction,
                        recentActions: _recentTunnelActions,
                        recentHealthSnapshots: _recentHealthSnapshots,
                        lastTunnelActionReport: store.lastTunnelActionReport,
                        runtimeMonitorEnabled: _runtimeMonitorEnabled,
                        onBootstrap: () => _handleBootstrapRefresh(store),
                        onApply: () => _handleTunnelApply(store),
                        onRecover: () => _handleRecoverSession(store),
                        onUp: () => _handleTunnelUp(store),
                        onInspect: () => _handleTunnelInspect(store),
                        onDown: () => _handleTunnelDown(store),
                        busy: store.busy,
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
          'Register a local device, attach a node, bootstrap overlay state, drive relay fallback, and inspect PacketTunnel runtime without leaving the desktop client.',
      trailing: trailing,
      footer: footer,
    );
  }

  Widget _buildIdentitySection(BuildContext context, AppCoreDemoStore store) {
    return _DevicesWorkbenchCard(
      title: 'Identity',
      subtitle: 'Device registration, node registration, and bootstrap.',
      child: Column(
        children: [
          _DevicesSubsection(
            title: 'Register Device',
            child: Column(
              children: [
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
                  child: FilledButton(
                    key: AppTestKeys.devicesRegisterDeviceButton,
                    onPressed: store.busy
                        ? null
                        : () => store.registerDevice(
                              name: _nameController.text.trim(),
                              platform: _platformController.text.trim(),
                              machineId: _machineIdController.text.trim(),
                              publicKey: _publicKeyController.text.trim(),
                            ),
                    child: const Text('Register Device'),
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
                  decoration: const InputDecoration(labelText: 'Node ID'),
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
                    onPressed: store.busy || store.device == null
                        ? null
                        : () => store.registerNode(
                              deviceId: store.device!.deviceId,
                              nodeId: _nodeIdController.text.trim(),
                              nodePublicKey:
                                  _nodePublicKeyController.text.trim(),
                            ),
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
                    onPressed: store.busy
                        ? null
                        : () => store.loadBootstrap(
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
      BuildContext context, AppCoreDemoStore store) {
    final peerNodeId = _peerNodeIdController.text.trim();
    final controlPlan = _connectPlanForPeer(store.controlStatus, peerNodeId);
    final recommendationMatch = _connectRecommendationMatchLabel(
      controlPlan: controlPlan,
      connectionState: store.connectionState,
      lastProbe: store.lastProbe,
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
                onPressed: store.busy ? null : () => _handleConnect(store),
                child: const Text('Connect'),
              ),
              OutlinedButton(
                key: AppTestKeys.devicesDisconnectButton,
                onPressed: store.busy ? null : store.disconnect,
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
                      : _describeConnectPlan(controlPlan),
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

  Widget _buildDiagnosticsSection(
      BuildContext context, AppCoreDemoStore store) {
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
                    onPressed: store.busy
                        ? null
                        : () => store.send(
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
                    onPressed: store.busy
                        ? null
                        : () => store.probe(
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
            title: 'Latest Diagnostics',
            child: DesktopKeyValueList(
              entries: [
                DesktopKeyValueEntry(
                  label: 'Send bytes',
                  value: store.lastSendBytes?.toString() ?? '-',
                ),
                DesktopKeyValueEntry(
                  label: 'Send failure',
                  value: store.lastSendFailure?.label ?? '-',
                ),
                DesktopKeyValueEntry(
                  label: 'Probe',
                  value: store.lastProbe?.probeId ?? 'none',
                ),
                DesktopKeyValueEntry(
                  label: 'Probe RTT',
                  value: store.lastProbe?.replyRttMs?.toString() ?? '-',
                ),
                DesktopKeyValueEntry(
                  label: 'Probe failure',
                  value: store.lastProbeFailure?.label ?? '-',
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildTunnelSection(
    BuildContext context,
    AppCoreDemoStore store, {
    bool isDesktop = false,
  }) {
    final actionButtons = _buildTunnelActionButtons(store);
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
                              tunnelRuntimeView: store.tunnelRuntimeView,
                              controlStatus: store.controlStatus,
                              tunnelDebugError: store.tunnelDebugError,
                              error: store.error,
                              busy: store.busy,
                              connectionState: store.connectionState,
                              peerVirtualIp:
                                  _tunnelPeerIpController.text.trim(),
                              activeAction: _activeTunnelAction,
                              recentActions: _recentTunnelActions,
                              recentHealthSnapshots: _recentHealthSnapshots,
                              lastTunnelActionReport:
                                  store.lastTunnelActionReport,
                              lastProbe: store.lastProbe,
                              lastSendFailure: store.lastSendFailure,
                              lastProbeFailure: store.lastProbeFailure,
                              onRecover: () => _handleRecoverSession(store),
                              onBootstrap: () => _handleBootstrapRefresh(store),
                              onApply: () => _handleTunnelApply(store),
                              onUp: () => _handleTunnelUp(store),
                              onInspect: () => _handleTunnelInspect(store),
                              onDown: () => _handleTunnelDown(store),
                              runtimeMonitorEnabled: _runtimeMonitorEnabled,
                              onToggleRuntimeMonitor: (enabled) =>
                                  _setRuntimeMonitorEnabled(enabled, store),
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
                    tunnelRuntimeView: store.tunnelRuntimeView,
                    tunnelDebugError: store.tunnelDebugError,
                    error: store.error,
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

  Widget _buildTunnelActionButtons(AppCoreDemoStore store) {
    return Wrap(
      spacing: 12,
      runSpacing: 12,
      children: [
        FilledButton(
          key: AppTestKeys.devicesTunnelApplyButton,
          onPressed: store.busy ? null : () => _handleTunnelApply(store),
          child: const Text('Apply Tunnel'),
        ),
        OutlinedButton(
          key: AppTestKeys.devicesTunnelUpButton,
          onPressed: store.busy ? null : () => _handleTunnelUp(store),
          child: const Text('Bring Up'),
        ),
        OutlinedButton(
          key: AppTestKeys.devicesTunnelViewButton,
          onPressed: store.busy ? null : () => _handleTunnelInspect(store),
          child: const Text('View Runtime'),
        ),
        OutlinedButton(
          key: AppTestKeys.devicesTunnelDownButton,
          onPressed: store.busy ? null : () => _handleTunnelDown(store),
          child: const Text('Bring Down'),
        ),
        OutlinedButton(
          key: AppTestKeys.devicesTunnelRemoveButton,
          onPressed: store.busy ? null : () => _handleTunnelRemovePeer(store),
          child: const Text('Remove Peer'),
        ),
      ],
    );
  }

  Future<void> _handleTunnelApply(AppCoreDemoStore store) {
    return _runTunnelWorkbenchAction(
      store,
      kind: _TunnelActionKind.apply,
      label: 'Apply configuration',
      detail:
          'Stage ${_tunnelLocalIpController.text.trim()} -> ${_tunnelPeerIpController.text.trim()}',
      action: () => store.applyTunnelConfiguration(
        configuration: _buildTunnelConfiguration(),
        verifyPeerVirtualIp: _tunnelPeerIpController.text.trim(),
      ),
    );
  }

  Future<void> _handleTunnelUp(AppCoreDemoStore store) {
    return _runTunnelWorkbenchAction(
      store,
      kind: _TunnelActionKind.up,
      label: 'Bring tunnel up',
      detail: 'Start PacketTunnel session',
      action: () => store.bringTunnelUp(
        verifyPeerVirtualIp: _tunnelPeerIpController.text.trim(),
      ),
    );
  }

  Future<void> _handleTunnelInspect(AppCoreDemoStore store) {
    return _runTunnelWorkbenchAction(
      store,
      kind: _TunnelActionKind.inspect,
      label: 'Refresh runtime',
      detail: 'Inspect ${_tunnelPeerIpController.text.trim()} runtime view',
      action: () => store.refreshTunnelRuntime(
        peerVirtualIp: _tunnelPeerIpController.text.trim(),
      ),
    );
  }

  Future<void> _handleTunnelDown(AppCoreDemoStore store) {
    return _runTunnelWorkbenchAction(
      store,
      kind: _TunnelActionKind.down,
      label: 'Bring tunnel down',
      detail: 'Stop PacketTunnel session',
      action: store.bringTunnelDown,
    );
  }

  Future<void> _handleTunnelRemovePeer(AppCoreDemoStore store) {
    return _runTunnelWorkbenchAction(
      store,
      kind: _TunnelActionKind.removePeer,
      label: 'Remove peer',
      detail:
          'Delete peer ${_tunnelPeerIpController.text.trim()} from tunnel view',
      action: () => store.removeTunnelPeer(
        peerVirtualIp: _tunnelPeerIpController.text.trim(),
      ),
    );
  }

  Future<void> _handleBootstrapRefresh(AppCoreDemoStore store) async {
    final targetNodeId = store.node?.nodeId.trim();
    final targetNetworkId = store.bootstrap?.networks.isNotEmpty == true
        ? store.bootstrap!.networks.first.networkId
        : store.networks.isNotEmpty
            ? store.networks.first.networkId
            : null;
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

    final canRunControlSync =
        (store.bootstrap?.controlPlane.sessionToken?.isNotEmpty ?? false) &&
            (store.bootstrap?.controlPlane.wsUrl.trim().isNotEmpty ?? false) &&
            targetNodeId != null &&
            targetNodeId.isNotEmpty &&
            targetNetworkId != null &&
            targetNetworkId.isNotEmpty;

    if (canRunControlSync) {
      await store.syncControlPlane(
        nodeId: targetNodeId,
        networkId: targetNetworkId,
      );
    } else {
      await store.loadBootstrap(
        nodeId: targetNodeId,
        networkId: targetNetworkId,
      );
    }

    final failureMessage = store.error;
    final completedEvent = _TunnelActionEvent(
      kind: _TunnelActionKind.bootstrap,
      label: 'Refresh bootstrap',
      detail: failureMessage ??
          (canRunControlSync
              ? 'Control session synchronized and bootstrap metadata were refreshed from the control plane.'
              : 'Bootstrap and relay metadata were refreshed from the control plane.'),
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
      _captureHealthSnapshot(store);
    });
  }

  Future<void> _handleConnect(AppCoreDemoStore store) async {
    final peerNodeId = _peerNodeIdController.text.trim();
    final plan = _connectPlanForPeer(store.controlStatus, peerNodeId);
    final preflightHint = plan == null
        ? 'No control-plane connect plan for $peerNodeId. Trying local direct path first, then relay fallback if needed.'
        : 'Trying ${_describeConnectPlan(plan)} for $peerNodeId before relay fallback.';
    setState(() {
      _connectionPlanHint = preflightHint;
    });

    await store.connectWithFallback(
      networkId: _networkIdController.text.trim(),
      peerNodeId: peerNodeId,
      reason: _reasonController.text.trim(),
    );

    final recommendationMatch = _connectRecommendationMatchLabel(
      controlPlan: plan,
      connectionState: store.connectionState,
      lastProbe: store.lastProbe,
    );
    final resultHint = switch (store.connectionState.status) {
      'connected' =>
        'Connected over ${store.connectionState.path?.name ?? 'unknown'} path. Recommendation $recommendationMatch.${plan == null ? '' : ' Control plane suggested ${_describeConnectPlan(plan)}.'}',
      'failed' =>
        'Connection failed after trying the planned path. Recommendation $recommendationMatch.${plan == null ? '' : ' Last control suggestion was ${_describeConnectPlan(plan)}.'}',
      'connecting' =>
        'Connection is still in progress. Recommendation $recommendationMatch.${plan == null ? '' : ' Following ${_describeConnectPlan(plan)}.'}',
      _ =>
        'Connection state is ${store.connectionState.status}. Recommendation $recommendationMatch.${plan == null ? '' : ' Control suggestion remains ${_describeConnectPlan(plan)}.'}',
    };
    if (!mounted) {
      return;
    }
    setState(() {
      _connectionPlanHint = resultHint;
    });
  }

  Future<void> _handleRecoverSession(AppCoreDemoStore store) {
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
          finalReport?.errorMessage ?? store.tunnelDebugError ?? store.error;
      final automationNote = failureMessage == null
          ? await _applyTunnelLifecycleAutomation(
              _TunnelActionKind.recover, store)
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
        _captureHealthSnapshot(store);
      });
    }

    return Future<void>(() async {
      setRecoveryProgress(
        detail: 'Applying configuration before recovery',
        progressLabel: '1/3 Apply configuration',
      );
      final applyReport = await store.applyTunnelConfiguration(
        configuration: _buildTunnelConfiguration(),
        verifyPeerVirtualIp: _tunnelPeerIpController.text.trim(),
      );
      if (!applyReport.succeeded) {
        await finishRecovery(applyReport);
        return;
      }

      setRecoveryProgress(
        detail: _formatTunnelActionReportDetail(applyReport),
        progressLabel: '2/3 Bring tunnel up',
      );
      final upReport = await store.bringTunnelUp(
        verifyPeerVirtualIp: _tunnelPeerIpController.text.trim(),
      );
      if (!upReport.succeeded) {
        await finishRecovery(upReport);
        return;
      }

      setRecoveryProgress(
        detail: _formatTunnelActionReportDetail(upReport),
        progressLabel: '3/3 Inspect runtime',
      );
      final inspectReport = await store.refreshTunnelRuntime(
        peerVirtualIp: _tunnelPeerIpController.text.trim(),
      );
      await finishRecovery(inspectReport);
    });
  }

  void _setRuntimeMonitorEnabled(bool enabled, AppCoreDemoStore store) {
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

    _refreshRuntimeFromMonitor(store);
    _runtimeMonitorTimer = Timer.periodic(
      const Duration(seconds: 4),
      (_) => _refreshRuntimeFromMonitor(store),
    );
  }

  Future<void> _refreshRuntimeFromMonitor(AppCoreDemoStore store) async {
    final peerVirtualIp = _tunnelPeerIpController.text.trim();
    if (!_runtimeMonitorEnabled ||
        peerVirtualIp.isEmpty ||
        store.busy ||
        !mounted) {
      return;
    }

    await store.refreshTunnelRuntime(peerVirtualIp: peerVirtualIp);
    if (!mounted) {
      return;
    }
    setState(() {
      _captureHealthSnapshot(store);
    });
  }

  Future<void> _runTunnelWorkbenchAction(
    AppCoreDemoStore store, {
    required _TunnelActionKind kind,
    required String label,
    required String detail,
    required Future<TunnelActionReport> Function() action,
  }) async {
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
        report.errorMessage ?? store.tunnelDebugError ?? store.error;
    final automationNote = failureMessage == null
        ? await _applyTunnelLifecycleAutomation(kind, store)
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
      _captureHealthSnapshot(store);
    });
  }

  String _formatTunnelActionReportDetail(TunnelActionReport? report) {
    if (report == null) {
      return 'Tunnel action completed without a structured backend report.';
    }
    return '${report.detail} Verified by ${report.sourceLabel}.';
  }

  void _captureHealthSnapshot(AppCoreDemoStore store) {
    final summary = _deriveTunnelSessionHealthSummary(
      runtime: store.tunnelRuntimeView,
      controlStatus: store.controlStatus,
      connectionState: store.connectionState,
      lastProbe: store.lastProbe,
      tunnelDebugError: store.tunnelDebugError,
      error: store.error,
      runtimeMonitorEnabled: _runtimeMonitorEnabled,
      recentActions: _recentTunnelActions,
      recentHealthSnapshots: _recentHealthSnapshots,
      lastSendFailure: store.lastSendFailure,
      lastProbeFailure: store.lastProbeFailure,
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
    AppCoreDemoStore store,
  ) async {
    switch (kind) {
      case _TunnelActionKind.apply:
        return _runtimeMonitorEnabled
            ? 'Configuration changed. Bring the tunnel up next, then re-observe runtime because live monitor is still running.'
            : 'Configuration changed. Bring the tunnel up next to activate the new settings.';
      case _TunnelActionKind.bootstrap:
        return 'Bootstrap and relay metadata refreshed from the control plane.';
      case _TunnelActionKind.up:
        final shouldStartMonitor = !_runtimeMonitorEnabled;
        _setRuntimeMonitorEnabled(true, store);
        await _refreshRuntimeFromMonitor(store);
        return shouldStartMonitor
            ? 'Live runtime monitor started automatically.'
            : 'Live runtime monitor kept running.';
      case _TunnelActionKind.down:
      case _TunnelActionKind.removePeer:
        final wasMonitoring = _runtimeMonitorEnabled;
        _setRuntimeMonitorEnabled(false, store);
        return wasMonitoring
            ? 'Live runtime monitor stopped automatically.'
            : null;
      case _TunnelActionKind.inspect:
        return null;
      case _TunnelActionKind.recover:
        final shouldStartMonitor = !_runtimeMonitorEnabled;
        _setRuntimeMonitorEnabled(true, store);
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
        interfaceName: 'utun9',
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

class _DeviceStateCard extends StatelessWidget {
  const _DeviceStateCard({
    required this.device,
    required this.node,
    required this.bootstrap,
    required this.controlStatus,
    required this.relayTicket,
    required this.lastSendBytes,
    required this.lastSendFailure,
    required this.lastProbe,
    required this.lastProbeFailure,
    required this.connectionState,
    required this.tunnelRuntimeView,
    required this.tunnelDebugError,
    required this.error,
    required this.activeAction,
    required this.recentActions,
    required this.recentHealthSnapshots,
    required this.lastTunnelActionReport,
    required this.runtimeMonitorEnabled,
    required this.onBootstrap,
    required this.onApply,
    required this.onRecover,
    required this.onUp,
    required this.onInspect,
    required this.onDown,
    required this.busy,
  });

  final DeviceModel? device;
  final NodeModel? node;
  final BootstrapModel? bootstrap;
  final ControlStatusModel? controlStatus;
  final RelayTicketModel? relayTicket;
  final int? lastSendBytes;
  final SendFailure? lastSendFailure;
  final DataPlaneProbeModel? lastProbe;
  final ProbeFailure? lastProbeFailure;
  final ConnectionStateModel connectionState;
  final WireGuardTunnelRuntimeView? tunnelRuntimeView;
  final String? tunnelDebugError;
  final String? error;
  final _TunnelActionEvent? activeAction;
  final List<_TunnelActionEvent> recentActions;
  final List<_TunnelHealthSnapshot> recentHealthSnapshots;
  final TunnelActionReport? lastTunnelActionReport;
  final bool runtimeMonitorEnabled;
  final Future<void> Function() onBootstrap;
  final Future<void> Function() onApply;
  final Future<void> Function() onRecover;
  final Future<void> Function() onUp;
  final Future<void> Function() onInspect;
  final Future<void> Function() onDown;
  final bool busy;

  @override
  Widget build(BuildContext context) {
    final runtime = tunnelRuntimeView;
    final control = controlStatus;
    final theme = Theme.of(context);
    final stagePlan = _deriveTunnelStagePlan(
      runtime: runtime,
      controlStatus: controlStatus,
      activeAction: activeAction,
      recentActions: recentActions,
      tunnelDebugError: tunnelDebugError,
      error: error,
    );
    final healthSummary = _deriveTunnelSessionHealthSummary(
      runtime: runtime,
      controlStatus: controlStatus,
      connectionState: connectionState,
      lastProbe: lastProbe,
      tunnelDebugError: tunnelDebugError,
      error: error,
      runtimeMonitorEnabled: runtimeMonitorEnabled,
      recentActions: recentActions,
      recentHealthSnapshots: recentHealthSnapshots,
      lastSendFailure: lastSendFailure,
      lastProbeFailure: lastProbeFailure,
    );
    final phaseGuidance = _deriveTunnelActionPhaseGuidance(
      lastTunnelActionReport,
    );
    final latestAction =
        activeAction ?? (recentActions.isEmpty ? null : recentActions.first);
    return DesktopSurfaceCard(
      key: AppTestKeys.devicesStateCard,
      title: 'Runtime State',
      subtitle:
          'Control plane, relay fallback, send/probe, and PacketTunnel runtime snapshots are summarized here.',
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Wrap(
            spacing: 10,
            runSpacing: 10,
            children: [
              DesktopBadge(label: 'connection ${connectionState.status}'),
              DesktopBadge(label: 'path ${connectionState.path?.name ?? '-'}'),
              DesktopBadge(label: 'tunnel ${runtime?.state ?? 'idle'}'),
            ],
          ),
          const SizedBox(height: 16),
          _DevicesSubsection(
            title: 'Pipeline Overview',
            child: _TunnelStageOverview(
              plan: stagePlan,
              latestAction: latestAction,
              sessionHealth: healthSummary.health,
            ),
          ),
          const SizedBox(height: 16),
          _DevicesSubsection(
            title: 'Control Plane',
            child: DesktopKeyValueList(
              entries: [
                DesktopKeyValueEntry(
                  label: 'Device',
                  value: device?.deviceId ?? 'none',
                ),
                DesktopKeyValueEntry(
                  label: 'Platform',
                  value: device?.platform ?? '-',
                ),
                DesktopKeyValueEntry(
                  label: 'Virtual IP',
                  value: device?.virtualIp ?? '-',
                ),
                DesktopKeyValueEntry(
                  label: 'Node',
                  value: node?.nodeId ?? 'none',
                ),
                DesktopKeyValueEntry(
                  label: 'Connection',
                  value: connectionState.status,
                ),
                DesktopKeyValueEntry(
                  label: 'Control WS',
                  value: control?.status ?? 'none',
                ),
                DesktopKeyValueEntry(
                  label: 'Path',
                  value: connectionState.path?.name ?? '-',
                ),
                DesktopKeyValueEntry(
                  label: 'Session token',
                  value: control?.sessionTokenPresent == true ? 'present' : '-',
                ),
                DesktopKeyValueEntry(
                  label: 'Peers',
                  value: control?.peerCount.toString() ?? '0',
                ),
              ],
            ),
          ),
          const SizedBox(height: 16),
          _DevicesSubsection(
            title: 'Relay & Overlay',
            child: DesktopKeyValueList(
              entries: [
                DesktopKeyValueEntry(
                  label: 'Relay ticket',
                  value: relayTicket?.ticketId ?? 'none',
                ),
                DesktopKeyValueEntry(
                  label: 'Bootstrap',
                  value: bootstrap?.relay.defaultClusterId ?? '-',
                ),
                DesktopKeyValueEntry(
                  label: 'Networks',
                  value: '${bootstrap?.networks.length ?? 0}',
                ),
                DesktopKeyValueEntry(
                  label: 'Control plans',
                  value: control?.connectPlanCount.toString() ?? '0',
                ),
                DesktopKeyValueEntry(
                  label: 'Suggested path',
                  value: _preferredControlPathLabel(control),
                ),
                DesktopKeyValueEntry(
                  label: 'Suggested relay',
                  value: _preferredControlRelayLabel(control),
                ),
                DesktopKeyValueEntry(
                  label: 'Tunnel peer',
                  value: runtime?.peerVirtualIp ?? '-',
                ),
                DesktopKeyValueEntry(
                  label: 'Endpoint',
                  value: runtime?.selectedEndpoint ?? '-',
                ),
              ],
            ),
          ),
          const SizedBox(height: 16),
          _DevicesSubsection(
            title: 'Pipeline Status',
            child: DesktopKeyValueList(
              entries: [
                DesktopKeyValueEntry(
                  label: 'Current phase',
                  value: phaseGuidance.currentPhaseLabel ??
                      stagePlan.steps
                          .firstWhere(
                            (step) => step.state == _TunnelStageState.current,
                            orElse: () => stagePlan.steps.lastWhere(
                              (step) =>
                                  step.state == _TunnelStageState.complete,
                              orElse: () => stagePlan.steps.first,
                            ),
                          )
                          .title,
                ),
                DesktopKeyValueEntry(
                  label: 'Next step',
                  value: phaseGuidance.nextStepLabel ?? stagePlan.nextStepLabel,
                ),
                DesktopKeyValueEntry(
                  label: 'Signal source',
                  value: phaseGuidance.signalSourceLabel ?? '-',
                ),
                DesktopKeyValueEntry(
                  label: 'Last action',
                  value: latestAction?.label ?? 'none',
                ),
                DesktopKeyValueEntry(
                  label: 'Last outcome',
                  value: latestAction == null
                      ? 'none'
                      : '${_statusLabel(latestAction.status)} at ${_formatActionTime(latestAction.occurredAt)}',
                ),
                DesktopKeyValueEntry(
                  label: 'Session health',
                  value: _sessionHealthLabel(healthSummary.health),
                ),
                DesktopKeyValueEntry(
                  label: 'Why',
                  value: healthSummary.reason,
                ),
                DesktopKeyValueEntry(
                  label: 'Action',
                  value: healthSummary.recommendedAction,
                ),
                DesktopKeyValueEntry(
                  label: 'Data plane signal',
                  value: healthSummary.dataPlaneSignal ?? '-',
                ),
                DesktopKeyValueEntry(
                  label: 'Recommendation',
                  value: healthSummary.recommendationMatch ?? '-',
                ),
                DesktopKeyValueEntry(
                  label: 'Monitor',
                  value: runtimeMonitorEnabled ? 'auto-refresh on' : 'manual',
                ),
                DesktopKeyValueEntry(
                  label: 'Control sync',
                  value: control?.networkMapPresent == true
                      ? 'map active'
                      : (control?.status ?? 'none'),
                ),
              ],
            ),
          ),
          const SizedBox(height: 16),
          _DevicesSubsection(
            title: 'Health Guidance',
            child: _TunnelHealthSummaryPanel(
              summary: healthSummary,
              recentSnapshots: recentHealthSnapshots,
              busy: busy,
              onPrimaryAction: () => _runHealthAction(
                healthSummary.primaryAction,
                onBootstrap: onBootstrap,
                onRecover: onRecover,
              ),
              onSecondaryAction: () => _runHealthAction(
                healthSummary.secondaryAction,
                onBootstrap: onBootstrap,
                onRecover: onRecover,
              ),
            ),
          ),
          const SizedBox(height: 16),
          _DevicesSubsection(
            title: 'Tunnel Runtime',
            child: _TunnelRuntimeSummary(
              tunnelRuntimeView: runtime,
              tunnelDebugError: tunnelDebugError,
              error: error,
              dense: true,
            ),
          ),
          const SizedBox(height: 16),
          _DevicesSubsection(
            title: 'Traffic & Diagnostics',
            child: DesktopKeyValueList(
              entries: [
                DesktopKeyValueEntry(
                  label: 'Send bytes',
                  value: lastSendBytes?.toString() ?? '-',
                ),
                DesktopKeyValueEntry(
                  label: 'Send failure',
                  value: lastSendFailure?.summary ?? '-',
                ),
                DesktopKeyValueEntry(
                  label: 'Probe',
                  value: lastProbe?.probeId ?? 'none',
                ),
                DesktopKeyValueEntry(
                  label: 'Probe RTT',
                  value: lastProbe?.replyRttMs?.toString() ?? '-',
                ),
                DesktopKeyValueEntry(
                  label: 'Probe failure',
                  value: lastProbeFailure?.summary ?? '-',
                ),
              ],
            ),
          ),
          const SizedBox(height: 16),
          _DevicesSubsection(
            title: 'Events & Errors',
            child: DefaultTextStyle(
              style: theme.textTheme.bodyMedium?.copyWith(
                    color: theme.colorScheme.onSurfaceVariant,
                    height: 1.45,
                  ) ??
                  const TextStyle(),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text('Backend error: ${runtime?.backendLastError ?? '-'}'),
                  const SizedBox(height: 8),
                  Text(
                      'Tunnel error: ${runtime?.lastError ?? tunnelDebugError ?? '-'}'),
                  const SizedBox(height: 8),
                  Text('Client error: ${error ?? '-'}'),
                ],
              ),
            ),
          ),
          if (error != null || runtime?.backendLastError != null) ...[
            const SizedBox(height: 12),
            Text(
              runtime?.backendLastError ?? error!,
              style: TextStyle(color: theme.colorScheme.error),
            ),
          ],
        ],
      ),
    );
  }

  Future<void> _runHealthAction(
    _TunnelActionKind? kind, {
    required Future<void> Function() onBootstrap,
    required Future<void> Function() onRecover,
  }) {
    if (kind == null) {
      return Future<void>.value();
    }
    return switch (kind) {
      _TunnelActionKind.apply => onApply(),
      _TunnelActionKind.bootstrap => onBootstrap(),
      _TunnelActionKind.recover => onRecover(),
      _TunnelActionKind.up => onUp(),
      _TunnelActionKind.inspect => onInspect(),
      _TunnelActionKind.down => onDown(),
      _TunnelActionKind.removePeer => onDown(),
    };
  }
}

class _TunnelRuntimeSummary extends StatelessWidget {
  const _TunnelRuntimeSummary({
    required this.tunnelRuntimeView,
    required this.tunnelDebugError,
    required this.error,
    this.dense = false,
  });

  final WireGuardTunnelRuntimeView? tunnelRuntimeView;
  final String? tunnelDebugError;
  final String? error;
  final bool dense;

  @override
  Widget build(BuildContext context) {
    final runtime = tunnelRuntimeView;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Wrap(
          spacing: 10,
          runSpacing: 10,
          children: [
            DesktopBadge(
              label: 'state ${runtime?.state ?? 'idle'}',
            ),
            DesktopBadge(
              label: 'backend ${runtime?.backendState ?? 'unavailable'}',
            ),
            DesktopBadge(
              label: 'engine ${runtime?.debugEngineMode ?? '-'}',
            ),
          ],
        ),
        const SizedBox(height: 14),
        DesktopKeyValueList(
          entries: [
            DesktopKeyValueEntry(
              label: 'Transport',
              value: runtime?.transport ?? '-',
            ),
            DesktopKeyValueEntry(
              label: 'Peer',
              value: runtime?.peerVirtualIp ?? '-',
            ),
            DesktopKeyValueEntry(
              label: 'Endpoint',
              value: runtime?.selectedEndpoint ?? '-',
            ),
            DesktopKeyValueEntry(
              label: 'Interface',
              value: runtime?.interfaceName ?? '-',
            ),
            DesktopKeyValueEntry(
              label: 'Backend',
              value:
                  '${runtime?.backendName ?? '-'} / ${runtime?.backendState ?? '-'}',
            ),
            DesktopKeyValueEntry(
              label: 'RX',
              value:
                  '${runtime?.packetRxCount ?? 0} pkt / ${runtime?.packetRxBytes ?? 0} B',
            ),
            DesktopKeyValueEntry(
              label: 'TX',
              value:
                  '${runtime?.packetTxCount ?? 0} pkt / ${runtime?.packetTxBytes ?? 0} B',
            ),
            DesktopKeyValueEntry(
              label: 'Last packet',
              value: runtime?.lastPacketAtMs?.toString() ?? '-',
            ),
            DesktopKeyValueEntry(
              label: 'Last apply',
              value: runtime?.lastAppliedAtMs?.toString() ?? '-',
            ),
            if (!dense)
              DesktopKeyValueEntry(
                label: 'Last start',
                value: runtime?.backendLastStartedAtMs?.toString() ?? '-',
              ),
            if (!dense)
              DesktopKeyValueEntry(
                label: 'Peer IP',
                value: runtime?.backendPeerVirtualIp ?? '-',
              ),
            DesktopKeyValueEntry(
              label: 'Error',
              value: runtime?.backendLastError ??
                  runtime?.lastError ??
                  tunnelDebugError ??
                  error ??
                  '-',
            ),
          ],
        ),
      ],
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

class _TunnelOperationsDesk extends StatelessWidget {
  const _TunnelOperationsDesk({
    required this.tunnelRuntimeView,
    required this.controlStatus,
    required this.tunnelDebugError,
    required this.error,
    required this.busy,
    required this.connectionState,
    required this.peerVirtualIp,
    required this.activeAction,
    required this.recentActions,
    required this.recentHealthSnapshots,
    required this.lastTunnelActionReport,
    required this.lastProbe,
    required this.lastSendFailure,
    required this.lastProbeFailure,
    required this.onBootstrap,
    required this.onRecover,
    required this.onApply,
    required this.onUp,
    required this.onInspect,
    required this.onDown,
    required this.runtimeMonitorEnabled,
    required this.onToggleRuntimeMonitor,
  });

  final WireGuardTunnelRuntimeView? tunnelRuntimeView;
  final ControlStatusModel? controlStatus;
  final String? tunnelDebugError;
  final String? error;
  final bool busy;
  final ConnectionStateModel connectionState;
  final String peerVirtualIp;
  final _TunnelActionEvent? activeAction;
  final List<_TunnelActionEvent> recentActions;
  final List<_TunnelHealthSnapshot> recentHealthSnapshots;
  final TunnelActionReport? lastTunnelActionReport;
  final DataPlaneProbeModel? lastProbe;
  final SendFailure? lastSendFailure;
  final ProbeFailure? lastProbeFailure;
  final Future<void> Function() onBootstrap;
  final Future<void> Function() onRecover;
  final Future<void> Function() onApply;
  final Future<void> Function() onUp;
  final Future<void> Function() onInspect;
  final Future<void> Function() onDown;
  final bool runtimeMonitorEnabled;
  final ValueChanged<bool> onToggleRuntimeMonitor;

  @override
  Widget build(BuildContext context) {
    final runtime = tunnelRuntimeView;
    final stagePlan = _deriveTunnelStagePlan(
      runtime: runtime,
      controlStatus: controlStatus,
      activeAction: activeAction,
      recentActions: recentActions,
      tunnelDebugError: tunnelDebugError,
      error: error,
    );
    final healthSummary = _deriveTunnelSessionHealthSummary(
      runtime: runtime,
      controlStatus: controlStatus,
      connectionState: connectionState,
      lastProbe: lastProbe,
      tunnelDebugError: tunnelDebugError,
      error: error,
      runtimeMonitorEnabled: runtimeMonitorEnabled,
      recentActions: recentActions,
      recentHealthSnapshots: recentHealthSnapshots,
      lastSendFailure: lastSendFailure,
      lastProbeFailure: lastProbeFailure,
    );
    final phaseGuidance = _deriveTunnelActionPhaseGuidance(
      lastTunnelActionReport,
    );
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        if (activeAction != null) ...[
          _TunnelActionBanner(
            event: activeAction!,
            nextStepLabel: stagePlan.nextStepLabel,
          ),
          const SizedBox(height: 16),
        ],
        DesktopInsetBlock(
          title: 'Recommended Flow',
          child: _TunnelStageFlow(
            plan: stagePlan,
            busy: busy,
            onRunNext: () => _runStageAction(stagePlan.primaryAction),
          ),
        ),
        const SizedBox(height: 16),
        if (activeAction == null) ...[
          const SizedBox(height: 16),
        ],
        Wrap(
          spacing: 10,
          runSpacing: 10,
          children: [
            DesktopMetricPill(
              label: 'Runtime',
              value: runtime?.state ?? 'idle',
            ),
            DesktopMetricPill(
              label: 'Backend',
              value: runtime?.backendState ?? 'unavailable',
            ),
            DesktopMetricPill(
              label: 'Health',
              value: _sessionHealthLabel(healthSummary.health),
              backgroundColor: _sessionHealthBackgroundColor(
                context,
                healthSummary.health,
              ),
              foregroundColor: _sessionHealthForegroundColor(
                context,
                healthSummary.health,
              ),
              borderColor: _sessionHealthBorderColor(
                context,
                healthSummary.health,
              ),
            ),
            DesktopMetricPill(
              label: 'Traffic',
              value: _trafficValue(runtime),
            ),
            DesktopMetricPill(
              label: 'Monitor',
              value: runtimeMonitorEnabled ? 'live' : 'off',
            ),
            if (phaseGuidance.currentPhaseLabel != null)
              DesktopMetricPill(
                label: 'Phase',
                value: phaseGuidance.currentPhaseLabel!,
              ),
          ],
        ),
        if (phaseGuidance.currentPhaseLabel != null) ...[
          const SizedBox(height: 12),
          Text(
            'Native phase: ${phaseGuidance.currentPhaseLabel!}${phaseGuidance.signalSourceLabel == null ? '' : ' via ${phaseGuidance.signalSourceLabel!}'}',
            style: Theme.of(context).textTheme.bodySmall?.copyWith(
                  color: Theme.of(context).colorScheme.onSurfaceVariant,
                  fontWeight: FontWeight.w700,
                ),
          ),
          if (phaseGuidance.nextStepLabel != null) ...[
            const SizedBox(height: 6),
            Text(
              'Phase-driven next step: ${phaseGuidance.nextStepLabel!}',
              style: Theme.of(context).textTheme.bodySmall?.copyWith(
                    color: Theme.of(context).colorScheme.onSurfaceVariant,
                    height: 1.4,
                  ),
            ),
          ],
        ],
        const SizedBox(height: 16),
        DesktopInsetBlock(
          title: 'Health Guidance',
          child: _TunnelHealthSummaryPanel(
            summary: healthSummary,
            recentSnapshots: recentHealthSnapshots,
            busy: busy,
            onPrimaryAction: () => _runStageAction(healthSummary.primaryAction),
            onSecondaryAction: () =>
                _runStageAction(healthSummary.secondaryAction),
          ),
        ),
        const SizedBox(height: 16),
        DesktopInsetBlock(
          title: 'Live Runtime Monitor',
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              SwitchListTile(
                contentPadding: EdgeInsets.zero,
                title: const Text('Auto-refresh runtime every 4 seconds'),
                subtitle: Text(
                  runtimeMonitorEnabled
                      ? 'The workbench will keep polling runtime while the page is open.'
                      : 'Leave this off to inspect runtime manually after each step.',
                ),
                value: runtimeMonitorEnabled,
                onChanged: busy ? null : onToggleRuntimeMonitor,
              ),
              const SizedBox(height: 8),
              Text(
                runtimeMonitorEnabled
                    ? 'Live monitor is active. Runtime cards will follow backend and traffic changes automatically.'
                    : 'Manual mode is active. Use View Runtime or the recommended flow button to refresh state.',
                style: Theme.of(context).textTheme.bodySmall?.copyWith(
                      color: Theme.of(context).colorScheme.onSurfaceVariant,
                      height: 1.45,
                    ),
              ),
            ],
          ),
        ),
        const SizedBox(height: 16),
        DesktopKeyValueList(
          entries: [
            DesktopKeyValueEntry(
              label: 'Connection',
              value: connectionState.status,
            ),
            DesktopKeyValueEntry(
              label: 'Peer',
              value: runtime?.peerVirtualIp ?? peerVirtualIp.ifEmpty('-'),
            ),
            DesktopKeyValueEntry(
              label: 'Engine',
              value: runtime?.debugEngineMode ?? '-',
            ),
            DesktopKeyValueEntry(
              label: 'Backend',
              value: runtime?.backendName ?? '-',
            ),
            DesktopKeyValueEntry(
              label: 'Last packet',
              value: runtime?.lastPacketAtMs?.toString() ?? '-',
            ),
            DesktopKeyValueEntry(
              label: 'Error',
              value: runtime?.backendLastError ??
                  runtime?.lastError ??
                  tunnelDebugError ??
                  error ??
                  '-',
            ),
          ],
        ),
        const SizedBox(height: 16),
        DesktopInsetBlock(
          title: 'Recovery Guidance',
          child: _TunnelRecoveryGuide(
            plan: stagePlan,
            busy: busy,
            onPrimaryAction: () =>
                _runStageAction(stagePlan.primaryRecoveryAction),
            onSecondaryAction: () =>
                _runStageAction(stagePlan.secondaryRecoveryAction),
          ),
        ),
        const SizedBox(height: 16),
        DesktopInsetBlock(
          title: 'Recent Actions',
          child: recentActions.isEmpty
              ? const Text('No tunnel actions yet.')
              : Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    for (var index = 0;
                        index < recentActions.length;
                        index++) ...[
                      _TunnelActionTimelineRow(event: recentActions[index]),
                      if (index != recentActions.length - 1)
                        const SizedBox(height: 12),
                    ],
                  ],
                ),
        ),
      ],
    );
  }

  Future<void> _runStageAction(_TunnelActionKind? kind) {
    if (kind == null) {
      return Future<void>.value();
    }
    return switch (kind) {
      _TunnelActionKind.apply => onApply(),
      _TunnelActionKind.bootstrap => onBootstrap(),
      _TunnelActionKind.recover => onRecover(),
      _TunnelActionKind.up => onUp(),
      _TunnelActionKind.inspect => onInspect(),
      _TunnelActionKind.down => onDown(),
      _TunnelActionKind.removePeer => onDown(),
    };
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
