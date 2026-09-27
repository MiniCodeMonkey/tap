import Foundation

/// The theme catalog and every theme's render, loaded once per app. A
/// stub until Task 6b, which fills it in; here it holds only what
/// `AppEnvironment` needs to compile.
@MainActor
final class ThemeImageLoader {
    private(set) var catalog: ThemeCatalog?

    func loadAll() {}
}
