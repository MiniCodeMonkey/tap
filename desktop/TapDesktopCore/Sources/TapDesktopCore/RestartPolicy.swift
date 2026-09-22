import Foundation

public enum RestartDecision: Equatable, Sendable {
    case restart(after: TimeInterval)
    case giveUp
}

/// Decides what happens when tap exits unexpectedly: restart with a delay
/// that doubles with each recent exit, and give up at the third exit within
/// thirty seconds.
public struct RestartPolicy: Sendable {
    public let window: TimeInterval
    public let maximumExits: Int
    public let baseDelay: TimeInterval
    public let maximumDelay: TimeInterval
    private var exits: [Date] = []

    public init(window: TimeInterval = 30, maximumExits: Int = 3, baseDelay: TimeInterval = 0.5, maximumDelay: TimeInterval = 8) {
        self.window = window
        self.maximumExits = maximumExits
        self.baseDelay = baseDelay
        self.maximumDelay = maximumDelay
    }

    public var recentExitCount: Int { exits.count }

    public mutating func recordExit(at date: Date) -> RestartDecision {
        exits = exits.filter { date.timeIntervalSince($0) <= window }
        exits.append(date)
        if exits.count >= maximumExits { return .giveUp }
        let delay = baseDelay * pow(2, Double(exits.count - 1))
        return .restart(after: min(delay, maximumDelay))
    }

    public mutating func reset() {
        exits = []
    }
}
