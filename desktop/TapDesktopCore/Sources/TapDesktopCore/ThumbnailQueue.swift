import Foundation

/// Which slide renders next: the current slide, then the visible ones in
/// order, then the rest by distance from the visible range (or from the
/// current slide when nothing is visible), then deck order.
///
/// A slide whose capture keeps coming back flat (the flat-image check can
/// misjudge a slide that painted late) is requeued, but only up to
/// `maxRetries` times. Past that the queue gives up on that number: it
/// drops out of `pending` and `replace` does not bring it back, so a truly
/// blank slide is rendered once, not forever.
public struct ThumbnailQueue: Equatable, Sendable {
    /// How many times `requeue` retries a number before backing off.
    public static let maxRetries = 3

    public private(set) var pending: [Int] = []
    private var retryCounts: [Int: Int] = [:]

    public init() {}

    public var isEmpty: Bool { pending.isEmpty }

    public mutating func replace(with numbers: [Int], visible: [Int], current: Int?) {
        let givenUp = Set(retryCounts.filter { $0.value > Self.maxRetries }.keys)
        let wanted = Array(Set(numbers).subtracting(givenUp)).sorted()
        retryCounts = retryCounts.filter { numbers.contains($0.key) }
        let visibleSet = Set(visible)
        var ordered: [Int] = []
        if let current, wanted.contains(current) { ordered.append(current) }
        ordered.append(contentsOf: visible.filter { wanted.contains($0) && !ordered.contains($0) })
        let anchorLow = visible.min() ?? current
        let anchorHigh = visible.max() ?? current
        let rest = wanted.filter { !ordered.contains($0) && !visibleSet.contains($0) }
        let sortedRest: [Int]
        if let anchorLow, let anchorHigh {
            func distance(_ number: Int) -> Int {
                if number < anchorLow { return anchorLow - number }
                if number > anchorHigh { return number - anchorHigh }
                return 0
            }
            sortedRest = rest.sorted { (distance($0), $0) < (distance($1), $1) }
        } else {
            sortedRest = rest
        }
        pending = ordered + sortedRest
    }

    public mutating func next() -> Int? {
        pending.isEmpty ? nil : pending.removeFirst()
    }

    /// Puts a number back at the end of the queue, for a capture that came
    /// back flat and might just have been too early. Backs off once the
    /// number has been requeued `maxRetries` times: it is dropped instead,
    /// so a genuinely blank slide is not retried without end.
    public mutating func requeue(_ number: Int) {
        pending.removeAll { $0 == number }
        let attempts = (retryCounts[number] ?? 0) + 1
        retryCounts[number] = attempts
        guard attempts <= Self.maxRetries else { return }
        pending.append(number)
    }

    public mutating func remove(_ number: Int) {
        pending.removeAll { $0 == number }
        retryCounts.removeValue(forKey: number)
    }
}
