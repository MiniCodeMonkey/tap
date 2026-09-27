import Foundation
@testable import Tap

extension TapClient {
    /// POSTs `body` to /api/execute the way the page does (JSON, from the
    /// same origin), with the app token, and returns the status and the
    /// answer's text. Test-only: the app itself never runs a block.
    func execute(json body: String) async throws -> (status: Int, body: String) {
        var request = authorizedRequest(path: "/api/execute")
        request.httpMethod = "POST"
        request.setValue("application/json", forHTTPHeaderField: "Content-Type")
        request.setValue("http://127.0.0.1:\(ready.port)", forHTTPHeaderField: "Origin")
        request.httpBody = Data(body.utf8)
        let (data, response) = try await session.data(for: request)
        return ((response as? HTTPURLResponse)?.statusCode ?? 0, String(decoding: data, as: UTF8.self))
    }

    /// `execute(json:)` with the answer's `error` field decoded, since tap
    /// writes it with `json.Encoder` and a quote inside it comes back escaped.
    func executeError(json body: String) async throws -> (status: Int, error: String) {
        let (status, text) = try await execute(json: body)
        let object = try JSONSerialization.jsonObject(with: Data(text.utf8)) as? [String: Any]
        return (status, object?["error"] as? String ?? text)
    }
}
