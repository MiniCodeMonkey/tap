import Foundation

/// Links the bundled tap into a folder on PATH (~/.local/bin). It makes
/// the folder if needed, replaces only a link of its own, and refuses a
/// file it did not put there. A test points it at a folder of its own.
final class CommandLineInstaller {
    enum InstallError: Error, Equatable {
        case refused(String)
    }

    let linkDirectory: URL
    let bundledTap: URL

    init(linkDirectory: URL = FileManager.default.homeDirectoryForCurrentUser.appendingPathComponent(".local/bin"), bundledTap: URL) {
        self.linkDirectory = linkDirectory
        self.bundledTap = bundledTap
    }

    var linkURL: URL { linkDirectory.appendingPathComponent("tap") }

    /// What is at ~/.local/bin/tap now.
    func existingFile() -> CommandLineTool.ExistingFile {
        let attributes = try? FileManager.default.attributesOfItem(atPath: linkURL.path)
        guard attributes != nil else { return .none }
        let destination = try? FileManager.default.destinationOfSymbolicLink(atPath: linkURL.path)
        return CommandLineTool.isBundledLink(destination: destination, ownTap: bundledTap.path) ? .bundledLink : .other
    }

    /// Whether the link is in place and points at this app's tap.
    var isInstalled: Bool {
        (try? FileManager.default.destinationOfSymbolicLink(atPath: linkURL.path)) == bundledTap.path
    }

    @discardableResult
    func install() throws -> URL {
        switch CommandLineTool.installDecision(for: existingFile()) {
        case .refuse(let reason):
            throw InstallError.refused(reason)
        case .replaceOwnLink:
            try FileManager.default.removeItem(at: linkURL)
        case .link:
            break
        }
        try FileManager.default.createDirectory(at: linkDirectory, withIntermediateDirectories: true)
        try FileManager.default.createSymbolicLink(at: linkURL, withDestinationURL: bundledTap)
        return linkURL
    }
}
