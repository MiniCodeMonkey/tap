import AppKit
import XCTest

/// Layout checks for a laid-out screen, on alignment rects rather than
/// frames: a bezelled control's frame (a rounded button, a switch, a
/// segmented control) carries a few points of shadow and focus ring
/// around what the person sees, and stack views space controls by their
/// alignment rects, so two frames side by side or one above the other
/// overlap by design while what is drawn does not.
extension XCTestCase {
    /// `view`'s alignment rect in `content`'s coordinates.
    @MainActor
    func alignmentRect(of view: NSView, in content: NSView) -> NSRect {
        let rect = view.alignmentRect(forFrame: view.frame)
        guard let superview = view.superview else { return rect }
        return superview.convert(rect, to: content)
    }

    /// Each row is the views that sit side by side on one line (a buttons
    /// row is one row). Every view has a real size and lies inside
    /// `content`; the views of a row do not overlap each other; no two
    /// rows overlap vertically.
    @MainActor
    func assertRowsDoNotOverlap(_ rows: [[NSView]], in content: NSView, file: StaticString = #filePath, line: UInt = #line) {
        content.layoutSubtreeIfNeeded()
        var rowRects: [NSRect] = []
        for row in rows {
            let rects = row.map { alignmentRect(of: $0, in: content) }
            for (view, rect) in zip(row, rects) {
                XCTAssertGreaterThan(rect.width, 0, "\(view) has a real width", file: file, line: line)
                XCTAssertGreaterThan(rect.height, 0, "\(view) has a real height", file: file, line: line)
                XCTAssertTrue(content.bounds.insetBy(dx: -0.5, dy: -0.5).contains(rect), "\(view) at \(rect) lies inside \(content.bounds)", file: file, line: line)
            }
            for (index, rect) in rects.enumerated() {
                for other in rects[(index + 1)...] {
                    XCTAssertLessThanOrEqual(rect.intersection(other).width, 0.5, "\(rect) and \(other) sit side by side without overlapping", file: file, line: line)
                }
            }
            rowRects.append(rects.dropFirst().reduce(rects.first ?? .zero) { $0.union($1) })
        }
        for (index, rect) in rowRects.enumerated() {
            for other in rowRects[(index + 1)...] {
                XCTAssertLessThanOrEqual(rect.intersection(other).height, 0.5, "the rows at \(rect) and \(other) do not overlap", file: file, line: line)
            }
        }
    }

    /// Every one of `views` lies inside `card`'s bounds less `insets` (a
    /// stack view's edge insets, say), and the card is as tall as the
    /// views it holds.
    @MainActor
    func assertCard(_ card: NSView, holds views: [NSView], insets: NSEdgeInsets = NSEdgeInsets(), in content: NSView, file: StaticString = #filePath, line: UInt = #line) {
        content.layoutSubtreeIfNeeded()
        let cardRect = card.convert(card.bounds, to: content)
        // Whether `content` is flipped decides which edge is the top, so the
        // smaller of the top and bottom insets holds on both.
        let vertical = min(insets.top, insets.bottom)
        let inner = NSRect(x: cardRect.minX + insets.left, y: cardRect.minY + vertical,
                           width: cardRect.width - insets.left - insets.right, height: cardRect.height - 2 * vertical)
        for view in views {
            let rect = alignmentRect(of: view, in: content)
            XCTAssertGreaterThan(rect.width, 0, "\(view) has a real width", file: file, line: line)
            XCTAssertTrue(inner.insetBy(dx: -0.5, dy: -0.5).contains(rect), "\(view) at \(rect) lies inside its card \(cardRect), within its insets", file: file, line: line)
        }
        let rects = views.map { alignmentRect(of: $0, in: content) }
        if let first = rects.first {
            let held = rects.dropFirst().reduce(first) { $0.union($1) }
            XCTAssertGreaterThanOrEqual(cardRect.height + 0.5, held.height, "the card \(cardRect) is as tall as its rows \(held)", file: file, line: line)
        }
    }
}
