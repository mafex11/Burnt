import Foundation

public enum Formatters {
    /// Compact token count: 1_234_567 -> "1.2M", 340_000 -> "340K".
    public static func tokens(_ n: Int) -> String {
        let v = Double(n)
        switch v {
        case 1_000_000_000...:
            return trim(v / 1_000_000_000) + "B"
        case 1_000_000...:
            return trim(v / 1_000_000) + "M"
        case 1_000...:
            return trim(v / 1_000) + "K"
        default:
            return "\(n)"
        }
    }

    /// 340.0 -> "340", 1.23 -> "1.2".
    private static func trim(_ x: Double) -> String {
        if x >= 100 { return String(Int(x.rounded())) }
        return String(format: "%.1f", x)
    }

    /// "$4.20"; for >= 1000 drop cents + add thousands separator: "$7,468".
    public static func cost(_ c: Double) -> String {
        if c >= 1000 {
            let f = NumberFormatter()
            f.numberStyle = .decimal
            f.maximumFractionDigits = 0
            f.groupingSeparator = ","
            return "$" + (f.string(from: NSNumber(value: c)) ?? String(Int(c)))
        }
        return String(format: "$%.2f", c)
    }

    /// Magnitude as a whole percent: 0.12 -> "12%", -0.5 -> "50%", 1.24 -> "124%".
    public static func percent(_ fraction: Double) -> String {
        "\(Int((abs(fraction) * 100).rounded()))%"
    }
}
