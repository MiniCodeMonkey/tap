import AppKit

/// The editor's font size and line height, from General settings, shared
/// by every editor. A change restyles every open editor.
struct EditorTypography: Equatable {
    static let didChangeNotification = Notification.Name("TapEditorTypographyDidChange")
    let fontSize: CGFloat
    let lineHeight: CGFloat

    var font: NSFont { .monospacedSystemFont(ofSize: fontSize, weight: .regular) }
    var boldFont: NSFont { .monospacedSystemFont(ofSize: fontSize, weight: .bold) }
    var smallFont: NSFont { .monospacedSystemFont(ofSize: max(8, (fontSize * 0.7).rounded()), weight: .regular) }

    static func from(_ settings: GeneralSettings) -> EditorTypography {
        EditorTypography(fontSize: settings.fontSize, lineHeight: settings.lineSpacing.lineHeight(forFontSize: settings.fontSize))
    }

    @MainActor private(set) static var current = EditorTypography(fontSize: 13, lineHeight: 21)

    /// Reads the settings and tells every editor when they changed.
    @MainActor static func refresh(from settings: GeneralSettings) {
        let next = from(settings)
        guard next != current else { return }
        current = next
        NotificationCenter.default.post(name: didChangeNotification, object: nil)
    }
}
