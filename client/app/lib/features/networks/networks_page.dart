import 'package:flutter/material.dart';

import '../../infra/app_core/app_core_scope.dart';
import '../../infra/app_core/models.dart';

class NetworksPage extends StatefulWidget {
  const NetworksPage({super.key});

  @override
  State<NetworksPage> createState() => _NetworksPageState();
}

class _NetworksPageState extends State<NetworksPage> {
  final _nameController = TextEditingController(text: 'home');
  final _cidrController = TextEditingController(text: '100.64.0.0/24');

  @override
  void initState() {
    super.initState();
    AppCoreScope.demo.refreshNetworks();
  }

  @override
  void dispose() {
    _nameController.dispose();
    _cidrController.dispose();
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
            TextField(
              controller: _nameController,
              decoration: const InputDecoration(labelText: 'Network Name'),
            ),
            const SizedBox(height: 12),
            TextField(
              controller: _cidrController,
              decoration: const InputDecoration(labelText: 'CIDR'),
            ),
            const SizedBox(height: 16),
            Wrap(
              spacing: 12,
              runSpacing: 12,
              children: [
                FilledButton(
                  onPressed: store.busy
                      ? null
                      : () => store.createNetwork(
                            name: _nameController.text.trim(),
                            cidr: _cidrController.text.trim(),
                          ),
                  child: const Text('Create Network'),
                ),
                OutlinedButton(
                  onPressed: store.busy ? null : store.refreshNetworks,
                  child: const Text('Refresh'),
                ),
              ],
            ),
            const SizedBox(height: 24),
            if (store.networks.isEmpty)
              const Card(
                child: ListTile(
                  title: Text('No networks'),
                  subtitle: Text('Create one to use it from the Devices tab.'),
                ),
              ),
            for (final network in store.networks)
              _NetworkCard(network: network),
          ],
        );
      },
    );
  }
}

class _NetworkCard extends StatelessWidget {
  const _NetworkCard({required this.network});

  final NetworkModel network;

  @override
  Widget build(BuildContext context) {
    return Card(
      margin: const EdgeInsets.only(bottom: 12),
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(network.name, style: Theme.of(context).textTheme.titleMedium),
            const SizedBox(height: 8),
            Text('networkId: ${network.networkId}'),
            Text('cidr: ${network.cidr}'),
            const SizedBox(height: 8),
            Text('members: ${network.members.length}'),
            for (final member in network.members)
              Text(
                  '${member.deviceId} (${member.role}) ${member.virtualIp ?? ''}'),
          ],
        ),
      ),
    );
  }
}
