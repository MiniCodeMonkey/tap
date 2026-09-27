import AppKit

/// A Regenerate item's image and the slide whose menu listed it: the
/// right-clicked slide, which need not be the caret's.
struct RegenerateTarget: Equatable {
    var slide: Int
    var imagePath: String
}

/// The menu a right-click on a thumbnail or a box header shows. Every item
/// is also in the Slide menu, so it is reachable from the keyboard.
enum SlideContextMenu {
    /// `showsTextShortcuts` is false for the editor's copy of this menu:
    /// with the editor focused, the Command+C and Command+V shown next to
    /// Copy and Paste are the keys that copy and paste the editor's text,
    /// not these slide commands.
    /// `aiImages` are one slide's AI images, one Regenerate item each.
    static func build(for numbers: [Int], target: AnyObject, showsTextShortcuts: Bool = true, aiImages: (slide: Int, images: [AIImageReference])? = nil) -> NSMenu {
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
        add("Copy", #selector(DeckWindowController.copySlides(_:)), key: showsTextShortcuts ? "c" : "")
        add("Paste", #selector(DeckWindowController.pasteSlides(_:)), key: showsTextShortcuts ? "v" : "")
        menu.addItem(.separator())
        add("Insert Image…", #selector(DeckWindowController.insertImage(_:)), key: "i", modifiers: [.command, .shift])
        add("Generate Image…", #selector(DeckWindowController.generateImage(_:)))
        for image in aiImages?.images ?? [] {
            let item = NSMenuItem(title: image.menuTitle(among: aiImages?.images ?? []), action: #selector(DeckWindowController.regenerateImage(_:)), keyEquivalent: "")
            item.target = target
            item.representedObject = RegenerateTarget(slide: aiImages?.slide ?? 0, imagePath: image.imagePath)
            menu.addItem(item)
        }
        menu.addItem(.separator())
        add(several ? "Delete Slides" : "Delete Slide", #selector(DeckWindowController.deleteSlides(_:)), key: "\u{8}")
        return menu
    }
}
