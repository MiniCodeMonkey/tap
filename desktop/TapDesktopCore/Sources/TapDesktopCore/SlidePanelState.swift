import Foundation

/// Whether the slide panel is pinned as a sidebar or peeks on hover. The
/// panel is pinned the first time a deck opens, so people find it, and
/// each deck then remembers its own state.
// UserDefaults is thread-safe, so a struct that only holds a reference to it is safe to share.
public struct SlidePanelState: @unchecked Sendable {
    private let defaults: UserDefaults

    public init(defaults: UserDefaults = .standard) {
        self.defaults = defaults
    }

    static func key(for deck: URL) -> String {
        "SlidePanelPinned:" + FilePaths.canonical(deck)
    }

    public func isPinned(deck: URL) -> Bool {
        defaults.object(forKey: Self.key(for: deck)) as? Bool ?? true
    }

    public func setPinned(_ pinned: Bool, deck: URL) {
        defaults.set(pinned, forKey: Self.key(for: deck))
    }

    /// The deck was renamed, moved or saved as another file: its state follows it.
    public func moveState(from old: URL, to new: URL) {
        guard let value = defaults.object(forKey: Self.key(for: old)) as? Bool else { return }
        defaults.removeObject(forKey: Self.key(for: old))
        defaults.set(value, forKey: Self.key(for: new))
    }
}
