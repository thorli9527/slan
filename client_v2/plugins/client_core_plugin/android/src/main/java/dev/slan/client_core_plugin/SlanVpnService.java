package dev.slan.client_core_plugin;

import android.app.Notification;
import android.app.NotificationChannel;
import android.app.NotificationManager;
import android.app.PendingIntent;
import android.content.Context;
import android.content.Intent;
import android.content.pm.ApplicationInfo;
import android.net.VpnService;
import android.os.Build;
import android.os.ParcelFileDescriptor;
import android.system.Os;
import android.system.OsConstants;
import android.util.Log;
import java.io.Closeable;
import java.io.FileDescriptor;
import java.io.IOException;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.List;
import java.net.InetAddress;
import java.net.InetSocketAddress;
import java.net.Socket;
import org.json.JSONArray;
import org.json.JSONObject;

/** Android VpnService that owns the TUN fd and hands it to the Rust data plane. */
public final class SlanVpnService extends VpnService {
  private static final String TAG = "SlanVpnService";
  static final String ACTION_START = "dev.slan.client_core_plugin.START_VPN";
  static final String ACTION_STOP = "dev.slan.client_core_plugin.STOP_VPN";
  static final String EXTRA_CONFIG_JSON = "configJson";
  private static final String RESOLVER_CONFIG_KEY = "dns";

  private static final String CHANNEL_ID = "slan_vpn";
  private static final int NOTIFICATION_ID = 24018;
  private static SlanVpnService activeService;

  /** Current Android VPN interface descriptor before ownership is detached to Rust. */
  private ParcelFileDescriptor vpnInterface;
  /** Protected relay/direct sockets whose fds are detached into Rust. */
  private final List<Closeable> protectedRelaySockets = new ArrayList<>();

  /** Protect an externally created socket fd from VPN routing. */
  static boolean protectSocketFd(int socketFd) {
    SlanVpnService service = activeService;
    return service != null && service.protect(socketFd);
  }

  @Override
  public void onCreate() {
    super.onCreate();
    activeService = this;
    SlanVpnRuntime.configure(this);
  }

  @Override
  public int onStartCommand(Intent intent, int flags, int startId) {
    if (intent == null || intent.getAction() == null) {
      return START_STICKY;
    }
    if (ACTION_STOP.equals(intent.getAction())) {
      stopVpn("Android VPN stopped");
      stopSelf();
      return START_NOT_STICKY;
    }
    if (ACTION_START.equals(intent.getAction())) {
      startForeground(NOTIFICATION_ID, notification());
      String configJson = intent.getStringExtra(EXTRA_CONFIG_JSON);
      new Thread(() -> {
        try {
          startVpn(new JSONObject(configJson));
        } catch (Exception error) {
          Log.e(TAG, "Android VPN start failed", error);
          SlanVpnRuntime.markError(error.getMessage());
          stopVpn("Android VPN start failed: " + error.getMessage());
          stopSelf();
        }
      }, "slan-vpn-start").start();
      return START_STICKY;
    }
    return START_STICKY;
  }

  @Override
  public void onRevoke() {
    stopVpn("Android VPN permission revoked");
    SlanVpnRuntime.markRevoked();
    stopSelf();
    super.onRevoke();
  }

  @Override
  public void onDestroy() {
    stopVpn("Android VPN destroyed");
    if (activeService == this) {
      activeService = null;
    }
    super.onDestroy();
  }

