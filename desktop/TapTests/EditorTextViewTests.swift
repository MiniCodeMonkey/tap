import XCTest
@testable import Tap

final class EditorTextViewTests: HostedTestCase {
    // Lines: 1 "---", 2 "title: Deck", 3 "---", 4 "", 5 "# Deck One", 6 "", 7 "<!--", 8 "notes:",
    // 9 "Say hello.", 10 "-->", 11 "", 12 "---", 13 "", 14 "# Two".
    let deck = "---\ntitle: Deck\n---\n\n# Deck One\n\n<!--\nnotes:\nSay hello.\n-->\n\n---\n\n# Two\n"
    let slides = SlideList(slides: [Slide(number: 1, startLine: 5, endLine: 10, title: "Deck One"),
                                    Slide(number: 2, startLine: 14, endLine: 14, title: "Two")], errors: [])
    var window: NSWindow!

    func makeEditor(_ text: String? = nil, list: SlideList? = nil) -> EditorTextView {
        let editor = EditorTextView.make()
        let scrollView = NSScrollView(frame: NSRect(x: 0, y: 0, width: 700, height: 800))
        scrollView.documentView = editor
        editor.frame = NSRect(x: 0, y: 0, width: 700, height: 800)
        window = NSWindow(contentRect: NSRect(x: 0, y: 0, width: 700, height: 800), styleMask: [.titled], backing: .buffered, defer: false)
        window.isReleasedWhenClosed = false
        window.contentView = scrollView
        editor.load(text: text ?? deck)
        let generation = editor.beginSend()
        editor.apply(list ?? slides, sentText: text ?? deck, sentGeneration: generation)
        return editor
    }

    override func tearDown() async throws {
        window?.close()
    }

    var frontmatter: String { "---\ntitle: Deck\n---\n\n" }

    func testTheFrontmatterIsHiddenAndTheCaretStaysOutOfIt() {
        let editor = makeEditor()
        XCTAssertEqual(editor.hiddenLength, (frontmatter as NSString).length)
        editor.setSelectedRange(NSRange(location: editor.hiddenLength, length: 0))
        for _ in 0..<3 { editor.moveUp(nil) }
        XCTAssertGreaterThanOrEqual(editor.selectedRange().location, editor.hiddenLength)
        editor.insertText("Z", replacementRange: NSRange(location: NSNotFound, length: 0))
        XCTAssertTrue(editor.string.hasPrefix(frontmatter + "Z# Deck One"))
    }

    func testSelectAllStaysOutOfTheFrontmatter() {
        let editor = makeEditor()
        editor.selectAll(nil)
        XCTAssertEqual(editor.selectedRange(), NSRange(location: editor.hiddenLength, length: (deck as NSString).length - editor.hiddenLength))
    }

    func testBackspaceAtSlideOneLeavesTheFrontmatterAlone() {
        let editor = makeEditor()
        editor.setSelectedRange(NSRange(location: editor.hiddenLength, length: 0))
        editor.deleteBackward(nil)
        XCTAssertEqual(editor.string, deck)
    }

    func testReplaceAllChangesOnlyVisibleText() {
        let editor = makeEditor()
        let text = deck as NSString
        let hiddenMatch = text.range(of: "Deck")
        let visibleMatch = text.range(of: "Deck", range: NSRange(location: editor.hiddenLength, length: text.length - editor.hiddenLength))
        let allowed = editor.shouldChangeText(inRanges: [NSValue(range: hiddenMatch), NSValue(range: visibleMatch)], replacementStrings: ["Talk", "Talk"])
        XCTAssertFalse(allowed, "the editor applies the visible replacements itself")
        XCTAssertTrue(editor.string.hasPrefix("---\ntitle: Deck\n"))
        XCTAssertTrue(editor.string.contains("# Talk One"))
    }

