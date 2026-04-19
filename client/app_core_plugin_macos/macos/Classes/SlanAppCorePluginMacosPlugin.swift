import Cocoa
import FlutterMacOS
import NetworkExtension

public class SlanAppCorePluginMacosPlugin: NSObject, FlutterPlugin {
  private let queue = DispatchQueue(label: "slan.app_core.plugin")
  private var helper: HelperBridgeClient?
  private var tunnelBackend: WireGuardTunnelBackendInvoking?

  public static func register(with registrar: FlutterPluginRegistrar) {
    let channel = FlutterMethodChannel(name: "slan/app_core", binaryMessenger: registrar.messenger)
    let instance = SlanAppCorePluginMacosPlugin()
    registrar.addMethodCallDelegate(instance, channel: channel)
  }

  public func handle(_ call: FlutterMethodCall, result: @escaping FlutterResult) {
    queue.async {
      do {
        let args = call.arguments as? [String: Any] ?? [:]
        let response: Any
        if let backendMethod = WireGuardTunnelBackendMethod(rawValue: call.method) {
          response = try self.handleTunnelBackend(method: backendMethod, args: args)
        } else {
          response = try self.helperProcess().invoke(method: call.method, args: args)
        }
        DispatchQueue.main.async {
          result(response)
        }
      } catch let error as SlanAppCorePluginError {
        DispatchQueue.main.async {
          result(
            FlutterError(
              code: error.code,
              message: error.message,
              details: nil
            )
          )
        }
      } catch {
        DispatchQueue.main.async {
          result(
            FlutterError(
              code: "app_core_bridge_error",
              message: error.localizedDescription,
              details: nil
            )
          )
        }
      }
    }
  }

  private func helperProcess() throws -> HelperBridgeInvoking {
    if let helper {
      return helper
    }
    let created = try HelperBridgeClient(transport: RustHelperProcessTransport())
    helper = created
    return created
  }

  private func tunnelProcess() -> WireGuardTunnelBackendInvoking {
    if let tunnelBackend {
      return tunnelBackend
    }
    let created = PacketTunnelBackendAdapter()
    tunnelBackend = created
    return created
  }

  private func handleTunnelBackend(
    method: WireGuardTunnelBackendMethod,
    args: [String: Any]
  ) throws -> Any {
    let backend = tunnelProcess()
    switch method {
    case .applyConfiguration:
      let configuration = try WireGuardTunnelConfigMapper.mapConfiguration(args)
      try backend.apply(configuration: configuration)
      let snapshot = try backend.actionSnapshot(preferredPeerVirtualIp: configuration.peerVirtualIp)
      let phase = snapshot.runtimeLastError?.isEmpty == false
        ? "failed"
        : snapshot.hasConfiguration ? "configured" : "pending_verification"
      return snapshot.toJson(
        action: method.rawValue,
        accepted: true,
        phase: phase,
        detail: "Native backend accepted tunnel configuration for \(configuration.peerVirtualIp)."
      )
    case .removePeer:
      let peerVirtualIp = try requireStringArg("peerVirtualIp", in: args)
      try backend.removePeer(peerVirtualIp: peerVirtualIp)
      let snapshot = try backend.actionSnapshot(preferredPeerVirtualIp: peerVirtualIp)
      let phase = snapshot.hasConfiguration ? "accepted" : "verified"
      return snapshot.toJson(
        action: method.rawValue,
        accepted: true,
        phase: phase,
        detail: "Native backend processed peer removal for \(peerVirtualIp)."
      )
    case .bringUp:
      try backend.bringUp()
      let snapshot = try backend.actionSnapshot(preferredPeerVirtualIp: nil)
      let runtimeState = snapshot.runtimeState?.lowercased() ?? ""
      let backendState = snapshot.backendState?.lowercased() ?? ""
      let phase: String
      if snapshot.runtimeLastError?.isEmpty == false {
        phase = "failed"
      } else if (runtimeState == "up" || runtimeState == "running") &&
        (backendState == "started" || backendState == "running") {
        phase = "started"
      } else {
        phase = "accepted"
      }
      return snapshot.toJson(
        action: method.rawValue,
        accepted: true,
        phase: phase,
        detail: "Native backend accepted the bring-up request."
      )
    case .bringDown:
      try backend.bringDown()
      let snapshot = try backend.actionSnapshot(preferredPeerVirtualIp: nil)
      let phase =
        (snapshot.connectionStatus == "disconnected" || snapshot.connectionStatus == "invalid")
        ? "verified" : "accepted"
      return snapshot.toJson(
        action: method.rawValue,
        accepted: true,
        phase: phase,
        detail: "Native backend accepted the bring-down request."
      )
    case .runtimeView:
      let peerVirtualIp = try requireStringArg("peerVirtualIp", in: args)
      guard let runtime = try backend.runtimeView(peerVirtualIp: peerVirtualIp) else {
        return NSNull()
      }
      return WireGuardTunnelRuntimeViewMapper.toJson(runtime)
    }
  }

