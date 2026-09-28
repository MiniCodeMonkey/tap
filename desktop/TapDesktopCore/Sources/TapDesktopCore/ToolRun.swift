import Foundation

/// One `--progress json` line on a tap command's standard error
/// (internal/cli/progress.go): a step, the export engine's download, or
/// the final done line with the command's result or its error.
public enum ProgressLine: Equatable, Sendable {
    case step(phase: String, done: Int, total: Int)
    case download(bytes: Int64, totalBytes: Int64)
    case finished(ToolOutcome)

    /// nil for a line that is not progress (a warning tap prints for a person).
    public static func decode(line: String) -> ProgressLine? {
        guard line.hasPrefix("{"), let data = line.data(using: .utf8),
              let object = try? JSONSerialization.jsonObject(with: data) as? [String: Any],
              let phase = object["phase"] as? String else { return nil }
        switch phase {
        case "download":
            guard let bytes = object["bytes"] as? NSNumber, let total = object["totalBytes"] as? NSNumber else { return nil }
            return .download(bytes: bytes.int64Value, totalBytes: total.int64Value)
        case "done":
            return ToolOutcome.decode(data).map(ProgressLine.finished)
        default:
            guard let done = object["done"] as? NSNumber, let total = object["total"] as? NSNumber else { return nil }
            return .step(phase: phase, done: done.intValue, total: total.intValue)
        }
    }
}

/// What a tap command answered: its `--json` object (the whole object, so
/// a typed decoder reads the fields it wants), or tap's error. The same
/// shape ends a `--progress json` run's done line.
public enum ToolOutcome: Equatable, Sendable {
    case ok(Data)
    case failed(code: String, message: String)

    public static func decode(_ data: Data) -> ToolOutcome? {
        guard let object = try? JSONSerialization.jsonObject(with: data) as? [String: Any], let ok = object["ok"] as? Bool else { return nil }
        if ok { return .ok(data) }
        let error = object["error"] as? [String: Any]
        return .failed(code: error?["code"] as? String ?? "failed", message: error?["message"] as? String ?? "tap failed")
    }

    /// The result's fields, or the error as `ToolError.failed`.
    public func result<Result: Decodable>(_ type: Result.Type) throws -> Result {
        switch self {
        case .ok(let data): return try JSONDecoder().decode(type, from: data)
        case .failed(let code, let message): throw ToolError.failed(code: code, message: message)
        }
    }

    public var message: String? {
        if case .failed(_, let message) = self { return message }
        return nil
    }
}

public enum ToolError: Error, Equatable {
    /// tap answered with an error: its code and its message, to show as they are.
    case failed(code: String, message: String)
    /// tap printed no result the app can read (a crash, a kill, a timeout).
    case noResult(status: Int32)
    case cancelled
}

/// One run of a tap subcommand that ends on its own: `tap new`, `tap theme
/// set`, `tap export pdf` and the rest. Standard output is collected for
/// the `--json` result; standard error is read line by line, progress
/// lines to `onProgress` and the others to `onStandardErrorLine` (the Tap
/// Log). Standard input is closed, so a tap that would ask a question
/// cannot wait on one. `cancel` sends SIGINT, the signal Ctrl-C sends and
/// tap's export commands clean up on, then SIGTERM and SIGKILL if tap is
/// still running two seconds later each. A run past its timeout is killed
/// and reports `timedOut`. Every callback runs on the main actor, in order.
@MainActor
public final class ToolRun {
    public struct Configuration: Sendable {
        public let executableURL: URL
        public let arguments: [String]
        public let environment: [String: String]
        public let currentDirectoryURL: URL?
        /// nil for a run with no deadline (`tap serve`, stopped by `cancel`).
        public let timeout: TimeInterval?
        /// True hands the process a pipe the run holds open until it is
        /// cancelled or freed (`tap serve --json` exits on EOF); false gives
        /// it /dev/null, so a tap that would ask a question cannot wait.
        public let keepsStandardInputOpen: Bool

        public init(executableURL: URL, arguments: [String], environment: [String: String], currentDirectoryURL: URL?, timeout: TimeInterval?,
                    keepsStandardInputOpen: Bool = false) {
            self.executableURL = executableURL
            self.arguments = arguments
            self.environment = environment
            self.currentDirectoryURL = currentDirectoryURL
            self.timeout = timeout
            self.keepsStandardInputOpen = keepsStandardInputOpen
        }
    }

