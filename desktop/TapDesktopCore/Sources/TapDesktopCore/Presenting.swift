import Foundation

/// A display, as the talk's windows see it: its name, its frame in screen
/// coordinates and whether it is the Mac's own screen. The app builds one
/// per `NSScreen`; tests build them by hand.
public struct ScreenInfo: Equatable, Sendable {
    public let name: String
    public let frame: CGRect
    public let isBuiltIn: Bool

    public init(name: String, frame: CGRect, isBuiltIn: Bool) {
        self.name = name
        self.frame = frame
        self.isBuiltIn = isBuiltIn
    }
}

/// Which display was the audience the last time this set of displays was
/// connected, across decks. Keyed by the displays' names, sorted, so the
/// order the system lists them in does not matter.
// UserDefaults is thread-safe, so a struct that only holds a reference to it is safe to share.
public struct DisplayAssignmentStore: @unchecked Sendable {
    private let defaults: UserDefaults

    public init(defaults: UserDefaults = .standard) {
        self.defaults = defaults
    }

    public static func key(for screens: [ScreenInfo]) -> String {
        "DisplayAssignment:" + screens.map(\.name).sorted().joined(separator: "|")
    }

    public func audienceName(for screens: [ScreenInfo]) -> String? {
        defaults.string(forKey: Self.key(for: screens))
    }

    public func setAudienceName(_ name: String, for screens: [ScreenInfo]) {
        defaults.set(name, forKey: Self.key(for: screens))
    }
}

/// Which screen shows the audience page and which the presenter page. With
/// one display both are the same screen.
public struct DisplayArrangement: Equatable, Sendable {
    public let audience: ScreenInfo
    public let presenter: ScreenInfo

    public init(audience: ScreenInfo, presenter: ScreenInfo) {
        self.audience = audience
        self.presenter = presenter
    }

    public var isSingleDisplay: Bool { audience == presenter }

    /// The arrangement for `screens`: the first external display is the
    /// audience and the built-in one the presenter, unless the store
    /// remembers the audience on another connected display. With no
    /// built-in display, the first listed is the audience. nil with no
    /// screens at all.
    public static func resolve(screens: [ScreenInfo], store: DisplayAssignmentStore) -> DisplayArrangement? {
        guard let first = screens.first else { return nil }
        guard screens.count > 1 else { return DisplayArrangement(audience: first, presenter: first) }
        let presenter = screens.first(where: \.isBuiltIn) ?? screens[1]
        let audience = screens.first { $0 != presenter } ?? first
        var arrangement = DisplayArrangement(audience: audience, presenter: presenter)
        if let remembered = store.audienceName(for: screens), remembered == presenter.name {
            arrangement = arrangement.swapped()
        }
        return arrangement
    }

    public func swapped() -> DisplayArrangement {
        DisplayArrangement(audience: presenter, presenter: audience)
    }
}

public enum PresentationMode: Equatable, Sendable {
    /// Play: the audience page on the projector, the presenter page on the laptop, tap's recording rule.
    case play
    /// Rehearse: the presenter page alone, never recorded.
    case rehearse
}

/// What the Present popover collects, and Rehearse assumes.
public struct PresentationOptions: Equatable, Sendable {
    public var mode: PresentationMode
    /// The slide the talk opens on, 1-based.
    public var startSlide: Int
    /// False passes --no-record for this run. True leaves recording to tap's
    /// own consent, stored in settings.yaml.
    public var record: Bool
    /// Phone remote: the tunnel, with the QR code panel.
    public var phoneRemote: Bool
    /// Advanced: the tunnel on its own.
    public var tunnel: Bool
    /// Advanced: the person's own presenter password, passed to tap.
    public var presenterPassword: String?

    public init(mode: PresentationMode, startSlide: Int, record: Bool = true, phoneRemote: Bool = false,
                tunnel: Bool = false, presenterPassword: String? = nil) {
        self.mode = mode
        self.startSlide = startSlide
        self.record = record
        self.phoneRemote = phoneRemote
        self.tunnel = tunnel
        self.presenterPassword = presenterPassword
    }

    /// The tap present command for these options, on `port` (the deck's
    /// remembered port, or nil for a free one).
    public func command(port: Int?) -> TapSession.Command {
        .present(record: mode == .play && record, presenterPassword: presenterPassword, port: port)
    }

    public var wantsTunnel: Bool { phoneRemote || tunnel }
}

/// The Present popover's settings, the ones Cmd+Option+P starts with: kept
/// across launches. The presenter password is not among them; it lives in
/// the popover's field for one launch of the app.
public struct PresentationSettings: Equatable, Sendable {
    public var startFromSlideOne = false
    public var record = true
    public var phoneRemote = false
    public var tunnel = false

    public init(startFromSlideOne: Bool = false, record: Bool = true, phoneRemote: Bool = false, tunnel: Bool = false) {
        self.startFromSlideOne = startFromSlideOne
        self.record = record
        self.phoneRemote = phoneRemote
        self.tunnel = tunnel
    }

