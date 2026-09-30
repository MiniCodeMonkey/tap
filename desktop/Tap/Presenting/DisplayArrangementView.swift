import AppKit

/// A view that fills itself with a tinted, outlined surface: the colours
/// are resolved for the current appearance whenever it changes, so light
/// and dark both read as system colours.
class TintedSurfaceView: NSView {
    var fillColor: NSColor = .clear { didSet { needsDisplay = true } }
    var strokeColor: NSColor? { didSet { needsDisplay = true } }
    var cornerRadius: CGFloat = 8 { didSet { needsDisplay = true } }

    override init(frame: NSRect) {
        super.init(frame: frame)
        wantsLayer = true
    }

    required init?(coder: NSCoder) { fatalError("init(coder:) is not used") }

    override var wantsUpdateLayer: Bool { true }

    override func updateLayer() {
        guard let layer else { return }
        layer.cornerRadius = cornerRadius
        effectiveAppearance.performAsCurrentDrawingAppearance {
            layer.backgroundColor = fillColor.cgColor
            layer.borderColor = strokeColor?.cgColor
        }
        layer.borderWidth = strokeColor == nil ? 0 : 1
    }
}

/// The popover's picture of the displays: every connected screen drawn to
/// scale, labelled with its role, and under each a menu that names the
/// screen and chooses its role. Clicking a screen makes it the audience.
final class DisplayArrangementView: NSView {
    /// One screen's shape, labelled with what it does.
    final class ScreenBox: TintedSurfaceView {
        let roleLabel = NSTextField(labelWithString: "")
        var onClick: (() -> Void)?

        override init(frame: NSRect) {
            super.init(frame: frame)
            cornerRadius = 5
            roleLabel.font = .systemFont(ofSize: 10, weight: .bold)
            roleLabel.translatesAutoresizingMaskIntoConstraints = false
            roleLabel.lineBreakMode = .byTruncatingTail
            addSubview(roleLabel)
            NSLayoutConstraint.activate([
                roleLabel.centerXAnchor.constraint(equalTo: centerXAnchor),
                roleLabel.centerYAnchor.constraint(equalTo: centerYAnchor),
                roleLabel.leadingAnchor.constraint(greaterThanOrEqualTo: leadingAnchor, constant: 4),
            ])
            setAccessibilityElement(true)
            setAccessibilityRole(.button)
        }

        required init?(coder: NSCoder) { fatalError("init(coder:) is not used") }

        func show(role: DisplayRole) {
            switch role {
            case .audience:
                roleLabel.stringValue = "Audience"
                roleLabel.textColor = .controlAccentColor
                fillColor = NSColor.controlAccentColor.withAlphaComponent(0.16)
                strokeColor = .controlAccentColor
            case .presenter:
                roleLabel.stringValue = "Presenter"
                roleLabel.textColor = .labelColor
                fillColor = .controlBackgroundColor
                strokeColor = .labelColor
            case .notUsed:
                roleLabel.stringValue = "Not used"
                roleLabel.textColor = .secondaryLabelColor
                fillColor = .controlBackgroundColor
                strokeColor = .separatorColor
            }
        }

        override func mouseDown(with event: NSEvent) {
            onClick?()
        }

        override func accessibilityPerformPress() -> Bool {
            onClick?()
            return onClick != nil
        }
    }

    /// The menu items carry their role in a box, since a Swift enum is not an object.
    private final class RoleBox: NSObject {
        let role: DisplayRole
        init(_ role: DisplayRole) { self.role = role }
    }

    /// A screen's box and its role menu.
    final class Tile: NSStackView {
        let screen: ScreenInfo
        let box = ScreenBox(frame: .zero)
        /// A pull-down: its title is the screen's name, its items the three roles.
        let menuButton = NSPopUpButton(frame: .zero, pullsDown: true)
        private(set) var role = DisplayRole.notUsed
        private let onAssign: (DisplayRole, ScreenInfo) -> Void

