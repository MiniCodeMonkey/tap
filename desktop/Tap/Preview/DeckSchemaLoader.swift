import Foundation

/// Runs the bundled tap once for `tap deck schema --json`: every
/// frontmatter key tap understands, which the Deck tab builds its form
/// from, so Swift hard-codes no key. Loaded at launch; a load that fails
/// is tried again when a deck asks, up to `maximumAttempts` runs. The run
/// has no stdin of its own and a deadline, so a tap that hangs cannot
/// hang a load.
@MainActor
final class DeckSchemaLoader {
    static let didLoadNotification = Notification.Name("TapDeckSchemaDidLoad")
    static let runTimeout: TimeInterval = 20
    private(set) var keys: [SchemaKey] = []
    var isLoaded: Bool { !keys.isEmpty }
    private var loading: Task<Void, Never>?
    static let maximumAttempts = 3
    private(set) var attempts = 0
    private let executable: () -> URL

    init(executable: @escaping () -> URL) {
        self.executable = executable
    }

    func load() async {
        if isLoaded { return }
        if let loading { return await loading.value }
        guard attempts < Self.maximumAttempts else { return }
        attempts += 1
        let task = Task { @MainActor [weak self] in
            guard let self else { return }
            let data = await Self.run(self.executable(), arguments: ["deck", "schema", "--json"], timeout: Self.runTimeout)
            if let data, let keys = try? DeckSchema.decode(data) {
                self.keys = keys
                NotificationCenter.default.post(name: Self.didLoadNotification, object: self)
            }
        }
        loading = task
        await task.value
        loading = nil
    }

    /// `LayoutCatalogLoader.run` with a deadline and no stdin: the process is
    /// terminated after `timeout`, and its output is then nil.
    nonisolated static func run(_ executable: URL, arguments: [String], timeout: TimeInterval) async -> Data? {
        await withCheckedContinuation { continuation in
            DispatchQueue.global().async {
                let process = Process()
                process.executableURL = executable
                process.arguments = arguments
                process.standardInput = FileHandle.nullDevice
                let output = Pipe()
                process.standardOutput = output
                process.standardError = FileHandle.nullDevice
                guard (try? process.run()) != nil else { return continuation.resume(returning: nil) }
                let killer = DispatchWorkItem { if process.isRunning { process.terminate() } }
                DispatchQueue.global().asyncAfter(deadline: .now() + timeout, execute: killer)
                let data = output.fileHandleForReading.readDataToEndOfFile()
                process.waitUntilExit()
                killer.cancel()
                continuation.resume(returning: process.terminationStatus == 0 ? data : nil)
            }
        }
    }
}
