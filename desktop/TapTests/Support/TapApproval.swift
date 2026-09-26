import Foundation
@testable import Tap

/// Runs the bundled tap's approval commands against the test's own
/// settings folder, the way a person would in a terminal.
enum TapApproval {
    /// Runs tap with no stdin of its own (a prompt would otherwise wait on
    /// the host's) and kills it after `timeout`, so a tap that hangs fails
    /// the test rather than the bundle.
    static func run(_ arguments: [String], configHome: URL, timeout: TimeInterval = 20) async throws -> String {
        let executable = await MainActor.run { AppEnvironment.shared.tapExecutableURL }
        return try await withCheckedThrowingContinuation { continuation in
            DispatchQueue.global().async {
                let process = Process()
                process.executableURL = executable
                process.arguments = arguments
                process.environment = ["XDG_CONFIG_HOME": configHome.path, "HOME": NSHomeDirectory(), "PATH": "/usr/bin:/bin"]
                process.standardInput = FileHandle.nullDevice
                let output = Pipe()
                process.standardOutput = output
                process.standardError = FileHandle.nullDevice
                do { try process.run() } catch { return continuation.resume(throwing: error) }
                let killer = DispatchWorkItem { if process.isRunning { process.terminate() } }
                DispatchQueue.global().asyncAfter(deadline: .now() + timeout, execute: killer)
                let data = output.fileHandleForReading.readDataToEndOfFile()
                process.waitUntilExit()
                killer.cancel()
                continuation.resume(returning: String(decoding: data, as: UTF8.self))
            }
        }
    }
}
