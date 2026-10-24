import Foundation

/// The studio's Go server: started from the app bundle, stopped with SIGTERM so queued work survives.
let studioAddr = "127.0.0.1:4317"
let studioURL = URL(string: "http://\(studioAddr)/")!

struct Health: Decodable {
    let app: String
    let pid: Int32
    let version: String
}

final class ServerProcess {
    private var proc: Process?
    var onExit: ((Int32) -> Void)?
    var isRunning: Bool { proc?.isRunning ?? false }

    func start() throws {
        let p = Process()
        guard let bin = Bundle.main.url(forAuxiliaryExecutable: "que-huong-studio") else {
            throw NSError(domain: "studio", code: 1, userInfo: [NSLocalizedDescriptionKey: "Thiếu chương trình máy chủ trong app"])
        }
        p.executableURL = bin
        var args = ["-no-open"]
        if let skill = Bundle.main.resourceURL?.appendingPathComponent("skill"),
           FileManager.default.fileExists(atPath: skill.appendingPathComponent("SKILL.md").path) {
            args += ["-bundled-skill", skill.path]
        }
        p.arguments = args
        var env = ProcessInfo.processInfo.environment
        env["PATH"] = loginShellPath()
        p.environment = env
        if let log = openLog() {
            p.standardOutput = log
            p.standardError = log
        }
        p.terminationHandler = { [weak self] pr in
            DispatchQueue.main.async { self?.onExit?(pr.terminationStatus) }
        }
        try p.run()
        proc = p
    }

    /// SIGTERM, then SIGKILL if the server has not exited in time.
    func stop(timeout: TimeInterval = 12, done: @escaping () -> Void) {
        guard let p = proc, p.isRunning else { done(); return }
        p.terminate()
        DispatchQueue.global().async {
            let deadline = Date().addingTimeInterval(timeout)
            while p.isRunning && Date() < deadline { usleep(100_000) }
            if p.isRunning { kill(p.processIdentifier, SIGKILL) }
            DispatchQueue.main.async(execute: done)
        }
    }

    private func openLog() -> FileHandle? {
        let dir = FileManager.default.homeDirectoryForCurrentUser.appendingPathComponent(".local/share/que-huong-studio")
        try? FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
        let log = dir.appendingPathComponent("studio.log")
        // keep the log small: rotate once it passes 5 MB
        if let size = try? FileManager.default.attributesOfItem(atPath: log.path)[.size] as? Int, size > 5 << 20 {
            let old = dir.appendingPathComponent("studio.log.1")
            try? FileManager.default.removeItem(at: old)
            try? FileManager.default.moveItem(at: log, to: old)
        }
        if !FileManager.default.fileExists(atPath: log.path) {
            FileManager.default.createFile(atPath: log.path, contents: nil)
        }
        let h = try? FileHandle(forWritingTo: log)
        _ = try? h?.seekToEnd()
        return h
    }
}

/// Finder starts apps with a bare PATH; the studio needs the user's (Homebrew, uv, nvm, ~/.local/bin).
func loginShellPath() -> String {
    let home = FileManager.default.homeDirectoryForCurrentUser.path
    let fallback = ["/opt/homebrew/bin", "/opt/homebrew/sbin", "/usr/local/bin", "\(home)/.local/bin",
                    "/usr/bin", "/bin", "/usr/sbin", "/sbin"]
    var parts: [String] = []
    let shell = ProcessInfo.processInfo.environment["SHELL"] ?? "/bin/zsh"
    let p = Process()
    p.executableURL = URL(fileURLWithPath: shell)
    p.arguments = ["-l", "-i", "-c", "printf '<<%s>>' \"$PATH\""]
    let out = Pipe()
    p.standardOutput = out
    p.standardError = FileHandle.nullDevice
    p.standardInput = FileHandle.nullDevice
    if (try? p.run()) != nil {
        let deadline = Date().addingTimeInterval(6)
        while p.isRunning && Date() < deadline { usleep(50_000) }
        if p.isRunning { p.terminate() }
        let text = String(decoding: out.fileHandleForReading.availableData, as: UTF8.self)
        if let a = text.range(of: "<<"), let b = text.range(of: ">>", range: a.upperBound..<text.endIndex) {
            parts = text[a.upperBound..<b.lowerBound].split(separator: ":").map(String.init)
        }
    }
    for f in fallback where !parts.contains(f) { parts.append(f) }
    return parts.joined(separator: ":")
}

/// Small synchronous HTTP helpers for the local server (short timeouts, never on a slow path).
enum Studio {
    static func get(_ path: String, timeout: TimeInterval = 1.5) -> Data? {
        var req = URLRequest(url: URL(string: path, relativeTo: studioURL)!)
        req.timeoutInterval = timeout
        return send(req, timeout: timeout)
    }

    static func send(_ req: URLRequest, timeout: TimeInterval) -> Data? {
        let sem = DispatchSemaphore(value: 0)
        var result: Data?
        URLSession.shared.dataTask(with: req) { data, resp, _ in
            if let http = resp as? HTTPURLResponse, (200..<300).contains(http.statusCode) { result = data }
            sem.signal()
        }.resume()
        _ = sem.wait(timeout: .now() + timeout + 0.5)
        return result
    }

    static func health() -> Health? {
        guard let d = get("/api/health", timeout: 1) else { return nil }
        return try? JSONDecoder().decode(Health.self, from: d)
    }

    /// Queued or running jobs, so quitting or updating can warn first.
    static func activeRuns() -> Int {
        struct Run: Decodable { let status: String }
        guard let d = get("/api/runs?limit=80"), let runs = try? JSONDecoder().decode([Run].self, from: d) else { return 0 }
        return runs.filter { $0.status == "queued" || $0.status == "running" }.count
    }

    /// Asks a studio started some other way (terminal, older app) to quit, using its page token.
    static func shutdownExisting() -> Bool {
        guard let page = get("/", timeout: 2), let html = String(data: page, encoding: .utf8),
              let r = html.range(of: #"name="studio-token" content="([^"]+)""#, options: .regularExpression) else { return false }
        let tag = String(html[r])
        guard let start = tag.range(of: "content=\"")?.upperBound else { return false }
        let token = String(tag[start...].dropLast())
        var req = URLRequest(url: URL(string: "/api/shutdown", relativeTo: studioURL)!)
        req.httpMethod = "POST"
        req.setValue(token, forHTTPHeaderField: "X-Studio-Token")
        req.setValue("http://\(studioAddr)", forHTTPHeaderField: "Origin")
        req.setValue("application/json", forHTTPHeaderField: "Content-Type")
        req.httpBody = Data("{}".utf8)
        return send(req, timeout: 3) != nil
    }
}
