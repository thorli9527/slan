import Foundation

enum SLANIosSharedStore {
  static let appGroupIdentifier = "group.dev.slan.client.v2"
  static let networkConfigKey = "dev.slan.client.v2.ios.networkConfig"
  static let packetTunnelStatsKey = "dev.slan.client.v2.packetTunnelStats"

  private static let networkConfigFile = "network-config.json"
  private static let packetTunnelStatsFile = "packet-tunnel-stats.json"

  static func readNetworkConfig() -> [String: Any]? {
    readJson(fileName: networkConfigFile, defaultsKey: networkConfigKey)
  }

  static func writeNetworkConfig(_ config: [String: Any]) {
    writeJson(config, fileName: networkConfigFile, defaultsKey: networkConfigKey)
  }

  static func readPacketTunnelStats() -> [String: Any]? {
    readJson(fileName: packetTunnelStatsFile, defaultsKey: packetTunnelStatsKey)
  }

  static func writePacketTunnelStats(_ stats: [String: Any]) {
    writeJson(stats, fileName: packetTunnelStatsFile, defaultsKey: packetTunnelStatsKey)
  }

  static func diagnostics() -> [String: Any] {
    [
      "appGroupIdentifier": appGroupIdentifier,
      "appGroupAvailable": sharedContainerURL() != nil,
      "networkConfigPresent": readNetworkConfig() != nil,
      "packetTunnelStatsPresent": readPacketTunnelStats() != nil,
      "networkConfigFile": sharedFileURL(fileName: networkConfigFile)?.path as Any,
      "packetTunnelStatsFile": sharedFileURL(fileName: packetTunnelStatsFile)?.path as Any
    ]
  }

  private static func readJson(fileName: String, defaultsKey: String) -> [String: Any]? {
    if let url = sharedFileURL(fileName: fileName),
      let data = try? Data(contentsOf: url),
      let object = try? JSONSerialization.jsonObject(with: data) as? [String: Any]
    {
      return object
    }
    guard let json = UserDefaults.standard.string(forKey: defaultsKey),
      let data = json.data(using: .utf8)
    else {
      return nil
    }
    return try? JSONSerialization.jsonObject(with: data) as? [String: Any]
  }

  private static func writeJson(_ object: [String: Any], fileName: String, defaultsKey: String) {
    guard let data = try? JSONSerialization.data(withJSONObject: object) else {
      return
    }
    if let url = sharedFileURL(fileName: fileName) {
      try? FileManager.default.createDirectory(
        at: url.deletingLastPathComponent(),
        withIntermediateDirectories: true
      )
      try? data.write(to: url, options: .atomic)
    }
    if let json = String(data: data, encoding: .utf8) {
      UserDefaults.standard.set(json, forKey: defaultsKey)
    }
  }

  private static func sharedFileURL(fileName: String) -> URL? {
    sharedContainerURL()?.appendingPathComponent(fileName)
  }

  private static func sharedContainerURL() -> URL? {
    FileManager.default
      .containerURL(forSecurityApplicationGroupIdentifier: appGroupIdentifier)
  }
}
