import AppKit

/// The drag state of the welcome window: while a Markdown file is dragged
/// over it, a green ring and halo sit inside the window's edge and a glass
/// pill names the file ("Drop to open slides.md"). It draws nothing
/// otherwise, takes no clicks, and hands a dropped .md file's URL to
/// `onDrop`; the window's `WelcomeContentView` receives the drag itself, so
/// the whole window accepts a file and anything but Markdown is refused.
final class WelcomeDropZone: NSView {
    static let markdownExtensions: Set<String> = ["md", "markdown"]
    private static let ringInset: CGFloat = 8

    var onDrop: ((URL) -> Void)?
    /// Called when a Markdown file enters or leaves the window, so the rest of the window can step back.
    var onTargetChange: ((Bool) -> Void)?
    private(set) var fileName = ""
    private let haloLayer = CALayer()
    private let ringLayer = CALayer()
    private let pill = FlippedView()
    private let pillLabel = NSTextField(labelWithString: "")
    private let dotLayer = CALayer()

    var isTargeted = false {
        didSet {
            guard isTargeted != oldValue else { return }
            updateAppearance()
            NSAnimationContext.runAnimationGroup { context in
                context.duration = WelcomeMotion.reduceMotion() ? 0 : 0.2
                animator().alphaValue = isTargeted ? 1 : 0
            }
            onTargetChange?(isTargeted)
        }
    }

    init() {
        super.init(frame: .zero)
        wantsLayer = true
        alphaValue = 0
        for layer in [haloLayer, ringLayer] { layer.cornerRadius = 9 }
        haloLayer.borderWidth = 6
        ringLayer.borderWidth = 2
        layer?.addSublayer(haloLayer)
        layer?.addSublayer(ringLayer)

        pill.wantsLayer = true
        pill.layer?.cornerRadius = 19
        pill.layer?.borderWidth = 1
        pill.layer?.shadowOpacity = 0.3
        pill.layer?.shadowRadius = 14
        pill.layer?.shadowOffset = CGSize(width: 0, height: -8)
        pillLabel.translatesAutoresizingMaskIntoConstraints = false
        pillLabel.lineBreakMode = .byTruncatingMiddle
        pillLabel.maximumNumberOfLines = 1
        pill.addSubview(pillLabel)
        dotLayer.cornerRadius = 3.5
        dotLayer.shadowOpacity = 1
        dotLayer.shadowRadius = 4
        dotLayer.shadowOffset = .zero
        pill.layer?.addSublayer(dotLayer)
        pill.translatesAutoresizingMaskIntoConstraints = false
        addSubview(pill)
        NSLayoutConstraint.activate([
            pill.centerXAnchor.constraint(equalTo: centerXAnchor),
            pill.centerYAnchor.constraint(equalTo: centerYAnchor),
            pill.heightAnchor.constraint(equalToConstant: 38),
            pill.widthAnchor.constraint(lessThanOrEqualTo: widthAnchor, constant: -80),
            pillLabel.leadingAnchor.constraint(equalTo: pill.leadingAnchor, constant: 34),
            pillLabel.trailingAnchor.constraint(equalTo: pill.trailingAnchor, constant: -18),
            pillLabel.centerYAnchor.constraint(equalTo: pill.centerYAnchor),
        ])
        setAccessibilityIdentifier("welcome-drop-zone")
        setAccessibilityLabel("Drop a Markdown file to open it")
        updateAppearance()
    }

    required init?(coder: NSCoder) { fatalError("init(coder:) is not used") }

    override func hitTest(_ point: NSPoint) -> NSView? { nil }

    /// The first dropped file that is a Markdown file.
    static func markdownURL(in pasteboard: NSPasteboard) -> URL? {
        let urls = pasteboard.readObjects(forClasses: [NSURL.self], options: [.urlReadingFileURLsOnly: true]) as? [URL]
        return urls?.first { markdownExtensions.contains($0.pathExtension.lowercased()) }
    }

    /// Names the file the pill offers to open.
    func setFileName(_ name: String) {
        guard name != fileName else { return }
        fileName = name
        updateAppearance()
    }

    override func layout() {
        super.layout()
        CATransaction.begin()
        CATransaction.setDisableActions(true)
        ringLayer.frame = bounds.insetBy(dx: Self.ringInset, dy: Self.ringInset)
        haloLayer.frame = ringLayer.frame.insetBy(dx: -6, dy: -6)
        haloLayer.cornerRadius = 15
        dotLayer.frame = CGRect(x: 18, y: 15.5, width: 7, height: 7)
        CATransaction.commit()
    }

    override func viewDidChangeEffectiveAppearance() {
        super.viewDidChangeEffectiveAppearance()
        updateAppearance()
    }

    private func updateAppearance() {
        pillLabel.attributedStringValue = WelcomeFont.attributed(fileName.isEmpty ? "Drop to open" : "Drop to open \(fileName)", size: 15, weight: .semibold, color: WelcomeColor.ink, alignment: .left)
        effectiveAppearance.performAsCurrentDrawingAppearance {
            ringLayer.borderColor = WelcomeColor.dropRing.cgColor
            ringLayer.backgroundColor = WelcomeColor.dropHalo.withAlphaComponent(0.05).cgColor
            haloLayer.borderColor = WelcomeColor.dropHalo.cgColor
            pill.layer?.backgroundColor = WelcomeColor.glass.cgColor
            pill.layer?.borderColor = WelcomeColor.glassLine.cgColor
            dotLayer.backgroundColor = WelcomeColor.dropRing.cgColor
            dotLayer.shadowColor = WelcomeColor.dropRing.cgColor
        }
    }
}

/// The welcome window's content view: a Markdown file dropped anywhere on
/// it opens, and the drop overlay lights up while one is over the window.
class WelcomeContentView: NSView {
    let dropZone: WelcomeDropZone

    init(dropZone: WelcomeDropZone) {
        self.dropZone = dropZone
        super.init(frame: .zero)
        registerForDraggedTypes([.fileURL])
    }

    required init?(coder: NSCoder) { fatalError("init(coder:) is not used") }

    override func draggingEntered(_ sender: NSDraggingInfo) -> NSDragOperation {
        guard let url = WelcomeDropZone.markdownURL(in: sender.draggingPasteboard) else {
            dropZone.isTargeted = false
            return []
        }
        dropZone.setFileName(url.lastPathComponent)
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