    public struct Exit: Equatable, Sendable {
        public let status: Int32
        public let standardOutput: Data
        public let cancelled: Bool
        public let timedOut: Bool
        /// The done line's outcome, else the standard output's `--json` object, else nil.
        public let outcome: ToolOutcome?

        /// A run cancelled before its process started: status -1, no output.
        public static let cancelledBeforeStart = Exit(status: -1, standardOutput: Data(), cancelled: true, timedOut: false, outcome: nil)
    }

    public nonisolated static let graceSeconds: TimeInterval = 2

    /// Every run whose process is alive, held weakly, so quitting the app
    /// can stop them all (`applicationWillTerminate` calls `stopAll`) and a
    /// freed run is freed: its escalation holds the `Process`, not the run.
    private static let registry = NSHashTable<ToolRun>.weakObjects()
    public static var activeRuns: [ToolRun] { registry.allObjects.filter(\.isRunning) }

    public var onProgress: ((ProgressLine) -> Void)?
    public var onStandardOutputLine: ((String) -> Void)?
    public var onStandardErrorLine: ((String) -> Void)?
    public var onExit: ((Exit) -> Void)?
    public private(set) var isRunning = false
    public private(set) var processIdentifier: Int32 = 0

    private let configuration: Configuration
    /// Liveness is `process.isRunning`, which Foundation updates on its own
    /// queue as it reaps the child, so it turns false even while the main
    /// thread is blocked. Nothing here ever calls waitpid on this pid.
    let process = Process()
    private let input: Pipe?
    private let output = Pipe()
    private let errors = Pipe()
    private var outputBuffer = LineBuffer()
    private var errorBuffer = LineBuffer()
    private var collectedOutput = Data()
    private var lastOutcome: ToolOutcome?
    private var cancelled = false
    private var timedOut = false
    private var deadline: DispatchWorkItem?

    public init(configuration: Configuration) {
        self.configuration = configuration
        input = configuration.keepsStandardInputOpen ? Pipe() : nil
        signal(SIGPIPE, SIG_IGN)
    }

    deinit {
        // A run dropped while its process runs leaves no orphan: the pipe it
        // held open closes (a tap serve waiting on EOF exits) and the process
        // is escalated. The last reference may drop on any thread, so this
        // touches only the Process, and escalate is not actor-isolated.
        try? input?.fileHandleForWriting.close()
        if process.isRunning { Self.escalate(process) }
    }

    public func start() throws {
        process.executableURL = configuration.executableURL
        process.arguments = configuration.arguments
        process.environment = configuration.environment
        if let directory = configuration.currentDirectoryURL { process.currentDirectoryURL = directory }
        process.standardInput = input ?? FileHandle.nullDevice
        process.standardOutput = output
        process.standardError = errors
        // Each pipe is read to EOF on its own thread, every chunk handed to
        // the main queue in order. The exit is handed over only once both
        // pipes reached EOF and the process terminated, so every chunk of
        // output is received before the result is decoded.
        let finished = DispatchGroup()
        finished.enter()
        process.terminationHandler = { _ in finished.leave() }
        try process.run()
        isRunning = true
        processIdentifier = process.processIdentifier
        Self.registry.add(self)
        for (handle, fromStandardError) in [(output.fileHandleForReading, false), (errors.fileHandleForReading, true)] {
            finished.enter()
            Thread.detachNewThread { [weak self] in
                while case let data = handle.availableData, !data.isEmpty {
                    DispatchQueue.main.async { MainActor.assumeIsolated { self?.receive(data, fromStandardError: fromStandardError) } }
                }
                finished.leave()
            }
        }
        finished.notify(queue: .main) { [weak self] in
            MainActor.assumeIsolated { self?.didTerminate() }
        }
        if let timeout = configuration.timeout {
            let work = DispatchWorkItem { [weak self] in
                MainActor.assumeIsolated {
                    guard let self, self.isRunning else { return }
                    // A deadline is Ctrl-C first, so tap closes its browser and
                    // its temporary server, then the same escalation as a cancel.
                    self.timedOut = true
                    Self.escalate(self.process)
                }
            }
            deadline = work
            DispatchQueue.main.asyncAfter(deadline: .now() + timeout, execute: work)
        }
    }

