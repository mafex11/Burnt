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

    /// Frames are deterministic and cached, so repeated calls must hand back the
    /// SAME NSImage instance rather than re-rasterizing the SF Symbol each tick.
    func testCachesFrameInstances() {
        let a = PixelFlame.image(frame: 1, pointHeight: 18)
        let b = PixelFlame.image(frame: 1, pointHeight: 18)
        XCTAssertTrue(a === b, "expected the cached image instance to be reused")
    }

    /// Out-of-range indices wrap, so animation can advance an unbounded counter.
    func testWrapsFrameIndex() {
        let wrapped = PixelFlame.image(frame: PixelFlame.frameCount, pointHeight: 18)
        let zero = PixelFlame.image(frame: 0, pointHeight: 18)
        XCTAssertTrue(wrapped === zero)
    }
}
