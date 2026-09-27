import XCTest
@testable import TapDesktopCore

final class DeckReferencesTests: XCTestCase {
    let deck = """
    # One

    <!-- ai-prompt: a fox at dusk -->
    ![](images/generated-1a2b3c4d.png)

    ---

    # Two

    <!--   ai-prompt:   a whale   -->
      ![](./images/generated-ffffffff.png)

    ![diagram](images/diagram.png)
    """

    func testFindsEveryAIImagePairAsTapDoes() {
        let found = AIImageReference.find(in: deck)
        XCTAssertEqual(found.map(\.prompt), ["a fox at dusk", "a whale"])
        let recorded = AIImageReference.find(in: "<!-- ai-prompt: a fox | not a choice | aspect: 16:9 | match-theme -->\n![](a.png)")
        XCTAssertEqual(recorded.map(\.prompt), ["a fox | not a choice"], "tap's recorded choices come off; the person's own separator stays")
        XCTAssertEqual(AIImageReference.words(in: "compare the two | aspect: wide"), "compare the two | aspect: wide", "an aspect tap does not accept is the person's words")
        XCTAssertEqual(AIImageReference.words(in: "a fox | aspect: 2:1"), "a fox | aspect: 2:1")
        XCTAssertEqual(AIImageReference.words(in: "a fox | match-theme please"), "a fox | match-theme please")
        XCTAssertEqual(AIImageReference.words(in: "a fox | match-theme | aspect: 1:1"), "a fox", "choices in either order")
        XCTAssertEqual(found.map(\.imagePath), ["images/generated-1a2b3c4d.png", "./images/generated-ffffffff.png"])
        XCTAssertEqual((deck as NSString).substring(with: found[0].range), "<!-- ai-prompt: a fox at dusk -->\n![](images/generated-1a2b3c4d.png)")
        XCTAssertEqual(AIImageReference.find(in: "![](images/plain.png)\n"), [], "a plain image is not an AI image")
        XCTAssertEqual(AIImageReference.find(in: "<!-- ai-prompt: x -->\n\n![](a.png)"), [], "the link must be on the next line, as tap requires")
    }

    func testFindsThePairsInsideASlidesRange() {
        let text = deck as NSString
        let slideTwo = NSRange(location: text.range(of: "# Two").location, length: text.length - text.range(of: "# Two").location)
        XCTAssertEqual(AIImageReference.find(in: deck, slideRange: slideTwo).map(\.prompt), ["a whale"])
    }

    func testFindsAComponentPathUnderTheCaret() {
        let line = "layout: ./slides/RollingDeploy.jsx"
        XCTAssertEqual(ComponentLink.find(in: line, at: 12), "./slides/RollingDeploy.jsx")
        XCTAssertEqual(ComponentLink.find(in: line, at: 33), "./slides/RollingDeploy.jsx", "the last character counts")
        XCTAssertNil(ComponentLink.find(in: line, at: 3), "a click on the key is not on the path")
        XCTAssertEqual(ComponentLink.find(in: "```component ./components/LatencyDrop.tsx", at: 20), "./components/LatencyDrop.tsx")
        XCTAssertEqual(ComponentLink.find(in: "<!-- layout: slides/Chart.jsx -->", at: 16), "slides/Chart.jsx", "without ./ too")
        XCTAssertNil(ComponentLink.find(in: "See ./notes/plan.md", at: 6), "only component files")
        XCTAssertNil(ComponentLink.find(in: "layout: ./slides/Notes.md", at: 12), "the extension alone decides inside slides/")
        XCTAssertNil(ComponentLink.find(in: "", at: 0))
    }
}
