import AppKit

/// `tap serve` for a previewed website export. Task 11 fills this in with a
/// real run; until then it is a stub `previewServer` can hold and stop.
final class PreviewServer {
    func stop() {}
}

enum ExportKind: Equatable {
    /// `content` is slides, notes or both, tap's --content values.
    case pdf(content: String)
    case website
    case images

    var title: String {
        switch self {
        case .pdf: return "Export PDF"
        case .website: return "Export Website"
        case .images: return "Export Slide Images"
        }
    }

    /// The default output next to the deck: <deck>.pdf, <folder>/dist, <folder>/<deck>-slides.
    func defaultOutput(for deck: URL) -> URL {
        switch self {
        case .pdf: return deck.deletingPathExtension().appendingPathExtension("pdf")
        case .website: return deck.deletingLastPathComponent().appendingPathComponent("dist")
        case .images: return deck.deletingLastPathComponent().appendingPathComponent(deck.deletingPathExtension().lastPathComponent + "-slides")
        }
    }

    /// The done state's title, as the ExportWebsite and ExportWarnings boards word it.
    func doneTitle(warnings: Int) -> String {
        switch self {
        case .pdf: return warnings == 0 ? "PDF exported" : "PDF exported with \(warnings) warning\(warnings == 1 ? "" : "s")"
        case .website: return "Website exported"
        case .images: return warnings == 0 ? "Slide images exported" : "Slide images exported with gaps"
        }
    }

    /// The warnings box's heading.
    var warningsHeader: String {
        switch self {
        case .pdf: return "These pages show an error card in the PDF"
        case .website: return "Warnings"
        case .images: return "These slides have no image"
        }
    }

    /// The download state's line, naming the command as the ExportPDF board does.
    var downloadDetail: String {
        switch self {
        case .pdf: return "This happens once. tap pdf renders with Chromium, so the PDF matches the CLI exactly."
        case .images: return "This happens once. tap export images renders with Chromium, so the images match the CLI exactly."
        case .website: return "This happens once."
        }
    }

    var isPDF: Bool { if case .pdf = self { return true } else { return false } }
    var isWebsite: Bool { self == .website }
}

struct ExportRequest: Equatable {
    var kind: ExportKind
    var output: String

    func arguments(deck: URL) -> [String] {
        switch kind {
        case .pdf(let content): return ["export", "pdf", deck.path, "--output", output, "--content", content, "--progress", "json"]
        case .website: return ["build", deck.path, "--output", output, "--progress", "json"]
        case .images: return ["export", "images", deck.path, "--all", "--output", output, "--progress", "json"]
        }
    }
}

/// One slide tap could not render whole, in tap's words.
struct ExportWarning: Equatable {
    var slide: Int
    var message: String
}

/// What a finished export shows: the output, a summary line, and tap's warnings.
struct ExportSummary: Equatable {
    var output: URL
    var summary: String
    var warnings: [ExportWarning]
}

/// One export at a time for a deck window: saves the buffer, runs tap
/// with --progress json, turns the lines into the sheet's states, cancels
/// with SIGINT, and stops with the window. tap does the export; this
/// object reads what tap says.
@MainActor
final class ExportController {
    enum State: Equatable {
        case idle
        case running(String)
        case done(ExportSummary)
        case failed(String)
    }

    private(set) var state: State = .idle {
        didSet { onStateChange?(state) }
    }
    var onStateChange: ((State) -> Void)?
    /// Bytes of total while the export engine downloads.
    private(set) var download: (bytes: Int64, totalBytes: Int64)?
    var onDownload: (((bytes: Int64, totalBytes: Int64)?) -> Void)?
    /// From Export until tap has started: a second Export starts nothing.
    private(set) var isStarting = false
    /// From Cancel until tap has exited: later progress lines change nothing.
    private(set) var isCancelling = false
    private var run: ToolRun?
    private var brokenLines: [ExportWarning] = []
    private weak var sessionController: DeckSessionController?
    private var startedAt: Date?

    init(sessionController: DeckSessionController) {
        self.sessionController = sessionController
    }

    var isRunning: Bool { isStarting || (run?.isRunning ?? false) }
    var processIdentifier: Int32? { run.map(\.processIdentifier) }

    /// Saves, then runs. A refused save is a failure with the document's reason.
    func start(_ request: ExportRequest) {
        guard !isRunning, let sessionController, let deck = sessionController.document?.fileURL else { return }
        isStarting = true
        isCancelling = false
        brokenLines = []
        state = .running("Saving…")
        sessionController.saveNow { [weak self] error in
            guard let self else { return }
            if let error {
                self.isStarting = false
                self.state = .failed("The deck could not be saved: \(error.localizedDescription)")
                return
            }
            Task { @MainActor [weak self] in
                guard let self, let sessionController = self.sessionController else { return }
                let run = await TapTool.makeRun(request.arguments(deck: deck), in: deck.deletingLastPathComponent(), timeout: 1800, log: sessionController.session.log)
                self.run = run
                self.startedAt = Date()
                let log = sessionController.session.log
                run.onProgress = { [weak self] line in self?.handle(line, request: request) }
                // tap export images names each broken slide on stderr ("slide N: reason"); the log keeps the line and the sheet lists it.
                run.onStandardErrorLine = { [weak self, weak log] line in
                    log?.append(line, source: .standardError)
                    self?.noteBrokenLine(line)
                }
                run.onExit = { [weak self] exit in self?.finished(exit, request: request) }
                do {
                    try run.start()
                    self.isStarting = false
                    self.state = .running("Preparing…")
                } catch {
                    self.isStarting = false
                    self.run = nil
                    self.state = .failed("tap could not be started: \(error.localizedDescription)")
                }
            }
        }
    }

