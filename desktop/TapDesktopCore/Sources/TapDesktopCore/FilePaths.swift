import Foundation

/// tap reports absolute paths, which may go through a different spelling of
/// the same folder (`/var` and `/private/var`), so paths compare after
/// resolving symlinks.
public enum FilePaths {
    public static func canonical(_ url: URL) -> String {
        url.standardizedFileURL.resolvingSymlinksInPath().path
    }

    public static func same(_ first: URL, _ second: URL) -> Bool {
        canonical(first) == canonical(second)
    }
}
