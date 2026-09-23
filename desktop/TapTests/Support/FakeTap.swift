import Foundation
import Network

/// Stands in for `tap dev --app` in the hosted tests. `dev --app` is not
/// implemented by the CLI on this branch yet (it lands with Task 13, on a
/// parallel branch), so `HostedTestCase` points every session at this
/// instead of the bundled tap. It prints a ready line at once and keeps
/// running until its standard input closes, same as `FakeTap` in
/// TapDesktopCoreTests.
///
/// Its `PUT /api/app/source` answers come from `FakeSlideParser`, a small
/// stand-in for tap's own slide parsing. `tap slide list --json`, which
/// would let this answer come from the real parser instead, is not
/// implemented by the CLI on this branch either (also Task 13); until it
/// is, this is the only source for a slide list a hosted test can reach.
enum FakeTap {
    /// Listeners started by `ready()`, kept alive for the process's lifetime.
    private static var listeners: [NWListener] = []
    private static let queue = DispatchQueue(label: "com.tap.faketap.server")

    static func ready() throws -> URL {
        let port = try startServer()
        let folder = FileManager.default.temporaryDirectory.appendingPathComponent("tap-desktop-app-tests-\(UUID().uuidString)")
        try FileManager.default.createDirectory(at: folder, withIntermediateDirectories: true)
        let url = folder.appendingPathComponent("tap")
        let body = """
        #!/bin/sh
        echo '{"type":"ready","port":\(port),"token":"token","launch":"launch"}'
        while IFS= read -r line; do :; done
        exit 0
        """
        try body.write(to: url, atomically: true, encoding: .utf8)
        try FileManager.default.setAttributes([.posixPermissions: 0o755], ofItemAtPath: url.path)
        return url
    }

    /// Starts a loopback HTTP listener on a fresh ephemeral port and
    /// returns it once the listener is ready to accept connections.
    private static func startServer() throws -> UInt16 {
        let parameters = NWParameters.tcp
        parameters.allowLocalEndpointReuse = true
        parameters.requiredLocalEndpoint = NWEndpoint.hostPort(host: "127.0.0.1", port: .any)
        let listener = try NWListener(using: parameters)
        let semaphore = DispatchSemaphore(value: 0)
        var assignedPort: UInt16 = 0
        listener.stateUpdateHandler = { state in
            if case .ready = state, let port = listener.port {
                assignedPort = port.rawValue
                semaphore.signal()
            }
        }
        listener.newConnectionHandler = { connection in
            connection.start(queue: queue)
            receive(connection, accumulated: Data())
        }
        listener.start(queue: queue)
        semaphore.wait()
        listeners.append(listener)
        return assignedPort
    }

    /// Reads one HTTP request, waiting for the header terminator and then
    /// for the full body named by `Content-Length`, and answers it.
    private static func receive(_ connection: NWConnection, accumulated: Data) {
        connection.receive(minimumIncompleteLength: 1, maximumLength: 65536) { data, _, isComplete, error in
            var buffer = accumulated
            if let data { buffer.append(data) }
            if let headerRange = buffer.range(of: Data("\r\n\r\n".utf8)) {
                let headerText = String(decoding: buffer[..<headerRange.lowerBound], as: UTF8.self)
                let contentLength = self.contentLength(in: headerText)
                let bodyStart = headerRange.upperBound
                if buffer.count - bodyStart >= contentLength {
                    let body = buffer[bodyStart..<(bodyStart + contentLength)]
                    respond(connection, headerText: headerText, body: Data(body))
                    return
                }
            }
            guard !isComplete, error == nil else {
                connection.cancel()
                return
            }
            receive(connection, accumulated: buffer)
        }
    }

