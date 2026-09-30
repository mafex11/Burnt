import Foundation

/// Decides when the menu-bar flame should flare: only when spend actually rises.
/// Fed one reading per poll; pure so the rule is unit-testable without timers.
public struct SpendWatcher: Sendable {
    /// Changes smaller than this are rounding noise between polls, not new spend.
    public static let minimumRise = 0.005

    private var lastCost: Double?

    public init() {}

    /// Records `cost` and reports whether it rose since the previous reading.
    /// The first reading only sets the baseline (no flare on launch), and a drop
    /// (e.g. today's cost resetting at midnight) just moves the baseline down.
    public mutating func observe(cost: Double) -> Bool {
        defer { lastCost = cost }
        guard let last = lastCost else { return false }
        return cost - last >= Self.minimumRise
    }
}
