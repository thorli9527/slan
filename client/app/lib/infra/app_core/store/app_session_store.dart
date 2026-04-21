import 'package:flutter/foundation.dart';

import '../models/identity_models.dart';
import 'app_core_session_store_state.dart';

class AppSessionStore extends ChangeNotifier with AppCoreSessionStoreState {
  void emit() => notifyListeners();

  void resetAll() => resetSessionState();

  void syncDevice(DeviceModel? nextDevice) => syncSessionDevice(nextDevice);

  void clearConnection() => clearSessionConnectionState();
}
