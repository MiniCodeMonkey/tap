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

/// What a display does in a talk.
public enum DisplayRole: Equatable, Sendable {
    /// The audience page, full screen. Exactly one display has it.
    case audience
    /// The presenter page. At most one display has it.
    case presenter
    /// Left alone.
    case notUsed
}

/// The roles remembered for one set of displays: the audience display's
/// name, and the presenter display's (nil when none was chosen).
public struct RememberedDisplayRoles: Equatable, Sendable {
    public var audience: String
    public var presenter: String?

    public init(audience: String, presenter: String?) {
        self.audience = audience
        self.presenter = presenter
    }
}

/// Which display had which role the last time this set of displays was
/// connected, across decks. Keyed by the displays' names, sorted, so the
/// order the system lists them in does not matter.
// UserDefaults is thread-safe, so a struct that only holds a reference to it is safe to share.
public struct DisplayAssignmentStore: @unchecked Sendable {
    private let defaults: UserDefaults

    public init(defaults: UserDefaults = .standard) {
        self.defaults = defaults
    }

    /// The set of displays as one string: their names, sorted.
    public static func setName(for screens: [ScreenInfo]) -> String {
        screens.map(\.name).sorted().joined(separator: "|")
    }

    public static func key(for screens: [ScreenInfo]) -> String {
        "DisplayRoles:" + setName(for: screens)
    }

    /// Where the audience display alone was remembered, before presenters were.
    private static func audienceOnlyKey(for screens: [ScreenInfo]) -> String {
        "DisplayAssignment:" + setName(for: screens)
    }

    /// The roles remembered for `screens`. A set remembered only by its
    /// audience display has the default presenter, unless that is the
    /// audience display itself.
    public func roles(for screens: [ScreenInfo]) -> RememberedDisplayRoles? {
        if let stored = defaults.dictionary(forKey: Self.key(for: screens)), let audience = stored["audience"] as? String {
            let presenter = stored["presenter"] as? String
            return RememberedDisplayRoles(audience: audience, presenter: presenter?.isEmpty == false ? presenter : nil)
        }
        guard let audience = defaults.string(forKey: Self.audienceOnlyKey(for: screens)) else { return nil }
        let fallback = DisplayArrangement.defaults(for: screens)
        let presenter = fallback?.presenter.name == audience ? fallback?.audience.name : fallback?.presenter.name
        return RememberedDisplayRoles(audience: audience, presenter: presenter)
    }

    public func setRoles(_ roles: RememberedDisplayRoles, for screens: [ScreenInfo]) {
        defaults.set(["audience": roles.audience, "presenter": roles.presenter ?? ""], forKey: Self.key(for: screens))
    }

    public func remember(_ arrangement: DisplayArrangement) {
        setRoles(RememberedDisplayRoles(audience: arrangement.audience.name,
                                        presenter: arrangement.presenterScreen?.name),
                 for: arrangement.screens)
    }
}

/// Which screen shows the audience page and which the presenter page, and
/// which connected screens are left alone. With one display, or with no
/// presenter display, the presenter page shares the audience's screen.
public struct DisplayArrangement: Equatable, Sendable {
    /// Every connected display, in the order the system lists them.
    public let screens: [ScreenInfo]
    public let audience: ScreenInfo
    /// The display with the presenter role; nil when none has it.
    public let presenterScreen: ScreenInfo?

    public init(screens: [ScreenInfo], audience: ScreenInfo, presenterScreen: ScreenInfo?) {
        self.screens = screens
        self.audience = audience
        self.presenterScreen = presenterScreen == audience ? nil : presenterScreen
    }

    /// Two displays with a role each.
    public init(audience: ScreenInfo, presenter: ScreenInfo) {
        self.init(screens: audience == presenter ? [audience] : [audience, presenter], audience: audience, presenterScreen: presenter)
    }

    /// Where the presenter page goes: its own display, or the audience's.
    public var presenter: ScreenInfo { presenterScreen ?? audience }

