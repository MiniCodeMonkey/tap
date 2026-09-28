import XCTest
@testable import Tap

/// The Image Generation and Command Line panes, and the InstallConfirm
/// sheet, at the window's real width (760pt), laid out on alignment rects
/// rather than raw frames (the house rule from D5's Deck tab defect: an
/// NSBox whose contentView is a stack collapses to its title). Both panes
/// are built from FormCard, as Task 12's SettingsCard fix requires. Every
/// state either board draws gets its own test here. Where another tap is
/// listed, its name and note are one block and the path is centered
/// against that block, as the boards draw it, so the block and the path
/// are one row.
final class ImageAndCommandLineLayoutTests: HostedTestCase {
    // MARK: Image Generation

    func testImageGenerationPaneLayoutWithSavedKey() {
        let pane = ImageGenerationSettingsViewController()
        let content = pane.view
        content.setFrameSize(NSSize(width: 760, height: 150))
        pane.keyField.stringValue = "placeholder-not-a-secret"
        pane.sourceLabel.stringValue = "Stored in your Keychain. GEMINI_API_KEY from your shell takes precedence."
        content.layoutSubtreeIfNeeded()
        assertRowsDoNotOverlap([[pane.keyField]], in: content)
        assertCard(pane.card, holds: [pane.sourceLabel, pane.keyField], in: content)
    }

    func testImageGenerationPaneLayoutWithShellKey() {
        let pane = ImageGenerationSettingsViewController()
        let content = pane.view
        content.setFrameSize(NSSize(width: 760, height: 150))
        pane.keyField.isEnabled = false
        pane.sourceLabel.stringValue = "Your shell sets GEMINI_API_KEY, so tap uses that key; the Keychain's is not used."
        content.layoutSubtreeIfNeeded()
        assertRowsDoNotOverlap([[pane.keyField]], in: content)
        assertCard(pane.card, holds: [pane.sourceLabel, pane.keyField], in: content)
    }

    // MARK: Command Line

    func testCommandLinePaneLayoutNoOtherTap() {
        let pane = CommandLineSettingsViewController()
        let content = pane.view
        content.setFrameSize(NSSize(width: 760, height: 260))
        pane.bundledLabel.stringValue = "tap 2.1.0"
        pane.bundledPathLabel.stringValue = "Tap.app/Contents/Resources/tap"
        pane.otherLabel.stringValue = "No other tap is on your PATH."
        pane.installButton.title = "Install in ~/.local/bin…"
        pane.installHint.stringValue = "Asks first. ~/.local/bin is on your PATH."
        content.layoutSubtreeIfNeeded()
        assertRowsDoNotOverlap([
            [pane.bundledLabel, pane.bundledPathLabel],
            [pane.otherLabel],
            [pane.installButton],
            [pane.installHint],
        ], in: content)
        for card in pane.cards { assertCard(card, holds: card.rows, in: content) }
    }

    func testCommandLinePaneLayoutAnotherTapOnPath() throws {
        let pane = CommandLineSettingsViewController()
        let content = pane.view
        content.setFrameSize(NSSize(width: 760, height: 260))
        pane.bundledLabel.stringValue = "tap 2.1.0"
        pane.bundledPathLabel.stringValue = "Tap.app/Contents/Resources/tap"
        pane.otherLabel.stringValue = "tap 2.0.0"
        pane.otherNoteLabel.stringValue = "Installed by Homebrew. Tap never replaces or deletes it."
        pane.otherPathLabel.stringValue = "/opt/homebrew/bin/tap"
        pane.installButton.title = "Install in ~/.local/bin…"
        pane.installHint.stringValue = "Asks first. ~/.local/bin comes before /opt/homebrew/bin on your PATH."
        content.layoutSubtreeIfNeeded()
        let otherBlock = try XCTUnwrap(pane.otherLabel.superview, "the name with its note under it")
        assertRowsDoNotOverlap([
            [pane.bundledLabel, pane.bundledPathLabel],
            [otherBlock, pane.otherPathLabel],
            [pane.installButton],
            [pane.installHint],
        ], in: content)
        for card in pane.cards { assertCard(card, holds: card.rows, in: content) }
    }

    func testCommandLinePaneLayoutForeignFile() throws {
        let pane = CommandLineSettingsViewController()
        let content = pane.view
        content.setFrameSize(NSSize(width: 760, height: 260))
        pane.bundledLabel.stringValue = "tap 2.1.0"
        pane.bundledPathLabel.stringValue = "Tap.app/Contents/Resources/tap"
        pane.otherLabel.stringValue = "tap (unknown version)"
        pane.otherNoteLabel.stringValue = "Tap did not install it. Tap never replaces or deletes it."
        pane.otherPathLabel.stringValue = "~/.local/bin/tap"
        pane.installButton.title = "Install in ~/.local/bin…"
        pane.installButton.isEnabled = false
        pane.installHint.stringValue = "~/.local/bin/tap is a tap that Tap did not install. Tap never replaces or deletes it."
        content.layoutSubtreeIfNeeded()
        let otherBlock = try XCTUnwrap(pane.otherLabel.superview, "the name with its note under it")
        assertRowsDoNotOverlap([
            [pane.bundledLabel, pane.bundledPathLabel],
            [otherBlock, pane.otherPathLabel],
            [pane.installButton],
            [pane.installHint],
        ], in: content)
        for card in pane.cards { assertCard(card, holds: card.rows, in: content) }
    }

