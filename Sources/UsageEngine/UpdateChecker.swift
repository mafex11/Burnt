import Foundation

public enum UpdateStatus: Sendable, Equatable {
    case upToDate
    case updateAvailable(String)   // the newer version string
}

public enum UpdateChecker {
    /// Numeric, component-wise semantic version comparison. Missing components are
    /// treated as 0; non-numeric components parse as 0. `.updateAvailable` only when
    /// `latest` is strictly greater than `current`.
    public static func compare(current: String, latest: String) -> UpdateStatus {
        let c = parts(current), l = parts(latest)
        let n = max(c.count, l.count)
        for i in 0..<n {
            let a = i < c.count ? c[i] : 0
            let b = i < l.count ? l[i] : 0
            if b > a { return .updateAvailable(latest) }
            if b < a { return .upToDate }
        }
        return .upToDate
    }

    private static func parts(_ v: String) -> [Int] {
        v.split(separator: ".").map { Int($0) ?? 0 }
    }

    public enum UpdateError: Error, Equatable { case unparseable }

    public static let caskURL = URL(string:
        "https://raw.githubusercontent.com/mafex11/homebrew-tap/main/Casks/burnt.rb")!

    /// First `version "X.Y.Z"` value in cask text, else nil.
    public static func parseVersion(fromCask text: String) -> String? {
        // matches: version "1.2.3"
        guard let r = text.range(of: #"version\s+"([0-9]+(?:\.[0-9]+)*)""#,
                                 options: .regularExpression) else { return nil }
        let match = String(text[r])
        guard let q = match.range(of: #"[0-9]+(?:\.[0-9]+)*"#, options: .regularExpression)
        else { return nil }
        return String(match[q])
    }

    /// Fetch the tap cask and parse its version. `fetch` is injected for tests.
    public static func latestVersion(fetch: (URL) throws -> Data = { try Data(contentsOf: $0) }) throws -> String {
        let data = try fetch(caskURL)
        guard let v = parseVersion(fromCask: String(decoding: data, as: UTF8.self)) else {
            throw UpdateError.unparseable
        }
        return v
    }
}
