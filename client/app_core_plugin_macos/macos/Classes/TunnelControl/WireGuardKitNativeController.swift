import Foundation

final class WireGuardKitNativeController: WireGuardKitNativeBackendSessionControlling {
  private let lifecycleController: WireGuardKitNativeBackendLifecycleControlling
  private var activeHandle: WireGuardKitNativeBackendHandleControlling?

  init(lifecycleController: WireGuardKitNativeBackendLifecycleControlling = WireGuardKitNativeLifecycleController()) {
    self.lifecycleController = lifecycleController
  }

  func start(configuration: WireGuardKitNativeBackendConfiguration) throws {
    activeHandle = nil
    activeHandle = try lifecycleController.start(configuration: configuration)
  }

  func stop() {
    activeHandle?.stop()
    activeHandle = nil
  }

  func handleInboundPackets(_ packets: [Data], protocols: [NSNumber]) throws -> PacketTunnelEngineOutput {
    guard let activeHandle else {
      return .empty
    }
    return try activeHandle.handleInboundPackets(packets, protocols: protocols)
  }

  func runtimeSnapshot() -> WireGuardBackendRuntimeSnapshot? {
    activeHandle?.runtimeSnapshot()
  }
}
