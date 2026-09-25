import Foundation

/// Talks to one running `tap dev --app` over HTTP and the WebSocket. Every
/// request carries the token as `Authorization: Bearer`. The page gets its
/// cookie through the one-time launch code instead, so the token is never
/// in a URL.
public final class TapClient: @unchecked Sendable {
    public let ready: TapReady
    public let session: URLSession

    public init(ready: TapReady, session: URLSession = URLSession(configuration: .ephemeral)) {
        self.ready = ready
        self.session = session
    }

    /// Builds a URL against the one running tap. `scheme` is the only thing
    /// that changes between the HTTP endpoints and the WebSocket. Query
    /// values are percent-encoded, a plus included: the person's own
    /// presenter password may hold `#`, `&`, `+` or `%`, and Go's
    /// Query().Get would cut, split or space an unencoded one.
    private func url(scheme: String = "http", path: String = "", query: [String: String] = [:], fragment: String? = nil) -> URL {
        var components = URLComponents()
        components.scheme = scheme
        components.host = "127.0.0.1"
        components.port = ready.port
        components.path = path
        if !query.isEmpty {
            components.percentEncodedQuery = query.sorted { $0.key < $1.key }
                .map { "\($0.key)=\(Self.encodedQueryValue($0.value))" }
                .joined(separator: "&")
        }
        components.fragment = fragment
        return components.url!
    }

    /// `value` percent-encoded for a query: everything a query allows
    /// except the characters that mean something there (`&`, `=`, `+`, `#`).
    static func encodedQueryValue(_ value: String) -> String {
        var allowed = CharacterSet.urlQueryAllowed
        allowed.remove(charactersIn: "&=+#")
        return value.addingPercentEncoding(withAllowedCharacters: allowed) ?? ""
    }

    public var baseURL: URL { url() }
    public var previewURL: URL { url(path: "/") }
    public var previewLaunchURL: URL { url(path: "/", query: ["launch": ready.launch]) }
    /// The presenter view, carrying the ready line's presenter secret. tap
    /// answers with the presenter cookie and a redirect to the same path
    /// without the key, so the secret leaves the address bar at once.
    public var presenterLaunchURL: URL { url(path: "/presenter", query: ["key": ready.presenter]) }

    /// The audience page for a talk, starting on `slide` (1-based): the
    /// launch code as the preview uses it, and the slide in the fragment,
    /// which the page reads on load (`initializeFromURL` in
    /// frontend/src/lib/stores/presentation.ts). The fragment survives
    /// tap's redirect, which names no fragment of its own.
    public func audienceLaunchURL(slide: Int) -> URL { url(path: "/", query: ["launch": ready.launch], fragment: String(slide)) }

    /// The presenter page for a talk, starting on `slide`. It carries the
    /// presenter secret as `key`: the server sets the presenter cookie and
    /// redirects to /presenter without the key, and the redirect keeps the
    /// fragment (internal/server/routes.go, handlePresenter). The page
    /// therefore never depends on a cookie the app injected first.
    public func presenterURL(slide: Int) -> URL { url(path: "/presenter", query: ["key": ready.presenter], fragment: String(slide)) }

    /// The cookie tap issues in exchange for the presenter secret.
    public static let presenterCookieName = "tap_presenter_key"

    /// The presenter cookie's value, once `authorizePresenter` has fetched it.
    public private(set) var presenterCookie: String?

    /// Trades the ready line's presenter secret for the hub's presenter
    /// cookie, and keeps it for `socketRequest`. Until the app holds that
    /// cookie the hub relays none of its slide messages to the pages, so the
    /// preview and the presenter window cannot drive the deck: see
    /// `WebSocketHub.checkPresenterAuth` in internal/server/websocket.go.
    ///
    /// The redirect is not followed, because the cookie is on the redirect's
    /// own response and reading the header is what proves tap issued it.
    @discardableResult
    public func authorizePresenter() async throws -> String {
        guard !ready.presenter.isEmpty else {
            throw TapErrorPayload(code: "no_presenter_secret", message: "the ready line carried no presenter secret")
        }
        var request = URLRequest(url: presenterLaunchURL)
        request.timeoutInterval = 10
        let (_, response) = try await session.data(for: request, delegate: RedirectBlocker())
        guard let http = response as? HTTPURLResponse, let fields = http.allHeaderFields as? [String: String] else {
            throw TapErrorPayload(code: "presenter_refused", message: "tap gave no HTTP answer for the presenter secret")
        }
        let cookies = HTTPCookie.cookies(withResponseHeaderFields: fields, for: baseURL)
        guard let cookie = cookies.first(where: { $0.name == Self.presenterCookieName }) else {
            throw TapErrorPayload(code: "presenter_refused",
                                  message: "tap answered \(http.statusCode) without a presenter cookie")
        }
        presenterCookie = cookie.value
        return cookie.value
    }

