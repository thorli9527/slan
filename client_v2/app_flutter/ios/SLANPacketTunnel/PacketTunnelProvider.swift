import NetworkExtension
import Network
import os.log

// iOS NetworkExtension 的包隧道入口，负责把系统 utun 收到的 IPv4 包路由到直连 UDP 或中继通道。
final class PacketTunnelProvider: NEPacketTunnelProvider {
  private static let logger = OSLog(subsystem: "dev.slan.client.v2", category: "PacketTunnel")
  // iOS 虚拟网卡地址固定按主机路由下发，真实可达子网由 includedRoutes 和 routeTable 控制。
  private static let hostInterfacePrefixLen = 32

  // PacketFlow 读取循环开关，stopTunnel 会置 false 终止下一轮异步读取。
  private var readingPackets = false
  // 服务端下发的可达子网表，用于判断每个出站 IPv4 包应该走 SLAN 数据面还是丢弃统计。
  private var routeTable: [RouteEntry] = []
  // RelayRuntime 内部同时管理 UDP 直链和中继路径，PacketTunnelProvider 只负责按目的地址投递。
  private var relayRuntime: RelayRuntime?
  // 暴露给宿主 App 查询的运行统计，便于客户端页面诊断路由、直链和中继状态。
  private var tunnelStats = PacketTunnelStats()

  // 启动 iOS utun：读取共享配置、安装虚拟地址和路由，再启动数据面读包循环。
  override func startTunnel(
    options: [String: NSObject]?,
    completionHandler: @escaping (Error?) -> Void
  ) {
    let config = SLANIosSharedStore.readNetworkConfig()
      ?? (protocolConfiguration as? NETunnelProviderProtocol)?
      .providerConfiguration ?? [:]
    guard var virtualIp = config["virtualIp"] as? String, !virtualIp.isEmpty else {
      completionHandler(PacketTunnelError("missing virtualIp"))
      return
    }
    let configuredPrefixLen = Self.addressPrefixLen(config: config, virtualIp: &virtualIp)

    let networkSettings = NEPacketTunnelNetworkSettings(tunnelRemoteAddress: "10.255.0.1")
    let ipv4 = NEIPv4Settings(
      addresses: [virtualIp],
      subnetMasks: [Self.mask(Self.hostInterfacePrefixLen)]
    )
    routeTable = Self.routeEntries(config["routes"])
    let routes = routeTable.map {
      NEIPv4Route(destinationAddress: $0.destination, subnetMask: $0.mask)
    }
    ipv4.includedRoutes = routes.isEmpty ? [NEIPv4Route.default()] : routes
    networkSettings.ipv4Settings = ipv4

    if let dnsServers = config["dnsServers"] as? [String], !dnsServers.isEmpty {
      networkSettings.dnsSettings = NEDNSSettings(servers: dnsServers)
    }
    if let mtu = config["mtu"] as? Int, mtu >= 576 {
      networkSettings.mtu = NSNumber(value: mtu)
    } else {
      networkSettings.mtu = 1280
    }

    setTunnelNetworkSettings(networkSettings) { [weak self] error in
      if error == nil {
        guard let self = self else {
          return
        }
        self.tunnelStats = PacketTunnelStats(
          networkId: config["networkId"] as? String ?? "",
          deviceId: config["deviceId"] as? String ?? "",
          virtualIp: virtualIp,
          routeCount: self.routeTable.count
        )
        self.relayRuntime = RelayRuntime(
          config: config["relayDataPlane"],
          localVirtualIp: virtualIp,
          packetFlow: self.packetFlow
        )
        self.tunnelStats.relaySessionCount = self.relayRuntime?.sessionCount ?? 0
        self.relayRuntime?.start()
        self.persistStats()
        os_log(
          "SLAN PacketTunnel started virtualIp=%{public}@/%{public}d configuredPrefix=%{public}d routes=%{public}d relaySessions=%{public}d",
          log: Self.logger,
          type: .info,
          virtualIp,
          Self.hostInterfacePrefixLen,
          configuredPrefixLen,
          self.routeTable.count,
          self.tunnelStats.relaySessionCount
        )
        self.readingPackets = true
        self.readPackets()
      }
      completionHandler(error)
    }
  }

  // 停止隧道时关闭读包循环和数据面运行时，并把最后状态写回共享存储。
  override func stopTunnel(
    with reason: NEProviderStopReason,
    completionHandler: @escaping () -> Void
  ) {
    readingPackets = false
    relayRuntime?.stop()
    relayRuntime = nil
    tunnelStats.stoppedAtMs = Self.nowMs()
    persistStats()
    os_log("SLAN PacketTunnel stopped reason=%{public}d", log: Self.logger, type: .info, reason.rawValue)
    completionHandler()
  }

  // 宿主 App 通过 NetworkExtension 消息查询统计信息，目前只处理 stats 命令。
  override func handleAppMessage(
    _ messageData: Data,
    completionHandler: ((Data?) -> Void)?
  ) {
    let command = String(data: messageData, encoding: .utf8) ?? ""
    guard command == "stats" else {
      completionHandler?(nil)
      return
    }
    if let relayRuntime = relayRuntime {
      tunnelStats.relayAttachedSessionCount = relayRuntime.attachedSessionCount
      tunnelStats.relayAttachFailures = relayRuntime.attachFailureCount
      tunnelStats.lastRelayAttachError = relayRuntime.lastAttachError
      tunnelStats.relayFramesReceived = relayRuntime.framesReceived
      tunnelStats.relayPacketsWritten = relayRuntime.packetsWritten
      tunnelStats.relayDetachSent = relayRuntime.detachSentCount
      tunnelStats.directUdpAttachedPeerCount = relayRuntime.directUdpAttachedPeerCount
      tunnelStats.directUdpReadyPeerCount = relayRuntime.directUdpReadyPeerCount
      tunnelStats.directUdpProbesSent = relayRuntime.directUdpProbesSent
      tunnelStats.directUdpProbesReceived = relayRuntime.directUdpProbesReceived
      tunnelStats.directUdpPongsSent = relayRuntime.directUdpPongsSent
      tunnelStats.directUdpPongsReceived = relayRuntime.directUdpPongsReceived
      tunnelStats.directUdpFramesSent = relayRuntime.directUdpFramesSent
      tunnelStats.directUdpFramesReceived = relayRuntime.directUdpFramesReceived
    }
    let data = try? JSONSerialization.data(withJSONObject: tunnelStats.dictionary)
    completionHandler?(data)
  }

