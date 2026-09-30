import Foundation

/// The per-deck interface state the deck window remembers: whether the
/// slide panel is pinned as a sidebar or peeks on hover (pinned the first
/// time a deck opens, so people find it), and whether the Deck card in the
/// editor is open (closed the first time). Each deck remembers its own.
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

    static func cardKey(for deck: URL) -> String {
        "DeckCardOpen:" + FilePaths.canonical(deck)
    }

    public func isDeckCardOpen(deck: URL) -> Bool {
        defaults.object(forKey: Self.cardKey(for: deck)) as? Bool ?? false
    }

    public func setDeckCardOpen(_ open: Bool, deck: URL) {
        defaults.set(open, forKey: Self.cardKey(for: deck))
    }

    /// The deck was renamed, moved or saved as another file: its state follows it.
    public func moveState(from old: URL, to new: URL) {
        for (oldKey, newKey) in [(Self.key(for: old), Self.key(for: new)), (Self.cardKey(for: old), Self.cardKey(for: new))] {
            guard let value = defaults.object(forKey: oldKey) as? Bool else { continue }
            defaults.removeObject(forKey: oldKey)
            defaults.set(value, forKey: newKey)
        }
    }
}
