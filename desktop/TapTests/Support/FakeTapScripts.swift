import Foundation

/// A stand-in for the bundled tap that prints a ready line, a panic, and exits.
enum FakeTapScripts {
    static func crashing() throws -> URL {
        let url = try Fixtures.temporaryFolder().appendingPathComponent("tap")
        try """
        #!/bin/sh
        echo '{"type":"ready","port":1,"token":"token","launch":"launch"}'
        echo "panic: runtime error: index out of range" >&2
        echo "internal/parser/slots.go:88" >&2
        exit 2
        """.write(to: url, atomically: true, encoding: .utf8)
        try FileManager.default.setAttributes([.posixPermissions: 0o755], ofItemAtPath: url.path)
        return url
    }

    /// Prints a ready line for `port`, then reads stdin until it closes,
    /// like a tap that is up but has no server of its own: on port 1 the
    /// talk's pages cannot load, and on a port a test listens on without
    /// ever answering they load forever.
    static func readyAndWaiting(port: Int = 1) throws -> URL {
        let url = try Fixtures.temporaryFolder().appendingPathComponent("tap")
        try """
        #!/bin/sh
        echo '{"type":"ready","port":\(port),"token":"token","launch":"launch","presenter":"presenter"}'
        while IFS= read -r line; do :; done
        exit 0
        """.write(to: url, atomically: true, encoding: .utf8)
        try FileManager.default.setAttributes([.posixPermissions: 0o755], ofItemAtPath: url.path)
        return url
    }

    /// Prints one error event and exits with status 1, as tap does when it cannot start.
    static func failing(code: String, message: String) throws -> URL {
        let url = try Fixtures.temporaryFolder().appendingPathComponent("tap")
        try """
        #!/bin/sh
        echo '{"type":"error","code":"\(code)","message":"\(message)"}'
        exit 1
        """.write(to: url, atomically: true, encoding: .utf8)
        try FileManager.default.setAttributes([.posixPermissions: 0o755], ofItemAtPath: url.path)
        return url
    }

    /// Prints a ready line, then ignores both its closed stdin and
    /// SIGTERM: only the SIGKILL at the end of a stop's escalation ends it.
    static func readyAndDeafToQuit() throws -> URL {
        let url = try Fixtures.temporaryFolder().appendingPathComponent("tap")
        try """
        #!/bin/sh
        trap '' TERM
        echo '{"type":"ready","port":1,"token":"token","launch":"launch","presenter":"presenter"}'
        while :; do sleep 1; done
        """.write(to: url, atomically: true, encoding: .utf8)
        try FileManager.default.setAttributes([.posixPermissions: 0o755], ofItemAtPath: url.path)
        return url
    }

    /// What the scripted tap present does with a quit command.
    enum QuitBehavior {
        /// Exits at once, as a run with no recording does.
        case exit
        /// Asks keep-recording and exits on the answer.
        case askToKeep(directory: URL, segments: Int)
        /// Asks keep-recording and exits after `seconds` whatever comes, as
        /// tap does once its 60 s wait is over (and at once when its stdin
        /// closes). The fake's wait is short so the test is not.
        case askToKeepThenExit(after: TimeInterval, directory: URL, segments: Int)
    }

    /// A 1 by 1 PNG, base64: the smallest QR code a fake can send.
    static let onePixelPNG = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNkYPhfDwAChwGA60e6kgAAAABJRU5ErkJggg=="

    /// A scripted `tap present --app`: it records its arguments and every
    /// stdin line in `record`, prints a ready line (with no server behind
    /// it), then `events` one per line, and answers commands the way tap
    /// does: a tunnel start with a running tunnel (or, with
    /// `tunnelUnavailable`, the error tap sends without cloudflared; or,
    /// with `tunnelFailed`, the `tunnel_failed` error followed by the
    /// stopped tunnel event tap sends after a failed start), a tunnel stop
    /// with a stopped tunnel, a recording stop with a stopped recording, a
    /// new segment with segment 2 recording, and quit as `quit` says.
    static func presenting(events: [String], quit: QuitBehavior = .exit, tunnelFailed: Bool = false, tunnelUnavailable: Bool = false,
                           recordingTo record: URL) throws -> URL {
        let url = try Fixtures.temporaryFolder().appendingPathComponent("tap")
        let eventLines = events.map { "echo '\($0)'" }.joined(separator: "\n")
        let tunnelRunning: String
        if tunnelUnavailable {
            tunnelRunning = #"echo '{"type":"error","code":"tunnel_unavailable","message":"the tunnel needs cloudflared: brew install cloudflared"}'"#
        } else if tunnelFailed {
            tunnelRunning = #"echo '{"type":"tunnel","state":"starting"}'; echo '{"type":"error","code":"tunnel_failed","message":"cloudflared exited: connection refused"}'; echo '{"type":"tunnel","state":"stopped"}'"#
        } else {
            tunnelRunning = #"echo '{"type":"tunnel","state":"starting"}'; echo '{"type":"tunnel","state":"running","url":"https://stark-lake-1234.trycloudflare.com","qr":"\#(onePixelPNG)"}'"#
        }
        let onQuit: String
        switch quit {
        case .exit:
            onQuit = "exit 0"
        case .askToKeep(let directory, let segments):
            onQuit = #"echo '{"type":"question","id":"q1","kind":"keep-recording","payload":{"directory":"\#(directory.path)","segments":\#(segments)}}'"#
        case .askToKeepThenExit(let seconds, let directory, let segments):
            onQuit = #"echo '{"type":"question","id":"q1","kind":"keep-recording","payload":{"directory":"\#(directory.path)","segments":\#(segments)}}'; sleep \#(seconds); exit 0"#
        }
        try """
        #!/bin/sh
        echo "arguments: $@" >> "\(record.path)"
        echo '{"type":"ready","port":1,"token":"token","launch":"launch","presenter":"presenter"}'
        \(eventLines)
        while IFS= read -r line; do
          echo "stdin: $line" >> "\(record.path)"
          case "$line" in
            *'"type":"quit"'*) \(onQuit) ;;
            *'"type":"answer"'*) exit 0 ;;
            *'"type":"tunnel","start":true'*) \(tunnelRunning) ;;
            *'"type":"tunnel","start":false'*) echo '{"type":"tunnel","state":"stopped"}' ;;
            *'"action":"stop"'*) echo '{"type":"recording","state":"stopped","segment":1,"elapsed":0,"disk":"ok"}' ;;
            *'"action":"new-segment"'*) echo '{"type":"recording","state":"recording","segment":2,"elapsed":0,"disk":"ok"}' ;;
          esac
        done
        exit 0
        """.write(to: url, atomically: true, encoding: .utf8)
        try FileManager.default.setAttributes([.posixPermissions: 0o755], ofItemAtPath: url.path)
        return url
    }
}
