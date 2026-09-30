import AppKit

/// The editor's own neutral colors. They follow system light and dark mode;
/// the deck theme shows only in the preview.
enum EditorPalette {
    static func dynamic(light: NSColor, dark: NSColor) -> NSColor {
        NSColor(name: nil) { appearance in
            appearance.bestMatch(from: [.aqua, .darkAqua]) == .darkAqua ? dark : light
        }
    }

    static let boxFill = dynamic(light: .white, dark: NSColor(white: 0.16, alpha: 1))
    static let boxBorder = dynamic(light: NSColor(red: 0.89, green: 0.89, blue: 0.906, alpha: 1), dark: NSColor(white: 0.28, alpha: 1))
    static let editorBackground = dynamic(light: NSColor(red: 0.975, green: 0.975, blue: 0.98, alpha: 1), dark: NSColor(white: 0.11, alpha: 1))
    static let directive = dynamic(light: NSColor(red: 0.61, green: 0.14, blue: 0.58, alpha: 1), dark: NSColor(red: 0.99, green: 0.37, blue: 0.64, alpha: 1))
    static let fence = dynamic(light: NSColor(red: 0.77, green: 0.10, blue: 0.09, alpha: 1), dark: NSColor(red: 0.99, green: 0.42, blue: 0.36, alpha: 1))
    static let notes = dynamic(light: NSColor(red: 0.36, green: 0.42, blue: 0.47, alpha: 1), dark: NSColor(white: 0.6, alpha: 1))
    static let error = NSColor.systemRed
    static let warning = dynamic(light: NSColor(red: 0.60, green: 0.36, blue: 0, alpha: 1), dark: NSColor(red: 0.96, green: 0.72, blue: 0.36, alpha: 1))
    /// The whole surface of something with a problem, tinted, never a stripe on one edge.
    static let errorTint = dynamic(light: NSColor(red: 0.992, green: 0.925, blue: 0.922, alpha: 1), dark: NSColor(red: 0.227, green: 0.102, blue: 0.09, alpha: 1))
    static let errorTintBorder = dynamic(light: NSColor(red: 0.957, green: 0.78, blue: 0.761, alpha: 1), dark: NSColor(red: 0.369, green: 0.165, blue: 0.145, alpha: 1))
    static let warningTint = dynamic(light: NSColor(red: 1, green: 0.961, blue: 0.878, alpha: 1), dark: NSColor(red: 0.208, green: 0.157, blue: 0.059, alpha: 1))
    static let warningTintBorder = dynamic(light: NSColor(red: 0.953, green: 0.851, blue: 0.651, alpha: 1), dark: NSColor(red: 0.353, green: 0.263, blue: 0.094, alpha: 1))
}
