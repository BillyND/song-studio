import Foundation

/// The interface language ("vi" or "en") chosen in the studio's Cài đặt · Settings, which the Go
/// server stores in ~/.config/que-huong-studio/config.json. Menus and dialogs follow it.
enum Lang {
    private(set) static var current = read()

    static func read() -> String {
        let url = FileManager.default.homeDirectoryForCurrentUser.appendingPathComponent(".config/que-huong-studio/config.json")
        guard let data = try? Data(contentsOf: url),
              let json = try? JSONSerialization.jsonObject(with: data) as? [String: Any],
              json["language"] as? String == "en" else { return "vi" }
        return "en"
    }

    /// Re-reads the setting (after the page reloads for a language change). True when it changed.
    /// It also tells AppKit which localization to prefer, so Sparkle's dialogs match.
    @discardableResult
    static func refresh() -> Bool {
        let next = read()
        UserDefaults.standard.set([next], forKey: "AppleLanguages")
        defer { current = next }
        return next != current
    }
}

/// Picks the Vietnamese or English text for the current interface language.
func L(_ vi: String, _ en: String) -> String { Lang.current == "en" ? en : vi }
