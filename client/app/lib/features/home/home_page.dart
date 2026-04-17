/// 首页容器页。
///
/// 通过 TabBar 将认证、网络和设备三个 Phase 1 子页面组合到一起。
import 'package:flutter/material.dart';

import '../auth/auth_page.dart';
import '../devices/devices_page.dart';
import '../networks/networks_page.dart';

class HomePage extends StatelessWidget {
  const HomePage({super.key});

  @override
  Widget build(BuildContext context) {
    return DefaultTabController(
      length: 3,
      child: Scaffold(
        appBar: AppBar(
          title: const Text('SLAN Phase 1'),
          bottom: const TabBar(
            tabs: [
              Tab(text: 'Auth'),
              Tab(text: 'Networks'),
              Tab(text: 'Devices'),
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
}
