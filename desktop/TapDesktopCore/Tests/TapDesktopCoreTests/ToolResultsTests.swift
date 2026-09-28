import XCTest
@testable import TapDesktopCore

/// Each shape is the one internal/cli prints (new.go, theme_set.go,
/// image.go, component.go, export_pdf.go, export_images.go, build.go,
/// serve.go, approval_command.go), copied from a real run.
final class ToolResultsTests: XCTestCase {
    func decode<Result: Decodable>(_ type: Result.Type, _ json: String) throws -> Result {
        try XCTUnwrap(ToolOutcome.decode(Data(json.utf8))).result(type)
    }

    func testDecodesEveryCommandsResult() throws {
        XCTAssertEqual(try decode(NewDeckResult.self, #"{"ok":true,"deck":"/t/my-talk/my-talk.md","folder":"/t/my-talk"}"#),
                       NewDeckResult(deck: "/t/my-talk/my-talk.md", folder: "/t/my-talk"))
        XCTAssertEqual(try decode(ThemeSetResult.self, #"{"ok":true,"deck":"/t/talk.md","theme":"terminal"}"#), ThemeSetResult(deck: "/t/talk.md", theme: "terminal"))
        XCTAssertEqual(try decode(AddedImageResult.self, #"{"ok":true,"deck":"/t/talk.md","image":"images/diagram-2.png","markdown":"![diagram](images/diagram-2.png)"}"#),
                       AddedImageResult(deck: "/t/talk.md", image: "images/diagram-2.png", markdown: "![diagram](images/diagram-2.png)"))
        XCTAssertEqual(try decode(GeneratedImageResult.self, #"{"ok":true,"deck":"/t/talk.md","slide":3,"image":"images/generated-1a2b3c4d.png","prompt":"a fox","markdown":"<!-- ai-prompt: a fox -->\n![](images/generated-1a2b3c4d.png)","replaced":"images/generated-00000000.png"}"#),
                       GeneratedImageResult(deck: "/t/talk.md", slide: 3, image: "images/generated-1a2b3c4d.png", prompt: "a fox", markdown: "<!-- ai-prompt: a fox -->\n![](images/generated-1a2b3c4d.png)", replaced: "images/generated-00000000.png"))
        XCTAssertNil(try decode(GeneratedImageResult.self, #"{"ok":true,"deck":"/t/talk.md","slide":3,"image":"i.png","prompt":"p","markdown":"m"}"#).replaced, "generate has no replaced")
        XCTAssertEqual(try decode(ComponentScaffold.self, #"{"ok":true,"files":["/t/slides/Counter.jsx"],"snippet":"<!--\nlayout: ./slides/Counter.jsx\n-->\n\n# Title\n"}"#),
                       ComponentScaffold(files: ["/t/slides/Counter.jsx"], snippet: "<!--\nlayout: ./slides/Counter.jsx\n-->\n\n# Title\n"))
        let pdf = try decode(PDFExportResult.self, #"{"phase":"done","ok":true,"output":"/t/talk.pdf","pages":14,"bytes":120000,"brokenSlides":[{"slide":2,"message":"boom"}]}"#)
        XCTAssertEqual(pdf, PDFExportResult(output: "/t/talk.pdf", pages: 14, bytes: 120_000, brokenSlides: [BrokenSlide(slide: 2, message: "boom")]))
        XCTAssertEqual(try decode(ImagesExportResult.self, #"{"phase":"done","ok":true,"files":["/t/out/slide-001.png","/t/out/slide-003.png"]}"#).files.count, 2)
        XCTAssertEqual(try decode(BuildResult.self, #"{"phase":"done","ok":true,"output":"/t/dist","files":38,"bytes":4300000}"#), BuildResult(output: "/t/dist", files: 38, bytes: 4_300_000))
        XCTAssertEqual(try decode(ServeReady.self, #"{"ok":true,"dir":"/t/dist","port":54321,"url":"http://localhost:54321"}"#), ServeReady(dir: "/t/dist", port: 54321, url: "http://localhost:54321"))
    }

    func testDecodesTheApprovalList() throws {
        let list = try decode(ApprovalList.self, #"{"ok":true,"approvals":[{"deck":"/Users/me/talks/3am/conference-talk.md","drivers":["sqlite","shell"],"approvedAt":"2026-09-25T19:32:00.123456Z"},{"deck":"/Users/me/talks/k8s/k8s-workshop.md","drivers":["shell","kubectl"],"commands":{"kubectl":["kubectl","--token","${KUBE_TOKEN}"]},"approvedAt":"2026-08-30T10:00:00Z"}]}"#)
        XCTAssertEqual(list.approvals.count, 2)
        let first = list.approvals[0]
        XCTAssertEqual(first.deckName, "conference-talk.md")
        XCTAssertEqual(first.folderPath, "/Users/me/talks/3am")
        XCTAssertEqual(first.driverSummary, "sqlite, shell")
        XCTAssertNotNil(first.approvedAtDate, "fractional seconds parse")
        let second = list.approvals[1]
        XCTAssertEqual(second.driverSummary, "shell, kubectl (custom)")
        XCTAssertEqual(second.commands?["kubectl"], ["kubectl", "--token", "${KUBE_TOKEN}"], "shown as tap masked it, never expanded here")
        XCTAssertNotNil(second.approvedAtDate)
        XCTAssertEqual(try decode(ApprovalList.self, #"{"ok":true,"approvals":[]}"#).approvals, [])
    }

    func testAFailureIsAnError() {
        let outcome = ToolOutcome.decode(Data(#"{"ok":false,"error":{"code":"no_api_key","message":"cannot start image generation: GEMINI_API_KEY is not set"}}"#.utf8))
        XCTAssertThrowsError(try outcome?.result(GeneratedImageResult.self)) { error in
            XCTAssertEqual(error as? ToolError, .failed(code: "no_api_key", message: "cannot start image generation: GEMINI_API_KEY is not set"))
        }
    }
}
