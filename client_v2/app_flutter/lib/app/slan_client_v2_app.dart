import 'package:flutter/material.dart';

import '../bridge/client_core_bridge.dart';
import '../ui/home/home_page.dart';

class SlanClientV2App extends StatelessWidget {
  const SlanClientV2App({required this.bridge, super.key});

  final ClientCoreBridge bridge;

  @override
  Widget build(BuildContext context) {
    return MaterialApp(
      debugShowCheckedModeBanner: false,
      title: 'SLAN Client',
      theme: ThemeData(
        colorScheme: ColorScheme.fromSeed(seedColor: const Color(0xff9b4f2d)),
        useMaterial3: true,
      ),
      home: HomePage(bridge: bridge),
    );
  }
}