  // 持续从 NEPacketTunnelFlow 异步读取系统写入 utun 的 IP 包。
  private func readPackets() {
    guard readingPackets else {
      return
    }
    packetFlow.readPackets { [weak self] packets, protocols in
      guard let self = self else {
        return
      }
      self.handlePackets(packets, protocols: protocols)
      self.readPackets()
    }
  }

  // 对每个出站包执行 IPv4 解析、路由匹配、校验和归一化和路径发送。
  private func handlePackets(_ packets: [Data], protocols: [NSNumber]) {
    guard !packets.isEmpty else {
      return
    }
    for packet in packets {
      tunnelStats.packetsRead += 1
      tunnelStats.bytesRead += packet.count
      guard let destination = Self.ipv4Destination(packet) else {
        tunnelStats.nonIpv4Packets += 1
        continue
      }
      if destination.address == RelayPeerRuntime.normalizeVirtualIp(tunnelStats.virtualIp) {
        continue
      }

      tunnelStats.lastDestination = destination.address
      if let route = routeTable.first(where: { $0.contains(destination.value) }) {
        tunnelStats.routedPackets += 1
        tunnelStats.lastRoute = route.cidr
        tunnelStats.lastRoutedAtMs = Self.nowMs()
        let outboundPacket = Ipv4Packet.normalizeTransportChecksums(packet)
        if let relayRuntime = relayRuntime,
          relayRuntime.send(packet: outboundPacket, destination: destination.address)
        {
          if relayRuntime.lastSendPath == "direct_udp" {
            tunnelStats.directUdpFramesSent = relayRuntime.directUdpFramesSent
          } else {
            tunnelStats.relayFramesSent += 1
          }
        } else if relayRuntime != nil {
          tunnelStats.relayNoPeerPackets += 1
        }
        os_log(
          "SLAN PacketTunnel packet matched destination=%{public}@ route=%{public}@ len=%{public}d",
          log: Self.logger,
          type: .debug,
          destination.address,
          route.cidr,
          packet.count
        )
      } else {
        tunnelStats.unroutablePackets += 1
      }
    }
    persistStats()
  }

  // 将最新运行统计写入 App Group，共宿主 App 页面和诊断入口读取。
  private func persistStats() {
    tunnelStats.updatedAtMs = Self.nowMs()
    SLANIosSharedStore.writePacketTunnelStats(tunnelStats.dictionary)
  }

  // 从服务端下发的 routes 配置解析 CIDR，生成本地快速匹配表。
  private static func routeEntries(_ value: Any?) -> [RouteEntry] {
    guard let routes = value as? [[String: Any]] else {
      return []
    }
    return routes.compactMap { route in
      guard
        let destination = route["destination"] as? String,
        let parsed = parseCidr(destination)
      else {
        return nil
      }
      return RouteEntry(
        cidr: destination,
        destination: parsed.address,
        mask: parsed.mask,
        network: parsed.network,
        prefix: parsed.prefix
      )
    }
  }

  // 兼容 virtualIp 内联 CIDR 和单独 prefix 字段，返回服务端配置的地址前缀。
  private static func addressPrefixLen(config: [String: Any], virtualIp: inout String) -> Int {
    if let parsed = parseCidr(virtualIp) {
      virtualIp = parsed.address
      return parsed.prefix
    }
    for key in ["prefixLen", "prefixLength", "subnetPrefixLen", "subnetPrefixLength"] {
      if let prefix = intValue(config[key]), (0...32).contains(prefix) {
        return prefix
      }
    }
    return 32
  }

  private static func intValue(_ value: Any?) -> Int? {
    switch value {
    case let value as Int:
      return value
    case let value as NSNumber:
      return value.intValue
    case let value as String:
      return Int(value.trimmingCharacters(in: .whitespacesAndNewlines))
    default:
      return nil
    }
  }

  // 解析 CIDR 字符串，得到地址、掩码和网络号，供 includedRoutes 与 routeTable 复用。
  private static func parseCidr(_ cidr: String) -> (
    address: String,
    mask: String,
    network: UInt32,
    prefix: Int
  )? {
    let parts = cidr.split(separator: "/")
    guard parts.count == 2, let prefix = Int(parts[1]), (0...32).contains(prefix) else {
      return nil
    }
    let address = String(parts[0])
    guard let value = ipv4Value(address) else {
      return nil
    }
    let networkMask = maskValue(prefix)
    return (address, mask(prefix), value & networkMask, prefix)
  }

  // 从 IPv4 包头提取目的地址，非 IPv4 或长度不足时返回 nil。
  private static func ipv4Destination(_ packet: Data) -> (address: String, value: UInt32)? {
    guard packet.count >= 20, packet[0] >> 4 == 4 else {
      return nil
    }
    let value =
      UInt32(packet[16]) << 24
      | UInt32(packet[17]) << 16
      | UInt32(packet[18]) << 8
      | UInt32(packet[19])
    return (ipv4String(value), value)
  }

  private static func ipv4Value(_ address: String) -> UInt32? {
    let parts = address.split(separator: ".")
    guard parts.count == 4 else {
      return nil
    }
    var value = UInt32(0)
    for part in parts {
      guard let octet = UInt32(part), octet <= 255 else {
        return nil
      }
      value = (value << 8) | octet
    }
    return value
  }

  private static func ipv4String(_ value: UInt32) -> String {
    [
      (value >> 24) & 0xff,
      (value >> 16) & 0xff,
      (value >> 8) & 0xff,
      value & 0xff
    ]
    .map(String.init)
    .joined(separator: ".")
  }

  private static func mask(_ prefix: Int) -> String {
    ipv4String(maskValue(prefix))
  }

  private static func maskValue(_ prefix: Int) -> UInt32 {
    if prefix == 0 {
      return 0
    }
    return UInt32.max << UInt32(32 - prefix)
  }