    public func authorizedRequest(path: String) -> URLRequest {
        var request = URLRequest(url: url(path: path))
        request.setValue("Bearer \(ready.token)", forHTTPHeaderField: "Authorization")
        request.timeoutInterval = 10
        return request
    }

    public func socketRequest() -> URLRequest {
        var request = URLRequest(url: url(scheme: "ws", path: "/ws"))
        request.setValue("Bearer \(ready.token)", forHTTPHeaderField: "Authorization")
        if let presenterCookie {
            request.setValue("\(Self.presenterCookieName)=\(presenterCookie)", forHTTPHeaderField: "Cookie")
        }
        return request
    }

    /// Sends the unsaved buffer. tap renders it and answers with the slide list.
    public func putSource(_ source: String) async throws -> SlideList {
        var request = authorizedRequest(path: "/api/app/source")
        request.httpMethod = "PUT"
        request.setValue("application/json", forHTTPHeaderField: "Content-Type")
        request.httpBody = try JSONEncoder().encode(["source": source])
        let (data, response) = try await session.data(for: request)
        let status = (response as? HTTPURLResponse)?.statusCode ?? 0
        do {
            return try SlideList.decodeResponse(data)
        } catch let error as TapErrorPayload {
            throw error
        } catch {
            throw TapErrorPayload(code: "http_\(status)", message: String(decoding: data, as: UTF8.self))
        }
    }

    /// The rendered deck's summary, from the render tap shows now: the
    /// buffer while there is one, and the file otherwise.
    public func presentation() async throws -> PresentationSummary {
        let request = authorizedRequest(path: "/api/presentation")
        let (data, response) = try await session.data(for: request)
        let status = (response as? HTTPURLResponse)?.statusCode ?? 0
        guard status == 200 else {
            throw TapErrorPayload(code: "http_\(status)", message: String(decoding: data, as: UTF8.self))
        }
        return try PresentationSummary.decode(data)
    }

    @MainActor
    public func openSocket() -> TapSocket {
        TapSocket(task: session.webSocketTask(with: socketRequest()))
    }
}

/// The app's own connection to tap's WebSocket hub. The app moves the
/// preview with `slide` messages and hears `update` and `reload`.
@MainActor
public final class TapSocket {
    public var onMessage: ((HubMessage) -> Void)?
    public var onClose: (() -> Void)?
    private let task: URLSessionWebSocketTask
    private var isClosed = false

    init(task: URLSessionWebSocketTask) {
        self.task = task
    }

    public func resume() {
        task.resume()
        receive()
    }

    public func send(_ message: SlideMessage) {
        guard !isClosed else { return }
        task.send(.string(message.text())) { _ in }
    }

    public func close() {
        isClosed = true
        task.cancel(with: .goingAway, reason: nil)
    }

    private func receive() {
        task.receive { [weak self] result in
            DispatchQueue.main.async {
                MainActor.assumeIsolated {
                    guard let self, !self.isClosed else { return }
                    switch result {
                    case .success(.string(let text)):
                        if let message = HubMessage.decode(text) { self.onMessage?(message) }
                        self.receive()
                    case .success(.data(let data)):
                        if let message = HubMessage.decode(String(decoding: data, as: UTF8.self)) { self.onMessage?(message) }
                        self.receive()
                    case .success:
                        self.receive()
                    case .failure:
                        self.isClosed = true
                        self.onClose?()
                    }
                }
            }
        }
    }
}

/// Stops URLSession following a redirect, so the redirect's own response,
/// and the `Set-Cookie` header on it, is what the caller reads.
private final class RedirectBlocker: NSObject, URLSessionTaskDelegate, Sendable {
    func urlSession(_ session: URLSession, task: URLSessionTask,
                    willPerformHTTPRedirection response: HTTPURLResponse, newRequest request: URLRequest) async -> URLRequest? {
        nil
    }
}
