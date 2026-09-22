import Foundation

/// The environment of the user's login shell. An app started from Finder
/// does not inherit it, and tap and its live code drivers need its PATH and
/// variables, so the app reads it once at launch.
public struct LoginShellEnvironment: Equatable, Sendable {
    public enum Outcome: Equatable, Sendable {
        case loaded
        case failed(String)
    }

    public static let beginMarker = "__TAP_ENVIRONMENT_BEGIN__"
    public static let endMarker = "__TAP_ENVIRONMENT_END__"

    public let variables: [String: String]
    public let outcome: Outcome

    public var notice: String? {
        guard case .failed(let reason) = outcome else { return nil }
        return "Tap could not read your login shell environment (\(reason)). tap runs with the default environment."
    }

    /// The variables printed by `env -0` between the two markers.
    public static func parse(_ output: Data) -> [String: String]? {
        let text = String(decoding: output, as: UTF8.self)
        guard let begin = text.range(of: beginMarker),
              let end = text.range(of: endMarker, range: begin.upperBound..<text.endIndex) else { return nil }
        var variables: [String: String] = [:]
        for record in text[begin.upperBound..<end.lowerBound].split(separator: "\u{0}") {
            guard let equals = record.firstIndex(of: "="), equals != record.startIndex else { continue }
            variables[String(record[..<equals])] = String(record[record.index(after: equals)...])
        }
        return variables
    }

    /// Runs `<shell> -l -i -c <script>`, where the script prints `env -0`
    /// between the markers, so a profile that prints text cannot spoil it.
    public static func load(shellPath: String, timeout: TimeInterval = 5,
                            fallback: [String: String] = ProcessInfo.processInfo.environment) async -> LoginShellEnvironment {
        await withCheckedContinuation { continuation in
            DispatchQueue.global(qos: .userInitiated).async {
                continuation.resume(returning: run(shellPath: shellPath, timeout: timeout, fallback: fallback))
            }
        }
    }

    private static func run(shellPath: String, timeout: TimeInterval, fallback: [String: String]) -> LoginShellEnvironment {
        func failed(_ reason: String) -> LoginShellEnvironment {
            LoginShellEnvironment(variables: fallback, outcome: .failed(reason))
        }
        let script = "printf '%s' '\(beginMarker)'; /usr/bin/env -0; printf '%s' '\(endMarker)'"
        let process = Process()
        process.executableURL = URL(fileURLWithPath: shellPath)
        process.arguments = ["-l", "-i", "-c", script]
        process.standardInput = FileHandle.nullDevice
        process.standardError = FileHandle.nullDevice
        let output = Pipe()
        process.standardOutput = output

        let lock = NSLock()
        var collected = Data()
        output.fileHandleForReading.readabilityHandler = { handle in
            let chunk = handle.availableData
            lock.lock()
            collected.append(chunk)
            lock.unlock()
        }
        let exited = DispatchSemaphore(value: 0)
        process.terminationHandler = { _ in exited.signal() }
        do {
            try process.run()
        } catch {
            output.fileHandleForReading.readabilityHandler = nil
            return failed("\(shellPath) did not start")
        }
        if exited.wait(timeout: .now() + timeout) == .timedOut {
            output.fileHandleForReading.readabilityHandler = nil
            kill(process.processIdentifier, SIGKILL)
            return failed("it took longer than \(Int(timeout.rounded(.up))) seconds")
        }
        // The last chunk can arrive just after the exit.
        let drainDeadline = Date().addingTimeInterval(0.5)
        func snapshot() -> Data {
            lock.lock()
            defer { lock.unlock() }
            return collected
        }
        while parse(snapshot()) == nil, Date() < drainDeadline { usleep(10_000) }
        output.fileHandleForReading.readabilityHandler = nil
        guard process.terminationStatus == 0 else { return failed("the shell exited with status \(process.terminationStatus)") }
        guard var variables = parse(snapshot()), !variables.isEmpty else { return failed("the shell printed no environment") }
        if variables["PATH"] == nil { variables["PATH"] = fallback["PATH"] ?? "/usr/bin:/bin:/usr/sbin:/sbin" }
        return LoginShellEnvironment(variables: variables, outcome: .loaded)
    }
}

/// Reads the login shell environment once and hands the same answer to
/// every caller.
public actor LoginShellEnvironmentLoader {
    private let shellPath: String
    private let timeout: TimeInterval
    private let fallback: [String: String]
    private var task: Task<LoginShellEnvironment, Never>?

    public init(shellPath: String, timeout: TimeInterval = 5, fallback: [String: String] = ProcessInfo.processInfo.environment) {
        self.shellPath = shellPath
        self.timeout = timeout
        self.fallback = fallback
    }

    public func environment() async -> LoginShellEnvironment {
        if let task { return await task.value }
        let shellPath = shellPath, timeout = timeout, fallback = fallback
        let newTask = Task { await LoginShellEnvironment.load(shellPath: shellPath, timeout: timeout, fallback: fallback) }
        task = newTask
        return await newTask.value
    }
}