    private static func contentLength(in headerText: String) -> Int {
        for line in headerText.components(separatedBy: "\r\n").dropFirst() {
            let fields = line.split(separator: ":", maxSplits: 1)
            if fields.count == 2, fields[0].trimmingCharacters(in: .whitespaces).lowercased() == "content-length" {
                return Int(fields[1].trimmingCharacters(in: .whitespaces)) ?? 0
            }
        }
        return 0
    }

    private static func respond(_ connection: NWConnection, headerText: String, body: Data) {
        guard let requestLine = headerText.components(separatedBy: "\r\n").first else {
            connection.cancel()
            return
        }
        let parts = requestLine.split(separator: " ")
        let method = parts.count > 0 ? String(parts[0]) : ""
        let path = parts.count > 1 ? String(parts[1]) : ""

        guard method == "PUT", path == "/api/app/source" else {
            send(connection, status: "404 Not Found", body: Data())
            return
        }
        var source = ""
        if let object = try? JSONSerialization.jsonObject(with: body) as? [String: Any], let text = object["source"] as? String {
            source = text
        }
        let (slides, errors) = FakeSlideParser.parse(source)
        let payload: [String: Any] = [
            "ok": true,
            "slides": slides.map(\.jsonObject),
            "errors": errors,
        ]
        let responseBody = (try? JSONSerialization.data(withJSONObject: payload)) ?? Data()
        send(connection, status: "200 OK", body: responseBody)
    }

    private static func send(_ connection: NWConnection, status: String, body: Data) {
        var response = Data("HTTP/1.1 \(status)\r\nContent-Type: application/json\r\nContent-Length: \(body.count)\r\nConnection: close\r\n\r\n".utf8)
        response.append(body)
        connection.send(content: response, completion: .contentProcessed { _ in connection.cancel() })
    }
}

/// One code block on a fake slide, in the shape tap reports
/// (`CodeBlock` in `internal/slidelist/slidelist.go`).
struct FakeCodeBlock {
    let block: Int
    let language: String
    let driver: String
    let live: Bool
    let line: Int

    var jsonObject: [String: Any] {
        ["block": block, "language": language, "driver": driver, "live": live, "line": line]
    }
}

/// One slide as `FakeSlideParser` reports it, in the same shape as tap's own answer.
struct FakeSlide {
    let number: Int
    let startLine: Int
    let endLine: Int
    let layout: String
    let title: String
    let fragments: Int
    let steps: Int
    let skip: Bool
    let errors: [String]
    let codeBlocks: [FakeCodeBlock]

    var jsonObject: [String: Any] {
        ["number": number, "startLine": startLine, "endLine": endLine, "layout": layout, "title": title,
         "fragments": fragments, "steps": steps, "skip": skip, "errors": errors, "codeBlocks": codeBlocks.map(\.jsonObject)]
    }
}

/// Follows fenced code blocks through markdown one line at a time, the same
/// rules `fenceTracker` in `internal/parser/fences.go` follows: a fence
/// opens with a run of at least three backticks or tildes, indented by at
/// most three spaces (a backtick fence's info string cannot itself hold a
/// backtick). It closes at a line that is a run of the same character, at
/// least as long as the opening run, indented by at most three spaces, with
/// only spaces after it. The zero value is outside any fence.
private struct FenceTracker {
    private var character: Character?
    private var length = 0

    /// Reads the next line and reports whether it belongs to a fenced code
    /// block, counting the opening and closing fence lines, and, when this
    /// line opens a fence, the fence's info string (the text after the
    /// opening run of fence characters).
    mutating func advance(_ line: String) -> (insideFence: Bool, opened: (character: Character, info: String)?) {
        let (runCharacter, run, rest, isRun) = FenceTracker.fenceRun(line)
        if length == 0 {
            guard isRun, !(runCharacter == "`" && rest.contains("`")) else {
                return (false, nil)
            }
            character = runCharacter
            length = run
            return (true, (runCharacter, rest))
        }
        if isRun, runCharacter == character, run >= length, rest.trimmingCharacters(in: .whitespaces).isEmpty {
            character = nil
            length = 0
        }
        return (true, nil)
    }

