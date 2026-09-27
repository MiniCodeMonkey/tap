import Foundation
@testable import Tap

/// Stand-ins for the bundled tap's subcommands (`AppEnvironment.
/// toolExecutableURL`), while the deck's real `tap dev --app` keeps
/// running. Each script records its arguments in `record`, handles the
/// subcommands a test names, and hands every other one to the real tap,
/// so `tap theme list` and `tap slide list` stay real. Every script
/// kills itself after five minutes, so a test that never waits leaves
/// nothing running. A fake never records the value of an environment
/// variable: `${GEMINI_API_KEY:+set}` records the word set.
@MainActor
enum FakeToolScripts {
    static let selfKill = "(sleep 300; kill -9 $$) </dev/null >/dev/null 2>&1 &"

    static func write(_ body: String, recordingTo record: URL) throws -> URL {
        let url = try Fixtures.temporaryFolder().appendingPathComponent("tap")
        let realTap = AppEnvironment.shared.tapExecutableURL.path
        try """
        #!/bin/sh
        \(selfKill)
        echo "arguments: $@" >> "\(record.path)"
        echo "gemini: ${GEMINI_API_KEY:+set}" >> "\(record.path)"
        case "$1 $2" in
        \(body)
          *) exec "\(realTap)" "$@" ;;
        esac
        """.write(to: url, atomically: true, encoding: .utf8)
        try FileManager.default.setAttributes([.posixPermissions: 0o755], ofItemAtPath: url.path)
        return url
    }

    /// `tap theme show <slug> --image ...`: copies `png` to a file named
    /// after the slug and prints tap's result, with `downloadLines` of
    /// download progress first when `--progress json` is given, a tenth of
    /// a second apart and a third of a second before the render line, so
    /// the download state lasts long enough for a test's poll to see it.
    static func themeShow(png: URL, downloadLines: Int = 0, recordingTo record: URL) throws -> URL {
        let folder = try Fixtures.temporaryFolder()
        let download = (0..<downloadLines).map { index in
            #"echo '{"phase":"download","bytes":\#((index + 1) * 50_000_000),"totalBytes":\#(downloadLines * 50_000_000)}' >&2; sleep 0.1"#
        }.joined(separator: "\n    ") + (downloadLines > 0 ? "\n    sleep 0.3" : "")
        return try write("""
          "theme show")
            slug="$3"
            out="\(folder.path)/$slug.png"
            cp "\(png.path)" "$out"
            case "$*" in *"--progress json"*) \(download.isEmpty ? ":" : download); echo '{"phase":"render","done":1,"total":1}' >&2; echo "{\\"phase\\":\\"done\\",\\"ok\\":true,\\"slug\\":\\"$slug\\",\\"image\\":\\"$out\\",\\"cached\\":false}" >&2 ;; esac
            printf '{"ok": true, "slug": "%s", "image": "%s", "cached": false}\\n' "$slug" "$out"
            exit 0 ;;
        """, recordingTo: record)
    }

    /// `tap theme set <slug> <deck>`: fails with tap's unknown_theme error, for the error path.
    static func themeSetFailing(recordingTo record: URL) throws -> URL {
        try write("""
          "theme set")
            printf '{"ok": false, "error": {"code": "unknown_theme", "message": "unknown theme \\\\"%s\\\\": valid themes are base, terminal"}}\\n' "$3"
            exit 1 ;;
        """, recordingTo: record)
    }
}
