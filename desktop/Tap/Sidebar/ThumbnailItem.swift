import AppKit
import TapDesktopCore

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
    /// The slide's text card, shown until the thumbnail's picture arrives.
    let cardView = SlideCardView()
    /// How long the picture takes to fade in over the card.
    static let crossfadeDuration: TimeInterval = 0.35
    private(set) var slide: Slide?
    /// True while the card is what the item shows: the slide has no picture yet.
    var showsCard: Bool { !cardView.isHidden }

    var isUpdating = false {
        didSet { updatingLabel.isHidden = !isUpdating || showsCard }
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
        cardView.layer?.cornerRadius = 6
        cardView.isHidden = true
        for view in [numberLabel, thumbnailImageView, cardView, updatingLabel] as [NSView] {
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
            cardView.leadingAnchor.constraint(equalTo: thumbnailImageView.leadingAnchor),
            cardView.trailingAnchor.constraint(equalTo: thumbnailImageView.trailingAnchor),
            cardView.topAnchor.constraint(equalTo: thumbnailImageView.topAnchor),
            cardView.bottomAnchor.constraint(equalTo: thumbnailImageView.bottomAnchor),
            updatingLabel.trailingAnchor.constraint(equalTo: thumbnailImageView.trailingAnchor, constant: -4),
            updatingLabel.bottomAnchor.constraint(equalTo: thumbnailImageView.bottomAnchor, constant: -4),
        ])
        root.setAccessibilityElement(true)
        root.setAccessibilityRole(.button)
        view = root
        applySelection()
    }

    /// Shows the slide. Without a picture the item shows `card` on `paper`;
    /// when the picture arrives for the slide it already shows, it fades in
    /// over the card, unless Reduce Motion is on.
    func configure(slide: Slide, image: NSImage?, isUpdating: Bool, card: SlideCard = SlideCard(heading: ""), paper: PaperColour = .neutral) {
        let sameSlide = self.slide?.number == slide.number
        let wasShowingCard = showsCard
        self.slide = slide
        numberLabel.stringValue = "\(slide.number)"
        thumbnailImageView.image = image
        if image == nil {
            cardView.layer?.removeAllAnimations()
            cardView.alphaValue = 1
            cardView.configure(card: card, paper: paper)
            cardView.isHidden = false
            cardView.startSheen()
        } else if wasShowingCard, sameSlide, !WelcomeMotion.reduceMotion() {
            fadeCardOut()
        } else {
            hideCard()
        }
        self.isUpdating = isUpdating
        // A skipped slide is dimmed, as its box is in the editor.
        thumbnailImageView.alphaValue = slide.skip ? 0.45 : 1
        numberLabel.alphaValue = slide.skip ? 0.6 : 1
        view.setAccessibilityLabel(SlideAccessibility.label(for: slide))
        view.setAccessibilityIdentifier("thumbnail-\(slide.number)")
    }

    private func hideCard() {
        cardView.stopSheen()
        cardView.isHidden = true
    }

    /// The picture is in place under the card; the card fades away over it.
    private func fadeCardOut() {
        cardView.stopSheen()
        cardView.fadeOut(duration: Self.crossfadeDuration) { [weak self] in
            // An item reused for a slide with no picture shows its card again; only this fade's end hides it.
            guard let self, self.thumbnailImageView.image != nil, self.cardView.alphaValue == 0 else { return }
            self.cardView.isHidden = true
            self.cardView.alphaValue = 1
        }
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
