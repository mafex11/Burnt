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
    /// The bundled invocation (bundled Node running ccusage's cli.js), or nil if the
    /// app isn't bundled that way. Injectable for tests.
    private let bundledInvocation: @Sendable () -> CcusageInvocation?
    /// Resolve a binary name on PATH to an absolute path, or nil. Injectable for tests.
    private let lookup: @Sendable (String) -> String?

    public init(
        bundledInvocation: @escaping @Sendable () -> CcusageInvocation? = CcusageLocator.defaultBundledInvocation,
        lookup: @escaping @Sendable (String) -> String? = CcusageLocator.which
    ) {
        self.bundledInvocation = bundledInvocation
        self.lookup = lookup
    }

    public func resolve() -> EngineState {
        // 1. Bundled Node + ccusage — the normal path. Self-contained, no system Node.
        if let bundled = bundledInvocation() {
            return .ready(bundled)
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

    /// Default: bundled Node at Resources/node, running Resources/node_modules/ccusage/dist/cli.js.
    public static func defaultBundledInvocation() -> CcusageInvocation? {
        guard let res = Bundle.main.resourceURL else { return nil }
        let node = res.appendingPathComponent("node")
        let cli = res.appendingPathComponent("node_modules/ccusage/dist/cli.js")
        let fm = FileManager.default
        guard fm.isExecutableFile(atPath: node.path), fm.fileExists(atPath: cli.path) else { return nil }
        return CcusageInvocation(executable: node.path, leadingArgs: [cli.path])
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
