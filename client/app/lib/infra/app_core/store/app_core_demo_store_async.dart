import 'package:flutter/foundation.dart';

import '../models/models.dart';
import 'app_core_demo_store_state.dart';

mixin AppCoreDemoStoreAsync on ChangeNotifier, AppCoreDemoStoreState {
  @protected
  Future<void> runAction(Future<void> Function() action) async {
    busy = true;
    error = null;
    notifyListeners();
    try {
      await action();
    } catch (err) {
      error = err.toString();
    } finally {
      busy = false;
      notifyListeners();
    }
  }

  @protected
  Future<T?> runTunnelAction<T>(Future<T> Function() action) async {
    busy = true;
    error = null;
    tunnelDebugError = null;
    notifyListeners();
    try {
      return await action();
    } catch (err) {
      final message = err.toString();
      tunnelDebugError = message;
      error = message;
      return null;
    } finally {
      busy = false;
      notifyListeners();
    }
  }

  @protected
  Future<void> runProbeAction(
      Future<DataPlaneProbeModel> Function() action) async {
    busy = true;
    error = null;
    notifyListeners();
    try {
      lastProbe = await action();
      lastProbeFailure = null;
    } on ProbeException catch (err) {
      lastProbe = null;
      lastProbeFailure = err.failure;
      error = err.failure.message;
    } catch (err) {
      lastProbe = null;
      lastProbeFailure = ProbeFailure(
        kind: ProbeFailureKind.unknown,
        code: null,
        message: err.toString(),
      );
      error = err.toString();
    } finally {
      busy = false;
      notifyListeners();
    }
  }

  @protected
  Future<void> runSendAction(Future<int> Function() action) async {
    busy = true;
    error = null;
    notifyListeners();
    try {
      lastSendBytes = await action();
      lastSendFailure = null;
    } on SendException catch (err) {
      lastSendBytes = null;
      lastSendFailure = err.failure;
      error = err.failure.message;
    } catch (err) {
      lastSendBytes = null;
      lastSendFailure = SendFailure(
        kind: SendFailureKind.unknown,
        code: null,
        message: err.toString(),
      );
      error = err.toString();
    } finally {
      busy = false;
      notifyListeners();
    }
  }
}
