package dev.slan.client_core_plugin;

final class SlanNativeBridge {
  private static boolean loadAttempted = false;
  private static boolean loaded = false;

  private SlanNativeBridge() {}

  static synchronized boolean isAvailable() {
    if (!loadAttempted) {
      loadAttempted = true;
      try {
        System.loadLibrary("client_core_ffi");
        loaded = true;
      } catch (UnsatisfiedLinkError error) {
        loaded = false;
      }
    }
    return loaded;
  }

  static int start(int tunFd, int[] relayFds, String configJson) {
    if (!isAvailable()) {
      return -1;
    }
    return startTun(tunFd, relayFds, configJson);
  }

  static void stop() {
    if (isAvailable()) {
      stopTun();
    }
  }

  static boolean running() {
    return isAvailable() && isTunRunning() == 1;
  }

  static String statsJson() {
    if (!isAvailable()) {
      return "{}";
    }
    return tunStatsJson();
  }

  static String serviceRequest(String requestJson) {
    if (!isAvailable()) {
      return "{\"error\":\"client_core_ffi is not available\"}";
    }
    return serviceRequestJson(requestJson == null ? "{}" : requestJson);
  }

  static String initialize(String stateDir) {
    if (!isAvailable()) {
      return "{\"error\":\"client_core_ffi is not available\"}";
    }
    return initializeRuntime(stateDir == null ? "" : stateDir);
  }

  private static native int startTun(int tunFd, int[] relayFds, String configJson);

  private static native void stopTun();

  private static native int isTunRunning();

  private static native String tunStatsJson();

  private static native String serviceRequestJson(String requestJson);

  private static native String initializeRuntime(String stateDir);
}
