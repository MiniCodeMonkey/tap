import XCTest
@testable import TapDesktopCore

/// Answers every request with a canned response and remembers the request.
final class StubURLProtocol: URLProtocol {
    nonisolated(unsafe) static var responseStatus = 200
    nonisolated(unsafe) static var responseBody = Data()
    nonisolated(unsafe) static var lastRequest: URLRequest?
    nonisolated(unsafe) static var lastBody = Data()

    override class func canInit(with request: URLRequest) -> Bool { true }
    override class func canonicalRequest(for request: URLRequest) -> URLRequest { request }

    override func startLoading() {
        Self.lastRequest = request
        if let stream = request.httpBodyStream {
            stream.open()
            var body = Data()
            var buffer = [UInt8](repeating: 0, count: 4096)
            while stream.hasBytesAvailable {
                let count = stream.read(&buffer, maxLength: buffer.count)
                if count <= 0 { break }
                body.append(buffer, count: count)
            }
            stream.close()
            Self.lastBody = body
        } else {
            Self.lastBody = request.httpBody ?? Data()
        }
        let response = HTTPURLResponse(url: request.url!, statusCode: Self.responseStatus, httpVersion: nil,
                                       headerFields: ["Content-Type": "application/json"])!
        client?.urlProtocol(self, didReceive: response, cacheStoragePolicy: .notAllowed)
        client?.urlProtocol(self, didLoad: Self.responseBody)
        client?.urlProtocolDidFinishLoading(self)
    }

    override func stopLoading() {}
}

final class TapClientTests: XCTestCase {
    let ready = TapReady(port: 49152, token: String(repeating: "a", count: 64), launch: String(repeating: "b", count: 64))

    func stubbedClient() -> TapClient {
        let configuration = URLSessionConfiguration.ephemeral
        configuration.protocolClasses = [StubURLProtocol.self]
        return TapClient(ready: ready, session: URLSession(configuration: configuration))
    }

    func testPutSendsTheTokenAndAJSONBody() async throws {
        StubURLProtocol.responseStatus = 200
        StubURLProtocol.responseBody = Data(#"{"ok": true, "slides": [{"number": 1, "startLine": 1, "endLine": 1, "layout": "default", "title": "One", "fragments": 0, "steps": 0, "skip": false, "errors": [], "codeBlocks": []}], "errors": []}"#.utf8)
        let list = try await stubbedClient().putSource("# One\n")
        XCTAssertEqual(list.slides.map(\.title), ["One"])

        let request = try XCTUnwrap(StubURLProtocol.lastRequest)
        XCTAssertEqual(request.httpMethod, "PUT")
        XCTAssertEqual(request.url?.absoluteString, "http://127.0.0.1:49152/api/app/source")
        XCTAssertEqual(request.value(forHTTPHeaderField: "Authorization"), "Bearer " + ready.token)
        XCTAssertEqual(request.value(forHTTPHeaderField: "Content-Type"), "application/json")
        let body = try JSONSerialization.jsonObject(with: StubURLProtocol.lastBody) as? [String: String]
        XCTAssertEqual(body, ["source": "# One\n"])
    }

    func testAnErrorAnswerThrowsTapsError() async {
        StubURLProtocol.responseStatus = 400
        StubURLProtocol.responseBody = Data(#"{"ok": false, "error": {"code": "invalid_request", "message": "bad body"}}"#.utf8)
        do {
            _ = try await stubbedClient().putSource("x")
            XCTFail("expected an error")
        } catch {
            XCTAssertEqual(error as? TapErrorPayload, TapErrorPayload(code: "invalid_request", message: "bad body"))
        }
    }

    func testAnAnswerThatIsNotJSONThrowsTheStatus() async {
        StubURLProtocol.responseStatus = 401
        StubURLProtocol.responseBody = Data("Unauthorized".utf8)
        do {
            _ = try await stubbedClient().putSource("x")
            XCTFail("expected an error")
        } catch {
            XCTAssertEqual((error as? TapErrorPayload)?.code, "http_401")
        }
    }

    func testTheSocketAndThePreviewURLs() {
        let client = TapClient(ready: ready)
        let socket = client.socketRequest()
        XCTAssertEqual(socket.url?.absoluteString, "ws://127.0.0.1:49152/ws")
        XCTAssertEqual(socket.value(forHTTPHeaderField: "Authorization"), "Bearer " + ready.token)
        XCTAssertEqual(client.previewLaunchURL.absoluteString, "http://127.0.0.1:49152/?launch=" + ready.launch)
        XCTAssertEqual(client.previewURL.absoluteString, "http://127.0.0.1:49152/")
        XCTAssertFalse(client.previewURL.absoluteString.contains(ready.token), "the token never appears in a URL")
    }
}