    func testCommandLinePaneLayoutInstalled() {
        let pane = CommandLineSettingsViewController()
        let content = pane.view
        content.setFrameSize(NSSize(width: 760, height: 260))
        pane.bundledLabel.stringValue = "tap 2.1.0"
        pane.bundledPathLabel.stringValue = "Tap.app/Contents/Resources/tap"
        pane.otherLabel.stringValue = "No other tap is on your PATH."
        pane.installButton.title = "Installed"
        pane.installButton.isEnabled = false
        pane.installHint.stringValue = "Terminal runs the bundled tap. To remove it, delete ~/.local/bin/tap."
        content.layoutSubtreeIfNeeded()
        assertRowsDoNotOverlap([
            [pane.bundledLabel, pane.bundledPathLabel],
            [pane.otherLabel],
            [pane.installButton],
            [pane.installHint],
        ], in: content)
        for card in pane.cards { assertCard(card, holds: card.rows, in: content) }
    }

    func testCommandLinePaneLayoutNotFirstOnPath() throws {
        let pane = CommandLineSettingsViewController()
        let content = pane.view
        content.setFrameSize(NSSize(width: 760, height: 260))
        pane.bundledLabel.stringValue = "tap 2.1.0"
        pane.bundledPathLabel.stringValue = "Tap.app/Contents/Resources/tap"
        pane.otherLabel.stringValue = "tap 2.0.0"
        pane.otherNoteLabel.stringValue = "Installed by Homebrew. Tap never replaces or deletes it."
        pane.otherPathLabel.stringValue = "/opt/homebrew/bin/tap"
        pane.installButton.title = "Install in ~/.local/bin…"
        pane.installButton.isEnabled = false
        pane.installHint.stringValue = "/opt/homebrew/bin comes before ~/.local/bin on your PATH, so Terminal would still run tap 2.0.0."
        content.layoutSubtreeIfNeeded()
        let otherBlock = try XCTUnwrap(pane.otherLabel.superview, "the name with its note under it")
        assertRowsDoNotOverlap([
            [pane.bundledLabel, pane.bundledPathLabel],
            [otherBlock, pane.otherPathLabel],
            [pane.installButton],
            [pane.installHint],
        ], in: content)
        for card in pane.cards { assertCard(card, holds: card.rows, in: content) }
    }

    func testCommandLinePaneLayoutNotOnPath() {
        let pane = CommandLineSettingsViewController()
        let content = pane.view
        content.setFrameSize(NSSize(width: 760, height: 260))
        pane.bundledLabel.stringValue = "tap 2.1.0"
        pane.bundledPathLabel.stringValue = "Tap.app/Contents/Resources/tap"
        pane.otherLabel.stringValue = "No other tap is on your PATH."
        pane.installButton.title = "Install in ~/.local/bin…"
        pane.installButton.isEnabled = false
        pane.installHint.stringValue = "~/.local/bin is not on your PATH: add it in your shell's profile first."
        content.layoutSubtreeIfNeeded()
        assertRowsDoNotOverlap([
            [pane.bundledLabel, pane.bundledPathLabel],
            [pane.otherLabel],
            [pane.installButton],
            [pane.installHint],
        ], in: content)
        for card in pane.cards { assertCard(card, holds: card.rows, in: content) }
    }

    // MARK: InstallConfirm sheet

    func testInstallConfirmSheetLayoutNoOtherTap() {
        let installer = CommandLineInstaller(linkDirectory: URL(fileURLWithPath: "/tmp/local-bin"), bundledTap: URL(fileURLWithPath: "/Applications/Tap.app/Contents/Resources/tap"))
        let sheet = InstallConfirmSheet(installer: installer, bundledVersion: "2.1.0", otherTap: nil)
        guard let content = sheet.contentView else { return XCTFail("no content view") }
        content.layoutSubtreeIfNeeded()
        assertRowsDoNotOverlap([
            [sheet.titleLabel],
            [sheet.bodyLabel],
            [sheet.pathLabel],
            [sheet.subLabel],
            [sheet.declineButton, sheet.acceptButton],
        ], in: content)
    }

    func testInstallConfirmSheetLayoutWithOtherTap() throws {
        let installer = CommandLineInstaller(linkDirectory: URL(fileURLWithPath: "/tmp/local-bin"), bundledTap: URL(fileURLWithPath: "/Applications/Tap.app/Contents/Resources/tap"))
        let sheet = InstallConfirmSheet(installer: installer, bundledVersion: "2.1.0", otherTap: (path: "/opt/homebrew/bin/tap", version: "2.0.0"))
        guard let content = sheet.contentView else { return XCTFail("no content view") }
        content.layoutSubtreeIfNeeded()
        let otherBlock = try XCTUnwrap(sheet.otherTapLabel.superview, "the name with its note under it")
        assertRowsDoNotOverlap([
            [sheet.titleLabel],
            [sheet.bodyLabel],
            [sheet.pathLabel],
            [otherBlock, sheet.otherTapPath],
            [sheet.declineButton, sheet.acceptButton],
        ], in: content)
        assertCard(sheet.otherTapCard, holds: [sheet.otherTapLabel, sheet.otherTapNote, sheet.otherTapPath], in: content)
    }
}
