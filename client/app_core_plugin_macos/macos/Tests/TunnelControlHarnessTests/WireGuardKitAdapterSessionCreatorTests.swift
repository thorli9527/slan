import XCTest
@testable import TunnelControlHarness

private typealias FakeWireGuardKitDeepRuntimeSessionCreator =
  FakeWireGuardKitDeepRuntimeSessionCreatorImpl
private typealias FakeWireGuardKitDeepRuntimeSessionFactory =
  FakeWireGuardKitDeepRuntimeSessionFactoryImpl
private typealias FakeWireGuardKitDeepBackedSessionCreator =
  FakeWireGuardKitDeepBackedSessionCreatorImpl
private typealias FakeWireGuardKitDeepBackedSessionFactory =
  FakeWireGuardKitDeepBackedSessionFactoryImpl
private typealias FakeWireGuardKitDeepBackedHandleCreator =
  FakeWireGuardKitDeepBackedHandleCreatorImpl
private typealias FakeWireGuardKitDeepBackedHandleFactory =
  FakeWireGuardKitDeepBackedHandleFactoryImpl
private typealias FakeWireGuardKitDeepBackedHandleSessionCreator =
  FakeWireGuardKitDeepBackedHandleSessionCreatorImpl
private typealias FakeWireGuardKitDeepBackedHandleSessionFactory =
  FakeWireGuardKitDeepBackedHandleSessionFactoryImpl
private typealias FakeWireGuardKitDeepLeafHandleCreator =
  FakeWireGuardKitDeepLeafHandleCreatorImpl
private typealias FakeWireGuardKitDeepLeafHandleFactory =
  FakeWireGuardKitDeepLeafHandleFactoryImpl
private typealias FakeWireGuardKitDeepRuntimeSession =
  FakeWireGuardKitDeepRuntimeSessionImpl
private typealias FakeWireGuardKitDeepBackedSession =
  FakeWireGuardKitDeepBackedSessionImpl
private typealias FakeWireGuardKitDeepBackedHandle =
  FakeWireGuardKitDeepBackedHandleImpl
private typealias FakeWireGuardKitDeepBackedHandleSession =
  FakeWireGuardKitDeepBackedHandleSessionImpl
private typealias FakeWireGuardKitDeepLeafHandle =
  FakeWireGuardKitDeepLeafHandleImpl
private typealias FakeWireGuardKitDeepBackedRootHandleCreator =
  FakeWireGuardKitDeepBackedRootHandleCreatorImpl
private typealias FakeWireGuardKitDeepBackedRootHandleFactory =
  FakeWireGuardKitDeepBackedRootHandleFactoryImpl
private typealias FakeWireGuardKitDeepBackedRootHandle =
  FakeWireGuardKitDeepBackedRootHandleImpl
private typealias FakeWireGuardKitDeepBackedNativeHandleFactory =
  FakeWireGuardKitDeepBackedNativeHandleFactoryImpl
private typealias FakeWireGuardKitDeepBackedNativeHandle =
  FakeWireGuardKitDeepBackedNativeHandleImpl
private typealias FakeWireGuardKitDeepBackedNativeSessionFactory =
  FakeWireGuardKitDeepBackedNativeSessionFactoryImpl
private typealias FakeWireGuardKitDeepBackedNativeSessionCreator =
  FakeWireGuardKitDeepBackedNativeSessionCreatorImpl
private typealias FakeWireGuardKitDeepBackedNativeSession =
  FakeWireGuardKitDeepBackedNativeSessionImpl
private typealias FakeWireGuardKitDeepAdapterHandleCreator =
  FakeWireGuardKitDeepAdapterHandleCreatorImpl
private typealias FakeWireGuardKitDeepAdapterHandle =
  FakeWireGuardKitDeepAdapterHandleImpl
private typealias FakeWireGuardKitDeepAdapterSessionFactory =
  FakeWireGuardKitDeepAdapterSessionFactoryImpl
private typealias FakeWireGuardKitDeepAdapterSession =
  FakeWireGuardKitDeepAdapterSessionImpl
private typealias FakeWireGuardKitDeepNativeSessionFactory =
  FakeWireGuardKitDeepNativeSessionFactoryImpl
private typealias FakeWireGuardKitDeepNativeSessionCreator =
  FakeWireGuardKitDeepNativeSessionCreatorImpl
private typealias FakeWireGuardKitDeepNativeSession =
  FakeWireGuardKitDeepNativeSessionImpl
private typealias FakeWireGuardKitDeepRuntimeBridgeHandleCreator =
  FakeWireGuardKitDeepRuntimeBridgeHandleCreatorImpl
private typealias FakeWireGuardKitDeepRuntimeBridgeHandle =
  FakeWireGuardKitDeepRuntimeBridgeHandleImpl
private typealias FakeWireGuardKitDeepRuntimeHandleCreator =
  FakeWireGuardKitDeepRuntimeHandleCreatorImpl
private typealias FakeWireGuardKitDeepRuntimeHandle =
  FakeWireGuardKitDeepRuntimeHandleImpl
private typealias FakeWireGuardKitDeepAdapterNativeHandleFactory =
  FakeWireGuardKitDeepAdapterNativeHandleFactoryImpl
private typealias FakeWireGuardKitDeepAdapterNativeHandle =
  FakeWireGuardKitDeepAdapterNativeHandleImpl

final class WireGuardKitAdapterSessionCreatorTests: XCTestCase {
  func testBackedSessionCreatorDelegatesToHandleFactory() throws {
    let handleFactory = FakeWireGuardKitDeepBackedRootHandleFactory()
    let creator = WireGuardKitAdapterBackedSessionCreator(handleFactory: handleFactory)

    let session = try creator.makeSession(
      configuration: WireGuardKitAdapterBackedSessionConfiguration(
        backendName: "wireguardkit-native",
        frameworkPath: "/tmp/WireGuardKit.framework",
        moduleSource: "framework",
        interfaceAddress: "100.64.0.10",
        peerVirtualIp: "100.64.0.2",
        selectedEndpoint: "203.0.113.10:51820"
      ),
      startedAtMs: 333
    )

    XCTAssertTrue(session is WireGuardKitAdapterBackedSession)
    XCTAssertEqual(handleFactory.startedAtMs, 333)
    XCTAssertEqual(handleFactory.configuration?.peerVirtualIp, "100.64.0.2")
  }

  func testBackedSessionHandleFactoryMapsConfigurationIntoHandleConfiguration() throws {
    let creator = FakeWireGuardKitDeepBackedRootHandleCreator()
    let factory = WireGuardKitDeepBackedRootHandleFactory(creator: creator)

    let handle = try factory.makeHandle(
      configuration: WireGuardKitAdapterBackedSessionConfiguration(
        backendName: "wireguardkit-native",
        frameworkPath: "/tmp/WireGuardKit.framework",
        moduleSource: "framework",
        interfaceAddress: "100.64.0.10",
        peerVirtualIp: "100.64.0.2",
        selectedEndpoint: "203.0.113.10:51820"
      ),
      startedAtMs: 444
    )

    XCTAssertTrue(handle === creator.providedHandle)
    XCTAssertEqual(creator.startedAtMs, 444)
    XCTAssertEqual(
      creator.configuration,
      WireGuardKitDeepBackedRootHandleConfiguration(
        backendName: "wireguardkit-native",
        frameworkPath: "/tmp/WireGuardKit.framework",
        moduleSource: "framework",
        interfaceAddress: "100.64.0.10",
        peerVirtualIp: "100.64.0.2",
        selectedEndpoint: "203.0.113.10:51820"
      )
    )
  }

  func testBackedSessionHandleCreatorDelegatesToNativeHandleFactory() throws {
    let nativeFactory = FakeWireGuardKitDeepBackedNativeHandleFactory()
    let runtimeHandleCreator = FakeWireGuardKitTerminalRuntimeHandleCreator()
    let creator = WireGuardKitAdapterBackedSessionHandleCreator(
      nativeHandleFactory: nativeFactory,
      runtimeHandleCreator: runtimeHandleCreator
    )

    let handle = try creator.makeHandle(
      configuration: WireGuardKitDeepBackedRootHandleConfiguration(
        backendName: "wireguardkit-native",
        frameworkPath: "/tmp/WireGuardKit.framework",
        moduleSource: "framework",
        interfaceAddress: "100.64.0.10",
        peerVirtualIp: "100.64.0.2",
        selectedEndpoint: "203.0.113.10:51820"
      ),
      startedAtMs: 555
    )

    XCTAssertTrue(handle is WireGuardKitDeepBackedRootHandle)
    XCTAssertEqual(nativeFactory.startedAtMs, 555)
    XCTAssertEqual(nativeFactory.configuration?.interfaceAddress, "100.64.0.10")
    XCTAssertEqual(nativeFactory.configuration?.peerVirtualIp, "100.64.0.2")
    XCTAssertEqual(runtimeHandleCreator.startedAtMs, 555)
    XCTAssertEqual(runtimeHandleCreator.configuration?.peerVirtualIp, "100.64.0.2")
    XCTAssertEqual(runtimeHandleCreator.configuration?.selectedEndpoint, "203.0.113.10:51820")
  }

  func testBackedSessionHandlePrefersNativeRuntimeSnapshot() throws {
    let nativeHandle = FakeWireGuardKitDeepBackedNativeHandle()
    nativeHandle.snapshot = WireGuardBackendRuntimeSnapshot(
      backendName: "wireguardkit-native",
      backendState: "native-running",
      lastBackendError: nil,
      lastStartedAtMs: 999,
      peerVirtualIp: "100.64.0.77",
      selectedEndpoint: "198.51.100.77:51820"
    )
    let handle = WireGuardKitDeepBackedRootHandle(
      configuration: WireGuardKitDeepBackedRootHandleConfiguration(
        backendName: "wireguardkit-native",
        frameworkPath: "/tmp/WireGuardKit.framework",
        moduleSource: "framework",
        interfaceAddress: "100.64.0.10",
        peerVirtualIp: "100.64.0.2",
        selectedEndpoint: "203.0.113.10:51820"
      ),
      startedAtMs: 66,
      nativeHandle: nativeHandle,
      runtimeHandle: FakeWireGuardKitTerminalRuntimeHandle()
    )

    let snapshot = handle.runtimeSnapshot()

    XCTAssertEqual(snapshot?.backendState, "native-running")
    XCTAssertEqual(snapshot?.lastStartedAtMs, 999)
    XCTAssertEqual(snapshot?.peerVirtualIp, "100.64.0.77")
    XCTAssertEqual(snapshot?.selectedEndpoint, "198.51.100.77:51820")
  }

  func testBackedSessionHandlePrefersTerminalRuntimeSnapshotWhenNativeMissing() throws {
    let runtimeHandle = FakeWireGuardKitTerminalRuntimeHandle()
    runtimeHandle.snapshot = WireGuardBackendRuntimeSnapshot(
      backendName: "wireguardkit-native",
      backendState: "backed-handle-terminal-running",
      lastBackendError: nil,
      lastStartedAtMs: 1999,
      peerVirtualIp: "100.64.0.177",
      selectedEndpoint: "198.51.100.177:51820"
    )
    let handle = WireGuardKitDeepBackedRootHandle(
      configuration: WireGuardKitDeepBackedRootHandleConfiguration(
        backendName: "wireguardkit-native",
        frameworkPath: "/tmp/WireGuardKit.framework",
        moduleSource: "framework",
        interfaceAddress: "100.64.0.10",
        peerVirtualIp: "100.64.0.2",
        selectedEndpoint: "203.0.113.10:51820"
      ),
      startedAtMs: 67,
      nativeHandle: FakeWireGuardKitDeepBackedNativeHandle(),
      runtimeHandle: runtimeHandle
    )

    let snapshot = handle.runtimeSnapshot()

    XCTAssertEqual(snapshot?.backendState, "backed-handle-terminal-running")
    XCTAssertEqual(snapshot?.lastStartedAtMs, 1999)
    XCTAssertEqual(snapshot?.peerVirtualIp, "100.64.0.177")
    XCTAssertEqual(snapshot?.selectedEndpoint, "198.51.100.177:51820")
  }

  func testBackedSessionNativeHandleCreatorDelegatesToSessionFactory() throws {
    let sessionFactory = FakeWireGuardKitDeepBackedNativeSessionFactory()
    let runtimeHandleCreator = FakeWireGuardKitTerminalRuntimeHandleCreator()
    let creator = WireGuardKitAdapterBackedSessionNativeHandleCreator(
      sessionFactory: sessionFactory,
      runtimeHandleCreator: runtimeHandleCreator
    )

    let handle = try creator.makeHandle(
      configuration: WireGuardKitAdapterBackedSessionNativeHandleConfiguration(
        backendName: "wireguardkit-native",
        frameworkPath: "/tmp/WireGuardKit.framework",
        moduleSource: "framework",
        interfaceAddress: "100.64.0.10",
        peerVirtualIp: "100.64.0.2",
        selectedEndpoint: "203.0.113.10:51820"
      ),
      startedAtMs: 666
    )

    XCTAssertTrue(handle is WireGuardKitAdapterBackedSessionNativeHandle)
    XCTAssertEqual(sessionFactory.startedAtMs, 666)
    XCTAssertEqual(sessionFactory.configuration?.interfaceAddress, "100.64.0.10")
    XCTAssertEqual(sessionFactory.configuration?.peerVirtualIp, "100.64.0.2")
    XCTAssertEqual(runtimeHandleCreator.startedAtMs, 666)
    XCTAssertEqual(runtimeHandleCreator.configuration?.peerVirtualIp, "100.64.0.2")
    XCTAssertEqual(runtimeHandleCreator.configuration?.selectedEndpoint, "203.0.113.10:51820")
  }