    /// Ctrl-C for tap: the kept stdin closes first (a tap serve leaves on
    /// the EOF), then SIGINT now, SIGTERM and SIGKILL after a grace each if
    /// it has not exited. A second cancel sends nothing: tap takes a second
    /// SIGINT as "die now" and would skip its cleanup. The exit arrives
    /// through `onExit` as usual.
    public func cancel() {
        guard isRunning, !cancelled else { return }
        cancelled = true
        closeStandardInput()
        Self.escalate(process)
    }

    /// Closes the pipe a `keepsStandardInputOpen` run holds; harmless otherwise.
    public func closeStandardInput() {
        try? input?.fileHandleForWriting.close()
    }

    /// SIGINT, then SIGTERM and SIGKILL after `graceSeconds` each while the
    /// process is still running. The steps hold the `Process`, never the
    /// run, so a run freed with its window still finishes killing its
    /// process, and they send no signal once the `Process` reports it has
    /// exited, so a pid that the system may have reused is never signalled.
    nonisolated static func escalate(_ process: Process) {
        guard process.isRunning else { return }
        let identifier = process.processIdentifier
        Darwin.kill(identifier, SIGINT)
        DispatchQueue.main.asyncAfter(deadline: .now() + graceSeconds) {
            guard process.isRunning else { return }
            Darwin.kill(identifier, SIGTERM)
            DispatchQueue.main.asyncAfter(deadline: .now() + graceSeconds) {
                guard process.isRunning else { return }
                Darwin.kill(identifier, SIGKILL)
            }
        }
    }

    /// Cancels every live run. With `waiting` greater than zero (the app is
    /// quitting, and the main queue's later escalation dies with it), this
    /// blocks until every process has exited or the wait is over, then sends
    /// SIGTERM to what is still running, as stopAllPresentations does for
    /// talks; a tap that ignores SIGTERM too is left to the system, which
    /// reaps it with the app's process group.
    public static func stopAll(waiting graceSeconds: TimeInterval = 0) {
        let runs = activeRuns
        let processes = runs.map(\.process)
        for run in runs { run.cancel() }
        guard graceSeconds > 0 else { return }
        let deadline = Date().addingTimeInterval(graceSeconds)
        while Date() < deadline, processes.contains(where: \.isRunning) { usleep(20_000) }
        for process in processes where process.isRunning { Darwin.kill(process.processIdentifier, SIGTERM) }
    }

    /// Starts and waits. A start failure is an exit with status -1 and no outcome.
    public func run() async -> Exit {
        await withCheckedContinuation { continuation in
            var resumed = false
            onExit = { exit in
                guard !resumed else { return }
                resumed = true
                continuation.resume(returning: exit)
            }
            do {
                try start()
            } catch {
                resumed = true
                continuation.resume(returning: Exit(status: -1, standardOutput: Data(), cancelled: false, timedOut: false, outcome: nil))
            }
        }
    }

    private func receive(_ data: Data, fromStandardError: Bool) {
        if fromStandardError {
            for line in errorBuffer.append(data) { handleErrorLine(line) }
        } else {
            collectedOutput.append(data)
            for line in outputBuffer.append(data) { onStandardOutputLine?(line) }
        }
    }

    private func handleErrorLine(_ line: String) {
        if let progress = ProgressLine.decode(line: line) {
            if case .finished(let outcome) = progress { lastOutcome = outcome }
            onProgress?(progress)
        } else {
            onStandardErrorLine?(line)
        }
    }

    private func didTerminate() {
        let status = process.terminationStatus
        deadline?.cancel()
        if let line = outputBuffer.finish() { onStandardOutputLine?(line) }
        if let line = errorBuffer.finish() { handleErrorLine(line) }
        isRunning = false
        try? input?.fileHandleForWriting.close()
        Self.registry.remove(self)
        let outcome = lastOutcome ?? (timedOut ? nil : ToolOutcome.decode(collectedOutput))
        onExit?(Exit(status: status, standardOutput: collectedOutput, cancelled: cancelled, timedOut: timedOut, outcome: outcome))
    }
}
