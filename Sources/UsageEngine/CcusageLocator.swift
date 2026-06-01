import Foundation

/// How to invoke ccusage: an executable plus any args that must precede the
/// ccusage subcommand (npx needs `-y ccusage@<version>`; bundled/PATH need none).
public struct CcusageInvocation: Sendable, Equatable {
    public let executable: String
    public let leadingArgs: [String]
}

public enum EngineState: Sendable, Equatable {
    case ready(CcusageInvocation)
    case unavailable
}

public struct CcusageLocator {
    /// Absolute path to the ccusage binary bundled in the app, or nil. Injectable for tests.
    private let bundledPath: @Sendable () -> String?
    /// Resolve a binary name on PATH to an absolute path, or nil. Injectable for tests.
    private let lookup: @Sendable (String) -> String?

    public init(
        bundledPath: @escaping @Sendable () -> String? = CcusageLocator.defaultBundledPath,
        lookup: @escaping @Sendable (String) -> String? = CcusageLocator.which
    ) {
        self.bundledPath = bundledPath
        self.lookup = lookup
    }

    public func resolve() -> EngineState {
        // 1. Bundled binary — the normal path. Instant, offline, no Node.
        if let bundled = bundledPath() {
            return .ready(CcusageInvocation(executable: bundled, leadingArgs: []))
        }
        // 2. ccusage on PATH (developer machines).
        if let path = lookup("ccusage") {
            return .ready(CcusageInvocation(executable: path, leadingArgs: []))
        }
        // 3. npx fallback, pinned version (last resort).
        if let npx = lookup("npx") {
            return .ready(CcusageInvocation(executable: npx,
                leadingArgs: ["-y", "ccusage@\(CcusageRunner.pinnedVersion)"]))
        }
        return .unavailable
    }

    /// Default: ccusage shipped at Burnt.app/Contents/Resources/ccusage.
    public static func defaultBundledPath() -> String? {
        guard let url = Bundle.main.url(forResource: "ccusage", withExtension: nil) else { return nil }
        return FileManager.default.isExecutableFile(atPath: url.path) ? url.path : nil
    }

    /// Default lookup: resolve a binary via `which`, with Homebrew paths injected
    /// (a GUI-launched app inherits a minimal PATH).
    public static func which(_ name: String) -> String? {
        let p = Process()
        p.executableURL = URL(fileURLWithPath: "/usr/bin/which")
        p.arguments = [name]
        let pipe = Pipe(); p.standardOutput = pipe; p.standardError = Pipe()
        var env = ProcessInfo.processInfo.environment
        let extra = "/opt/homebrew/bin:/usr/local/bin"
        env["PATH"] = (env["PATH"].map { "\(extra):\($0)" }) ?? extra
        p.environment = env
        do { try p.run() } catch { return nil }
        p.waitUntilExit()
        guard p.terminationStatus == 0 else { return nil }
        let data = pipe.fileHandleForReading.readDataToEndOfFile()
        let path = String(decoding: data, as: UTF8.self).trimmingCharacters(in: .whitespacesAndNewlines)
        return path.isEmpty ? nil : path
    }
}
