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

    // Two slides with a body line each, so an edit can join them and still
    // leave the second one text of its own.
    let bodiedDeck = "---\ntitle: Deck\n---\n\n# One\n\nAlpha\n\n---\n\n# Two\n\nBeta\n"
    var bodiedSlides: SlideList {
        SlideList(slides: [Slide(number: 1, startLine: 5, endLine: 7, title: "One"),
                           Slide(number: 2, startLine: 11, endLine: 13, title: "Two")], errors: [])
    }

    final class Recorder: EditorTextViewDelegate {
        var onSlide: (Int?) -> Void = { _ in }
        func editorTextDidChange(_ editor: EditorTextView) {}
        func editor(_ editor: EditorTextView, currentSlideDidChange index: Int?) { onSlide(index) }
    }

    /// The defect this covers: a person clicks a slide and the cursor ends
    /// up in a neighbouring one, so the preview shows a slide nobody asked
    /// for. It needs the boxes to move around the move, which the test
    /// forces rather than waits for: a PUT goes out, the person deletes
    /// across a slide boundary while it is in flight, which pulls both
    /// slides onto the text that is left, then clicks the second slide, and
    /// tap's answer is handed to the editor in the same turn.
    func testAMoveLandsInTheSlideAskedForWhenARenderArrivesWithIt() {
        let editor = makeEditor(bodiedDeck, list: bodiedSlides)
        let recorder = Recorder()
        var reported: [Int?] = []
        recorder.onSlide = { reported.append($0) }
        editor.editorDelegate = recorder

        // The text as it stands goes to tap.
        let generation = editor.beginSend()
        // The person selects from inside slide 1's body into slide 2's
        // heading and deletes.
        let text = bodiedDeck as NSString
        let from = text.range(of: "Alpha").location + 2
        let to = text.range(of: "# Two").location + 3
        editor.setSelectedRange(NSRange(location: from, length: to - from))
        editor.deleteBackward(nil)

        // The person clicks slide 2.
        editor.moveCursor(toSlide: 1)
        XCTAssertEqual(editor.currentBoxIndex, 1, "the cursor is in the slide the person clicked")
        XCTAssertEqual(editor.tracker.currentBoxIndex(caret: editor.selectedRange().location), 1,
                       "the caret reads back as that slide, not a neighbour: "
                       + "caret=\(editor.selectedRange()) boxes=\(editor.boxes.map(\.range))")
        XCTAssertEqual(reported.last, 1, "and that is the slide the preview is told about")

        // tap's answer for the text sent before the deletion arrives now.
        editor.apply(bodiedSlides, sentText: bodiedDeck, sentGeneration: generation)
        XCTAssertEqual(editor.currentBoxIndex, 1, "the render does not move the cursor to another slide")
        XCTAssertEqual(reported.last, 1)
    }

    /// An answer to a send begun before the whole text was replaced names
    /// lines of a document that is gone, so it is refused rather than laid
    /// over the text that replaced it.
    func testARenderForTextThatWasReplacedIsRefused() {
        let editor = makeEditor()
        let generation = editor.beginSend()
        editor.load(text: "# Only" + "\n")
        XCTAssertFalse(editor.apply(slides, sentText: deck, sentGeneration: generation))
        XCTAssertTrue(editor.boxes.isEmpty)
    }

    func testMovingTheCursorToASlideSelectsItsBox() {
        let editor = makeEditor()
        var reported: [Int?] = []
        let recorder = Recorder()
        recorder.onSlide = { reported.append($0) }
        editor.editorDelegate = recorder
        editor.moveCursor(toSlide: 1)
        XCTAssertEqual(editor.currentBoxIndex, 1)
        XCTAssertEqual(reported.last, 1)
    }
}