  func testBackedSessionNativeHandleSessionFactoryDelegatesToCreator() throws {
    let creator = FakeWireGuardKitDeepBackedNativeSessionCreator()
    let factory = WireGuardKitAdapterBackedSessionNativeHandleSessionFactory(creator: creator)

    let session = try factory.makeSession(
      configuration: WireGuardKitAdapterBackedSessionNativeHandleConfiguration(
        backendName: "wireguardkit-native",
        frameworkPath: "/tmp/WireGuardKit.framework",
        moduleSource: "framework",
        interfaceAddress: "100.64.0.10",
        peerVirtualIp: "100.64.0.2",
        selectedEndpoint: "203.0.113.10:51820"
      ),
      startedAtMs: 777
    )

    XCTAssertTrue(session === creator.providedSession)
    XCTAssertEqual(creator.startedAtMs, 777)
    XCTAssertEqual(creator.configuration?.interfaceAddress, "100.64.0.10")
    XCTAssertEqual(creator.configuration?.peerVirtualIp, "100.64.0.2")
  }

  func testBackedSessionNativeHandleSessionCreatorDelegatesToAdapterHandleCreator() throws {
    let handleCreator = FakeWireGuardKitDeepAdapterHandleCreator()
    let runtimeHandleCreator = FakeWireGuardKitTerminalRuntimeHandleCreator()
    let creator = WireGuardKitAdapterBackedSessionNativeHandleSessionCreator(
      handleCreator: handleCreator,
      runtimeHandleCreator: runtimeHandleCreator
    )

    let session = try creator.makeSession(
      configuration: WireGuardKitAdapterBackedSessionNativeHandleSessionConfiguration(
        backendName: "wireguardkit-native",
        frameworkPath: "/tmp/WireGuardKit.framework",
        moduleSource: "framework",
        interfaceAddress: "100.64.0.10",
        peerVirtualIp: "100.64.0.2",
        selectedEndpoint: "203.0.113.10:51820"
      ),
      startedAtMs: 888
    )

    XCTAssertTrue(session is WireGuardKitAdapterBackedSessionNativeHandleSession)
    XCTAssertEqual(handleCreator.startedAtMs, 888)
    XCTAssertEqual(handleCreator.configuration?.interfaceAddress, "100.64.0.10")
    XCTAssertEqual(handleCreator.configuration?.peerVirtualIp, "100.64.0.2")
    XCTAssertEqual(runtimeHandleCreator.startedAtMs, 888)
    XCTAssertEqual(runtimeHandleCreator.configuration?.peerVirtualIp, "100.64.0.2")
    XCTAssertEqual(runtimeHandleCreator.configuration?.selectedEndpoint, "203.0.113.10:51820")
  }

  func testBackedSessionNativeHandleSessionPrefersAdapterHandleRuntimeSnapshot() throws {
    let handle = FakeWireGuardKitDeepAdapterHandle()
    handle.snapshot = WireGuardBackendRuntimeSnapshot(
      backendName: "wireguardkit-native",
      backendState: "adapter-running",
      lastBackendError: nil,
      lastStartedAtMs: 1456,
      peerVirtualIp: "100.64.0.89",
      selectedEndpoint: "198.51.100.89:51820"
    )
    let session = WireGuardKitAdapterBackedSessionNativeHandleSession(
      configuration: WireGuardKitAdapterBackedSessionNativeHandleSessionConfiguration(
        backendName: "wireguardkit-native",
        frameworkPath: "/tmp/WireGuardKit.framework",
        moduleSource: "framework",
        interfaceAddress: "100.64.0.10",
        peerVirtualIp: "100.64.0.2",
        selectedEndpoint: "203.0.113.10:51820"
      ),
      startedAtMs: 90,
      handle: handle,
      runtimeHandle: FakeWireGuardKitTerminalRuntimeHandle()
    )

    let snapshot = session.runtimeSnapshot()

    XCTAssertEqual(snapshot?.backendState, "adapter-running")
    XCTAssertEqual(snapshot?.lastStartedAtMs, 1456)
    XCTAssertEqual(snapshot?.peerVirtualIp, "100.64.0.89")
    XCTAssertEqual(snapshot?.selectedEndpoint, "198.51.100.89:51820")
  }

  func testBackedSessionNativeHandleSessionPrefersTerminalRuntimeSnapshotWhenHandleMissing() throws {
    let runtimeHandle = FakeWireGuardKitTerminalRuntimeHandle()
    runtimeHandle.snapshot = WireGuardBackendRuntimeSnapshot(
      backendName: "wireguardkit-native",
      backendState: "native-session-terminal-running",
      lastBackendError: nil,
      lastStartedAtMs: 1888,
      peerVirtualIp: "100.64.0.189",
      selectedEndpoint: "198.51.100.189:51820"
    )
    let session = WireGuardKitAdapterBackedSessionNativeHandleSession(
      configuration: WireGuardKitAdapterBackedSessionNativeHandleSessionConfiguration(
        backendName: "wireguardkit-native",
        frameworkPath: "/tmp/WireGuardKit.framework",
        moduleSource: "framework",
        interfaceAddress: "100.64.0.10",
        peerVirtualIp: "100.64.0.2",
        selectedEndpoint: "203.0.113.10:51820"
      ),
      startedAtMs: 91,
      handle: FakeWireGuardKitDeepAdapterHandle(),
      runtimeHandle: runtimeHandle
    )

    let snapshot = session.runtimeSnapshot()

    XCTAssertEqual(snapshot?.backendState, "native-session-terminal-running")
    XCTAssertEqual(snapshot?.lastStartedAtMs, 1888)
    XCTAssertEqual(snapshot?.peerVirtualIp, "100.64.0.189")
    XCTAssertEqual(snapshot?.selectedEndpoint, "198.51.100.189:51820")
  }

  func testBackedSessionAdapterHandleCreatorDelegatesToSessionFactory() throws {
    let sessionFactory = FakeWireGuardKitDeepAdapterSessionFactory()
    let runtimeHandleCreator = FakeWireGuardKitTerminalRuntimeHandleCreator()
    let creator = WireGuardKitDeepAdapterHandleCreator(
      sessionFactory: sessionFactory,
      runtimeHandleCreator: runtimeHandleCreator
    )

    let handle = try creator.makeHandle(
      configuration: WireGuardKitDeepAdapterHandleConfiguration(
        backendName: "wireguardkit-native",
        frameworkPath: "/tmp/WireGuardKit.framework",
        moduleSource: "framework",
        interfaceAddress: "100.64.0.10",
        peerVirtualIp: "100.64.0.2",
        selectedEndpoint: "203.0.113.10:51820"
      ),
      startedAtMs: 901
    )

    XCTAssertTrue(handle is WireGuardKitDeepAdapterHandle)
    XCTAssertEqual(sessionFactory.startedAtMs, 901)
    XCTAssertEqual(sessionFactory.configuration?.interfaceAddress, "100.64.0.10")
    XCTAssertEqual(sessionFactory.configuration?.peerVirtualIp, "100.64.0.2")
    XCTAssertEqual(runtimeHandleCreator.startedAtMs, 901)
    XCTAssertEqual(runtimeHandleCreator.configuration?.peerVirtualIp, "100.64.0.2")
    XCTAssertEqual(runtimeHandleCreator.configuration?.selectedEndpoint, "203.0.113.10:51820")
  }

  func testBackedSessionAdapterHandlePrefersSessionRuntimeSnapshot() throws {
    let session = FakeWireGuardKitDeepAdapterSession()
    session.snapshot = WireGuardBackendRuntimeSnapshot(
      backendName: "wireguardkit-native",
      backendState: "adapter-session-running",
      lastBackendError: nil,
      lastStartedAtMs: 1901,
      peerVirtualIp: "100.64.0.91",
      selectedEndpoint: "198.51.100.91:51820"
    )
    let handle = WireGuardKitDeepAdapterHandle(
      configuration: WireGuardKitDeepAdapterHandleConfiguration(
        backendName: "wireguardkit-native",
        frameworkPath: "/tmp/WireGuardKit.framework",
        moduleSource: "framework",
        interfaceAddress: "100.64.0.10",
        peerVirtualIp: "100.64.0.2",
        selectedEndpoint: "203.0.113.10:51820"
      ),
      startedAtMs: 92,
      session: session,
      runtimeHandle: FakeWireGuardKitTerminalRuntimeHandle()
    )

    let snapshot = handle.runtimeSnapshot()

    XCTAssertEqual(snapshot?.backendState, "adapter-session-running")
    XCTAssertEqual(snapshot?.lastStartedAtMs, 1901)
    XCTAssertEqual(snapshot?.peerVirtualIp, "100.64.0.91")
    XCTAssertEqual(snapshot?.selectedEndpoint, "198.51.100.91:51820")
  }

  func testBackedSessionAdapterHandlePrefersTerminalRuntimeSnapshotWhenSessionMissing() throws {
    let runtimeHandle = FakeWireGuardKitTerminalRuntimeHandle()
    runtimeHandle.snapshot = WireGuardBackendRuntimeSnapshot(
      backendName: "wireguardkit-native",
      backendState: "adapter-handle-terminal-running",
      lastBackendError: nil,
      lastStartedAtMs: 1902,
      peerVirtualIp: "100.64.0.191",
      selectedEndpoint: "198.51.100.191:51820"
    )
    let handle = WireGuardKitDeepAdapterHandle(
      configuration: WireGuardKitDeepAdapterHandleConfiguration(
        backendName: "wireguardkit-native",
        frameworkPath: "/tmp/WireGuardKit.framework",
        moduleSource: "framework",
        interfaceAddress: "100.64.0.10",
        peerVirtualIp: "100.64.0.2",
        selectedEndpoint: "203.0.113.10:51820"
      ),
      startedAtMs: 93,
      session: FakeWireGuardKitDeepAdapterSession(),
      runtimeHandle: runtimeHandle
    )

    let snapshot = handle.runtimeSnapshot()

    XCTAssertEqual(snapshot?.backendState, "adapter-handle-terminal-running")
    XCTAssertEqual(snapshot?.lastStartedAtMs, 1902)
    XCTAssertEqual(snapshot?.peerVirtualIp, "100.64.0.191")
    XCTAssertEqual(snapshot?.selectedEndpoint, "198.51.100.191:51820")
  }

  func testBackedSessionAdapterHandleSessionCreatorDelegatesToNativeHandleFactory() throws {
    let nativeHandleFactory = FakeWireGuardKitDeepAdapterNativeHandleFactory()
    let creator = WireGuardKitAdapterBackedSessionAdapterHandleSessionCreator(
      nativeHandleFactory: nativeHandleFactory
    )

    let session = try creator.makeSession(
      configuration: WireGuardKitAdapterBackedSessionAdapterHandleSessionConfiguration(
        backendName: "wireguardkit-native",
        frameworkPath: "/tmp/WireGuardKit.framework",
        moduleSource: "framework",
        interfaceAddress: "100.64.0.10",
        peerVirtualIp: "100.64.0.2",
        selectedEndpoint: "203.0.113.10:51820"
      ),
      startedAtMs: 2001
    )

    XCTAssertTrue(session is WireGuardKitAdapterBackedSessionAdapterHandleSession)
    XCTAssertEqual(nativeHandleFactory.startedAtMs, 2001)
    XCTAssertEqual(nativeHandleFactory.configuration?.interfaceAddress, "100.64.0.10")
    XCTAssertEqual(nativeHandleFactory.configuration?.peerVirtualIp, "100.64.0.2")
  }

  func testBackedSessionAdapterHandleSessionPrefersNativeHandleRuntimeSnapshot() throws {
    let nativeHandle = FakeWireGuardKitDeepAdapterNativeHandle()
    nativeHandle.snapshot = WireGuardBackendRuntimeSnapshot(
      backendName: "wireguardkit-native",
      backendState: "adapter-native-running",
      lastBackendError: nil,
      lastStartedAtMs: 2002,
      peerVirtualIp: "100.64.0.92",
      selectedEndpoint: "198.51.100.92:51820"
    )
    let session = WireGuardKitAdapterBackedSessionAdapterHandleSession(
      configuration: WireGuardKitAdapterBackedSessionAdapterHandleSessionConfiguration(
        backendName: "wireguardkit-native",
        frameworkPath: "/tmp/WireGuardKit.framework",
        moduleSource: "framework",
        interfaceAddress: "100.64.0.10",
        peerVirtualIp: "100.64.0.2",
        selectedEndpoint: "203.0.113.10:51820"
      ),
      startedAtMs: 93,
      nativeHandle: nativeHandle
    )

    let snapshot = session.runtimeSnapshot()

    XCTAssertEqual(snapshot?.backendState, "adapter-native-running")
    XCTAssertEqual(snapshot?.lastStartedAtMs, 2002)
    XCTAssertEqual(snapshot?.peerVirtualIp, "100.64.0.92")
    XCTAssertEqual(snapshot?.selectedEndpoint, "198.51.100.92:51820")
  }

