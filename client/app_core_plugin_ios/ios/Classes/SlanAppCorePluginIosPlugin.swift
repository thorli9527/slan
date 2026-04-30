import Flutter
import UIKit

public class SlanAppCorePluginIosPlugin: NSObject, FlutterPlugin {
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
      result(
        FlutterError(
          code: "app_core_unsupported_platform",
          message: "iOS host integration is not implemented yet for \(call.method)",
          details: nil
        )
      )
    }
  }

  private func platformDoctor() -> [String: Any] {
    [
      "platform": iosPlatform(),
      "tunnelBackend": [
        "name": "ios-network-extension",
        "executionMode": "unimplemented",
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
          name: "network_extension",
          status: "fail",
          detail: "iOS Network Extension tunnel runtime still needs to be implemented"
        ),
        platformCheck(
          name: "app_core_bridge",
          status: "fail",
          detail: "iOS app-core control RPC bridge is not implemented yet"
        ),
      ],
    ]
  }

  private func platformInstallPlan() -> [String: Any] {
    [
      "platform": iosPlatform(),
      "packages": [] as [String],
      "supportedDriverModes": ["network-extension"],
      "warnings": [
        "iOS support requires a Network Extension target, entitlement provisioning, and a foreground-safe app-core control bridge",
        "Network enable/disable and heartbeat are unavailable until the iOS host integration is completed",
      ],
    ]
  }

  private func iosPlatform() -> [String: Any] {
    [
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
    [
      "name": name,
      "status": status,
      "detail": detail,
    ]
  }
}