    /// True when the presenter page has no display of its own: one
    /// display, or a presenter role nobody holds.
    public var sharesDisplay: Bool { presenterScreen == nil }

    public func role(of screen: ScreenInfo) -> DisplayRole {
        if screen == audience { return .audience }
        if screen == presenterScreen { return .presenter }
        return .notUsed
    }

    /// The roles `screens` start with: the first external display is the
    /// audience and the built-in one the presenter; with no built-in
    /// display, the first listed is the audience and the second the
    /// presenter. Any other display is not used. nil with no screens.
    static func defaults(for screens: [ScreenInfo]) -> DisplayArrangement? {
        guard let first = screens.first else { return nil }
        guard screens.count > 1 else { return DisplayArrangement(screens: screens, audience: first, presenterScreen: nil) }
        let presenter = screens.first(where: \.isBuiltIn) ?? screens[1]
        let audience = screens.first { $0 != presenter } ?? first
        return DisplayArrangement(screens: screens, audience: audience, presenterScreen: presenter)
    }

    /// The arrangement for `screens`: the store's roles when it remembers
    /// them for this set of displays and they are still connected, else
    /// the defaults. nil with no screens.
    public static func resolve(screens: [ScreenInfo], store: DisplayAssignmentStore) -> DisplayArrangement? {
        guard let defaults = defaults(for: screens) else { return nil }
        guard screens.count > 1, let remembered = store.roles(for: screens),
              let audience = screens.first(where: { $0.name == remembered.audience }) else { return defaults }
        let presenter = remembered.presenter.flatMap { name in screens.first { $0.name == name } }
        return DisplayArrangement(screens: screens, audience: audience, presenterScreen: presenter)
    }

    /// The arrangement with `screen` given `role`. A role another display
    /// holds is exchanged: that display takes the role `screen` had.
    /// There is always an audience display, and the audience role never
    /// goes to nobody: taking it off the audience display hands it to the
    /// presenter display, or else the first other display. One display has
    /// nothing to assign.
    public func assigning(_ role: DisplayRole, to screen: ScreenInfo) -> DisplayArrangement {
        guard screens.count > 1, screens.contains(screen), self.role(of: screen) != role else { return self }
        let previous = self.role(of: screen)
        var audience = self.audience
        var presenter = presenterScreen
        switch (role, previous) {
        case (.audience, .presenter):
            presenter = audience
            audience = screen
        case (.audience, _):
            audience = screen
        case (.presenter, .audience):
            if let holder = presenter {
                audience = holder
            } else if let other = screens.first(where: { $0 != screen }) {
                audience = other
            }
            presenter = screen
        case (.presenter, _):
            presenter = screen
        case (.notUsed, .audience):
            if let holder = presenter {
                audience = holder
                presenter = nil
            } else if let other = screens.first(where: { $0 != screen }) {
                audience = other
            }
        case (.notUsed, _):
            presenter = nil
        }
        return DisplayArrangement(screens: screens, audience: audience, presenterScreen: presenter)
    }

    /// The audience and presenter displays exchanged; the same with no presenter display.
    public func swapped() -> DisplayArrangement {
        guard let presenterScreen else { return self }
        return DisplayArrangement(screens: screens, audience: presenterScreen, presenterScreen: audience)
    }
}

/// The set of displays each deck was last presented on, so Play can ask
/// before the first talk of a deck and after the displays changed.
// UserDefaults is thread-safe, so a struct that only holds a reference to it is safe to share.
public struct PresentedDisplaysStore: @unchecked Sendable {
    private let defaults: UserDefaults

    public init(defaults: UserDefaults = .standard) {
        self.defaults = defaults
    }

    static func key(for deck: URL) -> String {
        "PresentedDisplays:" + deck.standardizedFileURL.path
    }

    /// Records that `deck` was played on `screens`.
    public func recordPlay(of deck: URL, on screens: [ScreenInfo]) {
        defaults.set(DisplayAssignmentStore.setName(for: screens), forKey: Self.key(for: deck))
    }

