import 'package:flutter/material.dart';

import '../../infra/app_core/scope/app_core_scope.dart';
import '../shared/desktop_client_widgets.dart';

class NetworksPage extends StatelessWidget {
  const NetworksPage({super.key});

  @override
  Widget build(BuildContext context) {
    final sessionStore = AppCoreScope.sessionStore;
    return AnimatedBuilder(
      animation: sessionStore,
      builder: (context, _) {
        final network =
            sessionStore.networks.isNotEmpty ? sessionStore.networks.first : null;
        return SingleChildScrollView(
          padding: const EdgeInsets.all(16),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              DesktopHeroPanel(
                title: 'Network Details',
                description: '客户端只展示当前唯一活动网络的基础信息，不再承担创建多个网络或切换复杂网络列表的入口。',
                footer: DesktopBadge(
                  label: network == null
                      ? 'no network loaded'
                      : 'active network ${network.networkId}',
                ),
              ),
              const SizedBox(height: 16),
              DesktopSurfaceCard(
                title: 'Current Network',
                child: network == null
                    ? const Text('No active network has been prepared yet.')
                    : Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          _Line(label: 'Name', value: network.name),
                          _Line(label: 'Network ID', value: network.networkId),
                          _Line(label: 'CIDR', value: network.cidr),
                          _Line(
                              label: 'Members',
                              value: '${network.members.length}'),
                          const SizedBox(height: 16),
                          const Text('Members'),
                          const SizedBox(height: 8),
                          if (network.members.isEmpty)
                            const Text('No members joined yet.')
                          else
                            ...network.members.map(
                              (member) => Padding(
                                padding: const EdgeInsets.only(bottom: 8),
                                child: DesktopInsetBlock(
                                  title: member.deviceId,
                                  child: Text(
                                    '${member.role}${member.virtualIp == null || member.virtualIp!.isEmpty ? '' : ' · ${member.virtualIp}'}',
                                  ),
                                ),
                              ),
                            ),
                        ],
                      ),
              ),
            ],
          ),
        );
      },
    );
  }
}

class _Line extends StatelessWidget {
  const _Line({
    required this.label,
    required this.value,
  });

  final String label;
  final String value;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.only(bottom: 10),
      child: Row(
        children: [
          SizedBox(
            width: 130,
            child: Text(
              label,
              style: Theme.of(context).textTheme.bodyMedium?.copyWith(
                    color: Theme.of(context).colorScheme.onSurfaceVariant,
                  ),
            ),
          ),
          Expanded(child: Text(value)),
        ],
      ),
    );
  }
}
