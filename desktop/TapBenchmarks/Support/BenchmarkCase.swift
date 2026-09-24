import XCTest
@testable import Tap

/// A benchmark on the generated 200-slide deck, with a component on every slide.
@MainActor
class BenchmarkCase: XCTestCase {
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

    func write(_ results: [String: Any], to name: String) {
        let folder = repositoryRoot.appendingPathComponent("desktop/build/benchmarks")
        try? FileManager.default.createDirectory(at: folder, withIntermediateDirectories: true)
        if let data = try? JSONSerialization.data(withJSONObject: results, options: [.prettyPrinted, .sortedKeys]) {
            try? data.write(to: folder.appendingPathComponent("\(name).json"))
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
