import Foundation
import NetworkExtension

struct PacketTunnelProviderConfiguration {
  let localVirtualIp: String
  let peerVirtualIp: String
  let remoteAddress: String
  let dnsServers: [String]
  let mtu: Int?
  let selectedEndpoint: String?
  let debugEngineMode: PacketTunnelDebugEngineMode
  let interfaceAddress: PacketTunnelIPv4CIDR
  let allowedIps: [String]
  let allowedIPv4Routes: [NEIPv4Route]

  init(providerConfiguration: [String: Any]) throws {
    localVirtualIp = try Self.requireString("localVirtualIp", in: providerConfiguration)
    peerVirtualIp = try Self.requireString("peerVirtualIp", in: providerConfiguration)

    let interface = try Self.requireObject("wireguardInterface", in: providerConfiguration)
    let peer = try Self.requireObject("wireguardPeer", in: providerConfiguration)

    let addresses = interface["addresses"] as? [String] ?? []
    let addressCIDR = addresses.first ?? "\(localVirtualIp)/32"
    interfaceAddress = try PacketTunnelIPv4CIDR(addressCIDR)

    dnsServers = interface["dnsServers"] as? [String] ?? []
    mtu = interface["mtu"] as? Int

    let endpoint = peer["endpoint"] as? String
    selectedEndpoint = endpoint
    remoteAddress = Self.remoteHost(from: endpoint) ?? peerVirtualIp
    debugEngineMode = try Self.optionalDebugEngineMode(in: providerConfiguration)

    let allowedIps = peer["allowedIps"] as? [[String: Any]] ?? []
    self.allowedIps = try allowedIps.compactMap { item in
      guard let cidr = item["cidr"] as? String else {
        return nil
      }
      _ = try PacketTunnelIPv4CIDR(cidr)
      return cidr
    }
    allowedIPv4Routes = try allowedIps.compactMap { item in
      guard let cidr = item["cidr"] as? String else {
        return nil
      }
      let route = try PacketTunnelIPv4CIDR(cidr)
      return NEIPv4Route(destinationAddress: route.address, subnetMask: route.subnetMask)
    }
  }

  private static func requireString(_ key: String, in json: [String: Any]) throws -> String {
    guard let value = json[key] as? String, !value.isEmpty else {
      throw PacketTunnelProviderError.invalidConfiguration("missing or invalid string field \(key)")
    }
    return value
  }

  private static func requireObject(_ key: String, in json: [String: Any]) throws -> [String: Any] {
    guard let value = json[key] as? [String: Any] else {
      throw PacketTunnelProviderError.invalidConfiguration("missing or invalid object field \(key)")
    }
    return value
  }

  private static func optionalDebugEngineMode(in json: [String: Any]) throws -> PacketTunnelDebugEngineMode {
    guard let value = json["debugEngineMode"] as? String, !value.isEmpty else {
      return .noop
    }
    guard let mode = PacketTunnelDebugEngineMode(rawValue: value) else {
      throw PacketTunnelProviderError.invalidConfiguration("invalid debug engine mode: \(value)")
    }
    return mode
  }

  private static func remoteHost(from endpoint: String?) -> String? {
    guard let endpoint, !endpoint.isEmpty else {
      return nil
    }
    if let url = URL(string: "udp://\(endpoint)"), let host = url.host, !host.isEmpty {
      return host
    }
    return endpoint.split(separator: ":").first.map(String.init)
  }
}

struct PacketTunnelIPv4CIDR {
  let address: String
  let subnetMask: String

  init(_ cidr: String) throws {
    let parts = cidr.split(separator: "/", maxSplits: 1).map(String.init)
    guard let address = parts.first, !address.isEmpty else {
      throw PacketTunnelProviderError.invalidConfiguration("invalid cidr: \(cidr)")
    }
    self.address = address
    let prefixLength: Int
    if parts.count == 2 {
      guard let value = Int(parts[1]), (0...32).contains(value) else {
        throw PacketTunnelProviderError.invalidConfiguration("invalid cidr prefix: \(cidr)")
      }
      prefixLength = value
    } else {
      prefixLength = 32
    }
    subnetMask = Self.mask(for: prefixLength)
  }

  private static func mask(for prefixLength: Int) -> String {
    let maskValue = prefixLength == 0 ? UInt32(0) : UInt32.max << (32 - UInt32(prefixLength))
    let octets = [
      (maskValue >> 24) & 0xff,
      (maskValue >> 16) & 0xff,
      (maskValue >> 8) & 0xff,
      maskValue & 0xff,
    ]
    return octets.map { String($0) }.joined(separator: ".")
  }
}

struct PacketTunnelProviderError: LocalizedError {
  let message: String

  static func invalidConfiguration(_ message: String) -> Self {
    Self(message: message)
  }

  var errorDescription: String? {
    message
  }
}

struct PacketTunnelProviderRuntimeView {
  let state: String
  let debugEngineMode: String
  let backendName: String?
  let backendState: String?
  let backendLastError: String?
  let backendLastStartedAtMs: Int64?
  let backendPeerVirtualIp: String?
  let backendSelectedEndpoint: String?
  let peerVirtualIp: String
  let selectedEndpoint: String?
  let interfaceName: String?
  let dnsServers: [String]
  let allowedIps: [String]
  let localVirtualIp: String
  let remoteAddress: String
  let mtu: Int?
  let interfaceAddresses: [String]
  let includedRoutes: [String]
  let packetRxCount: Int64
  let packetRxBytes: Int64
  let packetTxCount: Int64
  let packetTxBytes: Int64
  let lastPacketAtMs: Int64?
  let lastAppliedAtMs: Int64?
  let lastError: String?

