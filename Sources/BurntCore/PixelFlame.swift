import Foundation
import AppKit

/// Animates the original menu-bar flame — the `flame.fill` SF Symbol — by cycling
/// frames that stretch/squash, sway, and rock the icon, so it's the exact same
/// familiar flame, now dancing. Rendered as a TEMPLATE image so macOS tints it to
/// the menu bar (white on dark, black on light) like before.
public enum PixelFlame {
    public static var frameCount: Int { flicker.count }

    /// Per-frame (verticalScale, horizontalScale, swayPts, rotationDegrees).
    /// Bigger values = a livelier dancing flame: it stretches/squashes, leans, and
    /// rocks side to side.
    private static let flicker: [(vScale: CGFloat, hScale: CGFloat, dx: CGFloat, rot: CGFloat)] = [
        (1.00, 1.00,  0.0,  0),
        (1.16, 0.92,  1.2,  6),
        (0.90, 1.10, -1.0, -5),
        (1.20, 0.90,  1.6,  8),
        (0.94, 1.06, -1.4, -7),
        (1.10, 0.96,  0.6,  3),
    ]

    /// The 6 frames are fully deterministic, so render each one exactly once and
    /// hand back the cached `NSImage` on every subsequent tick. Without this the
    /// menu bar re-rasterizes an SF Symbol ~6×/second for the life of the process.
    /// Keyed by `pointHeight` so a non-default size still gets its own cached set.
    private static let cacheLock = NSLock()
    private nonisolated(unsafe) static var cache: [CGFloat: [NSImage]] = [:]

    /// Render the flame symbol for `index`, at `pointHeight` (menu bar ≈ 18pt).
    public static func image(frame index: Int, pointHeight: CGFloat = 18) -> NSImage {
        let i = index % flicker.count
        cacheLock.lock()
        if let frames = cache[pointHeight] {
            cacheLock.unlock()
            return frames[i]
        }
        cacheLock.unlock()

        let frames = (0..<flicker.count).map { render(frame: $0, pointHeight: pointHeight) }
        cacheLock.lock()
        cache[pointHeight] = frames
        cacheLock.unlock()
        return frames[i]
    }

    /// Rasterize a single frame. Called once per (frame, pointHeight); result cached.
    private static func render(frame index: Int, pointHeight: CGFloat) -> NSImage {
        let f = flicker[index % flicker.count]
        let h = pointHeight
        // Wider canvas so sway + rotation don't clip the flame.
        let w = pointHeight * 1.3

        let cfg = NSImage.SymbolConfiguration(pointSize: h * 0.92, weight: .semibold)
        let base = NSImage(systemSymbolName: "flame.fill", accessibilityDescription: "flame")?
            .withSymbolConfiguration(cfg) ?? NSImage()
        let bs = base.size
        guard bs.width > 0, bs.height > 0 else { return base }

        let canvas = NSImage(size: NSSize(width: w, height: h))
        canvas.lockFocus()
        let ctx = NSGraphicsContext.current!.cgContext
        ctx.interpolationQuality = .high

        let drawW = min(bs.width, w) * f.hScale
        let drawH = bs.height * f.vScale

        // Rock about a pivot near the flame's base-center so the tip swings most.
        let pivotX = w / 2 + f.dx
        let pivotY = h * 0.30
        ctx.saveGState()
        ctx.translateBy(x: pivotX, y: pivotY)
        ctx.rotate(by: f.rot * .pi / 180)
        ctx.translateBy(x: -drawW / 2, y: -pivotY)
        base.draw(in: NSRect(x: 0, y: 0, width: drawW, height: drawH),
                  from: .zero, operation: .sourceOver, fraction: 1)
        ctx.restoreGState()

        canvas.unlockFocus()
        canvas.isTemplate = true
        return canvas
    }
}
