package dev.slan.app_core_plugin_android

import io.flutter.embedding.engine.plugins.FlutterPlugin
import io.flutter.plugin.common.MethodCall
import io.flutter.plugin.common.MethodChannel
import io.flutter.plugin.common.MethodChannel.MethodCallHandler
import io.flutter.plugin.common.MethodChannel.Result

class SlanAppCorePluginAndroidPlugin : FlutterPlugin, MethodCallHandler {
  private lateinit var channel: MethodChannel

  override fun onAttachedToEngine(binding: FlutterPlugin.FlutterPluginBinding) {
    channel = MethodChannel(binding.binaryMessenger, "slan/app_core")
    channel.setMethodCallHandler(this)
  }

  override fun onMethodCall(call: MethodCall, result: Result) {
    if (call.method == "platformDoctor") {
      result.success(platformDoctor())
      return
    }
    if (call.method == "platformInstallPlan") {
      result.success(platformInstallPlan())
      return
    }
    result.error(
      "app_core_unsupported_platform",
      "Android host integration is not implemented yet for ${call.method}",
      null,
    )
  }

  override fun onDetachedFromEngine(binding: FlutterPlugin.FlutterPluginBinding) {
    channel.setMethodCallHandler(null)
  }

  private fun platformDoctor(): Map<String, Any?> = mapOf(
    "platform" to androidPlatform(),
    "tunnelBackend" to mapOf(
      "name" to "android-vpnservice",
      "executionMode" to "unimplemented",
      "executionBackend" to "vpnservice",
      "interfaceName" to null,
      "isUp" to false,
      "plannedPeerCount" to 0,
      "recentCommandCount" to 0,
    ),
    "checks" to listOf(
      platformCheck(
        "android_plugin",
        "fail",
        "Android MethodChannel is present, but the host integration is not implemented yet",
      ),
      platformCheck(
        "vpn_service",
        "fail",
        "Android VpnService tunnel runtime still needs to be implemented",
      ),
      platformCheck(
        "foreground_service",
        "fail",
        "Foreground service and network-state heartbeat are not implemented yet",
      ),
    ),
  )

  private fun platformInstallPlan(): Map<String, Any?> = mapOf(
    "platform" to androidPlatform(),
    "packages" to emptyList<String>(),
    "supportedDriverModes" to listOf("vpnservice"),
    "warnings" to listOf(
      "Android support requires implementing VpnService, a foreground service, and MethodChannel routing for app-core control RPCs",
      "Network enable/disable and state heartbeat are intentionally unavailable until the Android host integration is completed",
    ),
  )

  private fun androidPlatform(): Map<String, Any?> = mapOf(
    "os" to "android",
    "distroId" to null,
    "versionId" to android.os.Build.VERSION.RELEASE,
    "idLike" to emptyList<String>(),
    "family" to "android",
    "kernelRelease" to null,
    "packageManager" to "apk",
  )

  private fun platformCheck(
    name: String,
    status: String,
    detail: String,
  ): Map<String, Any?> = mapOf(
    "name" to name,
    "status" to status,
    "detail" to detail,
  )
}