  func testBackedSessionAdapterHandleNativeHandleCreatorDelegatesToSessionFactory() throws {
    let sessionFactory = FakeWireGuardKitDeepNativeSessionFactory()
    let creator = WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleCreator(
      sessionFactory: sessionFactory
    )

    let handle = try creator.makeHandle(
      configuration: WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleConfiguration(
        backendName: "wireguardkit-native",
        frameworkPath: "/tmp/WireGuardKit.framework",
        moduleSource: "framework",
        interfaceAddress: "100.64.0.10",
        peerVirtualIp: "100.64.0.2",
        selectedEndpoint: "203.0.113.10:51820"
      ),
      startedAtMs: 2003
    )

    XCTAssertTrue(handle is WireGuardKitAdapterBackedSessionAdapterHandleNativeHandle)
    XCTAssertEqual(sessionFactory.startedAtMs, 2003)
    XCTAssertEqual(sessionFactory.configuration?.interfaceAddress, "100.64.0.10")
    XCTAssertEqual(sessionFactory.configuration?.peerVirtualIp, "100.64.0.2")
  }

  func testBackedSessionAdapterHandleNativeHandleSessionFactoryDelegatesToCreator() throws {
    let creator = FakeWireGuardKitDeepNativeSessionCreator()
    let factory = WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionFactory(
      creator: creator
    )

    let session = try factory.makeSession(
      configuration: WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleConfiguration(
        backendName: "wireguardkit-native",
        frameworkPath: "/tmp/WireGuardKit.framework",
        moduleSource: "framework",
        interfaceAddress: "100.64.0.10",
        peerVirtualIp: "100.64.0.2",
        selectedEndpoint: "203.0.113.10:51820"
      ),
      startedAtMs: 2005
    )

    XCTAssertTrue(session === creator.providedSession)
    XCTAssertEqual(creator.startedAtMs, 2005)
    XCTAssertEqual(creator.configuration?.interfaceAddress, "100.64.0.10")
    XCTAssertEqual(creator.configuration?.peerVirtualIp, "100.64.0.2")
  }

  func testBackedSessionAdapterHandleNativeHandleSessionCreatorDelegatesToHandleCreator() throws {
    let handleCreator = FakeWireGuardKitDeepRuntimeBridgeHandleCreator()
    let creator = WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionCreator(
      handleCreator: handleCreator
    )

    let session = try creator.makeSession(
      configuration: WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionConfiguration(
        backendName: "wireguardkit-native",
        frameworkPath: "/tmp/WireGuardKit.framework",
        moduleSource: "framework",
        interfaceAddress: "100.64.0.10",
        peerVirtualIp: "100.64.0.2",
        selectedEndpoint: "203.0.113.10:51820"
      ),
      startedAtMs: 2006
    )

    XCTAssertTrue(session is WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSession)
    XCTAssertEqual(handleCreator.startedAtMs, 2006)
    XCTAssertEqual(handleCreator.configuration?.interfaceAddress, "100.64.0.10")
    XCTAssertEqual(handleCreator.configuration?.peerVirtualIp, "100.64.0.2")
  }

  func testBackedSessionAdapterHandleNativeHandleSessionHandleCreatorDelegatesToRuntimeHandleCreator()
    throws
  {
    let runtimeHandleCreator =
      FakeWireGuardKitDeepRuntimeHandleCreator()
    let creator = WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleCreator(
      runtimeHandleCreator: runtimeHandleCreator
    )

    let handle = try creator.makeHandle(
      configuration: WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleConfiguration(
        backendName: "wireguardkit-native",
        frameworkPath: "/tmp/WireGuardKit.framework",
        moduleSource: "framework",
        interfaceAddress: "100.64.0.10",
        peerVirtualIp: "100.64.0.2",
        selectedEndpoint: "203.0.113.10:51820"
      ),
      startedAtMs: 2008
    )

    XCTAssertTrue(handle is WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandle)
    XCTAssertEqual(runtimeHandleCreator.startedAtMs, 2008)
    XCTAssertEqual(runtimeHandleCreator.configuration?.interfaceAddress, "100.64.0.10")
    XCTAssertEqual(runtimeHandleCreator.configuration?.peerVirtualIp, "100.64.0.2")
  }

  func testBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleCreatorDelegatesToSessionFactory()
    throws
  {
    let sessionFactory =
      FakeWireGuardKitDeepRuntimeSessionFactory()
    let runtimeHandleCreator = FakeWireGuardKitTerminalRuntimeHandleCreator()
    let creator =
      WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleCreator(
        sessionFactory: sessionFactory,
        runtimeHandleCreator: runtimeHandleCreator
      )

    let handle = try creator.makeHandle(
      configuration:
        WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleConfiguration(
          backendName: "wireguardkit-native",
          frameworkPath: "/tmp/WireGuardKit.framework",
          moduleSource: "framework",
          interfaceAddress: "100.64.0.10",
          peerVirtualIp: "100.64.0.2",
          selectedEndpoint: "203.0.113.10:51820"
        ),
      startedAtMs: 2010
    )

    XCTAssertTrue(
      handle is WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandle
    )
    XCTAssertEqual(sessionFactory.startedAtMs, 2010)
    XCTAssertEqual(sessionFactory.configuration?.interfaceAddress, "100.64.0.10")
    XCTAssertEqual(sessionFactory.configuration?.peerVirtualIp, "100.64.0.2")
    XCTAssertEqual(runtimeHandleCreator.startedAtMs, 2010)
    XCTAssertEqual(runtimeHandleCreator.configuration?.peerVirtualIp, "100.64.0.2")
    XCTAssertEqual(runtimeHandleCreator.configuration?.selectedEndpoint, "203.0.113.10:51820")
  }

  func testDeepRuntimeSessionFactoryDelegatesToCreator()
    throws
  {
    let creator = FakeWireGuardKitDeepRuntimeSessionCreator()
    let factory = WireGuardKitDeepRuntimeSessionFactory(creator: creator)

    let session = try factory.makeSession(
      configuration:
        WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleConfiguration(
          backendName: "wireguardkit-native",
          frameworkPath: "/tmp/WireGuardKit.framework",
          moduleSource: "framework",
          interfaceAddress: "100.64.0.10",
          peerVirtualIp: "100.64.0.2",
          selectedEndpoint: "203.0.113.10:51820"
        ),
      startedAtMs: 2012
    )

    XCTAssertTrue(session === creator.providedSession)
    XCTAssertEqual(creator.startedAtMs, 2012)
    XCTAssertEqual(creator.configuration?.interfaceAddress, "100.64.0.10")
    XCTAssertEqual(creator.configuration?.peerVirtualIp, "100.64.0.2")
  }

  func testDeepRuntimeSessionCreatorDelegatesToBackedSessionFactory()
    throws
  {
    let backedSessionFactory = FakeWireGuardKitDeepBackedSessionFactory()
    let runtimeHandleCreator = FakeWireGuardKitTerminalRuntimeHandleCreator()
    let creator = WireGuardKitDeepRuntimeSessionCreator(
      backedSessionFactory: backedSessionFactory,
      runtimeHandleCreator: runtimeHandleCreator
    )

    let session = try creator.makeSession(
      configuration: WireGuardKitDeepRuntimeSessionConfiguration(
        backendName: "wireguardkit-native",
        frameworkPath: "/tmp/WireGuardKit.framework",
        moduleSource: "framework",
        interfaceAddress: "100.64.0.10",
        peerVirtualIp: "100.64.0.2",
        selectedEndpoint: "203.0.113.10:51820"
      ),
      startedAtMs: 2013
    )

    XCTAssertTrue(session is WireGuardKitDeepRuntimeSession)
    XCTAssertEqual(backedSessionFactory.startedAtMs, 2013)
    XCTAssertEqual(backedSessionFactory.configuration?.interfaceAddress, "100.64.0.10")
    XCTAssertEqual(backedSessionFactory.configuration?.peerVirtualIp, "100.64.0.2")
    XCTAssertEqual(runtimeHandleCreator.startedAtMs, 2013)
    XCTAssertEqual(runtimeHandleCreator.configuration?.peerVirtualIp, "100.64.0.2")
    XCTAssertEqual(runtimeHandleCreator.configuration?.selectedEndpoint, "203.0.113.10:51820")
  }

  func testDeepBackedSessionFactoryDelegatesToCreator()
    throws
  {
    let creator = FakeWireGuardKitDeepBackedSessionCreator()
    let factory = WireGuardKitDeepBackedSessionFactory(creator: creator)

    let session = try factory.makeSession(
      configuration: WireGuardKitDeepRuntimeSessionConfiguration(
        backendName: "wireguardkit-native",
        frameworkPath: "/tmp/WireGuardKit.framework",
        moduleSource: "framework",
        interfaceAddress: "100.64.0.10",
        peerVirtualIp: "100.64.0.2",
        selectedEndpoint: "203.0.113.10:51820"
      ),
      startedAtMs: 2015
    )

    XCTAssertTrue(session === creator.providedSession)
    XCTAssertEqual(creator.startedAtMs, 2015)
    XCTAssertEqual(creator.configuration?.interfaceAddress, "100.64.0.10")
    XCTAssertEqual(creator.configuration?.peerVirtualIp, "100.64.0.2")
  }

  func testDeepBackedSessionCreatorDelegatesToHandleFactory()
    throws
  {
    let handleFactory = FakeWireGuardKitDeepBackedHandleFactory()
    let creator = WireGuardKitDeepBackedSessionCreator(handleFactory: handleFactory)

    let session = try creator.makeSession(
      configuration: WireGuardKitDeepBackedSessionConfiguration(
        backendName: "wireguardkit-native",
        frameworkPath: "/tmp/WireGuardKit.framework",
        moduleSource: "framework",
        interfaceAddress: "100.64.0.10",
        peerVirtualIp: "100.64.0.2",
        selectedEndpoint: "203.0.113.10:51820"
      ),
      startedAtMs: 2016
    )

    XCTAssertTrue(session is WireGuardKitDeepBackedSession)
    XCTAssertEqual(handleFactory.startedAtMs, 2016)
    XCTAssertEqual(handleFactory.configuration?.interfaceAddress, "100.64.0.10")
    XCTAssertEqual(handleFactory.configuration?.peerVirtualIp, "100.64.0.2")
  }

  func testDeepBackedHandleFactoryDelegatesToCreator()
    throws
  {
    let creator = FakeWireGuardKitDeepBackedHandleCreator()
    let factory = WireGuardKitDeepBackedHandleFactory(creator: creator)

    let handle = try factory.makeHandle(
      configuration: WireGuardKitDeepBackedSessionConfiguration(
          backendName: "wireguardkit-native",
          frameworkPath: "/tmp/WireGuardKit.framework",
          moduleSource: "framework",
          interfaceAddress: "100.64.0.10",
          peerVirtualIp: "100.64.0.2",
          selectedEndpoint: "203.0.113.10:51820"
        ),
      startedAtMs: 2018
    )

    XCTAssertTrue(handle === creator.providedHandle)
    XCTAssertEqual(creator.startedAtMs, 2018)
    XCTAssertEqual(creator.configuration?.interfaceAddress, "100.64.0.10")
    XCTAssertEqual(creator.configuration?.peerVirtualIp, "100.64.0.2")
  }

  func testDeepBackedHandleCreatorDelegatesToSessionFactory()
    throws
  {
    let sessionFactory = FakeWireGuardKitDeepBackedHandleSessionFactory()
    let creator = WireGuardKitDeepBackedHandleCreator(sessionFactory: sessionFactory)

    let handle = try creator.makeHandle(
      configuration: WireGuardKitDeepBackedHandleConfiguration(
        backendName: "wireguardkit-native",
        frameworkPath: "/tmp/WireGuardKit.framework",
        moduleSource: "framework",
        interfaceAddress: "100.64.0.10",
        peerVirtualIp: "100.64.0.2",
        selectedEndpoint: "203.0.113.10:51820"
      ),
      startedAtMs: 2019
    )

    XCTAssertTrue(handle is WireGuardKitDeepBackedHandle)
    XCTAssertEqual(sessionFactory.startedAtMs, 2019)
    XCTAssertEqual(sessionFactory.configuration?.interfaceAddress, "100.64.0.10")
    XCTAssertEqual(sessionFactory.configuration?.peerVirtualIp, "100.64.0.2")
  }