  private static func nowMs() -> Int64 {
    Int64(Date().timeIntervalSince1970 * 1000)
  }
}

// 本地路由表条目，保存服务端下发子网的 CIDR、掩码和网络号。
private struct RouteEntry {
  let cidr: String
  let destination: String
  let mask: String
  let network: UInt32
  let prefix: Int

  // 判断目标 IPv4 地址是否落入当前 CIDR。
  func contains(_ address: UInt32) -> Bool {
    let mask = PacketTunnelProviderMask.value(prefix)
    return (address & mask) == network
  }
}

// PacketTunnelProvider 使用的 IPv4 掩码工具，避免在路由匹配时重复解析字符串。
private enum PacketTunnelProviderMask {
  static func value(_ prefix: Int) -> UInt32 {
    if prefix == 0 {
      return 0
    }
    return UInt32.max << UInt32(32 - prefix)
  }
}

// IPv4 包处理工具，负责修正校验和以及生成本机 ICMP Echo Reply。
private enum Ipv4Packet {
  // iOS utun 出站包在再次封装前需要重算 IP/TCP/UDP 校验和，避免远端协议栈丢包。
  static func normalizeTransportChecksums(_ packet: Data) -> Data {
    guard packet.count >= 20, packet[0] >> 4 == 4 else {
      return packet
    }
    let ihl = Int(packet[0] & 0x0f) * 4
    guard ihl >= 20, packet.count >= ihl else {
      return packet
    }
    let totalLen = Int(packet.readUInt16(at: 2))
    guard totalLen >= ihl, totalLen <= packet.count else {
      return packet
    }
    var normalized = packet.subdata(in: 0..<totalLen)
    normalized[10] = 0
    normalized[11] = 0
    normalized.replaceSubrange(10..<12, with: checksum(normalized.subdata(in: 0..<ihl)).bytes)
    let flagsFragment = normalized.readUInt16(at: 6)
    if flagsFragment & 0x1fff != 0 {
      return normalized
    }
    if normalized[9] == 6 {
      normalizeTcpChecksum(&normalized, ihl: ihl, totalLen: totalLen)
    } else if normalized[9] == 17 {
      normalizeUdpChecksum(&normalized, ihl: ihl, totalLen: totalLen)
    }
    return normalized
  }

  // 对发往本机虚拟 IP 的 ICMP Echo Request 生成本地响应，用于基础连通性探测。
  static func icmpEchoReply(for packet: Data, localVirtualIp: String) -> Data? {
    guard packet.count >= 28, packet[0] >> 4 == 4 else {
      return nil
    }
    let ihl = Int(packet[0] & 0x0f) * 4
    guard ihl >= 20, packet.count >= ihl + 8, packet[9] == 1 else {
      return nil
    }
    let totalLen = Int(packet.readUInt16(at: 2))
    guard totalLen >= ihl + 8, totalLen <= packet.count else {
      return nil
    }
    let flagsFragment = packet.readUInt16(at: 6)
    guard flagsFragment & 0x1fff == 0,
      ipv4String(packet, offset: 16) == RelayPeerRuntime.normalizeVirtualIp(localVirtualIp),
      packet[ihl] == 8,
      packet[ihl + 1] == 0
    else {
      return nil
    }
    var reply = packet.subdata(in: 0..<totalLen)
    let source = reply.subdata(in: 12..<16)
    let destination = reply.subdata(in: 16..<20)
    reply.replaceSubrange(12..<16, with: destination)
    reply.replaceSubrange(16..<20, with: source)
    reply[8] = 64
    reply[10] = 0
    reply[11] = 0
    reply[ihl] = 0
    reply[ihl + 2] = 0
    reply[ihl + 3] = 0
    reply.replaceSubrange(ihl + 2..<ihl + 4, with: checksum(reply.subdata(in: ihl..<totalLen)).bytes)
    reply.replaceSubrange(10..<12, with: checksum(reply.subdata(in: 0..<ihl)).bytes)
    return reply
  }

  private static func normalizeTcpChecksum(_ packet: inout Data, ihl: Int, totalLen: Int) {
    let tcpLen = totalLen - ihl
    guard tcpLen >= 20 else {
      return
    }
    packet[ihl + 16] = 0
    packet[ihl + 17] = 0
    let sum = transportChecksum(packet, offset: ihl, length: tcpLen, proto: 6)
    packet.replaceSubrange(ihl + 16..<ihl + 18, with: sum.bytes)
  }

  private static func normalizeUdpChecksum(_ packet: inout Data, ihl: Int, totalLen: Int) {
    let udpLen = totalLen - ihl
    guard udpLen >= 8 else {
      return
    }
    let declaredLen = Int(packet.readUInt16(at: ihl + 4))
    guard declaredLen >= 8, declaredLen <= udpLen else {
      return
    }
    packet[ihl + 6] = 0
    packet[ihl + 7] = 0
    let sum = transportChecksum(packet, offset: ihl, length: declaredLen, proto: 17)
    packet.replaceSubrange(ihl + 6..<ihl + 8, with: (sum == 0 ? UInt16.max : sum).bytes)
  }

  private static func transportChecksum(
    _ packet: Data,
    offset: Int,
    length: Int,
    proto: UInt8
  ) -> UInt16 {
    var bytes = Data()
    bytes.append(packet.subdata(in: 12..<20))
    bytes.append(0)
    bytes.append(proto)
    bytes.append(contentsOf: UInt16(length).bigEndianBytes)
    bytes.append(packet.subdata(in: offset..<offset + length))
    return checksum(bytes)
  }

  private static func checksum(_ data: Data) -> UInt16 {
    var sum: UInt32 = 0
    var index = 0
    while index < data.count {
      let high = UInt16(data[index]) << 8
      let low = index + 1 < data.count ? UInt16(data[index + 1]) : 0
      sum = sum &+ UInt32(high | low)
      index += 2
    }
    while (sum >> 16) != 0 {
      sum = (sum & 0xffff) + (sum >> 16)
    }
    return ~UInt16(sum & 0xffff)
  }

  private static func ipv4String(_ packet: Data, offset: Int) -> String {
    [
      packet[offset],
      packet[offset + 1],
      packet[offset + 2],
      packet[offset + 3]
    ]
    .map(String.init)
    .joined(separator: ".")
  }
}

