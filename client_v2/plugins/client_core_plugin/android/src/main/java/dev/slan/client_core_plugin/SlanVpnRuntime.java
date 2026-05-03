package dev.slan.client_core_plugin;

import java.util.ArrayDeque;
import java.util.HashMap;
import java.util.Map;

final class SlanVpnRuntime {
  private static final Object LOCK = new Object();
  private static final ArrayDeque<Map<String, Object>> EVENTS = new ArrayDeque<>();
  private static boolean adapterPresent = false;
  private static boolean networkEnabled = false;
  private static String virtualIp = null;
  private static Integer mtu = null;
  private static String relayAddress = null;
  private static Integer relaySessionCount = null;

  private SlanVpnRuntime() {}

  static Map<String, Object> runtimeState() {
    synchronized (LOCK) {
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
      return stateLocked();
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

  static Map<String, Object> pollEvent() {
    synchronized (LOCK) {
      return EVENTS.pollFirst();
    }
  }

  private static Map<String, Object> stateLocked() {
    Map<String, Object> state = new HashMap<>();
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
      state.put("relaySessionCount", relaySessionCount);
    }
    return state;
  }

  private static String emptyToNull(String value) {
    if (value == null || value.trim().isEmpty()) {
      return null;
    }
    return value.trim();
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
      EVENTS.pollFirst();
    }
  }
}
