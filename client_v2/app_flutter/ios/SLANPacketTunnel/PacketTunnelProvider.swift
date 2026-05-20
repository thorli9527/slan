import NetworkExtension
import Network
import os.log

final class PacketTunnelProvider: NEPacketTunnelProvider {
  private static let logger = OSLog(subsystem: "dev.slan.client.v2", category: "PacketTunnel")
  private static let hostInterfacePrefixLen = 32

  private var readingPackets = false
  private var routeTable: [RouteEntry] = []
  private var relayRuntime: RelayRuntime?
  private var tunnelStats = PacketTunnelStats()

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
        self.relayRuntime = RelayRuntime(config: config["relayDataPlane"], packetFlow: self.packetFlow)
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

      tunnelStats.lastDestination = destination.address
      if let route = routeTable.first(where: { $0.contains(destination.value) }) {
        tunnelStats.routedPackets += 1
        tunnelStats.lastRoute = route.cidr
        tunnelStats.lastRoutedAtMs = Self.nowMs()
        if let relayRuntime = relayRuntime,
          relayRuntime.send(packet: packet, destination: destination.address)
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

  private func persistStats() {
    tunnelStats.updatedAtMs = Self.nowMs()
    SLANIosSharedStore.writePacketTunnelStats(tunnelStats.dictionary)
  }

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

private struct RouteEntry {
  let cidr: String
  let destination: String
  let mask: String
  let network: UInt32
  let prefix: Int

  func contains(_ address: UInt32) -> Bool {
    let mask = PacketTunnelProviderMask.value(prefix)
    return (address & mask) == network
  }
}

private enum PacketTunnelProviderMask {
  static func value(_ prefix: Int) -> UInt32 {
    if prefix == 0 {
      return 0
    }
    return UInt32.max << UInt32(32 - prefix)
  }
}

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

private final class RelayRuntime {
  private let localNodeId: String
  private let relayAddress: String
  private let maxFramePayload: Int
  private let packetFlow: NEPacketTunnelFlow
  private var peers: [RelayPeerRuntime] = []
  private var directUdpRuntime: DirectUdpRuntime?
  private var seq: UInt64 = 0
  private var configHash: UInt64 = 0
  private(set) var lastSendPath = ""

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

  init?(config: Any?, packetFlow: NEPacketTunnelFlow) {
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
        packetFlow: packetFlow
      )
    }
    if peers.isEmpty {
      return nil
    }
    self.directUdpRuntime = DirectUdpRuntime(
      config: config,
      localNodeId: localNodeId,
      maxFramePayload: maxFramePayload,
      configHash: configHash,
      packetFlow: packetFlow
    )
  }

  func start() {
    peers.forEach { $0.start() }
    directUdpRuntime?.start()
  }

  func stop() {
    directUdpRuntime?.stop()
    directUdpRuntime = nil
    peers.forEach { $0.stop() }
    peers.removeAll()
  }

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

  private static func stableHash64(_ value: String) -> UInt64 {
    var hash: UInt64 = 0xcbf29ce484222325
    for byte in value.utf8 {
      hash ^= UInt64(byte)
      hash = hash &* 0x100000001b3
    }
    return hash
  }
}

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

  init?(
    config: [String: Any],
    localNodeId: String,
    maxFramePayload: Int,
    configHash: UInt64,
    packetFlow: NEPacketTunnelFlow
  ) {
    let peerPaths = config["peerPaths"] as? [[String: Any]] ?? []
    let peers = peerPaths.compactMap {
      DirectUdpPeerRuntime(
        peerPath: $0,
        localNodeId: localNodeId,
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

  func start() {
    peers.forEach { $0.start() }
  }

  func stop() {
    peers.forEach { $0.stop() }
    peers.removeAll()
  }

  func send(frame: Data, destination: String) -> Bool {
    guard frame.count <= maxFramePayload + 32,
      let peer = peers.first(where: { $0.matches(destination) && $0.ready })
    else {
      return false
    }
    return peer.send(frame)
  }
}

private final class DirectUdpPeerRuntime {
  private let peerNodeId: String
  private let peerVirtualIps: Set<String>
  private let localNodeId: String
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

  init?(
    peerPath: [String: Any],
    localNodeId: String,
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
    self.endpoint = endpoint
    self.packetFlow = packetFlow
  }

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

  func stop() {
    running = false
    ready = false
    connection?.cancel()
    connection = nil
  }

  func matches(_ destination: String) -> Bool {
    peerVirtualIps.contains(RelayPeerRuntime.normalizeVirtualIp(destination))
  }

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
          self.writePacketToFlow(packet)
        }
      }
      if self.running {
        self.receive()
      }
    }
  }

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

private final class RelayPeerRuntime {
  private static let maxAttachAttempts = 3

  private let sessionId: String
  private let peerVirtualIps: Set<String>
  private let relayAddress: String
  private let localNodeId: String
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

  init?(
    session: [String: Any],
    relayAddress: String,
    localNodeId: String,
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
    self.ticket = ticket
    self.packetFlow = packetFlow
  }

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

  func stop() {
    detach()
    ready = false
    connection?.cancel()
    connection = nil
  }

  func matches(_ destination: String) -> Bool {
    peerVirtualIps.contains(Self.normalizeVirtualIp(destination))
  }

  func send(_ frame: Data) {
    guard ready && attached else {
      return
    }
    connection?.send(content: frame, completion: .contentProcessed { _ in })
  }

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
          self.writePacketToFlow(packet)
        }
      }
      if self.ready {
        self.receive()
      }
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

private struct PacketTunnelError: LocalizedError {
  let message: String

  init(_ message: String) {
    self.message = message
  }

  var errorDescription: String? {
    message
  }
}