  private func requireStringArg(_ key: String, in args: [String: Any]) throws -> String {
    guard let value = args[key] as? String, !value.isEmpty else {
      throw SlanAppCorePluginError(
        code: "app_core_invalid_tunnel_config",
        message: "Missing or invalid string field \(key)"
      )
    }
    return value
  }
}

private protocol HelperBridgeInvoking {
  func invoke(method: String, args: [String: Any]) throws -> Any
}

private final class HelperBridgeClient: HelperBridgeInvoking {
  private let transport: HelperProcessTransporting

  init(transport: HelperProcessTransporting) {
    self.transport = transport
  }

  func invoke(method: String, args: [String: Any]) throws -> Any {
    let requestLine = try HelperRpcCodec.encodeRequest(method: method, args: args)
    let responseData = try transport.roundTrip(requestLine)
    return try HelperRpcCodec.decodeResponse(responseData)
  }
}

private enum HelperRpcCodec {
  static func encodeRequest(method: String, args: [String: Any]) throws -> Data {
    let payload = try JSONSerialization.data(
      withJSONObject: ["method": method, "args": args],
      options: []
    )
    var line = payload
    line.append(0x0A)
    return line
  }

  static func decodeResponse(_ responseData: Data) throws -> Any {
    let object = try JSONSerialization.jsonObject(with: responseData, options: [])
    guard let json = object as? [String: Any] else {
      throw SlanAppCorePluginError(
        code: "app_core_invalid_response",
        message: "Helper returned a non-object response"
      )
    }
    guard let ok = json["ok"] as? Bool else {
      throw SlanAppCorePluginError(
        code: "app_core_invalid_response",
        message: "Helper response missing ok flag"
      )
    }
    if ok {
      return json["result"] ?? [:]
    }
    let message =
      (json["errorMessage"] as? String)
      ?? (json["error"] as? String)
      ?? "unknown app-core helper error"
    let code = (json["errorCode"] as? String) ?? "app_core_helper_error"
    throw SlanAppCorePluginError(code: code, message: message)
  }
}

private protocol HelperProcessTransporting {
  func roundTrip(_ requestLine: Data) throws -> Data
}

private final class RustHelperProcessTransport: HelperProcessTransporting {
  private let process: Process
  private let input: FileHandle
  private let output: FileHandle

  init() throws {
    let executable = try Self.resolveExecutablePath()
    let stdinPipe = Pipe()
    let stdoutPipe = Pipe()
    let process = Process()
    process.executableURL = URL(fileURLWithPath: executable)
    process.standardInput = stdinPipe
    process.standardOutput = stdoutPipe
    process.standardError = FileHandle.standardError
    process.environment = Self.processEnvironment()

    do {
      try process.run()
    } catch {
      throw SlanAppCorePluginError(
        code: "app_core_process_start_failed",
        message: "Failed to launch app-core helper: \(error.localizedDescription)"
      )
    }

    self.process = process
    self.input = stdinPipe.fileHandleForWriting
    self.output = stdoutPipe.fileHandleForReading
  }

  deinit {
    try? input.close()
    try? output.close()
    if process.isRunning {
      process.terminate()
    }
  }

  func roundTrip(_ requestLine: Data) throws -> Data {
    input.write(requestLine)
    return try readLine()
  }

  private func readLine() throws -> Data {
    var buffer = Data()
    while true {
      let chunk = output.readData(ofLength: 1)
      if chunk.isEmpty {
        throw SlanAppCorePluginError(
          code: "app_core_process_closed",
          message: "app-core helper closed stdout unexpectedly"
        )
      }
      if chunk[0] == 0x0A {
        return buffer
      }
      buffer.append(chunk)
    }
  }

  private static func resolveExecutablePath() throws -> String {
    let env = ProcessInfo.processInfo.environment
    if let explicit = env["SLAN_APP_CORE_HELPER"], FileManager.default.isExecutableFile(atPath: explicit) {
      return explicit
    }
    if let bundled = Bundle.main.path(forResource: "app-core-helper", ofType: nil),
       FileManager.default.isExecutableFile(atPath: bundled) {
      return bundled
    }
    throw SlanAppCorePluginError(
      code: "app_core_helper_missing",
      message: "Set SLAN_APP_CORE_HELPER to the built Rust helper executable path"
    )
  }

  private static func processEnvironment() -> [String: String] {
    ProcessInfo.processInfo.environment
  }
}
