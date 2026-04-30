package dev.slan.app_core_plugin_android

import android.content.Context
import android.content.pm.PackageManager
import android.os.Build
import io.flutter.embedding.engine.plugins.FlutterPlugin
import io.flutter.plugin.common.MethodCall
import io.flutter.plugin.common.MethodChannel
import io.flutter.plugin.common.MethodChannel.MethodCallHandler
import io.flutter.plugin.common.MethodChannel.Result
import java.io.BufferedReader
import java.io.BufferedWriter
import java.io.InputStreamReader
import java.io.OutputStreamWriter
import java.net.InetSocketAddress
import java.net.Socket
import org.json.JSONArray
import org.json.JSONObject

class SlanAppCorePluginAndroidPlugin : FlutterPlugin, MethodCallHandler {
  private lateinit var channel: MethodChannel
  private lateinit var context: Context
  private var bridgeClient: JsonLineBridgeClient? = null

  override fun onAttachedToEngine(binding: FlutterPlugin.FlutterPluginBinding) {
    context = binding.applicationContext
    channel = MethodChannel(binding.binaryMessenger, "slan/app_core")
    channel.setMethodCallHandler(this)
  }

  override fun onMethodCall(call: MethodCall, result: Result) {
    try {
      when (call.method) {
        "platformDoctor" -> result.success(platformDoctor())
        "platformInstallPlan" -> result.success(platformInstallPlan())
        else -> result.success(forwardToBridge(call))
      }
    } catch (error: AppCoreBridgeException) {
      result.error(error.code, error.message, null)
    } catch (error: Exception) {
      result.error("app_core_bridge_error", error.message ?: "Android app-core bridge failed", null)
    }
  }

  override fun onDetachedFromEngine(binding: FlutterPlugin.FlutterPluginBinding) {
    bridgeClient?.close()
    bridgeClient = null
    channel.setMethodCallHandler(null)
  }

  private fun forwardToBridge(call: MethodCall): String {
    val client = bridgeClient ?: JsonLineBridgeClient(resolveBridgeEndpoint()).also {
      bridgeClient = it
    }
    return client.invoke(call.method, call.arguments)
  }

  private fun resolveBridgeEndpoint(): BridgeEndpoint {
    val metadata = appMetadata()
    val rawHost = metadata?.getString("dev.slan.app_core.SERVICE_HOST")
      ?: "127.0.0.1:46391"
    val value = rawHost.removePrefix("tcp://").trim()
    val separator = value.lastIndexOf(':')
    if (separator <= 0 || separator == value.length - 1) {
      throw AppCoreBridgeException(
        "app_core_service_host_invalid",
        "Invalid Android app-core service host. Expected host:port, got $rawHost",
      )
    }
    val port = value.substring(separator + 1).toIntOrNull()
      ?: throw AppCoreBridgeException(
        "app_core_service_host_invalid",
        "Invalid Android app-core service port in $rawHost",
      )
    return BridgeEndpoint(
      host = value.substring(0, separator),
      port = port,
      source = if (metadata?.containsKey("dev.slan.app_core.SERVICE_HOST") == true) {
        "AndroidManifest meta-data"
      } else {
        "default"
      },
    )
  }

  private fun appMetadata(): android.os.Bundle? {
    return try {
      val info = if (Build.VERSION.SDK_INT >= 33) {
        context.packageManager.getApplicationInfo(
          context.packageName,
          PackageManager.ApplicationInfoFlags.of(PackageManager.GET_META_DATA.toLong()),
        )
      } else {
        @Suppress("DEPRECATION")
        context.packageManager.getApplicationInfo(
          context.packageName,
          PackageManager.GET_META_DATA,
        )
      }
      info.metaData
    } catch (_: Exception) {
      null
    }
  }

  private fun platformDoctor(): Map<String, Any?> {
    val endpoint = runCatching { resolveBridgeEndpoint() }.getOrNull()
    val bridgeReachable = endpoint?.let { JsonLineBridgeClient.canConnect(it) } ?: false
    return mapOf(
      "platform" to androidPlatform(),
      "tunnelBackend" to mapOf(
        "name" to "android-vpnservice",
        "executionMode" to "system",
        "executionBackend" to "vpnservice",
        "interfaceName" to null,
        "isUp" to false,
        "plannedPeerCount" to 0,
        "recentCommandCount" to 0,
      ),
      "checks" to listOf(
        platformCheck(
          "android_method_channel",
          "ok",
          "Android MethodChannel is registered",
        ),
        platformCheck(
          "app_core_json_bridge",
          if (bridgeReachable) "ok" else "warn",
          if (endpoint == null) {
            "Android app-core bridge endpoint is not configured"
          } else if (bridgeReachable) {
            "Can connect to app-core JSON bridge at ${endpoint.summary()}"
          } else {
            "Cannot connect to app-core JSON bridge at ${endpoint.summary()}"
          },
        ),
        platformCheck(
          "vpn_service",
          "warn",
          "Android VpnService tunnel runtime still needs a native service implementation",
        ),
      ),
    )
  }

