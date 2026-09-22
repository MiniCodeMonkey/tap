import Foundation

/// Sends the editor's text to tap after a typing pause, with at most one
/// PUT in flight. Text typed during a PUT goes out when it returns.
@MainActor
public final class SourceSync {
    public var sender: ((String) async throws -> SlideList)?
    public var onAnswer: ((SlideList, _ sentText: String, _ generation: Int) -> Void)?
    public var onFailure: ((Error) -> Void)?
    public private(set) var lastSentText: String?
    public private(set) var lastSentGeneration = 0
    public private(set) var lastEditDate: Date?

    private let delay: TimeInterval
    private let text: () -> String
    private let beginSend: () -> Int
    private var pause: DispatchWorkItem?
    private var inFlight = false
    private var sendAgain = false

    public init(delay: TimeInterval = 0.1, text: @escaping () -> String, beginSend: @escaping () -> Int) {
        self.delay = delay
        self.text = text
        self.beginSend = beginSend
    }

    public func textDidChange() {
        lastEditDate = Date()
        pause?.cancel()
        let work = DispatchWorkItem { [weak self] in
            MainActor.assumeIsolated {
                guard let self else { return }
                Task { @MainActor in await self.sendNow() }
            }
        }
        pause = work
        DispatchQueue.main.asyncAfter(deadline: .now() + delay, execute: work)
    }

    public func sendNow() async {
        pause?.cancel()
        guard sender != nil else { return }
        if inFlight {
            sendAgain = true
            return
        }
        inFlight = true
        defer { inFlight = false }
        repeat {
            sendAgain = false
            guard let sender else { return }
            let sentText = text()
            let generation = beginSend()
            lastSentText = sentText
            lastSentGeneration = generation
            do {
                let list = try await sender(sentText)
                onAnswer?(list, sentText, generation)
            } catch {
                onFailure?(error)
            }
        } while sendAgain
    }
}
