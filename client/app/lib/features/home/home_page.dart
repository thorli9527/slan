/// 首页容器页。
///
/// 在移动端保留原有 tab 结构，在桌面端切到侧边栏 + 概览面板布局。
library slan_app.features.home;

import 'package:flutter/material.dart';

import '../../infra/app_core/scope/app_core_scope.dart';
import '../../infra/app_core/store/app_core_demo_store.dart';
import '../../testing/app_test_keys.dart';
import '../auth/auth_page.dart';
import '../devices/devices_page.dart';
import '../networks/networks_page.dart';
import '../shared/desktop_client_widgets.dart';

enum _HomeSection {
  auth('Auth', Icons.badge_outlined),
  networks('Networks', Icons.hub_outlined),
  devices('Devices', Icons.developer_board_outlined);

  const _HomeSection(this.label, this.icon);

  final String label;
  final IconData icon;
}

class HomePage extends StatefulWidget {
  const HomePage({super.key});

  @override
  State<HomePage> createState() => _HomePageState();
}

class _HomePageState extends State<HomePage> {
  _HomeSection _section = _HomeSection.auth;

  @override
  Widget build(BuildContext context) {
    return LayoutBuilder(
      builder: (context, constraints) {
        final isDesktop = constraints.maxWidth >= 1080;
        if (!isDesktop) {
          return _buildMobileShell();
        }
        return _buildDesktopShell();
      },
    );
  }

  Widget _buildMobileShell() {
    return DefaultTabController(
      length: _HomeSection.values.length,
      child: Scaffold(
        appBar: AppBar(
          title: const Text('SLAN Desktop Preview'),
          bottom: TabBar(
            onTap: (index) {
              setState(() {
                _section = _HomeSection.values[index];
              });
            },
            tabs: const [
              Tab(key: AppTestKeys.authTab, text: 'Auth'),
              Tab(key: AppTestKeys.networksTab, text: 'Networks'),
              Tab(key: AppTestKeys.devicesTab, text: 'Devices'),
            ],
          ),
        ),
        body: const TabBarView(
          children: [
            AuthPage(),
            NetworksPage(),
            DevicesPage(),
          ],
        ),
      ),
    );
  }

  Widget _buildDesktopShell() {
    final store = AppCoreScope.demo;
    return AnimatedBuilder(
      animation: store,
      builder: (context, _) {
        final theme = Theme.of(context);
        return Scaffold(
          body: DecoratedBox(
            decoration: BoxDecoration(
              gradient: LinearGradient(
                begin: Alignment.topLeft,
                end: Alignment.bottomRight,
                colors: [
                  theme.colorScheme.surface,
                  theme.colorScheme.surfaceContainerLowest,
                  const Color(0xFFE9F4EF),
                ],
              ),
            ),
            child: SafeArea(
              child: Padding(
                padding: const EdgeInsets.all(20),
                child: Row(
                  crossAxisAlignment: CrossAxisAlignment.stretch,
                  children: [
                    _DesktopSidebar(
                      section: _section,
                      onSelect: (section) => setState(() => _section = section),
                    ),
                    const SizedBox(width: 20),
                    Expanded(
                      flex: 3,
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.stretch,
                        children: [
                          _DesktopHero(section: _section, store: store),
                          const SizedBox(height: 20),
                          Expanded(
                            child: DesktopWorkspaceFrame(
                              backgroundColor: theme.colorScheme.surface
                                  .withValues(alpha: 0.94),
                              child: IndexedStack(
                                index: _section.index,
                                children: const [
                                  AuthPage(),
                                  NetworksPage(),
                                  DevicesPage(),
                                ],
                              ),
                            ),
                          ),
                        ],
                      ),
                    ),
                    const SizedBox(width: 20),
                    SizedBox(
                      width: 320,
                      child: _DesktopOverview(store: store, section: _section),
                    ),
                  ],
                ),
              ),
            ),
          ),
        );
      },
    );
  }
}

class _DesktopSidebar extends StatelessWidget {
  const _DesktopSidebar({
    required this.section,
    required this.onSelect,
  });

  final _HomeSection section;
  final ValueChanged<_HomeSection> onSelect;

  @override
  Widget build(BuildContext context) {
    return DesktopNavigationSidebar(
      title: 'SLAN',
      subtitle: 'Mac desktop control surface',
      footer: Container(
        padding: const EdgeInsets.all(16),
        decoration: BoxDecoration(
          color: Colors.white.withValues(alpha: 0.08),
          borderRadius: BorderRadius.circular(20),
          border: Border.all(color: Colors.white24),
        ),
        child: const Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(
              'Tunnel stack',
              style: TextStyle(
                color: Colors.white,
                fontWeight: FontWeight.w700,
              ),
            ),
            SizedBox(height: 8),
            Text(
              'PacketTunnel + WireGuard backend runtime lives behind the Devices workspace.',
              style: TextStyle(color: Colors.white70, height: 1.35),
            ),
          ],
        ),
      ),
      children: [
        for (final item in _HomeSection.values) ...[
          _SidebarButton(
            item: item,
            selected: item == section,
            onTap: () => onSelect(item),
          ),
          const SizedBox(height: 10),
        ],
      ],
    );
  }
}

class _SidebarButton extends StatelessWidget {
  const _SidebarButton({
    required this.item,
    required this.selected,
    required this.onTap,
  });

  final _HomeSection item;
  final bool selected;
  final VoidCallback onTap;

  Key _keyForSection() {
    switch (item) {
      case _HomeSection.auth:
        return AppTestKeys.authTab;
      case _HomeSection.networks:
        return AppTestKeys.networksTab;
      case _HomeSection.devices:
        return AppTestKeys.devicesTab;
    }
  }