  func testDeepBackedHandleSessionFactoryDelegatesToCreator()
    throws
  {
    let creator = FakeWireGuardKitDeepBackedHandleSessionCreator()
    let factory = WireGuardKitDeepBackedHandleSessionFactory(creator: creator)

    let session = try factory.makeSession(
      configuration: WireGuardKitDeepBackedHandleConfiguration(
          backendName: "wireguardkit-native",
          frameworkPath: "/tmp/WireGuardKit.framework",
          moduleSource: "framework",
          interfaceAddress: "100.64.0.10",
          peerVirtualIp: "100.64.0.2",
          selectedEndpoint: "203.0.113.10:51820"
        ),
      startedAtMs: 2020
    )

    XCTAssertTrue(session === creator.providedSession)
    XCTAssertEqual(creator.startedAtMs, 2020)
    XCTAssertEqual(creator.configuration?.interfaceAddress, "100.64.0.10")
    XCTAssertEqual(creator.configuration?.peerVirtualIp, "100.64.0.2")
  }

  func testDeepBackedHandleSessionCreatorDelegatesToHandleFactory()
    throws
  {
    let handleFactory = FakeWireGuardKitDeepLeafHandleFactory()
    let creator = WireGuardKitDeepBackedHandleSessionCreator(handleFactory: handleFactory)

    let session = try creator.makeSession(
      configuration: WireGuardKitDeepBackedHandleSessionConfiguration(
        backendName: "wireguardkit-native",
        frameworkPath: "/tmp/WireGuardKit.framework",
        moduleSource: "framework",
        interfaceAddress: "100.64.0.10",
        peerVirtualIp: "100.64.0.2",
        selectedEndpoint: "203.0.113.10:51820"
      ),
      startedAtMs: 2021
    )

    XCTAssertTrue(session is WireGuardKitDeepBackedHandleSession)
    XCTAssertEqual(handleFactory.startedAtMs, 2021)
    XCTAssertEqual(handleFactory.configuration?.interfaceAddress, "100.64.0.10")
    XCTAssertEqual(handleFactory.configuration?.peerVirtualIp, "100.64.0.2")
  }

  func testDeepLeafHandleFactoryDelegatesToCreator()
    throws
  {
    let creator = FakeWireGuardKitDeepLeafHandleCreator()
    let factory = WireGuardKitDeepLeafHandleFactory(creator: creator)

    let handle = try factory.makeHandle(
      configuration: WireGuardKitDeepBackedHandleSessionConfiguration(
          backendName: "wireguardkit-native",
          frameworkPath: "/tmp/WireGuardKit.framework",
          moduleSource: "framework",
          interfaceAddress: "100.64.0.10",
          peerVirtualIp: "100.64.0.2",
          selectedEndpoint: "203.0.113.10:51820"
        ),
      startedAtMs: 2023
    )

    XCTAssertTrue(handle === creator.providedHandle)
    XCTAssertEqual(creator.startedAtMs, 2023)
    XCTAssertEqual(creator.configuration?.interfaceAddress, "100.64.0.10")
    XCTAssertEqual(creator.configuration?.peerVirtualIp, "100.64.0.2")
  }

  func testDeepLeafHandleCreatorDelegatesToTerminalRuntimeHandleCreator()
    throws
  {
    let runtimeHandleCreator = FakeWireGuardKitTerminalRuntimeHandleCreator()
    let creator = WireGuardKitDeepLeafHandleCreator(runtimeHandleCreator: runtimeHandleCreator)

    let handle = try creator.makeHandle(
      configuration: WireGuardKitDeepLeafHandleConfiguration(
        backendName: "wireguardkit-native",
        frameworkPath: "/tmp/WireGuardKit.framework",
        moduleSource: "framework",
        interfaceAddress: "100.64.0.10",
        peerVirtualIp: "100.64.0.2",
        selectedEndpoint: "203.0.113.10:51820"
      ),
      startedAtMs: 2024
    )

    XCTAssertTrue(handle is WireGuardKitDeepLeafHandle)
    XCTAssertEqual(runtimeHandleCreator.startedAtMs, 2024)
    XCTAssertEqual(runtimeHandleCreator.configuration?.peerVirtualIp, "100.64.0.2")
    XCTAssertEqual(runtimeHandleCreator.configuration?.selectedEndpoint, "203.0.113.10:51820")
  }

  func testDeepLeafHandlePrefersTerminalRuntimeHandleSnapshot()
  {
    let runtimeHandle = FakeWireGuardKitTerminalRuntimeHandle()
    runtimeHandle.snapshot = WireGuardBackendRuntimeSnapshot(
      backendName: "wireguardkit-native",
      backendState: "terminal-runtime-running",
      lastBackendError: nil,
      lastStartedAtMs: 2025,
      peerVirtualIp: "100.64.0.97",
      selectedEndpoint: "198.51.100.97:51820"
    )
    let handle =
      WireGuardKitDeepLeafHandle(
        configuration: WireGuardKitDeepLeafHandleConfiguration(
            backendName: "wireguardkit-native",
            frameworkPath: "/tmp/WireGuardKit.framework",
            moduleSource: "framework",
            interfaceAddress: "100.64.0.10",
            peerVirtualIp: "100.64.0.2",
            selectedEndpoint: "203.0.113.10:51820"
          ),
        startedAtMs: 97,
        runtimeHandle: runtimeHandle
      )

    let snapshot = handle.runtimeSnapshot()

    XCTAssertEqual(snapshot?.backendState, "terminal-runtime-running")
    XCTAssertEqual(snapshot?.lastStartedAtMs, 2025)
    XCTAssertEqual(snapshot?.peerVirtualIp, "100.64.0.97")
    XCTAssertEqual(snapshot?.selectedEndpoint, "198.51.100.97:51820")
  }

  func testDeepBackedHandleSessionPrefersHandleSnapshot()
  {
    let handle = FakeWireGuardKitDeepLeafHandle()
    handle.snapshot = WireGuardBackendRuntimeSnapshot(
      backendName: "wireguardkit-native",
      backendState: "backed-session-handle-session-handle-running",
      lastBackendError: nil,
      lastStartedAtMs: 2022,
      peerVirtualIp: "100.64.0.96",
      selectedEndpoint: "198.51.100.96:51820"
    )
    let session =
      WireGuardKitDeepBackedHandleSession(
        configuration: WireGuardKitDeepBackedHandleSessionConfiguration(
            backendName: "wireguardkit-native",
            frameworkPath: "/tmp/WireGuardKit.framework",
            moduleSource: "framework",
            interfaceAddress: "100.64.0.10",
            peerVirtualIp: "100.64.0.2",
            selectedEndpoint: "203.0.113.10:51820"
          ),
        startedAtMs: 96,
        handle: handle
      )

    let snapshot = session.runtimeSnapshot()

    XCTAssertEqual(snapshot?.backendState, "backed-session-handle-session-handle-running")
    XCTAssertEqual(snapshot?.lastStartedAtMs, 2022)
    XCTAssertEqual(snapshot?.peerVirtualIp, "100.64.0.96")
    XCTAssertEqual(snapshot?.selectedEndpoint, "198.51.100.96:51820")
  }

  func testBackedSessionAdapterHandleNativeHandleSessionPrefersHandleRuntimeSnapshot() throws {
    let handle = FakeWireGuardKitDeepRuntimeBridgeHandle()
    handle.snapshot = WireGuardBackendRuntimeSnapshot(
      backendName: "wireguardkit-native",
      backendState: "adapter-native-session-handle-running",
      lastBackendError: nil,
      lastStartedAtMs: 2007,
      peerVirtualIp: "100.64.0.94",
      selectedEndpoint: "198.51.100.94:51820"
    )
    let session = WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSession(
      configuration: WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionConfiguration(
        backendName: "wireguardkit-native",
        frameworkPath: "/tmp/WireGuardKit.framework",
        moduleSource: "framework",
        interfaceAddress: "100.64.0.10",
        peerVirtualIp: "100.64.0.2",
        selectedEndpoint: "203.0.113.10:51820"
      ),
      startedAtMs: 95,
      handle: handle
    )

    let snapshot = session.runtimeSnapshot()

    XCTAssertEqual(snapshot?.backendState, "adapter-native-session-handle-running")
    XCTAssertEqual(snapshot?.lastStartedAtMs, 2007)
    XCTAssertEqual(snapshot?.peerVirtualIp, "100.64.0.94")
    XCTAssertEqual(snapshot?.selectedEndpoint, "198.51.100.94:51820")
  }

  func testBackedSessionAdapterHandleNativeHandleSessionHandlePrefersRuntimeHandleSnapshot() {
    let runtimeHandle =
      FakeWireGuardKitDeepRuntimeHandle()
    runtimeHandle.snapshot = WireGuardBackendRuntimeSnapshot(
      backendName: "wireguardkit-native",
      backendState: "adapter-native-session-runtime-running",
      lastBackendError: nil,
      lastStartedAtMs: 2009,
      peerVirtualIp: "100.64.0.95",
      selectedEndpoint: "198.51.100.95:51820"
    )
    let handle = WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandle(
      configuration: WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleConfiguration(
        backendName: "wireguardkit-native",
        frameworkPath: "/tmp/WireGuardKit.framework",
        moduleSource: "framework",
        interfaceAddress: "100.64.0.10",
        peerVirtualIp: "100.64.0.2",
        selectedEndpoint: "203.0.113.10:51820"
      ),
      startedAtMs: 96,
      runtimeHandle: runtimeHandle
    )

    let snapshot = handle.runtimeSnapshot()

    XCTAssertEqual(snapshot?.backendState, "adapter-native-session-runtime-running")
    XCTAssertEqual(snapshot?.lastStartedAtMs, 2009)
    XCTAssertEqual(snapshot?.peerVirtualIp, "100.64.0.95")
    XCTAssertEqual(snapshot?.selectedEndpoint, "198.51.100.95:51820")
  }

  func testBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandlePrefersSessionSnapshot() {
    let session =
      FakeWireGuardKitDeepRuntimeSession()
    session.snapshot = WireGuardBackendRuntimeSnapshot(
      backendName: "wireguardkit-native",
      backendState: "adapter-native-session-runtime-session-running",
      lastBackendError: nil,
      lastStartedAtMs: 2011,
      peerVirtualIp: "100.64.0.96",
      selectedEndpoint: "198.51.100.96:51820"
    )
    let handle =
      WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandle(
        configuration:
          WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleConfiguration(
            backendName: "wireguardkit-native",
            frameworkPath: "/tmp/WireGuardKit.framework",
            moduleSource: "framework",
            interfaceAddress: "100.64.0.10",
            peerVirtualIp: "100.64.0.2",
            selectedEndpoint: "203.0.113.10:51820"
          ),
        startedAtMs: 97,
        session: session,
        runtimeHandle: FakeWireGuardKitTerminalRuntimeHandle()
      )

    let snapshot = handle.runtimeSnapshot()

    XCTAssertEqual(snapshot?.backendState, "adapter-native-session-runtime-session-running")
    XCTAssertEqual(snapshot?.lastStartedAtMs, 2011)
    XCTAssertEqual(snapshot?.peerVirtualIp, "100.64.0.96")
    XCTAssertEqual(snapshot?.selectedEndpoint, "198.51.100.96:51820")
  }

  func testBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandlePrefersTerminalRuntimeHandleSnapshotWhenSessionMissing()
  {
    let runtimeHandle = FakeWireGuardKitTerminalRuntimeHandle()
    runtimeHandle.snapshot = WireGuardBackendRuntimeSnapshot(
      backendName: "wireguardkit-native",
      backendState: "adapter-native-session-runtime-terminal-running",
      lastBackendError: nil,
      lastStartedAtMs: 2011,
      peerVirtualIp: "100.64.0.196",
      selectedEndpoint: "198.51.100.196:51820"
    )
    let handle =
      WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandle(
        configuration:
          WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleConfiguration(
            backendName: "wireguardkit-native",
            frameworkPath: "/tmp/WireGuardKit.framework",
            moduleSource: "framework",
            interfaceAddress: "100.64.0.10",
            peerVirtualIp: "100.64.0.2",
            selectedEndpoint: "203.0.113.10:51820"
          ),
        startedAtMs: 197,
        session:
          FakeWireGuardKitDeepRuntimeSession(),
        runtimeHandle: runtimeHandle
      )

    let snapshot = handle.runtimeSnapshot()

    XCTAssertEqual(snapshot?.backendState, "adapter-native-session-runtime-terminal-running")
    XCTAssertEqual(snapshot?.lastStartedAtMs, 2011)
    XCTAssertEqual(snapshot?.peerVirtualIp, "100.64.0.196")
    XCTAssertEqual(snapshot?.selectedEndpoint, "198.51.100.196:51820")
  }

