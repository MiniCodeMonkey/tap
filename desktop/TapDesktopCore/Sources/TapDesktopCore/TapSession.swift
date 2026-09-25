import Foundation

/// The tap process of one open deck: `tap dev --app <deck>`. It restarts tap
/// with backoff when tap exits unexpectedly, gives up at the third exit
/// within thirty seconds, and starts over on Try Again.
@MainActor
public final class TapSession {
    public enum State: Equatable, Sendable {
        case stopped
        case starting
        case running(TapReady)
        case restarting(after: TimeInterval)
        case failed(lastOutput: [String])
    }

    /// Which tap process this session runs. `dev` is the deck's writing
    /// process; `present` is a talk, started beside it.
    public enum Command: Equatable, Sendable {
        case dev
        /// `record` false adds `--no-record` (Rehearse, or Play with the
        /// record checkbox off); `presenterPassword` is the person's own,
        /// otherwise tap generates one and prints it on the ready line;
        /// `port` is the deck's remembered port, which tap binds exactly
        /// (or fails), and nil lets tap pick a free one.
        case present(record: Bool, presenterPassword: String?, port: Int?)

        public func arguments(deck: URL) -> [String] {
            switch self {
            case .dev:
                return ["dev", "--app", deck.path]
            case .present(let record, let presenterPassword, let port):
                var arguments = ["present", "--app"]
                if !record { arguments.append("--no-record") }
                if let presenterPassword, !presenterPassword.isEmpty {
                    arguments += ["--presenter-password", presenterPassword]
                }
                if let port { arguments += ["--port", String(port)] }
                arguments.append(deck.path)
                return arguments
            }
        }

        /// The port a present command asks for, nil for none and for dev.
        public var port: Int? {
            if case .present(_, _, let port) = self { return port }
            return nil
        }

        /// The log line for a start: the arguments with the deck's name in
        /// place of its path and the presenter password hidden, since the
        /// Tap Log is copied into bug reports.
        public func logLine(deck: URL) -> String {
            var shown = arguments(deck: deck).dropLast() + [deck.lastPathComponent]
            if let index = shown.firstIndex(of: "--presenter-password"), shown.indices.contains(index + 1) {
                shown[index + 1] = "***"
            }
            return "tap " + shown.joined(separator: " ")
        }

        /// The Tap Log title: the deck's name, and ", talk" for a talk.
        public func logTitle(deck: URL) -> String {
            let name = deck.deletingPathExtension().lastPathComponent
            if case .present = self { return name + ", talk" }
            return name
        }
    }

    public struct Configuration {
        public var executableURL: URL
        public var environment: () async -> [String: String]
        public var readyTimeout: TimeInterval
        public var policy: RestartPolicy
        public var clock: () -> Date

        public init(executableURL: URL, environment: @escaping () async -> [String: String],
                    readyTimeout: TimeInterval = 20, policy: RestartPolicy = RestartPolicy(), clock: @escaping () -> Date = Date.init) {
            self.executableURL = executableURL
            self.environment = environment
            self.readyTimeout = readyTimeout
            self.policy = policy
            self.clock = clock
        }
    }

    public private(set) var state: State = .stopped {
        didSet { if state != oldValue { onStateChange?(state) } }
    }
    public private(set) var deckURL: URL
    public let log: TapLog
    /// While false, an exit leaves the session stopped. The app pauses
    /// restarts while the deck file is deleted.
    public var restartsWhenExited = true
    public var onStateChange: ((State) -> Void)?
    public var onEvent: ((TapEvent) -> Void)?
    public var processIdentifier: Int32? { process?.isRunning == true ? process?.processIdentifier : nil }
    /// The restart policy this session runs, so a caller can report the same
    /// exit count and window the session itself logs on giving up.
    public var restartPolicy: RestartPolicy { policy }

    public let command: Command
    /// True from `quit()` until the next `start()`: the exit that follows
    /// is tap answering the quit command, never a crash.
    public private(set) var quitRequested = false
    private var quitWork: DispatchWorkItem?

    private let configuration: Configuration
    private var policy: RestartPolicy
    private var process: TapProcess?
    private var launchGeneration = 0
    private var restartWork: DispatchWorkItem?
    private var readyWork: DispatchWorkItem?
    private var startsAfterStop = false

    public init(deckURL: URL, configuration: Configuration, command: Command = .dev) {
        self.deckURL = deckURL
        self.configuration = configuration
        self.command = command
        policy = configuration.policy
        log = TapLog(title: command.logTitle(deck: deckURL))
    }

    public func start() {
        quitRequested = false
        quitWork?.cancel()
        restartWork?.cancel()
        guard process == nil else { return }
        state = .starting
        launchGeneration += 1
        let generation = launchGeneration
        Task { @MainActor in
            let environment = await configuration.environment()
            guard generation == self.launchGeneration, self.state == .starting else { return }
            self.launch(environment: environment)
        }
    }

    public func stop() {
        restartWork?.cancel()
        readyWork?.cancel()
        quitWork?.cancel()
        startsAfterStop = false
        launchGeneration += 1
        if let process {
            process.stop()
        } else {
            state = .stopped
        }
    }

    public func tryAgain() {
        policy.reset()
        log.append("Try Again", source: .app)
        start()
    }

