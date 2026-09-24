import AppKit

/// The menu a right-click on a thumbnail or a box header shows. Every item
/// is also in the Slide menu, so it is reachable from the keyboard.
enum SlideContextMenu {
    static func build(for numbers: [Int], target: AnyObject) -> NSMenu {
        let menu = NSMenu(title: "Slide")
        func add(_ title: String, _ action: Selector?, key: String = "", modifiers: NSEvent.ModifierFlags = [.command]) {
            let item = NSMenuItem(title: title, action: action, keyEquivalent: key)
            item.keyEquivalentModifierMask = modifiers
            item.target = action == nil ? nil : target
            menu.addItem(item)
        }
        let several = numbers.count > 1
        add("New Slide After", #selector(DeckWindowController.newSlideAfter(_:)))
        add("Duplicate", #selector(DeckWindowController.duplicateSlides(_:)), key: "d")
        add("Skip Slide", #selector(DeckWindowController.toggleSkipSlides(_:)))
        menu.addItem(.separator())
        add("Move to Top", #selector(DeckWindowController.moveSlidesToTop(_:)))
        add("Move to Bottom", #selector(DeckWindowController.moveSlidesToBottom(_:)))
        menu.addItem(.separator())
        add("Copy", #selector(DeckWindowController.copySlides(_:)), key: "c")
        add("Paste", #selector(DeckWindowController.pasteSlides(_:)), key: "v")
        menu.addItem(.separator())
        add("Generate Image…", nil)
        add("Insert Image…", nil, key: "i", modifiers: [.command, .shift])
        menu.addItem(.separator())
        add(several ? "Delete Slides" : "Delete Slide", #selector(DeckWindowController.deleteSlides(_:)), key: "\u{8}")
        return menu
    }
}
