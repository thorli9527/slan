import Flutter
import Foundation
import UIKit

public class SlanAppCorePluginIosPlugin: NSObject, FlutterPlugin {
  private let lock = NSLock()
  private var bridgeClient: JsonLineBridgeClient?

  public static func register(with registrar: FlutterPluginRegistrar) {
    let channel = FlutterMethodChannel(
      name: "slan/app_core",
      binaryMessenger: registrar.messenger()
    )
    let instance = SlanAppCorePluginIosPlugin()
    registrar.addMethodCallDelegate(instance, channel: channel)
  }

  public func handle(_ call: FlutterMethodCall, result: @escaping FlutterResult) {
    switch call.method {
    case "platformDoctor":
      result(platformDoctor())
    case "platformInstallPlan":
      result(platformInstallPlan())
    default:
      do {
        result(try forwardToBridge(call))
      } catch let error as AppCoreBridgeError {
        result(FlutterError(code: error.code, message: error.message, details: nil))
      } catch {
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

  private func forwardToBridge(_ call: FlutterMethodCall) throws -> String {
    lock.lock()
    defer { lock.unlock() }
    let client: JsonLineBridgeClient
    if let bridgeClient {
      client = bridgeClient
    } else {
      let created = try JsonLineBridgeClient(endpoint: resolveBridgeEndpoint())
      bridgeClient = created
      client = created
    }
    return try client.invoke(
      method: call.method,
      arguments: call.arguments ?? [:]
    )
  }

  private func resolveBridgeEndpoint() throws -> BridgeEndpoint {
    let raw = (
      Bundle.main.object(forInfoDictionaryKey: "SLANAppCoreServiceHost") as? String
    )?.trimmingCharacters(in: .whitespacesAndNewlines)
    let value = (raw?.isEmpty == false ? raw! : "127.0.0.1:46391")
      .replacingOccurrences(of: "tcp://", with: "")
    guard let separator = value.lastIndex(of: ":") else {
      throw AppCoreBridgeError(
        code: "app_core_service_host_invalid",
        message: "Invalid iOS app-core service host. Expected host:port, got \(raw ?? value)."
      )
    }
    let host = String(value[..<separator])
    let portText = String(value[value.index(after: separator)...])
    guard !host.isEmpty, let port = UInt16(portText), port > 0 else {
      throw AppCoreBridgeError(
        code: "app_core_service_host_invalid",
        message: "Invalid iOS app-core service host. Expected host:port, got \(raw ?? value)."
      )
    }
    return BridgeEndpoint(
      host: host,
      port: port,
      source: raw?.isEmpty == false ? "Info.plist SLANAppCoreServiceHost" : "default"
    )
  }

  private func platformDoctor() -> [String: Any] {
    let endpoint = try? resolveBridgeEndpoint()
    let bridgeReachable = endpoint.map { JsonLineBridgeClient.canConnect(endpoint: $0) } ?? false
    let bridgeDetail: String
    if let endpoint {
      bridgeDetail = bridgeReachable
        ? "Can connect to app-core JSON bridge at \(endpoint.summary)"
        : "Cannot connect to app-core JSON bridge at \(endpoint.summary)"
    } else {
      bridgeDetail = "iOS app-core bridge endpoint is not configured"
    }
    return [
      "platform": iosPlatform(),
      "tunnelBackend": [
        "name": "ios-overlay",
        "executionMode": "system",
        "executionBackend": "network-extension",
        "interfaceName": NSNull(),
        "isUp": false,
        "plannedPeerCount": 0,
        "recentCommandCount": 0,
      ],
      "checks": [
        platformCheck(
          name: "ios_method_channel",
          status: "ok",
          detail: "iOS MethodChannel is registered"
        ),
        platformCheck(
          name: "overlay_packet_extension",
          status: "warn",
          detail: "iOS peer-network packet routing still needs a Network Extension implementation"
        ),
        platformCheck(
          name: "app_core_json_bridge",
          status: bridgeReachable ? "ok" : "warn",
          detail: bridgeDetail
        ),
      ],
    ]
  }

  private func platformInstallPlan() -> [String: Any] {
    return [
      "platform": iosPlatform(),
      "packages": [] as [String],
      "supportedDriverModes": ["network-extension", "json-bridge"],
      "warnings": [
        "Control-plane RPCs are routed through the same JSON-line app-core bridge used by desktop platforms",
        "Set Info.plist key SLANAppCoreServiceHost when an app or extension exposes a TCP endpoint",
        "Mobile mesh networking requires a Network Extension packet provider and app group storage",
      ],
    ]
  }

  private func iosPlatform() -> [String: Any] {
    return [
      "os": "ios",
      "distroId": NSNull(),
      "versionId": UIDevice.current.systemVersion,
      "idLike": [] as [String],
      "family": "darwin",
      "kernelRelease": NSNull(),
      "packageManager": NSNull(),
    ]
  }

  private func platformCheck(name: String, status: String, detail: String) -> [String: Any] {
    return [
      "name": name,
      "status": status,
      "detail": detail,
    ]
  }
}

private struct BridgeEndpoint {
  let host: String
  let port: UInt16
  let source: String

  var summary: String {
    "host=\(host):\(port), source=\(source)"
  }
}

private struct AppCoreBridgeError: Error {
  let code: String
  let message: String
}

private final class JsonLineBridgeClient {
  private let endpoint: BridgeEndpoint
  private var inputStream: InputStream?
  private var outputStream: OutputStream?

  init(endpoint: BridgeEndpoint) throws {
    self.endpoint = endpoint
    try connect()
  }

  deinit {
    close()
  }

  func invoke(method: String, arguments: Any) throws -> String {
    let payload = try JSONSerialization.data(
      withJSONObject: ["method": method, "args": jsonCompatible(arguments)],
      options: []
    )
    var request = payload
    request.append(0x0A)
    for attempt in 0..<2 {
      do {
        try ensureConnected()
        try writeAll(request)
        return try readLine()
      } catch let error as AppCoreBridgeError {
        close()
        if attempt == 1 {
          throw error
        }
      } catch {
        close()
        if attempt == 1 {
          throw AppCoreBridgeError(
            code: "app_core_service_io_failed",
            message: "iOS app-core bridge IO failed: \(error.localizedDescription). \(endpoint.summary)"
          )
        }
      }
    }
    throw AppCoreBridgeError(
      code: "app_core_service_io_failed",
      message: "iOS app-core bridge retry failed. \(endpoint.summary)"
    )
  }

  func close() {
    inputStream?.close()
    outputStream?.close()
    inputStream = nil
    outputStream = nil
  }

  private func ensureConnected() throws {
    if inputStream != nil && outputStream != nil {
      return
    }
    try connect()
  }

  private func connect() throws {
    var readStream: Unmanaged<CFReadStream>?
    var writeStream: Unmanaged<CFWriteStream>?
    CFStreamCreatePairWithSocketToHost(
      nil,
      endpoint.host as CFString,
      UInt32(endpoint.port),
      &readStream,
      &writeStream
    )
    guard let readStream, let writeStream else {
      throw AppCoreBridgeError(
        code: "app_core_service_connect_failed",
        message: "Failed to create iOS app-core bridge streams. \(endpoint.summary)"
      )
    }
    let input = readStream.takeRetainedValue() as InputStream
    let output = writeStream.takeRetainedValue() as OutputStream
    inputStream = input
    outputStream = output
    input.open()
    output.open()
    if input.streamStatus == .error || output.streamStatus == .error {
      close()
      throw AppCoreBridgeError(
        code: "app_core_service_connect_failed",
        message: "Failed to connect to iOS app-core bridge. \(endpoint.summary)"
      )
    }
  }

  private func writeAll(_ data: Data) throws {
    guard let outputStream else {
      throw AppCoreBridgeError(
        code: "app_core_service_closed",
        message: "iOS app-core bridge output stream is closed. \(endpoint.summary)"
      )
    }
    try data.withUnsafeBytes { rawBuffer in
      guard let base = rawBuffer.bindMemory(to: UInt8.self).baseAddress else {
        return
      }
      var offset = 0
      while offset < data.count {
        let written = outputStream.write(base.advanced(by: offset), maxLength: data.count - offset)
        if written <= 0 {
          throw AppCoreBridgeError(
            code: "app_core_service_write_failed",
            message: "Failed to write to iOS app-core bridge. \(endpoint.summary)"
          )
        }
        offset += written
      }
    }
  }

  private func readLine() throws -> String {
    guard let inputStream else {
      throw AppCoreBridgeError(
        code: "app_core_service_closed",
        message: "iOS app-core bridge input stream is closed. \(endpoint.summary)"
      )
    }
    var bytes: [UInt8] = []
    var byte = [UInt8](repeating: 0, count: 1)
    while true {
      let count = inputStream.read(&byte, maxLength: 1)
      if count <= 0 {
        throw AppCoreBridgeError(
          code: "app_core_service_closed",
          message: "iOS app-core bridge closed unexpectedly. \(endpoint.summary)"
        )
      }
      if byte[0] == 0x0A {
        return String(decoding: bytes, as: UTF8.self)
      }
      bytes.append(byte[0])
    }
  }

  static func canConnect(endpoint: BridgeEndpoint) -> Bool {
    do {
      let client = try JsonLineBridgeClient(endpoint: endpoint)
      client.close()
      return true
    } catch {
      return false
    }
  }
}

private func jsonCompatible(_ value: Any) -> Any {
  if value is NSNull ||
    value is String ||
    value is NSNumber ||
    value is Bool ||
    value is Int ||
    value is Double {
    return value
  }
  if let dictionary = value as? [String: Any] {
    return dictionary.mapValues { jsonCompatible($0) }
  }
  if let dictionary = value as? [AnyHashable: Any] {
    var mapped: [String: Any] = [:]
    for (key, item) in dictionary {
      mapped[String(describing: key)] = jsonCompatible(item)
    }
    return mapped
  }
  if let array = value as? [Any] {
    return array.map { jsonCompatible($0) }
  }
  return String(describing: value)
}