  @override
  Widget build(BuildContext context) {
    return KeyedSubtree(
      key: _keyForSection(),
      child: DesktopNavigationItem(
        icon: item.icon,
        label: item.label,
        selected: selected,
        onTap: onTap,
      ),
    );
  }
}

class _DesktopHero extends StatelessWidget {
  const _DesktopHero({
    required this.section,
    required this.store,
  });

  final _HomeSection section;
  final AppCoreDemoStore store;

  @override
  Widget build(BuildContext context) {
    final statusLine = switch (section) {
      _HomeSection.auth => store.session == null
          ? 'No active session yet'
          : 'Signed in as ${store.session!.userId}',
      _HomeSection.networks => store.networks.isEmpty
          ? 'No overlay networks loaded'
          : '${store.networks.length} overlay networks tracked',
      _HomeSection.devices => store.connectionState.status == 'connected'
          ? 'Connected via ${store.connectionState.path?.name ?? 'unknown'}'
          : 'Tunnel and path diagnostics are idle',
    };

    return DesktopHeroPanel(
      title: section.label,
      description: statusLine,
      backgroundColor: const Color(0xFF173128),
      foregroundColor: Colors.white,
      footer: Wrap(
        spacing: 10,
        runSpacing: 10,
        children: [
          DesktopMetricPill(
            label: 'Networks',
            value: '${store.networks.length}',
            backgroundColor: Colors.white.withValues(alpha: 0.08),
            foregroundColor: Colors.white,
            borderColor: Colors.white24,
          ),
          DesktopMetricPill(
            label: 'Session',
            value: store.session == null ? 'offline' : 'ready',
            backgroundColor: Colors.white.withValues(alpha: 0.08),
            foregroundColor: Colors.white,
            borderColor: Colors.white24,
          ),
          DesktopMetricPill(
            label: 'Path',
            value: store.connectionState.path?.name ?? 'idle',
            backgroundColor: Colors.white.withValues(alpha: 0.08),
            foregroundColor: Colors.white,
            borderColor: Colors.white24,
          ),
          DesktopMetricPill(
            label: 'Tunnel',
            value: store.tunnelRuntimeView?.state ?? 'none',
            backgroundColor: Colors.white.withValues(alpha: 0.08),
            foregroundColor: Colors.white,
            borderColor: Colors.white24,
          ),
        ],
      ),
    );
  }
}

class _DesktopOverview extends StatelessWidget {
  const _DesktopOverview({
    required this.store,
    required this.section,
  });

  final AppCoreDemoStore store;
  final _HomeSection section;

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        DesktopOverviewPanel(
          title: 'Control Plane',
          child: DesktopKeyValueList(
            entries: [
              DesktopKeyValueEntry(
                  label: 'Busy', value: store.busy ? 'yes' : 'no'),
              DesktopKeyValueEntry(
                  label: 'User', value: store.session?.userId ?? '-'),
              DesktopKeyValueEntry(
                  label: 'Device', value: store.device?.deviceId ?? '-'),
              DesktopKeyValueEntry(
                  label: 'Node', value: store.node?.nodeId ?? '-'),
              DesktopKeyValueEntry(label: 'Error', value: store.error ?? '-'),
            ],
          ),
        ),
        const SizedBox(height: 16),
        DesktopOverviewPanel(
          title: 'Overlay',
          child: DesktopKeyValueList(
            entries: [
              DesktopKeyValueEntry(
                  label: 'Networks', value: '${store.networks.length}'),
              DesktopKeyValueEntry(
                label: 'Members',
                value:
                    '${store.networks.fold<int>(0, (sum, item) => sum + item.members.length)}',
              ),
              DesktopKeyValueEntry(
                label: 'Connection',
                value: store.connectionState.status,
              ),
              DesktopKeyValueEntry(
                label: 'Relay Ticket',
                value: store.relayTicket?.ticketId ?? '-',
              ),
              DesktopKeyValueEntry(
                label: 'Bootstrap',
                value: store.bootstrap == null ? 'not loaded' : 'loaded',
              ),
            ],
          ),
        ),
        const SizedBox(height: 16),
        DesktopOverviewPanel(
          title: 'Data Plane',
          backgroundColor: const Color(0xFF204A3A),
          foregroundColor: Colors.white,
          child: DesktopKeyValueList(
            entries: [
              DesktopKeyValueEntry(
                  label: 'Selected View', value: section.label),
              DesktopKeyValueEntry(
                  label: 'Probe', value: store.lastProbe?.probeId ?? '-'),
              DesktopKeyValueEntry(
                label: 'Send Failure',
                value: store.lastSendFailure?.label ?? '-',
              ),
              DesktopKeyValueEntry(
                label: 'Probe Failure',
                value: store.lastProbeFailure?.label ?? '-',
              ),
              DesktopKeyValueEntry(
                label: 'Tunnel',
                value: store.tunnelRuntimeView == null
                    ? 'none'
                    : '${store.tunnelRuntimeView!.state}/${store.tunnelRuntimeView!.transport}',
              ),
              DesktopKeyValueEntry(
                label: 'Backend',
                value: store.tunnelRuntimeView?.backendState ?? '-',
              ),
            ],
          ),
        ),
        const Spacer(),
        const DesktopSurfaceCard(
          title: 'Desktop Shell',
          subtitle:
              'Desktop mode keeps the existing mock control-plane flows, but wraps them in a Mac-focused shell so auth, networks, device registration, relay fallback, and PacketTunnel diagnostics live in one workspace.',
          child: SizedBox.shrink(),
        ),
      ],
    );
  }
}
