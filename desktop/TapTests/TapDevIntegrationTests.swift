import XCTest
@testable import Tap

final class TapDevIntegrationTests: HostedTestCase {
    func startBundledTap(on deck: URL) async throws -> (TapSession, TapReady) {
        let session = TapSession(deckURL: deck, configuration: AppEnvironment.shared.sessionConfiguration())
        session.start()
        addTeardownBlock { @MainActor in session.stop() }
        try await waitUntil(timeout: 30, "the ready line") { if case .running = session.state { return true } else { return false } }
        guard case .running(let ready) = session.state else { throw CancellationError() }
        return (session, ready)
    }

    func status(of request: URLRequest) async throws -> Int {
        let (_, response) = try await URLSession(configuration: .ephemeral).data(for: request)
        return (response as? HTTPURLResponse)?.statusCode ?? 0
    }

    func testStart() async throws {
        let (session, ready) = try await startBundledTap(on: try Fixtures.copyAppFixture())
        XCTAssertGreaterThan(ready.port, 0)
        XCTAssertEqual(ready.token.count, 64)
        XCTAssertEqual(ready.launch.count, 64)
        XCTAssertTrue(session.log.text.contains("tap dev --app talk.md"))
        let client = TapClient(ready: ready)
        XCTAssertEqual(client.baseURL.host, "127.0.0.1")
        let status = try await status(of: client.authorizedRequest(path: "/api/presentation"))
        XCTAssertEqual(status, 200)
    }

    func testPutRoundTripOnTheFixtureDeck() async throws {
        let deck = try Fixtures.copyAppFixture()
        let (_, ready) = try await startBundledTap(on: deck)
        let source = try String(contentsOf: deck, encoding: .utf8).replacingOccurrences(of: "# Fragments", with: "# Fragments, edited")
        let list = try await TapClient(ready: ready).putSource(source)
        XCTAssertEqual(list.errors, [])
        XCTAssertEqual(list.slides.map(\.number), [1, 2, 3, 4])
        XCTAssertEqual(list.slides[2].title, "Fragments, edited")
        XCTAssertEqual(list.slides[2].fragments, 1)
        XCTAssertEqual(list.slides[3].steps, 2)
        XCTAssertEqual(list.slides[1].codeBlocks.first?.driver, "shell")
        XCTAssertEqual(list.slides[1].codeBlocks.first?.live, true)
        let lines = source.components(separatedBy: "\n")
        XCTAssertEqual(lines[list.slides[0].startLine - 1], "# App Mode Fixture")
        let onDisk = try String(contentsOf: deck, encoding: .utf8)
        XCTAssertFalse(onDisk.contains("Fragments, edited"), "PUT never writes the deck file")
    }

