import Foundation

/// Sends the editor's text to tap after a typing pause, with at most one
/// PUT in flight. Text typed during a PUT goes out when it returns.
///
/// A pause of `delay` triggers a send, same as before. Alongside it, a
/// `maxWait` fallback guarantees a send fires at least that often while
/// edits keep arriving, so someone who never pauses longer than `delay`
/// (holding a key down, pasting and continuing to type, typing quickly)
/// still sees the preview catch up on a steady cadence instead of the
/// debounce timer resetting forever and nothing going out at all.
@MainActor
public final class SourceSync {
    public var sender: ((String) async throws -> SlideList)?
    public var onAnswer: ((SlideList, _ sentText: String, _ generation: Int) -> Void)?
    public var onFailure: ((Error) -> Void)?
    public private(set) var lastSentText: String?
    public private(set) var lastSentGeneration = 0
    public private(set) var lastEditDate: Date?

    private let delay: TimeInterval
    private let maxWait: TimeInterval
    private let text: () -> String
    private let beginSend: () -> Int
    private var pause: DispatchWorkItem?
    private var maxWaitPause: DispatchWorkItem?
    private var inFlight = false
    private var sendAgain = false

    public init(delay: TimeInterval = 0.1, maxWait: TimeInterval = 1.0, text: @escaping () -> String, beginSend: @escaping () -> Int) {
        self.delay = delay
        self.maxWait = maxWait
        self.text = text
        self.beginSend = beginSend
    }

    public func textDidChange() {
        lastEditDate = Date()
        pause?.cancel()
        let work = DispatchWorkItem { [weak self] in
            MainActor.assumeIsolated {
                guard let self else { return }
                self.trigger()
            }
        }
        pause = work
        DispatchQueue.main.asyncAfter(deadline: .now() + delay, execute: work)

        // Pin one fallback timer per pending window: further edits reset
        // the debounce timer above, but they must not push this one back,
        // or a typist who never pauses would push it back forever.
        if maxWaitPause == nil {
            let fallback = DispatchWorkItem { [weak self] in
                MainActor.assumeIsolated {
                    guard let self else { return }
                    self.trigger()
                }
            }
            maxWaitPause = fallback
            DispatchQueue.main.asyncAfter(deadline: .now() + maxWait, execute: fallback)
        }
    }

    private func trigger() {
        pause?.cancel()
        pause = nil
        maxWaitPause?.cancel()
        maxWaitPause = nil
        Task { @MainActor in await self.sendNow() }
    }

    public func sendNow() async {
        pause?.cancel()
        pause = nil
        maxWaitPause?.cancel()
        maxWaitPause = nil
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