  func testBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleSessionPrefersBackedSessionSnapshot() {
    let backedSession =
      FakeWireGuardKitDeepBackedSession()
    backedSession.snapshot = WireGuardBackendRuntimeSnapshot(
      backendName: "wireguardkit-native",
      backendState: "adapter-native-session-runtime-backed-running",
      lastBackendError: nil,
      lastStartedAtMs: 2014,
      peerVirtualIp: "100.64.0.97",
      selectedEndpoint: "198.51.100.97:51820"
    )
    let session =
      WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleSession(
        configuration:
          WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleSessionConfiguration(
            backendName: "wireguardkit-native",
            frameworkPath: "/tmp/WireGuardKit.framework",
            moduleSource: "framework",
            interfaceAddress: "100.64.0.10",
            peerVirtualIp: "100.64.0.2",
            selectedEndpoint: "203.0.113.10:51820"
          ),
        startedAtMs: 98,
        backedSession: backedSession,
        runtimeHandle: FakeWireGuardKitTerminalRuntimeHandle()
      )

    let snapshot = session.runtimeSnapshot()

    XCTAssertEqual(snapshot?.backendState, "adapter-native-session-runtime-backed-running")
    XCTAssertEqual(snapshot?.lastStartedAtMs, 2014)
    XCTAssertEqual(snapshot?.peerVirtualIp, "100.64.0.97")
    XCTAssertEqual(snapshot?.selectedEndpoint, "198.51.100.97:51820")
  }

  func testBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleSessionPrefersTerminalRuntimeHandleSnapshotWhenBackedSessionMissing()
  {
    let runtimeHandle = FakeWireGuardKitTerminalRuntimeHandle()
    runtimeHandle.snapshot = WireGuardBackendRuntimeSnapshot(
      backendName: "wireguardkit-native",
      backendState: "adapter-native-session-runtime-terminal-running",
      lastBackendError: nil,
      lastStartedAtMs: 2014,
      peerVirtualIp: "100.64.0.197",
      selectedEndpoint: "198.51.100.197:51820"
    )
    let session =
      WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleSession(
        configuration:
          WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleSessionConfiguration(
            backendName: "wireguardkit-native",
            frameworkPath: "/tmp/WireGuardKit.framework",
            moduleSource: "framework",
            interfaceAddress: "100.64.0.10",
            peerVirtualIp: "100.64.0.2",
            selectedEndpoint: "203.0.113.10:51820"
          ),
        startedAtMs: 198,
        backedSession:
          FakeWireGuardKitDeepBackedSession(),
        runtimeHandle: runtimeHandle
      )

    let snapshot = session.runtimeSnapshot()

    XCTAssertEqual(snapshot?.backendState, "adapter-native-session-runtime-terminal-running")
    XCTAssertEqual(snapshot?.lastStartedAtMs, 2014)
    XCTAssertEqual(snapshot?.peerVirtualIp, "100.64.0.197")
    XCTAssertEqual(snapshot?.selectedEndpoint, "198.51.100.197:51820")
  }

  func testBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleSessionBackedSessionPrefersHandleSnapshot() {
    let handle =
      FakeWireGuardKitDeepBackedHandle()
    handle.snapshot = WireGuardBackendRuntimeSnapshot(
      backendName: "wireguardkit-native",
      backendState: "adapter-native-session-runtime-backed-handle-running",
      lastBackendError: nil,
      lastStartedAtMs: 2017,
      peerVirtualIp: "100.64.0.98",
      selectedEndpoint: "198.51.100.98:51820"
    )
    let session =
      WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleSessionBackedSession(
        configuration:
          WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleSessionBackedSessionConfiguration(
            backendName: "wireguardkit-native",
            frameworkPath: "/tmp/WireGuardKit.framework",
            moduleSource: "framework",
            interfaceAddress: "100.64.0.10",
            peerVirtualIp: "100.64.0.2",
            selectedEndpoint: "203.0.113.10:51820"
          ),
        startedAtMs: 99,
        handle: handle
      )

    let snapshot = session.runtimeSnapshot()

    XCTAssertEqual(snapshot?.backendState, "adapter-native-session-runtime-backed-handle-running")
    XCTAssertEqual(snapshot?.lastStartedAtMs, 2017)
    XCTAssertEqual(snapshot?.peerVirtualIp, "100.64.0.98")
    XCTAssertEqual(snapshot?.selectedEndpoint, "198.51.100.98:51820")
  }

  func testBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleSessionBackedSessionHandlePrefersSessionSnapshot() {
    let session =
      FakeWireGuardKitDeepBackedHandleSession()
    session.snapshot = WireGuardBackendRuntimeSnapshot(
      backendName: "wireguardkit-native",
      backendState: "adapter-native-session-runtime-backed-handle-session-running",
      lastBackendError: nil,
      lastStartedAtMs: 2020,
      peerVirtualIp: "100.64.0.99",
      selectedEndpoint: "198.51.100.99:51820"
    )
    let handle =
      WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleSessionBackedSessionHandle(
        configuration:
          WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleSessionBackedSessionHandleConfiguration(
            backendName: "wireguardkit-native",
            frameworkPath: "/tmp/WireGuardKit.framework",
            moduleSource: "framework",
            interfaceAddress: "100.64.0.10",
            peerVirtualIp: "100.64.0.2",
            selectedEndpoint: "203.0.113.10:51820"
          ),
        startedAtMs: 100,
        session: session
      )

    let snapshot = handle.runtimeSnapshot()

    XCTAssertEqual(snapshot?.backendState, "adapter-native-session-runtime-backed-handle-session-running")
    XCTAssertEqual(snapshot?.lastStartedAtMs, 2020)
    XCTAssertEqual(snapshot?.peerVirtualIp, "100.64.0.99")
    XCTAssertEqual(snapshot?.selectedEndpoint, "198.51.100.99:51820")
  }

  func testBackedSessionAdapterHandleNativeHandlePrefersSessionRuntimeSnapshot() throws {
    let session = FakeWireGuardKitDeepNativeSession()
    session.snapshot = WireGuardBackendRuntimeSnapshot(
      backendName: "wireguardkit-native",
      backendState: "adapter-native-session-running",
      lastBackendError: nil,
      lastStartedAtMs: 2004,
      peerVirtualIp: "100.64.0.93",
      selectedEndpoint: "198.51.100.93:51820"
    )
    let handle = WireGuardKitAdapterBackedSessionAdapterHandleNativeHandle(
      configuration: WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleConfiguration(
        backendName: "wireguardkit-native",
        frameworkPath: "/tmp/WireGuardKit.framework",
        moduleSource: "framework",
        interfaceAddress: "100.64.0.10",
        peerVirtualIp: "100.64.0.2",
        selectedEndpoint: "203.0.113.10:51820"
      ),
      startedAtMs: 94,
      session: session
    )

    let snapshot = handle.runtimeSnapshot()

    XCTAssertEqual(snapshot?.backendState, "adapter-native-session-running")
    XCTAssertEqual(snapshot?.lastStartedAtMs, 2004)
    XCTAssertEqual(snapshot?.peerVirtualIp, "100.64.0.93")
    XCTAssertEqual(snapshot?.selectedEndpoint, "198.51.100.93:51820")
  }

  func testBackedSessionNativeHandlePrefersSessionRuntimeSnapshot() throws {
    let session = FakeWireGuardKitDeepBackedNativeSession()
    session.snapshot = WireGuardBackendRuntimeSnapshot(
      backendName: "wireguardkit-native",
      backendState: "session-running",
      lastBackendError: nil,
      lastStartedAtMs: 1234,
      peerVirtualIp: "100.64.0.88",
      selectedEndpoint: "198.51.100.88:51820"
    )
    let handle = WireGuardKitAdapterBackedSessionNativeHandle(
      configuration: WireGuardKitAdapterBackedSessionNativeHandleConfiguration(
        backendName: "wireguardkit-native",
        frameworkPath: "/tmp/WireGuardKit.framework",
        moduleSource: "framework",
        interfaceAddress: "100.64.0.10",
        peerVirtualIp: "100.64.0.2",
        selectedEndpoint: "203.0.113.10:51820"
      ),
      startedAtMs: 77,
      session: session,
      runtimeHandle: FakeWireGuardKitTerminalRuntimeHandle()
    )

    let snapshot = handle.runtimeSnapshot()

    XCTAssertEqual(snapshot?.backendState, "session-running")
    XCTAssertEqual(snapshot?.lastStartedAtMs, 1234)
    XCTAssertEqual(snapshot?.peerVirtualIp, "100.64.0.88")
    XCTAssertEqual(snapshot?.selectedEndpoint, "198.51.100.88:51820")
  }

  func testBackedSessionNativeHandlePrefersTerminalRuntimeSnapshotWhenSessionMissing() throws {
    let runtimeHandle = FakeWireGuardKitTerminalRuntimeHandle()
    runtimeHandle.snapshot = WireGuardBackendRuntimeSnapshot(
      backendName: "wireguardkit-native",
      backendState: "native-handle-terminal-running",
      lastBackendError: nil,
      lastStartedAtMs: 1777,
      peerVirtualIp: "100.64.0.178",
      selectedEndpoint: "198.51.100.178:51820"
    )
    let handle = WireGuardKitAdapterBackedSessionNativeHandle(
      configuration: WireGuardKitAdapterBackedSessionNativeHandleConfiguration(
        backendName: "wireguardkit-native",
        frameworkPath: "/tmp/WireGuardKit.framework",
        moduleSource: "framework",
        interfaceAddress: "100.64.0.10",
        peerVirtualIp: "100.64.0.2",
        selectedEndpoint: "203.0.113.10:51820"
      ),
      startedAtMs: 78,
      session: FakeWireGuardKitDeepBackedNativeSession(),
      runtimeHandle: runtimeHandle
    )

    let snapshot = handle.runtimeSnapshot()

    XCTAssertEqual(snapshot?.backendState, "native-handle-terminal-running")
    XCTAssertEqual(snapshot?.lastStartedAtMs, 1777)
    XCTAssertEqual(snapshot?.peerVirtualIp, "100.64.0.178")
    XCTAssertEqual(snapshot?.selectedEndpoint, "198.51.100.178:51820")
  }

  func testNativeHandleSessionBuilderMapsDescriptorIntoBackedSessionConfiguration() throws {
    let creator = FakeWireGuardKitBackedSessionCreatorImpl()
    let builder = WireGuardKitAdapterNativeHandleSessionBuilder(creator: creator)

    let session = try builder.makeSession(
      descriptor: WireGuardKitAdapterSessionHandleDescriptorMapper.map(try makeWireGuardKitAdapterSessionConfiguration()),
      startedAtMs: 222
    )

    XCTAssertTrue(session === creator.providedSession)
    XCTAssertEqual(creator.startedAtMs, 222)
    XCTAssertEqual(
      creator.configuration,
      WireGuardKitAdapterBackedSessionConfiguration(
        backendName: "wireguardkit-native",
        frameworkPath: "/tmp/WireGuardKit.framework",
        moduleSource: "framework",
        interfaceAddress: "100.64.0.10",
        peerVirtualIp: "100.64.0.2",
        selectedEndpoint: "203.0.113.10:51820"
      )
    )
  }

  func testNativeHandleSessionFactoryDelegatesToBuilder() throws {
    let builder = FakeWireGuardKitAdapterNativeHandleSessionBuilder()
    let factory = WireGuardKitAdapterNativeHandleSessionFactory(builder: builder)

    let session = try factory.makeSession(
      descriptor: WireGuardKitAdapterSessionHandleDescriptorMapper.map(try makeWireGuardKitAdapterSessionConfiguration()),
      startedAtMs: 654
    )

    XCTAssertTrue(session === builder.providedSession)
    XCTAssertEqual(builder.startedAtMs, 654)
    XCTAssertEqual(builder.descriptor?.peer.address, "100.64.0.2")
  }

  func testNativeHandleCreatorDelegatesToSessionFactory() throws {
    let builder = FakeWireGuardKitAdapterNativeHandleSessionBuilder()
    let sessionFactory = WireGuardKitAdapterNativeHandleSessionFactory(builder: builder)
    let runtimeHandleCreator = FakeWireGuardKitTerminalRuntimeHandleCreator()
    let creator = WireGuardKitAdapterNativeHandleCreator(
      sessionFactory: sessionFactory,
      runtimeHandleCreator: runtimeHandleCreator
    )

    let handle = try creator.makeHandle(
      descriptor: WireGuardKitAdapterSessionHandleDescriptorMapper.map(try makeWireGuardKitAdapterSessionConfiguration()),
      startedAtMs: 456
    )

    XCTAssertTrue(handle is WireGuardKitAdapterNativeHandle)
    XCTAssertEqual(builder.startedAtMs, 456)
    XCTAssertEqual(builder.descriptor?.peer.address, "100.64.0.2")
    XCTAssertEqual(runtimeHandleCreator.startedAtMs, 456)
    XCTAssertEqual(runtimeHandleCreator.configuration?.peerVirtualIp, "100.64.0.2")
  }

  func testNativeHandlePrefersSessionRuntimeSnapshot() throws {
    let session = FakeWireGuardKitAdapterNativeHandleSession()
    session.snapshot = WireGuardBackendRuntimeSnapshot(
      backendName: "wireguardkit-native",
      backendState: "session-running",
      lastBackendError: nil,
      lastStartedAtMs: 888,
      peerVirtualIp: "100.64.0.55",
      selectedEndpoint: "198.51.100.55:51820"
    )
    let handle = WireGuardKitAdapterNativeHandle(
      descriptor: WireGuardKitAdapterSessionHandleDescriptorMapper.map(try makeWireGuardKitAdapterSessionConfiguration()),
      startedAtMs: 77,
      session: session,
      runtimeHandle: FakeWireGuardKitTerminalRuntimeHandle()
    )

    let snapshot = handle.runtimeSnapshot()

    XCTAssertEqual(snapshot?.backendState, "session-running")
    XCTAssertEqual(snapshot?.lastStartedAtMs, 888)
    XCTAssertEqual(snapshot?.peerVirtualIp, "100.64.0.55")
    XCTAssertEqual(snapshot?.selectedEndpoint, "198.51.100.55:51820")
  }

