import Foundation

/// The rules behind Settings > Command Line: where a `tap` is on the
/// login shell's PATH, whether a file is the app's own link, and whether
/// Install may touch what is at ~/.local/bin/tap. Pure functions; the
/// installer and the pane give them the file system.
public enum CommandLineTool {
    /// Every `<directory>/<name>` on `path` that exists, in PATH order.
    public static func locate(named name: String, onPath path: String, fileExists: (String) -> Bool) -> [String] {
        path.split(separator: ":", omittingEmptySubsequences: true).map { "\($0)/\(name)" }.filter(fileExists)
    }

    /// A symlink is ours to replace when its destination is this app's own
    /// tap (`ownTap`), or the bundled tap of an app named Tap.app or a
    /// Finder copy of it ("Tap 2.app"); anything else was installed by
    /// someone else.
    public static func isBundledLink(destination: String?, ownTap: String? = nil) -> Bool {
        guard let destination else { return false }
        if let ownTap, destination == ownTap || (destination as NSString).resolvingSymlinksInPath == (ownTap as NSString).resolvingSymlinksInPath {
            return true
        }
        let components = (destination as NSString).pathComponents
        guard components.count >= 5, Array(components.suffix(3)) == ["Contents", "Resources", "tap"] else { return false }
        return isTapBundleName(components[components.count - 4])
    }

    /// "Tap.app", or "Tap <number>.app" as Finder names a copy.
    static func isTapBundleName(_ name: String) -> Bool {
        if name == "Tap.app" { return true }
        guard name.hasPrefix("Tap "), name.hasSuffix(".app") else { return false }
        let number = name.dropFirst(4).dropLast(4)
        return !number.isEmpty && number.allSatisfy { ("0"..."9").contains($0) }
    }

    public enum ExistingFile: Equatable, Sendable {
        case none
        case bundledLink
        case other
    }

    public enum InstallDecision: Equatable, Sendable {
        case link
        case replaceOwnLink
        case refuse(String)
    }

    public static func installDecision(for existing: ExistingFile) -> InstallDecision {
        switch existing {
        case .none: return .link
        case .bundledLink: return .replaceOwnLink
        case .other: return .refuse("a tap that Tap did not install is already there")
        }
    }

    /// Whether `directory` comes before the folder of `otherExecutable` on
    /// `path`: true with no other tap, nil when `directory` is not on PATH.
    public static func directoryComesFirst(_ directory: String, beforeDirectoryOf otherExecutable: String?, onPath path: String) -> Bool? {
        let entries = path.split(separator: ":", omittingEmptySubsequences: true).map(String.init)
        guard let own = entries.firstIndex(of: directory) else { return nil }
        guard let otherExecutable else { return true }
        let otherDirectory = (otherExecutable as NSString).deletingLastPathComponent
        guard let other = entries.firstIndex(of: otherDirectory) else { return true }
        return own < other
    }
}
