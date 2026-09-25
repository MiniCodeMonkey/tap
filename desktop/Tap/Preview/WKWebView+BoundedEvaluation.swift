import WebKit

extension WKWebView {
    /// What a bounded evaluation came back with.
    enum BoundedEvaluation {
        case value(Any?)
        case failed(Error)
        /// The page did not answer within the time given.
        case noAnswer(TimeInterval)
    }

    /// Evaluates `script` in the page, giving it `timeout` seconds to
    /// answer. A page whose web content process is stuck or gone may
    /// never call back, so a caller that waits for the answer, such as a
    /// test reading the page's state after a timeout, returns anyway.
    @MainActor
    func evaluate(_ script: String, timeout: TimeInterval) async -> BoundedEvaluation {
        await withCheckedContinuation { (continuation: CheckedContinuation<BoundedEvaluation, Never>) in
            var answered = false
            evaluateJavaScript(script) { value, error in
                guard !answered else { return }
                answered = true
                continuation.resume(returning: error.map { .failed($0) } ?? .value(value))
            }
            DispatchQueue.main.asyncAfter(deadline: .now() + timeout) {
                guard !answered else { return }
                answered = true
                continuation.resume(returning: .noAnswer(timeout))
            }
        }
    }
}
