import Foundation

public enum ProjectAttributor {
    /// Pure join: session costs + a sessionID→cwd map → projects grouped by cwd leaf,
    /// sorted by cost desc. Unmapped sessions fall into an "Unknown" bucket.
    public static func group(sessions: [SessionRow], cwdBySession: [String: String]) -> [ProjectSlice] {
        struct Agg { var cost = 0.0; var tokens = 0 }
        var byPath: [String: Agg] = [:]

        for s in sessions {
            let path = cwdBySession[s.period] ?? ""
            var a = byPath[path] ?? Agg()
            a.cost += s.totalCost; a.tokens += s.totalTokens
            byPath[path] = a
        }

        let paths = byPath.keys.filter { !$0.isEmpty }
        let leaf: (String) -> String = { ($0 as NSString).lastPathComponent }
        var leafCounts: [String: Int] = [:]
        for p in paths { leafCounts[leaf(p), default: 0] += 1 }

        func name(for path: String) -> String {
            if path.isEmpty { return "Unknown" }
            let l = leaf(path)
            if (leafCounts[l] ?? 0) > 1 {
                let parent = ((path as NSString).deletingLastPathComponent as NSString).lastPathComponent
                return parent.isEmpty ? l : "\(parent)/\(l)"
            }
            return l
        }

        return byPath.map { (path, a) in
            ProjectSlice(name: name(for: path), path: path, cost: a.cost, totalTokens: a.tokens)
        }.sorted { $0.cost > $1.cost }
    }

    /// A session log's cwd never changes after the first line is written, so once we've
    /// read a file we cache (cwd, mtime) and re-read it only if its mtime moves. This
    /// turns the second and later builds from "open 2,400 files" into "stat 2,400 files,
    /// open the handful that changed" — the expensive line-parsing is paid once per file.
    private struct CachedCwd { let sid: String; let cwd: String; let mtime: Date }
    private static let cacheLock = NSLock()
    private nonisolated(unsafe) static var cwdCache: [String: CachedCwd] = [:]   // keyed by file path

    /// Build sessionID→cwd by reading the FIRST cwd-bearing line of each session log.
    /// Subsequent builds reuse cached values for files whose mtime is unchanged.
    public static func buildCwdMap(claudeRoot: URL, codexRoot: URL) -> [String: String] {
        var map: [String: String] = [:]
        let fm = FileManager.default

        if let proj = try? fm.contentsOfDirectory(at: claudeRoot.appendingPathComponent("projects"),
                                                  includingPropertiesForKeys: nil) {
            for dir in proj {
                guard let files = try? fm.contentsOfDirectory(
                    at: dir, includingPropertiesForKeys: [.contentModificationDateKey]) else { continue }
                for f in files where f.pathExtension == "jsonl" {
                    if let hit = cached(f, fm) { map[hit.sid] = hit.cwd; continue }
                    let sid = f.deletingPathExtension().lastPathComponent
                    if let cwd = firstCwd(in: f) {
                        map[sid] = cwd
                        store(f, sid: sid, cwd: cwd, fm: fm)
                    }
                }
            }
        }

        if let en = fm.enumerator(at: codexRoot.appendingPathComponent("sessions"),
                                  includingPropertiesForKeys: [.contentModificationDateKey]) {
            for case let f as URL in en where f.lastPathComponent.hasPrefix("rollout-") && f.pathExtension == "jsonl" {
                if let hit = cached(f, fm) { map[hit.sid] = hit.cwd; continue }
                if let (id, cwd) = codexIdAndCwd(in: f) {
                    map[id] = cwd
                    store(f, sid: id, cwd: cwd, fm: fm)
                }
            }
        }
        return map
    }

    /// Returns the cached entry for `url` iff its mtime matches what we last saw.
    private static func cached(_ url: URL, _ fm: FileManager) -> CachedCwd? {
        guard let mtime = (try? url.resourceValues(forKeys: [.contentModificationDateKey]))?.contentModificationDate
        else { return nil }
        cacheLock.lock(); defer { cacheLock.unlock() }
        guard let e = cwdCache[url.path], e.mtime == mtime else { return nil }
        return e
    }

    private static func store(_ url: URL, sid: String, cwd: String, fm: FileManager) {
        guard let mtime = (try? url.resourceValues(forKeys: [.contentModificationDateKey]))?.contentModificationDate
        else { return }
        cacheLock.lock(); defer { cacheLock.unlock() }
        cwdCache[url.path] = CachedCwd(sid: sid, cwd: cwd, mtime: mtime)
    }

    /// First "cwd" value found in a Claude jsonl. Claude puts cwd on a message line
    /// (often line ~2-3), and the first line can be very large, so we stream lines
    /// (not a fixed byte window) and scan up to `maxLines`.
    private static func firstCwd(in url: URL) -> String? {
        for line in lines(of: url, maxLines: 60) {
            if let obj = try? JSONSerialization.jsonObject(with: Data(line.utf8)) as? [String: Any],
               let cwd = obj["cwd"] as? String { return cwd }
        }
        return nil
    }

    /// Codex session_meta (typically line 1) → (id, cwd). payload may be nested.
    private static func codexIdAndCwd(in url: URL) -> (String, String)? {
        for line in lines(of: url, maxLines: 5) {
            guard let obj = try? JSONSerialization.jsonObject(with: Data(line.utf8)) as? [String: Any]
            else { continue }
            let p = (obj["payload"] as? [String: Any]) ?? obj
            if let id = p["id"] as? String, let cwd = p["cwd"] as? String { return (id, cwd) }
        }
        return nil
    }

    /// Reads up to `maxLines` newline-delimited lines from a file without loading
    /// the whole thing into memory at once. Robust to very large individual lines.
    private static func lines(of url: URL, maxLines: Int) -> [String] {
        guard let h = try? FileHandle(forReadingFrom: url) else { return [] }
        defer { try? h.close() }
        var out: [String] = []
        var buffer = Data()
        let chunkSize = 1 << 16   // 64KB chunks
        let newline = UInt8(ascii: "\n")
        let hardCap = 8 << 20     // never buffer more than 8MB looking for newlines

        while out.count < maxLines {
            // extract any complete lines already in the buffer
            while let nl = buffer.firstIndex(of: newline) {
                let lineData = buffer.subdata(in: buffer.startIndex..<nl)
                buffer.removeSubrange(buffer.startIndex...nl)
                if let s = String(data: lineData, encoding: .utf8) { out.append(s) }
                if out.count >= maxLines { return out }
            }
            if buffer.count > hardCap { break }   // pathological single line; give up
            let chunk = h.readData(ofLength: chunkSize)
            if chunk.isEmpty {   // EOF: flush any trailing partial line
                if !buffer.isEmpty, let s = String(data: buffer, encoding: .utf8) { out.append(s) }
                break
            }
            buffer.append(chunk)
        }
        return out
    }
}
