import AppKit

/// The welcome window's dashed "drop a Markdown file here" box. It shows
/// the drop target and hands a dropped .md file's URL to `onDrop`; the
/// window's `WelcomeContentView` receives the drag itself, so the whole
/// window accepts a file and anything but Markdown is refused.
final class WelcomeDropZone: NSView {
    static let markdownExtensions: Set<String> = ["md", "markdown"]

    var onDrop: ((URL) -> Void)?
    private let dashedBorder = CAShapeLayer()
    var isTargeted = false {
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
}

/// The welcome window's content view: a Markdown file dropped anywhere on
/// it opens, and the drop zone lights up while one is over the window.
final class WelcomeContentView: NSView {
    let dropZone: WelcomeDropZone

    init(dropZone: WelcomeDropZone) {
        self.dropZone = dropZone
        super.init(frame: .zero)
        registerForDraggedTypes([.fileURL])
    }

    required init?(coder: NSCoder) { fatalError("init(coder:) is not used") }

    override func draggingEntered(_ sender: NSDraggingInfo) -> NSDragOperation {
        guard WelcomeDropZone.markdownURL(in: sender.draggingPasteboard) != nil else { return [] }
        dropZone.isTargeted = true
        return .copy
    }

    override func draggingUpdated(_ sender: NSDraggingInfo) -> NSDragOperation {
        draggingEntered(sender)
    }

    override func draggingExited(_ sender: NSDraggingInfo?) { dropZone.isTargeted = false }

    override func draggingEnded(_ sender: NSDraggingInfo) { dropZone.isTargeted = false }

    override func performDragOperation(_ sender: NSDraggingInfo) -> Bool {
        dropZone.isTargeted = false
        guard let url = WelcomeDropZone.markdownURL(in: sender.draggingPasteboard) else { return false }
        dropZone.onDrop?(url)
        return true
    }
}
