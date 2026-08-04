import Cocoa
import FlutterMacOS
import XCTest
@testable import slan_client_v2

class RunnerTests: XCTestCase {

  func testKeyUpEventGuardForwardsOneBalancedKeySequence() {
    let guardState = KeyUpEventGuard()

    XCTAssertTrue(guardState.shouldForward(type: .keyDown, keyCode: 101))
    XCTAssertTrue(guardState.shouldForward(type: .keyUp, keyCode: 101))
  }

  func testKeyUpEventGuardDropsRepeatedKeyUp() {
    let guardState = KeyUpEventGuard()

    XCTAssertTrue(guardState.shouldForward(type: .keyDown, keyCode: 101))
    XCTAssertTrue(guardState.shouldForward(type: .keyUp, keyCode: 101))
    XCTAssertFalse(guardState.shouldForward(type: .keyUp, keyCode: 101))
  }

  func testKeyUpEventGuardDropsKeyUpAfterFocusReset() {
    let guardState = KeyUpEventGuard()

    XCTAssertTrue(guardState.shouldForward(type: .keyDown, keyCode: 101))
    guardState.reset()

    XCTAssertFalse(guardState.shouldForward(type: .keyUp, keyCode: 101))
  }

}
