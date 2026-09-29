import AppKit

/// The welcome window's dashed "drop a Markdown file here" box. It takes a
/// dropped .md file and hands its URL to `onDrop`; anything else is refused.
final class WelcomeDropZone: NSView {
    static let markdownExtensions: Set<String> = ["md", "markdown"]

    var onDrop: ((URL) -> Void)?
    private let dashedBorder = CAShapeLayer()
    private var isTargeted = false {
        didSet { updateAppearance() }
    }

    init(text: String) {
        super.init(frame: .zero)
        wantsLayer = true
        dashedBorder.fillColor = nil
        dashedBorder.lineWidth = 1.5
        dashedBorder.lineDashPattern = [5, 4]
        layer?.addSublayer(dashedBorder)
        let label = NSTextField(labelWithString: text)
        label.font = .systemFont(ofSize: 12)
        label.textColor = .secondaryLabelColor
        label.translatesAutoresizingMaskIntoConstraints = false
        addSubview(label)
        NSLayoutConstraint.activate([
            label.centerXAnchor.constraint(equalTo: centerXAnchor), label.centerYAnchor.constraint(equalTo: centerYAnchor),
            widthAnchor.constraint(equalTo: label.widthAnchor, constant: 36), heightAnchor.constraint(equalTo: label.heightAnchor, constant: 20),
        ])
        registerForDraggedTypes([.fileURL])
        setAccessibilityIdentifier("welcome-drop-zone")
        setAccessibilityLabel(text)
    }

    required init?(coder: NSCoder) { fatalError("init(coder:) is not used") }

    /// The first dropped file that is a Markdown file.
    static func markdownURL(in pasteboard: NSPasteboard) -> URL? {
        let urls = pasteboard.readObjects(forClasses: [NSURL.self], options: [.urlReadingFileURLsOnly: true]) as? [URL]
        return urls?.first { markdownExtensions.contains($0.pathExtension.lowercased()) }
    }

    override func layout() {
        super.layout()
        dashedBorder.frame = bounds
        dashedBorder.path = CGPath(roundedRect: bounds.insetBy(dx: 1, dy: 1), cornerWidth: 10, cornerHeight: 10, transform: nil)
        updateAppearance()
    }

    override func viewDidChangeEffectiveAppearance() {
        super.viewDidChangeEffectiveAppearance()
        updateAppearance()
    }

    private func updateAppearance() {
        effectiveAppearance.performAsCurrentDrawingAppearance {
            dashedBorder.strokeColor = (isTargeted ? NSColor.controlAccentColor : NSColor.separatorColor).cgColor
            dashedBorder.fillColor = isTargeted ? NSColor.controlAccentColor.withAlphaComponent(0.1).cgColor : nil
        }
    }

    override func draggingEntered(_ sender: NSDraggingInfo) -> NSDragOperation {
        guard Self.markdownURL(in: sender.draggingPasteboard) != nil else { return [] }
        isTargeted = true
        return .copy
    }

    override func draggingExited(_ sender: NSDraggingInfo?) { isTargeted = false }

    override func draggingEnded(_ sender: NSDraggingInfo) { isTargeted = false }

    override func performDragOperation(_ sender: NSDraggingInfo) -> Bool {
        isTargeted = false
        guard let url = Self.markdownURL(in: sender.draggingPasteboard) else { return false }
        onDrop?(url)
        return true
    }
}
