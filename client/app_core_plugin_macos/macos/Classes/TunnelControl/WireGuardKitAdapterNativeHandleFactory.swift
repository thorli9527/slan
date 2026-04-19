import Foundation

protocol WireGuardKitAdapterNativeHandleFactorying {
  func makeHandle(
    descriptor: WireGuardKitAdapterSessionHandleDescriptor,
    startedAtMs: Int64
  ) throws -> WireGuardKitAdapterNativeHandleControlling
}

final class UnavailableWireGuardKitAdapterNativeHandleFactory: WireGuardKitAdapterNativeHandleFactorying {
  private let creator: WireGuardKitAdapterNativeHandleCreating

  init(
    creator: WireGuardKitAdapterNativeHandleCreating = UnavailableWireGuardKitAdapterNativeHandleCreator()
  ) {
    self.creator = creator
  }

  func makeHandle(
    descriptor: WireGuardKitAdapterSessionHandleDescriptor,
    startedAtMs: Int64
  ) throws -> WireGuardKitAdapterNativeHandleControlling {
    try creator.makeHandle(descriptor: descriptor, startedAtMs: startedAtMs)
  }
}

final class WireGuardKitAdapterNativeHandleFactory: WireGuardKitAdapterNativeHandleFactorying {
  private let creator: WireGuardKitAdapterNativeHandleCreating

  init(
    creator: WireGuardKitAdapterNativeHandleCreating = WireGuardKitAdapterNativeHandleCreator()
  ) {
    self.creator = creator
  }

  func makeHandle(
    descriptor: WireGuardKitAdapterSessionHandleDescriptor,
    startedAtMs: Int64
  ) throws -> WireGuardKitAdapterNativeHandleControlling {
    try creator.makeHandle(descriptor: descriptor, startedAtMs: startedAtMs)
  }
}
