import XCTest
@testable import Tap

/// The app bundle ships every theme's thumbnail, rendered at build time, so
/// no grid waits on tap.
@MainActor
final class BundledThemeThumbnailTests: XCTestCase {
    private var bundled: BundledThemeThumbnails {
        BundledThemeThumbnails(resourcesFolder: Bundle.main.resourceURL!)
    }

    func testEveryCatalogThemeHasABundledThumbnail() throws {
        let catalog = try XCTUnwrap(bundled.catalog(), "the bundle has its theme catalog")
        XCTAssertEqual(catalog.themes.count, 21)
        for theme in catalog.themes {
            let url = try XCTUnwrap(bundled.imageURL(forSlug: theme.slug), "no bundled thumbnail for \(theme.slug)")
            XCTAssertNotNil(NSImage(contentsOf: url), "\(theme.slug).png is a readable image")
        }
    }

    func testTheLoaderReturnsThemAtOnceWithoutARender() throws {
        let loader = ThemeImageLoader()
        let catalog = try XCTUnwrap(loader.catalog, "the catalog is there before any tap run")
        for theme in catalog.themes {
            XCTAssertNotNil(loader.image(for: theme.slug), "\(theme.slug) shows at once")
        }
        XCTAssertNotNil(loader.image(for: ThemeGridViewController.defaultSlug))
        loader.loadAll(retryingFailures: true)
        loader.loadImages(for: catalog.themes.map(\.slug))
        XCTAssertFalse(loader.isWorking, "nothing is left to render, so no tap run starts")
    }

    func testAThemeWithNoBundledImageFallsBackToTap() throws {
        let empty = BundledThemeThumbnails(folder: try Fixtures.temporaryFolder())
        let loader = ThemeImageLoader(bundledThumbnails: empty)
        XCTAssertNil(loader.catalog)
        XCTAssertNil(loader.image(for: "base"))
        loader.loadCatalog()
        XCTAssertTrue(loader.isWorking, "the fallback loads the catalog from tap")
        loader.stop()
    }
}