// PacketTunnel 运行统计快照，会序列化到共享存储并通过 handleAppMessage 返回给宿主 App。
private struct PacketTunnelStats {
  var networkId = ""
  var deviceId = ""
  var virtualIp = ""
  var routeCount = 0
  var relaySessionCount = 0
  var relayAttachedSessionCount = 0
  var relayAttachFailures = 0
  var lastRelayAttachError = ""
  var packetsRead = 0
  var bytesRead = 0
  var routedPackets = 0
  var unroutablePackets = 0
  var nonIpv4Packets = 0
  var relayFramesSent = 0
  var relayFramesReceived = 0
  var relayPacketsWritten = 0
  var bytesWritten = 0
  var relayDetachSent = 0
  var relayNoPeerPackets = 0
  var directUdpAttachedPeerCount = 0
  var directUdpReadyPeerCount = 0
  var directUdpProbesSent = 0
  var directUdpProbesReceived = 0
  var directUdpPongsSent = 0
  var directUdpPongsReceived = 0
  var directUdpFramesSent = 0
  var directUdpFramesReceived = 0
  var lastDestination = ""
  var lastRoute = ""
  var lastRoutedAtMs: Int64 = 0
  var updatedAtMs: Int64 = 0
  var stoppedAtMs: Int64 = 0

  var dictionary: [String: Any] {
    [
      "networkId": networkId,
      "deviceId": deviceId,
      "virtualIp": virtualIp,
      "routeCount": routeCount,
      "relaySessionCount": relaySessionCount,
      "relayAttachedSessionCount": relayAttachedSessionCount,
      "relayAttachFailures": relayAttachFailures,
      "lastRelayAttachError": lastRelayAttachError,
      "packetsRead": packetsRead,
      "bytesRead": bytesRead,
      "routedPackets": routedPackets,
      "unroutablePackets": unroutablePackets,
      "nonIpv4Packets": nonIpv4Packets,
      "relayFramesSent": relayFramesSent,
      "relayFramesReceived": relayFramesReceived,
      "relayPacketsWritten": relayPacketsWritten,
      "bytesWritten": bytesWritten,
      "relayDetachSent": relayDetachSent,
      "relayNoPeerPackets": relayNoPeerPackets,
      "directUdpAttachedPeerCount": directUdpAttachedPeerCount,
      "directUdpReadyPeerCount": directUdpReadyPeerCount,
      "directUdpProbesSent": directUdpProbesSent,
      "directUdpProbesReceived": directUdpProbesReceived,
      "directUdpPongsSent": directUdpPongsSent,
      "directUdpPongsReceived": directUdpPongsReceived,
      "directUdpFramesSent": directUdpFramesSent,
      "directUdpFramesReceived": directUdpFramesReceived,
      "lastDestination": lastDestination,
      "lastRoute": lastRoute,
      "lastRoutedAtMs": lastRoutedAtMs,
      "updatedAtMs": updatedAtMs,
      "stoppedAtMs": stoppedAtMs
    ]
  }
}

// 中继数据面调度器：优先尝试 Direct UDP，失败时回落到 Relay UDP 会话。
private final class RelayRuntime {
  private let localNodeId: String
  private let localVirtualIp: String
  private let relayAddress: String
  private let maxFramePayload: Int
  private let packetFlow: NEPacketTunnelFlow
  private var peers: [RelayPeerRuntime] = []
  private var directUdpRuntime: DirectUdpRuntime?
  private var seq: UInt64 = 0
  private var configHash: UInt64 = 0
  private(set) var lastSendPath = ""

  // 服务端下发的可用中继会话数量。
  var sessionCount: Int {
    peers.count
  }

  var attachedSessionCount: Int {
    peers.filter { $0.attached }.count
  }

  var attachFailureCount: Int {
    peers.filter { !$0.attachError.isEmpty }.count
  }

  var lastAttachError: String {
    peers.reversed().first { !$0.attachError.isEmpty }?.attachError ?? ""
  }

  var framesReceived: Int {
    peers.reduce(0) { $0 + $1.framesReceived }
  }

  var packetsWritten: Int {
    peers.reduce(0) { $0 + $1.packetsWritten }
  }

  var detachSentCount: Int {
    peers.filter { $0.detachSent }.count
  }

  var directUdpAttachedPeerCount: Int {
    directUdpRuntime?.attachedPeerCount ?? 0
  }

  var directUdpReadyPeerCount: Int {
    directUdpRuntime?.readyPeerCount ?? 0
  }

  var directUdpProbesSent: Int {
    directUdpRuntime?.probesSent ?? 0
  }

  var directUdpProbesReceived: Int {
    directUdpRuntime?.probesReceived ?? 0
  }

  var directUdpPongsSent: Int {
    directUdpRuntime?.pongsSent ?? 0
  }

  var directUdpPongsReceived: Int {
    directUdpRuntime?.pongsReceived ?? 0
  }

  var directUdpFramesSent: Int {
    directUdpRuntime?.framesSent ?? 0
  }

  var directUdpFramesReceived: Int {
    directUdpRuntime?.framesReceived ?? 0
  }

  // 从 relayDataPlane 配置构建中继会话和直连候选，配置缺失时返回 nil。
  init?(config: Any?, localVirtualIp: String, packetFlow: NEPacketTunnelFlow) {
    guard let config = config as? [String: Any],
      config["enabled"] as? Bool == true
    else {
      return nil
    }
    let localNodeId = (config["localNodeId"] as? String ?? "")
      .trimmingCharacters(in: .whitespacesAndNewlines)
    let relayAddress = (config["relayAddress"] as? String ?? "")
      .trimmingCharacters(in: .whitespacesAndNewlines)
    let transport = (config["transport"] as? String ?? "")
      .trimmingCharacters(in: .whitespacesAndNewlines)
      .lowercased()
    guard !localNodeId.isEmpty, !relayAddress.isEmpty, transport == "udp" else {
      return nil
    }
    self.localNodeId = localNodeId
    self.localVirtualIp = RelayPeerRuntime.normalizeVirtualIp(localVirtualIp)
    self.relayAddress = relayAddress
    self.maxFramePayload = max(512, min(1400, config["maxFramePayload"] as? Int ?? 1200))
    self.packetFlow = packetFlow
    if let data = try? JSONSerialization.data(withJSONObject: config),
      let json = String(data: data, encoding: .utf8)
    {
      self.configHash = Self.stableHash64(json)
    }
    let sessions = config["sessions"] as? [[String: Any]] ?? []
    self.peers = sessions.compactMap { session in
      RelayPeerRuntime(
        session: session,
        relayAddress: relayAddress,
        localNodeId: localNodeId,
        localVirtualIp: self.localVirtualIp,
        configHash: configHash,
        packetFlow: packetFlow
      )
    }
    if peers.isEmpty {
      return nil
    }
    self.directUdpRuntime = DirectUdpRuntime(
      config: config,
      localNodeId: localNodeId,
      localVirtualIp: self.localVirtualIp,
      maxFramePayload: maxFramePayload,
      configHash: configHash,
      packetFlow: packetFlow
    )
  }

