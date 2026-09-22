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
}
