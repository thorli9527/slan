import Foundation

protocol WireGuardKitBackendSessionControlling {
  func start(sessionConfiguration: WireGuardKitSessionConfiguration) throws
  func stop()
  func handleInboundPackets(_ packets: [Data], protocols: [NSNumber]) throws -> PacketTunnelEngineOutput
  func runtimeSnapshot() -> WireGuardBackendRuntimeSnapshot?
}

protocol WireGuardKitBackendHandle {
  func stop()
  func handleInboundPackets(_ packets: [Data], protocols: [NSNumber]) throws -> PacketTunnelEngineOutput
  func runtimeSnapshot() -> WireGuardBackendRuntimeSnapshot?
}

protocol WireGuardKitBackendSessionLifecycleControlling {
  func start(sessionConfiguration: WireGuardKitSessionConfiguration) throws -> WireGuardKitBackendHandle
}

final class DefaultWireGuardKitBackendHandle: WireGuardKitBackendHandle {
  private let backendName: String
  private let sessionConfiguration: WireGuardKitSessionConfiguration
  private let startedAtMs: Int64
  private let driver: WireGuardKitBackendDriving
  private var backendState = "running"

  init(
    backendName: String = "wireguardkit",
    sessionConfiguration: WireGuardKitSessionConfiguration,
    startedAtMs: Int64,
    driver: WireGuardKitBackendDriving
  ) {
    self.backendName = backendName
    self.sessionConfiguration = sessionConfiguration
    self.startedAtMs = startedAtMs
    self.driver = driver
  }

  func stop() {
    driver.stop()
    backendState = "stopped"
  }

  func handleInboundPackets(_ packets: [Data], protocols: [NSNumber]) throws -> PacketTunnelEngineOutput {
    try driver.handleInboundPackets(packets, protocols: protocols)
  }

  func runtimeSnapshot() -> WireGuardBackendRuntimeSnapshot? {
    let driverSnapshot = driver.runtimeSnapshot()
    return WireGuardBackendRuntimeSnapshot(
      backendName: driverSnapshot?.backendName ?? backendName,
      backendState: driverSnapshot?.backendState ?? backendState,
      lastBackendError: driverSnapshot?.lastBackendError,
      lastStartedAtMs: driverSnapshot?.lastStartedAtMs ?? startedAtMs,
      peerVirtualIp: driverSnapshot?.peerVirtualIp ?? sessionConfiguration.peerAddress,
      selectedEndpoint: driverSnapshot?.selectedEndpoint ?? sessionConfiguration.endpoint
    )
  }
}

final class DefaultWireGuardKitBackendLifecycleController: WireGuardKitBackendSessionLifecycleControlling {
  private let backendName: String
  private let nowMs: () -> Int64
  private let driverFactory: () -> WireGuardKitBackendDriving

  init(
    backendName: String = "wireguardkit",
    nowMs: @escaping () -> Int64 = { Int64(Date().timeIntervalSince1970 * 1000) },
    driverFactory: @escaping () -> WireGuardKitBackendDriving = { WireGuardKitNativeBackendDriver() }
  ) {
    self.backendName = backendName
    self.nowMs = nowMs
    self.driverFactory = driverFactory
  }

  func start(sessionConfiguration: WireGuardKitSessionConfiguration) throws -> WireGuardKitBackendHandle {
    let driver = driverFactory()
    try driver.start(sessionConfiguration: sessionConfiguration)
    return DefaultWireGuardKitBackendHandle(
      backendName: backendName,
      sessionConfiguration: sessionConfiguration,
      startedAtMs: nowMs(),
      driver: driver
    )
  }
}

struct WireGuardKitBackendAvailability {
  let isAvailable: Bool
  let reason: String?
}

protocol WireGuardKitBackendAvailabilityChecking {
  var backendName: String { get }
  func availability() -> WireGuardKitBackendAvailability
}

struct DefaultWireGuardKitBackendAvailabilityChecker: WireGuardKitBackendAvailabilityChecking {
  let backendName = "wireguardkit"
  private let environment: [String: String]
  private let privateFrameworksPath: String?
  private let builtInPlugInsPath: String?
  private let fileExistsAtPath: (String) -> Bool

  init(
    environment: [String: String] = ProcessInfo.processInfo.environment,
    mainBundle: Bundle = .main,
    fileExistsAtPath: @escaping (String) -> Bool = { FileManager.default.fileExists(atPath: $0) }
  ) {
    self.environment = environment
    self.privateFrameworksPath = mainBundle.privateFrameworksPath
    self.builtInPlugInsPath = mainBundle.builtInPlugInsPath
    self.fileExistsAtPath = fileExistsAtPath
  }

