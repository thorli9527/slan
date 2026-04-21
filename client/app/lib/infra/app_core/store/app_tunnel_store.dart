import 'package:flutter/foundation.dart';

import 'app_core_tunnel_store_state.dart';

class AppTunnelStore extends ChangeNotifier with AppCoreTunnelStoreState {
  void emit() => notifyListeners();

  void resetAll() => resetTunnelState();

  void clearConnection() => clearTunnelConnectionArtifacts();
}