    /// True when `deck` was last played on exactly the displays `screens` names.
    public func hasPlayed(_ deck: URL, on screens: [ScreenInfo]) -> Bool {
        defaults.string(forKey: Self.key(for: deck)) == DisplayAssignmentStore.setName(for: screens)
    }
}

/// Whether tap can start its public tunnel: tap looks for a `cloudflared`
/// executable on the PATH it runs with (`tunnel.Available`), so this looks
/// on the same PATH.
public enum CloudflaredLocator {
    public static let installCommand = "brew install cloudflared"

    public static func isInstalled(searchPath: String?, fileManager: FileManager = .default) -> Bool {
        guard let searchPath else { return false }
        return searchPath.split(separator: ":").contains { directory in
            var isDirectory: ObjCBool = false
            let candidate = String(directory) + "/cloudflared"
            return fileManager.fileExists(atPath: candidate, isDirectory: &isDirectory) && !isDirectory.boolValue
                && fileManager.isExecutableFile(atPath: candidate)
        }
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
    /// Whether this run records: the Record switch decides, passed to tap as
    /// --record or --no-record.
    public var record: Bool
    /// Phone remote: tap's public tunnel, with the QR code panel. tap has
    /// no remote without the tunnel.
    public var phoneRemote: Bool
    /// The person's own presenter password for the remote, passed to tap.
    public var presenterPassword: String?

    public init(mode: PresentationMode, startSlide: Int, record: Bool = false, phoneRemote: Bool = false,
                presenterPassword: String? = nil) {
        self.mode = mode
        self.startSlide = startSlide
        self.record = record
        self.phoneRemote = phoneRemote
        self.presenterPassword = presenterPassword
    }

    /// The tap present command for these options, on `port` (the deck's
    /// remembered port, or nil for a free one).
    public func command(port: Int?) -> TapSession.Command {
        .present(record: mode == .play && record, presenterPassword: presenterPassword, port: port)
    }

    public var wantsTunnel: Bool { phoneRemote }
}

/// The Present Settings the Play button and Cmd+Option+P start with: kept
/// across launches. The presenter password is not among them; it lives in
/// the popover's field for one launch of the app. Where the talk starts is
/// not among them either: a click on Play starts at the cursor's slide.
public struct PresentationSettings: Equatable, Sendable {
    public var record = true
    public var phoneRemote = false

    public init(record: Bool = true, phoneRemote: Bool = false) {
        self.record = record
        self.phoneRemote = phoneRemote
    }

    /// The options for a start with these settings, opening on `startSlide`.
    public func options(mode: PresentationMode, startSlide: Int, presenterPassword: String?) -> PresentationOptions {
        PresentationOptions(mode: mode, startSlide: startSlide, record: record, phoneRemote: phoneRemote,
                            presenterPassword: presenterPassword)
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
            // A tunnel saved on its own is a phone remote: the tunnel is only ever used for the remote.
            return PresentationSettings(record: stored["record"] as? Bool ?? true,
                                        phoneRemote: (stored["phoneRemote"] as? Bool ?? false) || (stored["tunnel"] as? Bool ?? false))
        }
        nonmutating set {
            defaults.set(["record": newValue.record, "phoneRemote": newValue.phoneRemote], forKey: Self.key)
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

/// The URL a talk page loads again after its web content process ended.
/// It is the page's own URL, the one tap redirected to, which the cookies
/// from the first load still open (the audience page's launch code works
/// only once), with the fragment set to the talk's current slide. The page
/// keeps its hash in step as it moves, but a talk that moved while the
/// page was dead did not move the page, so its own hash is stale; the
/// fragment is what the page opens on (`initializeFromURL` in
/// frontend/src/lib/stores/presentation.ts). With no slide known, the page
/// URL is loaded as it is.
public enum TalkPageReload {
    public static func url(reloading pageURL: URL, atSlide slide: Int?) -> URL {
        guard let slide, var components = URLComponents(url: pageURL, resolvingAgainstBaseURL: false) else { return pageURL }
        components.fragment = String(slide)
        return components.url ?? pageURL
    }
}
