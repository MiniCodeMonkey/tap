import Foundation

/// Shell scripts that behave like `tap dev --app`.
enum FakeTap {
    /// Prints a ready line, records its arguments, its FROM_LOGIN_SHELL
    /// variable and its stdin into `record`, and exits when stdin closes.
    static func ready(recordingTo record: URL, port: Int = 4242) throws -> URL {
        try TestScripts.make("""
        echo "arguments: $@" >> "\(record.path)"
        echo "FROM_LOGIN_SHELL=$FROM_LOGIN_SHELL" >> "\(record.path)"
        echo '{"type":"ready","port":\(port),"token":"token","launch":"launch"}'
        echo "listening on 127.0.0.1:\(port)" >&2
        while IFS= read -r line; do echo "stdin: $line" >> "\(record.path)"; done
        exit 0
        """)
    }

    /// Prints a ready line and a panic, then exits with status 2.
    static func crashing() throws -> URL {
        try TestScripts.make("""
        echo '{"type":"ready","port":4242,"token":"token","launch":"launch"}'
        echo "panic: runtime error: index out of range" >&2
        echo "internal/parser/slots.go:88" >&2
        exit 2
        """)
    }

    /// Prints one error event and exits with status 1, as tap does when it cannot start.
    static func failing(code: String, message: String) throws -> URL {
        try TestScripts.make("""
        echo '{"type":"error","code":"\(code)","message":"\(message)"}'
        exit 1
        """)
    }

    /// Prints a deck-problems event with one error and exits with status 1,
    /// as tap does for a deck whose settings it cannot render. Every start
    /// is counted in `record`.
    static func rejectingTheDeck(recordingTo record: URL) throws -> URL {
        try TestScripts.make("""
        echo "started" >> "\(record.path)"
        echo '{"type":"deck-problems","problems":[{"key":"aspectRatio","value":"16/9","message":"The aspect ratio bad.","severity":"error","suggestions":["16:9"],"allowed":["16:9","4:3","16:10"]}]}'
        echo '{"type":"error","code":"invalid_deck","message":"invalid config: invalid aspectRatio"}'
        echo "Error: invalid config" >&2
        exit 1
        """)
    }

    /// Never prints a ready line.
    static func silent() throws -> URL {
        try TestScripts.make("while true; do sleep 0.1; done")
    }
}
