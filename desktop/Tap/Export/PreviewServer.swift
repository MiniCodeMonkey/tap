import Foundation

/// `tap serve <folder> --port 0 --json` for the export sheet's Preview: tap
/// picks a free port and prints one ready line; the server runs until
/// the sheet or the deck closes, when SIGINT stops it.
@MainActor
final class PreviewServer {
    private var run: ToolRun?
    private(set) var url: URL?
    private let log: TapLog?

    init(log: TapLog?) {
        self.log = log
    }

    var isRunning: Bool { run?.isRunning ?? false }
    var processIdentifier: Int32? { run.map(\.processIdentifier) }

    /// Starts serving and reports the URL from tap's ready line, or nil
    /// when tap did not start (its error is in the log).
    func start(folder: URL, completion: @escaping (URL?) -> Void) {
        stop()
        Task { @MainActor [weak self] in
            guard let self else { return }
            // The run holds tap's stdin open; tap serve --json exits on its EOF, so a server never outlives the app.
            let run = await TapTool.makeRun(["serve", folder.path, "--port", "0", "--json"], in: folder, timeout: nil, log: self.log, keepsStandardInputOpen: true)
            var answered = false
            run.onStandardOutputLine = { [weak self] line in
                guard !answered, let outcome = ToolOutcome.decode(Data(line.utf8)) else { return }
                answered = true
                let ready = try? outcome.result(ServeReady.self)
                self?.url = ready.flatMap { URL(string: $0.url) }
                completion(self?.url)
            }
            run.onExit = { [weak self] exit in
                self?.run = nil
                self?.url = nil
                if !answered {
                    answered = true
                    self?.log?.append("tap serve did not start (exit \(exit.status))", source: .app)
                    completion(nil)
                }
            }
            self.run = run
            do { try run.start() } catch {
                self.run = nil
                if !answered { answered = true; completion(nil) }
            }
        }
    }

    func stop() {
        run?.cancel()
    }
}
