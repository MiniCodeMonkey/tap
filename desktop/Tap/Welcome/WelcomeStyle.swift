import AppKit
import CoreText

extension NSColor {
    /// An sRGB color from a 0xRRGGBB value.
    convenience init(hex: UInt32, alpha: CGFloat = 1) {
        self.init(srgbRed: CGFloat((hex >> 16) & 255) / 255, green: CGFloat((hex >> 8) & 255) / 255, blue: CGFloat(hex & 255) / 255, alpha: alpha)
    }

    /// A color that resolves to `light` or `dark` with the view's effective appearance.
    static func welcomeDynamic(light: NSColor, dark: NSColor) -> NSColor {
        NSColor(name: nil) { appearance in
            appearance.bestMatch(from: [.darkAqua, .aqua]) == .darkAqua ? dark : light
        }
    }
}

extension NSAppearance {
    var isDarkAppearance: Bool { bestMatch(from: [.darkAqua, .aqua]) == .darkAqua }
}

/// The welcome window's color tokens, from the approved mockup. The text
/// tokens were measured against the running aurora: `text2` is secondary
/// text that may sit on the lights, `text3` only ever sits on a flat surface.
enum WelcomeColor {
    static let window = NSColor.welcomeDynamic(light: NSColor(hex: 0xffffff), dark: NSColor(hex: 0x0b0d12))
    static let ink = NSColor.welcomeDynamic(light: NSColor(hex: 0x0a0b0d), dark: NSColor(hex: 0xf2f4f7))
    static let text2 = NSColor.welcomeDynamic(light: NSColor(hex: 0x1d2939), dark: NSColor(hex: 0xe6eaf0))
    static let text3 = NSColor.welcomeDynamic(light: NSColor(hex: 0x5b6577), dark: NSColor(hex: 0x8b93a1))
    static let line = NSColor.welcomeDynamic(light: NSColor(hex: 0xe6e8ec), dark: NSColor(white: 1, alpha: 0.08))
    static let hairline = NSColor.welcomeDynamic(light: NSColor(hex: 0xd0d5dd), dark: NSColor(white: 1, alpha: 0.18))
    static let linkLine = NSColor.welcomeDynamic(light: NSColor(hex: 0x178a4c), dark: NSColor(hex: 0x7fd18c))
    static let primaryFill = NSColor.welcomeDynamic(light: NSColor(hex: 0x0a0b0d), dark: NSColor(hex: 0xf2f4f7))
    static let primaryText = NSColor.welcomeDynamic(light: NSColor(hex: 0xffffff), dark: NSColor(hex: 0x0a0b0d))
    static let primaryHint = NSColor.welcomeDynamic(light: NSColor(hex: 0xb8bcc4), dark: NSColor(hex: 0x4a505a))
    static let glassButtonFill = NSColor.welcomeDynamic(light: NSColor(white: 1, alpha: 0.84), dark: NSColor(hex: 0x0b0d12, alpha: 0.55))
    static let glassButtonLine = NSColor.welcomeDynamic(light: NSColor(hex: 0x0a0b0d, alpha: 0.14), dark: NSColor(white: 1, alpha: 0.22))
    static let hint = NSColor.welcomeDynamic(light: NSColor(hex: 0x4a505a), dark: NSColor(hex: 0xc9d0da))
    static let dot = NSColor.welcomeDynamic(light: NSColor(hex: 0x101828, alpha: 0.10), dark: NSColor(white: 1, alpha: 0.07))
    static let glass = NSColor.welcomeDynamic(light: NSColor(white: 1, alpha: 0.78), dark: NSColor(hex: 0x0b0d12, alpha: 0.62))
    static let glassLine = NSColor.welcomeDynamic(light: NSColor(hex: 0x0a0b0d, alpha: 0.12), dark: NSColor(white: 1, alpha: 0.16))
    static let iconGlow = NSColor.welcomeDynamic(light: NSColor(hex: 0x059669, alpha: 0.55), dark: NSColor(hex: 0x10b981, alpha: 0.55))
    static let dropRing = NSColor.welcomeDynamic(light: NSColor(hex: 0x178a4c), dark: NSColor(hex: 0x7fd18c))
    static let dropHalo = NSColor.welcomeDynamic(light: NSColor(hex: 0x178a4c, alpha: 0.16), dark: NSColor(hex: 0x7fd18c, alpha: 0.18))
    static let caret = NSColor(hex: 0x7fd18c)
}

/// The welcome window's type: Instrument Sans (bundled, registered through
/// ATSApplicationFontsPath) for the wordmark, tagline, buttons and names;
/// the system font elsewhere and SF Mono for paths. A Mac where the font
/// did not register gets the system font at the same weight.
enum WelcomeFont {
    static let familyName = "Instrument Sans"
    private static var didTryRegistering = false

    /// True when Instrument Sans is available to this process.
    static var isDisplayFontAvailable: Bool { display(size: 12, weight: .regular).familyName == familyName }

    static func display(size: CGFloat, weight: NSFont.Weight) -> NSFont {
        if let font = instrument(size: size, weight: weight) { return font }
        if !didTryRegistering {
            didTryRegistering = true
            registerBundledFont()
            if let font = instrument(size: size, weight: weight) { return font }
        }
        return .systemFont(ofSize: size, weight: weight)
    }

    private static func instrument(size: CGFloat, weight: NSFont.Weight) -> NSFont? {
        let descriptor = NSFontDescriptor(fontAttributes: [.family: familyName, .traits: [NSFontDescriptor.TraitKey.weight: weight.rawValue]])
        guard let font = NSFont(descriptor: descriptor, size: size), font.familyName == familyName else { return nil }
        return font
    }

    /// A build whose plist lacks the key, or a test host that is not the app, still finds the file.
    private static func registerBundledFont() {
        guard let folder = Bundle.main.resourceURL?.appendingPathComponent("Fonts", isDirectory: true),
              let files = try? FileManager.default.contentsOfDirectory(at: folder, includingPropertiesForKeys: nil) else { return }
        for file in files where file.pathExtension == "ttf" {
            CTFontManagerRegisterFontsForURL(file as CFURL, .process, nil)
        }
    }

    static func mono(size: CGFloat, weight: NSFont.Weight = .regular) -> NSFont {
        .monospacedSystemFont(ofSize: size, weight: weight)
    }

    /// `text` in the display font with the letter spacing the mockup sets in em.
    static func attributed(_ text: String, size: CGFloat, weight: NSFont.Weight, color: NSColor, trackingEm: CGFloat = 0, alignment: NSTextAlignment = .center) -> NSAttributedString {
        let style = NSMutableParagraphStyle()
        style.alignment = alignment
        return NSAttributedString(string: text, attributes: [
            .font: display(size: size, weight: weight), .foregroundColor: color, .kern: trackingEm * size, .paragraphStyle: style,
        ])
    }
}

/// The Reduce Motion setting, replaceable so a test can turn it on.
enum WelcomeMotion {
    nonisolated(unsafe) static var reduceMotion: () -> Bool = { NSWorkspace.shared.accessibilityDisplayShouldReduceMotion }
}

/// A view whose origin is at the top left, so layout reads as the mockup does.
class FlippedView: NSView {
    override var isFlipped: Bool { true }
}
