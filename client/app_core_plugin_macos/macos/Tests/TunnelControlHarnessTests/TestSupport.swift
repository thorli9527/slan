import Foundation
@testable import TunnelControlHarness

func makeConfiguration() -> WireGuardTunnelConfiguration {
  WireGuardTunnelConfiguration(
    transport: "relay",
    localVirtualIp: "100.64.0.10",
    peerVirtualIp: "100.64.0.2",
    debugEngineMode: nil,
    interface: WireGuardTunnelInterfaceConfiguration(
      interfaceName: "utun9",
      keyPair: WireGuardTunnelKeyPair(publicKey: "pub", privateKey: "priv"),
      listenPort: 51820,
      mtu: 1280,
      addresses: ["100.64.0.10/32"],
      dnsServers: ["1.1.1.1"]
    ),
    peer: WireGuardTunnelPeerConfiguration(
      peerNodeId: "peer-1",
      publicKey: "peer-pub",
      presharedKey: nil,
      endpoint: "203.0.113.10:51820",
      allowedIps: ["100.64.0.2/32"],
      persistentKeepaliveSeconds: 25
    )
  )
}

func makeWireGuardKitBackendConfiguration() throws -> WireGuardKitBackendConfiguration {
  let configuration = try PacketTunnelProviderConfiguration(
    providerConfiguration: [
      "localVirtualIp": "100.64.0.10",
      "peerVirtualIp": "100.64.0.2",
      "transport": "relay",
      "debugEngineMode": "external",
      "wireguardInterface": [
        "addresses": ["100.64.0.10/32"],
        "dnsServers": ["1.1.1.1"],
        "mtu": 1280,
      ],
      "wireguardPeer": [
        "publicKey": "peer-pub",
        "endpoint": "203.0.113.10:51820",
        "allowedIps": [
          ["cidr": "100.64.0.2/32"],
        ],
      ],
    ]
  )
  return WireGuardKitBackendConfigMapper.map(configuration)
}

func makeWireGuardKitSessionConfiguration() throws -> WireGuardKitSessionConfiguration {
  try WireGuardKitSessionConfigMapper.map(makeWireGuardKitBackendConfiguration())
}

func makeWireGuardKitNativeBackendConfiguration() throws -> WireGuardKitNativeBackendConfiguration {
  try WireGuardKitNativeBackendConfigMapper.map(makeWireGuardKitSessionConfiguration())
}

func makeWireGuardKitNativeModule() -> WireGuardKitNativeModule {
  WireGuardKitNativeModule(
    backendName: "wireguardkit-native",
    frameworkPath: "/tmp/WireGuardKit.framework",
    moduleSource: "framework"
  )
}

func makeWireGuardKitNativeSessionConfiguration() throws -> WireGuardKitNativeSessionConfiguration {
  WireGuardKitNativeSessionConfigurationMapper.map(
    module: makeWireGuardKitNativeModule(),
    configuration: try makeWireGuardKitNativeBackendConfiguration()
  )
}

func makeWireGuardKitAdapterSessionConfiguration() throws -> WireGuardKitAdapterSessionConfiguration {
  WireGuardKitAdapterSessionConfigurationMapper.map(try makeWireGuardKitNativeSessionConfiguration())
}
