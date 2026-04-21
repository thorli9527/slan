import '../../../application/device_runtime_service.dart';
import '../models/identity_models.dart';
import 'app_core_coordinator.dart';

class AppSessionController {
  const AppSessionController(this._coordinator);

  final AppCoreCoordinator _coordinator;

  Future<void> applyExternalSession(SessionModel nextSession) =>
      _coordinator.applyExternalSession(nextSession);

  Future<void> signOut() => _coordinator.signOut();

  Future<void> registerDevice({
    required String name,
    required String platform,
    required String machineId,
    required String publicKey,
  }) =>
      _coordinator.registerDevice(
        name: name,
        platform: platform,
        machineId: machineId,
        publicKey: publicKey,
      );

  Future<String?> ensureHomeWorkspaceReady({
    required String deviceName,
    required String platform,
    required String machineId,
    required String devicePublicKey,
  }) =>
      _coordinator.ensureHomeWorkspaceReady(
        deviceName: deviceName,
        platform: platform,
        machineId: machineId,
        devicePublicKey: devicePublicKey,
      );

  Future<void> registerNode({
    required String deviceId,
    required String nodeId,
    required String nodePublicKey,
    List<String> capabilities = const [],
    String? bootstrapNetworkId,
  }) =>
      _coordinator.registerNode(
        deviceId: deviceId,
        nodeId: nodeId,
        nodePublicKey: nodePublicKey,
        capabilities: capabilities,
        bootstrapNetworkId: bootstrapNetworkId,
      );

  Future<String> ensureNetworkAvailableAndJoined({
    required DeviceModel currentDevice,
    required String preferredNetworkId,
    required String fallbackNetworkName,
    String fallbackCidr = AppCoreCoordinator.defaultAutoNetworkCidr,
  }) =>
      _coordinator.ensureNetworkAvailableAndJoined(
        currentDevice: currentDevice,
        preferredNetworkId: preferredNetworkId,
        fallbackNetworkName: fallbackNetworkName,
        fallbackCidr: fallbackCidr,
      );

  Future<void> enableActiveNetwork() => _coordinator.enableActiveNetwork();

  Future<void> disableActiveNetwork() => _coordinator.disableActiveNetwork();

  Future<void> loadBootstrap({
    String? nodeId,
    String? networkId,
  }) =>
      _coordinator.loadBootstrap(
        nodeId: nodeId,
        networkId: networkId,
      );

  Future<String> refreshBootstrapOrControlSync() =>
      _coordinator.refreshBootstrapOrControlSync();

  Future<ConnectAttemptResult> connectUsingControlPlan({
    required String networkId,
    required String peerNodeId,
    required String reason,
  }) =>
      _coordinator.connectUsingControlPlan(
        networkId: networkId,
        peerNodeId: peerNodeId,
        reason: reason,
      );

  Future<void> disconnect() => _coordinator.disconnect();
}
