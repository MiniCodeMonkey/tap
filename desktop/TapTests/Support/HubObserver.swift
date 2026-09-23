import Foundation
@testable import Tap

/// A second client of tap's hub. The hub relays every `slide` message a
/// client sends to the others, so this sees exactly what the app sends.
@MainActor
final class HubObserver {
    private(set) var slideMessages: [SlideMessage] = []
    private let task: URLSessionWebSocketTask

    init(ready: TapReady) {
        task = URLSession(configuration: .ephemeral).webSocketTask(with: TapClient(ready: ready).socketRequest())
        task.resume()
        receive()
    }

    func close() {
        task.cancel(with: .goingAway, reason: nil)
    }

    private func receive() {
        task.receive { [weak self] result in
            guard case .success(.string(let text)) = result else { return }
            DispatchQueue.main.async {
                MainActor.assumeIsolated {
                    guard let self else { return }
                    if let object = try? JSONSerialization.jsonObject(with: Data(text.utf8)) as? [String: Any],
                       object["type"] as? String == "slide", object["initial"] == nil,
                       let index = object["slideIndex"] as? Int {
                        self.slideMessages.append(SlideMessage(slideIndex: index, fragment: object["fragment"] as? Int ?? -1,
                                                               step: object["step"] as? Int ?? 0))
                    }
                    self.receive()
                }
            }
        }
    }
}
