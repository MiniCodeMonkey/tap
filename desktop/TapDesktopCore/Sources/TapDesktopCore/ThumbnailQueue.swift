import Foundation

/// Which slide renders next: the current slide, then the visible ones in
/// order, then the rest by distance from the visible range (or from the
/// current slide when nothing is visible), then deck order.
public struct ThumbnailQueue: Equatable, Sendable {
    public private(set) var pending: [Int] = []

    public init() {}

    public var isEmpty: Bool { pending.isEmpty }

    public mutating func replace(with numbers: [Int], visible: [Int], current: Int?) {
        let wanted = Array(Set(numbers)).sorted()
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

    public mutating func requeue(_ number: Int) {
        pending.removeAll { $0 == number }
        pending.append(number)
    }

    public mutating func remove(_ number: Int) {
        pending.removeAll { $0 == number }
    }
}