  func testNativeHandlePrefersTerminalRuntimeSnapshotWhenSessionMissing() throws {
    let runtimeHandle = FakeWireGuardKitTerminalRuntimeHandle()
    runtimeHandle.snapshot = WireGuardBackendRuntimeSnapshot(
      backendName: "wireguardkit-native",
      backendState: "terminal-runtime-running",
      lastBackendError: nil,
      lastStartedAtMs: 2027,
      peerVirtualIp: "100.64.0.98",
      selectedEndpoint: "198.51.100.98:51820"
    )
    let handle = WireGuardKitAdapterNativeHandle(
      descriptor: WireGuardKitAdapterSessionHandleDescriptorMapper.map(try makeWireGuardKitAdapterSessionConfiguration()),
      startedAtMs: 82,
      session: FakeWireGuardKitAdapterNativeHandleSession(),
      runtimeHandle: runtimeHandle
    )

    let snapshot = handle.runtimeSnapshot()

    XCTAssertEqual(snapshot?.backendState, "terminal-runtime-running")
    XCTAssertEqual(snapshot?.lastStartedAtMs, 2027)
    XCTAssertEqual(snapshot?.peerVirtualIp, "100.64.0.98")
    XCTAssertEqual(snapshot?.selectedEndpoint, "198.51.100.98:51820")
  }

  func testNativeHandleFactoryDelegatesToCreator() throws {
    let creator = FakeWireGuardKitAdapterNativeHandleCreator()
    let factory = WireGuardKitAdapterNativeHandleFactory(creator: creator)

    let handle = try factory.makeHandle(
      descriptor: WireGuardKitAdapterSessionHandleDescriptorMapper.map(try makeWireGuardKitAdapterSessionConfiguration()),
      startedAtMs: 321
    )

    XCTAssertTrue(handle === creator.providedHandle)
    XCTAssertEqual(creator.startedAtMs, 321)
    XCTAssertEqual(creator.descriptor?.peer.address, "100.64.0.2")
  }

  func testHandleCreatorDelegatesToNativeHandleCreator() throws {
    let nativeCreator = FakeWireGuardKitAdapterNativeHandleCreator()
    let nativeFactory = WireGuardKitAdapterNativeHandleFactory(creator: nativeCreator)
    let creator = WireGuardKitAdapterSessionHandleCreator(nativeHandleFactory: nativeFactory)

    let handle = try creator.makeHandle(
      descriptor: WireGuardKitAdapterSessionHandleDescriptorMapper.map(try makeWireGuardKitAdapterSessionConfiguration()),
      startedAtMs: 123
    )

    XCTAssertEqual(nativeCreator.startedAtMs, 123)
    XCTAssertEqual(nativeCreator.descriptor?.backendName, "wireguardkit-native")
    XCTAssertTrue(handle is WireGuardKitAdapterSessionHandle)
  }

  func testHandlePrefersNativeRuntimeSnapshot() throws {
    let nativeHandle = FakeWireGuardKitAdapterNativeHandle()
    nativeHandle.snapshot = WireGuardBackendRuntimeSnapshot(
      backendName: "wireguardkit-native",
      backendState: "native-running",
      lastBackendError: nil,
      lastStartedAtMs: 777,
      peerVirtualIp: "100.64.0.99",
      selectedEndpoint: "198.51.100.10:51820"
    )
    let handle = WireGuardKitAdapterSessionHandle(
      descriptor: WireGuardKitAdapterSessionHandleDescriptorMapper.map(try makeWireGuardKitAdapterSessionConfiguration()),
      startedAtMs: 99,
      nativeHandle: nativeHandle
    )

    let snapshot = handle.runtimeSnapshot()

    XCTAssertEqual(snapshot?.backendState, "native-running")
    XCTAssertEqual(snapshot?.lastStartedAtMs, 777)
    XCTAssertEqual(snapshot?.peerVirtualIp, "100.64.0.99")
    XCTAssertEqual(snapshot?.selectedEndpoint, "198.51.100.10:51820")
  }

  func testHandleFactoryMapsConfigurationIntoDescriptorAndDelegatesToCreator() throws {
    let creator = FakeWireGuardKitAdapterSessionHandleCreator()
    let factory = WireGuardKitAdapterSessionHandleFactory(
      handleCreator: creator,
      nowMs: { 99 }
    )

    let handle = try factory.makeHandle(configuration: makeWireGuardKitAdapterSessionConfiguration())

    XCTAssertTrue(handle === creator.providedHandle)
    XCTAssertEqual(
      creator.descriptor,
      WireGuardKitAdapterSessionHandleDescriptor(
        backendName: "wireguardkit-native",
        frameworkPath: "/tmp/WireGuardKit.framework",
        moduleSource: "framework",
        interface: WireGuardKitAdapterInterfaceConfiguration(
          privateKey: "pending-wireguardkit-private-key",
          publicKey: "pending-wireguardkit-public-key",
          addresses: ["100.64.0.10"],
          dnsServers: ["1.1.1.1"],
          mtu: 1280,
          listenPort: nil
        ),
        peer: WireGuardKitAdapterPeerConfiguration(
          publicKey: "pending-wireguardkit-peer-public-key",
          address: "100.64.0.2",
          endpoint: "203.0.113.10:51820",
          allowedIps: ["100.64.0.2/32"],
          persistentKeepaliveSeconds: nil
        )
      )
    )
    XCTAssertEqual(creator.startedAtMs, 99)
  }

  func testLifecycleDelegatesToHandleFactory() throws {
    let factory = FakeWireGuardKitAdapterSessionHandleFactory()
    let lifecycleController = WireGuardKitAdapterSessionLifecycleController(handleFactory: factory)

    let handle = try lifecycleController.start(configuration: makeWireGuardKitAdapterSessionConfiguration())

    XCTAssertTrue(handle === factory.providedHandle)
    XCTAssertEqual(factory.configuration, try makeWireGuardKitAdapterSessionConfiguration())
  }

  func testDefaultCreatorProducesRunningBackedSession() throws {
    let creator = WireGuardKitAdapterSessionCreator()

    let session = try creator.makeSession(configuration: makeWireGuardKitAdapterSessionConfiguration())
    let snapshot = session.runtimeSnapshot()

    XCTAssertEqual(snapshot?.backendName, "wireguardkit-native")
    XCTAssertEqual(snapshot?.backendState, "running")
    XCTAssertNil(snapshot?.lastBackendError)
    XCTAssertEqual(snapshot?.peerVirtualIp, "100.64.0.2")
    XCTAssertEqual(snapshot?.selectedEndpoint, "203.0.113.10:51820")
  }

  func testCreatorDelegatesToLifecycleAndReturnsBackedSession() throws {
    let handle = FakeWireGuardKitAdapterSessionHandle()
    var capturedConfiguration: WireGuardKitAdapterSessionConfiguration?
    let lifecycleController = FakeWireGuardKitAdapterSessionLifecycleController(handle: handle) { configuration in
      capturedConfiguration = configuration
    }
    let creator = WireGuardKitAdapterSessionCreator(lifecycleController: lifecycleController)

    let resolvedSession = try creator.makeSession(configuration: makeWireGuardKitAdapterSessionConfiguration())

    XCTAssertTrue(resolvedSession is WireGuardKitAdapterBackedNativeSession)
    XCTAssertEqual(capturedConfiguration, try makeWireGuardKitAdapterSessionConfiguration())
    let snapshot = resolvedSession.runtimeSnapshot()
    XCTAssertEqual(snapshot?.backendName, "wireguardkit-native")
    XCTAssertEqual(snapshot?.backendState, "running")
    XCTAssertNil(snapshot?.lastBackendError)
    XCTAssertEqual(snapshot?.lastStartedAtMs, 42)
    XCTAssertEqual(snapshot?.peerVirtualIp, "100.64.0.2")
    XCTAssertEqual(snapshot?.selectedEndpoint, "203.0.113.10:51820")
  }

  func testBackedSessionDelegatesStopAndInboundPacketsToHandle() throws {
    let handle = FakeWireGuardKitAdapterSessionHandle()
    let session = WireGuardKitAdapterBackedNativeSession(handle: handle)

    let output = try session.handleInboundPackets([Data([1, 2, 3])], protocols: [NSNumber(value: 2)])
    session.stop()

    XCTAssertEqual(handle.inboundPackets.count, 1)
    XCTAssertEqual(handle.inboundProtocols, [NSNumber(value: 2)])
    XCTAssertTrue(output.outboundPackets.isEmpty)
    XCTAssertTrue(output.outboundProtocols.isEmpty)
    XCTAssertEqual(handle.stopCallCount, 1)
  }
}

private final class FakeWireGuardKitAdapterSessionHandleFactory: WireGuardKitAdapterSessionHandleFactorying {
  let providedHandle = FakeWireGuardKitAdapterSessionHandle()
  var configuration: WireGuardKitAdapterSessionConfiguration?

  func makeHandle(configuration: WireGuardKitAdapterSessionConfiguration) throws -> WireGuardKitAdapterSessionHandleControlling {
    self.configuration = configuration
    return providedHandle
  }
}

private final class FakeWireGuardKitAdapterSessionHandleCreator: WireGuardKitAdapterSessionHandleCreating {
  let providedHandle = FakeWireGuardKitAdapterSessionHandle()
  var descriptor: WireGuardKitAdapterSessionHandleDescriptor?
  var startedAtMs: Int64?

  func makeHandle(
    descriptor: WireGuardKitAdapterSessionHandleDescriptor,
    startedAtMs: Int64
  ) throws -> WireGuardKitAdapterSessionHandleControlling {
    self.descriptor = descriptor
    self.startedAtMs = startedAtMs
    return providedHandle
  }
}

private final class FakeWireGuardKitAdapterNativeHandleCreator: WireGuardKitAdapterNativeHandleCreating {
  let providedHandle = FakeWireGuardKitAdapterNativeHandle()
  var descriptor: WireGuardKitAdapterSessionHandleDescriptor?
  var startedAtMs: Int64?

  func makeHandle(
    descriptor: WireGuardKitAdapterSessionHandleDescriptor,
    startedAtMs: Int64
  ) throws -> WireGuardKitAdapterNativeHandleControlling {
    self.descriptor = descriptor
    self.startedAtMs = startedAtMs
    return providedHandle
  }
}

private final class FakeWireGuardKitAdapterNativeHandleSessionBuilder: WireGuardKitAdapterNativeHandleSessionBuilding {
  let providedSession = FakeWireGuardKitAdapterNativeHandleSession()
  var descriptor: WireGuardKitAdapterSessionHandleDescriptor?
  var startedAtMs: Int64?

  func makeSession(
    descriptor: WireGuardKitAdapterSessionHandleDescriptor,
    startedAtMs: Int64
  ) throws -> WireGuardKitAdapterNativeHandleSessioning {
    self.descriptor = descriptor
    self.startedAtMs = startedAtMs
    return providedSession
  }
}

private final class FakeWireGuardKitBackedSessionCreatorImpl: WireGuardKitAdapterBackedSessionCreating {
  let providedSession = FakeWireGuardKitAdapterNativeHandleSession()
  var configuration: WireGuardKitAdapterBackedSessionConfiguration?
  var startedAtMs: Int64?

  func makeSession(
    configuration: WireGuardKitAdapterBackedSessionConfiguration,
    startedAtMs: Int64
  ) throws -> WireGuardKitAdapterNativeHandleSessioning {
    self.configuration = configuration
    self.startedAtMs = startedAtMs
    return providedSession
  }
}

private final class FakeWireGuardKitDeepBackedRootHandleFactoryImpl: WireGuardKitAdapterBackedSessionHandleFactorying {
  let providedHandle = FakeWireGuardKitDeepBackedRootHandle()
  var configuration: WireGuardKitAdapterBackedSessionConfiguration?
  var startedAtMs: Int64?

  func makeHandle(
    configuration: WireGuardKitAdapterBackedSessionConfiguration,
    startedAtMs: Int64
  ) throws -> WireGuardKitAdapterBackedSessionHandleControlling {
    self.configuration = configuration
    self.startedAtMs = startedAtMs
    return providedHandle
  }
}

private final class FakeWireGuardKitDeepBackedRootHandleCreatorImpl: WireGuardKitAdapterBackedSessionHandleCreating {
  let providedHandle = FakeWireGuardKitDeepBackedRootHandle()
  var configuration: WireGuardKitAdapterBackedSessionHandleConfiguration?
  var startedAtMs: Int64?

