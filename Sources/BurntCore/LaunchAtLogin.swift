import Foundation
import ServiceManagement

/// Abstraction over login-item registration so it can be stubbed in tests.
public protocol LoginItemControlling {
    var isEnabled: Bool { get }
    func enable() throws
    func disable() throws
}

/// Production implementation using SMAppService (macOS 13+). Registers the main
/// app itself as a login item — no helper bundle required.
public struct LaunchAtLogin: LoginItemControlling {
    public init() {}

    public var isEnabled: Bool {
        SMAppService.mainApp.status == .enabled
    }

    public func enable() throws {
        try SMAppService.mainApp.register()
    }

    public func disable() throws {
        try SMAppService.mainApp.unregister()
    }
}
