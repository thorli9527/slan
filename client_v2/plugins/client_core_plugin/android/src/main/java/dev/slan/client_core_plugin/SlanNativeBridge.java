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

  private static native int startTun(int tunFd, int[] relayFds, String configJson);

  private static native void stopTun();

  private static native int isTunRunning();
}
