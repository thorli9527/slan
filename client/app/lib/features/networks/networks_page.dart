import 'package:flutter/material.dart';

import '../../infra/app_core/models/network_models.dart';
import '../../infra/app_core/scope/app_core_scope.dart';
import '../../testing/app_test_keys.dart';
import '../shared/desktop_client_widgets.dart';

class NetworksPage extends StatefulWidget {
  const NetworksPage({super.key});

  @override
  State<NetworksPage> createState() => _NetworksPageState();
}

class _NetworksPageState extends State<NetworksPage> {
  final TextEditingController _ipAddressController =
      TextEditingController(text: '10.0.0.0');
  final TextEditingController _subnetMaskController =
      TextEditingController(text: '255.255.252.0');
  final TextEditingController _allocationStartIpController =
      TextEditingController();
  final TextEditingController _allocationEndIpController =
      TextEditingController();
  final TextEditingController _joinKeyController = TextEditingController();

  @override
  Widget build(BuildContext context) {
    final sessionStore = AppCoreScope.sessionStore;
    final sessionController = AppCoreScope.sessionController;
    return AnimatedBuilder(
      animation: sessionStore,
      builder: (context, _) {
        final selectedNetwork = sessionStore.selectedNetwork;
        return SingleChildScrollView(
          padding: const EdgeInsets.all(16),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              DesktopHeroPanel(
                title: 'Network Details',
                description:
                    'Create a network plan, confirm access with an invite code, and switch the client between joined networks.',
                trailing: IconButton.filledTonal(
                  key: AppTestKeys.networksRefreshButton,
                  onPressed: sessionStore.busy || sessionStore.session == null
                      ? null
                      : sessionController.refreshNetworks,
                  icon: const Icon(Icons.refresh_rounded),
                  tooltip: 'Refresh networks',
                ),
                footer: DesktopBadge(
                  label: selectedNetwork == null
                      ? 'no network selected'
                      : 'selected ${selectedNetwork.networkId}',
                ),
              ),
              const SizedBox(height: 16),
              DesktopSurfaceCard(
                title: 'Network Actions',
                subtitle: 'Create a network or join one from a dialog.',
                child: Wrap(
                  spacing: 12,
                  runSpacing: 12,
                  children: [
                    FilledButton.icon(
                      key: AppTestKeys.networksOpenCreateDialogButton,
                      onPressed: sessionStore.busy ||
                              sessionStore.session == null ||
                              sessionStore.device == null
                          ? null
                          : _openCreateNetworkDialog,
                      icon: const Icon(Icons.add_rounded),
                      label: const Text('Network plan'),
                    ),
                    OutlinedButton.icon(
                      key: AppTestKeys.networksOpenJoinDialogButton,
                      onPressed: sessionStore.busy ||
                              sessionStore.session == null ||
                              sessionStore.device == null
                          ? null
                          : _openJoinNetworkDialog,
                      icon: const Icon(Icons.group_add_rounded),
                      label: const Text('Access confirmation'),
                    ),
                    if (sessionStore.error != null)
                      Text(
                        sessionStore.error!,
                        style: TextStyle(
                          color: Theme.of(context).colorScheme.error,
                        ),
                      ),
                    if (sessionStore.notice != null) Text(sessionStore.notice!),
                  ],
                ),
              ),
              const SizedBox(height: 16),
              DesktopSurfaceCard(
                title: 'Current Network',
                child: selectedNetwork == null
                    ? const Text('No active network has been prepared yet.')
                    : _NetworkDetails(network: selectedNetwork),
              ),
              const SizedBox(height: 16),
              DesktopSurfaceCard(
                title: 'Joined Networks',
                subtitle: 'Switch changes which network the home toggle uses.',
                child: sessionStore.networks.isEmpty
                    ? const Text('No joined networks yet.')
                    : Column(
                        children: [
                          for (final network in sessionStore.networks) ...[
                            _NetworkSwitchTile(
                              network: network,
                              selected: network.networkId ==
                                  selectedNetwork?.networkId,
                              busy: sessionStore.busy,
                              onSwitch: () => sessionController.selectNetwork(
                                network.networkId,
                              ),
                            ),
                            if (network != sessionStore.networks.last)
                              const SizedBox(height: 10),
                          ],
                        ],
                      ),
              ),
            ],
          ),
        );
      },
    );
  }

  Future<void> _joinNetwork() async {
    await AppCoreScope.sessionController.joinNetwork(
      joinKey: _joinKeyController.text,
    );
  }

  Future<void> _createNetwork() async {
    await AppCoreScope.sessionController.createNetwork(
      name: _defaultNetworkName,
      cidr: _cidrFromAddressAndMask(
        _ipAddressController.text,
        _subnetMaskController.text,
      ),
      allocationStartIp: _allocationStartIpController.text,
      allocationEndIp: _allocationEndIpController.text,
    );
  }

  static const String _defaultNetworkName = 'My Network';

  String _cidrFromAddressAndMask(String address, String mask) {
    final ipValue = _parseIpv4(address, 'IP address is invalid');
    final maskValue = _parseIpv4(mask, 'Subnet mask is invalid');
    final prefix = _subnetMaskPrefix(maskValue);
    final network = ipValue & maskValue;
    return '${_formatIpv4(network)}/$prefix';
  }

  int _parseIpv4(String value, String message) {
    final parts = value.trim().split('.');
    if (parts.length != 4) {
      throw FormatException(message);
    }
    var result = 0;
    for (final part in parts) {
      final octet = int.tryParse(part);
      if (octet == null || octet < 0 || octet > 255) {
        throw FormatException(message);
      }
      result = (result << 8) | octet;
    }
    return result;
  }

  int _subnetMaskPrefix(int mask) {
    var seenZero = false;
    var prefix = 0;
    for (var bit = 31; bit >= 0; bit--) {
      final isOne = ((mask >> bit) & 1) == 1;
      if (isOne && seenZero) {
        throw const FormatException('Subnet mask is invalid');
      }
      if (isOne) {
        prefix++;
      } else {
        seenZero = true;
      }
    }
    if (prefix < 1 || prefix > 30) {
      throw const FormatException(
          'Subnet mask must allow usable host addresses');
    }
    return prefix;
  }

  String _formatIpv4(int value) {
    return [
      (value >> 24) & 255,
      (value >> 16) & 255,
      (value >> 8) & 255,
      value & 255,
    ].join('.');
  }

  Future<void> _openCreateNetworkDialog() async {
    await showDialog<void>(
      context: context,
      builder: (context) {
        return StatefulBuilder(
          builder: (context, setDialogState) {
            return AlertDialog(
              title: const Text('Network plan'),
              content: SingleChildScrollView(
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    TextField(
                      key: AppTestKeys.networksIpAddressField,
                      controller: _ipAddressController,
                      decoration: const InputDecoration(
                        labelText: 'IP address',
                        hintText: '10.0.0.0',
                      ),
                    ),
                    const SizedBox(height: 12),
                    TextField(
                      key: AppTestKeys.networksSubnetMaskField,
                      controller: _subnetMaskController,
                      decoration: const InputDecoration(
                        labelText: 'Subnet mask',
                        hintText: '255.255.252.0',
                      ),
                    ),
                    const SizedBox(height: 12),
                    Align(
                      alignment: Alignment.centerLeft,
                      child: Text(
                        'DHCP',
                        style: Theme.of(context).textTheme.labelLarge,
                      ),
                    ),
                    const SizedBox(height: 8),
                    TextField(
                      key: AppTestKeys.networksAllocationStartIpField,
                      controller: _allocationStartIpController,
                      decoration: const InputDecoration(labelText: 'Start IP'),
                    ),
                    const SizedBox(height: 12),
                    TextField(
                      key: AppTestKeys.networksAllocationEndIpField,
                      controller: _allocationEndIpController,
                      decoration: const InputDecoration(labelText: 'End IP'),
                    ),
                  ],
                ),
              ),
              actions: [
                SizedBox(
                  width: 120,
                  child: OutlinedButton(
                    onPressed: () => Navigator.of(context).pop(),
                    child: const Text('Cancel'),
                  ),
                ),
                SizedBox(
                  width: 160,
                  child: FilledButton(
                    key: AppTestKeys.networksCreateButton,
                    onPressed: () async {
                      await _createNetwork();
                      if (context.mounted) {
                        Navigator.of(context).pop();
                      }
                    },
                    child: const Text('Save'),
                  ),
                ),
              ],
            );
          },
        );
      },
    );
  }

  Future<void> _openJoinNetworkDialog() async {
    await showDialog<void>(
      context: context,
      builder: (context) {
        return AlertDialog(
          title: const Text('Access confirmation'),
          content: SingleChildScrollView(
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                TextField(
                  key: AppTestKeys.networksJoinKeyField,
                  controller: _joinKeyController,
                  decoration: const InputDecoration(
                    labelText: 'Invite code',
                    hintText: 'Paste invite code',
                  ),
                ),
              ],
            ),
          ),
          actions: [
            TextButton(
              onPressed: () => Navigator.of(context).pop(),
              child: const Text('Cancel'),
            ),
            FilledButton(
              key: AppTestKeys.networksJoinButton,
              onPressed: () async {
                await _joinNetwork();
                if (context.mounted) {
                  Navigator.of(context).pop();
                }
              },
              child: const Text('Confirm'),
            ),
          ],
        );
      },
    );
  }

  @override
  void dispose() {
    _ipAddressController.dispose();
    _subnetMaskController.dispose();
    _allocationStartIpController.dispose();
    _allocationEndIpController.dispose();
    _joinKeyController.dispose();
    super.dispose();
  }
}

