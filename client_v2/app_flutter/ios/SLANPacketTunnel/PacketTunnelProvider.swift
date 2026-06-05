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
  fileprivate static func parseCidr(_ cidr: String) -> (
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

  fileprivate static func ipv4Value(_ address: String) -> UInt32? {
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

private struct AclPeer {
  let peerNodeId: String
  let peerVirtualIps: Set<String>
}

private enum AclDirection {
  case ingress
  case egress
}

private struct AclPolicy {
  let networkId: String
  let defaultPolicy: String
  let rules: [AclRule]

  static func parse(_ value: Any?) -> [AclPolicy] {
    guard let policies = value as? [[String: Any]] else {
      return []
    }
    return policies.map { policy in
      let rules = (policy["rules"] as? [[String: Any]] ?? [])
        .map(AclRule.parse)
        .sorted {
          if $0.priority == $1.priority {
            return $0.ruleId < $1.ruleId
          }
          return $0.priority < $1.priority
        }
      return AclPolicy(
        networkId: string(policy["networkId"]),
        defaultPolicy: string(policy["defaultPolicy"]),
        rules: rules
      )
    }
  }

  static func allows(
    _ packet: Data,
    policies: [AclPolicy],
    direction: AclDirection,
    peer: AclPeer?
  ) -> Bool {
    guard Ipv4Packet.sourceAddress(packet) != nil, Ipv4Packet.destinationAddress(packet) != nil else {
      return true
    }
    var hasEnabledRule = false
    for policy in policies {
      for rule in policy.rules where rule.enabled {
        hasEnabledRule = true
        if !rule.matches(packet: packet, direction: direction, networkId: policy.networkId, peer: peer) {
          continue
        }
        return rule.action.caseInsensitiveCompare("allow") == .orderedSame
      }
    }
    if !hasEnabledRule {
      return true
    }
    return policies.contains {
      $0.defaultPolicy.trimmingCharacters(in: .whitespacesAndNewlines)
        .caseInsensitiveCompare("allow") == .orderedSame
    }
  }

  fileprivate static func string(_ value: Any?) -> String {
    (value as? String)?.trimmingCharacters(in: .whitespacesAndNewlines) ?? ""
  }

  fileprivate static func int(_ value: Any?) -> Int {
    switch value {
    case let value as Int:
      return value
    case let value as NSNumber:
      return value.intValue
    case let value as String:
      return Int(value.trimmingCharacters(in: .whitespacesAndNewlines)) ?? 0
    default:
      return 0
    }
  }
}

private struct AclRule {
  let ruleId: String
  let direction: String
  let priority: Int
  let action: String
  let protocolName: String
  let portFrom: Int
  let portTo: Int
  let peerType: String
  let peerValue: String
  let enabled: Bool
  let resolvedPeerNodeId: String
  let resolvedPeerVirtualIps: Set<String>

  static func parse(_ value: [String: Any]) -> AclRule {
    AclRule(
      ruleId: AclPolicy.string(value["ruleId"]),
      direction: AclPolicy.string(value["direction"]),
      priority: AclPolicy.int(value["priority"]),
      action: AclPolicy.string(value["action"]),
      protocolName: AclPolicy.string(value["protocol"]),
      portFrom: AclPolicy.int(value["portFrom"]),
      portTo: AclPolicy.int(value["portTo"]),
      peerType: AclPolicy.string(value["peerType"]),
      peerValue: AclPolicy.string(value["peerValue"]),
      enabled: (value["enabled"] as? Bool) ?? false,
      resolvedPeerNodeId: AclPolicy.string(value["resolvedPeerNodeId"]),
      resolvedPeerVirtualIps: Set(
        (value["resolvedPeerVirtualIps"] as? [String] ?? [])
          .map(RelayPeerRuntime.normalizeVirtualIp)
          .filter { !$0.isEmpty }
      )
    )
  }

  func matches(
    packet: Data,
    direction packetDirection: AclDirection,
    networkId: String,
    peer: AclPeer?
  ) -> Bool {
    directionMatches(packetDirection)
      && protocolMatches(packet)
      && portMatches(packet)
      && peerMatches(packet: packet, packetDirection: packetDirection, networkId: networkId, peer: peer)
  }

  private func directionMatches(_ packetDirection: AclDirection) -> Bool {
    let value = direction.lowercased()
    if value.isEmpty || value == "all" || value == "any" {
      return true
    }
    if packetDirection == .egress {
      return value == "egress" || value == "out" || value == "outbound"
    }
    return value == "ingress" || value == "in" || value == "inbound"
  }

  private func protocolMatches(_ packet: Data) -> Bool {
    let value = protocolName.lowercased()
    if value.isEmpty || value == "all" || value == "any" {
      return true
    }
    guard let proto = Ipv4Packet.ipv4Protocol(packet) else {
      return true
    }
    return (value == "icmp" && proto == 1)
      || (value == "tcp" && proto == 6)
      || (value == "udp" && proto == 17)
      || value == String(proto)
  }

  private func portMatches(_ packet: Data) -> Bool {
    let from = max(0, portFrom)
    let to = max(0, portTo)
    if from == 0 && to == 0 {
      return true
    }
    guard let port = Ipv4Packet.destinationPort(packet) else {
      return false
    }
    let lower = min(from == 0 ? to : from, to == 0 ? from : to)
    let upper = max(from == 0 ? to : from, to == 0 ? from : to)
    return Int(port) >= lower && Int(port) <= upper
  }

  private func peerMatches(packet: Data, packetDirection: AclDirection, networkId: String, peer: AclPeer?) -> Bool {
    let type = peerType.lowercased()
    if type.isEmpty || type == "all" || type == "any" {
      return peerValue.isEmpty
        || peerValue.caseInsensitiveCompare("all") == .orderedSame
        || peerValue.caseInsensitiveCompare("any") == .orderedSame
        || peerValue == "*"
    }
    if type == "network" || type == "workspace" {
      return peerValue.isEmpty
        || peerValue.caseInsensitiveCompare("self") == .orderedSame
        || peerValue.caseInsensitiveCompare("all") == .orderedSame
        || peerValue == networkId
    }
    let subjectIps = aclSubjectIps(ruleDirection: direction, packetDirection: packetDirection, packet: packet)
    if ["ip", "cidr", "subnet"].contains(type) {
      return subjectIps.contains { ipMatches(peerValue, $0) }
    }
    if type == "device" {
      if let peer = peer {
        let expectedNodeId = "node-\(peerValue)"
        let peerIsSubject = peer.peerVirtualIps.contains { peerIp in
          subjectIps.contains { RelayPeerRuntime.normalizeVirtualIp(peerIp) == $0 }
        }
        if peerIsSubject && (peer.peerNodeId == peerValue || peer.peerNodeId == expectedNodeId) {
          return true
        }
        if peerIsSubject && !resolvedPeerNodeId.isEmpty && peer.peerNodeId == resolvedPeerNodeId {
          return true
        }
      }
      return subjectIps.contains { resolvedPeerVirtualIps.contains($0) }
    }
    if type == "domain" || type == "dns" {
      return subjectIps.contains { resolvedPeerVirtualIps.contains($0) }
    }
    return false
  }

  private func aclSubjectIps(ruleDirection: String, packetDirection: AclDirection, packet: Data) -> [String] {
    let value = ruleDirection.lowercased()
    let source = Ipv4Packet.sourceAddress(packet)
    let destination = Ipv4Packet.destinationAddress(packet)
    if packetDirection == .egress && ["egress", "out", "outbound"].contains(value) {
      return destination.map { [$0] } ?? []
    }
    if packetDirection == .ingress && ["ingress", "in", "inbound"].contains(value) {
      return destination.map { [$0] } ?? []
    }
    if value.isEmpty || value == "all" || value == "any" {
      return [source, destination].compactMap { $0 }
    }
    return destination.map { [$0] } ?? []
  }

  private func ipMatches(_ pattern: String, _ ip: String) -> Bool {
    let value = pattern.trimmingCharacters(in: .whitespacesAndNewlines)
    if value.isEmpty || value.caseInsensitiveCompare("all") == .orderedSame || value == "*" {
      return true
    }
    if value.contains("/") {
      guard
        let parsed = PacketTunnelProvider.parseCidr(value),
        let ipValue = PacketTunnelProvider.ipv4Value(ip)
      else {
        return false
      }
      return (ipValue & PacketTunnelProviderMask.value(parsed.prefix)) == parsed.network
    }
    return RelayPeerRuntime.normalizeVirtualIp(value) == RelayPeerRuntime.normalizeVirtualIp(ip)
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
    normalized.replaceSubrange(10..<12, with: checksum(normalized.subdata(in: 0..<ihl)).bigEndianBytes)
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
    reply.replaceSubrange(ihl + 2..<ihl + 4, with: checksum(reply.subdata(in: ihl..<totalLen)).bigEndianBytes)
    reply.replaceSubrange(10..<12, with: checksum(reply.subdata(in: 0..<ihl)).bigEndianBytes)
    return reply
  }

  static func relaySendAttemptCount(_ packet: Data) -> Int {
    if ipv4Protocol(packet) == 17 {
      return 3
    }
    guard let flags = tcpFlags(packet) else {
      return 1
    }
    if flags & 0x12 == 0x02 {
      return 4
    }
    if flags & 0x0b != 0 || (tcpPayloadLength(packet) ?? 0) > 0 {
      return 2
    }
    return 1
  }

  static func shouldHedgeDirectPacketToRelay(_ packet: Data) -> Bool {
    if ipv4Protocol(packet) == 17 {
      return true
    }
    guard let flags = tcpFlags(packet) else {
      return false
    }
    return flags & 0x12 == 0x02 || flags & 0x0b != 0 || (tcpPayloadLength(packet) ?? 0) > 0
  }

  static func relaySendAttemptDelay(_ packet: Data) -> TimeInterval {
    guard let flags = tcpFlags(packet) else {
      return 0.002
    }
    return flags & 0x12 == 0x02 ? 0.03 : 0.002
  }

  static func sourceAddress(_ packet: Data) -> String? {
    guard packet.count >= 20, packet[0] >> 4 == 4 else {
      return nil
    }
    return ipv4String(packet, offset: 12)
  }

  static func destinationAddress(_ packet: Data) -> String? {
    guard packet.count >= 20, packet[0] >> 4 == 4 else {
      return nil
    }
    return ipv4String(packet, offset: 16)
  }

  static func ipv4Protocol(_ packet: Data) -> UInt8? {
    guard packet.count >= 20, packet[0] >> 4 == 4 else {
      return nil
    }
    return packet[9]
  }

  static func destinationPort(_ packet: Data) -> UInt16? {
    guard packet.count >= 20, packet[0] >> 4 == 4 else {
      return nil
    }
    let ihl = Int(packet[0] & 0x0f) * 4
    guard ihl >= 20, packet.count >= ihl + 4 else {
      return nil
    }
    if packet.readUInt16(at: 6) & 0x1fff != 0 {
      return nil
    }
    guard packet[9] == 6 || packet[9] == 17 else {
      return nil
    }
    return packet.readUInt16(at: ihl + 2)
  }

  private static func tcpFlags(_ packet: Data) -> UInt8? {
    guard packet.count >= 20, packet[0] >> 4 == 4, packet[9] == 6 else {
      return nil
    }
    let ihl = Int(packet[0] & 0x0f) * 4
    guard ihl >= 20, packet.count >= ihl + 14 else {
      return nil
    }
    return packet[ihl + 13]
  }

  private static func tcpPayloadLength(_ packet: Data) -> Int? {
    guard packet.count >= 20, packet[0] >> 4 == 4, packet[9] == 6 else {
      return nil
    }
    let ihl = Int(packet[0] & 0x0f) * 4
    guard ihl >= 20, packet.count >= ihl + 20 else {
      return nil
    }
    let totalLen = Int(packet.readUInt16(at: 2))
    guard totalLen >= ihl + 20, totalLen <= packet.count else {
      return nil
    }
    let dataOffset = Int(packet[ihl + 12] >> 4) * 4
    guard dataOffset >= 20, totalLen >= ihl + dataOffset else {
      return nil
    }
    return totalLen - ihl - dataOffset
  }

  private static func normalizeTcpChecksum(_ packet: inout Data, ihl: Int, totalLen: Int) {
    let tcpLen = totalLen - ihl
    guard tcpLen >= 20 else {
      return
    }
    packet[ihl + 16] = 0
    packet[ihl + 17] = 0
    let sum = transportChecksum(packet, offset: ihl, length: tcpLen, proto: 6)
    packet.replaceSubrange(ihl + 16..<ihl + 18, with: sum.bigEndianBytes)
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
    packet.replaceSubrange(ihl + 6..<ihl + 8, with: (sum == 0 ? UInt16.max : sum).bigEndianBytes)
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
  private let aclPolicies: [AclPolicy]
  private var peers: [RelayPeerRuntime] = []
  private var derpPeers: [DerpPeerRuntime] = []
  private var directUdpRuntime: DirectUdpRuntime?
  private var seq: UInt64 = 0
  private var configHash: UInt64 = 0
  private(set) var lastSendPath = ""

  // 服务端下发的可用中继会话数量。
  var sessionCount: Int {
    peers.count + derpPeers.count
  }

  var attachedSessionCount: Int {
    peers.filter { $0.attached }.count + derpPeers.filter { $0.attached }.count
  }

  var attachFailureCount: Int {
    peers.filter { !$0.attachError.isEmpty }.count
      + derpPeers.filter { !$0.attachError.isEmpty }.count
  }

  var lastAttachError: String {
    derpPeers.reversed().first { !$0.attachError.isEmpty }?.attachError
      ?? peers.reversed().first { !$0.attachError.isEmpty }?.attachError ?? ""
  }

  var framesReceived: Int {
    peers.reduce(0) { $0 + $1.framesReceived } + derpPeers.reduce(0) { $0 + $1.framesReceived }
  }

  var packetsWritten: Int {
    peers.reduce(0) { $0 + $1.packetsWritten } + derpPeers.reduce(0) { $0 + $1.packetsWritten }
  }

  var detachSentCount: Int {
    peers.filter { $0.detachSent }.count + derpPeers.filter { $0.detachSent }.count
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
    guard !localNodeId.isEmpty,
      transport == "udp" || transport == "relay_udp" || transport == "derp_tcp_tls_443"
    else {
      return nil
    }
    self.localNodeId = localNodeId
    self.localVirtualIp = RelayPeerRuntime.normalizeVirtualIp(localVirtualIp)
    self.relayAddress = relayAddress
    self.maxFramePayload = max(512, min(1400, config["maxFramePayload"] as? Int ?? 1200))
    self.packetFlow = packetFlow
    self.aclPolicies = AclPolicy.parse(config["aclPolicies"])
    if let data = try? JSONSerialization.data(withJSONObject: config),
      let json = String(data: data, encoding: .utf8)
    {
      self.configHash = Self.stableHash64(json)
    }
    let sessions = config["sessions"] as? [[String: Any]] ?? []
    self.peers = sessions.compactMap { session in
      guard Self.relayPathKind(session, fallbackRelayAddress: relayAddress) == "udp" else {
        return nil
      }
      return RelayPeerRuntime(
        session: session,
        relayAddress: relayAddress,
        localNodeId: localNodeId,
        localVirtualIp: self.localVirtualIp,
        configHash: configHash,
        packetFlow: packetFlow,
        aclPolicies: aclPolicies
      )
    }
    self.derpPeers = sessions.compactMap { session in
      guard Self.relayPathKind(session, fallbackRelayAddress: relayAddress) == "derp" else {
        return nil
      }
      return DerpPeerRuntime(
        session: session,
        relayAddress: relayAddress,
        localNodeId: localNodeId,
        localVirtualIp: self.localVirtualIp,
        configHash: configHash,
        packetFlow: packetFlow,
        aclPolicies: aclPolicies
      )
    }
    if peers.isEmpty && derpPeers.isEmpty {
      return nil
    }
    self.directUdpRuntime = DirectUdpRuntime(
      config: config,
      localNodeId: localNodeId,
      localVirtualIp: self.localVirtualIp,
      maxFramePayload: maxFramePayload,
      configHash: configHash,
      packetFlow: packetFlow,
      aclPolicies: aclPolicies
    )
  }

  // 启动所有 Relay UDP 会话和 Direct UDP 探测。
  func start() {
    peers.forEach { $0.start() }
    derpPeers.forEach { $0.start() }
    directUdpRuntime?.start()
  }

  // 停止所有底层 UDP 连接，并清理会话状态。
  func stop() {
    directUdpRuntime?.stop()
    directUdpRuntime = nil
    peers.forEach { $0.stop() }
    derpPeers.forEach { $0.stop() }
    peers.removeAll()
    derpPeers.removeAll()
  }

  // 按目的虚拟 IP 选择 peer，优先直链发送，直链不可用时发送到中继节点。
  func send(packet: Data, destination: String) -> Bool {
    guard packet.count <= maxFramePayload else {
      return false
    }
    let relayPeer = peers.first(where: { $0.matches(destination) })
    let derpPeer = derpPeers.first(where: { $0.matches(destination) })
    guard relayPeer != nil || derpPeer != nil else {
      return false
    }
    let aclPeer = relayPeer?.aclPeer ?? derpPeer?.aclPeer
    guard AclPolicy.allows(packet, policies: aclPolicies, direction: .egress, peer: aclPeer)
    else {
      return false
    }
    seq &+= 1
    guard let frame = Self.encodeFrame(seq: seq, configHash: configHash, payload: packet) else {
      return false
    }
    if directUdpRuntime?.send(frame: frame, destination: destination) == true {
      lastSendPath = "direct_udp"
      if let peer = relayPeer, Ipv4Packet.shouldHedgeDirectPacketToRelay(packet) {
        sendRelayFrame(peer, frame: frame, packet: packet)
      }
      return true
    }
    if let peer = relayPeer {
      sendRelayFrame(peer, frame: frame, packet: packet)
      lastSendPath = "relay_udp"
      return true
    }
    if let peer = derpPeer {
      sendDerpFrame(peer, frame: frame, packet: packet)
      lastSendPath = "derp_tcp_tls_443"
      return true
    }
    return false
  }

  private static func relayPathKind(_ session: [String: Any], fallbackRelayAddress: String) -> String? {
    let ticket = session["ticket"] as? [String: Any] ?? [:]
    let ticketRelayUrl = (ticket["relayUrl"] as? String ?? "")
      .trimmingCharacters(in: .whitespacesAndNewlines)
    let relayUrl = (ticketRelayUrl.isEmpty ? fallbackRelayAddress : ticketRelayUrl)
      .trimmingCharacters(in: .whitespacesAndNewlines)
      .lowercased()
    if relayUrl.hasPrefix("udp://") || relayUrl.hasPrefix("relay+udp://") {
      return "udp"
    }
    if relayUrl.hasPrefix("derp://")
      || relayUrl.hasPrefix("derp+tcp+tls://")
      || relayUrl.hasPrefix("derp_tcp_tls_443://")
    {
      return "derp"
    }
    return nil
  }

  private func sendDerpFrame(_ peer: DerpPeerRuntime, frame: Data, packet: Data) {
    let attempts = Ipv4Packet.relaySendAttemptCount(packet)
    let delay = Ipv4Packet.relaySendAttemptDelay(packet)
    for attempt in 0..<attempts {
      if attempt == 0 {
        _ = peer.send(frame)
      } else {
        DispatchQueue.global(qos: .utility).asyncAfter(deadline: .now() + delay * Double(attempt)) {
          _ = peer.send(frame)
        }
      }
    }
  }

  private func sendRelayFrame(_ peer: RelayPeerRuntime, frame: Data, packet: Data) {
    let attempts = Ipv4Packet.relaySendAttemptCount(packet)
    let delay = Ipv4Packet.relaySendAttemptDelay(packet)
    for attempt in 0..<attempts {
      if attempt == 0 {
        _ = peer.send(frame)
      } else {
        DispatchQueue.global(qos: .utility).asyncAfter(deadline: .now() + delay * Double(attempt)) {
          _ = peer.send(frame)
        }
      }
    }
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
  private let aclPolicies: [AclPolicy]
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
    packetFlow: NEPacketTunnelFlow,
    aclPolicies: [AclPolicy]
  ) {
    let peerPaths = config["peerPaths"] as? [[String: Any]] ?? []
    let peers = peerPaths.compactMap {
      DirectUdpPeerRuntime(
        peerPath: $0,
        localNodeId: localNodeId,
        localVirtualIp: localVirtualIp,
        configHash: configHash,
        packetFlow: packetFlow,
        aclPolicies: aclPolicies
      )
    }
    if peers.isEmpty {
      return nil
    }
    self.localNodeId = localNodeId
    self.maxFramePayload = maxFramePayload
    self.configHash = configHash
    self.packetFlow = packetFlow
    self.aclPolicies = aclPolicies
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
  private let aclPolicies: [AclPolicy]
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
  var aclPeer: AclPeer {
    AclPeer(peerNodeId: peerNodeId, peerVirtualIps: peerVirtualIps)
  }

  // 解析服务端 peerPath，选择第一个 Direct UDP/LAN UDP/IPv6 UDP 候选作为远端端点。
  init?(
    peerPath: [String: Any],
    localNodeId: String,
    localVirtualIp: String,
    configHash: UInt64,
    packetFlow: NEPacketTunnelFlow,
    aclPolicies: [AclPolicy]
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
    self.aclPolicies = aclPolicies
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
          guard AclPolicy.allows(packet, policies: self.aclPolicies, direction: .ingress, peer: self.aclPeer) else {
            if self.running {
              self.receive()
            }
            return
          }
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

// 单个 DERP/TCP 会话，负责 connect/disconnect 票据握手和 TCP relay 数据帧收发。
private final class DerpPeerRuntime {
  private let sessionId: String
  private let peerNodeId: String
  private let peerVirtualIps: Set<String>
  private let localNodeId: String
  private let localVirtualIp: String
  private let configHash: UInt64
  private let ticket: [String: Any]
  private let endpoint: (host: Network.NWEndpoint.Host, port: Network.NWEndpoint.Port)
  private let packetFlow: NEPacketTunnelFlow
  private let aclPolicies: [AclPolicy]
  private var connection: NWConnection?
  private var readBuffer = Data()
  private var ready = false
  private var serverSessionId = ""
  private(set) var attached = false
  private(set) var attachError = ""
  private(set) var framesReceived = 0
  private(set) var packetsWritten = 0
  private(set) var detachSent = false
  private var seq: UInt64 = 0
  var aclPeer: AclPeer {
    AclPeer(peerNodeId: peerNodeId, peerVirtualIps: peerVirtualIps)
  }

  init?(
    session: [String: Any],
    relayAddress: String,
    localNodeId: String,
    localVirtualIp: String,
    configHash: UInt64,
    packetFlow: NEPacketTunnelFlow,
    aclPolicies: [AclPolicy]
  ) {
    let sessionId = (session["sessionId"] as? String ?? "")
      .trimmingCharacters(in: .whitespacesAndNewlines)
    let peerNodeId = (session["peerNodeId"] as? String ?? "")
      .trimmingCharacters(in: .whitespacesAndNewlines)
    let ticket = session["ticket"] as? [String: Any] ?? [:]
    let peerVirtualIps = (session["peerVirtualIps"] as? [String] ?? [])
      .map(RelayPeerRuntime.normalizeVirtualIp)
      .filter { !$0.isEmpty }
    let ticketRelayUrl = (ticket["relayUrl"] as? String ?? "")
      .trimmingCharacters(in: .whitespacesAndNewlines)
    let relayUrl = ticketRelayUrl.isEmpty ? relayAddress : ticketRelayUrl
    guard !sessionId.isEmpty,
      !peerNodeId.isEmpty,
      !peerVirtualIps.isEmpty,
      !ticket.isEmpty,
      let endpoint = Self.endpoint(relayUrl)
    else {
      return nil
    }
    self.sessionId = sessionId
    self.peerNodeId = peerNodeId
    self.peerVirtualIps = Set(peerVirtualIps)
    self.localNodeId = localNodeId
    self.localVirtualIp = RelayPeerRuntime.normalizeVirtualIp(localVirtualIp)
    self.configHash = configHash
    self.ticket = ticket
    self.endpoint = endpoint
    self.packetFlow = packetFlow
    self.aclPolicies = aclPolicies
  }

  func start() {
    guard connection == nil else {
      return
    }
    let connection = NWConnection(host: endpoint.host, port: endpoint.port, using: .tcp)
    connection.stateUpdateHandler = { [weak self] (state: NWConnection.State) in
      guard let self = self else {
        return
      }
      if case .ready = state {
        self.ready = true
        self.connect()
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

  func stop() {
    disconnect()
    ready = false
    connection?.cancel()
    connection = nil
  }

  func matches(_ destination: String) -> Bool {
    peerVirtualIps.contains(RelayPeerRuntime.normalizeVirtualIp(destination))
  }

  @discardableResult
  func send(_ frame: Data) -> Bool {
    guard ready && attached else {
      return false
    }
    let payload: [String: Any] = [
      "kind": "send",
      "sessionId": serverSessionId.isEmpty ? sessionId : serverSessionId,
      "targetPeerId": peerNodeId,
      "payload": frame.base64EncodedString()
    ]
    sendJsonLine(payload)
    return true
  }

  private func connect() {
    let payload: [String: Any] = [
      "kind": "connect",
      "peerId": localNodeId,
      "nodeId": derpTicketNodeId(),
      "regionId": derpTicketRegionId(),
      "ticket": derpTicketWire(ticket)
    ]
    sendJsonLine(payload)
  }

  private func disconnect() {
    guard ready && attached else {
      return
    }
    let payload: [String: Any] = [
      "kind": "disconnect",
      "sessionId": serverSessionId.isEmpty ? sessionId : serverSessionId,
      "peerId": localNodeId
    ]
    sendJsonLine(payload)
    detachSent = true
    attached = false
  }

  private func sendJsonLine(_ payload: [String: Any]) {
    guard var data = try? JSONSerialization.data(withJSONObject: payload) else {
      return
    }
    data.append(0x0a)
    connection?.send(content: data, completion: .contentProcessed { _ in })
  }

  private func receive() {
    connection?.receive(minimumIncompleteLength: 1, maximumLength: 4096) {
      [weak self] data, _, isComplete, error in
      guard let self = self else {
        return
      }
      if let data = data, !data.isEmpty {
        self.readBuffer.append(data)
        self.consumeBufferedLines()
      }
      if isComplete || error != nil {
        self.ready = false
        return
      }
      if self.ready {
        self.receive()
      }
    }
  }

  private func consumeBufferedLines() {
    while let newline = readBuffer.firstIndex(of: 0x0a) {
      let line = readBuffer.subdata(in: 0..<newline)
      readBuffer.removeSubrange(0...newline)
      consumeLine(line)
    }
  }

  private func consumeLine(_ line: Data) {
    guard let value = try? JSONSerialization.jsonObject(with: line) as? [String: Any],
      let kind = value["kind"] as? String
    else {
      return
    }
    if kind == "connected" {
      attached = true
      serverSessionId = (value["sessionId"] as? String) ?? sessionId
      attachError = ""
      return
    }
    if kind == "error" {
      attachError = ((value["error"] as? [String: Any])?["message"] as? String) ?? "DERP error"
      return
    }
    guard kind == "recv",
      let payload = value["payload"] as? String,
      let frame = Data(base64Encoded: payload),
      let packet = RelayRuntime.decodeFrame(frame)
    else {
      return
    }
    framesReceived += 1
    guard AclPolicy.allows(packet, policies: aclPolicies, direction: .ingress, peer: aclPeer) else {
      return
    }
    if let reply = Ipv4Packet.icmpEchoReply(for: packet, localVirtualIp: localVirtualIp) {
      seq &+= 1
      if let frame = RelayRuntime.encodeFrame(seq: seq, configHash: configHash, payload: reply) {
        _ = send(frame)
      }
    } else {
      writePacketToFlow(Ipv4Packet.normalizeTransportChecksums(packet))
    }
  }

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

  private func derpTicketNodeId() -> String {
    let allowed = ticket["allowedDerpNodeIds"] as? [String] ?? []
    return allowed
      .map { $0.trimmingCharacters(in: .whitespacesAndNewlines) }
      .first { !$0.isEmpty } ?? "derp"
  }

  private func derpTicketRegionId() -> String {
    let value = (ticket["derpClusterId"] as? String ?? "")
      .trimmingCharacters(in: .whitespacesAndNewlines)
    return value.isEmpty ? "default" : value
  }

  private func derpTicketWire(_ ticket: [String: Any]) -> [String: Any] {
    [
      "ticketId": stringField(ticket, "ticketId"),
      "peerId": localNodeId,
      "networkId": stringField(ticket, "networkId"),
      "path": "derp_tcp_tls_443",
      "regionId": derpTicketRegionId(),
      "nodeId": derpTicketNodeId(),
      "sessionId": stringField(ticket, "sessionId"),
      "srcNodeId": stringField(ticket, "srcNodeId"),
      "dstNodeId": stringField(ticket, "dstNodeId"),
      "relayUrl": stringField(ticket, "relayUrl"),
      "sessionKey": stringField(ticket, "sessionKey"),
      "allowedDerpNodeIds": ticket["allowedDerpNodeIds"] as? [String] ?? [],
      "expiresAt": stringField(ticket, "expiresAt"),
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
    let trimmed = address.trimmingCharacters(in: .whitespacesAndNewlines)
    let normalized: String
    if trimmed.hasPrefix("derp://") {
      normalized = String(trimmed.dropFirst("derp://".count))
    } else if trimmed.hasPrefix("derp+tcp+tls://") {
      normalized = String(trimmed.dropFirst("derp+tcp+tls://".count))
    } else if trimmed.hasPrefix("derp_tcp_tls_443://") {
      normalized = String(trimmed.dropFirst("derp_tcp_tls_443://".count))
    } else {
      return nil
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
  private let peerNodeId: String
  private let peerVirtualIps: Set<String>
  private let relayAddress: String
  private let localNodeId: String
  private let localVirtualIp: String
  private let configHash: UInt64
  private let ticket: [String: Any]
  private let packetFlow: NEPacketTunnelFlow
  private let aclPolicies: [AclPolicy]
  private var connection: NWConnection?
  private var ready = false
  private(set) var attached = false
  private(set) var attachError = ""
  private(set) var framesReceived = 0
  private(set) var packetsWritten = 0
  private(set) var detachSent = false
  private var attachAttempts = 0
  private var seq: UInt64 = 0
  var aclPeer: AclPeer {
    AclPeer(peerNodeId: peerNodeId, peerVirtualIps: peerVirtualIps)
  }

  // 从服务端 session 配置解析 relay ticket 与远端虚拟 IP 集合。
  init?(
    session: [String: Any],
    relayAddress: String,
    localNodeId: String,
    localVirtualIp: String,
    configHash: UInt64,
    packetFlow: NEPacketTunnelFlow,
    aclPolicies: [AclPolicy]
  ) {
    let sessionId = (session["sessionId"] as? String ?? "")
      .trimmingCharacters(in: .whitespacesAndNewlines)
    let peerNodeId = (session["peerNodeId"] as? String ?? "")
      .trimmingCharacters(in: .whitespacesAndNewlines)
    let ticket = session["ticket"] as? [String: Any] ?? [:]
    let sessionRelayAddress = (ticket["relayUrl"] as? String ?? "")
      .trimmingCharacters(in: .whitespacesAndNewlines)
    let resolvedRelayAddress = Self.udpRelayAddress(sessionRelayAddress)
      ?? Self.udpRelayAddress(relayAddress)
    let peerVirtualIps = (session["peerVirtualIps"] as? [String] ?? [])
      .map(Self.normalizeVirtualIp)
      .filter { !$0.isEmpty }
    guard !sessionId.isEmpty,
      !peerNodeId.isEmpty,
      !peerVirtualIps.isEmpty,
      !ticket.isEmpty,
      resolvedRelayAddress != nil
    else {
      return nil
    }
    self.sessionId = sessionId
    self.peerNodeId = peerNodeId
    self.peerVirtualIps = Set(peerVirtualIps)
    self.relayAddress = resolvedRelayAddress ?? relayAddress
    self.localNodeId = localNodeId
    self.localVirtualIp = RelayPeerRuntime.normalizeVirtualIp(localVirtualIp)
    self.configHash = configHash
    self.ticket = ticket
    self.packetFlow = packetFlow
    self.aclPolicies = aclPolicies
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
  @discardableResult
  func send(_ frame: Data) -> Bool {
    guard ready && attached else {
      return false
    }
    connection?.send(content: frame, completion: .contentProcessed { _ in })
    return true
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
          guard AclPolicy.allows(packet, policies: self.aclPolicies, direction: .ingress, peer: self.aclPeer) else {
            if self.ready {
              self.receive()
            }
            return
          }
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

  private static func udpRelayAddress(_ address: String) -> String? {
    let trimmed = address.trimmingCharacters(in: .whitespacesAndNewlines)
    if trimmed.isEmpty {
      return nil
    }
    if trimmed.hasPrefix("udp://") {
      return String(trimmed.dropFirst("udp://".count))
    }
    if trimmed.hasPrefix("relay+udp://") {
      return String(trimmed.dropFirst("relay+udp://".count))
    }
    if trimmed.contains("://") {
      return nil
    }
    return trimmed
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
