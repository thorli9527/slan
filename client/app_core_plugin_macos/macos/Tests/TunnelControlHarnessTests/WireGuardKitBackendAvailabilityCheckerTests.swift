import XCTest
@testable import TunnelControlHarness

final class WireGuardKitBackendAvailabilityCheckerTests: XCTestCase {
  func testEnvironmentCanForceBackendAvailable() {
    let checker = DefaultWireGuardKitBackendAvailabilityChecker(
      environment: ["SLAN_WIREGUARDKIT_BACKEND_AVAILABLE": "true"],
      fileExistsAtPath: { _ in false }
    )

    let availability = checker.availability()

    XCTAssertTrue(availability.isAvailable)
    XCTAssertNil(availability.reason)
  }

  func testEnvironmentCanForceBackendUnavailable() {
    let checker = DefaultWireGuardKitBackendAvailabilityChecker(
      environment: ["SLAN_WIREGUARDKIT_BACKEND_UNAVAILABLE": "1"],
      fileExistsAtPath: { _ in true }
    )

    let availability = checker.availability()

    XCTAssertFalse(availability.isAvailable)
    XCTAssertEqual(availability.reason, "WireGuardKit backend disabled by environment override")
  }

  func testFrameworkOverridePathMarksBackendAvailable() {
    let expectedPath = "/tmp/WireGuardKit.framework"
    let checker = DefaultWireGuardKitBackendAvailabilityChecker(
      environment: ["SLAN_WIREGUARDKIT_FRAMEWORK_PATH": expectedPath],
      fileExistsAtPath: { path in path == expectedPath }
    )

    let availability = checker.availability()

    XCTAssertTrue(availability.isAvailable)
    XCTAssertNil(availability.reason)
  }

  func testMissingFrameworkReturnsStableReason() {
    let checker = DefaultWireGuardKitBackendAvailabilityChecker(
      environment: [:],
      fileExistsAtPath: { _ in false }
    )

    let availability = checker.availability()

    XCTAssertFalse(availability.isAvailable)
    XCTAssertEqual(availability.reason, "WireGuardKit framework is not linked into the app bundle")
  }
}
