import XCTest
@testable import TapDesktopCore

final class WelcomeLogicTests: XCTestCase {
    func testAnEmptyQueryMatchesEverything() {
        XCTAssertTrue(RecentDeckMatcher.matches(name: "a.md", path: "~/x", query: ""))
        XCTAssertTrue(RecentDeckMatcher.matches(name: "a.md", path: "~/x", query: "   "))
    }

    func testEveryWordMustMatchTheNameOrThePathIgnoringCaseAndAccents() {
        XCTAssertTrue(RecentDeckMatcher.matches(name: "slides.md", path: "~/Talks/Café", query: "TALKS cafe"))
        XCTAssertTrue(RecentDeckMatcher.matches(name: "slides.md", path: "~/Talks/spx", query: "sli"))
        XCTAssertFalse(RecentDeckMatcher.matches(name: "slides.md", path: "~/Talks/spx", query: "slides laracon"))
    }

    func testDateLabels() {
        var calendar = Calendar(identifier: .gregorian)
        calendar.timeZone = TimeZone(identifier: "UTC")!
        let locale = Locale(identifier: "en_US")
        let now = calendar.date(from: DateComponents(year: 2026, month: 9, day: 30, hour: 12))!
        func label(_ year: Int, _ month: Int, _ day: Int, _ hour: Int = 9) -> String {
            RecentDeckDate.label(for: calendar.date(from: DateComponents(year: year, month: month, day: day, hour: hour))!, now: now, calendar: calendar, locale: locale)
        }
        XCTAssertEqual(label(2026, 9, 30, 1), "Today")
        XCTAssertEqual(label(2026, 9, 29, 23), "Yesterday")
        XCTAssertEqual(label(2026, 9, 24), "Sep 24")
        XCTAssertEqual(label(2025, 12, 31), "Dec 31, 2025")
    }

    func testTheFilmstripOffsetWrapsAtItsPeriod() {
        let loop = FilmstripLoop(cardCount: 21)
        XCTAssertEqual(loop.period, 21 * 188)
        XCTAssertEqual(loop.offset(afterSeconds: 0), 0)
        XCTAssertEqual(loop.offset(afterSeconds: 45), loop.period / 2, accuracy: 0.0001)
        XCTAssertEqual(loop.offset(afterSeconds: 90), 0, accuracy: 0.0001, "one cycle is one row, so the wrap is seamless")
        XCTAssertEqual(loop.offset(afterSeconds: 135), loop.period / 2, accuracy: 0.0001)
        XCTAssertEqual(loop.advance(loop.period - 1, bySeconds: 90 / loop.period), 0, accuracy: 0.0001)
        XCTAssertEqual(loop.advance(10, bySeconds: -90 / loop.period * 20), loop.period - 10, accuracy: 0.0001, "a backward step wraps into range")
        XCTAssertEqual(FilmstripLoop(cardCount: 0).offset(afterSeconds: 10), 0)
    }

    func testTheFilmstripRepeatsUntilTheWindowIsCovered() {
        XCTAssertEqual(FilmstripLoop(cardCount: 21).copiesNeeded(visibleWidth: 880), 2)
        XCTAssertEqual(FilmstripLoop(cardCount: 2).copiesNeeded(visibleWidth: 880), 4, "two cards are 752 wide: covering 880 at any offset takes four copies")
        XCTAssertEqual(FilmstripLoop(cardCount: 0).copiesNeeded(visibleWidth: 880), 0)
    }

    func testTheAuroraRisesFromItsOpeningHeightToRest() {
        let resting = AuroraTimeline.restingHeight(isDark: false)
        XCTAssertEqual(resting, 0.34)
        XCTAssertEqual(AuroraTimeline.restingHeight(isDark: true), 0.30)
        XCTAssertEqual(AuroraTimeline.height(elapsed: 0, resting: resting, reduceMotion: false), 0.08, accuracy: 1e-9)
        XCTAssertEqual(AuroraTimeline.height(elapsed: 2.2, resting: resting, reduceMotion: false), resting, accuracy: 1e-9)
        XCTAssertEqual(AuroraTimeline.height(elapsed: 99, resting: resting, reduceMotion: false), resting, accuracy: 1e-9)
        let middle = AuroraTimeline.height(elapsed: 1.1, resting: resting, reduceMotion: false)
        XCTAssertGreaterThan(middle, 0.08 + (resting - 0.08) / 2, "ease-out is past halfway at half the time")
        XCTAssertEqual(AuroraTimeline.height(elapsed: 0, resting: resting, reduceMotion: true), resting, "Reduce Motion starts at rest")
    }

    func testTheAuroraFadesInAndBoostsSmoothly() {
        XCTAssertEqual(AuroraTimeline.opacity(elapsed: 0, reduceMotion: false), 0)
        XCTAssertEqual(AuroraTimeline.opacity(elapsed: 0.8, reduceMotion: false), 0.5, accuracy: 1e-9)
        XCTAssertEqual(AuroraTimeline.opacity(elapsed: 5, reduceMotion: false), 1)
        XCTAssertEqual(AuroraTimeline.opacity(elapsed: 0, reduceMotion: true), 1)
        XCTAssertEqual(AuroraTimeline.restingIntensity(isDark: false), 1.06)
        XCTAssertEqual(AuroraTimeline.restingIntensity(isDark: true), 1.25)

        var boost = 1.0
        for _ in 0..<300 { boost = AuroraTimeline.boost(current: boost, dragging: true, deltaSeconds: 1.0 / 60) }
        XCTAssertEqual(boost, 1.6)
        let oneStep = AuroraTimeline.boost(current: 1, dragging: true, deltaSeconds: 1.0 / 60)
        XCTAssertTrue(oneStep > 1 && oneStep < 1.6, "the boost eases in")
        var relaxed = 1.6
        for _ in 0..<300 { relaxed = AuroraTimeline.boost(current: relaxed, dragging: false, deltaSeconds: 1.0 / 60) }
        XCTAssertEqual(relaxed, 1)
    }
}
