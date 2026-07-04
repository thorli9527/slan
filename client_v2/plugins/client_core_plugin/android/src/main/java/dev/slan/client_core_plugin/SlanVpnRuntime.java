package dev.slan.client_core_plugin;

import android.content.Context;
import android.content.SharedPreferences;
import java.util.ArrayDeque;
import java.util.HashMap;
import java.util.Map;

final class SlanVpnRuntime {
  private static final Object LOCK = new Object();
  private static final ArrayDeque<Map<String, Object>> EVENTS = new ArrayDeque<>();
  private static final String PREFS_NAME = "slan_client_v2";
  private static final String VPN_ENABLED_KEY = "dev.slan.client.v2.android.vpn.enabled";
  private static final String VPN_IP_KEY = "dev.slan.client.v2.android.vpn.virtualIp";
  private static final String VPN_MTU_KEY = "dev.slan.client.v2.android.vpn.mtu";
  private static final String VPN_RELAY_ADDRESS_KEY = "dev.slan.client.v2.android.vpn.relayAddress";
  private static final String VPN_RELAY_SESSION_COUNT_KEY =
      "dev.slan.client.v2.android.vpn.relaySessionCount";
  private static boolean adapterPresent = false;
  private static boolean networkEnabled = false;
  private static String virtualIp = null;
  private static Integer mtu = null;
  private static String relayAddress = null;
  private static Integer relaySessionCount = null;
  private static Context applicationContext = null;

  private SlanVpnRuntime() {}

  static void configure(Context context) {
    if (context == null) {
      return;
    }
    synchronized (LOCK) {
      applicationContext = context.getApplicationContext();
      restorePersistedStateLocked();
    }
  }

  static Map<String, Object> runtimeState() {
    synchronized (LOCK) {
      restorePersistedStateLocked();
      return stateLocked();
    }
  }

  static Map<String, Object> disabledState() {
    synchronized (LOCK) {
      adapterPresent = false;
      networkEnabled = false;
      virtualIp = null;
      mtu = null;
      relayAddress = null;
      relaySessionCount = null;
      persistStateLocked();
      return stateLocked();
    }
  }

  static void markStarting(String ip, Integer nextMtu, String nextRelayAddress, Integer nextRelaySessionCount) {
    synchronized (LOCK) {
      adapterPresent = true;
      networkEnabled = true;
      virtualIp = emptyToNull(ip);
      mtu = nextMtu;
      relayAddress = emptyToNull(nextRelayAddress);
      relaySessionCount = nextRelaySessionCount;
      persistStateLocked();
    }
  }

  static void markStarted(String ip, Integer nextMtu, String nextRelayAddress, Integer nextRelaySessionCount) {
    synchronized (LOCK) {
      adapterPresent = true;
      networkEnabled = true;
      virtualIp = ip;
      mtu = nextMtu;
      relayAddress = emptyToNull(nextRelayAddress);
      relaySessionCount = nextRelaySessionCount;
      persistStateLocked();
      pushEventLocked("vpnStarted", "Android VPN started", stateLocked());
    }
  }

  static void markStopped(String message) {
    synchronized (LOCK) {
      adapterPresent = false;
      networkEnabled = false;
      virtualIp = null;
      mtu = null;
      relayAddress = null;
      relaySessionCount = null;
      persistStateLocked();
      pushEventLocked("vpnStopped", message, stateLocked());
    }
  }

  static void markRevoked() {
    synchronized (LOCK) {
      adapterPresent = false;
      networkEnabled = false;
      virtualIp = null;
      mtu = null;
      relayAddress = null;
      relaySessionCount = null;
      persistStateLocked();
      pushEventLocked("vpnRevoked", "Android VPN permission was revoked", stateLocked());
    }
  }

  static void markError(String message) {
    synchronized (LOCK) {
      pushEventLocked("error", message, stateLocked());
    }
  }

  static void pushEvent(String type, String message, Map<String, Object> runtimeState) {
    synchronized (LOCK) {
      pushEventLocked(type, message, runtimeState);
    }
  }

  static Map<String, Object> nextEvent() {
    synchronized (LOCK) {
      return EVENTS.isEmpty() ? null : EVENTS.removeFirst();
    }
  }

