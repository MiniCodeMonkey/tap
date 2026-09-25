import XCTest
@testable import TapDesktopCore

final class TapProtocolTests: XCTestCase {
    func testDecodesTheReadyLine() {
        let event = TapEvent.decode(line: #"{"type":"ready","port":49152,"token":"abc","launch":"def","presenter":"ghi"}"#)
        XCTAssertEqual(event, .ready(TapReady(port: 49152, token: "abc", launch: "def", presenter: "ghi")))
        // A tap older than the presenter secret still reaches running, and
        // TapClient refuses to authorize on the empty secret instead.
        let withoutPresenter = TapEvent.decode(line: #"{"type":"ready","port":49152,"token":"abc","launch":"def"}"#)
        XCTAssertEqual(withoutPresenter, .ready(TapReady(port: 49152, token: "abc", launch: "def", presenter: "")))
    }

    func testDecodesFileChangedForTheDeckAndForAComponent() {
        XCTAssertEqual(TapEvent.decode(line: #"{"type":"file-changed","path":"/talks/talk.md"}"#),
                       .fileChanged(path: "/talks/talk.md", slideList: nil))
        let component = TapEvent.decode(line: #"{"type":"file-changed","path":"/talks/slides/Counter.jsx","slides":[],"errors":["bad"]}"#)
        XCTAssertEqual(component, .fileChanged(path: "/talks/slides/Counter.jsx", slideList: SlideList(slides: [], errors: ["bad"])))
    }

    func testDecodesQuestionsErrorsAndOtherEvents() {
        XCTAssertEqual(TapEvent.decode(line: #"{"type":"question","id":"q1","kind":"approval","payload":{"deck":"/a.md"}}"#),
                       .question(id: "q1", kind: "approval", payload: QuestionPayload(deck: "/a.md")))
        XCTAssertEqual(TapEvent.decode(line: #"{"type":"question","id":"q2","kind":"record-consent","payload":{"settingsPath":"/c/tap/settings.yaml"}}"#),
                       .question(id: "q2", kind: "record-consent", payload: QuestionPayload(settingsPath: "/c/tap/settings.yaml")))
        XCTAssertEqual(TapEvent.decode(line: #"{"type":"question","id":"q3","kind":"keep-recording","payload":{"directory":"/r/talk-1","segments":2}}"#),
                       .question(id: "q3", kind: "keep-recording", payload: QuestionPayload(directory: "/r/talk-1", segments: 2)))
        XCTAssertEqual(TapEvent.decode(line: #"{"type":"question","id":"q4","kind":"approval"}"#),
                       .question(id: "q4", kind: "approval", payload: QuestionPayload()), "a missing payload decodes as an empty one")
        XCTAssertEqual(TapEvent.decode(line: #"{"type":"error","code":"deck_not_found","message":"deck not found: a.md"}"#),
                       .error(TapErrorPayload(code: "deck_not_found", message: "deck not found: a.md")))
        XCTAssertEqual(TapEvent.decode(line: #"{"type":"connected"}"#), .other(type: "connected"))
    }

    func testDecodesTheTalkEvents() {
        XCTAssertEqual(TapEvent.decode(line: #"{"type":"recording","state":"recording","segment":2,"elapsed":75,"disk":"ok"}"#),
                       .recording(RecordingEvent(state: "recording", segment: 2, elapsed: 75, disk: "ok")))
        XCTAssertEqual(TapEvent.decode(line: #"{"type":"tunnel","state":"running","url":"https://x.trycloudflare.com","qr":"aGk="}"#),
                       .tunnel(TunnelEvent(state: "running", url: "https://x.trycloudflare.com", qr: "aGk=")))
        XCTAssertEqual(TapEvent.decode(line: #"{"type":"tunnel","state":"stopped"}"#), .tunnel(TunnelEvent(state: "stopped", url: nil, qr: nil)))
        XCTAssertEqual(TapEvent.decode(line: #"{"type":"slide","slide":3,"step":1}"#), .slide(slide: 3, step: 1))
        XCTAssertNil(TapEvent.decode(line: #"{"type":"slide","step":1}"#), "a slide event without its slide is not an event")
    }

    func testIgnoresALineThatIsNotAnEvent() {
        XCTAssertNil(TapEvent.decode(line: "starting tap"))
        XCTAssertNil(TapEvent.decode(line: #"{"port": 1}"#))
        XCTAssertNil(TapEvent.decode(line: #"{"type":"ready","port":"not a number"}"#))
    }

    func testDecodesTheSlideListResponse() throws {
        let json = """
        {"ok": true, "slides": [{"number": 1, "startLine": 6, "endLine": 8, "layout": "title", "title": "Hello",
          "fragments": 1, "steps": 2, "skip": false, "errors": [],
          "codeBlocks": [{"block": 1, "language": "sql", "driver": "sqlite", "live": true, "line": 7}]}],
         "errors": []}
        """
        let list = try SlideList.decodeResponse(Data(json.utf8))
        XCTAssertEqual(list.slides.count, 1)
        XCTAssertEqual(list.slides[0], Slide(number: 1, startLine: 6, endLine: 8, layout: "title", title: "Hello",
                                             fragments: 1, steps: 2, skip: false, errors: [],
                                             codeBlocks: [CodeBlock(block: 1, language: "sql", driver: "sqlite", live: true, line: 7)]))
    }

    func testAnErrorResponseThrowsItsCodeAndMessage() {
        let json = #"{"ok": false, "error": {"code": "invalid_request", "message": "send {\"source\": ...}"}}"#
        XCTAssertThrowsError(try SlideList.decodeResponse(Data(json.utf8))) { error in
            XCTAssertEqual((error as? TapErrorPayload)?.code, "invalid_request")
        }
    }

    func testEncodesCommandsAsJSONLines() {
        XCTAssertEqual(TapCommand.saved.line, #"{"type":"saved"}"#)
        XCTAssertEqual(TapCommand.reload.line, #"{"type":"reload"}"#)
        XCTAssertEqual(TapCommand.quit.line, #"{"type":"quit"}"#)
        XCTAssertEqual(TapCommand.answer(id: "q1", value: true).line, #"{"type":"answer","id":"q1","value":true}"#)
        XCTAssertEqual(TapCommand.answer(id: "q\"2", value: false).line, #"{"type":"answer","id":"q\"2","value":false}"#, "the id is JSON-escaped")
        XCTAssertEqual(TapCommand.tunnel(start: true).line, #"{"type":"tunnel","start":true}"#)
        XCTAssertEqual(TapCommand.tunnel(start: false).line, #"{"type":"tunnel","start":false}"#)
        XCTAssertEqual(TapCommand.recording(action: .newSegment).line, #"{"type":"recording","action":"new-segment"}"#)
        XCTAssertEqual(TapCommand.recording(action: .stop).line, #"{"type":"recording","action":"stop"}"#)
    }

    func testEncodesTheSlideMessage() throws {
        let text = SlideMessage(slideIndex: 2, fragment: -1, step: 0).text()
        let object = try XCTUnwrap(JSONSerialization.jsonObject(with: Data(text.utf8)) as? [String: Any])
        XCTAssertEqual(object["type"] as? String, "slide")
        XCTAssertEqual(object["slideIndex"] as? Int, 2)
        XCTAssertEqual(object["fragment"] as? Int, -1)
        XCTAssertEqual(object["step"] as? Int, 0)
    }

    func testDecodesHubMessages() {
        XCTAssertEqual(HubMessage.decode(#"{"type":"update","revision":"7f3c","slides":[3]}"#), .update(revision: "7f3c", slides: [3]))
        XCTAssertEqual(HubMessage.decode(#"{"type":"reload"}"#), .reload)
        XCTAssertEqual(HubMessage.decode(#"{"type":"file-changed","path":"/a.md"}"#), .fileChanged(path: "/a.md"))
        XCTAssertEqual(HubMessage.decode(#"{"type":"slide","slideIndex":4,"step":1}"#), .slide(slideIndex: 4))
        XCTAssertEqual(HubMessage.decode(#"{"type":"connected","version":"1.0.0"}"#), .other(type: "connected"))
        XCTAssertNil(HubMessage.decode("not json"))
    }
}
