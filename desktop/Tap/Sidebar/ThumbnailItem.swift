import AppKit

/// One thumbnail in the slide panel: the number, the image, an "updating"
/// mark while a new render is on its way, and the selection ring.
final class ThumbnailItem: NSCollectionViewItem {
    static let identifier = NSUserInterfaceItemIdentifier("ThumbnailItem")
    static let imageSize = NSSize(width: 150, height: 84)
    static let numberWidth: CGFloat = 20
    static let gap: CGFloat = 8

    let numberLabel = NSTextField(labelWithString: "")
    let thumbnailImageView = NSImageView()
    let updatingLabel = NSTextField(labelWithString: "updating")
    private(set) var slide: Slide?

    var isUpdating = false {
        didSet { updatingLabel.isHidden = !isUpdating }
    }

    override func loadView() {
        let root = NSView()
        numberLabel.font = .monospacedDigitSystemFont(ofSize: 11, weight: .semibold)
        numberLabel.alignment = .right
        numberLabel.textColor = .secondaryLabelColor
        thumbnailImageView.wantsLayer = true
        thumbnailImageView.imageScaling = .scaleProportionallyUpOrDown
        thumbnailImageView.layer?.cornerRadius = 6
        thumbnailImageView.layer?.masksToBounds = true
        thumbnailImageView.layer?.backgroundColor = NSColor.quaternaryLabelColor.cgColor
        updatingLabel.font = .systemFont(ofSize: 9, weight: .medium)
        updatingLabel.textColor = .white
        updatingLabel.wantsLayer = true
        updatingLabel.layer?.backgroundColor = NSColor.black.withAlphaComponent(0.55).cgColor
        updatingLabel.layer?.cornerRadius = 4
        updatingLabel.isHidden = true
        for view in [numberLabel, thumbnailImageView, updatingLabel] as [NSView] {
            view.translatesAutoresizingMaskIntoConstraints = false
            root.addSubview(view)
        }
        NSLayoutConstraint.activate([
            numberLabel.leadingAnchor.constraint(equalTo: root.leadingAnchor),
            numberLabel.widthAnchor.constraint(equalToConstant: Self.numberWidth),
            numberLabel.topAnchor.constraint(equalTo: root.topAnchor, constant: 2),
            thumbnailImageView.leadingAnchor.constraint(equalTo: numberLabel.trailingAnchor, constant: Self.gap),
            thumbnailImageView.topAnchor.constraint(equalTo: root.topAnchor),
            thumbnailImageView.widthAnchor.constraint(equalToConstant: Self.imageSize.width),
            thumbnailImageView.heightAnchor.constraint(equalToConstant: Self.imageSize.height),
            updatingLabel.trailingAnchor.constraint(equalTo: thumbnailImageView.trailingAnchor, constant: -4),
            updatingLabel.bottomAnchor.constraint(equalTo: thumbnailImageView.bottomAnchor, constant: -4),
        ])
        root.setAccessibilityElement(true)
        root.setAccessibilityRole(.button)
        view = root
        applySelection()
    }

    func configure(slide: Slide, image: NSImage?, isUpdating: Bool) {
        self.slide = slide
        numberLabel.stringValue = "\(slide.number)"
        thumbnailImageView.image = image
        self.isUpdating = isUpdating
        // A skipped slide is dimmed, as its box is in the editor.
        thumbnailImageView.alphaValue = slide.skip ? 0.45 : 1
        numberLabel.alphaValue = slide.skip ? 0.6 : 1
        view.setAccessibilityLabel(SlideAccessibility.label(for: slide))
        view.setAccessibilityIdentifier("thumbnail-\(slide.number)")
    }

    override var isSelected: Bool {
        didSet { applySelection() }
    }

    private func applySelection() {
        thumbnailImageView.layer?.borderWidth = isSelected ? 3 : 0.5
        thumbnailImageView.layer?.borderColor = isSelected ? NSColor.controlAccentColor.cgColor : NSColor.black.withAlphaComponent(0.07).cgColor
        numberLabel.textColor = isSelected ? .controlAccentColor : .secondaryLabelColor
        view.setAccessibilitySelected(isSelected)
    }
}