  private static Map<String, Object> stateLocked() {
    Map<String, Object> state = new HashMap<>();
    state.putAll(nativeStats());
    boolean nativeRunning = Boolean.TRUE.equals(state.get("running"));
    String nativeVirtualIp = stringValue(state.get("virtualIp"));
    if (nativeRunning && !networkEnabled) {
      adapterPresent = true;
      networkEnabled = true;
    }
    if (nativeRunning && nativeVirtualIp != null) {
      virtualIp = nativeVirtualIp;
      persistStateLocked();
    }
    state.put("adapterPresent", adapterPresent);
    state.put("networkEnabled", networkEnabled);
    if (virtualIp != null && !virtualIp.isEmpty()) {
      state.put("virtualIp", virtualIp);
    }
    if (mtu != null) {
      state.put("mtu", mtu);
    }
    if (relayAddress != null) {
      state.put("relayAddress", relayAddress);
    }
    if (relaySessionCount != null) {
      putIfAbsent(state, "relaySessionCount", relaySessionCount);
      putIfAbsent(state, "requestedRelaySessionCount", relaySessionCount);
      putIfAbsent(state, "attachedRelaySessionCount", relaySessionCount);
    }
    return state;
  }

  private static void restorePersistedStateLocked() {
    SharedPreferences prefs = prefs();
    if (prefs == null || networkEnabled || adapterPresent) {
      return;
    }
    if (!prefs.getBoolean(VPN_ENABLED_KEY, false)) {
      return;
    }
    adapterPresent = true;
    networkEnabled = true;
    virtualIp = emptyToNull(prefs.getString(VPN_IP_KEY, null));
    int persistedMtu = prefs.getInt(VPN_MTU_KEY, -1);
    mtu = persistedMtu > 0 ? persistedMtu : null;
    relayAddress = emptyToNull(prefs.getString(VPN_RELAY_ADDRESS_KEY, null));
    int persistedRelaySessions = prefs.getInt(VPN_RELAY_SESSION_COUNT_KEY, -1);
    relaySessionCount = persistedRelaySessions >= 0 ? persistedRelaySessions : null;
  }

  private static void persistStateLocked() {
    SharedPreferences prefs = prefs();
    if (prefs == null) {
      return;
    }
    SharedPreferences.Editor editor = prefs.edit()
        .putBoolean(VPN_ENABLED_KEY, networkEnabled)
        .putString(VPN_IP_KEY, virtualIp == null ? "" : virtualIp)
        .putString(VPN_RELAY_ADDRESS_KEY, relayAddress == null ? "" : relayAddress);
    if (mtu == null) {
      editor.remove(VPN_MTU_KEY);
    } else {
      editor.putInt(VPN_MTU_KEY, mtu);
    }
    if (relaySessionCount == null) {
      editor.remove(VPN_RELAY_SESSION_COUNT_KEY);
    } else {
      editor.putInt(VPN_RELAY_SESSION_COUNT_KEY, relaySessionCount);
    }
    editor.apply();
  }

  private static SharedPreferences prefs() {
    Context context = applicationContext;
    if (context == null) {
      return null;
    }
    return context.getSharedPreferences(PREFS_NAME, Context.MODE_PRIVATE);
  }

  private static Map<String, Object> nativeStats() {
    try {
      return JsonCodec.toMap(SlanNativeBridge.statsJson());
    } catch (Exception ignored) {
      return new HashMap<>();
    }
  }

  private static String emptyToNull(String value) {
    if (value == null || value.trim().isEmpty()) {
      return null;
    }
    return value.trim();
  }

  private static String stringValue(Object value) {
    if (value == null) {
      return null;
    }
    return emptyToNull(String.valueOf(value));
  }

  private static void putIfAbsent(Map<String, Object> state, String key, Object value) {
    if (value == null || state.containsKey(key)) {
      return;
    }
    state.put(key, value);
  }

  private static void pushEventLocked(String type, String message, Map<String, Object> runtimeState) {
    Map<String, Object> event = new HashMap<>();
    event.put("eventType", type);
    if (message != null) {
      event.put("message", message);
    }
    if (runtimeState != null) {
      event.put("runtimeState", runtimeState);
    }
    EVENTS.addLast(event);
    while (EVENTS.size() > 32) {
      if (!EVENTS.isEmpty()) {
        EVENTS.removeFirst();
      }
    }
  }
}
