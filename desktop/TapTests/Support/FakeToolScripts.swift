import Foundation
@testable import Tap

/// Stand-ins for the bundled tap's subcommands (`AppEnvironment.
/// toolExecutableURL`), while the deck's real `tap dev --app` keeps
/// running. Each script records its arguments in `record`, handles the
/// subcommands a test names, and hands every other one to the real tap,
/// so `tap theme list` and `tap slide list` stay real. `tap theme show`
/// is always scripted (the fixture PNG, unless the test names it), so no
/// hosted test starts Chromium for a theme render. Every script
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
        \(jsonStringFunction)
        echo "arguments: $@" >> "\(record.path)"
        echo "gemini: ${GEMINI_API_KEY:+set}" >> "\(record.path)"
        case "$1 $2" in
        \(body)
        \(try themeShowCase(png: fixturePNG, downloadLines: 0))
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
    static func themeShow(png: URL = fixturePNG, downloadLines: Int = 0, recordingTo record: URL) throws -> URL {
        try write(themeShowCase(png: png, downloadLines: downloadLines), recordingTo: record)
    }

    /// `text` in single quotes for sh: every character is literal, and a
    /// single quote in it closes, escapes and reopens the quoting.
    nonisolated static func shellQuoted(_ text: String) -> String {
        "'" + text.replacingOccurrences(of: "'", with: #"'\''"#) + "'"
    }

    /// `value` as JSON text, keys sorted.
    nonisolated static func jsonText(_ value: Any) -> String {
        guard let data = try? JSONSerialization.data(withJSONObject: value, options: [.sortedKeys, .withoutEscapingSlashes]) else { return "null" }
        return String(decoding: data, as: UTF8.self)
    }

    /// `json_string "$value"` prints the value as a JSON string, quotes
    /// included, with its backslashes and double quotes escaped: for a
    /// path the script only knows when it runs.
    static let jsonStringFunction = #"json_string() { printf '"%s"' "$(printf '%s' "$1" | sed -e 's/\\/\\\\/g' -e 's/"/\\"/g')"; }"#

    nonisolated static var fixturePNG: URL { Fixtures.repositoryRoot.appendingPathComponent("desktop/TapTests/Fixtures/diagram.png") }

    private static func themeShowCase(png: URL, downloadLines: Int) throws -> String {
        let folder = try Fixtures.temporaryFolder()
        let download = (0..<downloadLines).map { index in
            #"echo '{"phase":"download","bytes":\#((index + 1) * 50_000_000),"totalBytes":\#(downloadLines * 50_000_000)}' >&2; sleep 0.1"#
        }.joined(separator: "\n    ") + (downloadLines > 0 ? "\n    sleep 0.3" : "")
        return """
          "theme show")
            slug="$3"
            out="\(folder.path)/$slug.png"
            cp "\(png.path)" "$out"
            case "$*" in *"--progress json"*) \(download.isEmpty ? ":" : download); echo '{"phase":"render","done":1,"total":1}' >&2; echo "{\\"phase\\":\\"done\\",\\"ok\\":true,\\"slug\\":\\"$slug\\",\\"image\\":\\"$out\\",\\"cached\\":false}" >&2 ;; esac
            printf '{"ok": true, "slug": "%s", "image": "%s", "cached": false}\\n' "$slug" "$out"
            exit 0 ;;
        """
    }

    /// `tap theme set`: waits `before` seconds, the real tap sets the
    /// theme, then the script waits `after` seconds before it exits and
    /// records "finished", so a test can act between the save and tap's
    /// write, or between the write and the run's end.
    static func slowThemeSet(before: Double = 0, after: Double, recordingTo record: URL) throws -> URL {
        let realTap = AppEnvironment.shared.tapExecutableURL.path
        return try write("""
          "theme set")
            sleep \(before)
            "\(realTap)" "$@"
            status=$?
            sleep \(after)
            echo "finished" >> "\(record.path)"
            exit $status ;;
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

    /// `tap export pdf`: `download` lines first when `downloadLines` is set,
    /// a render line per slide with `secondsPerSlide` between them, the
    /// warnings for `broken`, then the done line and the file. It copies
    /// the deck file as it starts to `record` + ".deck", for what tap read.
    /// With `gate`, it waits after the first render line until that file
    /// exists, so a test sees the render state for as long as it needs.
    /// On SIGINT (which sh acts on once the current sleep ends) it prints
    /// `renderLinesAfterInterrupt` more render lines a tenth of a second
    /// apart, as tap finishes the slide in flight, then tap's interrupted
    /// done line, and exits 130, as tap does.
    static func exportPDF(slides: Int, secondsPerSlide: Double = 0, downloadLines: Int = 0, broken: [(slide: Int, message: String)] = [],
                          gate: URL? = nil, renderLinesAfterInterrupt: Int = 0, recordingTo record: URL) throws -> URL {
        let download = (0..<downloadLines).map { index in
            #"echo '{"phase":"download","bytes":\#((index + 1) * 50_000_000),"totalBytes":\#(downloadLines * 50_000_000)}' >&2; sleep 0.2"#
        }.joined(separator: "; ")
        let brokenJSON = jsonText(broken.map { ["slide": $0.slide, "message": $0.message] as [String: Any] })
        let warnings = broken.map { "printf '%s\\n' \(shellQuoted("warning: slide \($0.slide) shows an error card: \($0.message)")) >&2" }.joined(separator: "; ")
        let interruptRender = renderLinesAfterInterrupt == 0 ? ":" : (1...renderLinesAfterInterrupt).map { index in
            #"echo "{\"phase\":\"render\",\"done\":$((i + \#(index))),\"total\":\#(slides)}" >&2; sleep 0.1"#
        }.joined(separator: "; ")
        let gateWait = gate.map { "[ $i -eq 1 ] && while [ ! -e \(shellQuoted($0.path)) ]; do sleep 0.05; done" } ?? ":"
        // The done line goes out through printf's %s, so no character of the payload is the shell's to read.
        let doneLine = "printf '%s%s%s\\n' \(shellQuoted(#"{"phase":"done","ok":true,"output":"#)) \"$(json_string \"$out\")\" \(shellQuoted(#","pages":\#(slides),"bytes":14,"brokenSlides":\#(brokenJSON)}"#)) >&2"
        return try write("""
          "export pdf")
            interrupted() {
              \(interruptRender)
              echo '{"phase":"done","ok":false,"error":{"code":"interrupted","message":"interrupted"}}' >&2
              exit 130
            }
            trap interrupted INT
            cp "$3" "\(record.path).deck"
            out=""; while [ $# -gt 0 ]; do case "$1" in --output|-o) out="$2"; shift ;; esac; shift; done
            \(download.isEmpty ? ":" : download)
            i=1; while [ $i -le \(slides) ]; do
              echo "{\\"phase\\":\\"render\\",\\"done\\":$i,\\"total\\":\(slides)}" >&2
              \(gateWait)
              sleep \(secondsPerSlide); i=$((i + 1))
            done
            \(warnings.isEmpty ? ":" : warnings)
            printf 'not a real pdf' > "$out"
            \(doneLine)
            exit 0 ;;
        """, recordingTo: record)
    }
}
