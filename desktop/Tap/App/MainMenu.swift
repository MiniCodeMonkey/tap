import AppKit

/// The menu bar: Tap, File, Edit, Slide, View, Present, Window, Help. Every
/// toolbar action also has an item here. Items whose feature comes in a
/// later version have no action, so AppKit shows them disabled.
@MainActor
enum MainMenu {
    static func build() -> NSMenu {
        let main = NSMenu(title: "Main Menu")
        let windows = windowMenu()
        let help = helpMenu()
        for menu in [tapMenu(), fileMenu(), editMenu(), slideMenu(), viewMenu(), presentMenu(), windows, help] {
            let item = NSMenuItem(title: menu.title, action: nil, keyEquivalent: "")
            item.submenu = menu
            main.addItem(item)
        }
        NSApp.windowsMenu = windows
        NSApp.helpMenu = help
        return main
    }

    static func item(_ title: String, action: Selector?, key: String = "", modifiers: NSEvent.ModifierFlags = [.command]) -> NSMenuItem {
        let item = NSMenuItem(title: title, action: action, keyEquivalent: key)
        item.keyEquivalentModifierMask = modifiers
        return item
    }

    static func tapMenu() -> NSMenu {
        let menu = NSMenu(title: "Tap")
        menu.addItem(item("About Tap", action: #selector(AppDelegate.showAbout(_:))))
        menu.addItem(.separator())
        let services = item("Services", action: nil)
        services.submenu = NSMenu(title: "Services")
        NSApp.servicesMenu = services.submenu
        menu.addItem(services)
        menu.addItem(.separator())
        menu.addItem(item("Hide Tap", action: #selector(NSApplication.hide(_:)), key: "h"))
        menu.addItem(item("Hide Others", action: #selector(NSApplication.hideOtherApplications(_:)), key: "h", modifiers: [.command, .option]))
        menu.addItem(item("Show All", action: #selector(NSApplication.unhideAllApplications(_:))))
        menu.addItem(.separator())
        menu.addItem(item("Quit Tap", action: #selector(NSApplication.terminate(_:)), key: "q"))
        return menu
    }

    static func fileMenu() -> NSMenu {
        let menu = NSMenu(title: "File")
        // New Deck arrives with the New Deck sheet, which runs tap new.
        menu.addItem(item("New Deck…", action: nil, key: "n"))
        menu.addItem(item("Open…", action: #selector(NSDocumentController.openDocument(_:)), key: "o"))
        let recent = item("Open Recent", action: nil)
        let recentMenu = NSMenu(title: "Open Recent")
        // AppKit fills a menu that holds a Clear Menu item with recent documents.
        recentMenu.addItem(item("Clear Menu", action: #selector(NSDocumentController.clearRecentDocuments(_:))))
        recent.submenu = recentMenu
        menu.addItem(recent)
        menu.addItem(.separator())
        menu.addItem(item("Close", action: #selector(NSWindow.performClose(_:)), key: "w"))
        menu.addItem(item("Save", action: #selector(NSDocument.save(_:)), key: "s"))
        menu.addItem(item("Duplicate", action: #selector(NSDocument.duplicate(_:)), key: "s", modifiers: [.command, .shift]))
        menu.addItem(item("Rename…", action: #selector(NSDocument.rename(_:))))
        menu.addItem(item("Move To…", action: #selector(NSDocument.move(_:))))
        // With autosave in place, AppKit turns this item into the Revert To
        // menu, with Browse All Versions.
        menu.addItem(item("Revert to Saved", action: #selector(NSDocument.revertToSaved(_:))))
        return menu
    }

    static func editMenu() -> NSMenu {
        let menu = NSMenu(title: "Edit")
        menu.addItem(item("Undo", action: Selector(("undo:")), key: "z"))
        menu.addItem(item("Redo", action: Selector(("redo:")), key: "z", modifiers: [.command, .shift]))
        menu.addItem(.separator())
        menu.addItem(item("Cut", action: #selector(NSText.cut(_:)), key: "x"))
        menu.addItem(item("Copy", action: #selector(NSText.copy(_:)), key: "c"))
        menu.addItem(item("Paste", action: #selector(NSText.paste(_:)), key: "v"))
        menu.addItem(item("Select All", action: #selector(NSText.selectAll(_:)), key: "a"))
        menu.addItem(.separator())
        let find = item("Find", action: nil)
        let findMenu = NSMenu(title: "Find")
        let findItems: [(String, String, NSEvent.ModifierFlags, NSTextFinder.Action)] = [
            ("Find…", "f", [.command], .showFindInterface),
            ("Find and Replace…", "f", [.command, .option], .showReplaceInterface),
            ("Find Next", "g", [.command], .nextMatch),
            ("Find Previous", "g", [.command, .shift], .previousMatch),
            ("Use Selection for Find", "e", [.command], .setSearchString),
        ]
        for (title, key, modifiers, action) in findItems {
            let findItem = item(title, action: #selector(NSTextView.performTextFinderAction(_:)), key: key, modifiers: modifiers)
            findItem.tag = action.rawValue
            findMenu.addItem(findItem)
        }
        find.submenu = findMenu
        menu.addItem(find)
        return menu
    }

    static func slideMenu() -> NSMenu {
        let menu = NSMenu(title: "Slide")
        menu.addItem(item("Go to Slide…", action: #selector(DeckWindowController.goToSlide(_:)), key: "o", modifiers: [.command, .shift]))
        return menu
    }

    static func viewMenu() -> NSMenu {
        let menu = NSMenu(title: "View")
        menu.addItem(item("Unpin Slide Panel", action: #selector(DeckWindowController.toggleSlidePanel(_:)), key: "s", modifiers: [.command, .control]))
        menu.addItem(item("Hide Preview", action: #selector(DeckWindowController.togglePreview(_:)), key: "0", modifiers: [.command, .option]))
        menu.addItem(item("Pin Preview", action: #selector(DeckWindowController.togglePreviewPin(_:)), key: "p", modifiers: [.command, .shift]))
        menu.addItem(item("Preview in Window", action: #selector(DeckWindowController.showPreviewInWindow(_:))))
        menu.addItem(.separator())
        menu.addItem(item("Enter Full Screen", action: #selector(NSWindow.toggleFullScreen(_:)), key: "f", modifiers: [.command, .control]))
        return menu
    }

    static func presentMenu() -> NSMenu {
        let menu = NSMenu(title: "Present")
        // Presenting arrives with tap present --app.
        menu.addItem(item("Play", action: nil, key: "p", modifiers: [.command, .option]))
        menu.addItem(item("Rehearse", action: nil, key: "p", modifiers: [.command, .option, .shift]))
        return menu
    }

    static func windowMenu() -> NSMenu {
        let menu = NSMenu(title: "Window")
        menu.addItem(item("Minimize", action: #selector(NSWindow.performMiniaturize(_:)), key: "m"))
        menu.addItem(item("Zoom", action: #selector(NSWindow.performZoom(_:))))
        menu.addItem(.separator())
        menu.addItem(item("Tap Log", action: #selector(AppDelegate.showTapLog(_:)), key: "l", modifiers: [.command, .option]))
        menu.addItem(.separator())
        menu.addItem(item("Bring All to Front", action: #selector(NSApplication.arrangeInFront(_:))))
        return menu
    }

    static func helpMenu() -> NSMenu {
        let menu = NSMenu(title: "Help")
        menu.addItem(item("Tap Help", action: #selector(AppDelegate.showHelp(_:)), key: "?"))
        return menu
    }
}
