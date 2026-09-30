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
    /// The five-minute watchdog: it checks each second that the script (or
    /// the tap it became through exec) is still running, and ends with it,
    /// so no watchdog outlives its run and none can reach a process that
    /// took the pid later. After five minutes it kills the run, if the pid
    /// still names a tap.
    static let selfKill = #"(i=0; while [ $i -lt 300 ] && kill -0 $$ 2>/dev/null; do sleep 1; i=$((i + 1)); done; [ $i -ge 300 ] && ps -p $$ -o command= | grep -q tap && kill -9 $$) </dev/null >/dev/null 2>&1 &"#

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

    /// `tap export images --all`: a render line per slide, the files, and,
    /// with `broken`, tap's `slide N: reason` lines and its broken_slides
    /// failure after the files that did land, as tap does. Every JSON line
    /// goes out through `printf`, never `echo`, so a message's own
    /// characters cannot be read as escapes.
    static func exportImages(slides: Int, broken: [Int] = [], recordingTo record: URL) throws -> URL {
        let brokenList = broken.map(String.init).joined(separator: " ")
        return try write("""
          "export images")
            out=""; while [ $# -gt 0 ]; do case "$1" in --output|-o) out="$2"; shift ;; esac; shift; done
            mkdir -p "$out"; files=""
            i=1; while [ $i -le \(slides) ]; do
              printf '{"phase":"render","done":%s,"total":%s}\\n' "$i" "\(slides)" >&2
              case " \(brokenList) " in
                *" $i "*) printf 'slide %s: %s\\n' "$i" "an error card" >&2 ;;
                *) f=$(printf '%s/slide-%03d.png' "$out" "$i"); printf 'png' > "$f"; files="$files$(json_string "$f"),"  ;;
              esac
              i=$((i + 1))
            done
            files="[${files%,}]"
            if [ -n "\(brokenList)" ]; then
              printf '{"phase":"done","ok":false,"error":{"code":"broken_slides","message":"%s slide(s) failed to capture"}}\\n' "\(broken.count)" >&2
              exit 1
            fi
            printf '{"phase":"done","ok":true,"files":%s}\\n' "$files" >&2
            exit 0 ;;
        """, recordingTo: record)
    }
}
