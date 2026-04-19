import XCTest
@testable import TunnelControlHarness

final class WireGuardKitNativeAdapterTests: XCTestCase {
  func testDefaultAdapterFailsWithStableMessage() throws {
    let adapter = WireGuardKitNativeAdapter()
    let configuration = try makeWireGuardKitNativeBackendConfiguration()

    XCTAssertThrowsError(try adapter.makeSession(configuration: configuration)) { error in
      let providerError = error as? PacketTunnelProviderError
      XCTAssertEqual(providerError?.message, "WireGuardKit native backend session is not integrated yet")
    }
  }

  func testAdapterDelegatesToResolverAndSessionFactory() throws {
    let resolver = FakeWireGuardKitNativeModuleResolver()
    let session = FakeWireGuardKitNativeSession()
    let factory = FakeWireGuardKitNativeSessionFactory(session: session)
    let adapter = WireGuardKitNativeAdapter(
      moduleResolver: resolver,
      sessionFactory: factory
    )
    let configuration = try makeWireGuardKitNativeBackendConfiguration()

    let resolvedSession = try adapter.makeSession(configuration: configuration)

    XCTAssertEqual(resolver.resolveCallCount, 1)
    XCTAssertEqual(factory.module?.backendName, "wireguardkit-native")
    XCTAssertEqual(factory.configuration?.peerAddress, "100.64.0.2")
    XCTAssertTrue(resolvedSession === session)
  }

  func testDefaultResolverCanUseFrameworkOverridePath() throws {
    let expectedPath = "/tmp/WireGuardKit.framework"
    let resolver = DefaultWireGuardKitNativeModuleResolver(
      environment: ["SLAN_WIREGUARDKIT_FRAMEWORK_PATH": expectedPath],
      fileExistsAtPath: { path in path == expectedPath }
    )

    let module = try resolver.resolveModule()

    XCTAssertEqual(module.backendName, "wireguardkit-native")
    XCTAssertEqual(module.frameworkPath, expectedPath)
    XCTAssertEqual(module.moduleSource, "framework")
  }

  func testDefaultResolverCanBeForcedAvailableWithoutFramework() throws {
    let resolver = DefaultWireGuardKitNativeModuleResolver(
      environment: ["SLAN_WIREGUARDKIT_BACKEND_AVAILABLE": "true"],
      fileExistsAtPath: { _ in false }
    )

    let module = try resolver.resolveModule()

    XCTAssertEqual(module.backendName, "wireguardkit-native")
    XCTAssertNil(module.frameworkPath)
    XCTAssertEqual(module.moduleSource, "environment")
  }

  func testDefaultResolverHonorsUnavailableOverride() {
    let resolver = DefaultWireGuardKitNativeModuleResolver(
      environment: ["SLAN_WIREGUARDKIT_BACKEND_UNAVAILABLE": "1"],
      fileExistsAtPath: { _ in true }
    )

    XCTAssertThrowsError(try resolver.resolveModule()) { error in
      let providerError = error as? PacketTunnelProviderError
      XCTAssertEqual(providerError?.message, "WireGuardKit native backend disabled by environment override")
    }
  }
}

private final class FakeWireGuardKitNativeModuleResolver: WireGuardKitNativeModuleResolving {
  var resolveCallCount = 0

  func resolveModule() throws -> WireGuardKitNativeModule {
    resolveCallCount += 1
    return WireGuardKitNativeModule(
      backendName: "wireguardkit-native",
      frameworkPath: "/tmp/WireGuardKit.framework",
      moduleSource: "framework"
    )
  }
}

private final class FakeWireGuardKitNativeSessionFactory: WireGuardKitNativeSessionFactorying {
  private let providedSession: WireGuardKitNativeSessioning
  var module: WireGuardKitNativeModule?
  var configuration: WireGuardKitNativeBackendConfiguration?

  init(session: WireGuardKitNativeSessioning) {
    self.providedSession = session
  }

  func makeSession(
    module: WireGuardKitNativeModule,
    configuration: WireGuardKitNativeBackendConfiguration
  ) throws -> WireGuardKitNativeSessioning {
    self.module = module
    self.configuration = configuration
    return providedSession
  }
}

private final class FakeWireGuardKitNativeSession: WireGuardKitNativeSessioning {
  func stop() {}

  func handleInboundPackets(_ packets: [Data], protocols: [NSNumber]) throws -> PacketTunnelEngineOutput {
    .empty
  }

  func runtimeSnapshot() -> WireGuardBackendRuntimeSnapshot? {
    nil
  }
}
