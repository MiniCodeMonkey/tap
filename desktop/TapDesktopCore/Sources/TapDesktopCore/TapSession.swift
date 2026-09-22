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

    private let configuration: Configuration
    private var policy: RestartPolicy
    private var process: TapProcess?
    private var launchGeneration = 0
    private var restartWork: DispatchWorkItem?
    private var readyWork: DispatchWorkItem?
    private var startsAfterStop = false

    public init(deckURL: URL, configuration: Configuration) {
        self.deckURL = deckURL
        self.configuration = configuration
        policy = configuration.policy
        log = TapLog(title: deckURL.deletingPathExtension().lastPathComponent)
    }

    public func start() {
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
            arguments: ["dev", "--app", deckURL.path],
            environment: environment,
            currentDirectoryURL: deckURL.deletingLastPathComponent()))
        tapProcess.onOutputLine = { [weak self] line in self?.receive(line) }
        tapProcess.onStandardErrorLine = { [weak self] line in self?.log.append(line, source: .standardError) }
        tapProcess.onExit = { [weak self] status, requested in self?.processExited(status: status, requested: requested) }
        process = tapProcess
        log.append("tap dev --app \(deckURL.lastPathComponent)", source: .app)
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
        case .question(_, let kind):
            log.append("tap asks a \(kind) question; this version of the app does not answer it", source: .event)
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
        if requested {
            log.append("tap stopped", source: .app)
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
            log.append("tap exited \(policy.maximumExits) times in \(Int(policy.window)) seconds; the app stopped restarting it", source: .app)
            state = .failed(lastOutput: log.lastLines(8, from: .standardError))
        }
    }
}
