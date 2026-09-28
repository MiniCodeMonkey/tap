import XCTest
@testable import Tap

/// The bounds a benchmark run is held to, chosen by where it runs: the
/// TAP_BENCH_BOUNDS environment variable, which `make -C desktop bench
/// BENCH_BOUNDS=ci` hands the test host through xcodebuild's TEST_RUNNER_
/// prefix. The developer bounds are the product's targets, measured on an
/// M-series Mac. The CI bounds are for the shared macos-15 runner, which is
/// slower and noisier; each is the runner's worst result over its recent
/// history plus a margin, and catches a regression of the size its comment
/// states rather than any slowdown at all.
struct BenchmarkBounds {
    let name: String
    /// The median, in milliseconds, from a key to the frame that shows it
    /// in the preview.
    let previewShownMedian: Double
    /// The 95th percentile, in milliseconds, from a key to the editor's
    /// frame that shows it.
    let typingP95: Double
    /// The median of the same samples, or nil where the bounds hold no
    /// median.
    let typingMedian: Double?

    /// 13-performance.feature's targets.
    static let developer = BenchmarkBounds(name: "developer", previewShownMedian: 200, typingP95: 16, typingMedian: nil)

    /// Runs 2026-09-25 to 2026-09-28 on the runner (87 jobs, 54 of which
    /// measured a median) found a preview median of at most 352.8 ms
    /// (typically about 255), a typing p95 of at most 25.3 ms (typically
    /// about 17) and a typing median of at most 12.1 ms. A preview about
    /// 1.6 times slower than typical, or a keystroke that costs 13 ms more
    /// at the tail or 8 ms more at the median, fails these in about half of
    /// the runs; twice as slow a preview, or 16 ms more per keystroke, in
    /// nearly all.
    static let continuousIntegration = BenchmarkBounds(name: "ci", previewShownMedian: 400, typingP95: 30, typingMedian: 16)

    /// The bounds TAP_BENCH_BOUNDS names; the developer bounds when it is
    /// not set. A name it does not know is nil, which fails the run rather
    /// than quietly holding it to other bounds.
    static func named(_ name: String?) -> BenchmarkBounds? {
        switch name {
        case nil, "", "developer": return developer
        case "ci": return continuousIntegration
        default: return nil
        }
    }
}

/// A benchmark on the generated 200-slide deck, with a component on every slide.
@MainActor
class BenchmarkCase: XCTestCase {
    /// The bounds this run is held to (see BenchmarkBounds). The name is
    /// printed, so a CI log shows which bounds judged the numbers.
    func bounds() throws -> BenchmarkBounds {
        let name = ProcessInfo.processInfo.environment["TAP_BENCH_BOUNDS"]
        let bounds = try XCTUnwrap(BenchmarkBounds.named(name), "TAP_BENCH_BOUNDS=\(name ?? "") names no bounds; use developer or ci")
        print("benchmark bounds: \(bounds.name)")
        return bounds
    }

