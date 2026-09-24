import AppKit
import XCTest
@testable import TapDesktopCore

final class FlatImageCheckTests: XCTestCase {
    func image(width: Int, height: Int, draw: (NSRect) -> Void) -> NSImage {
        let image = NSImage(size: NSSize(width: width, height: height))
        image.lockFocus()
        draw(NSRect(x: 0, y: 0, width: width, height: height))
        image.unlockFocus()
        return image
    }

    func testAOneColourImageIsFlat() {
        let white = image(width: 320, height: 180) { NSColor.white.setFill(); $0.fill() }
        XCTAssertTrue(FlatImageCheck.isFlat(white))
        let black = image(width: 320, height: 180) { NSColor.black.setFill(); $0.fill() }
        XCTAssertTrue(FlatImageCheck.isFlat(black))
    }

    func testAnImageWithContentIsNotFlat() {
        let slide = image(width: 320, height: 180) { rect in
            NSColor(white: 0.06, alpha: 1).setFill(); rect.fill()
            NSColor.white.setFill(); NSRect(x: 40, y: 80, width: 200, height: 30).fill()
        }
        XCTAssertFalse(FlatImageCheck.isFlat(slide))
    }

    func testAThinStrokeBetweenSamplePointsIsStillContent() {
        // One white row on black, at a height a 24 by 14 sample grid never reads (row 7 of 180).
        let sparse = image(width: 320, height: 180) { rect in
            NSColor.black.setFill(); rect.fill()
            NSColor.white.setFill(); NSRect(x: 0, y: 7, width: 320, height: 1).fill()
        }
        XCTAssertFalse(FlatImageCheck.isFlat(sparse), "every pixel counts, through the downsample")
        // A single blue channel difference is content too.
        let blue = image(width: 320, height: 180) { rect in
            NSColor(red: 0.5, green: 0.5, blue: 0.5, alpha: 1).setFill(); rect.fill()
            NSColor(red: 0.5, green: 0.5, blue: 0.9, alpha: 1).setFill(); NSRect(x: 0, y: 0, width: 160, height: 180).fill()
        }
        XCTAssertFalse(FlatImageCheck.isFlat(blue))
    }

    func testAnEmptyImageIsFlat() {
        XCTAssertTrue(FlatImageCheck.isFlat(NSImage(size: .zero)))
    }
}