class _NetworkDetails extends StatelessWidget {
  const _NetworkDetails({required this.network});

  final NetworkModel network;

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        _Line(label: 'Name', value: network.name),
        if (network.description != null && network.description!.isNotEmpty)
          _Line(label: 'Remark', value: network.description!),
        _Line(label: 'Network ID', value: network.networkId),
        _Line(label: 'CIDR', value: network.cidr),
        _Line(label: 'Members', value: '${network.members.length}'),
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
                title: _memberTitle(member),
                child: Text(_memberSummary(member)),
              ),
            ),
          ),
      ],
    );
  }

  String _memberTitle(NetworkMemberModel member) {
    final remark = member.remark?.trim();
    return remark == null || remark.isEmpty ? member.deviceId : remark;
  }

  String _memberSummary(NetworkMemberModel member) {
    final parts = [
      member.role,
      if (member.status != null && member.status!.isNotEmpty) member.status!,
      if (member.virtualIp != null && member.virtualIp!.isNotEmpty)
        member.virtualIp!,
    ];
    return parts.join(' / ');
  }
}

class _NetworkSwitchTile extends StatelessWidget {
  const _NetworkSwitchTile({
    required this.network,
    required this.selected,
    required this.busy,
    required this.onSwitch,
  });

  final NetworkModel network;
  final bool selected;
  final bool busy;
  final VoidCallback onSwitch;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return DesktopInsetBlock(
      title: network.name,
      child: Row(
        children: [
          Expanded(
            child: Text(
              '${network.networkId} / ${network.cidr}',
              style: theme.textTheme.bodyMedium?.copyWith(
                color: theme.colorScheme.onSurfaceVariant,
              ),
            ),
          ),
          const SizedBox(width: 12),
          selected
              ? const Chip(label: Text('Selected'))
              : OutlinedButton.icon(
                  key: AppTestKeys.networksSwitchButton(network.networkId),
                  onPressed: busy ? null : onSwitch,
                  icon: const Icon(Icons.swap_horiz_rounded),
                  label: const Text('Switch'),
                ),
        ],
      ),
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