        init(screen: ScreenInfo, boxSize: NSSize, width: CGFloat, onAssign: @escaping (DisplayRole, ScreenInfo) -> Void) {
            self.screen = screen
            self.onAssign = onAssign
            super.init(frame: .zero)
            orientation = .vertical
            alignment = .centerX
            spacing = 5
            box.translatesAutoresizingMaskIntoConstraints = false
            box.widthAnchor.constraint(equalToConstant: boxSize.width).isActive = true
            box.heightAnchor.constraint(equalToConstant: boxSize.height).isActive = true
            box.onClick = { onAssign(.audience, screen) }
            menuButton.controlSize = .small
            menuButton.font = .systemFont(ofSize: 11)
            menuButton.lineBreakMode = .byTruncatingTail
            menuButton.addItem(withTitle: screen.name)
            for (title, role) in [("Audience", DisplayRole.audience), ("Presenter", .presenter), ("Not used", .notUsed)] {
                let item = NSMenuItem(title: title, action: #selector(roleChosen(_:)), keyEquivalent: "")
                item.representedObject = RoleBox(role)
                item.target = self
                menuButton.menu?.addItem(item)
            }
            menuButton.translatesAutoresizingMaskIntoConstraints = false
            menuButton.widthAnchor.constraint(equalToConstant: width).isActive = true
            addArrangedSubview(box)
            addArrangedSubview(menuButton)
            menuButton.setAccessibilityIdentifier("display-role-menu")
            menuButton.setAccessibilityLabel("Role of \(screen.name)")
        }

        required init?(coder: NSCoder) { fatalError("init(coder:) is not used") }

        func show(role: DisplayRole) {
            self.role = role
            box.show(role: role)
            box.setAccessibilityLabel("\(box.roleLabel.stringValue), \(screen.name). Click to make it the audience screen.")
            for item in menuButton.menu?.items.dropFirst() ?? [] {
                item.state = (item.representedObject as? RoleBox)?.role == role ? .on : .off
            }
        }

        @objc private func roleChosen(_ sender: NSMenuItem) {
            guard let role = (sender.representedObject as? RoleBox)?.role else { return }
            onAssign(role, screen)
        }

        /// The menu item for `role`, for a test to press.
        func menuItem(for role: DisplayRole) -> NSMenuItem? {
            menuButton.menu?.items.first { ($0.representedObject as? RoleBox)?.role == role }
        }
    }

    /// The most the row of screens is wide.
    static let availableWidth: CGFloat = 328
    private static let spacing: CGFloat = 12
    private let row = NSStackView()
    private(set) var tiles: [Tile] = []
    /// Called with the role a person chose for a screen, by menu or by a click.
    var onAssign: ((DisplayRole, ScreenInfo) -> Void)?

    override init(frame: NSRect) {
        super.init(frame: frame)
        row.orientation = .horizontal
        row.alignment = .bottom
        row.spacing = Self.spacing
        row.translatesAutoresizingMaskIntoConstraints = false
        addSubview(row)
        NSLayoutConstraint.activate([
            row.topAnchor.constraint(equalTo: topAnchor),
            row.bottomAnchor.constraint(equalTo: bottomAnchor),
            row.centerXAnchor.constraint(equalTo: centerXAnchor),
            row.leadingAnchor.constraint(greaterThanOrEqualTo: leadingAnchor),
            row.trailingAnchor.constraint(lessThanOrEqualTo: trailingAnchor),
        ])
        setAccessibilityElement(true)
        setAccessibilityRole(.group)
        setAccessibilityLabel("Displays")
    }

    required init?(coder: NSCoder) { fatalError("init(coder:) is not used") }

    /// Draws every screen of the arrangement, left to right as they sit on
    /// the desk (by frame, not by the order the system lists them). The tiles
    /// are rebuilt only when the set of screens changed, so a menu that is
    /// open stays open.
    func update(arrangement: DisplayArrangement) {
        let screens = arrangement.screens.enumerated().sorted { first, second in
            first.element.frame.minX != second.element.frame.minX ? first.element.frame.minX < second.element.frame.minX : first.offset < second.offset
        }.map(\.element)
        if tiles.map(\.screen) != screens {
            for tile in tiles {
                row.removeArrangedSubview(tile)
                tile.removeFromSuperview()
            }
            let sizes = Self.boxSizes(for: screens)
            let minimumColumn = Self.minimumColumnWidth(count: screens.count)
            tiles = zip(screens, sizes).map { screen, size in
                Tile(screen: screen, boxSize: size, width: max(size.width, minimumColumn)) { [weak self] role, screen in
                    self?.onAssign?(role, screen)
                }
            }
            for tile in tiles { row.addArrangedSubview(tile) }
        }
        for tile in tiles { tile.show(role: arrangement.role(of: tile.screen)) }
    }

    private static func minimumColumnWidth(count: Int) -> CGFloat {
        min(96, (availableWidth - spacing * CGFloat(max(count - 1, 0))) / CGFloat(max(count, 1)))
    }

    /// The screens' shapes, drawn to one scale, the widest that fits the row.
    static func boxSizes(for screens: [ScreenInfo]) -> [NSSize] {
        let minimumColumn = minimumColumnWidth(count: screens.count)
        let widths = screens.map { max($0.frame.width, 1) }
        let gaps = spacing * CGFloat(max(screens.count - 1, 0))
        var scale: CGFloat = 0.06
        var attempts = 0
        while widths.map({ max($0 * scale, minimumColumn) }).reduce(0, +) + gaps > availableWidth, attempts < 60 {
            scale *= 0.92
            attempts += 1
        }
        return zip(screens, widths).map { screen, width in
            let boxWidth = max((width * scale).rounded(), 44)
            let ratio = screen.frame.width > 0 ? min(max(screen.frame.height / screen.frame.width, 0.3), 1) : 0.625
            return NSSize(width: boxWidth, height: (boxWidth * ratio).rounded())
        }
    }
}
