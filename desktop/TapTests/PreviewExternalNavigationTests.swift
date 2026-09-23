import XCTest
import WebKit
@testable import Tap

/// A deck's own page must never be able to make the app open the system
/// browser on its own. The only navigation worth handing outside the
/// preview is a person clicking a plain web link; anything a page does to
/// itself, no click involved, is cancelled and goes nowhere.
///
/// `PreviewViewController.isExternalWebLink` is the one place both WebKit
/// delegate methods ask this question, and it is tested directly here:
/// constructing a real `WKNavigationAction` with a given `navigationType` is
/// not possible (WebKit hands out no public initializer), but the
/// `WKNavigationType` values it carries are plain enum cases and need none.
/// The two hosted tests below then prove the real delegate methods, wired to
/// a real `WKWebView`, actually reach that function: a page's own redirect
/// and a page's own `window.open` both arrive as `.other`, never a click.
final class PreviewExternalNavigationTests: HostedTestCase {
    func testALinkActivationToAPlainWebURLIsExternal() {
        XCTAssertTrue(PreviewViewController.isExternalWebLink(
            url: URL(string: "https://example.com/")!, navigationType: .linkActivated))
        XCTAssertTrue(PreviewViewController.isExternalWebLink(
            url: URL(string: "http://example.com/")!, navigationType: .linkActivated))
    }

    func testANavigationThatIsNotALinkActivationIsNeverExternal() {
        for navigationType: WKNavigationType in [.other, .formSubmitted, .backForward, .reload, .formResubmitted] {
            XCTAssertFalse(PreviewViewController.isExternalWebLink(url: URL(string: "https://example.com/")!, navigationType: navigationType),
                            "\(navigationType) must not be treated as a person clicking a link")
        }
    }

    /// A click is not enough on its own: a hostile deck can point a real,
    /// clicked link at something other than the web, and none of it goes
    /// anywhere outside the sandboxed preview either. `file:` reaches local
    /// files, `javascript:` runs in the page's own origin outside the
    /// sandbox this method exists to enforce, and an app-registered scheme
    /// hands control to a third program the person never chose to involve.
    func testAClickedLinkToAnythingOtherThanPlainWebIsNotExternal() {
        for scheme in ["file", "javascript", "mailto", "com.example.otherapp"] {
            XCTAssertFalse(PreviewViewController.isExternalWebLink(
                url: URL(string: "\(scheme)://something")!, navigationType: .linkActivated),
                "a clicked \(scheme): link must not reach the system browser")
        }
    }

    func makePreview() -> PreviewViewController {
        let controller = PreviewViewController()
        _ = controller.view
        return controller
    }

    func waitForPageLoad(_ controller: PreviewViewController) async {
        let deadline = Date().addingTimeInterval(5)
        while Date() < deadline {
            if await controller.pageValue("document.readyState").contains("complete") { return }
            try? await Task.sleep(nanoseconds: 50_000_000)
        }
    }

    /// A page redirecting its own top frame, with nobody having clicked
    /// anything, must not reach the system browser. This drives the real
    /// `decidePolicyFor` delegate method with a real `WKWebView` navigation,
    /// which WebKit classifies `.other`, not `.linkActivated`.
    func testAPagesOwnRedirectDoesNotReachTheSystemBrowser() async throws {
        let controller = makePreview()
        var opened: [URL] = []
        controller.openExternally = { opened.append($0) }
        controller.webView.loadHTMLString("<html></html>", baseURL: URL(string: "http://127.0.0.1:9999/"))
        await waitForPageLoad(controller)

        _ = try? await controller.webView.evaluateJavaScript("window.location.href = 'https://evil.example/'")
        try await waitUntil(timeout: 5, "the redirected navigation to be cancelled") {
            controller.webView.url?.absoluteString != "https://evil.example/"
        }
        // Give any (wrongly triggered) external open a moment to land before asserting its absence.
        try await Task.sleep(nanoseconds: 300_000_000)

        XCTAssertTrue(opened.isEmpty, "a page redirecting itself must not reach the system browser, but opened \(opened)")
    }

    /// A page opening a new window itself, with nobody clicking anything,
    /// must not reach the system browser either. This drives the real
    /// `createWebViewWith` delegate method the same way.
    func testAPageOpeningAWindowItselfDoesNotReachTheSystemBrowser() async throws {
        let controller = makePreview()
        var opened: [URL] = []
        controller.openExternally = { opened.append($0) }
        controller.webView.loadHTMLString("<html></html>", baseURL: URL(string: "http://127.0.0.1:9999/"))
        await waitForPageLoad(controller)

        _ = try? await controller.webView.evaluateJavaScript("window.open('https://evil.example/')")
        try await Task.sleep(nanoseconds: 300_000_000)

        XCTAssertTrue(opened.isEmpty, "a page opening its own window must not reach the system browser, but opened \(opened)")
    }
}
