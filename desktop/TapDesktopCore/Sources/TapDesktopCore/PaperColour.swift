import Foundation

/// A colour as three 0...1 components.
public struct PaperColour: Equatable, Sendable {
    public let red: Double
    public let green: Double
    public let blue: Double

    public init(red: Double, green: Double, blue: Double) {
        self.red = red
        self.green = green
        self.blue = blue
    }

    /// The paper of a deck whose theme is unknown.
    public static let neutral = PaperColour(red: 0.97, green: 0.97, blue: 0.96)

    /// True when dark text reads on this colour.
    public var isLight: Bool {
        0.299 * red + 0.587 * green + 0.114 * blue > 0.55
    }

    /// The most common colour among `samples`, after rounding each
    /// component to a 16th so near-equal pixels count together; nil for
    /// no samples. A tie goes to the colour seen first.
    public static func dominant(of samples: [PaperColour]) -> PaperColour? {
        var counts: [[Int]: (count: Int, first: Int, sum: [Double])] = [:]
        for (index, sample) in samples.enumerated() {
            let bucket = [sample.red, sample.green, sample.blue].map { Int(($0 * 15).rounded()) }
            var entry = counts[bucket] ?? (0, index, [0, 0, 0])
            entry.count += 1
            entry.sum = zip(entry.sum, [sample.red, sample.green, sample.blue]).map(+)
            counts[bucket] = entry
        }
        guard let best = counts.values.max(by: { ($0.count, -$0.first) < ($1.count, -$1.first) }) else { return nil }
        let count = Double(best.count)
        return PaperColour(red: best.sum[0] / count, green: best.sum[1] / count, blue: best.sum[2] / count)
    }
}