    /// The options for a start with these settings, from `cursorSlide`
    /// unless the settings say slide 1.
    public func options(mode: PresentationMode, cursorSlide: Int, presenterPassword: String?) -> PresentationOptions {
        PresentationOptions(mode: mode, startSlide: startFromSlideOne ? 1 : cursorSlide, record: record,
                            phoneRemote: phoneRemote, tunnel: tunnel, presenterPassword: presenterPassword)
    }
}

// UserDefaults is thread-safe, so a struct that only holds a reference to it is safe to share.
public struct PresentationSettingsStore: @unchecked Sendable {
    public let defaults: UserDefaults
    static let key = "PresentationSettings"

    public init(defaults: UserDefaults = .standard) {
        self.defaults = defaults
    }

    public var settings: PresentationSettings {
        get {
            guard let stored = defaults.dictionary(forKey: Self.key) else { return PresentationSettings() }
            return PresentationSettings(startFromSlideOne: stored["startFromSlideOne"] as? Bool ?? false,
                                        record: stored["record"] as? Bool ?? true,
                                        phoneRemote: stored["phoneRemote"] as? Bool ?? false,
                                        tunnel: stored["tunnel"] as? Bool ?? false)
        }
        nonmutating set {
            defaults.set(["startFromSlideOne": newValue.startFromSlideOne, "record": newValue.record,
                          "phoneRemote": newValue.phoneRemote, "tunnel": newValue.tunnel], forKey: Self.key)
        }
    }
}

/// The port each deck's talks run on. WebKit keys a page's localStorage,
/// where the presenter page keeps its layout and notes size, by origin,
/// and the origin includes the port; one port per deck keeps that state
/// across launches. Keyed by the deck's standardized path.
// UserDefaults is thread-safe, so a struct that only holds a reference to it is safe to share.
public struct DeckPortStore: @unchecked Sendable {
    private let defaults: UserDefaults

    public init(defaults: UserDefaults = .standard) {
        self.defaults = defaults
    }

    public static func key(for deck: URL) -> String {
        "PresentPort:" + deck.standardizedFileURL.path
    }

    public func port(for deck: URL) -> Int? {
        let port = defaults.integer(forKey: Self.key(for: deck))
        return port > 0 ? port : nil
    }

    public func setPort(_ port: Int, for deck: URL) {
        defaults.set(port, forKey: Self.key(for: deck))
    }

    /// The port a deck's first talk asks for: one of 20000 to 29999,
    /// derived from the deck's path with FNV-1a (Swift's own hasher is
    /// seeded per process), so it is the same in every launch and outside
    /// the ephemeral range that outgoing connections and `tap dev --app`
    /// draw from, which a port tap picked itself would sit in.
    public static func suggestedPort(for deck: URL) -> Int {
        var hash: UInt64 = 0xcbf29ce484222325
        for byte in deck.standardizedFileURL.path.utf8 {
            hash ^= UInt64(byte)
            hash = hash &* 0x100000001b3
        }
        return 20000 + Int(hash % 10000)
    }
}

/// The recording state the presenter toolbar shows, kept from tap's
/// recording events and ticked once a second in between, since tap sends
/// an event only when the state, segment or disk level changes.
public struct RecordingStatus: Equatable, Sendable {
    public var state = "stopped"
    public var segment = 0
    public var elapsed = 0
    public var disk = "ok"
    /// Why tap cannot record at all this run (a `recording_blocked` error), if it said so.
    public var blockedReason: String?

    public init() {}

    public var isRecording: Bool { state == "recording" }

    public var label: String {
        switch state {
        case "recording": return "REC " + Self.clock(elapsed)
        case "paused": return "REC PAUSED"
        default: return "NOT RECORDING"
        }
    }

    public mutating func apply(_ event: RecordingEvent) {
        state = event.state
        segment = event.segment
        elapsed = event.elapsed
        disk = event.disk
    }

    /// One second passed.
    public mutating func tick() {
        if isRecording { elapsed += 1 }
    }

    /// `seconds` as m:ss, or h:mm:ss from an hour.
    public static func clock(_ seconds: Int) -> String {
        let hours = seconds / 3600
        let minutes = (seconds % 3600) / 60
        let rest = seconds % 60
        if hours > 0 { return String(format: "%d:%02d:%02d", hours, minutes, rest) }
        return String(format: "%d:%02d", minutes, rest)
    }
}

/// Whether the Focus hint has been shown: it appears before the first talk
/// on this Mac and never again.
public struct FocusHintState: @unchecked Sendable {
    private let defaults: UserDefaults
    static let key = "FocusHintShown"

    public init(defaults: UserDefaults = .standard) {
        self.defaults = defaults
    }

    public var hasBeenShown: Bool { defaults.bool(forKey: Self.key) }

    public func markShown() {
        defaults.set(true, forKey: Self.key)
    }
}
