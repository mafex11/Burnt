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

    /// Build sessionID→cwd by reading the FIRST cwd-bearing line of each session log.
    public static func buildCwdMap(claudeRoot: URL, codexRoot: URL) -> [String: String] {
        var map: [String: String] = [:]
        let fm = FileManager.default

        if let proj = try? fm.contentsOfDirectory(at: claudeRoot.appendingPathComponent("projects"),
                                                  includingPropertiesForKeys: nil) {
            for dir in proj {
                guard let files = try? fm.contentsOfDirectory(at: dir, includingPropertiesForKeys: nil) else { continue }
                for f in files where f.pathExtension == "jsonl" {
                    let sid = f.deletingPathExtension().lastPathComponent
                    if let cwd = firstCwd(in: f) { map[sid] = cwd }
                }
            }
        }

        if let en = fm.enumerator(at: codexRoot.appendingPathComponent("sessions"),
                                  includingPropertiesForKeys: nil) {
            for case let f as URL in en where f.lastPathComponent.hasPrefix("rollout-") && f.pathExtension == "jsonl" {
                if let (id, cwd) = codexIdAndCwd(in: f) { map[id] = cwd }
            }
        }
        return map
    }

    private static func firstCwd(in url: URL) -> String? {
        guard let h = try? FileHandle(forReadingFrom: url) else { return nil }
        defer { try? h.close() }
        let data = h.readData(ofLength: 64_000)
        guard let text = String(data: data, encoding: .utf8) else { return nil }
        for line in text.split(separator: "\n") {
            if let obj = try? JSONSerialization.jsonObject(with: Data(line.utf8)) as? [String: Any],
               let cwd = obj["cwd"] as? String { return cwd }
        }
        return nil
    }

    private static func codexIdAndCwd(in url: URL) -> (String, String)? {
        guard let h = try? FileHandle(forReadingFrom: url) else { return nil }
        defer { try? h.close() }
        let data = h.readData(ofLength: 64_000)
        guard let text = String(data: data, encoding: .utf8),
              let firstLine = text.split(separator: "\n").first,
              let obj = try? JSONSerialization.jsonObject(with: Data(firstLine.utf8)) as? [String: Any]
        else { return nil }
        let p = (obj["payload"] as? [String: Any]) ?? obj
        guard let id = p["id"] as? String, let cwd = p["cwd"] as? String else { return nil }
        return (id, cwd)
    }
}