  func makeHandle(
    configuration: WireGuardKitAdapterBackedSessionHandleConfiguration,
    startedAtMs: Int64
  ) throws -> WireGuardKitAdapterBackedSessionHandleControlling {
    self.configuration = configuration
    self.startedAtMs = startedAtMs
    return providedHandle
  }
}

private final class FakeWireGuardKitDeepBackedNativeHandleFactoryImpl:
  WireGuardKitAdapterBackedSessionNativeHandleFactorying
{
  let providedHandle = FakeWireGuardKitDeepBackedNativeHandle()
  var configuration: WireGuardKitAdapterBackedSessionHandleConfiguration?
  var startedAtMs: Int64?

  func makeHandle(
    configuration: WireGuardKitAdapterBackedSessionHandleConfiguration,
    startedAtMs: Int64
  ) throws -> WireGuardKitAdapterBackedSessionNativeHandleControlling {
    self.configuration = configuration
    self.startedAtMs = startedAtMs
    return providedHandle
  }
}

private final class FakeWireGuardKitDeepBackedNativeSessionFactoryImpl:
  WireGuardKitAdapterBackedSessionNativeHandleSessionFactorying
{
  let providedSession = FakeWireGuardKitDeepBackedNativeSession()
  var configuration: WireGuardKitAdapterBackedSessionNativeHandleConfiguration?
  var startedAtMs: Int64?

  func makeSession(
    configuration: WireGuardKitAdapterBackedSessionNativeHandleConfiguration,
    startedAtMs: Int64
  ) throws -> WireGuardKitAdapterBackedSessionNativeHandleSessioning {
    self.configuration = configuration
    self.startedAtMs = startedAtMs
    return providedSession
  }
}

private final class FakeWireGuardKitDeepBackedNativeSessionCreatorImpl:
  WireGuardKitAdapterBackedSessionNativeHandleSessionCreating
{
  let providedSession = FakeWireGuardKitDeepBackedNativeSession()
  var configuration: WireGuardKitAdapterBackedSessionNativeHandleSessionConfiguration?
  var startedAtMs: Int64?

  func makeSession(
    configuration: WireGuardKitAdapterBackedSessionNativeHandleSessionConfiguration,
    startedAtMs: Int64
  ) throws -> WireGuardKitAdapterBackedSessionNativeHandleSessioning {
    self.configuration = configuration
    self.startedAtMs = startedAtMs
    return providedSession
  }
}

private final class FakeWireGuardKitDeepAdapterSessionFactoryImpl:
  WireGuardKitAdapterBackedSessionAdapterHandleSessionFactorying
{
  let providedSession = FakeWireGuardKitDeepAdapterSession()
  var configuration: WireGuardKitAdapterBackedSessionAdapterHandleConfiguration?
  var startedAtMs: Int64?

  func makeSession(
    configuration: WireGuardKitAdapterBackedSessionAdapterHandleConfiguration,
    startedAtMs: Int64
  ) throws -> WireGuardKitAdapterBackedSessionAdapterHandleSessioning {
    self.configuration = configuration
    self.startedAtMs = startedAtMs
    return providedSession
  }
}

private final class FakeWireGuardKitDeepAdapterNativeHandleFactoryImpl:
  WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleFactorying
{
  let providedHandle = FakeWireGuardKitDeepAdapterNativeHandle()
  var configuration: WireGuardKitAdapterBackedSessionAdapterHandleSessionConfiguration?
  var startedAtMs: Int64?

  func makeHandle(
    configuration: WireGuardKitAdapterBackedSessionAdapterHandleSessionConfiguration,
    startedAtMs: Int64
  ) throws -> WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleControlling {
    self.configuration = configuration
    self.startedAtMs = startedAtMs
    return providedHandle
  }
}

private final class FakeWireGuardKitDeepNativeSessionFactoryImpl:
  WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionFactorying
{
  let providedSession = FakeWireGuardKitDeepNativeSession()
  var configuration: WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleConfiguration?
  var startedAtMs: Int64?

  func makeSession(
    configuration: WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleConfiguration,
    startedAtMs: Int64
  ) throws -> WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessioning {
    self.configuration = configuration
    self.startedAtMs = startedAtMs
    return providedSession
  }
}

private final class FakeWireGuardKitDeepNativeSessionCreatorImpl:
  WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionCreating
{
  let providedSession = FakeWireGuardKitDeepNativeSession()
  var configuration: WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionConfiguration?
  var startedAtMs: Int64?

  func makeSession(
    configuration: WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionConfiguration,
    startedAtMs: Int64
  ) throws -> WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessioning {
    self.configuration = configuration
    self.startedAtMs = startedAtMs
    return providedSession
  }
}

private final class FakeWireGuardKitDeepRuntimeBridgeHandleCreatorImpl:
  WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleCreating
{
  let providedHandle = FakeWireGuardKitDeepRuntimeBridgeHandle()
  var configuration: WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleConfiguration?
  var startedAtMs: Int64?

  func makeHandle(
    configuration: WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleConfiguration,
    startedAtMs: Int64
  ) throws -> WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleControlling {
    self.configuration = configuration
    self.startedAtMs = startedAtMs
    return providedHandle
  }
}

private final class FakeWireGuardKitDeepRuntimeHandleCreatorImpl:
  WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleCreating
{
  let providedHandle =
    FakeWireGuardKitDeepRuntimeHandle()
  var configuration:
    WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleConfiguration?
  var startedAtMs: Int64?

  func makeHandle(
    configuration:
      WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleConfiguration,
    startedAtMs: Int64
  ) throws -> WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleControlling {
    self.configuration = configuration
    self.startedAtMs = startedAtMs
    return providedHandle
  }
}

private final class FakeWireGuardKitDeepRuntimeSessionCreatorImpl:
  WireGuardKitDeepRuntimeSessionCreating
{
  let providedSession = FakeWireGuardKitDeepRuntimeSession()
  var configuration: WireGuardKitDeepRuntimeSessionConfiguration?
  var startedAtMs: Int64?

  func makeSession(
    configuration: WireGuardKitDeepRuntimeSessionConfiguration,
    startedAtMs: Int64
  ) throws -> WireGuardKitDeepRuntimeSessioning {
    self.configuration = configuration
    self.startedAtMs = startedAtMs
    return providedSession
  }
}

private final class FakeWireGuardKitDeepRuntimeSessionFactoryImpl:
  WireGuardKitDeepRuntimeSessionFactorying
{
  let providedSession = FakeWireGuardKitDeepRuntimeSession()
  var configuration:
    WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleConfiguration?
  var startedAtMs: Int64?

  func makeSession(
    configuration:
      WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleConfiguration,
    startedAtMs: Int64
  ) throws -> WireGuardKitDeepRuntimeSessioning {
    self.configuration = configuration
    self.startedAtMs = startedAtMs
    return providedSession
  }
}

private final class FakeWireGuardKitDeepBackedSessionCreatorImpl:
  WireGuardKitDeepBackedSessionCreating
{
  let providedSession = FakeWireGuardKitDeepBackedSession()
  var configuration: WireGuardKitDeepBackedSessionConfiguration?
  var startedAtMs: Int64?

  func makeSession(
    configuration:
      WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleSessionBackedSessionConfiguration,
    startedAtMs: Int64
  ) throws -> WireGuardKitDeepBackedSessioning {
    self.configuration = configuration
    self.startedAtMs = startedAtMs
    return providedSession
  }
}

private final class FakeWireGuardKitDeepBackedSessionFactoryImpl:
  WireGuardKitDeepBackedSessionFactorying
{
  let providedSession = FakeWireGuardKitDeepBackedSession()
  var configuration: WireGuardKitDeepRuntimeSessionConfiguration?
  var startedAtMs: Int64?

  func makeSession(
    configuration:
      WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleSessionConfiguration,
    startedAtMs: Int64
  ) throws -> WireGuardKitDeepBackedSessioning {
    self.configuration = configuration
    self.startedAtMs = startedAtMs
    return providedSession
  }
}

private final class FakeWireGuardKitDeepBackedHandleCreatorImpl:
  WireGuardKitDeepBackedHandleCreating
{
  let providedHandle = FakeWireGuardKitDeepBackedHandle()
  var configuration: WireGuardKitDeepBackedHandleConfiguration?
  var startedAtMs: Int64?

  func makeHandle(
    configuration:
      WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleSessionBackedSessionHandleConfiguration,
    startedAtMs: Int64
  ) throws -> WireGuardKitDeepBackedHandleControlling {
    self.configuration = configuration
    self.startedAtMs = startedAtMs
    return providedHandle
  }
}

private final class FakeWireGuardKitDeepBackedHandleFactoryImpl:
  WireGuardKitDeepBackedHandleFactorying
{
  let providedHandle = FakeWireGuardKitDeepBackedHandle()
  var configuration: WireGuardKitDeepBackedSessionConfiguration?
  var startedAtMs: Int64?

  func makeHandle(
    configuration:
      WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleSessionBackedSessionConfiguration,
    startedAtMs: Int64
  ) throws -> WireGuardKitDeepBackedHandleControlling {
    self.configuration = configuration
    self.startedAtMs = startedAtMs
    return providedHandle
  }
}

private final class FakeWireGuardKitDeepBackedHandleSessionCreatorImpl:
  WireGuardKitDeepBackedHandleSessionCreating
{
  let providedSession = FakeWireGuardKitDeepBackedHandleSession()
  var configuration: WireGuardKitDeepBackedHandleSessionConfiguration?
  var startedAtMs: Int64?

  func makeSession(
    configuration:
      WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleSessionBackedSessionHandleSessionConfiguration,
    startedAtMs: Int64
  ) throws -> WireGuardKitDeepBackedHandleSessioning {
    self.configuration = configuration
    self.startedAtMs = startedAtMs
    return providedSession
  }
}

private final class FakeWireGuardKitDeepBackedHandleSessionFactoryImpl:
  WireGuardKitDeepBackedHandleSessionFactorying
{
  let providedSession = FakeWireGuardKitDeepBackedHandleSession()
  var configuration: WireGuardKitDeepBackedHandleConfiguration?
  var startedAtMs: Int64?

  func makeSession(
    configuration:
      WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleSessionBackedSessionHandleConfiguration,
    startedAtMs: Int64
  ) throws -> WireGuardKitDeepBackedHandleSessioning {
    self.configuration = configuration
    self.startedAtMs = startedAtMs
    return providedSession
  }
}

private final class FakeWireGuardKitDeepLeafHandleCreatorImpl:
  WireGuardKitDeepLeafHandleCreating
{
  let providedHandle = FakeWireGuardKitDeepLeafHandle()
  var configuration: WireGuardKitDeepLeafHandleConfiguration?
  var startedAtMs: Int64?

  func makeHandle(
    configuration:
      WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleSessionBackedSessionHandleSessionHandleConfiguration,
    startedAtMs: Int64
  ) throws -> WireGuardKitDeepLeafHandleControlling {
    self.configuration = configuration
    self.startedAtMs = startedAtMs
    return providedHandle
  }
}

private final class FakeWireGuardKitDeepLeafHandleFactoryImpl:
  WireGuardKitDeepLeafHandleFactorying
{
  let providedHandle = FakeWireGuardKitDeepLeafHandle()
  var configuration: WireGuardKitDeepBackedHandleSessionConfiguration?
  var startedAtMs: Int64?

  func makeHandle(
    configuration:
      WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleSessionBackedSessionHandleSessionConfiguration,
    startedAtMs: Int64
  ) throws
    -> WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleSessionBackedSessionHandleSessionHandleControlling
  {
    self.configuration = configuration
    self.startedAtMs = startedAtMs
    return providedHandle
  }
}

private final class FakeWireGuardKitTerminalRuntimeHandleCreator: WireGuardKitTerminalRuntimeHandleCreating {
  let providedHandle = FakeWireGuardKitTerminalRuntimeHandle()
  var configuration: WireGuardKitTerminalRuntimeHandleConfiguration?
  var startedAtMs: Int64?

  func makeHandle(
    configuration: WireGuardKitTerminalRuntimeHandleConfiguration,
    startedAtMs: Int64
  ) throws -> WireGuardKitTerminalRuntimeHandleControlling {
    self.configuration = configuration
    self.startedAtMs = startedAtMs
    return providedHandle
  }
}

private final class FakeWireGuardKitDeepAdapterHandleCreatorImpl:
  WireGuardKitAdapterBackedSessionAdapterHandleCreating
{
  let providedHandle = FakeWireGuardKitDeepAdapterHandle()
  var configuration: WireGuardKitAdapterBackedSessionAdapterHandleConfiguration?
  var startedAtMs: Int64?

  func makeHandle(
    configuration: WireGuardKitAdapterBackedSessionAdapterHandleConfiguration,
    startedAtMs: Int64
  ) throws -> WireGuardKitAdapterBackedSessionAdapterHandleControlling {
    self.configuration = configuration
    self.startedAtMs = startedAtMs
    return providedHandle
  }
}

private final class FakeWireGuardKitDeepBackedRootHandleImpl: WireGuardKitAdapterBackedSessionHandleControlling {
  var stopCallCount = 0
  var inboundPackets: [Data] = []
  var inboundProtocols: [NSNumber] = []
  var snapshot: WireGuardBackendRuntimeSnapshot?

