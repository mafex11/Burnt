import Foundation

public struct CcusageRunner: Sendable {
    /// Pinned ccusage version — bump deliberately after verifying JSON shape.
    public static let pinnedVersion = "17.1.3"

    public enum RunError: Error, Sendable {
        case nonZeroExit(code: Int32, stderr: String)
        case timedOut
        case decodeFailed(String)
    }

    private let invocation: CcusageInvocation
    private let timeout: TimeInterval

    public init(invocation: CcusageInvocation, timeout: TimeInterval = 30) {
        self.invocation = invocation
        self.timeout = timeout
    }

    /// Runs `ccusage daily --json` and decodes the result.
    /// - Parameter offline: when true, appends `--offline` so ccusage uses cached
    ///   pricing (no network). Used by the cheap 60s background poll. When false,
    ///   ccusage fetches live LiteLLM prices — used for human-triggered refreshes.
    public func fetchDailyReport(offline: Bool) throws -> CcusageReport {
        let process = Process()
        process.executableURL = URL(fileURLWithPath: invocation.executable)
        var args = invocation.leadingArgs + ["daily", "--json"]
        if offline { args.append("--offline") }
        process.arguments = args

        var env = ProcessInfo.processInfo.environment
        let extra = "/opt/homebrew/bin:/usr/local/bin"
        env["PATH"] = (env["PATH"].map { "\(extra):\($0)" }) ?? extra
        process.environment = env

        let out = Pipe(); let err = Pipe()
        process.standardOutput = out
        process.standardError = err

        try process.run()

        // Enforce a timeout without blocking forever.
        let deadline = Date().addingTimeInterval(timeout)
        while process.isRunning && Date() < deadline {
            usleep(50_000) // 50ms
        }
        if process.isRunning {
            process.terminate()
            throw RunError.timedOut
        }

        let outData = out.fileHandleForReading.readDataToEndOfFile()
        let errData = err.fileHandleForReading.readDataToEndOfFile()

        guard process.terminationStatus == 0 else {
            throw RunError.nonZeroExit(code: process.terminationStatus,
                stderr: String(decoding: errData, as: UTF8.self))
        }

        do {
            return try JSONDecoder().decode(CcusageReport.self, from: outData)
        } catch {
            throw RunError.decodeFailed(String(describing: error))
        }
    }
}
