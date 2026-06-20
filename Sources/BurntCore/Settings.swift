import Foundation
import Combine

public enum MenuBarMode: String, CaseIterable, Sendable {
    case todayCost, todayTokens, weekCost, iconOnly

    public var label: String {
        switch self {
        case .todayCost: return "Today $"
        case .todayTokens: return "Today tokens"
        case .weekCost: return "Week $"
        case .iconOnly: return "Icon only"
        }
    }
}

/// Popover information density. Levels are additive: each shows everything the
/// lower level does, plus more — so `Comparable` ordering drives visibility.
public enum DashboardStyle: String, CaseIterable, Sendable, Comparable {
    case minimal, standard, detailed

    public var label: String {
        switch self {
        case .minimal: return "Minimal"
        case .standard: return "Standard"
        case .detailed: return "Detailed"
        }
    }

    private var rank: Int {
        switch self {
        case .minimal: return 0
        case .standard: return 1
        case .detailed: return 2
        }
    }

    public static func < (lhs: DashboardStyle, rhs: DashboardStyle) -> Bool {
        lhs.rank < rhs.rank
    }
}

public final class Settings: ObservableObject {
    private let defaults: UserDefaults
    private let loginItem: LoginItemControlling

    private enum Key {
        static let menuBarMode = "menuBarMode"
        static let dailyBudget = "dailyBudget"
        static let dashboardStyle = "dashboardStyle"
        static let notifyBudget = "notifyBudget"
        static let notifyDailySummary = "notifyDailySummary"
        static let notifyMilestones = "notifyMilestones"
        static let animateFlame = "animateFlame"
        static let autoUpdate = "autoUpdate"
    }

    public init(defaults: UserDefaults = .standard, loginItem: LoginItemControlling = LaunchAtLogin()) {
        self.defaults = defaults
        self.loginItem = loginItem
        let raw = defaults.string(forKey: Key.menuBarMode) ?? MenuBarMode.todayCost.rawValue
        self._menuBarMode = Published(initialValue: MenuBarMode(rawValue: raw) ?? .todayCost)
        self._dailyBudget = Published(initialValue: defaults.double(forKey: Key.dailyBudget))
        self._launchAtLogin = Published(initialValue: loginItem.isEnabled)
        // Default to Standard when unset.
        let styleRaw = defaults.string(forKey: Key.dashboardStyle) ?? DashboardStyle.standard.rawValue
        self._dashboardStyle = Published(initialValue: DashboardStyle(rawValue: styleRaw) ?? .standard)
        self._notifyBudget = Published(initialValue: defaults.bool(forKey: Key.notifyBudget))
        self._notifyDailySummary = Published(initialValue: defaults.bool(forKey: Key.notifyDailySummary))
        self._notifyMilestones = Published(initialValue: defaults.bool(forKey: Key.notifyMilestones))
        // Animate the menu-bar flame; default ON when unset.
        let animate = defaults.object(forKey: Key.animateFlame) == nil ? true : defaults.bool(forKey: Key.animateFlame)
        self._animateFlame = Published(initialValue: animate)
        // Keep Burnt current via brew; default ON when unset.
        let auto = defaults.object(forKey: Key.autoUpdate) == nil ? true : defaults.bool(forKey: Key.autoUpdate)
        self._autoUpdate = Published(initialValue: auto)
    }

    @Published public var menuBarMode: MenuBarMode {
        didSet { defaults.set(menuBarMode.rawValue, forKey: Key.menuBarMode) }
    }

    @Published public var dashboardStyle: DashboardStyle {
        didSet { defaults.set(dashboardStyle.rawValue, forKey: Key.dashboardStyle) }
    }

    @Published public var dailyBudget: Double {   // 0 = off
        didSet { defaults.set(dailyBudget, forKey: Key.dailyBudget) }
    }

    @Published public var notifyBudget: Bool {
        didSet { defaults.set(notifyBudget, forKey: Key.notifyBudget) }
    }

    @Published public var notifyDailySummary: Bool {
        didSet { defaults.set(notifyDailySummary, forKey: Key.notifyDailySummary) }
    }

    @Published public var animateFlame: Bool {
        didSet { defaults.set(animateFlame, forKey: Key.animateFlame) }
    }

    @Published public var notifyMilestones: Bool {
        didSet { defaults.set(notifyMilestones, forKey: Key.notifyMilestones) }
    }

    @Published public var autoUpdate: Bool {
        didSet { defaults.set(autoUpdate, forKey: Key.autoUpdate) }
    }

    @Published public var launchAtLogin: Bool {
        didSet {
            guard launchAtLogin != oldValue else { return }
            do {
                if launchAtLogin { try loginItem.enable() } else { try loginItem.disable() }
            } catch {
                launchAtLogin = loginItem.isEnabled
            }
        }
    }
}
