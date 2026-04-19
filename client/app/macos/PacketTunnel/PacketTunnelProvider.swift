import Foundation
import NetworkExtension

final class PacketTunnelProvider: NEPacketTunnelProvider {
    private var wireGuardEngine: WireGuardEngine = NoopWireGuardEngine()
    private let wireGuardBackendAdapter: WireGuardBackendAdapting = WireGuardKitBackendAdapter()
    private var runtimeView: PacketTunnelProviderRuntimeView?
    private var currentConfiguration: PacketTunnelProviderConfiguration?
    private var currentSettings: NEPacketTunnelNetworkSettings?
    private var lastAppliedAtMs: Int64?
    private var lastErrorMessage: String?
    private var packetRxCount: Int64 = 0
    private var packetRxBytes: Int64 = 0
    private var packetTxCount: Int64 = 0
    private var packetTxBytes: Int64 = 0
    private var lastPacketAtMs: Int64?
    private var isReadingPackets = false

    override func startTunnel(options: [String: NSObject]?, completionHandler: @escaping (Error?) -> Void) {
        do {
            let configuration = try loadConfiguration()
            let settings = try PacketTunnelProviderSupport.makeNetworkSettings(from: configuration)
            wireGuardEngine = PacketTunnelProviderSupport.makeWireGuardEngine(
                from: configuration,
                backendAdapter: wireGuardBackendAdapter
            )
            currentConfiguration = configuration
            currentSettings = settings
            packetRxCount = 0
            packetRxBytes = 0
            packetTxCount = 0
            packetTxBytes = 0
            lastPacketAtMs = nil
            lastErrorMessage = nil
            refreshRuntimeView(state: "connecting")

            setTunnelNetworkSettings(settings) { [weak self] error in
                guard let self else {
                    completionHandler(error)
                    return
                }
                var completionError: Error? = error
                if let error {
                    self.lastErrorMessage = error.localizedDescription
                    self.isReadingPackets = false
                    self.refreshRuntimeView(state: "failed")
                    NSLog("PacketTunnel failed to apply tunnel settings: %@", error.localizedDescription)
                } else {
                    do {
                        try self.wireGuardEngine.start(configuration: configuration)
                        self.lastAppliedAtMs = Self.currentTimestampMs()
                        self.lastErrorMessage = nil
                        self.isReadingPackets = true
                        self.refreshRuntimeView(state: "connected")
                        self.startPacketReadLoop()
                        NSLog("PacketTunnel started for peer %@", configuration.peerVirtualIp)
                    } catch {
                        self.lastErrorMessage = error.localizedDescription
                        self.isReadingPackets = false
                        self.refreshRuntimeView(state: "failed")
                        completionError = error
                        NSLog("PacketTunnel failed to start WireGuard engine: %@", error.localizedDescription)
                    }
                }
                completionHandler(completionError)
            }
        } catch {
            NSLog("PacketTunnel failed to load configuration: %@", error.localizedDescription)
            completionHandler(error)
        }
    }

    override func stopTunnel(with reason: NEProviderStopReason, completionHandler: @escaping () -> Void) {
        isReadingPackets = false
        wireGuardEngine.stop()
        wireGuardEngine = NoopWireGuardEngine()
        if currentConfiguration == nil {
            currentConfiguration = try? loadConfiguration()
        }
        if currentSettings == nil, let currentConfiguration {
            currentSettings = try? PacketTunnelProviderSupport.makeNetworkSettings(from: currentConfiguration)
        }
        refreshRuntimeView(state: "disconnected")
        NSLog("PacketTunnel stopping, reason=%ld", reason.rawValue)
        completionHandler()
    }

