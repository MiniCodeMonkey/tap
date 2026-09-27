import AppKit

/// Tap > Settings: one window, four panes as tabs in the toolbar style.
final class SettingsWindowController: NSWindowController {
    static let shared = SettingsWindowController()

    enum Pane: Int {
        case general = 0, liveCode, imageGeneration, commandLine
    }

    let tabViewController = NSTabViewController()
    let general = GeneralSettingsViewController()
    let liveCode = LiveCodeSettingsViewController()
    let imageGeneration = ImageGenerationSettingsViewController()
    let commandLine = CommandLineSettingsViewController()

    private init() {
        let window = NSWindow(contentRect: NSRect(x: 0, y: 0, width: 760, height: 520), styleMask: [.titled, .closable], backing: .buffered, defer: false)
        window.isReleasedWhenClosed = false
        window.center()
        super.init(window: window)
        tabViewController.tabStyle = .toolbar
        for (pane, symbol) in [(general, "gearshape"), (liveCode, "checkmark.shield"), (imageGeneration, "sparkles"), (commandLine, "terminal")] as [(NSViewController, String)] {
            let item = NSTabViewItem(viewController: pane)
            item.label = pane.title ?? ""
            item.image = NSImage(systemSymbolName: symbol, accessibilityDescription: pane.title)
            tabViewController.addTabViewItem(item)
        }
        window.contentViewController = tabViewController
        window.setAccessibilityIdentifier("settings-window")
    }

    required init?(coder: NSCoder) { fatalError("init(coder:) is not used") }

    func show(pane: Pane) {
        tabViewController.selectedTabViewItemIndex = pane.rawValue
        window?.title = tabViewController.tabViewItems[pane.rawValue].label
        showWindow(nil)
    }
}

/// Image Generation is Task 13's; this stub only carries the pane's title
/// so the window's tab bar and Task 12's tests compile and pass.
final class ImageGenerationSettingsViewController: NSViewController {
    override init(nibName: NSNib.Name?, bundle: Bundle?) {
        super.init(nibName: nil, bundle: nil)
        title = "Image Generation"
    }

    required init?(coder: NSCoder) { fatalError("init(coder:) is not used") }

    override func loadView() { view = NSView() }
}

/// Command Line is Task 13's; this stub only carries the pane's title so
/// the window's tab bar and Task 12's tests compile and pass.
final class CommandLineSettingsViewController: NSViewController {
    override init(nibName: NSNib.Name?, bundle: Bundle?) {
        super.init(nibName: nil, bundle: nil)
        title = "Command Line"
    }

    required init?(coder: NSCoder) { fatalError("init(coder:) is not used") }

    override func loadView() { view = NSView() }
}