  // 启动所有 Relay UDP 会话和 Direct UDP 探测。
  func start() {
    peers.forEach { $0.start() }
    directUdpRuntime?.start()
  }

  // 停止所有底层 UDP 连接，并清理会话状态。
  func stop() {
    directUdpRuntime?.stop()
    directUdpRuntime = nil
    peers.forEach { $0.stop() }
    peers.removeAll()
  }

  // 按目的虚拟 IP 选择 peer，优先直链发送，直链不可用时发送到中继节点。
  func send(packet: Data, destination: String) -> Bool {
    guard packet.count <= maxFramePayload,
      let peer = peers.first(where: { $0.matches(destination) })
    else {
      return false
    }
    seq &+= 1
    guard let frame = Self.encodeFrame(seq: seq, configHash: configHash, payload: packet) else {
      return false
    }
    if directUdpRuntime?.send(frame: frame, destination: destination) == true {
      lastSendPath = "direct_udp"
      return true
    }
    peer.send(frame)
    lastSendPath = "relay_udp"
    return true
  }

  // 封装 SLAN 数据帧头，承载 utun 读取到的原始 IPv4 包。
  fileprivate static func encodeFrame(seq: UInt64, configHash: UInt64, payload: Data) -> Data? {
    guard payload.count <= UInt32.max else {
      return nil
    }
    var frame = Data()
    frame.append(contentsOf: [0x53, 0x4c, 0x41, 0x4e])
    frame.append(1)
    frame.append(1)
    frame.appendUInt16(32)
    frame.appendUInt64(seq)
    frame.appendUInt64(configHash)
    frame.appendUInt32(UInt32(payload.count))
    frame.appendUInt32(0)
    frame.append(payload)
    return frame
  }

  // 解出 SLAN 数据帧中的 IPv4 载荷，帧头非法或长度越界时丢弃。
  fileprivate static func decodeFrame(_ frame: Data) -> Data? {
    guard frame.count >= 32,
      frame[0] == 0x53,
      frame[1] == 0x4c,
      frame[2] == 0x41,
      frame[3] == 0x4e,
      frame[4] == 1,
      frame[5] == 1
    else {
      return nil
    }
    let headerLen = Int(frame.readUInt16(at: 6))
    let payloadLen = Int(frame.readUInt32(at: 24))
    let end = headerLen + payloadLen
    guard headerLen >= 32, end <= frame.count else {
      return nil
    }
    return frame.subdata(in: headerLen..<end)
  }

  fileprivate static func stableHash64(_ value: String) -> UInt64 {
    var hash: UInt64 = 0xcbf29ce484222325
    for byte in value.utf8 {
      hash ^= UInt64(byte)
      hash = hash &* 0x100000001b3
    }
    return hash
  }
}

// Direct UDP 运行时，聚合多个点对点候选路径并统计探测与数据帧状态。
private final class DirectUdpRuntime {
  private let localNodeId: String
  private let maxFramePayload: Int
  private let configHash: UInt64
  private let packetFlow: NEPacketTunnelFlow
  private var peers: [DirectUdpPeerRuntime]

  var attachedPeerCount: Int {
    peers.count
  }

  var readyPeerCount: Int {
    peers.filter { $0.ready }.count
  }

  var probesSent: Int {
    peers.reduce(0) { $0 + $1.probesSent }
  }

  var probesReceived: Int {
    peers.reduce(0) { $0 + $1.probesReceived }
  }

  var pongsSent: Int {
    peers.reduce(0) { $0 + $1.pongsSent }
  }

  var pongsReceived: Int {
    peers.reduce(0) { $0 + $1.pongsReceived }
  }

  var framesSent: Int {
    peers.reduce(0) { $0 + $1.framesSent }
  }

  var framesReceived: Int {
    peers.reduce(0) { $0 + $1.framesReceived }
  }

  // 从 peerPaths 构建可用直链 peer，没有可用候选时不启用直链。
  init?(
    config: [String: Any],
    localNodeId: String,
    localVirtualIp: String,
    maxFramePayload: Int,
    configHash: UInt64,
    packetFlow: NEPacketTunnelFlow
  ) {
    let peerPaths = config["peerPaths"] as? [[String: Any]] ?? []
    let peers = peerPaths.compactMap {
      DirectUdpPeerRuntime(
        peerPath: $0,
        localNodeId: localNodeId,
        localVirtualIp: localVirtualIp,
        configHash: configHash,
        packetFlow: packetFlow
      )
    }
    if peers.isEmpty {
      return nil
    }
    self.localNodeId = localNodeId
    self.maxFramePayload = maxFramePayload
    self.configHash = configHash
    self.packetFlow = packetFlow
    self.peers = peers
  }

  // 启动所有直链候选的 UDP 探测。
  func start() {
    peers.forEach { $0.start() }
  }

  // 停止直链 UDP 连接，避免 NetworkExtension 退出后仍持有 socket。
  func stop() {
    peers.forEach { $0.stop() }
    peers.removeAll()
  }

  // 只在目标 peer 已完成探测握手并 ready 后才走直链发送。
  func send(frame: Data, destination: String) -> Bool {
    guard frame.count <= maxFramePayload + 32,
      let peer = peers.first(where: { $0.matches(destination) && $0.ready })
    else {
      return false
    }
    return peer.send(frame)
  }
}