  private fun platformInstallPlan(): Map<String, Any?> = mapOf(
    "platform" to androidPlatform(),
    "packages" to emptyList<String>(),
    "supportedDriverModes" to listOf("vpnservice", "json-bridge"),
    "warnings" to listOf(
      "Control-plane RPCs are routed through the same JSON-line app-core bridge used by desktop platforms",
      "Set Android manifest meta-data dev.slan.app_core.SERVICE_HOST when a foreground app-core service exposes a TCP endpoint",
      "Network tunneling still requires implementing Android VpnService and a foreground service",
    ),
  )

  private fun androidPlatform(): Map<String, Any?> = mapOf(
    "os" to "android",
    "distroId" to null,
    "versionId" to Build.VERSION.RELEASE,
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

private data class BridgeEndpoint(
  val host: String,
  val port: Int,
  val source: String,
) {
  fun summary(): String = "host=$host:$port, source=$source"
}

private class AppCoreBridgeException(
  val code: String,
  override val message: String,
) : Exception(message)

private class JsonLineBridgeClient(private val endpoint: BridgeEndpoint) {
  private var socket: Socket? = null
  private var reader: BufferedReader? = null
  private var writer: BufferedWriter? = null

  fun invoke(method: String, arguments: Any?): String {
    val request = JSONObject()
      .put("method", method)
      .put("args", toJsonValue(arguments ?: emptyMap<String, Any?>()))
      .toString() + "\n"
    repeat(2) { attempt ->
      try {
        ensureConnected()
        writer!!.write(request)
        writer!!.flush()
        return reader!!.readLine()
          ?: throw AppCoreBridgeException(
            "app_core_service_closed",
            "Android app-core bridge closed unexpectedly. ${endpoint.summary()}",
          )
      } catch (error: AppCoreBridgeException) {
        close()
        if (attempt == 1) {
          throw error
        }
      } catch (error: Exception) {
        close()
        if (attempt == 1) {
          throw AppCoreBridgeException(
            "app_core_service_io_failed",
            "Android app-core bridge IO failed: ${error.message}. ${endpoint.summary()}",
          )
        }
      }
    }
    throw AppCoreBridgeException(
      "app_core_service_io_failed",
      "Android app-core bridge retry failed. ${endpoint.summary()}",
    )
  }

  fun close() {
    runCatching { reader?.close() }
    runCatching { writer?.close() }
    runCatching { socket?.close() }
    reader = null
    writer = null
    socket = null
  }

  private fun ensureConnected() {
    val current = socket
    if (current != null && current.isConnected && !current.isClosed) {
      return
    }
    val created = Socket()
    try {
      created.connect(InetSocketAddress(endpoint.host, endpoint.port), 2_500)
      socket = created
      reader = BufferedReader(InputStreamReader(created.getInputStream(), Charsets.UTF_8))
      writer = BufferedWriter(OutputStreamWriter(created.getOutputStream(), Charsets.UTF_8))
    } catch (error: Exception) {
      runCatching { created.close() }
      throw AppCoreBridgeException(
        "app_core_service_connect_failed",
        "Android app-core bridge cannot connect: ${error.message}. ${endpoint.summary()}",
      )
    }
  }

  companion object {
    fun canConnect(endpoint: BridgeEndpoint): Boolean {
      val socket = Socket()
      return try {
        socket.connect(InetSocketAddress(endpoint.host, endpoint.port), 700)
        true
      } catch (_: Exception) {
        false
      } finally {
        runCatching { socket.close() }
      }
    }
  }
}

private fun toJsonValue(value: Any?): Any {
  return when (value) {
    null -> JSONObject.NULL
    is JSONObject, is JSONArray -> value
    is Boolean, is Number, is String -> value
    is Map<*, *> -> {
      val json = JSONObject()
      value.forEach { (key, item) ->
        if (key != null) {
          json.put(key.toString(), toJsonValue(item))
        }
      }
      json
    }
    is Iterable<*> -> {
      val json = JSONArray()
      value.forEach { json.put(toJsonValue(it)) }
      json
    }
    is Array<*> -> {
      val json = JSONArray()
      value.forEach { json.put(toJsonValue(it)) }
      json
    }
    else -> value.toString()
  }
}