  /** Build Android VPN interface, protect relay/direct sockets, and start Rust TUN runtime. */
  private void startVpn(JSONObject config) throws Exception {
    Log.i(TAG, "Android VPN start requested with fresh config");
    // Peer/relay refresh can re-enter ACTION_START while a previous Android VPN
    // runtime is still active. Always tear down the previous native/TUN/socket
    // state first so socket protection and fd ownership restart from a clean slate.
    stopVpn("Android VPN reconfiguring");
    String virtualIp = config.optString("virtualIp", "").trim();
    int prefixLen = config.optInt("prefixLen", 32);
    Cidr virtualAddress = Cidr.parse(virtualIp);
    if (virtualAddress != null) {
      virtualIp = virtualAddress.address;
      prefixLen = virtualAddress.prefixLen;
      config.put("virtualIp", virtualIp);
      config.put("prefixLen", prefixLen);
    }
    if (virtualIp.isEmpty()) {
      throw new IllegalArgumentException("Android VPN virtualIp is empty");
    }

    Builder builder = new Builder()
        .setSession(config.optString("sessionName", "SLAN"))
        .allowFamily(OsConstants.AF_INET)
        .allowFamily(OsConstants.AF_INET6)
        .addAddress(virtualIp, 32);
    builder.setUnderlyingNetworks(null);

    int mtu = config.optInt("mtu", 1280);
    if (mtu >= 576) {
      builder.setMtu(mtu);
    }
    addDnsServers(
        builder,
        config.optJSONObject(RESOLVER_CONFIG_KEY) != null
            ? config.optJSONObject(RESOLVER_CONFIG_KEY).optJSONArray("servers")
            : null);
    addRoutes(builder, config.optJSONArray("routes"));
    if (isDebugBuild()) {
      builder.addAllowedApplication(getPackageName());
    }
    if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.Q) {
      builder.setMetered(false);
    }