// 单个 Direct UDP peer，会对一个远端公网候选地址进行 probe/pong 打洞和数据收发。
private final class DirectUdpPeerRuntime {
  private let peerNodeId: String
  private let peerVirtualIps: Set<String>
  private let localNodeId: String
  private let localVirtualIp: String
  private let configHash: UInt64
  private let endpoint: (host: Network.NWEndpoint.Host, port: Network.NWEndpoint.Port)
  private let packetFlow: NEPacketTunnelFlow
  private var connection: NWConnection?
  private var running = false
  private(set) var ready = false
  private(set) var probesSent = 0
  private(set) var probesReceived = 0
  private(set) var pongsSent = 0
  private(set) var pongsReceived = 0
  private(set) var framesSent = 0
  private(set) var framesReceived = 0
  private var seq: UInt64 = 0

  // 解析服务端 peerPath，选择第一个 Direct UDP/LAN UDP/IPv6 UDP 候选作为远端端点。
  init?(
    peerPath: [String: Any],
    localNodeId: String,
    localVirtualIp: String,
    configHash: UInt64,
    packetFlow: NEPacketTunnelFlow
  ) {
    let peerNodeId = (peerPath["peerNodeId"] as? String ?? "")
      .trimmingCharacters(in: .whitespacesAndNewlines)
    let peerVirtualIps = (peerPath["peerVirtualIps"] as? [String] ?? [])
      .map(RelayPeerRuntime.normalizeVirtualIp)
      .filter { !$0.isEmpty }
    let candidates = peerPath["candidates"] as? [[String: Any]] ?? []
    guard let address = candidates.compactMap(Self.directUdpAddress).first,
      let endpoint = Self.endpoint(address),
      !peerNodeId.isEmpty,
      !peerVirtualIps.isEmpty
    else {
      return nil
    }
    self.peerNodeId = peerNodeId
    self.peerVirtualIps = Set(peerVirtualIps)
    self.localNodeId = localNodeId
    self.localVirtualIp = RelayPeerRuntime.normalizeVirtualIp(localVirtualIp)
    self.configHash = configHash
    self.endpoint = endpoint
    self.packetFlow = packetFlow
  }

  // 建立 UDP NWConnection，ready 后立即开始接收和周期性探测。
  func start() {
    guard connection == nil else {
      return
    }
    running = true
    let connection = NWConnection(host: endpoint.host, port: endpoint.port, using: .udp)
    connection.stateUpdateHandler = { [weak self] (state: NWConnection.State) in
      guard let self = self else {
        return
      }
      if case .ready = state {
        self.receive()
        self.sendProbe()
        self.scheduleProbe()
      }
      if case .failed = state {
        self.ready = false
      }
      if case .cancelled = state {
        self.ready = false
      }
    }
    self.connection = connection
    connection.start(queue: DispatchQueue.global(qos: .utility))
  }

  // 关闭 peer 直链连接并重置 ready 状态。
  func stop() {
    running = false
    ready = false
    connection?.cancel()
    connection = nil
  }

  // 判断目的虚拟 IP 是否属于当前 peer。
  func matches(_ destination: String) -> Bool {
    peerVirtualIps.contains(RelayPeerRuntime.normalizeVirtualIp(destination))
  }

  // 向已 ready 的直链 peer 发送封装后的 SLAN 数据帧。
  func send(_ frame: Data) -> Bool {
    guard running && ready else {
      return false
    }
    framesSent += 1
    connection?.send(content: frame, completion: .contentProcessed { _ in })
    return true
  }

  private func sendProbe() {
    sendControl(type: "probe")
    probesSent += 1
  }

  private func sendPong() {
    sendControl(type: "pong")
    pongsSent += 1
  }

  private func sendControl(type: String) {
    let payload: [String: Any] = [
      "kind": "direct_udp",
      "type": type,
      "nodeId": localNodeId
    ]
    guard let data = try? JSONSerialization.data(withJSONObject: payload) else {
      return
    }
    connection?.send(content: data, completion: .contentProcessed { _ in })
  }

  private func scheduleProbe() {
    DispatchQueue.global(qos: .utility).asyncAfter(deadline: .now() + 15) { [weak self] in
      guard let self = self, self.running else {
        return
      }
      self.sendProbe()
      self.scheduleProbe()
    }
  }

  private func receive() {
    connection?.receiveMessage { [weak self] data, _, _, _ in
      guard let self = self else {
        return
      }
      if let data = data {
        if self.consumeControl(data) {
          if self.running {
            self.receive()
          }
          return
        }
        if let packet = RelayRuntime.decodeFrame(data) {
          self.ready = true
          self.framesReceived += 1
          if let reply = Ipv4Packet.icmpEchoReply(for: packet, localVirtualIp: self.localVirtualIp)
          {
            self.seq &+= 1
            if let frame = RelayRuntime.encodeFrame(
              seq: self.seq,
              configHash: self.configHash,
              payload: reply
            ) {
              _ = self.send(frame)
            }
          } else {
            self.writePacketToFlow(Ipv4Packet.normalizeTransportChecksums(packet))
          }
        }
      }
      if self.running {
        self.receive()
      }
    }
  }

  // 处理直链控制消息，probe 会回 pong，pong/probe 都会把 peer 标记为 ready。
  private func consumeControl(_ data: Data) -> Bool {
    guard let value = try? JSONSerialization.jsonObject(with: data) as? [String: Any],
      value["kind"] as? String == "direct_udp",
      let type = value["type"] as? String
    else {
      return false
    }
    if let nodeId = value["nodeId"] as? String, !nodeId.isEmpty, nodeId != peerNodeId {
      return true
    }
    ready = true
    if type == "probe" {
      probesReceived += 1
      sendPong()
    } else if type == "pong" {
      pongsReceived += 1
    }
    return true
  }

  // 将远端回包写回 iOS utun；短暂拥塞时做有限次微延迟重试。
  private func writePacketToFlow(_ packet: Data, attempt: Int = 0) {
    if packetFlow.writePackets([packet], withProtocols: [NSNumber(value: AF_INET)]) {
      return
    }
    guard attempt < 50 else {
      return
    }
    DispatchQueue.global(qos: .utility).asyncAfter(deadline: .now() + 0.002) { [weak self] in
      self?.writePacketToFlow(packet, attempt: attempt + 1)
    }
  }

