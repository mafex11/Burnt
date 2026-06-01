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

public final class Settings: ObservableObject {
    private let defaults: UserDefaults
    private let loginItem: LoginItemControlling

    private enum Key {
        static let menuBarMode = "menuBarMode"
        static let dailyBudget = "dailyBudget"
    }

    public init(defaults: UserDefaults = .standard, loginItem: LoginItemControlling = LaunchAtLogin()) {
        self.defaults = defaults
        self.loginItem = loginItem
        let raw = defaults.string(forKey: Key.menuBarMode) ?? MenuBarMode.todayCost.rawValue
        self._menuBarMode = Published(initialValue: MenuBarMode(rawValue: raw) ?? .todayCost)
        self._dailyBudget = Published(initialValue: defaults.double(forKey: Key.dailyBudget))
        self._launchAtLogin = Published(initialValue: loginItem.isEnabled)
    }

    @Published public var menuBarMode: MenuBarMode {
        didSet { defaults.set(menuBarMode.rawValue, forKey: Key.menuBarMode) }
    }

    @Published public var dailyBudget: Double {   // 0 = off
        didSet { defaults.set(dailyBudget, forKey: Key.dailyBudget) }
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
