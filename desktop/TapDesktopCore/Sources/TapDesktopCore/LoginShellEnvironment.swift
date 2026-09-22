import Foundation

/// The environment of the user's login shell. An app started from Finder
/// does not inherit it, and tap and its live code drivers need its PATH and
/// variables, so the app reads it once at launch.
public struct LoginShellEnvironment: Equatable, Sendable {
    public enum Outcome: Equatable, Sendable {
        case loaded
        case failed(String)
    }

    public let variables: [String: String]
    public let outcome: Outcome

    public var notice: String? {
        guard case .failed(let reason) = outcome else { return nil }
        return "Tap could not read your login shell environment (\(reason)). tap runs with the default environment."
    }

    /// A random, unguessable marker pair for one run of the login shell.
    ///
    /// Earlier versions used fixed marker literals, which anything a dotfile
    /// prints (by accident, or by a value that happens to contain the
    /// literal text) could collide with. A per-run token nothing has ever
    /// seen before closes that off entirely: no dotfile can print a token
    /// it was never given.
    struct Markers: Equatable {
        let token: String
        var begin: String { "__TAP_ENVIRONMENT_BEGIN_\(token)__" }
        var end: String { "__TAP_ENVIRONMENT_END_\(token)__" }

        /// 128 bits from the system random source, rendered as lowercase
        /// hex so the token is only letters and digits: it is embedded in a
        /// single-quoted string the shell executes, and this alphabet
        /// cannot break out of that quoting.
        static func generate() -> Markers {
            let bytes = (0..<16).map { _ in UInt8.random(in: 0...255) }
            return Markers(token: bytes.map { String(format: "%02x", $0) }.joined())
        }
    }

    /// The variables printed by `env -0` between one run's markers.
    ///
    /// Because the markers are unguessable, any occurrence of either marker
    /// in the output other than the one pair our own script printed means
    /// something is wrong, not that a profile's noise needs to be worked
    /// around: we require exactly one begin marker and exactly one end
    /// marker, with the begin before the end, and fail cleanly otherwise. A
    /// record that does not parse as `KEY=VALUE` also fails the whole parse
    /// rather than returning a partial result.
    static func parse(_ output: Data, markers: Markers) -> [String: String]? {
        let text = String(decoding: output, as: UTF8.self)
        let begins = ranges(of: markers.begin, in: text)
        let ends = ranges(of: markers.end, in: text)
        guard begins.count == 1, ends.count == 1, begins[0].upperBound <= ends[0].lowerBound else { return nil }
        var variables: [String: String] = [:]
        for record in text[begins[0].upperBound..<ends[0].lowerBound].split(separator: "\u{0}") {
            guard let equals = record.firstIndex(of: "="), equals != record.startIndex else { return nil }
            variables[String(record[..<equals])] = String(record[record.index(after: equals)...])
        }
        return variables
    }

    private static func ranges(of marker: String, in text: String) -> [Range<String.Index>] {
        var found: [Range<String.Index>] = []
        var searchStart = text.startIndex
        while let range = text.range(of: marker, range: searchStart..<text.endIndex) {
            found.append(range)
            searchStart = range.upperBound
        }
        return found
    }

    /// Runs `<shell> -l -i -c <script>`, where the script prints `env -0`
    /// between a fresh, unguessable marker pair.
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
        let markers = Markers.generate()
        let script = "printf '%s' '\(markers.begin)'; /usr/bin/env -0; printf '%s' '\(markers.end)'"
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
        while parse(snapshot(), markers: markers) == nil, Date() < drainDeadline { usleep(10_000) }
        output.fileHandleForReading.readabilityHandler = nil
        guard process.terminationStatus == 0 else { return failed("the shell exited with status \(process.terminationStatus)") }
        guard var variables = parse(snapshot(), markers: markers), !variables.isEmpty else { return failed("the shell printed no environment") }
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