    func testAProgrammaticEditOfTheFrontmatterIsOneUndoStep() throws {
        let editor = makeEditor()
        window.makeFirstResponder(editor)
        let range = (deck as NSString).range(of: "Deck")
        editor.replaceText(in: range, with: "Talk", actionName: "Load Disk Version")
        XCTAssertTrue(editor.string.hasPrefix("---\ntitle: Talk\n"))
        let undoManager = try XCTUnwrap(editor.undoManager)
        XCTAssertEqual(undoManager.undoActionName, "Load Disk Version")
        undoManager.undo()
        XCTAssertEqual(editor.string, deck)
    }

    func testSpeakerNotes() throws {
        let editor = makeEditor()
        let notes = (deck as NSString).range(of: "Say hello.")
        XCTAssertNotNil(editor.boxes[0].range.intersection(notes), "the notes stay in the box as text")
        let attributes = try XCTUnwrap(editor.textStorage?.attributes(at: notes.location, effectiveRange: nil))
        XCTAssertEqual(attributes[.foregroundColor] as? NSColor, EditorPalette.notes)
        XCTAssertGreaterThan(attributes[.obliqueness] as? Double ?? 0, 0, "notes are in italics")
    }

    func testTheEditorNeverFolds() throws {
        let editor = makeEditor()
        let layoutManager = try XCTUnwrap(editor.textLayoutManager)
        let contentManager = try XCTUnwrap(layoutManager.textContentManager)
        var laidOut: [Int: CGFloat] = [:]
        layoutManager.enumerateTextLayoutFragments(from: contentManager.documentRange.location, options: [.ensuresLayout]) { fragment in
            let offset = contentManager.offset(from: contentManager.documentRange.location, to: fragment.rangeInElement.location)
            laidOut[offset] = fragment.layoutFragmentFrame.height
            return true
        }
        let text = editor.string as NSString
        for box in editor.boxes {
            var location = box.range.location
            while location < box.end {
                let paragraph = text.paragraphRange(for: NSRange(location: location, length: 0))
                XCTAssertGreaterThan(laidOut[paragraph.location] ?? 0, 0, "line at \(paragraph.location) is not visible")
                location = NSMaxRange(paragraph)
            }
        }
        XCTAssertTrue(laidOut.keys.allSatisfy { $0 >= editor.hiddenLength }, "only the frontmatter is hidden")
    }

    func testAnErrorMarksTheBoxAndMakesRoomForTheMessage() throws {
        let broken = SlideList(slides: [Slide(number: 1, startLine: 5, endLine: 10, title: "Deck One", errors: [#"Unknown layout "sectoin""#]),
                                        Slide(number: 2, startLine: 14, endLine: 14, title: "Two")], errors: [])
        let editor = makeEditor(list: broken)
        XCTAssertEqual(editor.header(forBoxAt: 0).errors, [#"Unknown layout "sectoin""#])
        let first = try XCTUnwrap(editor.textStorage?.attribute(.paragraphStyle, at: editor.boxes[0].range.location, effectiveRange: nil) as? NSParagraphStyle)
        let second = try XCTUnwrap(editor.textStorage?.attribute(.paragraphStyle, at: editor.boxes[1].range.location, effectiveRange: nil) as? NSParagraphStyle)
        XCTAssertGreaterThan(first.paragraphSpacingBefore, second.paragraphSpacingBefore)
    }

    func testDeckErrorsShowTheFrontmatter() {
        let editor = makeEditor(list: SlideList(slides: slides.slides, errors: ["frontmatter: bad"]))
        XCTAssertEqual(editor.hiddenLength, 0)
        XCTAssertEqual(editor.deckErrors, ["frontmatter: bad"])
    }

    func testMovingTheCursorToASlideSelectsItsBox() {
        let editor = makeEditor()
        var reported: [Int?] = []
        final class Recorder: EditorTextViewDelegate {
            var onSlide: (Int?) -> Void = { _ in }
            func editorTextDidChange(_ editor: EditorTextView) {}
            func editor(_ editor: EditorTextView, currentSlideDidChange index: Int?) { onSlide(index) }
        }
        let recorder = Recorder()
        recorder.onSlide = { reported.append($0) }
        editor.editorDelegate = recorder
        editor.moveCursor(toSlide: 1)
        XCTAssertEqual(editor.currentBoxIndex, 1)
        XCTAssertEqual(reported.last, 1)
    }
}