  func stop() {
    stopCallCount += 1
  }

  func handleInboundPackets(_ packets: [Data], protocols: [NSNumber]) throws -> PacketTunnelEngineOutput {
    inboundPackets = packets
    inboundProtocols = protocols
    return .empty
  }

  func runtimeSnapshot() -> WireGuardBackendRuntimeSnapshot? {
    snapshot
  }
}

private final class FakeWireGuardKitDeepAdapterSessionImpl:
  WireGuardKitAdapterBackedSessionAdapterHandleSessioning
{
  var stopCallCount = 0
  var inboundPackets: [Data] = []
  var inboundProtocols: [NSNumber] = []
  var snapshot: WireGuardBackendRuntimeSnapshot?

  func stop() {
    stopCallCount += 1
  }

  func handleInboundPackets(_ packets: [Data], protocols: [NSNumber]) throws -> PacketTunnelEngineOutput {
    inboundPackets = packets
    inboundProtocols = protocols
    return .empty
  }

  func runtimeSnapshot() -> WireGuardBackendRuntimeSnapshot? {
    snapshot
  }
}

private final class FakeWireGuardKitDeepAdapterNativeHandleImpl:
  WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleControlling
{
  var stopCallCount = 0
  var inboundPackets: [Data] = []
  var inboundProtocols: [NSNumber] = []
  var snapshot: WireGuardBackendRuntimeSnapshot?

  func stop() {
    stopCallCount += 1
  }

  func handleInboundPackets(_ packets: [Data], protocols: [NSNumber]) throws -> PacketTunnelEngineOutput {
    inboundPackets = packets
    inboundProtocols = protocols
    return .empty
  }

  func runtimeSnapshot() -> WireGuardBackendRuntimeSnapshot? {
    snapshot
  }
}

private final class FakeWireGuardKitDeepNativeSessionImpl:
  WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessioning
{
  var stopCallCount = 0
  var inboundPackets: [Data] = []
  var inboundProtocols: [NSNumber] = []
  var snapshot: WireGuardBackendRuntimeSnapshot?

  func stop() {
    stopCallCount += 1
  }

  func handleInboundPackets(_ packets: [Data], protocols: [NSNumber]) throws -> PacketTunnelEngineOutput {
    inboundPackets = packets
    inboundProtocols = protocols
    return .empty
  }

  func runtimeSnapshot() -> WireGuardBackendRuntimeSnapshot? {
    snapshot
  }
}

private final class FakeWireGuardKitDeepRuntimeBridgeHandleImpl:
  WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleControlling
{
  var stopCallCount = 0
  var inboundPackets: [Data] = []
  var inboundProtocols: [NSNumber] = []
  var snapshot: WireGuardBackendRuntimeSnapshot?

  func stop() {
    stopCallCount += 1
  }

  func handleInboundPackets(_ packets: [Data], protocols: [NSNumber]) throws -> PacketTunnelEngineOutput {
    inboundPackets = packets
    inboundProtocols = protocols
    return .empty
  }

  func runtimeSnapshot() -> WireGuardBackendRuntimeSnapshot? {
    snapshot
  }
}

private final class FakeWireGuardKitDeepRuntimeHandleImpl:
  WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleControlling
{
  var stopCallCount = 0
  var inboundPackets: [Data] = []
  var inboundProtocols: [NSNumber] = []
  var snapshot: WireGuardBackendRuntimeSnapshot?

  func stop() {
    stopCallCount += 1
  }

  func handleInboundPackets(_ packets: [Data], protocols: [NSNumber]) throws -> PacketTunnelEngineOutput {
    inboundPackets = packets
    inboundProtocols = protocols
    return .empty
  }

  func runtimeSnapshot() -> WireGuardBackendRuntimeSnapshot? {
    snapshot
  }
}

private final class FakeWireGuardKitDeepRuntimeSessionImpl:
  WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleSessioning
{
  var stopCallCount = 0
  var inboundPackets: [Data] = []
  var inboundProtocols: [NSNumber] = []
  var snapshot: WireGuardBackendRuntimeSnapshot?

  func stop() {
    stopCallCount += 1
  }

  func handleInboundPackets(_ packets: [Data], protocols: [NSNumber]) throws -> PacketTunnelEngineOutput {
    inboundPackets = packets
    inboundProtocols = protocols
    return .empty
  }

  func runtimeSnapshot() -> WireGuardBackendRuntimeSnapshot? {
    snapshot
  }
}

private final class FakeWireGuardKitDeepBackedSessionImpl:
  WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleSessionBackedSessioning
{
  var stopCallCount = 0
  var inboundPackets: [Data] = []
  var inboundProtocols: [NSNumber] = []
  var snapshot: WireGuardBackendRuntimeSnapshot?

  func stop() {
    stopCallCount += 1
  }

  func handleInboundPackets(_ packets: [Data], protocols: [NSNumber]) throws -> PacketTunnelEngineOutput {
    inboundPackets = packets
    inboundProtocols = protocols
    return .empty
  }

  func runtimeSnapshot() -> WireGuardBackendRuntimeSnapshot? {
    snapshot
  }
}

private final class FakeWireGuardKitDeepBackedHandleImpl:
  WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleSessionBackedSessionHandleControlling
{
  var stopCallCount = 0
  var inboundPackets: [Data] = []
  var inboundProtocols: [NSNumber] = []
  var snapshot: WireGuardBackendRuntimeSnapshot?

  func stop() {
    stopCallCount += 1
  }

  func handleInboundPackets(_ packets: [Data], protocols: [NSNumber]) throws -> PacketTunnelEngineOutput {
    inboundPackets = packets
    inboundProtocols = protocols
    return .empty
  }

  func runtimeSnapshot() -> WireGuardBackendRuntimeSnapshot? {
    snapshot
  }
}

private final class FakeWireGuardKitDeepBackedHandleSessionImpl:
  WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleSessionBackedSessionHandleSessioning
{
  var stopCallCount = 0
  var inboundPackets: [Data] = []
  var inboundProtocols: [NSNumber] = []
  var snapshot: WireGuardBackendRuntimeSnapshot?

  func stop() {
    stopCallCount += 1
  }

  func handleInboundPackets(_ packets: [Data], protocols: [NSNumber]) throws -> PacketTunnelEngineOutput {
    inboundPackets = packets
    inboundProtocols = protocols
    return .empty
  }

  func runtimeSnapshot() -> WireGuardBackendRuntimeSnapshot? {
    snapshot
  }
}

private final class FakeWireGuardKitDeepLeafHandleImpl:
  WireGuardKitAdapterBackedSessionAdapterHandleNativeHandleSessionHandleRuntimeHandleSessionBackedSessionHandleSessionHandleControlling
{
  var stopCallCount = 0
  var inboundPackets: [Data] = []
  var inboundProtocols: [NSNumber] = []
  var snapshot: WireGuardBackendRuntimeSnapshot?

  func stop() {
    stopCallCount += 1
  }

  func handleInboundPackets(_ packets: [Data], protocols: [NSNumber]) throws -> PacketTunnelEngineOutput {
    inboundPackets = packets
    inboundProtocols = protocols
    return .empty
  }

  func runtimeSnapshot() -> WireGuardBackendRuntimeSnapshot? {
    snapshot
  }
}

private final class FakeWireGuardKitTerminalRuntimeHandle: WireGuardKitTerminalRuntimeHandleControlling {
  var stopCallCount = 0
  var inboundPackets: [Data] = []
  var inboundProtocols: [NSNumber] = []
  var snapshot: WireGuardBackendRuntimeSnapshot?

  func stop() {
    stopCallCount += 1
  }

  func handleInboundPackets(_ packets: [Data], protocols: [NSNumber]) throws -> PacketTunnelEngineOutput {
    inboundPackets = packets
    inboundProtocols = protocols
    return .empty
  }

  func runtimeSnapshot() -> WireGuardBackendRuntimeSnapshot? {
    snapshot
  }
}

private final class FakeWireGuardKitDeepBackedNativeHandleImpl:
  WireGuardKitAdapterBackedSessionNativeHandleControlling
{
  var stopCallCount = 0
  var inboundPackets: [Data] = []
  var inboundProtocols: [NSNumber] = []
  var snapshot: WireGuardBackendRuntimeSnapshot?

  func stop() {
    stopCallCount += 1
  }

  func handleInboundPackets(_ packets: [Data], protocols: [NSNumber]) throws -> PacketTunnelEngineOutput {
    inboundPackets = packets
    inboundProtocols = protocols
    return .empty
  }

  func runtimeSnapshot() -> WireGuardBackendRuntimeSnapshot? {
    snapshot
  }
}

private final class FakeWireGuardKitDeepBackedNativeSessionImpl:
  WireGuardKitAdapterBackedSessionNativeHandleSessioning
{
  var stopCallCount = 0
  var inboundPackets: [Data] = []
  var inboundProtocols: [NSNumber] = []
  var snapshot: WireGuardBackendRuntimeSnapshot?

  func stop() {
    stopCallCount += 1
  }

  func handleInboundPackets(_ packets: [Data], protocols: [NSNumber]) throws -> PacketTunnelEngineOutput {
    inboundPackets = packets
    inboundProtocols = protocols
    return .empty
  }

  func runtimeSnapshot() -> WireGuardBackendRuntimeSnapshot? {
    snapshot
  }
}

private final class FakeWireGuardKitDeepAdapterHandleImpl:
  WireGuardKitAdapterBackedSessionAdapterHandleControlling
{
  var stopCallCount = 0
  var inboundPackets: [Data] = []
  var inboundProtocols: [NSNumber] = []
  var snapshot: WireGuardBackendRuntimeSnapshot?

  func stop() {
    stopCallCount += 1
  }

  func handleInboundPackets(_ packets: [Data], protocols: [NSNumber]) throws -> PacketTunnelEngineOutput {
    inboundPackets = packets
    inboundProtocols = protocols
    return .empty
  }

  func runtimeSnapshot() -> WireGuardBackendRuntimeSnapshot? {
    snapshot
  }
}

private final class FakeWireGuardKitAdapterSessionLifecycleController: WireGuardKitAdapterSessionLifecycleControlling {
  private let providedHandle: WireGuardKitAdapterSessionHandleControlling
  private let onConfiguration: (WireGuardKitAdapterSessionConfiguration) -> Void

  init(
    handle: WireGuardKitAdapterSessionHandleControlling,
    onConfiguration: @escaping (WireGuardKitAdapterSessionConfiguration) -> Void = { _ in }
  ) {
    self.providedHandle = handle
    self.onConfiguration = onConfiguration
  }

  func start(configuration: WireGuardKitAdapterSessionConfiguration) throws -> WireGuardKitAdapterSessionHandleControlling {
    onConfiguration(configuration)
    return providedHandle
  }
}

private final class FakeWireGuardKitAdapterSessionHandle: WireGuardKitAdapterSessionHandleControlling {
  var stopCallCount = 0
  var inboundPackets: [Data] = []
  var inboundProtocols: [NSNumber] = []

  func stop() {
    stopCallCount += 1
  }

  func handleInboundPackets(_ packets: [Data], protocols: [NSNumber]) throws -> PacketTunnelEngineOutput {
    inboundPackets = packets
    inboundProtocols = protocols
    return .empty
  }

  func runtimeSnapshot() -> WireGuardBackendRuntimeSnapshot? {
    WireGuardBackendRuntimeSnapshot(
      backendName: "wireguardkit-native",
      backendState: "running",
      lastBackendError: nil,
      lastStartedAtMs: 42,
      peerVirtualIp: "100.64.0.2",
      selectedEndpoint: "203.0.113.10:51820"
    )
  }
}

private final class FakeWireGuardKitAdapterNativeHandle: WireGuardKitAdapterNativeHandleControlling {
  var stopCallCount = 0
  var inboundPackets: [Data] = []
  var inboundProtocols: [NSNumber] = []
  var snapshot: WireGuardBackendRuntimeSnapshot?

  func stop() {
    stopCallCount += 1
  }

  func handleInboundPackets(_ packets: [Data], protocols: [NSNumber]) throws -> PacketTunnelEngineOutput {
    inboundPackets = packets
    inboundProtocols = protocols
    return .empty
  }

  func runtimeSnapshot() -> WireGuardBackendRuntimeSnapshot? {
    snapshot
  }
}

private final class FakeWireGuardKitAdapterNativeHandleSession: WireGuardKitAdapterNativeHandleSessioning {
  var stopCallCount = 0
  var inboundPackets: [Data] = []
  var inboundProtocols: [NSNumber] = []
  var snapshot: WireGuardBackendRuntimeSnapshot?

  func stop() {
    stopCallCount += 1
  }

  func handleInboundPackets(_ packets: [Data], protocols: [NSNumber]) throws -> PacketTunnelEngineOutput {
    inboundPackets = packets
    inboundProtocols = protocols
    return .empty
  }

  func runtimeSnapshot() -> WireGuardBackendRuntimeSnapshot? {
    snapshot
  }
}
