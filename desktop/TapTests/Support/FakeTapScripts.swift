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
}
