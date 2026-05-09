import NetworkExtension
import Network
import os.log

final class PacketTunnelProvider: NEPacketTunnelProvider {
  private static let logger = OSLog(subsystem: "dev.slan.client.v2", category: "PacketTunnel")

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
    guard let virtualIp = config["virtualIp"] as? String, !virtualIp.isEmpty else {
      completionHandler(PacketTunnelError("missing virtualIp"))
      return
    }

    let networkSettings = NEPacketTunnelNetworkSettings(tunnelRemoteAddress: "10.255.0.1")
    let ipv4 = NEIPv4Settings(addresses: [virtualIp], subnetMasks: ["255.255.255.255"])
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
          "SLAN PacketTunnel started virtualIp=%{public}@ routes=%{public}d relaySessions=%{public}d",
          log: Self.logger,
          type: .info,
          virtualIp,
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
          tunnelStats.relayFramesSent += 1
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
  private var seq: UInt64 = 0
  private var configHash: UInt64 = 0

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
  }

  func start() {
    peers.forEach { $0.start() }
  }

  func stop() {
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
    peer.send(frame)
    return true
  }

  private static func encodeFrame(seq: UInt64, configHash: UInt64, payload: Data) -> Data? {
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

  static func decodeFrame(_ frame: Data) -> Data? {
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
          self.packetFlow.writePackets([packet], withProtocols: [NSNumber(value: AF_INET)])
          self.packetsWritten += 1
        }
      }
      if self.ready {
        self.receive()
      }
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

  private static func normalizeVirtualIp(_ value: String) -> String {
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