  func toJson() -> [String: Any] {
    [
      "state": state,
      "debugEngineMode": debugEngineMode,
      "backendName": backendName ?? NSNull(),
      "backendState": backendState ?? NSNull(),
      "backendLastError": backendLastError ?? NSNull(),
      "backendLastStartedAtMs": backendLastStartedAtMs ?? NSNull(),
      "backendPeerVirtualIp": backendPeerVirtualIp ?? NSNull(),
      "backendSelectedEndpoint": backendSelectedEndpoint ?? NSNull(),
      "peerVirtualIp": peerVirtualIp,
      "selectedEndpoint": selectedEndpoint ?? NSNull(),
      "interfaceName": interfaceName ?? NSNull(),
      "dnsServers": dnsServers,
      "allowedIps": allowedIps,
      "localVirtualIp": localVirtualIp,
      "remoteAddress": remoteAddress,
      "mtu": mtu ?? NSNull(),
      "interfaceAddresses": interfaceAddresses,
      "includedRoutes": includedRoutes,
      "packetRxCount": packetRxCount,
      "packetRxBytes": packetRxBytes,
      "packetTxCount": packetTxCount,
      "packetTxBytes": packetTxBytes,
      "lastPacketAtMs": lastPacketAtMs ?? NSNull(),
      "lastAppliedAtMs": lastAppliedAtMs ?? NSNull(),
      "lastError": lastError ?? NSNull(),
    ]
  }
}

enum PacketTunnelProviderSupport {
  static func makeWireGuardEngine(
    from configuration: PacketTunnelProviderConfiguration,
    backendAdapter: WireGuardBackendAdapting = WireGuardKitBackendAdapter()
  ) -> WireGuardEngine {
    WireGuardEngineFactory.make(from: configuration, backendAdapter: backendAdapter)
  }

  static func makeNetworkSettings(
    from configuration: PacketTunnelProviderConfiguration
  ) throws -> NEPacketTunnelNetworkSettings {
    let settings = NEPacketTunnelNetworkSettings(tunnelRemoteAddress: configuration.remoteAddress)

    let interfaceAddress = configuration.interfaceAddress
    let ipv4Settings = NEIPv4Settings(
      addresses: [interfaceAddress.address],
      subnetMasks: [interfaceAddress.subnetMask]
    )
    let includedRoutes = configuration.allowedIPv4Routes
    ipv4Settings.includedRoutes = includedRoutes.isEmpty ? [NEIPv4Route.default()] : includedRoutes
    settings.ipv4Settings = ipv4Settings

    if !configuration.dnsServers.isEmpty {
      settings.dnsSettings = NEDNSSettings(servers: configuration.dnsServers)
    }
    if let mtu = configuration.mtu {
      settings.mtu = NSNumber(value: mtu)
    }
    return settings
  }

  static func makeRuntimeView(
    configuration: PacketTunnelProviderConfiguration,
    settings: NEPacketTunnelNetworkSettings?,
    state: String,
    backendSnapshot: WireGuardBackendRuntimeSnapshot? = nil,
    packetRxCount: Int64 = 0,
    packetRxBytes: Int64 = 0,
    packetTxCount: Int64 = 0,
    packetTxBytes: Int64 = 0,
    lastPacketAtMs: Int64? = nil,
    lastAppliedAtMs: Int64? = nil,
    lastError: String? = nil
  ) -> PacketTunnelProviderRuntimeView {
    PacketTunnelProviderRuntimeView(
      state: state,
      debugEngineMode: configuration.debugEngineMode.rawValue,
      backendName: backendSnapshot?.backendName,
      backendState: backendSnapshot?.backendState,
      backendLastError: backendSnapshot?.lastBackendError,
      backendLastStartedAtMs: backendSnapshot?.lastStartedAtMs,
      backendPeerVirtualIp: backendSnapshot?.peerVirtualIp,
      backendSelectedEndpoint: backendSnapshot?.selectedEndpoint,
      peerVirtualIp: configuration.peerVirtualIp,
      selectedEndpoint: configuration.selectedEndpoint ?? configuration.remoteAddress,
      interfaceName: "utun",
      dnsServers: configuration.dnsServers,
      allowedIps: configuration.allowedIps,
      localVirtualIp: configuration.localVirtualIp,
      remoteAddress: settings?.tunnelRemoteAddress ?? configuration.remoteAddress,
      mtu: settings?.mtu?.intValue ?? configuration.mtu,
      interfaceAddresses: settings?.ipv4Settings?.addresses ?? [configuration.interfaceAddress.address],
      includedRoutes: settings?.ipv4Settings?.includedRoutes?.map { route in
        "\(route.destinationAddress)/\(route.destinationSubnetMask)"
      } ?? configuration.allowedIps,
      packetRxCount: packetRxCount,
      packetRxBytes: packetRxBytes,
      packetTxCount: packetTxCount,
      packetTxBytes: packetTxBytes,
      lastPacketAtMs: lastPacketAtMs,
      lastAppliedAtMs: lastAppliedAtMs,
      lastError: lastError
    )
  }
}
