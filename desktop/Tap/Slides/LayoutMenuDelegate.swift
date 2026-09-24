import AppKit

/// Fills "New Slide from Layout" from tap's catalog each time it opens,
/// so the menu never hard-codes a layout name.
@MainActor
final class LayoutMenuDelegate: NSObject, NSMenuDelegate {
    static let shared = LayoutMenuDelegate()

    func menuNeedsUpdate(_ menu: NSMenu) {
        menu.removeAllItems()
        let templates = AppEnvironment.shared.layoutCatalog.templates
        if templates.isEmpty {
            menu.addItem(MainMenu.item("Loading layouts from tap…", action: nil))
            Task { await AppEnvironment.shared.layoutCatalog.load() }
        }
        for template in templates {
            let item = MainMenu.item(LayoutCatalog.displayName(template.name), action: #selector(DeckWindowController.newSlideFromLayout(_:)))
            item.representedObject = template.name
            menu.addItem(item)
        }
        menu.addItem(.separator())
        menu.addItem(MainMenu.item("Show Layout Gallery…", action: #selector(DeckWindowController.showLayoutGallery(_:))))
    }
}
