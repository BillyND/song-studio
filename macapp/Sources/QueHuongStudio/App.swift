import AppKit
import Sparkle
import WebKit

/// Song Studio as a Mac app: one window showing the studio UI served by the bundled Go server,
/// Vietnamese menus, and Sparkle checking the public releases feed for new versions.
final class AppDelegate: NSObject, NSApplicationDelegate, SPUUpdaterDelegate {
    private var window: NSWindow!
    private var web: WKWebView!
    private let server = ServerProcess()
    private let browser = BrowserDelegate()
    private var updater: SPUStandardUpdaterController!
    private var quitting = false
    private var updating = false

    func applicationDidFinishLaunching(_ note: Notification) {
        if renameOldBundle() { return }
        updater = SPUStandardUpdaterController(startingUpdater: true, updaterDelegate: self, userDriverDelegate: nil)
        NSApp.mainMenu = buildMenu()

        let config = WKWebViewConfiguration()
        config.preferences.isElementFullscreenEnabled = true
        web = WKWebView(frame: .zero, configuration: config)
        web.navigationDelegate = browser
        web.uiDelegate = browser
        web.allowsMagnification = true
        if #available(macOS 13.3, *) { web.isInspectable = true }
        browser.onRetry = { [weak self] in self?.startStudio() }

        window = NSWindow(contentRect: NSRect(x: 0, y: 0, width: 1360, height: 880),
                          styleMask: [.titled, .closable, .miniaturizable, .resizable], backing: .buffered, defer: false)
        window.title = "Song Studio"
        window.minSize = NSSize(width: 860, height: 560)
        window.contentView = web
        window.center()
        window.setFrameAutosaveName("QueHuongStudioMain")
        window.makeKeyAndOrderFront(nil)
        browser.window = window

        server.onExit = { [weak self] code in self?.serverExited(code) }
        startStudio()
    }

    /// Sparkle updates an app in place, so copies installed before the rename keep the file name
    /// "Quê Hương Studio.app". On first start such a copy renames itself to "Song Studio.app" and
    /// reopens from there; the bundle identifier, data and keys do not change. False when nothing to do.
    private func renameOldBundle() -> Bool {
        let url = Bundle.main.bundleURL
        let name = url.lastPathComponent.precomposedStringWithCanonicalMapping
        guard name == "Quê Hương Studio.app".precomposedStringWithCanonicalMapping else { return false }
        let target = url.deletingLastPathComponent().appendingPathComponent("Song Studio.app")
        guard !FileManager.default.fileExists(atPath: target.path),
              (try? FileManager.default.moveItem(at: url, to: target)) != nil else { return false } // e.g. no write access: keep going under the old name
        let config = NSWorkspace.OpenConfiguration()
        config.createsNewApplicationInstance = true
        NSWorkspace.shared.openApplication(at: target, configuration: config) { _, _ in
            DispatchQueue.main.async { NSApp.terminate(nil) }
        }
        return true
    }

    /// Replaces any studio already on the port (an older app or a terminal run), starts the bundled
    /// server, waits until it answers, then shows it.
    private func startStudio() {
        browser.showMessage(title: "Đang mở studio…", detail: "Khởi động máy chủ trên máy bạn.")
        DispatchQueue.global().async {
            if let h = Studio.health(), !self.server.isRunning {
                _ = Studio.shutdownExisting()
                let deadline = Date().addingTimeInterval(15)
                while Studio.health() != nil && Date() < deadline { usleep(250_000) }
                if Studio.health() != nil {
                    DispatchQueue.main.async {
                        self.browser.showMessage(title: "Cổng 4317 đang bận",
                                                 detail: "Một studio khác (bản \(h.version), PID \(h.pid)) không chịu tắt. Tắt nó rồi bấm Thử lại.", retry: true)
                    }
                    return
                }
            }
            DispatchQueue.main.async {
                do {
                    if !self.server.isRunning { try self.server.start() }
                } catch {
                    self.browser.showMessage(title: "Không chạy được máy chủ", detail: error.localizedDescription, retry: true)
                    return
                }
                DispatchQueue.global().async {
                    let deadline = Date().addingTimeInterval(40)
                    while Studio.health() == nil && Date() < deadline && self.server.isRunning { usleep(200_000) }
                    DispatchQueue.main.async {
                        if Studio.health() != nil {
                            self.web.load(URLRequest(url: studioURL))
                        } else if self.server.isRunning {
                            self.browser.showMessage(title: "Máy chủ chưa trả lời", detail: "Xem nhật ký ở ~/.local/share/que-huong-studio/studio.log", retry: true)
                        }
                    }
                }
            }
        }
    }

    private func serverExited(_ code: Int32) {
        if quitting { return }
        if code == 0 { // the server was asked to quit (POST /api/shutdown): close the app with it
            quitting = true
            NSApp.terminate(nil)
            return
        }
        browser.showMessage(title: "Máy chủ studio đã dừng (mã \(code))",
                            detail: "Việc đang chờ vẫn được giữ. Xem ~/.local/share/que-huong-studio/studio.log", retry: true)
    }

    func applicationShouldTerminateAfterLastWindowClosed(_ app: NSApplication) -> Bool { true }

