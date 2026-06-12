import Foundation
import UserNotifications
import BurntCore

/// Real macOS poster. Requests permission lazily on first post.
final class NotificationService: NotificationPosting {
    private var authorized = false
    private let queue = DispatchQueue(label: "com.burnt.notifications")

    func post(title: String, body: String, id: String) {
        ensureAuth { ok in
            guard ok else { return }
            let c = UNMutableNotificationContent()
            c.title = title; c.body = body
            let req = UNNotificationRequest(identifier: id, content: c, trigger: nil)
            UNUserNotificationCenter.current().add(req)
        }
    }

    private func ensureAuth(_ done: @escaping (Bool) -> Void) {
        queue.async { [weak self] in
            guard let self else { return }
            if self.authorized { done(true); return }
            UNUserNotificationCenter.current().requestAuthorization(options: [.alert, .sound]) { ok, _ in
                self.queue.async {
                    self.authorized = ok
                    done(ok)
                }
            }
        }
    }
}
