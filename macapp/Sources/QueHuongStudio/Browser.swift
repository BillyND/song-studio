import AppKit
import WebKit

/// What a browser would do for the studio page: dialogs, file pickers, downloads, and sending
/// links that leave the studio to the default browser.
final class BrowserDelegate: NSObject, WKNavigationDelegate, WKUIDelegate, WKDownloadDelegate {
    weak var window: NSWindow?
    var onRetry: (() -> Void)?
    var onStudioLoaded: (() -> Void)?
    private var downloads: [ObjectIdentifier: URL] = [:]

    private func isStudio(_ url: URL) -> Bool {
        url.scheme == "http" && url.host == "127.0.0.1" && url.port == 4317
    }

    // MARK: messages shown before the studio answers

    func showMessage(title: String, detail: String, retry: Bool = false) {
        guard let web = window?.contentView as? WKWebView else { return }
        let esc = { (s: String) in s.replacingOccurrences(of: "&", with: "&amp;").replacingOccurrences(of: "<", with: "&lt;") }
        let button = retry ? "<p><a href=\"qhs://retry\">\(L("Thử lại", "Try again"))</a></p>" : #"<div class="spin"></div>"#
        web.loadHTMLString("""
        <!doctype html><meta charset="utf-8"><style>
        :root{color-scheme:light dark}body{margin:0;height:100vh;display:grid;place-items:center;font:15px -apple-system,sans-serif;background:#0f1117;color:#e6e8ef}
        @media(prefers-color-scheme:light){body{background:#f5f6fa;color:#1d2130}}
        .box{text-align:center;max-width:520px;padding:24px}h1{font-size:20px;margin:0 0 8px}p{opacity:.75;line-height:1.5}
        a{display:inline-block;margin-top:8px;padding:9px 18px;border-radius:10px;background:linear-gradient(90deg,#ff5fa2,#ffb36b);color:#fff;text-decoration:none;font-weight:600}
        .spin{width:28px;height:28px;margin:18px auto 0;border:3px solid #8884;border-top-color:#ff5fa2;border-radius:50%;animation:s 1s linear infinite}@keyframes s{to{transform:rotate(1turn)}}
        </style><div class="box"><h1>\(esc(title))</h1><p>\(esc(detail))</p>\(button)</div>
        """, baseURL: nil)
    }

    /// A short note at the bottom of the studio page (for example a pending update).
    func notice(_ text: String) {
        guard let web = window?.contentView as? WKWebView, let url = web.url, isStudio(url) else { return }
        let js = "(()=>{let n=document.getElementById('qhs-native-note');if(!n){n=document.createElement('div');n.id='qhs-native-note';n.style.cssText='position:fixed;left:50%;bottom:14px;transform:translateX(-50%);z-index:99;padding:8px 14px;border-radius:10px;background:#222c;color:#fff;font:13px -apple-system,sans-serif';document.body.appendChild(n)}n.textContent=\(jsString(text));})()"
        web.evaluateJavaScript(js)
    }

    private func jsString(_ s: String) -> String {
        let data = try? JSONSerialization.data(withJSONObject: [s])
        let arr = data.flatMap { String(data: $0, encoding: .utf8) } ?? "[\"\"]"
        return String(arr.dropFirst().dropLast())
    }

    // MARK: navigation

    func webView(_ webView: WKWebView, decidePolicyFor action: WKNavigationAction,
                 decisionHandler: @escaping (WKNavigationActionPolicy) -> Void) {
        guard let url = action.request.url else { return decisionHandler(.cancel) }
        if url.scheme == "qhs" {
            onRetry?()
            return decisionHandler(.cancel)
        }
        if action.shouldPerformDownload { return decisionHandler(.download) }
        if isStudio(url) || url.scheme == "about" { return decisionHandler(.allow) }
        if url.scheme == "file" { return decisionHandler(.cancel) } // a file dropped outside a drop zone
        NSWorkspace.shared.open(url)
        decisionHandler(.cancel)
    }

    func webView(_ webView: WKWebView, decidePolicyFor response: WKNavigationResponse,
                 decisionHandler: @escaping (WKNavigationResponsePolicy) -> Void) {
        let disposition = (response.response as? HTTPURLResponse)?.value(forHTTPHeaderField: "Content-Disposition") ?? ""
        decisionHandler(!response.canShowMIMEType || disposition.hasPrefix("attachment") ? .download : .allow)
    }

    func webView(_ webView: WKWebView, navigationAction: WKNavigationAction, didBecome download: WKDownload) { download.delegate = self }
    func webView(_ webView: WKWebView, navigationResponse: WKNavigationResponse, didBecome download: WKDownload) { download.delegate = self }

