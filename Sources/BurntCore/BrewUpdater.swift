// Sources/BurntCore/BrewUpdater.swift
import Foundation

/// Drives updates through Homebrew so brew stays the single source of truth — Burnt
/// never rewrites its own bytes, it asks `brew` to replace the cask. Detection guards
/// against running brew for users who installed by direct download.
public struct BrewUpdater: Sendable {
    private let caskroomReceipt: URL?
    private let brewCandidates: [String]

    public static let defaultReceipt: URL? =
        URL(fileURLWithPath: "/opt/homebrew/Caskroom/burnt/.metadata/INSTALL_RECEIPT.json")

    public init(caskroomReceipt: URL? = BrewUpdater.defaultReceipt,
                brewCandidates: [String] = ["/opt/homebrew/bin/brew", "/usr/local/bin/brew"]) {
        self.caskroomReceipt = caskroomReceipt
        self.brewCandidates = brewCandidates
    }

    /// True iff this install lives under a Homebrew Caskroom (receipt present).
    public func isBrewManaged() -> Bool {
        guard let r = caskroomReceipt else { return false }
        return FileManager.default.fileExists(atPath: r.path)
    }

    /// First existing brew executable, else nil.
    public func brewPath() -> String? {
        brewCandidates.first { FileManager.default.isExecutableFile(atPath: $0) }
    }

    /// Run `brew update` then `brew upgrade --cask burnt` directly (no shell, so the
    /// brew path is never string-interpolated into a command line). No-op if brew is
    /// absent. brew's cask postflight handles quarantine-strip + relaunch.
    public func upgrade() {
        guard let brew = brewPath() else { return }
        let update = Process()
        update.executableURL = URL(fileURLWithPath: brew)
        update.arguments = ["update"]
        try? update.run()
        update.waitUntilExit()

        let upgrade = Process()
        upgrade.executableURL = URL(fileURLWithPath: brew)
        upgrade.arguments = ["upgrade", "--cask", "burnt"]
        try? upgrade.run()
    }
}
