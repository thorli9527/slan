import Foundation

enum WireGuardTunnelConfigMapper {
  static func mapConfiguration(_ json: [String: Any]) throws -> WireGuardTunnelConfiguration {
    WireGuardTunnelConfiguration(
      transport: try requireString("transport", in: json),
      localVirtualIp: try requireString("localVirtualIp", in: json),
      peerVirtualIp: try requireString("peerVirtualIp", in: json),
      debugEngineMode: json["debugEngineMode"] as? String,
      interface: try mapInterface(try requireObject("wireguardInterface", in: json)),
      peer: try mapPeer(try requireObject("wireguardPeer", in: json))
    )
  }

  private static func mapInterface(_ json: [String: Any]) throws -> WireGuardTunnelInterfaceConfiguration {
    WireGuardTunnelInterfaceConfiguration(
      interfaceName: json["interfaceName"] as? String,
      keyPair: try mapKeyPair(try requireObject("keyPair", in: json)),
      listenPort: json["listenPort"] as? Int,
      mtu: json["mtu"] as? Int,
      addresses: try requireStringArray("addresses", in: json),
      dnsServers: requireOptionalStringArray("dnsServers", in: json)
    )
  }

  private static func mapPeer(_ json: [String: Any]) throws -> WireGuardTunnelPeerConfiguration {
    WireGuardTunnelPeerConfiguration(
      peerNodeId: json["peerNodeId"] as? String,
      publicKey: try requireString("publicKey", in: json),
      presharedKey: json["presharedKey"] as? String,
      endpoint: json["endpoint"] as? String,
      allowedIps: try mapAllowedIps(try requireArray("allowedIps", in: json)),
      persistentKeepaliveSeconds: json["persistentKeepaliveSeconds"] as? Int
    )
  }

  private static func mapKeyPair(_ json: [String: Any]) throws -> WireGuardTunnelKeyPair {
    WireGuardTunnelKeyPair(
      publicKey: try requireString("publicKey", in: json),
      privateKey: try requireString("privateKey", in: json)
    )
  }

  private static func mapAllowedIps(_ values: [Any]) throws -> [String] {
    try values.enumerated().map { index, value in
      guard let item = value as? [String: Any] else {
        throw SlanAppCorePluginError(
          code: "app_core_invalid_tunnel_config",
          message: "allowedIps[\(index)] must be an object"
        )
      }
      return try requireString("cidr", in: item)
    }
  }

  private static func requireObject(_ key: String, in json: [String: Any]) throws -> [String: Any] {
    guard let value = json[key] as? [String: Any] else {
      throw SlanAppCorePluginError(
        code: "app_core_invalid_tunnel_config",
        message: "Missing or invalid object field \(key)"
      )
    }
    return value
  }

  private static func requireArray(_ key: String, in json: [String: Any]) throws -> [Any] {
    guard let value = json[key] as? [Any] else {
      throw SlanAppCorePluginError(
        code: "app_core_invalid_tunnel_config",
        message: "Missing or invalid array field \(key)"
      )
    }
    return value
  }

  private static func requireString(_ key: String, in json: [String: Any]) throws -> String {
    guard let value = json[key] as? String, !value.isEmpty else {
      throw SlanAppCorePluginError(
        code: "app_core_invalid_tunnel_config",
        message: "Missing or invalid string field \(key)"
      )
    }
    return value
  }

  private static func requireStringArray(_ key: String, in json: [String: Any]) throws -> [String] {
    guard let values = json[key] as? [String], !values.isEmpty else {
      throw SlanAppCorePluginError(
        code: "app_core_invalid_tunnel_config",
        message: "Missing or invalid string array field \(key)"
      )
    }
    return values
  }

  private static func requireOptionalStringArray(_ key: String, in json: [String: Any]) -> [String] {
    json[key] as? [String] ?? []
  }
}
