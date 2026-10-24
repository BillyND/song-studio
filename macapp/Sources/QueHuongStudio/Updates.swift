import Foundation
import Sparkle
import WebKit

/// The update settings the studio page shows under Settings → Updates. The page asks through
/// `window.webkit.messageHandlers.updates.postMessage(...)` and gets the current state back.
/// Both switches live in Sparkle's own user defaults (SUEnableAutomaticChecks, SUAutomaticallyUpdate).
final class UpdatePrefs: NSObject, WKScriptMessageHandlerWithReply {
    private let updater: SPUUpdater

    init(updater: SPUUpdater) {
        self.updater = updater
        super.init()
        Self.migrate(updater)
    }

    /// Updates used to install on their own. Since this version they only do when the user turns
    /// that on, so every copy starts with automatic installing off, once.
    private static func migrate(_ updater: SPUUpdater) {
        let key = "studioUpdatesOptIn"
        guard !UserDefaults.standard.bool(forKey: key) else { return }
        updater.automaticallyDownloadsUpdates = false
        UserDefaults.standard.set(true, forKey: key)
    }

    func userContentController(_ controller: WKUserContentController, didReceive message: WKScriptMessage,
                               replyHandler: @escaping (Any?, String?) -> Void) {
        let body = message.body as? [String: Any] ?? [:]
        switch body["action"] as? String {
        case "set":
            if let on = body["autoCheck"] as? Bool { updater.automaticallyChecksForUpdates = on }
            if let on = body["autoInstall"] as? Bool { updater.automaticallyDownloadsUpdates = on }
        case "check":
            updater.checkForUpdates()
        default:
            break
        }
        replyHandler(state(), nil)
    }

    private func state() -> [String: Any] {
        [
            "autoCheck": updater.automaticallyChecksForUpdates,
            "autoInstall": updater.automaticallyDownloadsUpdates,
            "canCheck": updater.canCheckForUpdates,
            "version": Bundle.main.object(forInfoDictionaryKey: "CFBundleShortVersionString") as? String ?? "",
        ]
    }
}
