import '../models/diagnostic_models.dart';

mixin AppCoreCoordinatorAsync {
  bool get busy;
  set busy(bool value);

  String? get error;
  set error(String? value);

  String? get tunnelDebugError;
  set tunnelDebugError(String? value);

  DataPlaneProbeModel? get lastProbe;
  set lastProbe(DataPlaneProbeModel? value);

  ProbeFailure? get lastProbeFailure;
  set lastProbeFailure(ProbeFailure? value);

  int? get lastSendBytes;
  set lastSendBytes(int? value);

  SendFailure? get lastSendFailure;
  set lastSendFailure(SendFailure? value);

  void emitStateChanged();

  Future<void> runAction(Future<void> Function() action) async {
    busy = true;
    error = null;
    emitStateChanged();
    try {
      await action();
    } on StateError catch (err) {
      error = err.message;
    } catch (err) {
      error = err.toString();
    } finally {
      busy = false;
      emitStateChanged();
    }
  }

  Future<T?> runTunnelAction<T>(Future<T> Function() action) async {
    busy = true;
    error = null;
    tunnelDebugError = null;
    emitStateChanged();
    try {
      return await action();
    } catch (err) {
      final message = err.toString();
      tunnelDebugError = message;
      error = message;
      return null;
    } finally {
      busy = false;
      emitStateChanged();
    }
  }

  Future<void> runProbeAction(
      Future<DataPlaneProbeModel> Function() action) async {
    busy = true;
    error = null;
    emitStateChanged();
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
      emitStateChanged();
    }
  }

  Future<void> runSendAction(Future<int> Function() action) async {
    busy = true;
    error = null;
    emitStateChanged();
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
      emitStateChanged();
    }
  }
}