    /// Asks tap to shut down with the `quit` command, which lets tap present
    /// ask keep-recording and finish its recording, and closes standard
    /// input after `timeout` if tap is still running then (D2's stop, with
    /// its SIGTERM and SIGKILL escalation). The exit that follows counts as
    /// requested: nothing restarts.
    public func quit(timeout: TimeInterval = 15) {
        restartWork?.cancel()
        readyWork?.cancel()
        quitWork?.cancel()
        startsAfterStop = false
        launchGeneration += 1
        quitRequested = true
        guard let process else {
            state = .stopped
            return
        }
        process.send(.quit)
        let deadline = DispatchWorkItem { [weak self, weak process] in
            MainActor.assumeIsolated {
                guard let self, let process, self.process === process else { return }
                self.log.append("tap did not quit within \(String(format: "%.1f", timeout)) seconds", source: .app)
                process.stop()
            }
        }
        quitWork = deadline
        DispatchQueue.main.asyncAfter(deadline: .now() + timeout, execute: deadline)
    }

    /// Moves the quit deadline to `timeout` from now, for a tap that has
    /// asked keep-recording and is waiting for a person: the deadline
    /// must outlast tap's own wait for the answer. Nothing happens unless
    /// a quit is in progress.
    public func extendQuit(timeout: TimeInterval) {
        guard quitRequested, let process else { return }
        quitWork?.cancel()
        let deadline = DispatchWorkItem { [weak self, weak process] in
            MainActor.assumeIsolated {
                guard let self, let process, self.process === process else { return }
                self.log.append("tap did not quit within \(String(format: "%.1f", timeout)) seconds", source: .app)
                process.stop()
            }
        }
        quitWork = deadline
        DispatchQueue.main.asyncAfter(deadline: .now() + timeout, execute: deadline)
    }

    /// Follows a renamed or newly saved deck: tap restarts on the new path.
    public func changeDeck(to url: URL) {
        guard !FilePaths.same(url, deckURL) else { return }
        deckURL = url
        log.title = url.deletingPathExtension().lastPathComponent
        log.append("the deck is now \(url.path)", source: .app)
        if let process {
            startsAfterStop = true
            process.stop()
        }
    }

    public func send(_ command: TapCommand) {
        process?.send(command)
    }

    private func launch(environment: [String: String]) {
        let tapProcess = TapProcess(configuration: TapProcess.Configuration(
            executableURL: configuration.executableURL,
            arguments: command.arguments(deck: deckURL),
            environment: environment,
            currentDirectoryURL: deckURL.deletingLastPathComponent()))
        tapProcess.onOutputLine = { [weak self] line in self?.receive(line) }
        tapProcess.onStandardErrorLine = { [weak self] line in self?.log.append(line, source: .standardError) }
        tapProcess.onExit = { [weak self] status, requested in self?.processExited(status: status, requested: requested) }
        process = tapProcess
        log.append(command.logLine(deck: deckURL), source: .app)
        do {
            try tapProcess.start()
        } catch {
            process = nil
            log.append("tap did not start: \(error.localizedDescription)", source: .app)
            unexpectedExit()
            return
        }
        let timeout = DispatchWorkItem { [weak self, weak tapProcess] in
            MainActor.assumeIsolated {
                guard let self, let tapProcess, self.process === tapProcess, self.state == .starting else { return }
                self.log.append("tap did not print its ready line within \(Int(self.configuration.readyTimeout)) seconds", source: .app)
                tapProcess.kill()
            }
        }
        readyWork = timeout
        DispatchQueue.main.asyncAfter(deadline: .now() + configuration.readyTimeout, execute: timeout)
    }

    private func receive(_ line: String) {
        guard let event = TapEvent.decode(line: line) else {
            log.append(line, source: .standardOutput)
            return
        }
        switch event {
        case .ready(let ready):
            readyWork?.cancel()
            log.append("ready on 127.0.0.1:\(ready.port)", source: .event)
            if state == .starting { state = .running(ready) }
        case .fileChanged(let path, _):
            log.append("file changed on disk: \((path as NSString).lastPathComponent)", source: .event)
        case .question(_, let kind, _):
            log.append("tap asks a \(kind) question", source: .event)
        case .recording(let recording):
            log.append("recording \(recording.state), segment \(recording.segment), disk \(recording.disk)", source: .event)
        case .tunnel(let tunnel):
            log.append("tunnel \(tunnel.state)" + (tunnel.url.map { " \($0)" } ?? ""), source: .event)
        case .slide(let slide, let step):
            log.append("audience on slide \(slide), step \(step)", source: .event)
        case .error(let payload):
            log.append("error \(payload.code): \(payload.message)", source: .event)
        case .other(let type):
            log.append("event \(type)", source: .event)
        }
        onEvent?(event)
    }

    private func processExited(status: Int32, requested: Bool) {
        readyWork?.cancel()
        process = nil
        if requested || quitRequested {
            quitWork?.cancel()
            log.append(quitRequested ? "tap quit" : "tap stopped", source: .app)
            quitRequested = false
            state = .stopped
            if startsAfterStop {
                startsAfterStop = false
                start()
            }
            return
        }
        log.append("tap exited with status \(status)", source: .app)
        unexpectedExit()
    }

    private func unexpectedExit() {
        guard restartsWhenExited else {
            state = .stopped
            return
        }
        switch policy.recordExit(at: configuration.clock()) {
        case .restart(let delay):
            state = .restarting(after: delay)
            let work = DispatchWorkItem { [weak self] in
                MainActor.assumeIsolated {
                    guard let self, case .restarting = self.state else { return }
                    self.start()
                }
            }
            restartWork = work
            DispatchQueue.main.asyncAfter(deadline: .now() + delay, execute: work)
        case .giveUp:
            log.append("\(policy.exitSummary); the app stopped restarting it", source: .app)
            state = .failed(lastOutput: log.lastLines(8, from: .standardError))
        }
    }
}