    func cancel() {
        guard let run, run.isRunning, !isCancelling else { return }
        isCancelling = true
        state = .running("Cancelling…")
        run.cancel()
    }

    /// The window is closing: no export outlives its deck.
    func stop() {
        run?.cancel()
    }

    private func handle(_ line: ProgressLine, request: ExportRequest) {
        guard !isCancelling else { return }
        switch line {
        case .download(let bytes, let totalBytes):
            download = (bytes, totalBytes)
            onDownload?(download)
            state = .running("Downloading the export engine")
        case .step(let phase, let done, let total):
            if download != nil {
                download = nil
                onDownload?(nil)
            }
            state = .running(Self.status(for: phase, done: done, total: total, kind: request.kind))
        case .finished:
            break
        }
    }

    private static let brokenLinePattern = try! NSRegularExpression(pattern: #"^slide (\d+): (.+)$"#)

    private func noteBrokenLine(_ line: String) {
        let whole = line as NSString
        guard let match = Self.brokenLinePattern.firstMatch(in: line, range: NSRange(location: 0, length: whole.length)),
              let slide = Int(whole.substring(with: match.range(at: 1))) else { return }
        brokenLines.append(ExportWarning(slide: slide, message: whole.substring(with: match.range(at: 2))))
    }

    static func status(for phase: String, done: Int, total: Int, kind: ExportKind) -> String {
        switch phase {
        case "render": return "Rendering slide \(done) of \(total)"
        case "load": return "Loading the deck"
        case "parse": return "Parsing the deck"
        case "bundle": return "Bundling components"
        case "write": return "Writing files"
        default: return "\(phase) \(done) of \(total)"
        }
    }

    private func finished(_ exit: ToolRun.Exit, request: ExportRequest) {
        run = nil
        download = nil
        onDownload?(nil)
        let log = sessionController?.session.log
        if exit.cancelled {
            isCancelling = false
            log?.append("export cancelled (exit \(exit.status))", source: .app)
            state = .idle
            return
        }
        switch exit.outcome {
        case .ok?:
            state = .done(summary(from: exit, request: request))
        case .failed(let code, let message)?:
            // tap export images exits 1 with broken_slides after writing the
            // other files; that is warnings with a partial result, not a failure.
            if code == "broken_slides", case .images = request.kind {
                state = .done(ExportSummary(output: URL(fileURLWithPath: request.output), summary: "Some slides could not be rendered. The others are in the folder.",
                                            warnings: brokenLines.isEmpty ? [ExportWarning(slide: 0, message: message)] : brokenLines))
            } else {
                state = .failed(message)
            }
        case nil:
            state = .failed(exit.timedOut ? "tap took longer than 30 minutes" : "tap did not answer (exit \(exit.status)); see the Tap Log")
        }
    }

    private func summary(from exit: ToolRun.Exit, request: ExportRequest) -> ExportSummary {
        let seconds = startedAt.map { String(format: "%.1f s", Date().timeIntervalSince($0)) } ?? ""
        let formatter = ByteCountFormatter()
        formatter.countStyle = .file
        switch request.kind {
        case .pdf:
            let result = try? exit.outcome?.result(PDFExportResult.self)
            let warnings = (result?.brokenSlides ?? []).map { ExportWarning(slide: $0.slide, message: $0.message) }
            let summary = result.map { "\($0.pages) pages, \(formatter.string(fromByteCount: $0.bytes)) in \(seconds). Finder shows the file." } ?? "Exported. Finder shows the file."
            return ExportSummary(output: URL(fileURLWithPath: result?.output ?? request.output), summary: summary, warnings: warnings)
        case .website:
            let result = try? exit.outcome?.result(BuildResult.self)
            let slides = sessionController?.editor.boxes.count ?? 0
            let summary = result.map { "\(slides) slides, \($0.files) files, \(formatter.string(fromByteCount: $0.bytes)) in \(seconds). Live code does not run in a static site." } ?? "Exported."
            return ExportSummary(output: URL(fileURLWithPath: result?.output ?? request.output), summary: summary, warnings: [])
        case .images:
            let result = try? exit.outcome?.result(ImagesExportResult.self)
            return ExportSummary(output: URL(fileURLWithPath: request.output), summary: "\(result?.files.count ?? 0) images in \(seconds).", warnings: [])
        }
    }
}
