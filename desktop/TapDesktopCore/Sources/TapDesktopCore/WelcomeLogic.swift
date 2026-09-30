import Foundation

/// Which recent decks a search shows: every whitespace-separated word of the
/// query must appear in the deck's file name or in its folder path, ignoring
/// case and accents.
public enum RecentDeckMatcher {
    public static func matches(name: String, path: String, query: String) -> Bool {
        let words = query.split(whereSeparator: \.isWhitespace).map(String.init)
        guard !words.isEmpty else { return true }
        let haystack = name + "\n" + path
        return words.allSatisfy { haystack.range(of: $0, options: [.caseInsensitive, .diacriticInsensitive]) != nil }
    }
}

/// The short date the recent decks list shows beside a deck.
public enum RecentDeckDate {
    /// "Today", "Yesterday", "Sep 24" within the year, "Sep 24, 2025" before it.
    public static func label(for date: Date, now: Date = Date(), calendar: Calendar = .current, locale: Locale = .current) -> String {
        if calendar.isDate(date, inSameDayAs: now) { return "Today" }
        if let yesterday = calendar.date(byAdding: .day, value: -1, to: now), calendar.isDate(date, inSameDayAs: yesterday) { return "Yesterday" }
        let sameYear = calendar.component(.year, from: date) == calendar.component(.year, from: now)
        let formatter = DateFormatter()
        formatter.locale = locale
        formatter.calendar = calendar
        formatter.timeZone = calendar.timeZone
        formatter.setLocalizedDateFormatFromTemplate(sameYear ? "MMMd" : "MMMdyyyy")
        return formatter.string(from: date)
    }
}

/// The welcome window's theme filmstrip: cards in a row that drifts left and
/// wraps, so the strip repeats the row until it covers the window and the
/// wrap is invisible.
public struct FilmstripLoop: Equatable, Sendable {
    public static let cardWidth: Double = 172
    public static let gap: Double = 16
    /// Seconds for the row to travel its own length.
    public static let cycleDuration: Double = 90

    public let cardCount: Int

    public init(cardCount: Int) { self.cardCount = max(cardCount, 0) }

    /// One card and the gap after it.
    public var pitch: Double { Self.cardWidth + Self.gap }
    /// The distance after which the strip looks the same as it did.
    public var period: Double { Double(cardCount) * pitch }

    /// The scroll offset `elapsed` seconds into the drift, in 0..<period.
    public func offset(afterSeconds elapsed: Double) -> Double {
        guard period > 0 else { return 0 }
        let travelled = elapsed / Self.cycleDuration * period
        let wrapped = travelled.truncatingRemainder(dividingBy: period)
        return wrapped < 0 ? wrapped + period : wrapped
    }

    /// `offset` moved on by `seconds`, from any offset, wrapped into 0..<period.
    public func advance(_ offset: Double, bySeconds seconds: Double) -> Double {
        guard period > 0 else { return 0 }
        let moved = (offset + seconds / Self.cycleDuration * period).truncatingRemainder(dividingBy: period)
        return moved < 0 ? moved + period : moved
    }

    /// How many copies of the row keep `visibleWidth` covered at any offset.
    public func copiesNeeded(visibleWidth: Double) -> Int {
        guard period > 0 else { return 0 }
        return max(2, Int((visibleWidth / period).rounded(.up)) + 1)
    }
}

/// The aurora's motion over time, as the mockup specifies it.
public enum AuroraTimeline {
    /// The height the lights rise from when the window opens.
    public static let openingHeight = 0.08
    public static let riseDuration = 2.2
    public static let fadeInDuration = 1.6
    /// How much brighter the lights get while a file is dragged over the window.
    public static let dragBoost = 1.6

    /// The resting height for an appearance.
    public static func restingHeight(isDark: Bool) -> Double { isDark ? 0.30 : 0.34 }
    /// The resting intensity for an appearance.
    public static func restingIntensity(isDark: Bool) -> Double { isDark ? 1.25 : 1.06 }

    /// The height `elapsed` seconds after the window opened: an ease-out cubic from the opening height to `resting`.
    public static func height(elapsed: Double, resting: Double, reduceMotion: Bool) -> Double {
        guard !reduceMotion else { return resting }
        let progress = min(max(elapsed / riseDuration, 0), 1)
        let eased = 1 - pow(1 - progress, 3)
        return openingHeight + (resting - openingHeight) * eased
    }

    /// The layer's opacity `elapsed` seconds after the window opened.
    public static func opacity(elapsed: Double, reduceMotion: Bool) -> Double {
        guard !reduceMotion else { return 1 }
        return min(max(elapsed / fadeInDuration, 0), 1)
    }

    /// The boost multiplier one frame later: it eases toward its target and, unlike a per-frame lerp, takes the same time at any frame rate.
    public static func boost(current: Double, dragging: Bool, deltaSeconds: Double) -> Double {
        let target = dragging ? dragBoost : 1
        let blend = 1 - exp(-5 * max(deltaSeconds, 0))
        let next = current + (target - current) * blend
        return abs(next - target) < 0.001 ? target : next
    }
}