    int[] relayFds = detachProtectedRelaySockets(config);
    Log.i(
        TAG,
        "Starting Rust Android VPN tunFd(detach pending)"
            + " relayFds="
            + Arrays.toString(relayFds)
            + " relaySessionCount="
            + relaySessionCount(config)
            + " relayAddress="
            + relayAddress(config));
    ParcelFileDescriptor nextInterface = builder.establish();
    if (nextInterface == null) {
      closeDetachedFds(relayFds);
      closeRelaySockets();
      throw new IllegalStateException("Android VPN establish returned null");
    }
    closeInterface();
    int tunFd = nextInterface.detachFd();
    vpnInterface = null;
    int nativeStart = SlanNativeBridge.start(tunFd, relayFds, config.toString());
    if (nativeStart != 0) {
      try {
        ParcelFileDescriptor.adoptFd(tunFd).close();
      } catch (IOException ignored) {
      }
      if (nativeStart == -1) {
        for (int relayFd : relayFds) {
          try {
            ParcelFileDescriptor.adoptFd(relayFd).close();
          } catch (IOException ignored) {
          }
        }
      }
      closeRelaySockets();
      throw new IllegalStateException("Rust Android TUN runtime is unavailable");
    }
    SlanVpnRuntime.markStarted(
        virtualIp,
        mtu >= 576 ? mtu : null,
        relayAddress(config),
        relaySessionCount(config));
  }

  private String relayAddress(JSONObject config) {
    String relayAddress = config.optString("relayAddress", "").trim();
    if (!relayAddress.isEmpty()) {
      return relayAddress;
    }
    JSONObject relayDataPlane = config.optJSONObject("relayDataPlane");
    return relayDataPlane == null ? "" : relayDataPlane.optString("relayAddress", "").trim();
  }

  /** Create protected relay sockets and an optional direct UDP socket for Rust data plane use. */
  private int[] detachProtectedRelaySockets(JSONObject config) throws Exception {
    JSONObject relayDataPlane = config.optJSONObject("relayDataPlane");
    if (relayDataPlane == null || !relayDataPlane.optBoolean("enabled", false)) {
      return new int[0];
    }
    int sessionCount = relaySessionCount(config);
    int directCandidateCount = directPeerCandidateCount(config);
    boolean needsDirectSocket = directCandidateCount > 0 || hasPeerPaths(config);
    if (sessionCount <= 0 && !needsDirectSocket) {
      return new int[0];
    }
    closeRelaySockets();
    int[] fds = new int[sessionCount + (needsDirectSocket ? 1 : 0)];
    for (int index = 0; index < sessionCount; index += 1) {
      JSONObject session = relayDataPlane.optJSONArray("sessions").optJSONObject(index);
      String relayUrl = sessionRelayUrl(session, relayDataPlane);
      HostPort hostPort = HostPort.parse(relayUrl);
      if (hostPort == null) {
        throw new IllegalStateException("Android VPN relay address is invalid");
      }
      Log.i(
          TAG,
          "Preparing Android VPN relay socket index="
              + index
              + " sessionId="
              + (session == null ? "" : session.optString("sessionId", "").trim())
              + " peerNodeId="
              + (session == null ? "" : session.optString("peerNodeId", "").trim())
              + " url="
              + relayUrl
              + " host="
              + hostPort.host
              + " port="
              + hostPort.port);
      if (isDerpRelayUrl(relayUrl)) {
        Socket socket = new Socket();
        // Allocate the underlying fd before VpnService.protect(Socket). An unbound
        // java.net.Socket can report protect=false on Android because no fd exists yet.
        socket.bind(new InetSocketAddress(0));
        if (!protect(socket)) {
          socket.close();
          throw new IllegalStateException("Android VPN failed to protect DERP socket");
        }
        socket.connect(new InetSocketAddress(hostPort.host, hostPort.port), 3000);
        ParcelFileDescriptor descriptor = ParcelFileDescriptor.fromSocket(socket);
        ParcelFileDescriptor rustDescriptor = ParcelFileDescriptor.dup(descriptor.getFileDescriptor());
        fds[index] = rustDescriptor.detachFd();
        Log.i(
            TAG,
            "Prepared Android VPN DERP relay fd index="
                + index
                + " fd="
                + fds[index]
                + " local="
                + socket.getLocalSocketAddress()
                + " remote="
                + socket.getRemoteSocketAddress());
        protectedRelaySockets.add(descriptor);
        protectedRelaySockets.add(socket);
      } else {
        int relayFd = detachProtectedIpv4DatagramSocket(hostPort.host, hostPort.port);
        fds[index] = relayFd;
        Log.i(
            TAG,
            "Prepared Android VPN UDP relay fd index="
                + index
                + " fd="
                + relayFd
                + " remote="
                + hostPort.host
                + ":"
                + hostPort.port);
      }
    }
    if (needsDirectSocket) {
      int directFd = detachProtectedIpv4DatagramSocket(null, 0);
      fds[sessionCount] = directFd;
      Log.i(
          TAG,
          "Prepared Android VPN direct UDP fd index="
              + sessionCount
              + " fd="
              + directFd);
    }
    return fds;
  }

  private int detachProtectedIpv4DatagramSocket(String remoteHost, int remotePort)
      throws Exception {
    FileDescriptor rawFd =
        Os.socket(OsConstants.AF_INET, OsConstants.SOCK_DGRAM, OsConstants.IPPROTO_UDP);
    ParcelFileDescriptor descriptor = null;
    try {
      Os.bind(rawFd, InetAddress.getByName("0.0.0.0"), 0);
      if (remoteHost != null && !remoteHost.isEmpty()) {
        Os.connect(rawFd, InetAddress.getByName(remoteHost), remotePort);
      }
      descriptor = ParcelFileDescriptor.dup(rawFd);
    } finally {
      Os.close(rawFd);
    }
    int socketFd = descriptor.detachFd();
    if (!protect(socketFd)) {
      closeDetachedFds(new int[] {socketFd});
      throw new IllegalStateException("Android VPN failed to protect IPv4 UDP socket");
    }
    return socketFd;
  }

  private String sessionRelayUrl(JSONObject session, JSONObject relayDataPlane) {
    JSONObject ticket = session == null ? null : session.optJSONObject("ticket");
    String relayUrl = ticket == null ? "" : ticket.optString("relayUrl", "").trim();
    if (!relayUrl.isEmpty()) {
      return relayUrl;
    }
    return relayDataPlane.optString("relayAddress", "").trim();
  }

  private boolean isDerpRelayUrl(String relayUrl) {
    String lower = relayUrl == null ? "" : relayUrl.trim().toLowerCase();
    return lower.startsWith("derp://")
        || lower.startsWith("derp+tcp+tls://")
        || lower.startsWith("derp_tcp_tls_443://");
  }

  /** Return whether config contains peer path entries that need a direct UDP socket. */
  private boolean hasPeerPaths(JSONObject config) {
    JSONObject relayDataPlane = config.optJSONObject("relayDataPlane");
    if (relayDataPlane == null || !relayDataPlane.optBoolean("enabled", false)) {
      return false;
    }
    JSONArray peerPaths = relayDataPlane.optJSONArray("peerPaths");
    return peerPaths != null && peerPaths.length() > 0;
  }

  /** Count relay sessions requiring protected UDP sockets. */
  private int relaySessionCount(JSONObject config) {
    JSONObject relayDataPlane = config.optJSONObject("relayDataPlane");
    if (relayDataPlane == null || !relayDataPlane.optBoolean("enabled", false)) {
      return 0;
    }
    JSONArray sessions = relayDataPlane.optJSONArray("sessions");
    return sessions == null ? 0 : sessions.length();
  }

  /** Count peers that have LAN/IPv6/direct UDP candidates. */
  private int directPeerCandidateCount(JSONObject config) {
    JSONObject relayDataPlane = config.optJSONObject("relayDataPlane");
    if (relayDataPlane == null || !relayDataPlane.optBoolean("enabled", false)) {
      return 0;
    }
    JSONArray peerPaths = relayDataPlane.optJSONArray("peerPaths");
    if (peerPaths == null) {
      return 0;
    }
    int count = 0;
    for (int peerIndex = 0; peerIndex < peerPaths.length(); peerIndex += 1) {
      JSONObject peerPath = peerPaths.optJSONObject(peerIndex);
      JSONArray candidates = peerPath == null ? null : peerPath.optJSONArray("candidates");
      if (candidates == null) {
        continue;
      }
      for (int candidateIndex = 0; candidateIndex < candidates.length(); candidateIndex += 1) {
        JSONObject candidate = candidates.optJSONObject(candidateIndex);
        String kind = candidate == null ? "" : candidate.optString("kind", "").trim();
        String address = candidate == null ? "" : candidate.optString("address", "").trim();
        if (!address.isEmpty()
            && ("direct_udp".equals(kind) || "lan_udp".equals(kind) || "ipv6_udp".equals(kind))) {
          count += 1;
          break;
        }
      }
    }
    return count;
  }

  private void addDnsServers(Builder builder, JSONArray dnsServers) {
    if (dnsServers == null) {
      return;
    }
    for (int index = 0; index < dnsServers.length(); index += 1) {
      String dns = dnsServers.optString(index, "").trim();
      if (isUsableIpv4(dns)) {
        builder.addDnsServer(dns);
      }
    }
  }

  private void addRoutes(Builder builder, JSONArray routes) {
    if (routes == null) {
      return;
    }
    for (int index = 0; index < routes.length(); index += 1) {
      JSONObject route = routes.optJSONObject(index);
      if (route == null) {
        continue;
      }
      String destination = route.optString("destination", "").trim();
      Cidr cidr = Cidr.parse(destination);
      if (cidr != null) {
        builder.addRoute(cidr.address, cidr.prefixLen);
      }
    }
  }

  private boolean isUsableIpv4(String value) {
    return !value.isEmpty()
        && !"0.0.0.0".equals(value)
        && !value.startsWith("169.254.")
        && value.indexOf(':') < 0;
  }

  private boolean isDebugBuild() {
    return (getApplicationInfo().flags & ApplicationInfo.FLAG_DEBUGGABLE) != 0;
  }

  private void stopVpn(String message) {
    SlanNativeBridge.stop();
    closeRelaySockets();
    closeInterface();
    SlanVpnRuntime.markStopped(message);
    if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.N) {
      stopForeground(STOP_FOREGROUND_REMOVE);
    } else {
      stopForeground(true);
    }
  }

  private void closeInterface() {
    ParcelFileDescriptor current = vpnInterface;
    vpnInterface = null;
    if (current != null) {
      try {
        current.close();
      } catch (IOException ignored) {
      }
    }
  }

  private void closeRelaySockets() {
    for (Closeable socket : protectedRelaySockets) {
      try {
        socket.close();
      } catch (IOException ignored) {
      }
    }
    protectedRelaySockets.clear();
  }

  private void closeDetachedFds(int[] fds) {
    if (fds == null) {
      return;
    }
    for (int fd : fds) {
      if (fd < 0) {
        continue;
      }
      try {
        ParcelFileDescriptor.adoptFd(fd).close();
      } catch (IOException ignored) {
      }
    }
  }

  private Notification notification() {
    NotificationManager manager =
        (NotificationManager) getSystemService(Context.NOTIFICATION_SERVICE);
    if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O && manager != null) {
      NotificationChannel channel =
          new NotificationChannel(CHANNEL_ID, "SLAN Network", NotificationManager.IMPORTANCE_LOW);
      manager.createNotificationChannel(channel);
    }
    Intent launchIntent = getPackageManager().getLaunchIntentForPackage(getPackageName());
    PendingIntent pendingIntent = null;
    if (launchIntent != null) {
      int flags = PendingIntent.FLAG_UPDATE_CURRENT;
      if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.M) {
        flags |= PendingIntent.FLAG_IMMUTABLE;
      }
      pendingIntent = PendingIntent.getActivity(this, 0, launchIntent, flags);
    }
    Notification.Builder builder = Build.VERSION.SDK_INT >= Build.VERSION_CODES.O
        ? new Notification.Builder(this, CHANNEL_ID)
        : new Notification.Builder(this);
    builder
        .setSmallIcon(android.R.drawable.stat_sys_download_done)
        .setContentTitle("SLAN Client V2")
        .setContentText("SLAN network is running")
        .setOngoing(true);
    if (pendingIntent != null) {
      builder.setContentIntent(pendingIntent);
    }
    return builder.build();
  }

  private static final class Cidr {
    final String address;
    final int prefixLen;

    Cidr(String address, int prefixLen) {
      this.address = address;
      this.prefixLen = prefixLen;
    }

    static Cidr parse(String value) {
      if (value == null || value.isEmpty() || "mesh".equalsIgnoreCase(value)) {
        return null;
      }
      String[] parts = value.split("/", 2);
      if (parts.length != 2) {
        return null;
      }
      String address = parts[0].trim();
      int prefix;
      try {
        prefix = Integer.parseInt(parts[1].trim());
      } catch (NumberFormatException error) {
        return null;
      }
      if (address.indexOf(':') >= 0 || prefix < 0 || prefix > 32) {
        return null;
      }
      return new Cidr(address, prefix);
    }
  }

  private static final class HostPort {
    final String host;
    final int port;

    HostPort(String host, int port) {
      this.host = host;
      this.port = port;
    }

    static HostPort parse(String value) {
      value = value == null ? "" : value.trim();
      int schemeSeparator = value.indexOf("://");
      if (schemeSeparator >= 0) {
        value = value.substring(schemeSeparator + 3).trim();
      }
      int separator = value.lastIndexOf(':');
      if (separator <= 0 || separator == value.length() - 1) {
        return null;
      }
      String host = value.substring(0, separator).trim();
      int port;
      try {
        port = Integer.parseInt(value.substring(separator + 1).trim());
      } catch (NumberFormatException error) {
        return null;
      }
      if (host.isEmpty() || port <= 0 || port > 65535) {
        return null;
      }
      return new HostPort(host, port);
    }
  }
}