  // 从路径候选中筛选可以用于点对点直链的 UDP 地址。
  private static func directUdpAddress(_ candidate: [String: Any]) -> String? {
    let kind = (candidate["kind"] as? String ?? "")
      .trimmingCharacters(in: .whitespacesAndNewlines)
      .lowercased()
    guard kind == "direct_udp" || kind == "lan_udp" || kind == "ipv6_udp" else {
      return nil
    }
    let address = (candidate["address"] as? String ?? "")
      .trimmingCharacters(in: .whitespacesAndNewlines)
    return address.isEmpty ? nil : address
  }

  private static func endpoint(_ address: String) -> (
    host: Network.NWEndpoint.Host,
    port: Network.NWEndpoint.Port
  )? {
    let trimmed = address.trimmingCharacters(in: .whitespacesAndNewlines)
    let normalized: String
    if trimmed.hasPrefix("udp://") {
      normalized = String(trimmed.dropFirst("udp://".count))
    } else if trimmed.hasPrefix("direct+udp://") {
      normalized = String(trimmed.dropFirst("direct+udp://".count))
    } else if trimmed.hasPrefix("relay+udp://") {
      normalized = String(trimmed.dropFirst("relay+udp://".count))
    } else if trimmed.contains("://") {
      return nil
    } else {
      normalized = trimmed
    }
    guard let separator = normalized.lastIndex(of: ":") else {
      return nil
    }
    let host = String(normalized[..<separator])
    let port = String(normalized[normalized.index(after: separator)...])
    guard !host.isEmpty,
      let portValue = UInt16(port),
      let endpointPort = Network.NWEndpoint.Port(rawValue: portValue)
    else {
      return nil
    }
    return (Network.NWEndpoint.Host(host), endpointPort)
  }
}

// 单个 Relay UDP 会话，负责 attach/detach 票据握手和中继数据帧收发。
private final class RelayPeerRuntime {
  private static let maxAttachAttempts = 3

  private let sessionId: String
  private let peerVirtualIps: Set<String>
  private let relayAddress: String
  private let localNodeId: String
  private let localVirtualIp: String
  private let configHash: UInt64
  private let ticket: [String: Any]
  private let packetFlow: NEPacketTunnelFlow
  private var connection: NWConnection?
  private var ready = false
  private(set) var attached = false
  private(set) var attachError = ""
  private(set) var framesReceived = 0
  private(set) var packetsWritten = 0
  private(set) var detachSent = false
  private var attachAttempts = 0
  private var seq: UInt64 = 0

  // 从服务端 session 配置解析 relay ticket 与远端虚拟 IP 集合。
  init?(
    session: [String: Any],
    relayAddress: String,
    localNodeId: String,
    localVirtualIp: String,
    configHash: UInt64,
    packetFlow: NEPacketTunnelFlow
  ) {
    let sessionId = (session["sessionId"] as? String ?? "")
      .trimmingCharacters(in: .whitespacesAndNewlines)
    let ticket = session["ticket"] as? [String: Any] ?? [:]
    let peerVirtualIps = (session["peerVirtualIps"] as? [String] ?? [])
      .map(Self.normalizeVirtualIp)
      .filter { !$0.isEmpty }
    guard !sessionId.isEmpty, !peerVirtualIps.isEmpty, !ticket.isEmpty else {
      return nil
    }
    self.sessionId = sessionId
    self.peerVirtualIps = Set(peerVirtualIps)
    self.relayAddress = relayAddress
    self.localNodeId = localNodeId
    self.localVirtualIp = RelayPeerRuntime.normalizeVirtualIp(localVirtualIp)
    self.configHash = configHash
    self.ticket = ticket
    self.packetFlow = packetFlow
  }

  // 建立到 relay 节点的 UDP 连接，ready 后发送 attach 并进入接收循环。
  func start() {
    guard connection == nil,
      let endpoint = Self.endpoint(relayAddress)
    else {
      return
    }
    let connection = NWConnection(host: endpoint.host, port: endpoint.port, using: .udp)
    connection.stateUpdateHandler = { [weak self] (state: NWConnection.State) in
      guard let self = self else {
        return
      }
      if case .ready = state {
        self.ready = true
        self.attach()
        self.receive()
      }
      if case .failed = state {
        self.ready = false
      }
      if case .cancelled = state {
        self.ready = false
      }
    }
    self.connection = connection
    connection.start(queue: DispatchQueue.global(qos: .utility))
  }

  // 退出 relay 会话，尽量发送 detach 后再关闭 UDP 连接。
  func stop() {
    detach()
    ready = false
    connection?.cancel()
    connection = nil
  }

  // 判断目的虚拟 IP 是否应由当前 relay session 承载。
  func matches(_ destination: String) -> Bool {
    peerVirtualIps.contains(Self.normalizeVirtualIp(destination))
  }

  // Relay attach 成功后发送封装数据帧。
  func send(_ frame: Data) {
    guard ready && attached else {
      return
    }
    connection?.send(content: frame, completion: .contentProcessed { _ in })
  }

  // 使用服务端下发 ticket 向 relay 节点注册当前参与方。
  private func attach() {
    guard ready && !attached && attachAttempts < Self.maxAttachAttempts else {
      return
    }
    attachAttempts += 1
    let payload: [String: Any] = [
      "kind": "attach",
      "participant_id": localNodeId,
      "ticket": relayTicketWire(ticket)
    ]
    guard let data = try? JSONSerialization.data(withJSONObject: payload) else {
      return
    }
    connection?.send(content: data, completion: .contentProcessed { _ in })
    scheduleAttachTimeout(attempt: attachAttempts)
  }

  // 主动通知 relay 节点释放当前 session 参与方。
  private func detach() {
    guard ready && attached else {
      return
    }
    let payload: [String: Any] = [
      "kind": "detach",
      "session_id": sessionId,
      "participant_id": localNodeId
    ]
    guard let data = try? JSONSerialization.data(withJSONObject: payload) else {
      return
    }
    connection?.send(content: data, completion: .contentProcessed { _ in })
    detachSent = true
    attached = false
  }

