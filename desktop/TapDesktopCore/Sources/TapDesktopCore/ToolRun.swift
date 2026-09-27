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
    }

    public static let graceSeconds: TimeInterval = 2

    /// Every run whose process is alive, held weakly, so quitting the app
    /// can stop them all (`applicationWillTerminate` calls `stopAll`) and a
    /// freed run is freed: its escalation holds the process identifier.
    /// Escalation also reads this table (never a captured `self`) to tell
    /// a pid a live run still owns from a pid whose run is gone.
    private static let registry = NSHashTable<ToolRun>.weakObjects()
    public static var activeRuns: [ToolRun] { registry.allObjects.filter(\.isRunning) }

    public var onProgress: ((ProgressLine) -> Void)?
    public var onStandardOutputLine: ((String) -> Void)?
    public var onStandardErrorLine: ((String) -> Void)?
    public var onExit: ((Exit) -> Void)?
    public private(set) var isRunning = false
    public private(set) var processIdentifier: Int32 = 0

    private let configuration: Configuration
    private let process = Process()
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
        // is escalated by its identifier, which the closures hold, not the run.
        try? input?.fileHandleForWriting.close()
        // deinit is not actor-isolated even for a MainActor class, but every
        // ToolRun is made and freed on the main actor, so this is safe.
        if isRunning { MainActor.assumeIsolated { Self.escalate(processIdentifier) } }
    }

    public func start() throws {
        process.executableURL = configuration.executableURL
        process.arguments = configuration.arguments
        process.environment = configuration.environment
        if let directory = configuration.currentDirectoryURL { process.currentDirectoryURL = directory }
        process.standardInput = input ?? FileHandle.nullDevice
        process.standardOutput = output
        process.standardError = errors
        output.fileHandleForReading.readabilityHandler = { [weak self] handle in
            let data = handle.availableData
            guard !data.isEmpty else { return }
            DispatchQueue.main.async { MainActor.assumeIsolated { self?.receive(data, fromStandardError: false) } }
        }
        errors.fileHandleForReading.readabilityHandler = { [weak self] handle in
            let data = handle.availableData
            guard !data.isEmpty else { return }
            DispatchQueue.main.async { MainActor.assumeIsolated { self?.receive(data, fromStandardError: true) } }
        }
        let outputHandle = output.fileHandleForReading
        let errorHandle = errors.fileHandleForReading
        process.terminationHandler = { [weak self] finished in
            outputHandle.readabilityHandler = nil
            errorHandle.readabilityHandler = nil
            let restOfOutput = (try? outputHandle.readToEnd()) ?? Data()
            let restOfErrors = (try? errorHandle.readToEnd()) ?? Data()
            let status = finished.terminationStatus
            DispatchQueue.main.async {
                MainActor.assumeIsolated { self?.didTerminate(status: status, output: restOfOutput, errors: restOfErrors) }
            }
        }
        try process.run()
        isRunning = true
        processIdentifier = process.processIdentifier
        Self.registry.add(self)
        if let timeout = configuration.timeout {
            let work = DispatchWorkItem { [weak self] in
                MainActor.assumeIsolated {
                    guard let self, self.isRunning else { return }
                    // A deadline is Ctrl-C first, so tap closes its browser and
                    // its temporary server, then the same escalation as a cancel.
                    self.timedOut = true
                    Self.escalate(self.processIdentifier)
                }
            }
            deadline = work
            DispatchQueue.main.asyncAfter(deadline: .now() + timeout, execute: work)
        }
    }

    /// Ctrl-C for tap: the kept stdin closes first (a tap serve leaves on
    /// the EOF), then SIGINT now, SIGTERM and SIGKILL after a grace each if
    /// it has not exited. The exit arrives through `onExit` as usual.
    public func cancel() {
        guard isRunning, !cancelled else { return }
        cancelled = true
        closeStandardInput()
        Self.escalate(processIdentifier)
    }

    /// Closes the pipe a `keepsStandardInputOpen` run holds; harmless otherwise.
    public func closeStandardInput() {
        try? input?.fileHandleForWriting.close()
    }

    /// Whether a pid the registry no longer answers for has exited: not yet
    /// reaped (a zombie, which this also reaps), or gone. Only correct for
    /// a pid whose `ToolRun` and `Process` are gone with it (the dropped or
    /// freed run's own escalation, past this file's boundary): a live run's
    /// own `Process` object is reaping this same pid on its own terminationHandler
    /// thread, and a second waitpid here would race it for the exit status.
    /// `stillRunning` below is the one caller of this that a live run can reach.
    public static func isAlive(_ identifier: Int32) -> Bool {
        var status: Int32 = 0
        let result = waitpid(identifier, &status, WNOHANG)
        return result == 0
    }

    /// Whether `identifier` is still going, asked the safe way: a live
    /// `ToolRun` still in the registry answers for its own pid through
    /// `isRunning`, never through `waitpid`, since Foundation's own
    /// termination handler is the one thing reaping that pid and a second
    /// reap here could steal its exit status out from under it. Only once
    /// no live run answers for the pid (freed with its window, or dropped
    /// with its process still running) does `isAlive`'s `waitpid` become
    /// the only thing left that can tell, and the only thing left to reap it.
    private static func stillRunning(_ identifier: Int32) -> Bool {
        if let owner = registry.allObjects.first(where: { $0.processIdentifier == identifier }) {
            return owner.isRunning
        }
        return isAlive(identifier)
    }

    /// SIGINT, then SIGTERM and SIGKILL after `graceSeconds` each while the
    /// process is still going, then a reap. This looks the process up in
    /// the registry by identifier at each step, never through a captured
    /// `self`: a run freed with its window (D4's stopAndRetain lesson)
    /// still finishes killing its process, because nothing here goes nil
    /// when the run does, and a run freed while its process ran (whose
    /// `Process` object is gone with it) has its zombie reaped here, once,
    /// through `stillRunning`'s fallback. A reused identifier within four
    /// seconds is the one risk this takes, the same risk every signal to a
    /// child takes.
    public static func escalate(_ identifier: Int32) {
        Darwin.kill(identifier, SIGINT)
        DispatchQueue.main.asyncAfter(deadline: .now() + graceSeconds) {
            guard stillRunning(identifier) else { return }
            Darwin.kill(identifier, SIGTERM)
            DispatchQueue.main.asyncAfter(deadline: .now() + graceSeconds) {
                guard stillRunning(identifier) else { return }
                Darwin.kill(identifier, SIGKILL)
                DispatchQueue.main.asyncAfter(deadline: .now() + 0.5) { _ = stillRunning(identifier) }
            }
        }
    }

    /// Cancels every live run. With `waiting` greater than zero (the app is
    /// quitting, and the main queue's later escalation dies with it), this
    /// blocks up to that long for the exits and sends SIGTERM to what is
    /// still running, as stopAllPresentations does for talks; a tap that
    /// ignores SIGTERM too is left to the system, which reaps it with the
    /// app's process group. The wait reads liveness through `stillRunning`,
    /// the same registry-first check `escalate` uses, never a bare `waitpid`
    /// on a pid these runs' own `Process` objects still own.
    public static func stopAll(waiting graceSeconds: TimeInterval = 0) {
        let identifiers = activeRuns.map(\.processIdentifier)
        for run in activeRuns { run.cancel() }
        guard graceSeconds > 0 else { return }
        let deadline = Date().addingTimeInterval(graceSeconds)
        while Date() < deadline, identifiers.contains(where: stillRunning) { usleep(50_000) }
        for identifier in identifiers where stillRunning(identifier) { Darwin.kill(identifier, SIGTERM) }
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

    private func didTerminate(status: Int32, output restOfOutput: Data, errors restOfErrors: Data) {
        deadline?.cancel()
        receive(restOfOutput, fromStandardError: false)
        receive(restOfErrors, fromStandardError: true)
        if let line = outputBuffer.finish() { onStandardOutputLine?(line) }
        if let line = errorBuffer.finish() { handleErrorLine(line) }
        isRunning = false
        try? input?.fileHandleForWriting.close()
        Self.registry.remove(self)
        let outcome = lastOutcome ?? (timedOut ? nil : ToolOutcome.decode(collectedOutput))
        onExit?(Exit(status: status, standardOutput: collectedOutput, cancelled: cancelled, timedOut: timedOut, outcome: outcome))
    }
}
