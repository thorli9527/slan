import 'package:flutter/material.dart';

import '../../infra/app_core/app_core_scope.dart';
import '../../infra/app_core/models.dart';

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

  @override
  void dispose() {
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
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final store = AppCoreScope.demo;
    return AnimatedBuilder(
      animation: store,
      builder: (context, _) {
        return ListView(
          padding: const EdgeInsets.all(16),
          children: [
            Text(
              'Register Device',
              style: Theme.of(context).textTheme.titleMedium,
            ),
            const SizedBox(height: 12),
            TextField(
              controller: _nameController,
              decoration: const InputDecoration(labelText: 'Name'),
            ),
            const SizedBox(height: 12),
            TextField(
              controller: _platformController,
              decoration: const InputDecoration(labelText: 'Platform'),
            ),
            const SizedBox(height: 12),
            TextField(
              controller: _machineIdController,
              decoration: const InputDecoration(labelText: 'Machine ID'),
            ),
            const SizedBox(height: 12),
            TextField(
              controller: _publicKeyController,
              decoration: const InputDecoration(labelText: 'Public Key'),
            ),
            const SizedBox(height: 16),
            FilledButton(
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
            const SizedBox(height: 24),
            Text(
              'Register Node',
              style: Theme.of(context).textTheme.titleMedium,
            ),
            const SizedBox(height: 12),
            TextField(
              controller: _nodeIdController,
              decoration: const InputDecoration(labelText: 'Node ID'),
            ),
            const SizedBox(height: 12),
            TextField(
              controller: _nodePublicKeyController,
              decoration: const InputDecoration(labelText: 'Node Public Key'),
            ),
            const SizedBox(height: 16),
            OutlinedButton(
              onPressed: store.busy || store.device == null
                  ? null
                  : () => store.registerNode(
                        deviceId: store.device!.deviceId,
                        nodeId: _nodeIdController.text.trim(),
                        nodePublicKey: _nodePublicKeyController.text.trim(),
                      ),
              child: const Text('Register Node'),
            ),
            const SizedBox(height: 24),
            Text(
              'Bootstrap',
              style: Theme.of(context).textTheme.titleMedium,
            ),
            const SizedBox(height: 12),
            TextField(
              controller: _bootstrapNodeIdController,
              decoration: const InputDecoration(
                labelText: 'Node ID',
                hintText: 'Leave empty to use current node',
              ),
            ),
            const SizedBox(height: 12),
            TextField(
              controller: _networkIdController,
              decoration: const InputDecoration(labelText: 'Network ID'),
            ),
            const SizedBox(height: 16),
            OutlinedButton(
              onPressed: store.busy
                  ? null
                  : () => store.loadBootstrap(
                        nodeId: _bootstrapNodeIdController.text,
                        networkId: _networkIdController.text,
                      ),
              child: const Text('Load Bootstrap'),
            ),
            const SizedBox(height: 24),
            Text(
              'Connect With Fallback',
              style: Theme.of(context).textTheme.titleMedium,
            ),
            const SizedBox(height: 4),
            const Text(
              'Use peer node id starting with fail- to simulate P2P failure and trigger relay fallback.',
            ),
            const SizedBox(height: 12),
            TextField(
              controller: _peerNodeIdController,
              decoration: const InputDecoration(labelText: 'Peer Node ID'),
            ),
            const SizedBox(height: 12),
            TextField(
              controller: _reasonController,
              decoration: const InputDecoration(labelText: 'Failure Reason'),
            ),
            const SizedBox(height: 16),
            Wrap(
              spacing: 12,
              runSpacing: 12,
              children: [
                FilledButton(
                  onPressed: store.busy
                      ? null
                      : () => store.connectWithFallback(
                            networkId: _networkIdController.text.trim(),
                            peerNodeId: _peerNodeIdController.text.trim(),
                            reason: _reasonController.text.trim(),
                          ),
                  child: const Text('Connect'),
                ),
                OutlinedButton(
                  onPressed: store.busy ? null : store.disconnect,
                  child: const Text('Disconnect'),
                ),
              ],
            ),
            const SizedBox(height: 24),
            _DeviceStateCard(
              device: store.device,
              node: store.node,
              bootstrap: store.bootstrap,
              relayTicket: store.relayTicket,
              connectionState: store.connectionState,
              error: store.error,
            ),
          ],
        );
      },
    );
  }
}

class _DeviceStateCard extends StatelessWidget {
  const _DeviceStateCard({
    required this.device,
    required this.node,
    required this.bootstrap,
    required this.relayTicket,
    required this.connectionState,
    required this.error,
  });

  final DeviceModel? device;
  final NodeModel? node;
  final BootstrapModel? bootstrap;
  final RelayTicketModel? relayTicket;
  final ConnectionStateModel connectionState;
  final String? error;

  @override
  Widget build(BuildContext context) {
    return Card(
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text('device: ${device?.deviceId ?? 'none'}'),
            if (device != null) ...[
              Text('platform: ${device!.platform}'),
              Text('virtualIp: ${device!.virtualIp ?? '-'}'),
            ],
            const SizedBox(height: 12),
            Text('node: ${node?.nodeId ?? 'none'}'),
            Text('node device: ${node?.deviceId ?? '-'}'),
            const SizedBox(height: 12),
            Text('connection: ${connectionState.status}'),
            Text('path: ${connectionState.path?.name ?? '-'}'),
            Text('reason: ${connectionState.reason ?? '-'}'),
            const SizedBox(height: 12),
            Text('bootstrap relay: ${bootstrap?.relay.udpEndpoint ?? '-'}'),
            Text('bootstrap networks: ${bootstrap?.networks.length ?? 0}'),
            const SizedBox(height: 12),
            Text('relay ticket: ${relayTicket?.ticketId ?? 'none'}'),
            Text('relay session: ${relayTicket?.sessionId ?? '-'}'),
            Text(
                'relay nodes: ${relayTicket?.srcNodeId ?? '-'} -> ${relayTicket?.dstNodeId ?? '-'}'),
            Text('relay url: ${relayTicket?.relayUrl ?? '-'}'),
            if (error != null) ...[
              const SizedBox(height: 12),
              Text(
                error!,
                style: TextStyle(color: Theme.of(context).colorScheme.error),
              ),
            ],
          ],
        ),
      ),
    );
  }
}