    var repositoryRoot: URL {
        URL(fileURLWithPath: #filePath)
            .deletingLastPathComponent() // Support
            .deletingLastPathComponent() // TapBenchmarks
            .deletingLastPathComponent() // desktop
            .deletingLastPathComponent()
    }

    override func tearDown() async throws {
        for document in NSDocumentController.shared.documents { document.close() }
        try await Task.sleep(nanoseconds: 200_000_000)
    }

    // condition is called across await points inside the loop below, which
    // this toolchain only allows a closure parameter to do when it is
    // escaping; a non-escaping parameter fails to build here with "escaping
    // local function captures non-escaping value" (see HostedTestCase.swift).
    func waitUntil(timeout: TimeInterval, _ message: String, _ condition: @escaping () -> Bool) async throws {
        let deadline = Date().addingTimeInterval(timeout)
        while !condition() {
            if Date() > deadline {
                XCTFail("timed out waiting for \(message)")
                throw CancellationError()
            }
            try await Task.sleep(nanoseconds: 20_000_000)
        }
    }

    /// Opens a copy of the 200-slide deck and waits for tap's first answer.
    func openStressDeck() async throws -> DeckDocument {
        let folder = FileManager.default.temporaryDirectory.appendingPathComponent("tap-bench-\(UUID().uuidString)")
        try FileManager.default.copyItem(at: repositoryRoot.appendingPathComponent("desktop/TapBenchmarks/Decks/stress-200"), to: folder)
        let (document, _) = try await NSDocumentController.shared.openDocument(withContentsOf: folder.appendingPathComponent("deck.md"), display: true)
        let deck = try XCTUnwrap(document as? DeckDocument)
        let window = try XCTUnwrap(deck.windowControllers.first?.window)
        NSApp.activate(ignoringOtherApps: true)
        window.makeKeyAndOrderFront(nil)
        try await waitUntil(timeout: 120, "200 boxes") { deck.sessionController?.editor.boxes.count == 200 }
        return deck
    }

    @discardableResult
    func send(key characters: String, code: UInt16, to window: NSWindow) -> TimeInterval {
        let timestamp = ProcessInfo.processInfo.systemUptime
        for type in [NSEvent.EventType.keyDown, .keyUp] {
            if let event = NSEvent.keyEvent(with: type, location: .zero, modifierFlags: [], timestamp: timestamp,
                                            windowNumber: window.windowNumber, context: nil, characters: characters,
                                            charactersIgnoringModifiers: characters, isARepeat: false, keyCode: code) {
                window.sendEvent(event)
            }
        }
        return timestamp
    }

    func summarize(_ values: [Double]) -> [String: Double] {
        guard !values.isEmpty else { return ["count": 0] }
        let sorted = values.sorted()
        func percentile(_ fraction: Double) -> Double {
            let rank = fraction * Double(sorted.count - 1)
            let low = Int(rank.rounded(.down))
            let high = Int(rank.rounded(.up))
            return sorted[low] + (sorted[high] - sorted[low]) * (rank - Double(low))
        }
        func rounded(_ value: Double) -> Double { (value * 100).rounded() / 100 }
        return ["count": Double(sorted.count), "median": rounded(percentile(0.5)), "p95": rounded(percentile(0.95)),
                "min": rounded(sorted.first!), "max": rounded(sorted.last!)]
    }

    /// Writes `results` to desktop/build/benchmarks/`name`.json, and prints
    /// them on one line, so a CI log keeps every run's numbers, a passing
    /// run's too, and the bounds can be checked against the runner's
    /// history. Every number is rounded to 2 decimals first, so the log
    /// shows 296.78 rather than a Double's binary tail like
    /// 296.77999999999997.
    func write(_ results: [String: Any], to name: String) {
        let folder = repositoryRoot.appendingPathComponent("desktop/build/benchmarks")
        try? FileManager.default.createDirectory(at: folder, withIntermediateDirectories: true)
        let rounded = roundedForLogging(results)
        if let data = try? JSONSerialization.data(withJSONObject: rounded, options: [.prettyPrinted, .sortedKeys]) {
            try? data.write(to: folder.appendingPathComponent("\(name).json"))
        }
        if let line = try? JSONSerialization.data(withJSONObject: rounded, options: [.sortedKeys]) {
            print("benchmark results \(name): \(String(decoding: line, as: UTF8.self))")
        }
    }

    /// Rounds every Double in `value` to 2 decimals, recursing into nested
    /// dictionaries, so the JSON `write` emits never carries a rounded
    /// number's binary imprecision.
    private func roundedForLogging(_ value: Any) -> Any {
        switch value {
        case let number as Double:
            return (number * 100).rounded() / 100
        case let dictionary as [String: Any]:
            return dictionary.mapValues { roundedForLogging($0) }
        default:
            return value
        }
    }
}

/// Times each key event from the event's timestamp to the run loop pass,
/// after the Core Animation commit, in which the keystroke is in the
/// editor's text: that commit is the frame that shows it. A sample opens
/// when the key is sent and closes in the first such pass, so nothing the
/// benchmark does after sending the key, such as waiting to check the text,
/// is inside it. A pass in which the keystroke has not landed yet leaves
/// its sample open for a later one.
@MainActor
final class KeyLatencyRecorder {
    private struct Sample {
        let eventTimestamp: TimeInterval
        let hasLanded: () -> Bool
    }

    private var pending: [Sample] = []
    private var observer: CFRunLoopObserver?
    private(set) var milliseconds: [Double] = []

    /// Samples opened and not yet closed by a frame commit.
    var openSampleCount: Int { pending.count }

    func install() {
        // The highest order runs this after Core Animation's own
        // before-waiting observer, which commits the frame.
        observer = CFRunLoopObserverCreateWithHandler(nil, CFRunLoopActivity.beforeWaiting.rawValue, true, CFIndex.max) { [weak self] _, _ in
            MainActor.assumeIsolated { self?.flush() }
        }
        CFRunLoopAddObserver(CFRunLoopGetMain(), observer, .commonModes)
    }

    func uninstall() {
        if let observer { CFRunLoopRemoveObserver(CFRunLoopGetMain(), observer, .commonModes) }
        observer = nil
    }

    /// Opens a sample for a key event sent at `eventTimestamp`. `hasLanded`
    /// says whether the keystroke is in the editor's text yet.
    func record(eventTimestamp: TimeInterval, hasLanded: @escaping () -> Bool) {
        pending.append(Sample(eventTimestamp: eventTimestamp, hasLanded: hasLanded))
    }

    private func flush() {
        guard !pending.isEmpty else { return }
        // The time is read before hasLanded runs, so checking the text is
        // never part of a sample.
        let now = ProcessInfo.processInfo.systemUptime
        var stillOpen: [Sample] = []
        for sample in pending {
            if sample.hasLanded() {
                milliseconds.append((now - sample.eventTimestamp) * 1000)
            } else {
                stillOpen.append(sample)
            }
        }
        pending = stillOpen
    }
}
