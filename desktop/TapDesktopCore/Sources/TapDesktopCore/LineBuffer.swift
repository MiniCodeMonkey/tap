import Foundation

/// Splits a byte stream into lines. A line ends at "\n"; a trailing "\r" is
/// dropped. Bytes after the last newline wait for the next chunk.
public struct LineBuffer: Sendable {
    private var pending = Data()

    public init() {}

    public mutating func append(_ data: Data) -> [String] {
        pending.append(data)
        var lines: [String] = []
        while let newline = pending.firstIndex(of: 0x0A) {
            var line = Data(pending[pending.startIndex..<newline])
            if line.last == 0x0D { line.removeLast() }
            lines.append(String(decoding: line, as: UTF8.self))
            pending = Data(pending[pending.index(after: newline)...])
        }
        return lines
    }

    /// Returns the text after the last newline once, at the end of the stream.
    public mutating func finish() -> String? {
        guard !pending.isEmpty else { return nil }
        defer { pending = Data() }
        return String(decoding: pending, as: UTF8.self)
    }
}
