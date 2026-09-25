import Foundation

public struct TapLogLine: Equatable, Sendable {
    public enum Source: Equatable, Sendable {
        /// What the app did: started tap, saw it exit, restarted it.
        case app
        /// A line tap printed on standard output that is not an event.
        case standardOutput
        case standardError
        /// A summary of a stdout event.
        case event
    }

    public let date: Date
    public let source: Source
    public let text: String

    private static let timeFormatter: DateFormatter = {
        let formatter = DateFormatter()
        formatter.dateFormat = "HH:mm:ss"
        return formatter
    }()

    /// The line as the Tap Log window shows it: "· 19:02:11  text".
    public var formatted: String { "· \(Self.timeFormatter.string(from: date))  \(text)" }
}

/// The output of one deck's tap process, newest last.
@MainActor
public final class TapLog {
    public static let didAppendNotification = Notification.Name("TapLogDidAppend")

    public var title: String
    public let capacity: Int
    public private(set) var lines: [TapLogLine] = []

    public init(title: String, capacity: Int = 2_000) {
        self.title = title
        self.capacity = capacity
    }

    public func append(_ text: String, source: TapLogLine.Source, date: Date = Date()) {
        lines.append(TapLogLine(date: date, source: source, text: text))
        if lines.count > capacity { lines.removeFirst(lines.count - capacity) }
        NotificationCenter.default.post(name: Self.didAppendNotification, object: self)
    }

    public func lastLines(_ count: Int, from source: TapLogLine.Source) -> [String] {
        Array(lines.filter { $0.source == source }.suffix(count).map(\.text))
    }

    public var text: String { lines.map(\.formatted).joined(separator: "\n") }
}
