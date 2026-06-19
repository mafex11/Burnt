import XCTest
import AppKit
@testable import BurntCore

final class PixelFlameTests: XCTestCase {
    func testHasMultipleFrames() {
        XCTAssertGreaterThanOrEqual(PixelFlame.frameCount, 2)
    }

    func testRendersNonEmptyImagePerFrame() {
        for i in 0..<PixelFlame.frameCount {
            let img = PixelFlame.image(frame: i, pointHeight: 18)
            XCTAssertGreaterThan(img.size.width, 0)
            XCTAssertGreaterThan(img.size.height, 0)
        }
    }
}