    /// Whether `line` starts, after at most three spaces, with a run of at
    /// least three backticks or three tildes. Returns the run's character
    /// and length, and the rest of the line after the run.
    private static func fenceRun(_ line: String) -> (character: Character, run: Int, rest: String, isRun: Bool) {
        let characters = Array(line)
        var indent = 0
        while indent < characters.count, characters[indent] == " " { indent += 1 }
        guard indent <= 3, indent < characters.count else { return (" ", 0, "", false) }
        let fenceCharacter = characters[indent]
        guard fenceCharacter == "`" || fenceCharacter == "~" else { return (" ", 0, "", false) }
        var run = 1
        while indent + run < characters.count, characters[indent + run] == fenceCharacter { run += 1 }
        guard run >= 3 else { return (" ", 0, "", false) }
        let rest = String(characters[(indent + run)...])
        return (fenceCharacter, run, rest, true)
    }
}

/// A small stand-in for tap's own slide parser, used only to answer
/// `FakeTap`'s `PUT /api/app/source`. It understands exactly what the
/// fixtures in `desktop/TapTests/Fixtures` need: YAML frontmatter (only
/// enough to notice unbalanced brackets and braces), `---` separators
/// aware of fenced code blocks, `<!-- layout: NAME -->` directives against
/// a known layout list, `# ` headings as titles, `<!-- pause -->` as a
/// fragment, and fenced code blocks (excluding ```component fences) with
/// an inline `{driver: NAME}` attribute marking them live.
enum FakeSlideParser {
    /// The full built-in layout registry, copied from
    /// `internal/layouts/layouts.json` in the tap repository. Keep this in
    /// sync with that file if it changes.
    static let knownLayouts: Set<String> = [
        "big-stat", "blank", "code-focus", "cover", "default", "quote",
        "section", "sidebar", "split-media", "three-column", "title", "two-column",
    ]

    /// Sorted the same way `internal/layouts/layouts.go`'s `loadRegistry`
    /// sorts `layoutNames`, so the error message's suffix matches real tap's.
    private static let sortedKnownLayouts = knownLayouts.sorted()

    static func parse(_ text: String) -> (slides: [FakeSlide], errors: [String]) {
        let lines = text.components(separatedBy: "\n")
        var contentStart = 0

        if lines.first == "---" {
            guard let closingIndex = (1..<lines.count).first(where: { lines[$0] == "---" }) else {
                return ([], ["the deck settings never close"])
            }
            let frontmatter = lines[1..<closingIndex].joined(separator: "\n")
            guard isBalanced(frontmatter) else {
                return ([], ["the deck settings do not parse"])
            }
            contentStart = closingIndex + 1
        }

        var segments: [[Int]] = []
        var current: [Int] = []
        var fences = FenceTracker()
        for index in contentStart..<lines.count {
            let (insideFence, _) = fences.advance(lines[index])
            if !insideFence, lines[index].trimmingCharacters(in: .whitespaces) == "---" {
                segments.append(current)
                current = []
            } else {
                current.append(index)
            }
        }
        segments.append(current)

        var slides: [FakeSlide] = []
        var number = 1
        for segment in segments {
            guard let slide = makeSlide(number: number, segment: segment, lines: lines) else { continue }
            slides.append(slide)
            number += 1
        }
        return (slides, [])
    }

