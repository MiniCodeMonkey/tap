import Foundation

/// The General pane's settings, in UserDefaults: the editor's font size
/// and line spacing, the theme New Deck preselects (passed to tap new
/// --theme; nil leaves tap's own default), the autosave delay, and the
/// folder New Deck last saved into. A test uses a suite of its own.
public final class GeneralSettings {
    public static let didChangeNotification = Notification.Name("TapGeneralSettingsDidChange")
    public static let fontSizes: [CGFloat] = [11, 12, 13, 14, 15, 16, 18]
    public static let autosaveDelays: [TimeInterval] = [0.5, 1, 2, 5, 10]

    public enum LineSpacing: String, CaseIterable, Sendable {
        case tight, normal, roomy

        /// The editor's line height: D2's 21 points at 13 points normal.
        public func lineHeight(forFontSize fontSize: CGFloat) -> CGFloat {
            let factor: CGFloat
            switch self {
            case .tight: factor = 1.38
            case .normal: factor = 1.62
            case .roomy: factor = 1.92
            }
            return (fontSize * factor).rounded()
        }
    }

    public let defaults: UserDefaults

    public init(defaults: UserDefaults = .standard) {
        self.defaults = defaults
    }

    public var fontSize: CGFloat {
        get {
            let stored = CGFloat(defaults.double(forKey: "TapEditorFontSize"))
            return Self.fontSizes.contains(stored) ? stored : 13
        }
        set { set(Double(newValue), forKey: "TapEditorFontSize") }
    }

    public var lineSpacing: LineSpacing {
        get { defaults.string(forKey: "TapEditorLineSpacing").flatMap(LineSpacing.init(rawValue:)) ?? .normal }
        set { set(newValue.rawValue, forKey: "TapEditorLineSpacing") }
    }

    public var defaultTheme: String? {
        get { defaults.string(forKey: "TapDefaultTheme") }
        set { set(newValue, forKey: "TapDefaultTheme") }
    }

    public var autosaveDelay: TimeInterval {
        get {
            let stored = defaults.double(forKey: "TapAutosaveDelay")
            return Self.autosaveDelays.contains(stored) ? stored : 1
        }
        set { set(newValue, forKey: "TapAutosaveDelay") }
    }

    public var lastNewDeckFolder: URL? {
        get { defaults.string(forKey: "TapLastNewDeckFolder").map { URL(fileURLWithPath: $0) } }
        set { set(newValue?.path, forKey: "TapLastNewDeckFolder") }
    }

    private func set(_ value: Any?, forKey key: String) {
        if let value { defaults.set(value, forKey: key) } else { defaults.removeObject(forKey: key) }
        NotificationCenter.default.post(name: Self.didChangeNotification, object: self)
    }
}
