import Foundation

protocol WireGuardKitAdapterNativeHandleSessionFactorying {
  func makeSession(
    descriptor: WireGuardKitAdapterSessionHandleDescriptor,
    startedAtMs: Int64
  ) throws -> WireGuardKitAdapterNativeHandleSessioning
}

final class UnavailableWireGuardKitAdapterNativeHandleSessionFactory: WireGuardKitAdapterNativeHandleSessionFactorying {
  private let builder: WireGuardKitAdapterNativeHandleSessionBuilding

  init(
    builder: WireGuardKitAdapterNativeHandleSessionBuilding = UnavailableWireGuardKitAdapterNativeHandleSessionBuilder()
  ) {
    self.builder = builder
  }

  func makeSession(
    descriptor: WireGuardKitAdapterSessionHandleDescriptor,
    startedAtMs: Int64
  ) throws -> WireGuardKitAdapterNativeHandleSessioning {
    try builder.makeSession(descriptor: descriptor, startedAtMs: startedAtMs)
  }
}

final class WireGuardKitAdapterNativeHandleSessionFactory: WireGuardKitAdapterNativeHandleSessionFactorying {
  private let builder: WireGuardKitAdapterNativeHandleSessionBuilding

  init() {
    self.builder = WireGuardKitAdapterNativeHandleSessionBuilder()
  }

  init(builder: WireGuardKitAdapterNativeHandleSessionBuilding) {
    self.builder = builder
  }

  func makeSession(
    descriptor: WireGuardKitAdapterSessionHandleDescriptor,
    startedAtMs: Int64
  ) throws -> WireGuardKitAdapterNativeHandleSessioning {
    try builder.makeSession(descriptor: descriptor, startedAtMs: startedAtMs)
  }
}
