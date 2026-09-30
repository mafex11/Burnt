import Foundation
import SwiftUI
import BurntCore

/// Owns the menu-bar flame's frame index. Kept separate from `AppModel` so a
/// frame tick invalidates only `MenuBarLabel`, not the whole scene and popover.
/// Rests on frame 0 and flickers only for a short burst when `flare()` is called.
@MainActor
final class FlameAnimator: ObservableObject {
    @Published private(set) var frame = 0

    /// ~6fps, three full flicker cycles (~3s) per flare.
    private static let frameInterval: TimeInterval = 0.16
    private static let flareFrames = PixelFlame.frameCount * 3

    private var timer: Timer?
    private var framesLeft = 0

    /// Flicker for one burst. A flare while one is running restarts the burst
    /// rather than stacking timers.
    func flare() {
        framesLeft = Self.flareFrames
        guard timer == nil else { return }
        timer = Timer.scheduledTimer(withTimeInterval: Self.frameInterval, repeats: true) { [weak self] _ in
            Task { @MainActor in self?.tick() }
        }
    }

    /// Stop immediately and hold the resting frame.
    func stop() {
        timer?.invalidate()
        timer = nil
        framesLeft = 0
        if frame != 0 { frame = 0 }
    }

    private func tick() {
        guard framesLeft > 0 else { stop(); return }
        framesLeft -= 1
        frame = (frame + 1) % PixelFlame.frameCount
    }
}
