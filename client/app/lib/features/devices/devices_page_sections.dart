part of 'devices_page.dart';

extension _DevicesPageSections on _DevicesPageState {
  String _helperStatusValue(AppCoreHelperStatusModel? status) {
    if (status == null) {
      return 'unknown';
    }
    if (!status.helperReachable) {
      return 'unreachable';
    }
    if (status.tunnelBackendRunning) {
      return 'running';
    }
    return status.sessionPresent ? 'ready' : 'waiting';
  }

  Widget _buildHelperStatusSection(
    BuildContext context,
    AppSessionStore sessionStore,
    AppSessionController sessionController,
  ) {
    final status = sessionStore.helperStatus;
    return _DevicesWorkbenchCard(
      title: 'Helper Service',
      subtitle: 'Runtime ownership, persisted state, and control endpoint.',
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Wrap(
            spacing: 10,
            runSpacing: 10,
            children: [
              DesktopBadge(label: 'helper ${_helperStatusValue(status)}'),
              DesktopBadge(
                label:
                    'control ${status?.configuredControlBaseUrl ?? AppCoreScope.controlBaseUrl ?? 'unset'}',
              ),
              if (status?.currentNetworkId != null)
                DesktopBadge(label: 'network ${status!.currentNetworkId}'),
            ],
          ),
          const SizedBox(height: 12),
          if (status?.tunnelLastError?.trim().isNotEmpty == true)
            Text(
              status!.tunnelLastError!,
              style: TextStyle(color: Theme.of(context).colorScheme.error),
            )
          else
            Text(
              status == null
                  ? 'No helper status has been collected yet.'
                  : 'session=${status.sessionPresent} bootstrap=${status.bootstrapPresent} runtime=${status.tunnelRuntimePresent}',
              style: Theme.of(context).textTheme.bodySmall,
            ),
          const SizedBox(height: 12),
          Align(
            alignment: Alignment.centerLeft,
            child: OutlinedButton.icon(
              onPressed: sessionStore.busy
                  ? null
                  : () => sessionController.refreshHelperStatus(),
              icon: const Icon(Icons.health_and_safety_rounded),
              label: const Text('Refresh Helper'),
            ),
          ),
        ],
      ),
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
          'Register a local device, attach a node, bootstrap overlay state, drive relay fallback, and inspect the local mesh runtime without leaving the desktop client.',
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
                  'Local mesh runtime actions are routed through the native host backend, so apply, bring-up, and runtime inspection can now run through the desktop bridge.',
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
                      OutlinedButton(
                        key: AppTestKeys.devicesRefreshInventoryButton,
                        onPressed:
                            sessionStore.busy || sessionStore.session == null
                                ? null
                                : sessionController.refreshDeviceInventory,
                        child: const Text('Refresh Devices'),
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
    final controlPlan =
        connectPlanForPeer(sessionStore.controlStatus, peerNodeId);
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
                onPressed:
                    sessionStore.busy ? null : sessionController.disconnect,
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
                          : () => tunnelController.refreshPlatformInstallPlan(),
                      child: const Text('Install Plan'),
                    ),
                  ],
                ),
                const SizedBox(height: 16),
                DesktopKeyValueList(
                  entries: [
                    DesktopKeyValueEntry(
                      label: 'Platform',
                      value: _formatPlatformSummary(
                          tunnelStore.platformDoctor?.platform ??
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
                      value: tunnelStore.platformInstallPlan?.packages
                              .join(', ') ??
                          '-',
                    ),
                    DesktopKeyValueEntry(
                      label: 'Driver modes',
                      value: tunnelStore
                              .platformInstallPlan?.supportedDriverModes
                              .join(', ') ??
                          '-',
                    ),
                    DesktopKeyValueEntry(
                      label: 'Warnings',
                      value: tunnelStore.platformInstallPlan?.warnings
                              .join('; ') ??
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
    return checks.map((check) => '${check.name}:${check.status}').join(', ');
  }

  Widget _buildTunnelSection(
    BuildContext context,
    AppSessionStore sessionStore,
    AppTunnelStore tunnelStore, {
    required AppSessionController sessionController,
    required AppTunnelController tunnelController,
    bool isDesktop = false,
  }) {
    final serviceOwned = _serviceOwnsRuntime;
    final actionButtons =
        _buildTunnelActionButtons(sessionStore, tunnelController);
    if (serviceOwned) {
      return _DevicesWorkbenchCard(
        title: 'Service Network Runtime',
        subtitle:
            'app-core-service owns tunnel, DNS, control-sync, and network-state reporting; Flutter only triggers enable, sync, and disable actions.',
        child: Column(
          children: [
            _DevicesSubsection(
              title: 'Runtime Control',
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  actionButtons,
                  const SizedBox(height: 12),
                  Text(
                    sessionStore.notice ??
                        tunnelStore.lastTunnelActionReport?.detail ??
                        'Service runtime is ready for network actions.',
                    style: Theme.of(context).textTheme.bodyMedium,
                  ),
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
                peerVirtualIp: tunnelStore.tunnelRuntimeView?.peerVirtualIp ??
                    sessionStore.device?.virtualIp ??
                    '-',
                activeAction: _activeTunnelAction,
                recentActions: _recentTunnelActions,
                recentHealthSnapshots: _recentHealthSnapshots,
                lastTunnelActionReport: tunnelStore.lastTunnelActionReport,
                lastProbe: tunnelStore.lastProbe,
                lastSendFailure: tunnelStore.lastSendFailure,
                lastProbeFailure: tunnelStore.lastProbeFailure,
                onRecover: () => _handleRecoverSession(tunnelController),
                onBootstrap: () => _handleBootstrapRefresh(sessionController),
                onApply: () => _handleTunnelApply(tunnelController),
                onUp: () => _handleTunnelUp(tunnelController),
                onInspect: () => _handleTunnelInspect(tunnelController),
                onDown: () => _handleTunnelDown(tunnelController),
                runtimeMonitorEnabled: _runtimeMonitorEnabled,
                onToggleRuntimeMonitor: (enabled) =>
                    _setRuntimeMonitorEnabled(enabled),
              ),
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
        ),
      );
    }
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
    final serviceOwned = _serviceOwnsRuntime;
    return Wrap(
      spacing: 12,
      runSpacing: 12,
      children: [
        FilledButton(
          key: AppTestKeys.devicesTunnelApplyButton,
          onPressed: sessionStore.busy
              ? null
              : () => _handleTunnelApply(tunnelController),
          child: Text(serviceOwned ? 'Enable Network' : 'Apply Mesh Config'),
        ),
        OutlinedButton(
          key: AppTestKeys.devicesTunnelUpButton,
          onPressed: sessionStore.busy
              ? null
              : () => _handleTunnelUp(tunnelController),
          child: Text(serviceOwned ? 'Start Runtime' : 'Start Mesh'),
        ),
        OutlinedButton(
          key: AppTestKeys.devicesTunnelViewButton,
          onPressed: sessionStore.busy
              ? null
              : () => _handleTunnelInspect(tunnelController),
          child: Text(serviceOwned ? 'Sync State' : 'View Runtime'),
        ),
        OutlinedButton(
          key: AppTestKeys.devicesTunnelDownButton,
          onPressed: sessionStore.busy
              ? null
              : () => _handleTunnelDown(tunnelController),
          child: Text(serviceOwned ? 'Disable Network' : 'Stop Mesh'),
        ),
        OutlinedButton(
          key: AppTestKeys.devicesTunnelRemoveButton,
          onPressed: sessionStore.busy
              ? null
              : () => _handleTunnelRemovePeer(tunnelController),
          child: Text(serviceOwned ? 'Clear Runtime' : 'Remove Peer'),
        ),
      ],
    );
  }
}
