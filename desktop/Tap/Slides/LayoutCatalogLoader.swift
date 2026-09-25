import Foundation

/// Runs the bundled tap once for every layout's template. The gallery and
/// the Slide menu read `templates`; New Slide reads `template(named:)`.
/// A load that fails is tried again the next time something asks for the
/// catalog (a New Slide refused for want of it, the layout menu opening),
/// up to `maximumAttempts` runs of tap in all, so a tap that cannot answer
/// is not run on every menu open.
@MainActor
final class LayoutCatalogLoader {
    static let didLoadNotification = Notification.Name("TapLayoutCatalogDidLoad")
    private(set) var templates: [LayoutTemplate] = []
    var isLoaded: Bool { !templates.isEmpty }
    private var loading: Task<Void, Never>?
    static let maximumAttempts = 3
    private(set) var attempts = 0
    private let executable: () -> URL

    init(executable: @escaping () -> URL) {
        self.executable = executable
    }

    func template(named name: String) -> LayoutTemplate? {
        templates.first { $0.name == name }
    }

    func load() async {
        if isLoaded { return }
        if let loading { return await loading.value }
        guard attempts < Self.maximumAttempts else { return }
        attempts += 1
        let task = Task { @MainActor [weak self] in
            guard let self else { return }
            let executable = self.executable()
            let data = await Self.run(executable, arguments: ["slide", "add", "--print", "--json"])
            if let data, let templates = try? LayoutCatalog.decode(data) {
                self.templates = templates
                NotificationCenter.default.post(name: Self.didLoadNotification, object: self)
            }
        }
        loading = task
        await task.value
        loading = nil
    }

    nonisolated static func run(_ executable: URL, arguments: [String]) async -> Data? {
        await withCheckedContinuation { continuation in
            DispatchQueue.global().async {
                let process = Process()
                process.executableURL = executable
                process.arguments = arguments
                let output = Pipe()
                process.standardOutput = output
                process.standardError = FileHandle.nullDevice
                guard (try? process.run()) != nil else { return continuation.resume(returning: nil) }
                let data = output.fileHandleForReading.readDataToEndOfFile()
                process.waitUntilExit()
                continuation.resume(returning: process.terminationStatus == 0 ? data : nil)
            }
        }
    }
}
