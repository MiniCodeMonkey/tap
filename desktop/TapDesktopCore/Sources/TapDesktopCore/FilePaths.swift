import Foundation

/// tap reports absolute paths, which may go through a different spelling of
/// the same folder (`/var` and `/private/var`), so paths compare after
/// resolving symlinks. `resolvingSymlinksInPath` only resolves a path that
/// exists on disk, and an editor's atomic save can leave the deck's path
/// briefly absent, so symlinks are resolved on the deepest ancestor that
/// does exist and the remaining components are appended unresolved.
public enum FilePaths {
    public static func canonical(_ url: URL) -> String {
        let fileManager = FileManager.default
        var existing = url.standardizedFileURL
        var missingComponents: [String] = []
        while existing.path != "/", !fileManager.fileExists(atPath: existing.path) {
            missingComponents.append(existing.lastPathComponent)
            existing = existing.deletingLastPathComponent()
        }
        var resolved = existing.resolvingSymlinksInPath()
        for component in missingComponents.reversed() {
            resolved = resolved.appendingPathComponent(component)
        }
        return resolved.path
    }

    public static func same(_ first: URL, _ second: URL) -> Bool {
        canonical(first) == canonical(second)
    }
}
