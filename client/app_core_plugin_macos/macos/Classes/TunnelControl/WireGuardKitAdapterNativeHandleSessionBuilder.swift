import Foundation

protocol WireGuardKitAdapterNativeHandleSessionBuilding {
  func makeSession(
    descriptor: WireGuardKitAdapterSessionHandleDescriptor,
    startedAtMs: Int64
  ) throws -> WireGuardKitAdapterNativeHandleSessioning
}

final class UnavailableWireGuardKitAdapterNativeHandleSessionBuilder: WireGuardKitAdapterNativeHandleSessionBuilding {
  private let creator: WireGuardKitAdapterBackedSessionCreating

  init(
    creator: WireGuardKitAdapterBackedSessionCreating = UnavailableWireGuardKitAdapterBackedSessionCreator()
  ) {
    self.creator = creator
  }

  func makeSession(
    descriptor: WireGuardKitAdapterSessionHandleDescriptor,
    startedAtMs: Int64
  ) throws -> WireGuardKitAdapterNativeHandleSessioning {
    try creator.makeSession(
      configuration: WireGuardKitAdapterBackedSessionConfigurationMapper.map(descriptor: descriptor),
      startedAtMs: startedAtMs
    )
  }
}

final class WireGuardKitAdapterNativeHandleSessionBuilder: WireGuardKitAdapterNativeHandleSessionBuilding {
  private let creator: WireGuardKitAdapterBackedSessionCreating

  init(
    creator: WireGuardKitAdapterBackedSessionCreating = WireGuardKitAdapterBackedSessionCreator()
  ) {
    self.creator = creator
  }

  func makeSession(
    descriptor: WireGuardKitAdapterSessionHandleDescriptor,
    startedAtMs: Int64
  ) throws -> WireGuardKitAdapterNativeHandleSessioning {
    try creator.makeSession(
      configuration: WireGuardKitAdapterBackedSessionConfigurationMapper.map(descriptor: descriptor),
      startedAtMs: startedAtMs
    )
  }
}