    func webView(_ webView: WKWebView, didFailProvisionalNavigation navigation: WKNavigation!, withError error: Error) {
        if (error as NSError).code == NSURLErrorCancelled { return }
        showMessage(title: L("Không mở được studio", "Could not open the studio"), detail: error.localizedDescription, retry: true)
    }

    func webViewWebContentProcessDidTerminate(_ webView: WKWebView) { webView.reload() }

    func webView(_ webView: WKWebView, didFinish navigation: WKNavigation!) {
        if let url = webView.url, isStudio(url) { onStudioLoaded?() }
    }

    /// window.open and target=_blank: studio pages stay here, everything else opens in the browser.
    func webView(_ webView: WKWebView, createWebViewWith configuration: WKWebViewConfiguration,
                 for action: WKNavigationAction, windowFeatures: WKWindowFeatures) -> WKWebView? {
        if let url = action.request.url {
            if isStudio(url) { webView.load(action.request) } else { NSWorkspace.shared.open(url) }
        }
        return nil
    }

    // MARK: downloads go to ~/Downloads and are revealed in Finder

    func download(_ download: WKDownload, decideDestinationUsing response: URLResponse, suggestedFilename: String,
                  completionHandler: @escaping (URL?) -> Void) {
        let dir = FileManager.default.urls(for: .downloadsDirectory, in: .userDomainMask)[0]
        var url = dir.appendingPathComponent(suggestedFilename)
        let stem = url.deletingPathExtension().lastPathComponent, ext = url.pathExtension
        var n = 2
        while FileManager.default.fileExists(atPath: url.path) {
            url = dir.appendingPathComponent(ext.isEmpty ? "\(stem) \(n)" : "\(stem) \(n).\(ext)")
            n += 1
        }
        downloads[ObjectIdentifier(download)] = url
        completionHandler(url)
    }

    func downloadDidFinish(_ download: WKDownload) {
        if let url = downloads.removeValue(forKey: ObjectIdentifier(download)) {
            NSWorkspace.shared.activateFileViewerSelecting([url])
        }
    }

    func download(_ download: WKDownload, didFailWithError error: Error, resumeData: Data?) {
        downloads.removeValue(forKey: ObjectIdentifier(download))
        alert(L("Tải xuống lỗi", "Download failed"), error.localizedDescription)
    }

    // MARK: JavaScript dialogs and file pickers

    private func alert(_ title: String, _ text: String) {
        let a = NSAlert()
        a.messageText = title
        a.informativeText = text
        a.runModal()
    }

    func webView(_ webView: WKWebView, runJavaScriptAlertPanelWithMessage message: String,
                 initiatedByFrame frame: WKFrameInfo, completionHandler: @escaping () -> Void) {
        let a = NSAlert()
        a.messageText = message
        a.addButton(withTitle: "OK")
        a.runModal()
        completionHandler()
    }

    func webView(_ webView: WKWebView, runJavaScriptConfirmPanelWithMessage message: String,
                 initiatedByFrame frame: WKFrameInfo, completionHandler: @escaping (Bool) -> Void) {
        let a = NSAlert()
        a.messageText = message
        a.addButton(withTitle: L("Đồng ý", "OK"))
        a.addButton(withTitle: L("Huỷ", "Cancel"))
        completionHandler(a.runModal() == .alertFirstButtonReturn)
    }

    func webView(_ webView: WKWebView, runJavaScriptTextInputPanelWithPrompt prompt: String, defaultText: String?,
                 initiatedByFrame frame: WKFrameInfo, completionHandler: @escaping (String?) -> Void) {
        let a = NSAlert()
        a.messageText = prompt
        let field = NSTextField(frame: NSRect(x: 0, y: 0, width: 320, height: 24))
        field.stringValue = defaultText ?? ""
        a.accessoryView = field
        a.addButton(withTitle: L("Đồng ý", "OK"))
        a.addButton(withTitle: L("Huỷ", "Cancel"))
        completionHandler(a.runModal() == .alertFirstButtonReturn ? field.stringValue : nil)
    }

    func webView(_ webView: WKWebView, runOpenPanelWith parameters: WKOpenPanelParameters,
                 initiatedByFrame frame: WKFrameInfo, completionHandler: @escaping ([URL]?) -> Void) {
        let panel = NSOpenPanel()
        panel.allowsMultipleSelection = parameters.allowsMultipleSelection
        panel.canChooseDirectories = parameters.allowsDirectories
        panel.canChooseFiles = true
        if let w = window {
            panel.beginSheetModal(for: w) { completionHandler($0 == .OK ? panel.urls : nil) }
        } else {
            completionHandler(panel.runModal() == .OK ? panel.urls : nil)
        }
    }
}