    func testRequestsAreProtected() async throws {
        let (_, ready) = try await startBundledTap(on: try Fixtures.copyAppFixture())
        let client = TapClient(ready: ready)

        // Every request from the app carries the token.
        XCTAssertEqual(client.authorizedRequest(path: "/api/app/source").value(forHTTPHeaderField: "Authorization"), "Bearer \(ready.token)")
        XCTAssertEqual(client.socketRequest().value(forHTTPHeaderField: "Authorization"), "Bearer \(ready.token)")

        // tap rejects an app-only route without the token. The audience
        // routes are deliberately open, because a phone or a projector
        // browser reaching this server over the tunnel holds no app token:
        // see audienceRoutes in internal/server/app_auth.go, which lists
        // /api/presentation and /ws among them.
        let withoutToken = try await status(of: URLRequest(url: client.baseURL.appendingPathComponent("api/app/source")))
        XCTAssertEqual(withoutToken, 401)
        let audienceWithoutToken = try await status(of: URLRequest(url: client.baseURL.appendingPathComponent("api/presentation")))
        XCTAssertEqual(audienceWithoutToken, 200, "an audience route needs no app token")

        // ...a foreign Origin...
        var foreign = client.authorizedRequest(path: "/api/app/source")
        foreign.httpMethod = "PUT"
        foreign.setValue("application/json", forHTTPHeaderField: "Content-Type")
        foreign.setValue("https://evil.example", forHTTPHeaderField: "Origin")
        // Two hashes, because the body itself contains the sequence that one
        // hash would read as the end of the literal.
        foreign.httpBody = Data(##"{"source": "# One\n"}"##.utf8)
        let foreignStatus = try await status(of: foreign)
        XCTAssertEqual(foreignStatus, 403)

        // ...and a body that is not JSON.
        var plainText = client.authorizedRequest(path: "/api/app/source")
        plainText.httpMethod = "PUT"
        plainText.setValue("text/plain", forHTTPHeaderField: "Content-Type")
        plainText.httpBody = Data("# One\n".utf8)
        let plainTextStatus = try await status(of: plainText)
        XCTAssertEqual(plainTextStatus, 415)

        // A WebSocket upgrade without the token is allowed, because the
        // audience's own page opens it. What the token does not buy, and
        // the presenter secret does, is the right to drive other clients:
        // see WebSocketHub.checkPresenterAuth.
        let socket = URLSession(configuration: .ephemeral).webSocketTask(with: URL(string: "ws://127.0.0.1:\(ready.port)/ws")!)
        socket.resume()
        _ = try await socket.receive()
        socket.cancel(with: .goingAway, reason: nil)

        // With the token, the upgrade works.
        let authorized = URLSession(configuration: .ephemeral).webSocketTask(with: client.socketRequest())
        authorized.resume()
        let first = try await authorized.receive()
        if case .string(let text) = first {
            XCTAssertEqual(HubMessage.decode(text), .other(type: "connected"))
        } else {
            XCTFail("expected the connected message")
        }
        authorized.cancel(with: .goingAway, reason: nil)
    }

    /// Collects hub messages off one socket for as long as it is held. A
    /// bare `receive()` waits for a message that may never come, so a test
    /// that has to prove nothing arrives reads the collector instead.
    final class HubListener: @unchecked Sendable {
        private let lock = NSLock()
        private var messages: [HubMessage] = []

        init(_ socket: URLSessionWebSocketTask) {
            socket.resume()
            receive(socket)
        }

        private func receive(_ socket: URLSessionWebSocketTask) {
            socket.receive { [weak self] result in
                guard let self, case .success(.string(let text)) = result else { return }
                if let message = HubMessage.decode(text) {
                    self.lock.lock()
                    self.messages.append(message)
                    self.lock.unlock()
                }
                self.receive(socket)
            }
        }

        func heard(slideIndex: Int) -> Bool {
            lock.lock()
            defer { lock.unlock() }
            return messages.contains { if case .slide(let index) = $0 { return index == slideIndex } else { return false } }
        }
    }

    /// True once a `slide` message for `slideIndex` has arrived, false once
    /// `timeout` has passed with none.
    func waitForSlide(_ listener: HubListener, slideIndex: Int, timeout: TimeInterval) async throws -> Bool {
        let deadline = Date().addingTimeInterval(timeout)
        while Date() < deadline {
            if listener.heard(slideIndex: slideIndex) { return true }
            try await Task.sleep(nanoseconds: 50_000_000)
        }
        return false
    }

    func testThePresenterSecretIsWhatLetsTheAppDriveTheDeck() async throws {
        let (_, ready) = try await startBundledTap(on: try Fixtures.copyAppFixture())
        XCTAssertEqual(ready.presenter.count, 64)
        let client = TapClient(ready: ready)
        XCTAssertNil(client.presenterCookie)

        // A page that only watches. It hears whatever the hub relays.
        let audienceSocket = URLSession(configuration: .ephemeral)
            .webSocketTask(with: URLRequest(url: URL(string: "ws://127.0.0.1:\(ready.port)/ws")!))
        let audience = HubListener(audienceSocket)

        // Without the presenter cookie the hub drops what the app sends.
        let unauthorized = URLSession(configuration: .ephemeral).webSocketTask(with: client.socketRequest())
        unauthorized.resume()
        _ = try await unauthorized.receive()
        try await unauthorized.send(.string(SlideMessage(slideIndex: 1, fragment: -1, step: 0).text()))
        let relayedWithoutTheCookie = try await waitForSlide(audience, slideIndex: 1, timeout: 2)
        XCTAssertFalse(relayedWithoutTheCookie, "the hub relays nothing from a connection with no presenter cookie")
        unauthorized.cancel(with: .goingAway, reason: nil)

        // With it, the app drives every other page.
        let cookie = try await client.authorizePresenter()
        XCTAssertEqual(cookie.count, 64)
        XCTAssertNotEqual(cookie, ready.presenter, "the cookie carries the hub's session token, never the secret")
        XCTAssertEqual(client.socketRequest().value(forHTTPHeaderField: "Cookie"), "tap_presenter_key=\(cookie)")
        let presenter = URLSession(configuration: .ephemeral).webSocketTask(with: client.socketRequest())
        presenter.resume()
        _ = try await presenter.receive()
        try await presenter.send(.string(SlideMessage(slideIndex: 2, fragment: -1, step: 0).text()))
        let relayed = try await waitForSlide(audience, slideIndex: 2, timeout: 10)
        XCTAssertTrue(relayed, "the hub relays a slide message from the app once it holds the presenter cookie")
        presenter.cancel(with: .goingAway, reason: nil)
        audienceSocket.cancel(with: .goingAway, reason: nil)
    }

    func testThePresenterViewOpensWithTheSecret() async throws {
        let (_, ready) = try await startBundledTap(on: try Fixtures.copyAppFixture())
        let client = TapClient(ready: ready)
        let closed = try await status(of: URLRequest(url: client.baseURL.appendingPathComponent("presenter")))
        XCTAssertEqual(closed, 403, "the presenter view is closed without the secret")
        var withSecret = URLRequest(url: client.presenterLaunchURL)
        withSecret.setValue("Bearer \(ready.token)", forHTTPHeaderField: "Authorization")
        let open = try await status(of: withSecret)
        XCTAssertEqual(open, 200, "and open with it")
    }
}
