import 'package:flutter/material.dart';

import '../../infra/app_core/models/models.dart';
import '../../infra/app_core/scope/app_core_scope.dart';
import '../../testing/app_test_keys.dart';
import '../shared/desktop_client_widgets.dart';

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
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (!mounted) {
        return;
      }
      AppCoreScope.demo.refreshNetworks();
    });
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
        return LayoutBuilder(
          builder: (context, constraints) {
            final isDesktop = constraints.maxWidth >= 1040;
            final child = isDesktop
                ? _buildDesktopWorkspace(context, store)
                : _buildCompactWorkspace(context, store);
            return SingleChildScrollView(
              padding: const EdgeInsets.all(16),
              child: child,
            );
          },
        );
      },
    );
  }

  Widget _buildCompactWorkspace(BuildContext context, dynamic store) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        _NetworksHero(networkCount: store.networks.length),
        const SizedBox(height: 16),
        _NetworkCreateCard(
          nameController: _nameController,
          cidrController: _cidrController,
          busy: store.busy,
          onCreate: () => store.createNetwork(
                name: _nameController.text.trim(),
                cidr: _cidrController.text.trim(),
              ),
          onRefresh: store.refreshNetworks,
        ),
        const SizedBox(height: 16),
        _NetworkListCard(networks: store.networks),
      ],
    );
  }

  Widget _buildDesktopWorkspace(BuildContext context, dynamic store) {
    return Row(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Expanded(
          child: Column(
            children: [
              _NetworksHero(networkCount: store.networks.length),
              const SizedBox(height: 16),
              _NetworkCreateCard(
                nameController: _nameController,
                cidrController: _cidrController,
                busy: store.busy,
                onCreate: () => store.createNetwork(
                      name: _nameController.text.trim(),
                      cidr: _cidrController.text.trim(),
                    ),
                onRefresh: store.refreshNetworks,
              ),
            ],
          ),
        ),
        const SizedBox(width: 16),
        Expanded(
          flex: 2,
          child: _NetworkListCard(networks: store.networks),
        ),
      ],
    );
  }
}

class _NetworksHero extends StatelessWidget {
  const _NetworksHero({required this.networkCount});

  final int networkCount;

  @override
  Widget build(BuildContext context) {
    return DesktopHeroPanel(
      title: 'Overlay Networks',
      description:
          'Create and inspect the overlay networks that feed device bootstrap and tunnel path selection.',
      backgroundColor: const Color(0xFFF1F4FB),
      footer: DesktopBadge(
        label: '$networkCount network${networkCount == 1 ? '' : 's'} loaded',
      ),
    );
  }
}

class _NetworkCreateCard extends StatelessWidget {
  const _NetworkCreateCard({
    required this.nameController,
    required this.cidrController,
    required this.busy,
    required this.onCreate,
    required this.onRefresh,
  });

  final TextEditingController nameController;
  final TextEditingController cidrController;
  final bool busy;
  final VoidCallback onCreate;
  final VoidCallback onRefresh;

  @override
  Widget build(BuildContext context) {
    return DesktopSurfaceCard(
      title: 'Create Network',
      subtitle:
          'This is still the existing Phase 1 mock control plane, but now wrapped in a desktop-oriented authoring panel.',
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          TextField(
            key: AppTestKeys.networksNameField,
            controller: nameController,
            decoration: const InputDecoration(labelText: 'Network Name'),
          ),
          const SizedBox(height: 12),
          TextField(
            key: AppTestKeys.networksCidrField,
            controller: cidrController,
            decoration: const InputDecoration(labelText: 'CIDR'),
          ),
          const SizedBox(height: 16),
          Wrap(
            spacing: 12,
            runSpacing: 12,
            children: [
              FilledButton(
                key: AppTestKeys.networksCreateButton,
                onPressed: busy ? null : onCreate,
                child: const Text('Create Network'),
              ),
              OutlinedButton(
                key: AppTestKeys.networksRefreshButton,
                onPressed: busy ? null : onRefresh,
                child: const Text('Refresh'),
              ),
            ],
          ),
        ],
      ),
    );
  }
}

class _NetworkListCard extends StatelessWidget {
  const _NetworkListCard({required this.networks});

  final List<NetworkModel> networks;

  @override
  Widget build(BuildContext context) {
    return DesktopSurfaceCard(
      title: 'Network Inventory',
      child: networks.isEmpty
          ? const Text(
              'No networks yet. Create one here, then use it from the Devices workspace.',
            )
          : Column(
              children: [
                for (final network in networks) _NetworkCard(network: network),
              ],
            ),
    );
  }
}

class _NetworkCard extends StatelessWidget {
  const _NetworkCard({required this.network});

  final NetworkModel network;

  @override
  Widget build(BuildContext context) {
    return Container(
      margin: const EdgeInsets.only(bottom: 12),
      child: DesktopInsetBlock(
        title: network.name,
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text('networkId: ${network.networkId}'),
            Text('cidr: ${network.cidr}'),
            const SizedBox(height: 8),
            Text('members: ${network.members.length}'),
            for (final member in network.members)
              Text('${member.deviceId} (${member.role}) ${member.virtualIp ?? ''}'),
          ],
        ),
      ),
    );
  }
}
