import XCTest
@testable import TapDesktopCore

final class BundledTourThumbnailsTests: XCTestCase {
    private func makeFolder(source: String, images: Int) throws -> BundledTourThumbnails {
        let folder = FileManager.default.temporaryDirectory.appendingPathComponent("tour-\(UUID().uuidString)", isDirectory: true)
        try FileManager.default.createDirectory(at: folder, withIntermediateDirectories: true)
        addTeardownBlock { try? FileManager.default.removeItem(at: folder) }
        try source.write(to: folder.appendingPathComponent(BundledTourThumbnails.sourceFileName), atomically: true, encoding: .utf8)
        for number in 1...max(1, images) where number <= images {
            try Data([1]).write(to: folder.appendingPathComponent("slide-\(number).png"))
        }
        return BundledTourThumbnails(folder: folder)
    }

    func testTheUneditedTourGetsEveryImage() throws {
        let tour = try makeFolder(source: "# One\n---\n# Two\n", images: 2)
        let urls = tour.imageURLs(forText: "# One\n---\n# Two", slideCount: 2)
        XCTAssertEqual(urls.keys.sorted(), [1, 2])
        XCTAssertEqual(urls[2]?.lastPathComponent, "slide-2.png")
    }

    func testAnyEditMeansNoImages() throws {
        let tour = try makeFolder(source: "# One\n---\n# Two\n", images: 2)
        XCTAssertEqual(tour.imageURLs(forText: "# One!\n---\n# Two\n", slideCount: 2), [:])
    }

    func testADifferentSlideCountMeansNoImages() throws {
        let tour = try makeFolder(source: "# One\n", images: 2)
        XCTAssertEqual(tour.imageURLs(forText: "# One\n", slideCount: 1), [:], "more images than slides")
        XCTAssertEqual(tour.imageURLs(forText: "# One\n", slideCount: 3), [:], "a slide with no image")
        XCTAssertEqual(tour.imageURLs(forText: "# One\n", slideCount: 0), [:])
    }

    func testAMissingFolderMeansNoImages() {
        let tour = BundledTourThumbnails(folder: URL(fileURLWithPath: "/nonexistent/\(UUID().uuidString)"))
        XCTAssertEqual(tour.imageURLs(forText: "x", slideCount: 1), [:])
    }
}
