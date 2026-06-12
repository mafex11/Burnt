import Foundation

public struct CcusageRunner: Sendable {
    /// Pinned ccusage version — bump deliberately after verifying JSON shape.
    public static let pinnedVersion = "20.0.6"

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

    /// Runs `ccusage daily --json` (always live pricing) and decodes the result.
    ///
    /// Pricing is always fetched online: the pinned ccusage's bundled offline price
    /// cache lags real model prices, producing wildly wrong figures, so offline mode
    /// is not used.
    public func fetchDailyReport() throws -> CcusageReport {
        try run(subcommand: "daily", as: CcusageReport.self)
    }

    public func fetchSessionReport() throws -> SessionReport {
        try run(subcommand: "session", as: SessionReport.self)
    }

    private func run<T: Decodable>(subcommand: String, as type: T.Type) throws -> T {
        let process = Process()
        process.executableURL = URL(fileURLWithPath: invocation.executable)
        process.arguments = invocation.leadingArgs + [subcommand, "--json"]

        var env = ProcessInfo.processInfo.environment
        let extra = "/opt/homebrew/bin:/usr/local/bin"
        env["PATH"] = (env["PATH"].map { "\(extra):\($0)" }) ?? extra
        process.environment = env

        let out = Pipe(); let err = Pipe()
        process.standardOutput = out
        process.standardError = err

        // Drain both pipes on background queues WHILE the process runs. ccusage can
        // emit 100+ KB of JSON; if we waited for exit before reading, the child would
        // block on a full pipe buffer and we'd deadlock until the timeout fired.
        let outBox = DataBox(), errBox = DataBox()
        let drain = DispatchGroup()
        readInBackground(out.fileHandleForReading, into: outBox, group: drain)
        readInBackground(err.fileHandleForReading, into: errBox, group: drain)

        try process.run()

        let finished = waitForExit(process, timeout: timeout)
        guard finished else {
            process.terminate()
            _ = drain.wait(timeout: .now() + 2)
            throw RunError.timedOut
        }

        // Process has exited; ensure both readers have consumed everything (EOF).
        drain.wait()

        guard process.terminationStatus == 0 else {
            throw RunError.nonZeroExit(code: process.terminationStatus,
                stderr: String(decoding: errBox.data, as: UTF8.self))
        }

        do {
            return try JSONDecoder().decode(T.self, from: outBox.data)
        } catch {
            throw RunError.decodeFailed(String(describing: error))
        }
    }

    /// Reads a file handle to EOF on a background queue, storing the bytes in `box`.
    private func readInBackground(_ handle: FileHandle, into box: DataBox, group: DispatchGroup) {
        group.enter()
        DispatchQueue.global(qos: .utility).async {
            let data = handle.readDataToEndOfFile()
            box.data = data
            group.leave()
        }
    }

    /// Waits up to `timeout` seconds for the process to exit. Returns true if it
    /// exited in time, false on timeout. Uses the process's own termination handler
    /// rather than a busy-wait.
    private func waitForExit(_ process: Process, timeout: TimeInterval) -> Bool {
        let sema = DispatchSemaphore(value: 0)
        process.terminationHandler = { _ in sema.signal() }
        // If the process already finished before the handler was set, signal now.
        if !process.isRunning { return true }
        return sema.wait(timeout: .now() + timeout) == .success
    }
}

/// A reference box so a background queue can hand bytes back without data races on
/// a captured `var`. Access is synchronized by the DispatchGroup (write happens-before
/// the group's wait() returns).
private final class DataBox: @unchecked Sendable {
    var data = Data()
}
