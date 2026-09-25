import Foundation

/// One `tap dev --app` child process. Standard input stays open for the
/// process's whole life: tap exits when it closes, so a crashed app never
/// leaves tap running. Every callback runs on the main actor, in order.
@MainActor
public final class TapProcess {
    public struct Configuration: Sendable {
        public let executableURL: URL
        public let arguments: [String]
        public let environment: [String: String]
        public let currentDirectoryURL: URL

        public init(executableURL: URL, arguments: [String], environment: [String: String], currentDirectoryURL: URL) {
            self.executableURL = executableURL
            self.arguments = arguments
            self.environment = environment
            self.currentDirectoryURL = currentDirectoryURL
        }
    }

    public var onOutputLine: ((String) -> Void)?
    public var onStandardErrorLine: ((String) -> Void)?
    public var onExit: ((_ status: Int32, _ requested: Bool) -> Void)?
    public private(set) var isRunning = false
    public private(set) var processIdentifier: Int32 = 0

    private let configuration: Configuration
    private let process = Process()
    private let input = Pipe()
    private let output = Pipe()
    private let errors = Pipe()
    private var outputBuffer = LineBuffer()
    private var errorBuffer = LineBuffer()
    private var stopRequested = false

    public init(configuration: Configuration) {
        self.configuration = configuration
        // A write to a closed pipe must fail with an error, not kill the app.
        signal(SIGPIPE, SIG_IGN)
    }

    public func start() throws {
        process.executableURL = configuration.executableURL
        process.arguments = configuration.arguments
        process.environment = configuration.environment
        process.currentDirectoryURL = configuration.currentDirectoryURL
        process.standardInput = input
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
    }

    public func send(_ command: TapCommand) {
        guard isRunning, !stopRequested else { return }
        try? input.fileHandleForWriting.write(contentsOf: Data((command.line + "\n").utf8))
    }

    /// Closes standard input, so tap shuts down. A tap that is still running
    /// after `graceSeconds` gets SIGTERM, and SIGKILL after as long again.
    public func stop(graceSeconds: TimeInterval = 2) {
        guard isRunning, !stopRequested else { return }
        stopRequested = true
        try? input.fileHandleForWriting.close()
        let identifier = processIdentifier
        DispatchQueue.main.asyncAfter(deadline: .now() + graceSeconds) { [weak self] in
            MainActor.assumeIsolated {
                guard let self, self.isRunning else { return }
                Darwin.kill(identifier, SIGTERM)
                DispatchQueue.main.asyncAfter(deadline: .now() + graceSeconds) { [weak self] in
                    MainActor.assumeIsolated {
                        guard let self, self.isRunning else { return }
                        Darwin.kill(identifier, SIGKILL)
                    }
                }
            }
        }
    }

    /// Kills tap at once. The exit counts as unexpected, like a crash.
    public func kill() {
        guard isRunning else { return }
        Darwin.kill(processIdentifier, SIGKILL)
    }

    private func receive(_ data: Data, fromStandardError: Bool) {
        if fromStandardError {
            for line in errorBuffer.append(data) { onStandardErrorLine?(line) }
        } else {
            for line in outputBuffer.append(data) { onOutputLine?(line) }
        }
    }

    private func didTerminate(status: Int32, output restOfOutput: Data, errors restOfErrors: Data) {
        receive(restOfOutput, fromStandardError: false)
        receive(restOfErrors, fromStandardError: true)
        if let line = outputBuffer.finish() { onOutputLine?(line) }
        if let line = errorBuffer.finish() { onStandardErrorLine?(line) }
        isRunning = false
        try? input.fileHandleForWriting.close()
        onExit?(status, stopRequested)
    }
}