  // attach 未确认时按固定次数重试，最终暴露错误给统计页面。
  private func scheduleAttachTimeout(attempt: Int) {
    DispatchQueue.global(qos: .utility).asyncAfter(deadline: .now() + 2) { [weak self] in
      guard let self = self, self.ready, !self.attached, self.attachAttempts == attempt else {
        return
      }
      if self.attachAttempts < Self.maxAttachAttempts {
        self.attach()
      } else {
        self.attachError = "relay attach timed out"
      }
    }
  }

  // 接收 relay 控制响应和数据帧，数据帧会写回 utun 或本地响应 ICMP。
  private func receive() {
    connection?.receiveMessage { [weak self] data, _, _, _ in
      guard let self = self else {
        return
      }
      if let data = data {
        if self.consumeAttachResponse(data) {
          if self.ready {
            self.receive()
          }
          return
        }
        if let packet = RelayRuntime.decodeFrame(data) {
          self.framesReceived += 1
          if let reply = Ipv4Packet.icmpEchoReply(for: packet, localVirtualIp: self.localVirtualIp)
          {
            self.seq &+= 1
            if let frame = RelayRuntime.encodeFrame(
              seq: self.seq,
              configHash: self.configHash,
              payload: reply
            ) {
              self.send(frame)
            }
          } else {
            self.writePacketToFlow(Ipv4Packet.normalizeTransportChecksums(packet))
          }
        }
      }
      if self.ready {
        self.receive()
      }
    }
  }

  // 将 relay 收到的远端 IP 包写回系统协议栈，写入失败时短暂重试。
  private func writePacketToFlow(_ packet: Data, attempt: Int = 0) {
    if packetFlow.writePackets([packet], withProtocols: [NSNumber(value: AF_INET)]) {
      packetsWritten += 1
      return
    }
    guard attempt < 50 else {
      return
    }
    DispatchQueue.global(qos: .utility).asyncAfter(deadline: .now() + 0.002) { [weak self] in
      self?.writePacketToFlow(packet, attempt: attempt + 1)
    }
  }

  // 解析 relay attach 的 attached/error 响应并更新会话状态。
  private func consumeAttachResponse(_ data: Data) -> Bool {
    guard let value = try? JSONSerialization.jsonObject(with: data) as? [String: Any],
      let kind = value["kind"] as? String
    else {
      return false
    }
    if kind == "attached" {
      let ackSessionId = (value["session_id"] as? String)
        ?? (value["sessionId"] as? String)
        ?? ""
      if ackSessionId == sessionId {
        attached = true
        attachError = ""
      } else {
        attached = false
        attachError = "relay attach session mismatch"
      }
      return true
    }
    if kind == "error" {
      attached = false
      attachError = (value["message"] as? String) ?? "relay attach failed"
      return true
    }
    return false
  }

  // 将客户端内部 ticket 字段映射为 relay 服务期望的下划线 JSON 字段。
  private func relayTicketWire(_ ticket: [String: Any]) -> [String: Any] {
    [
      "ticket_id": stringField(ticket, "ticketId"),
      "network_id": stringField(ticket, "networkId"),
      "session_id": stringField(ticket, "sessionId"),
      "src_node_id": stringField(ticket, "srcNodeId"),
      "dst_node_id": stringField(ticket, "dstNodeId"),
      "derp_cluster_id": stringField(ticket, "derpClusterId"),
      "country_code": stringField(ticket, "countryCode"),
      "city_code": stringField(ticket, "cityCode"),
      "allowed_derp_node_ids": ticket["allowedDerpNodeIds"] as? [String] ?? [],
      "relay_url": stringField(ticket, "relayUrl"),
      "expires_at": stringField(ticket, "expiresAt"),
      "session_key": stringField(ticket, "sessionKey"),
      "signature": stringField(ticket, "signature")
    ]
  }

  private func stringField(_ object: [String: Any], _ field: String) -> String {
    (object[field] as? String)?.trimmingCharacters(in: .whitespacesAndNewlines) ?? ""
  }

  private static func endpoint(_ address: String) -> (
    host: Network.NWEndpoint.Host,
    port: Network.NWEndpoint.Port
  )? {
    guard let separator = address.lastIndex(of: ":") else {
      return nil
    }
    let host = String(address[..<separator])
    let port = String(address[address.index(after: separator)...])
    guard !host.isEmpty,
      let portValue = UInt16(port),
      let endpointPort = Network.NWEndpoint.Port(rawValue: portValue)
    else {
      return nil
    }
    return (Network.NWEndpoint.Host(host), endpointPort)
  }

  fileprivate static func normalizeVirtualIp(_ value: String) -> String {
    value
      .trimmingCharacters(in: .whitespacesAndNewlines)
      .split(separator: "/", maxSplits: 1)
      .first
      .map(String.init) ?? ""
  }
}

// 数据帧编解码辅助方法，统一按网络字节序读写整数。
private extension Data {
  mutating func appendUInt16(_ value: UInt16) {
    append(contentsOf: value.bigEndianBytes)
  }

  mutating func appendUInt32(_ value: UInt32) {
    append(contentsOf: value.bigEndianBytes)
  }

  mutating func appendUInt64(_ value: UInt64) {
    append(contentsOf: value.bigEndianBytes)
  }

  func readUInt16(at offset: Int) -> UInt16 {
    UInt16(self[offset]) << 8 | UInt16(self[offset + 1])
  }

  func readUInt32(at offset: Int) -> UInt32 {
    UInt32(self[offset]) << 24
      | UInt32(self[offset + 1]) << 16
      | UInt32(self[offset + 2]) << 8
      | UInt32(self[offset + 3])
  }
}

private extension FixedWidthInteger {
  var bigEndianBytes: [UInt8] {
    withUnsafeBytes(of: self.bigEndian) { Array($0) }
  }
}

// PacketTunnel 启动阶段返回给系统的轻量错误类型。
private struct PacketTunnelError: LocalizedError {
  let message: String

  init(_ message: String) {
    self.message = message
  }

  var errorDescription: String? {
    message
  }
}
