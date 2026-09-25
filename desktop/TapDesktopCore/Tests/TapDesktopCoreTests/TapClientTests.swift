import XCTest
@testable import TapDesktopCore

/// Answers every request with a canned response and remembers the request.
final class StubURLProtocol: URLProtocol {
    nonisolated(unsafe) static var responseStatus = 200
    nonisolated(unsafe) static var responseBody = Data()
    nonisolated(unsafe) static var lastRequest: URLRequest?
    nonisolated(unsafe) static var lastBody = Data()
    /// Headers on top of the JSON content type, such as a Set-Cookie.
    nonisolated(unsafe) static var responseHeaders: [String: String] = [:]

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
        var headers = ["Content-Type": "application/json"]
        headers.merge(Self.responseHeaders) { _, extra in extra }
        let response = HTTPURLResponse(url: request.url!, statusCode: Self.responseStatus, httpVersion: nil,
                                       headerFields: headers)!
        client?.urlProtocol(self, didReceive: response, cacheStoragePolicy: .notAllowed)
        client?.urlProtocol(self, didLoad: Self.responseBody)
        client?.urlProtocolDidFinishLoading(self)
    }

    override func stopLoading() {}
}

final class TapClientTests: XCTestCase {
    let ready = TapReady(port: 49152, token: String(repeating: "a", count: 64), launch: String(repeating: "b", count: 64),
                         presenter: String(repeating: "c", count: 64))

    override func setUp() {
        StubURLProtocol.responseHeaders = [:]
    }

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
        XCTAssertNil(socket.value(forHTTPHeaderField: "Cookie"), "no presenter cookie until one is fetched")
        XCTAssertEqual(client.presenterLaunchURL.absoluteString, "http://127.0.0.1:49152/presenter?key=" + ready.presenter)
    }

    func testThePresenterSecretBuysTheCookieTheSocketCarries() async throws {
        StubURLProtocol.responseStatus = 302
        StubURLProtocol.responseBody = Data()
        StubURLProtocol.responseHeaders = ["Set-Cookie": "tap_presenter_key=session-token; Path=/; HttpOnly"]
        let client = stubbedClient()
        let cookie = try await client.authorizePresenter()
        XCTAssertEqual(cookie, "session-token")
        XCTAssertEqual(StubURLProtocol.lastRequest?.url?.absoluteString,
                       "http://127.0.0.1:49152/presenter?key=" + ready.presenter)
        XCTAssertEqual(client.socketRequest().value(forHTTPHeaderField: "Cookie"), "tap_presenter_key=session-token")
    }

    func testAnAnswerWithoutThePresenterCookieThrows() async {
        StubURLProtocol.responseStatus = 403
        StubURLProtocol.responseBody = Data()
        StubURLProtocol.responseHeaders = [:]
        do {
            _ = try await stubbedClient().authorizePresenter()
            XCTFail("expected an error")
        } catch {
            XCTAssertEqual((error as? TapErrorPayload)?.code, "presenter_refused")
        }
    }

    func testAReadyLineWithoutAPresenterSecretIsRefusedBeforeAnyRequest() async {
        let client = TapClient(ready: TapReady(port: 49152, token: "t", launch: "l"))
        do {
            _ = try await client.authorizePresenter()
            XCTFail("expected an error")
        } catch {
            XCTAssertEqual((error as? TapErrorPayload)?.code, "no_presenter_secret")
        }
    }

    func testPresentationFetchesTheSummaryWithTheToken() async throws {
        StubURLProtocol.responseStatus = 200
        StubURLProtocol.responseBody = Data(#"{"config": {"theme": "base"}, "slides": [{"hash": "h1", "steps": 1}], "revision": "r1"}"#.utf8)
        let summary = try await stubbedClient().presentation()
        XCTAssertEqual(summary.revision, "r1")
        XCTAssertEqual(summary.slides, [SlideSummary(hash: "h1", skip: false, steps: 1)])
        let request = try XCTUnwrap(StubURLProtocol.lastRequest)
        XCTAssertEqual(request.httpMethod, "GET")
        XCTAssertEqual(request.url?.absoluteString, "http://127.0.0.1:49152/api/presentation")
        XCTAssertEqual(request.value(forHTTPHeaderField: "Authorization"), "Bearer " + ready.token)
    }

    func testPresentationThrowsTheStatusOnAnError() async {
        StubURLProtocol.responseStatus = 404
        StubURLProtocol.responseBody = Data(#"{"error": "No presentation loaded"}"#.utf8)
        do {
            _ = try await stubbedClient().presentation()
            XCTFail("expected an error")
        } catch {
            XCTAssertEqual((error as? TapErrorPayload)?.code, "http_404")
        }
    }

    func testTheTalkPagesCarryTheStartSlideInTheirHash() {
        let client = TapClient(ready: TapReady(port: 4242, token: "t", launch: "launch-code", presenter: "p"))
        XCTAssertEqual(client.audienceLaunchURL(slide: 3).absoluteString, "http://127.0.0.1:4242/?launch=launch-code#3")
        XCTAssertEqual(client.presenterURL(slide: 3).absoluteString, "http://127.0.0.1:4242/presenter?key=p#3",
                       "the presenter page carries its key: the server sets the cookie and redirects, keeping the hash")
        XCTAssertEqual(client.audienceLaunchURL(slide: 1).fragment, "1")
        XCTAssertEqual(client.presenterURL(slide: 1).fragment, "1")
    }

    func testThePresenterKeyIsPercentEncoded() throws {
        // The person's own password can hold anything; each of these would cut, split or space the key if it went in raw.
        let password = "a b#c&d+e%"
        let client = TapClient(ready: TapReady(port: 4242, token: "t", launch: "launch-code", presenter: password))
        for url in [client.presenterLaunchURL, client.presenterURL(slide: 2)] {
            let components = try XCTUnwrap(URLComponents(url: url, resolvingAgainstBaseURL: false))
            XCTAssertEqual(components.path, "/presenter")
            XCTAssertEqual(components.queryItems?.count, 1)
            XCTAssertEqual(components.queryItems?.first?.name, "key")
            XCTAssertEqual(components.queryItems?.first?.value, password, "decodes back to the same string: \(url)")
            XCTAssertFalse(components.percentEncodedQuery?.contains("+") ?? true, "a plus is encoded, or Go reads it as a space")
        }
        XCTAssertEqual(client.presenterURL(slide: 2).fragment, "2", "the hash survives the encoding")
    }
}