  func availability() -> WireGuardKitBackendAvailability {
    if Self.isTruthy(environment["SLAN_WIREGUARDKIT_BACKEND_AVAILABLE"]) {
      return WireGuardKitBackendAvailability(
        isAvailable: true,
        reason: nil
      )
    }

    if Self.isTruthy(environment["SLAN_WIREGUARDKIT_BACKEND_UNAVAILABLE"]) {
      return WireGuardKitBackendAvailability(
        isAvailable: false,
        reason: "WireGuardKit backend disabled by environment override"
      )
    }

    for path in candidateFrameworkPaths() {
      if fileExistsAtPath(path) {
        return WireGuardKitBackendAvailability(
          isAvailable: true,
          reason: nil
        )
      }
    }

    return WireGuardKitBackendAvailability(
      isAvailable: false,
      reason: "WireGuardKit framework is not linked into the app bundle"
    )
  }

  private func candidateFrameworkPaths() -> [String] {
    var paths: [String] = []

    if let overridePath = environment["SLAN_WIREGUARDKIT_FRAMEWORK_PATH"],
       !overridePath.isEmpty {
      paths.append(overridePath)
    }

    if let privateFrameworksPath {
      paths.append((privateFrameworksPath as NSString).appendingPathComponent("WireGuardKit.framework"))
    }

    if let builtInPlugInsPath {
      paths.append((builtInPlugInsPath as NSString).appendingPathComponent("WireGuardKit.framework"))
    }

    return paths
  }

  private static func isTruthy(_ value: String?) -> Bool {
    guard let value else {
      return false
    }

    switch value.lowercased() {
    case "1", "true", "yes", "on":
      return true
    default:
      return false
    }
  }
}

final class WireGuardKitController: WireGuardKitBackendSessionControlling {
  private let lifecycleController: WireGuardKitBackendSessionLifecycleControlling
  private let availabilityChecker: WireGuardKitBackendAvailabilityChecking
  private let nowMs: () -> Int64
  private var activeHandle: WireGuardKitBackendHandle?
  private var lastSessionConfiguration: WireGuardKitSessionConfiguration?
  private var backendState = "idle"
  private var lastBackendError: String?
  private var lastStartedAtMs: Int64?

  init(
    lifecycleController: WireGuardKitBackendSessionLifecycleControlling = DefaultWireGuardKitBackendLifecycleController(),
    availabilityChecker: WireGuardKitBackendAvailabilityChecking = DefaultWireGuardKitBackendAvailabilityChecker(),
    nowMs: @escaping () -> Int64 = { Int64(Date().timeIntervalSince1970 * 1000) }
  ) {
    self.lifecycleController = lifecycleController
    self.availabilityChecker = availabilityChecker
    self.nowMs = nowMs
  }

  func start(sessionConfiguration: WireGuardKitSessionConfiguration) throws {
    lastSessionConfiguration = sessionConfiguration
    lastStartedAtMs = nowMs()
    let availability = availabilityChecker.availability()

    guard availability.isAvailable else {
      backendState = "unavailable"
      lastBackendError = availability.reason ?? "WireGuardKit backend is not integrated yet"
      throw PacketTunnelProviderError.invalidConfiguration(lastBackendError!)
    }

    do {
      activeHandle = try lifecycleController.start(sessionConfiguration: sessionConfiguration)
      backendState = "started"
      lastBackendError = nil
    } catch {
      activeHandle = nil
      backendState = "failed"
      lastBackendError = Self.errorMessage(from: error)
      throw error
    }
  }

  func stop() {
    activeHandle?.stop()
    activeHandle = nil
    backendState = "stopped"
  }

  func handleInboundPackets(_ packets: [Data], protocols: [NSNumber]) throws -> PacketTunnelEngineOutput {
    guard let activeHandle else {
      return .empty
    }
    return try activeHandle.handleInboundPackets(packets, protocols: protocols)
  }

  func runtimeSnapshot() -> WireGuardBackendRuntimeSnapshot? {
    let snapshot = activeHandle?.runtimeSnapshot()
    guard snapshot != nil || lastSessionConfiguration != nil || lastBackendError != nil else {
      return nil
    }
    return WireGuardBackendRuntimeSnapshot(
      backendName: snapshot?.backendName ?? availabilityChecker.backendName,
      backendState: snapshot?.backendState ?? backendState,
      lastBackendError: snapshot?.lastBackendError ?? lastBackendError,
      lastStartedAtMs: snapshot?.lastStartedAtMs ?? lastStartedAtMs,
      peerVirtualIp: snapshot?.peerVirtualIp ?? lastSessionConfiguration?.peerAddress,
      selectedEndpoint: snapshot?.selectedEndpoint ?? lastSessionConfiguration?.endpoint
    )
  }

  private static func errorMessage(from error: Error) -> String {
    if let providerError = error as? PacketTunnelProviderError {
      return providerError.message
    }
    return error.localizedDescription
  }
}
