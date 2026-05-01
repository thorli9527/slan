import 'package:flutter/material.dart';

import 'app/slan_client_v2_app.dart';
import 'bridge/client_core_bridge.dart';

void main() {
  runApp(SlanClientV2App(bridge: MethodChannelClientCoreBridge()));
}
