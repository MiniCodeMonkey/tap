import Foundation

/// Watches the deck file for deletion and renames that bypass file
/// coordination, such as `rm` in a terminal.
@MainActor
final class DeckFileWatcher {
    var onChange: (() -> Void)?
    private var source: DispatchSourceFileSystemObject?

    func watch(_ url: URL?) {
        source?.cancel()
        source = nil
        guard let url else { return }
        let descriptor = open(url.path, O_EVTONLY)
        guard descriptor >= 0 else { return }
        let newSource = DispatchSource.makeFileSystemObjectSource(fileDescriptor: descriptor, eventMask: [.delete, .rename], queue: .main)
        newSource.setEventHandler { [weak self] in
            MainActor.assumeIsolated { self?.onChange?() }
        }
        newSource.setCancelHandler { close(descriptor) }
        newSource.resume()
        source = newSource
    }
}
