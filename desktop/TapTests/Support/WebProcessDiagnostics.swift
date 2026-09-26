import XCTest
import WebKit
@testable import Tap

extension WKWebView {
    /// The process identifier of this web view's content process, read
    /// through WebKit's private `_webProcessIdentifier` key. Tests only:
    /// the app never reads it. Nil when WebKit does not offer the key or
    /// the web view has no process.
    var contentProcessIdentifier: pid_t? {
        guard responds(to: NSSelectorFromString("_webProcessIdentifier")),
              let number = value(forKey: "_webProcessIdentifier") as? NSNumber, number.int32Value > 0 else { return nil }
        return number.int32Value
    }
}

/// Where a hosted test saves the files a failure produces, such as a
/// spindump. CI uploads this folder when the Desktop Tests job fails.
enum TestDiagnosticsFolder {
    static var url: URL {
        let folder = FileManager.default.homeDirectoryForCurrentUser
            .appendingPathComponent("Library/Logs/TapTests", isDirectory: true)
        try? FileManager.default.createDirectory(at: folder, withIntermediateDirectories: true)
        return folder
    }
}

/// Runs a command and returns what it printed, standard output then
/// standard error. A command still running after `timeout` seconds is
/// terminated, so a stuck tool cannot hang the failure it is reporting on.
@MainActor
func runBounded(_ executable: String, _ arguments: [String], timeout: TimeInterval) async -> String {
    let process = Process()
    process.executableURL = URL(fileURLWithPath: executable)
    process.arguments = arguments
    let output = Pipe()
    process.standardOutput = output
    process.standardError = output
    process.standardInput = FileHandle.nullDevice
    do {
        try process.run()
    } catch {
        return "\(executable) did not start: \(error.localizedDescription)"
    }
    // Reading on a background queue keeps a full pipe from blocking the tool.
    let reader = Task.detached { output.fileHandleForReading.readDataToEndOfFile() }
    let deadline = Date().addingTimeInterval(timeout)
    while process.isRunning, Date() < deadline {
        try? await Task.sleep(nanoseconds: 50_000_000)
    }
    var suffix = ""
    if process.isRunning {
        process.terminate()
        suffix = "\n(terminated after \(Int(timeout)) s)"
    }
    let data = await reader.value
    return String(decoding: data, as: UTF8.self).trimmingCharacters(in: .whitespacesAndNewlines) + suffix
}

extension HostedTestCase {
    /// Whether `webView`'s page answers a no-op script within `timeout`.
    func pageAnswers(_ webView: WKWebView, timeout: TimeInterval = 3) async -> String {
        switch await webView.evaluate("1+1", timeout: timeout) {
        case .value(let value): return "answered \(String(describing: value ?? "nil"))"
        case .failed(let error): return "script failed: \(error.localizedDescription)"
        case .noAnswer(let seconds): return "no answer in \(Int(seconds)) s"
        }
    }

    /// Whether a brand-new web view, with a new content process and a
    /// store of its own, answers a no-op script within 3 s: a stall that
    /// catches it too is not one process's but WebKit's as a whole.
    func freshWebViewAnswers() async -> String {
        let configuration = WKWebViewConfiguration()
        configuration.websiteDataStore = .nonPersistent()
        let webView = WKWebView(frame: NSRect(x: 0, y: 0, width: 100, height: 100), configuration: configuration)
        webView.loadHTMLString("<p>ping</p>", baseURL: nil)
        let answer = await pageAnswers(webView)
        webView.stopLoading()
        return answer
    }

    /// What a first ready that never arrived records about WebKit's side:
    /// the preview's content process as `ps` sees it, how many WebKit
    /// processes run, a short spindump of the preview's process where
    /// `sudo -n` works (CI; it fails harmlessly elsewhere), and whether the
    /// thumbnail renderer's page and a brand-new web view answer script at
    /// the same moment. The spindump is attached to the test and kept in
    /// `TestDiagnosticsFolder`, and its section for the process is printed.
    func webProcessDiagnostics(_ controller: DeckSessionController) async -> String {
        let preview = controller.previewViewController
        let renderer = controller.thumbnails.renderer
        var lines: [String] = []
        let previewProcess = preview.webView.contentProcessIdentifier
        let rendererProcess = renderer.webView.contentProcessIdentifier
        lines.append("previewProcess=\(previewProcess.map(String.init) ?? "unknown") rendererProcess=\(rendererProcess.map(String.init) ?? "unknown")")
        if let previewProcess {
            let status = await runBounded("/bin/ps", ["-o", "pid,stat,%cpu,rss,etime,comm", "-p", String(previewProcess)], timeout: 5)
            lines.append("ps: \(status.replacingOccurrences(of: "\n", with: " | "))")
        }
        let webKitProcesses = await runBounded("/usr/bin/pgrep", ["-fl", "com.apple.WebKit"], timeout: 5)
        let counts = Dictionary(grouping: webKitProcesses.split(separator: "\n"), by: { line -> String in
            line.contains("WebContent") ? "WebContent" : line.contains("Networking") ? "Networking" : line.contains("GPU") ? "GPU" : "other"
        }).mapValues(\.count)
        lines.append("webKitProcesses=\(counts.sorted { $0.key < $1.key }.map { "\($0.key):\($0.value)" }.joined(separator: " "))")
        lines.append("rendererPage=\(await pageAnswers(renderer.webView))")
        lines.append("freshWebView=\(await freshWebViewAnswers())")
        if let previewProcess {
            lines.append("spindump: \(await spindump(previewProcess))")
        }
        return lines.joined(separator: " ")
    }

    /// A 3 s spindump of `processIdentifier`, attached to the test. Returns
    /// where it went, or why there is none.
    private func spindump(_ processIdentifier: pid_t) async -> String {
        let file = TestDiagnosticsFolder.url.appendingPathComponent("spindump-\(processIdentifier)-\(Int(Date().timeIntervalSince1970)).txt")
        let result = await runBounded("/usr/bin/sudo", ["-n", "/usr/sbin/spindump", String(processIdentifier), "3", "-file", file.path], timeout: 60)
        guard let text = try? String(contentsOf: file, encoding: .utf8) else {
            return "none (\(result.split(separator: "\n").first.map(String.init) ?? "no output"))"
        }
        let attachment = XCTAttachment(contentsOfFile: file)
        attachment.name = file.lastPathComponent
        attachment.lifetime = .keepAlways
        add(attachment)
        // The process's own section, so the CI log shows its threads
        // without the artifact.
        let allLines = text.components(separatedBy: "\n")
        let start = allLines.firstIndex { $0.hasPrefix("Process:") && $0.contains("[\(processIdentifier)]") } ?? 0
        print("spindump of \(processIdentifier):\n" + allLines[start..<min(allLines.count, start + 150)].joined(separator: "\n"))
        return file.path
    }
}
