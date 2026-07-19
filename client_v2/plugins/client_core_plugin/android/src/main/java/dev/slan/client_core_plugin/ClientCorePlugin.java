package dev.slan.client_core_plugin;

import android.app.Activity;
import android.content.Context;
import android.content.Intent;
import android.content.SharedPreferences;
import android.content.pm.ApplicationInfo;
import android.net.VpnService;
import android.os.Build;
import android.os.Handler;
import android.os.Looper;
import android.system.Os;
import io.flutter.embedding.engine.plugins.FlutterPlugin;
import io.flutter.embedding.engine.plugins.activity.ActivityAware;
import io.flutter.embedding.engine.plugins.activity.ActivityPluginBinding;
import io.flutter.plugin.common.MethodCall;
import io.flutter.plugin.common.MethodChannel;
import io.flutter.plugin.common.PluginRegistry;
import java.util.HashMap;
import java.util.Map;
import java.util.UUID;
import java.util.concurrent.ArrayBlockingQueue;
import java.util.concurrent.RejectedExecutionException;
import java.util.concurrent.ThreadPoolExecutor;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.atomic.AtomicLong;
import org.json.JSONObject;

/** Flutter MethodChannel entrypoint for Android client-core integration. */
public final class ClientCorePlugin
    implements FlutterPlugin, MethodChannel.MethodCallHandler, ActivityAware,
        PluginRegistry.ActivityResultListener {
  private static final int VPN_PERMISSION_REQUEST = 24017;
  private static final String PREFS_NAME = "slan_client_v2";
  private static final String DEVICE_ID_KEY = "dev.slan.client.v2.android.deviceId";
  private static final String NODE_ID_KEY = "dev.slan.client.v2.android.nodeId";
  private static final String SERVER_BASE_URL_KEY = "dev.slan.client.v2.mobile.serverBaseUrl";

  /** Guards the lightweight state map returned to Flutter. */
  private final Object stateLock = new Object();
  private final Handler mainHandler = new Handler(Looper.getMainLooper());
  private final Map<String, Object> state = new HashMap<>();
  private final ThreadPoolExecutor embeddedServiceExecutor =
      new ThreadPoolExecutor(
          1,
          1,
          0L,
          TimeUnit.MILLISECONDS,
          new ArrayBlockingQueue<>(128),
          runnable -> {
            Thread thread = new Thread(runnable, "slan-embedded-service");
            thread.setDaemon(true);
            return thread;
          },
          new ThreadPoolExecutor.AbortPolicy());
  private final AtomicLong embeddedServiceRejectedTotal = new AtomicLong();
  private final ThreadPoolExecutor embeddedWatchExecutor =
      new ThreadPoolExecutor(
          1,
          1,
          0L,
          TimeUnit.MILLISECONDS,
          new ArrayBlockingQueue<>(2),
          runnable -> {
            Thread thread = new Thread(runnable, "slan-embedded-watch");
            thread.setDaemon(true);
            return thread;
          },
          new ThreadPoolExecutor.AbortPolicy());
  private final AtomicLong embeddedWatchRejectedTotal = new AtomicLong();

  private MethodChannel channel;
  private Context applicationContext;
  private Activity activity;

  public ClientCorePlugin() {
    resetState(false);
  }

  /** Attach plugin to Flutter engine and prepare stable Android device/node identity. */
  @Override
  public void onAttachedToEngine(FlutterPluginBinding binding) {
    applicationContext = binding.getApplicationContext();
    SlanVpnRuntime.configure(applicationContext);
    synchronized (stateLock) {
      state.put("deviceId", stableDeviceId());
      state.put("nodeId", stableNodeId());
    }
    channel = new MethodChannel(binding.getBinaryMessenger(), "dev.slan/client_core_v2");
    channel.setMethodCallHandler(this);
  }

  @Override
  public void onDetachedFromEngine(FlutterPluginBinding binding) {
    if (channel != null) {
      channel.setMethodCallHandler(null);
    }
    channel = null;
    applicationContext = null;
    embeddedServiceExecutor.shutdownNow();
    embeddedWatchExecutor.shutdownNow();
  }

  @Override
  public void onAttachedToActivity(ActivityPluginBinding binding) {
    activity = binding.getActivity();
    binding.addActivityResultListener(this);
  }

  @Override
  public void onDetachedFromActivityForConfigChanges() {
    activity = null;
  }

  @Override
  public void onReattachedToActivityForConfigChanges(ActivityPluginBinding binding) {
    onAttachedToActivity(binding);
  }

  @Override
  public void onDetachedFromActivity() {
    activity = null;
  }

  @Override
  public boolean onActivityResult(int requestCode, int resultCode, Intent data) {
    if (requestCode != VPN_PERMISSION_REQUEST) {
      return false;
    }
    if (resultCode == Activity.RESULT_OK) {
      SlanVpnRuntime.pushEvent("permissionGranted", "Android VPN permission granted", null);
    } else {
      SlanVpnRuntime.pushEvent("permissionRequired", "Android VPN permission was not granted", null);
    }
    return true;
  }

  /** Dispatch Flutter MethodChannel calls to Android VPN, embedded service, and settings APIs. */
  @Override
  public void onMethodCall(MethodCall call, MethodChannel.Result result) {
    try {
      switch (call.method) {
        case "androidVpnPermissionState":
          result.success(vpnPermissionGranted() ? "granted" : "needsUserConsent");
          return;
        case "androidRequestVpnPermission":
          requestVpnPermission(result);
          return;
        case "androidStartVpn":
          startVpn(call, result);
          return;
        case "androidStopVpn":
          stopVpn(result);
          return;
        case "androidProtectSocket":
          protectSocket(call, result);
          return;
        case "androidRuntimeState":
          result.success(runtimeStateWithEmbeddedServiceDiagnostics());
          return;
        case "androidWatchNetworkEvent":
          watchNetworkEvent(result);
          return;
        case "embeddedServiceRequest":
          executeEmbeddedServiceRequest(call.arguments, result);
          return;
        case "mobileServerBaseUrl":
          result.success(mobileServerBaseUrl());
          return;
        case "setMobileServerBaseUrl":
          setMobileServerBaseUrl(call.arguments == null ? "" : String.valueOf(call.arguments));
          result.success(true);
          return;
        case "setAndroidDebugEmulatorVpnBypass":
          setAndroidDebugEmulatorVpnBypass(call.arguments);
          result.success(true);
          return;
        case "setAndroidTestForceRelayOnly":
          setAndroidTestForceRelayOnly(call.arguments);
          result.success(true);
          return;
        default:
          result.notImplemented();
      }
    } catch (Exception error) {
      result.error("android_plugin_error", error.getMessage(), null);
    }
  }

  private void executeEmbeddedServiceRequest(Object arguments, MethodChannel.Result result)
      throws Exception {
    final String requestJson =
        embeddedServiceRequestJson(arguments == null ? "{}" : String.valueOf(arguments));
    final boolean watchRequest =
        "localBusinessEventWatch".equals(new JSONObject(requestJson).optString("method"));
    final ThreadPoolExecutor executor =
        watchRequest ? embeddedWatchExecutor : embeddedServiceExecutor;
    try {
      executor.execute(
          () -> {
            try {
              String response = SlanNativeBridge.serviceRequest(requestJson);
              mainHandler.post(() -> result.success(response));
            } catch (Exception error) {
              mainHandler.post(
                  () -> result.error("embedded_service_error", error.getMessage(), null));
            }
          });
    } catch (RejectedExecutionException error) {
      if (watchRequest) {
        embeddedWatchRejectedTotal.incrementAndGet();
        result.error("embedded_watch_busy", "embedded watch request queue is full", null);
      } else {
        embeddedServiceRejectedTotal.incrementAndGet();
        result.error("embedded_service_busy", "embedded service request queue is full", null);
      }
    }
  }

  private Map<String, Object> runtimeStateWithEmbeddedServiceDiagnostics() {
    Map<String, Object> runtime = new HashMap<>(SlanVpnRuntime.runtimeState());
    int queueDepth = embeddedServiceExecutor.getQueue().size();
    int activeCount = embeddedServiceExecutor.getActiveCount();
    runtime.put("embeddedServicePendingLimit", 129);
    runtime.put("embeddedServicePendingCount", queueDepth + activeCount);
    runtime.put("embeddedServiceQueueDepth", queueDepth);
    runtime.put("embeddedServiceActiveCount", activeCount);
    runtime.put("embeddedServiceCompletedTotal", embeddedServiceExecutor.getCompletedTaskCount());
    runtime.put("embeddedServiceRejectedTotal", embeddedServiceRejectedTotal.get());
    int watchQueueDepth = embeddedWatchExecutor.getQueue().size();
    int watchActiveCount = embeddedWatchExecutor.getActiveCount();
    runtime.put("embeddedWatchPendingLimit", 3);
    runtime.put("embeddedWatchPendingCount", watchQueueDepth + watchActiveCount);
    runtime.put("embeddedWatchQueueDepth", watchQueueDepth);
    runtime.put("embeddedWatchActiveCount", watchActiveCount);
    runtime.put("embeddedWatchCompletedTotal", embeddedWatchExecutor.getCompletedTaskCount());
    runtime.put("embeddedWatchRejectedTotal", embeddedWatchRejectedTotal.get());
    return runtime;
  }

  /** Reset cached UI state after startup or explicit sign-out. */
  private void resetState(boolean signedOut) {
    synchronized (stateLock) {
      state.clear();
      state.put("signedIn", false);
      state.put("deviceId", null);
      state.put("nodeId", null);
      state.put("networkEnabled", false);
      state.put("syncing", false);
      state.put("switchEnabled", true);
      state.put("notice", signedOut ? "signedOut" : "androidClientReady");
      state.put("error", null);
    }
  }

  /** Return whether Android VpnService already has user consent. */
  private boolean vpnPermissionGranted() {
    Context context = applicationContext;
    if (context == null) {
      return false;
    }
    if (allowDebugEmulatorVpnBypass()) {
      return true;
    }
    return VpnService.prepare(context) == null;
  }

  /** Start Android system consent flow for VpnService. */
  private void requestVpnPermission(MethodChannel.Result result) {
    String requestId = "vpn-" + UUID.randomUUID();
    if (allowDebugEmulatorVpnBypass()) {
      SlanVpnRuntime.pushEvent(
          "permissionGranted",
          "Android VPN permission bypassed for debug emulator",
          null);
      result.success(consentRequest(requestId, "Android VPN permission bypassed for debug emulator"));
      return;
    }
    Activity currentActivity = activity;
    if (currentActivity == null) {
      result.error("activity_unavailable", "Android Activity is not attached", null);
      return;
    }
    Intent intent = VpnService.prepare(currentActivity);
    if (intent == null) {
      SlanVpnRuntime.pushEvent("permissionGranted", "Android VPN permission already granted", null);
      result.success(consentRequest(requestId, "Android VPN permission already granted"));
      return;
    }
    currentActivity.startActivityForResult(intent, VPN_PERMISSION_REQUEST);
    SlanVpnRuntime.pushEvent("permissionRequired", "Android VPN permission requested", null);
    result.success(consentRequest(requestId, "Android VPN permission requested"));
  }

  private Map<String, Object> consentRequest(String requestId, String message) {
    Map<String, Object> response = new HashMap<>();
    response.put("requestId", requestId);
    response.put("message", message);
    return response;
  }

  /** Start foreground VpnService with the JSON config produced by Flutter/Rust. */
  private void startVpn(MethodCall call, MethodChannel.Result result) throws Exception {
    Context context = applicationContext;
    if (context == null) {
      result.error("context_unavailable", "Android context is not attached", null);
      return;
    }
    if (!vpnPermissionGranted()) {
      SlanVpnRuntime.pushEvent("permissionRequired", "Android VPN permission required", null);
      result.error("vpn_permission_required", "Android VPN permission required", null);
      return;
    }
    JSONObject config = JsonCodec.toJsonObject(call.arguments);
    SlanVpnRuntime.markStarting(
        config.optString("virtualIp", ""),
        config.has("mtu") ? config.optInt("mtu") : null,
        relayAddress(config),
        relaySessionCount(config));
    Intent intent = new Intent(context, SlanVpnService.class);
    intent.setAction(SlanVpnService.ACTION_START);
    intent.putExtra(SlanVpnService.EXTRA_CONFIG_JSON, config.toString());
    startVpnService(context, intent);
    synchronized (stateLock) {
      state.put("networkEnabled", true);
      state.put("virtualIp", config.optString("virtualIp", ""));
    }
    result.success(runtimeStateWithEmbeddedServiceDiagnostics());
  }

  private String relayAddress(JSONObject config) {
    String relayAddress = config.optString("relayAddress", "").trim();
    if (!relayAddress.isEmpty()) {
      return relayAddress;
    }
    JSONObject relayDataPlane = config.optJSONObject("relayDataPlane");
    return relayDataPlane == null ? "" : relayDataPlane.optString("relayAddress", "").trim();
  }

  private Integer relaySessionCount(JSONObject config) {
    JSONObject relayDataPlane = config.optJSONObject("relayDataPlane");
    if (relayDataPlane == null || !relayDataPlane.optBoolean("enabled", false)) {
      return 0;
    }
    return relayDataPlane.optJSONArray("sessions") == null
        ? 0
        : relayDataPlane.optJSONArray("sessions").length();
  }

  /** Stop foreground VpnService and clear cached network state. */
  private void stopVpn(MethodChannel.Result result) {
    Context context = applicationContext;
    if (context == null) {
      result.error("context_unavailable", "Android context is not attached", null);
      return;
    }
    Intent intent = new Intent(context, SlanVpnService.class);
    intent.setAction(SlanVpnService.ACTION_STOP);
    startVpnService(context, intent);
    synchronized (stateLock) {
      state.put("networkEnabled", false);
      state.remove("virtualIp");
    }
    result.success(SlanVpnRuntime.disabledState());
  }

  private void startVpnService(Context context, Intent intent) {
    if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
      context.startForegroundService(intent);
    } else {
      context.startService(intent);
    }
  }

  /** Protect a socket fd from being routed back into Android VPN. */
  private void protectSocket(MethodCall call, MethodChannel.Result result) {
    Object value = call.argument("socketFd");
    int socketFd = value instanceof Number ? ((Number) value).intValue() : -1;
    if (socketFd < 0) {
      result.error("invalid_socket_fd", "socketFd must be a non-negative integer", null);
      return;
    }
    if (!SlanVpnService.protectSocketFd(socketFd)) {
      result.error("protect_socket_failed", "Android VpnService.protect failed", null);
      return;
    }
    result.success(Boolean.TRUE);
  }

  private void watchNetworkEvent(MethodChannel.Result result) {
    Map<String, Object> event = SlanVpnRuntime.nextEvent();
    if (event != null) {
      result.success(event);
      return;
    }
    mainHandler.postDelayed(() -> result.success(SlanVpnRuntime.nextEvent()), 1000);
  }

  private String stableDeviceId() {
    Context context = applicationContext;
    if (context == null) {
      return UUID.randomUUID().toString().replace("-", "").toLowerCase();
    }
    SharedPreferences prefs = prefs();
    String existing = prefs.getString(DEVICE_ID_KEY, "");
    String normalized = normalizedUuidV4(existing);
    if (!normalized.isEmpty()) {
      prefs.edit().putString(DEVICE_ID_KEY, normalized).apply();
      return normalized;
    }
    String value = UUID.randomUUID().toString().replace("-", "").toLowerCase();
    prefs.edit().putString(DEVICE_ID_KEY, value).apply();
    return value;
  }

  private String embeddedServiceRequestJson(String requestJson) throws Exception {
    JSONObject request = new JSONObject(requestJson == null ? "{}" : requestJson);
    JSONObject args = request.optJSONObject("args");
    if (args == null) {
      args = new JSONObject();
      request.put("args", args);
    }
    String requestedDeviceId = args.optString("deviceIdOverride", "").trim();
    if (requestedDeviceId.isEmpty()) {
      requestedDeviceId = args.optString("deviceId", "").trim();
    }
    String effectiveDeviceId = requestedDeviceId.isEmpty() ? stableDeviceId() : normalizedUuidV4(requestedDeviceId);
    if (effectiveDeviceId.isEmpty()) {
      effectiveDeviceId = stableDeviceId();
    } else if (!requestedDeviceId.isEmpty()) {
      prefs().edit().putString(DEVICE_ID_KEY, effectiveDeviceId).apply();
    }
    args.put("deviceId", effectiveDeviceId);
    args.put("deviceIdOverride", effectiveDeviceId);
    Context context = applicationContext;
    if (context != null) {
      args.put("stateDir", context.getFilesDir().getAbsolutePath());
    }
    return request.toString();
  }

  private String normalizedUuidV4(String value) {
    if (value == null) {
      return "";
    }
    String normalized = value.trim().replace("-", "").toLowerCase();
    return normalized.matches("^[0-9a-f]{12}4[0-9a-f]{3}[89ab][0-9a-f]{15}$") ? normalized : "";
  }

  private String stableNodeId() {
    Context context = applicationContext;
    if (context == null) {
      return "node-android-" + UUID.randomUUID().toString().toLowerCase();
    }
    SharedPreferences prefs = prefs();
    String existing = prefs.getString(NODE_ID_KEY, "");
    if (existing != null && !existing.trim().isEmpty()) {
      return existing;
    }
    String value = "node-android-" + UUID.randomUUID().toString().toLowerCase();
    prefs.edit().putString(NODE_ID_KEY, value).apply();
    return value;
  }

  private String mobileServerBaseUrl() {
    Context context = applicationContext;
    if (context == null) {
      return "";
    }
    String existing = prefs().getString(SERVER_BASE_URL_KEY, "");
    return existing == null ? "" : existing.trim();
  }

  private void setMobileServerBaseUrl(String value) {
    Context context = applicationContext;
    if (context == null) {
      return;
    }
    prefs().edit().putString(SERVER_BASE_URL_KEY, value == null ? "" : value.trim()).apply();
  }

  private void setAndroidDebugEmulatorVpnBypass(Object value) {
    Context context = applicationContext;
    if (context == null) {
      return;
    }
    boolean enabled = value instanceof Boolean
        ? (Boolean) value
        : Boolean.parseBoolean(String.valueOf(value));
    prefs().edit()
        .putBoolean("dev.slan.client.v2.android.debug.emulatorVpnBypass", enabled)
        .apply();
  }

  private void setAndroidTestForceRelayOnly(Object value) throws Exception {
    if (!isDebugBuild()) {
      throw new SecurityException("relay-only test policy is only available in debug builds");
    }
    boolean enabled = value instanceof Boolean
        ? (Boolean) value
        : Boolean.parseBoolean(String.valueOf(value));
    Os.setenv("SLAN_FORCE_RELAY_ONLY", enabled ? "1" : "0", true);
  }

  private boolean allowDebugEmulatorVpnBypass() {
    if (!(isDebugBuild() && isProbablyEmulator())) {
      return false;
    }
    Context context = applicationContext;
    if (context == null) {
      return false;
    }
    return context.getSharedPreferences(PREFS_NAME, Context.MODE_PRIVATE)
        .getBoolean("dev.slan.client.v2.android.debug.emulatorVpnBypass", false);
  }

  private boolean isDebugBuild() {
    Context context = applicationContext;
    if (context == null) {
      return false;
    }
    return (context.getApplicationInfo().flags & ApplicationInfo.FLAG_DEBUGGABLE) != 0;
  }

  private boolean isProbablyEmulator() {
    String fingerprint = Build.FINGERPRINT == null ? "" : Build.FINGERPRINT;
    String model = Build.MODEL == null ? "" : Build.MODEL;
    String manufacturer = Build.MANUFACTURER == null ? "" : Build.MANUFACTURER;
    String hardware = Build.HARDWARE == null ? "" : Build.HARDWARE;
    String product = Build.PRODUCT == null ? "" : Build.PRODUCT;
    String device = Build.DEVICE == null ? "" : Build.DEVICE;
    return fingerprint.contains("generic")
        || fingerprint.contains("emulator")
        || model.contains("Emulator")
        || model.contains("Android SDK built for")
        || manufacturer.contains("Genymotion")
        || hardware.contains("ranchu")
        || hardware.contains("goldfish")
        || product.contains("sdk_gphone")
        || device.contains("emu");
  }

  private SharedPreferences prefs() {
    return applicationContext.getSharedPreferences(PREFS_NAME, Context.MODE_PRIVATE);
  }
}
