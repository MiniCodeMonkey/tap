import AppKit

/// "Asks first": the sheet before the link is made, as the InstallConfirm
/// board draws it. Return is Install: the action is harmless and undone by
/// deleting the link. The other-tap card is a FormCard, as the Settings
/// panes' cards are (an NSBox whose contentView is a stack collapses to
/// its title).
final class InstallConfirmSheet: QuestionSheet {
    let subLabel = NSTextField(labelWithString: "To remove it later, delete the link.")
    let otherTapCard: NSView
    let otherTapLabel = NSTextField(labelWithString: "")
    let otherTapNote = NSTextField(labelWithString: "Stays exactly as it is. Tap never replaces or deletes it.")
    let otherTapPath = NSTextField(labelWithString: "")
    var installButton: NSButton { acceptButton }
    var cancelButton: NSButton { declineButton }

    init(installer: CommandLineInstaller, bundledVersion: String?, otherTap: (path: String, version: String?)?) {
        let directory = (installer.linkDirectory.path as NSString).abbreviatingWithTildeInPath
        let body: String
        if let otherTap {
            let otherDirectory = (otherTap.path as NSString).deletingLastPathComponent
            body = "Tap links its bundled tap \(bundledVersion ?? "") into \(directory), which comes before \(otherDirectory) on your PATH, so Terminal will run tap \(bundledVersion ?? "")."
        } else {
            body = "Tap links its bundled tap into \(directory), so Terminal runs the same tap as the app. Nothing else on your Mac changes."
        }
        otherTapLabel.font = .systemFont(ofSize: 13, weight: .semibold)
        otherTapNote.font = .systemFont(ofSize: 11)
        otherTapNote.textColor = .secondaryLabelColor
        otherTapPath.font = .monospacedSystemFont(ofSize: 12, weight: .regular)
        otherTapPath.textColor = .secondaryLabelColor
        let left = NSStackView(views: [otherTapLabel, otherTapNote])
        left.orientation = .vertical
        left.alignment = .leading
        left.spacing = 2
        let row = FormCard.row(leading: [left], trailing: [otherTapPath])
        let card = FormCard(rows: [row])
        otherTapCard = card
        let detail = NSStackView(views: [subLabel, card])
        detail.orientation = .vertical
        detail.alignment = .leading
        detail.spacing = 8
        super.init(kind: "install-command", title: "Install the tap command?", body: body,
                   path: "\(directory)/tap \u{2192} \(installer.bundledTap.path)", decline: "Cancel", accept: "Install", escape: .decline, returnAnswer: .accept, detail: detail)
        subLabel.font = .systemFont(ofSize: 12)
        subLabel.textColor = .secondaryLabelColor
        if let otherTap {
            otherTapLabel.stringValue = otherTap.version.map { "tap \($0)" } ?? "tap (unknown version)"
            otherTapPath.stringValue = otherTap.path
            subLabel.isHidden = true
        } else {
            card.isHidden = true
        }
        card.widthAnchor.constraint(equalTo: detail.widthAnchor).isActive = true
        // The board draws the path line before the small line and the
        // card; QuestionSheet puts pathLabel after the detail view.
        if let stack = contentView as? NSStackView {
            stack.removeArrangedSubview(pathLabel)
            stack.insertArrangedSubview(pathLabel, at: 2)
        }
    }
}