    /// Builds one slide from a segment's 0-based line indices, trimmed of
    /// leading and trailing blank lines, or nil if nothing is left.
    private static func makeSlide(number: Int, segment: [Int], lines: [String]) -> FakeSlide? {
        var start = 0
        var end = segment.count - 1
        while start <= end, lines[segment[start]].trimmingCharacters(in: .whitespaces).isEmpty { start += 1 }
        while end >= start, lines[segment[end]].trimmingCharacters(in: .whitespaces).isEmpty { end -= 1 }
        guard start <= end else { return nil }
        let indices = Array(segment[start...end])

        var layout = ""
        var title = ""
        var fragments = 0
        var errors: [String] = []
        var codeBlocks: [FakeCodeBlock] = []
        var fences = FenceTracker()

        for lineIndex in indices {
            let line = lines[lineIndex]
            let trimmed = line.trimmingCharacters(in: .whitespaces)
            if let name = layoutDirective(trimmed) {
                layout = name
                if !knownLayouts.contains(name) {
                    errors.append("unknown layout \"\(name)\" (valid layouts: \(sortedKnownLayouts.joined(separator: ", ")))")
                }
            }
            if title.isEmpty, trimmed.hasPrefix("# "), !trimmed.hasPrefix("##") {
                title = String(trimmed.dropFirst(2)).trimmingCharacters(in: .whitespaces)
            }
            if trimmed.contains("<!-- pause -->") {
                fragments += 1
            }
            let (_, opened) = fences.advance(line)
            if let opened, !isComponentFence(opened.info) {
                codeBlocks.append(fence(info: opened.info, line: lineIndex + 1, block: codeBlocks.count))
            }
        }

        return FakeSlide(number: number, startLine: indices.first! + 1, endLine: indices.last! + 1, layout: layout, title: title,
                         fragments: fragments, steps: 0, skip: false, errors: errors, codeBlocks: codeBlocks)
    }

    private static func layoutDirective(_ line: String) -> String? {
        guard line.hasPrefix("<!-- layout:"), line.hasSuffix("-->") else { return nil }
        let inner = line.dropFirst("<!-- layout:".count).dropLast("-->".count)
        return inner.trimmingCharacters(in: .whitespaces)
    }

    /// Whether a fence's info string names a component fence, the same
    /// shape `componentFencePattern` in `internal/parser/codeblocks.go`
    /// matches: the word "component" followed by whitespace and a path.
    private static func isComponentFence(_ info: String) -> Bool {
        let trimmedInfo = info.trimmingCharacters(in: .whitespaces)
        guard trimmedInfo.hasPrefix("component") else { return false }
        let rest = trimmedInfo.dropFirst("component".count)
        guard let first = rest.first, first == " " || first == "\t" else { return false }
        return !rest.trimmingCharacters(in: .whitespaces).isEmpty
    }

    /// Parses a fence's info string, `` language {attribute: value, ...} ``,
    /// the text after the opening run of fence characters.
    private static func fence(info: String, line lineNumber: Int, block: Int) -> FakeCodeBlock {
        let language = String(info.prefix { $0 != " " && $0 != "{" })
        var driver = ""
        if let braceStart = info.firstIndex(of: "{"), let braceEnd = info.firstIndex(of: "}") {
            let attributes = info[info.index(after: braceStart)..<braceEnd]
            for pair in attributes.split(separator: ",") {
                let keyValue = pair.split(separator: ":", maxSplits: 1)
                if keyValue.count == 2, keyValue[0].trimmingCharacters(in: .whitespaces) == "driver" {
                    driver = keyValue[1].trimmingCharacters(in: .whitespaces)
                }
            }
        }
        return FakeCodeBlock(block: block, language: language, driver: driver, live: !driver.isEmpty, line: lineNumber)
    }

    /// Whether `[...]` and `{...}` pairs in `text` all close, the only
    /// check needed to tell `broken-frontmatter.md` from a deck whose
    /// frontmatter is fine.
    private static func isBalanced(_ text: String) -> Bool {
        var stack: [Character] = []
        let closers: [Character: Character] = ["]": "[", "}": "{"]
        for character in text {
            if character == "[" || character == "{" {
                stack.append(character)
            } else if let opener = closers[character] {
                guard stack.last == opener else { return false }
                stack.removeLast()
            }
        }
        return stack.isEmpty
    }
}
