import Foundation

struct WireGuardKitNativeModule: Equatable {
  let backendName: String
  let frameworkPath: String?
  let moduleSource: String
}

protocol WireGuardKitNativeSessioning: AnyObject {
  func stop()
  func handleInboundPackets(_ packets: [Data], protocols: [NSNumber]) throws -> PacketTunnelEngineOutput
  func runtimeSnapshot() -> WireGuardBackendRuntimeSnapshot?
}

protocol WireGuardKitNativeModuleResolving {
  func resolveModule() throws -> WireGuardKitNativeModule
}

protocol WireGuardKitNativeAdapting {
  func makeSession(configuration: WireGuardKitNativeBackendConfiguration) throws -> WireGuardKitNativeSessioning
}

struct DefaultWireGuardKitNativeModuleResolver: WireGuardKitNativeModuleResolving {
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

  func resolveModule() throws -> WireGuardKitNativeModule {
    if Self.isTruthy(environment["SLAN_WIREGUARDKIT_BACKEND_UNAVAILABLE"]) {
      throw PacketTunnelProviderError.invalidConfiguration("WireGuardKit native backend disabled by environment override")
    }

    for path in candidateFrameworkPaths() {
      if fileExistsAtPath(path) {
        return WireGuardKitNativeModule(
          backendName: "wireguardkit-native",
          frameworkPath: path,
          moduleSource: "framework"
        )
      }
    }

    if Self.isTruthy(environment["SLAN_WIREGUARDKIT_BACKEND_AVAILABLE"]) {
      return WireGuardKitNativeModule(
        backendName: "wireguardkit-native",
        frameworkPath: nil,
        moduleSource: "environment"
      )
    }

    throw PacketTunnelProviderError.invalidConfiguration("WireGuardKit native backend session is not integrated yet")
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

final class UnavailableWireGuardKitNativeModuleResolver: WireGuardKitNativeModuleResolving {
  func resolveModule() throws -> WireGuardKitNativeModule {
    throw PacketTunnelProviderError.invalidConfiguration("WireGuardKit native backend session is not integrated yet")
  }
}

final class UnavailableWireGuardKitNativeAdapter: WireGuardKitNativeAdapting {
  func makeSession(configuration: WireGuardKitNativeBackendConfiguration) throws -> WireGuardKitNativeSessioning {
    throw PacketTunnelProviderError.invalidConfiguration("WireGuardKit native backend session is not integrated yet")
  }
}

final class WireGuardKitNativeAdapter: WireGuardKitNativeAdapting {
  private let moduleResolver: WireGuardKitNativeModuleResolving
  private let sessionFactory: WireGuardKitNativeSessionFactorying

  init(
    moduleResolver: WireGuardKitNativeModuleResolving = DefaultWireGuardKitNativeModuleResolver(),
    sessionFactory: WireGuardKitNativeSessionFactorying = WireGuardKitNativeSessionFactory()
  ) {
    self.moduleResolver = moduleResolver
    self.sessionFactory = sessionFactory
  }

  func makeSession(configuration: WireGuardKitNativeBackendConfiguration) throws -> WireGuardKitNativeSessioning {
    let module = try moduleResolver.resolveModule()
    return try sessionFactory.makeSession(module: module, configuration: configuration)
  }
}
