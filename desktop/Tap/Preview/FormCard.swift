import AppKit

/// A card of the inspector's forms, as the DeckTab boards draw it: a
/// rounded surface with its rows one under the other and a hairline
/// between two rows. The rows sit in a stack pinned by constraints to the
/// card's four edges, so the card is exactly as tall as its rows and as
/// wide as whoever places it makes it; nothing about its size comes from
/// a frame set by hand.
final class FormCard: NSBox {
    /// The rows, top to bottom, without the hairlines between them.
    private(set) var rows: [NSView] = []
    private let column = NSStackView()

    init(rows: [NSView]) {
        super.init(frame: .zero)
        boxType = .custom
        titlePosition = .noTitle
        fillColor = EditorPalette.boxFill
        borderColor = EditorPalette.boxBorder
        borderWidth = 0.5
        cornerRadius = 10
        contentViewMargins = .zero
        // A row with a problem is tinted to the card's edge: the corners are the card's.
        contentView?.wantsLayer = true
        contentView?.layer?.cornerRadius = 10
        contentView?.layer?.masksToBounds = true
        translatesAutoresizingMaskIntoConstraints = false
        column.orientation = .vertical
        column.alignment = .leading
        column.spacing = 0
        column.translatesAutoresizingMaskIntoConstraints = false
        contentView?.addSubview(column)
        NSLayoutConstraint.activate([
            column.topAnchor.constraint(equalTo: topAnchor),
            column.bottomAnchor.constraint(equalTo: bottomAnchor),
            column.leadingAnchor.constraint(equalTo: leadingAnchor),
            column.trailingAnchor.constraint(equalTo: trailingAnchor),
        ])
        for row in rows { append(row) }
    }

    @available(*, unavailable)
    required init?(coder: NSCoder) { nil }

    /// Adds a row under the others, a hairline above it when it is not the first.
    func append(_ row: NSView) {
        if !rows.isEmpty {
            let hairline = NSBox()
            hairline.boxType = .separator
            column.addArrangedSubview(hairline)
            hairline.widthAnchor.constraint(equalTo: column.widthAnchor).isActive = true
        }
        column.addArrangedSubview(row)
        row.widthAnchor.constraint(equalTo: column.widthAnchor).isActive = true
        rows.append(row)
    }

    /// A row of a card: the leading views on the left, the trailing views
    /// on the right, centered on one line at least 38 points tall. The
    /// leading views give way first when the card is narrow.
    static func row(leading: [NSView], trailing: [NSView]) -> NSStackView {
        let row = NSStackView()
        row.orientation = .horizontal
        row.alignment = .centerY
        row.spacing = 12
        row.edgeInsets = NSEdgeInsets(top: 6, left: 12, bottom: 6, right: 12)
        row.setViews(leading, in: .leading)
        row.setViews(trailing, in: .trailing)
        for view in leading { view.setContentCompressionResistancePriority(.defaultLow, for: .horizontal) }
        row.heightAnchor.constraint(greaterThanOrEqualToConstant: 38).isActive = true
        return row
    }

    /// A group of the form: its heading, then its cards (and any note
    /// under them), each as wide as the group.
    static func section(title: String?, content: [NSView]) -> NSStackView {
        var leading: [NSView] = []
        if let title {
            let heading = NSTextField(labelWithString: title)
            heading.font = .systemFont(ofSize: 11, weight: .semibold)
            heading.textColor = .secondaryLabelColor
            let headingRow = NSStackView(views: [heading])
            headingRow.edgeInsets = NSEdgeInsets(top: 0, left: 4, bottom: 0, right: 4)
            leading = [headingRow]
        }
        let section = NSStackView(views: leading + content)
        section.orientation = .vertical
        section.alignment = .leading
        section.spacing = 8
        if let headingRow = leading.first { section.setCustomSpacing(6, after: headingRow) }
        for view in content { view.widthAnchor.constraint(equalTo: section.widthAnchor).isActive = true }
        return section
    }
}
