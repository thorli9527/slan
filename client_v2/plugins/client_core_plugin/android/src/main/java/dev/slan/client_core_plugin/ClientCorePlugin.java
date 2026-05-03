package dev.slan.client_core_plugin;

import android.app.Activity;
import android.content.Context;
import android.content.Intent;
import android.net.VpnService;
import android.os.Build;
import io.flutter.embedding.engine.plugins.FlutterPlugin;
import io.flutter.embedding.engine.plugins.activity.ActivityAware;
import io.flutter.embedding.engine.plugins.activity.ActivityPluginBinding;
import io.flutter.plugin.common.MethodCall;
import io.flutter.plugin.common.MethodChannel;
import io.flutter.plugin.common.PluginRegistry;
import java.util.HashMap;
import java.util.Map;
import java.util.UUID;
import org.json.JSONObject;

public final class ClientCorePlugin
    implements FlutterPlugin, MethodChannel.MethodCallHandler, ActivityAware,
        PluginRegistry.ActivityResultListener {
  private static final int VPN_PERMISSION_REQUEST = 24017;

  private MethodChannel channel;
  private Context applicationContext;
  private Activity activity;

  @Override
  public void onAttachedToEngine(FlutterPluginBinding binding) {
    applicationContext = binding.getApplicationContext();
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
          result.success(SlanVpnRuntime.runtimeState());
          return;
        case "androidPollNetworkEvent":
          pollNetworkEvent(result);
          return;
        default:
          result.notImplemented();
      }
    } catch (Exception error) {
      result.error("android_vpn_error", error.getMessage(), null);
    }
  }

  private boolean vpnPermissionGranted() {
    Context context = applicationContext;
    return context != null && VpnService.prepare(context) == null;
  }

  private void requestVpnPermission(MethodChannel.Result result) {
    String callbackId = "vpn-" + UUID.randomUUID();
    Activity currentActivity = activity;
    if (currentActivity == null) {
      result.error("activity_unavailable", "Android Activity is not attached", null);
      return;
    }
    Intent intent = VpnService.prepare(currentActivity);
    if (intent == null) {
      SlanVpnRuntime.pushEvent("permissionGranted", "Android VPN permission already granted", null);
      result.success(consentRequest(callbackId, "Android VPN permission already granted"));
      return;
    }
    currentActivity.startActivityForResult(intent, VPN_PERMISSION_REQUEST);
    SlanVpnRuntime.pushEvent("permissionRequired", "Android VPN permission requested", null);
    result.success(consentRequest(callbackId, "Android VPN permission requested"));
  }

  private Map<String, Object> consentRequest(String callbackId, String message) {
    Map<String, Object> response = new HashMap<>();
    response.put("callbackId", callbackId);
    response.put("message", message);
    return response;
  }

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
    Intent intent = new Intent(context, SlanVpnService.class);
    intent.setAction(SlanVpnService.ACTION_START);
    intent.putExtra(SlanVpnService.EXTRA_CONFIG_JSON, config.toString());
    startVpnService(context, intent);
    result.success(SlanVpnRuntime.runtimeState());
  }

  private void stopVpn(MethodChannel.Result result) {
    Context context = applicationContext;
    if (context == null) {
      result.error("context_unavailable", "Android context is not attached", null);
      return;
    }
    Intent intent = new Intent(context, SlanVpnService.class);
    intent.setAction(SlanVpnService.ACTION_STOP);
    startVpnService(context, intent);
    result.success(SlanVpnRuntime.disabledState());
  }

  private void startVpnService(Context context, Intent intent) {
    if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
      context.startForegroundService(intent);
    } else {
      context.startService(intent);
    }
  }

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

  private void pollNetworkEvent(MethodChannel.Result result) {
    Map<String, Object> event = SlanVpnRuntime.pollEvent();
    if (event != null) {
      result.success(event);
      return;
    }
    android.os.Handler handler = new android.os.Handler(android.os.Looper.getMainLooper());
    handler.postDelayed(() -> result.success(SlanVpnRuntime.pollEvent()), 1000);
  }
}
