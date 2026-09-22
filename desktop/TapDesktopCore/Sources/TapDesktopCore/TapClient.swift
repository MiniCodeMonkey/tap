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

    public var baseURL: URL { URL(string: "http://127.0.0.1:\(ready.port)")! }
    public var previewURL: URL { URL(string: "http://127.0.0.1:\(ready.port)/")! }
    public var previewLaunchURL: URL { URL(string: "http://127.0.0.1:\(ready.port)/?launch=\(ready.launch)")! }

    public func authorizedRequest(path: String) -> URLRequest {
        var request = URLRequest(url: URL(string: "http://127.0.0.1:\(ready.port)\(path)")!)
        request.setValue("Bearer \(ready.token)", forHTTPHeaderField: "Authorization")
        request.timeoutInterval = 10
        return request
    }

    public func socketRequest() -> URLRequest {
        var request = URLRequest(url: URL(string: "ws://127.0.0.1:\(ready.port)/ws")!)
        request.setValue("Bearer \(ready.token)", forHTTPHeaderField: "Authorization")
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