    override func handleAppMessage(_ messageData: Data, completionHandler: ((Data?) -> Void)? = nil) {
        do {
            let object = try JSONSerialization.jsonObject(with: messageData, options: [])
            guard let json = object as? [String: Any],
                  let method = json["method"] as? String else {
                completionHandler?(nil)
                return
            }
            switch method {
            case "runtimeView":
                if runtimeView == nil {
                    if currentConfiguration == nil {
                        currentConfiguration = try? loadConfiguration()
                    }
                    if currentSettings == nil, let currentConfiguration {
                        currentSettings = try? PacketTunnelProviderSupport.makeNetworkSettings(from: currentConfiguration)
                    }
                    refreshRuntimeView(state: "disconnected")
                }
                guard let runtimeView else {
                    completionHandler?(nil)
                    return
                }
                completionHandler?(try JSONSerialization.data(withJSONObject: runtimeView.toJson(), options: []))
            default:
                completionHandler?(nil)
            }
        } catch {
            NSLog("PacketTunnel failed to handle app message: %@", error.localizedDescription)
            completionHandler?(nil)
        }
    }

    override func sleep(completionHandler: @escaping () -> Void) {
        completionHandler()
    }

    override func wake() {
        NSLog("PacketTunnel wake")
    }

    private func loadConfiguration() throws -> PacketTunnelProviderConfiguration {
        guard let tunnelProtocol = protocolConfiguration as? NETunnelProviderProtocol,
              let providerConfiguration = tunnelProtocol.providerConfiguration else {
            throw PacketTunnelProviderError.invalidConfiguration("missing NETunnelProviderProtocol providerConfiguration")
        }
        return try PacketTunnelProviderConfiguration(providerConfiguration: providerConfiguration)
    }

    private static func currentTimestampMs() -> Int64 {
        Int64(Date().timeIntervalSince1970 * 1000)
    }

    private func refreshRuntimeView(state: String) {
        guard let currentConfiguration else {
            runtimeView = nil
            return
        }
        runtimeView = PacketTunnelProviderSupport.makeRuntimeView(
            configuration: currentConfiguration,
            settings: currentSettings,
            state: state,
            backendSnapshot: wireGuardBackendAdapter.runtimeSnapshot(),
            packetRxCount: packetRxCount,
            packetRxBytes: packetRxBytes,
            packetTxCount: packetTxCount,
            packetTxBytes: packetTxBytes,
            lastPacketAtMs: lastPacketAtMs,
            lastAppliedAtMs: lastAppliedAtMs,
            lastError: lastErrorMessage
        )
    }

    private func startPacketReadLoop() {
        guard isReadingPackets else {
            return
        }
        packetFlow.readPackets { [weak self] packets, protocols in
            guard let self, self.isReadingPackets else {
                return
            }
            if !packets.isEmpty {
                self.packetRxCount += Int64(packets.count)
                self.packetRxBytes += Int64(packets.reduce(0) { $0 + $1.count })
                self.lastPacketAtMs = Self.currentTimestampMs()
                do {
                    let engineOutput = try self.wireGuardEngine.handleInboundPackets(packets, protocols: protocols)
                    self.writeOutboundPacketsIfNeeded(engineOutput)
                    self.refreshRuntimeView(state: "connected")
                } catch {
                    self.lastErrorMessage = error.localizedDescription
                    self.isReadingPackets = false
                    self.refreshRuntimeView(state: "failed")
                    NSLog("PacketTunnel failed to process inbound packets: %@", error.localizedDescription)
                    return
                }
            }
            self.startPacketReadLoop()
        }
    }

    private func writeOutboundPacketsIfNeeded(_ engineOutput: PacketTunnelEngineOutput) {
        guard !engineOutput.outboundPackets.isEmpty else {
            return
        }
        packetFlow.writePackets(engineOutput.outboundPackets, withProtocols: engineOutput.outboundProtocols)
        packetTxCount += Int64(engineOutput.outboundPackets.count)
        packetTxBytes += Int64(engineOutput.outboundPackets.reduce(0) { $0 + $1.count })
        lastPacketAtMs = Self.currentTimestampMs()
    }
}
