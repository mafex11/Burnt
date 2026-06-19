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
}