    func applicationShouldTerminate(_ app: NSApplication) -> NSApplication.TerminateReply {
        if !server.isRunning {
            quitting = true
            return .terminateNow
        }
        if !updating && !quitting {
            let n = Studio.activeRuns()
            if n > 0 {
                let alert = NSAlert()
                alert.messageText = "Đang có \(n) việc chạy hoặc chờ"
                alert.informativeText = "Thoát sẽ dừng việc đang chạy. Việc đang chờ được giữ lại; script dở sẽ chạy lại khi mở app lần sau."
                alert.addButton(withTitle: "Thoát")
                alert.addButton(withTitle: "Huỷ")
                if alert.runModal() != .alertFirstButtonReturn { return .terminateCancel }
            }
        }
        quitting = true
        server.stopAndWait() // usually well under a second; up to 12 s while jobs wind down
        return .terminateNow
    }

    // MARK: Sparkle

    /// Waits for running work to finish before an update relaunches the app.
    func updater(_ updater: SPUUpdater, shouldPostponeRelaunchForUpdate item: SUAppcastItem,
                 untilInvokingBlock installHandler: @escaping () -> Void) -> Bool {
        if Studio.activeRuns() == 0 {
            updating = true
            return false
        }
        browser.notice("Bản \(item.displayVersionString) sẽ cài khi các việc đang chạy xong.")
        Timer.scheduledTimer(withTimeInterval: 15, repeats: true) { [weak self] t in
            DispatchQueue.global().async {
                guard Studio.activeRuns() == 0 else { return }
                DispatchQueue.main.async {
                    t.invalidate()
                    self?.updating = true
                    installHandler()
                }
            }
        }
        return true
    }

    func updaterWillRelaunchApplication(_ updater: SPUUpdater) { updating = true }

    // MARK: Menus

    private func buildMenu() -> NSMenu {
        let main = NSMenu()
        func submenu(_ title: String, _ items: [NSMenuItem]) -> NSMenu {
            let item = NSMenuItem(title: title, action: nil, keyEquivalent: "")
            let menu = NSMenu(title: title)
            items.forEach(menu.addItem)
            item.submenu = menu
            main.addItem(item)
            return menu
        }
        func item(_ title: String, _ action: Selector?, _ key: String = "", _ mods: NSEvent.ModifierFlags = .command, target: AnyObject? = nil) -> NSMenuItem {
            let i = NSMenuItem(title: title, action: action, keyEquivalent: key)
            i.keyEquivalentModifierMask = mods
            i.target = target
            return i
        }
        let check = item("Kiểm tra cập nhật…", #selector(SPUStandardUpdaterController.checkForUpdates(_:)), target: updater)
        _ = submenu("Song Studio", [
            item("Giới thiệu Song Studio", #selector(NSApplication.orderFrontStandardAboutPanel(_:)), ""),
            check,
            .separator(),
            item("Cài đặt…", #selector(openSettings), ",", target: self),
            .separator(),
            item("Ẩn Song Studio", #selector(NSApplication.hide(_:)), "h"),
            item("Ẩn ứng dụng khác", #selector(NSApplication.hideOtherApplications(_:)), "h", [.command, .option]),
            item("Hiện tất cả", #selector(NSApplication.unhideAllApplications(_:))),
            .separator(),
            item("Thoát Song Studio", #selector(NSApplication.terminate(_:)), "q"),
        ])
        _ = submenu("Sửa", [
            item("Hoàn tác", Selector(("undo:")), "z"),
            item("Làm lại", Selector(("redo:")), "z", [.command, .shift]),
            .separator(),
            item("Cắt", #selector(NSText.cut(_:)), "x"),
            item("Chép", #selector(NSText.copy(_:)), "c"),
            item("Dán", #selector(NSText.paste(_:)), "v"),
            item("Chọn tất cả", #selector(NSText.selectAll(_:)), "a"),
        ])
        _ = submenu("Xem", [
            item("Tải lại", #selector(reload), "r", target: self),
            .separator(),
            item("Phóng to chữ", #selector(zoomIn), "+", target: self),
            item("Thu nhỏ chữ", #selector(zoomOut), "-", target: self),
            item("Cỡ chữ thật", #selector(zoomReset), "0", target: self),
            .separator(),
            item("Toàn màn hình", #selector(NSWindow.toggleFullScreen(_:)), "f", [.command, .control]),
        ])
        let windowMenu = submenu("Cửa sổ", [
            item("Thu nhỏ", #selector(NSWindow.performMiniaturize(_:)), "m"),
            item("Phóng to", #selector(NSWindow.performZoom(_:))),
        ])
        NSApp.windowsMenu = windowMenu
        _ = submenu("Trợ giúp", [
            item("Mở trong trình duyệt", #selector(openInBrowser), target: self),
            item("Mở thư mục nhật ký", #selector(openLogs), target: self),
        ])
        return main
    }

    @objc private func openSettings() { web.evaluateJavaScript("location.hash = '#/settings'") }
    @objc private func reload() { if web.url?.host == "127.0.0.1" { web.reload() } else { startStudio() } }
    @objc private func zoomIn() { web.pageZoom = min(web.pageZoom + 0.1, 2) }
    @objc private func zoomOut() { web.pageZoom = max(web.pageZoom - 0.1, 0.6) }
    @objc private func zoomReset() { web.pageZoom = 1 }
    @objc private func openInBrowser() { NSWorkspace.shared.open(studioURL) }
    @objc private func openLogs() {
        NSWorkspace.shared.open(FileManager.default.homeDirectoryForCurrentUser.appendingPathComponent(".local/share/que-huong-studio"))
    }
}
