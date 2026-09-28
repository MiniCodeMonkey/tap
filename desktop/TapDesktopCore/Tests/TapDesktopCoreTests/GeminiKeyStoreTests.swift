import XCTest
@testable import TapDesktopCore

final class GeminiKeyStoreTests: XCTestCase {
    func testTheShellsKeyWinsOverTheKeychains() {
        XCTAssertEqual(GeminiKeySource.resolve(shellValue: "from-shell", storedKey: "from-keychain"), .shell)
        XCTAssertEqual(GeminiKeySource.resolve(shellValue: nil, storedKey: "from-keychain"), .keychain)
        XCTAssertEqual(GeminiKeySource.resolve(shellValue: "", storedKey: "from-keychain"), .keychain, "an empty shell value is no key")
        XCTAssertEqual(GeminiKeySource.resolve(shellValue: nil, storedKey: ""), .none, "an empty stored key is none")
        XCTAssertEqual(GeminiKeySource.resolve(shellValue: nil, storedKey: nil), .none)
    }

    func testTheEnvironmentGetsTheKeyOnlyWhenTheShellHasNone() throws {
        let store = MemoryGeminiKeyStore(key: "placeholder-not-a-secret")
        var environment = ["PATH": "/usr/bin"]
        try GeminiKeySource.apply(store: store, to: &environment)
        XCTAssertEqual(environment["GEMINI_API_KEY"], "placeholder-not-a-secret")
        var shell = ["GEMINI_API_KEY": "shell-placeholder"]
        try GeminiKeySource.apply(store: store, to: &shell)
        XCTAssertEqual(shell["GEMINI_API_KEY"], "shell-placeholder", "the shell's value is kept")
        var empty = ["PATH": "/usr/bin"]
        try GeminiKeySource.apply(store: MemoryGeminiKeyStore(), to: &empty)
        XCTAssertNil(empty["GEMINI_API_KEY"], "no key sets nothing, so tap's own no_api_key message is what the person sees")
    }

    func testTheKeychainStoreNamesItsItem() {
        let store = KeychainGeminiKeyStore(service: "io.geocod.tap.desktop.tests", account: "GEMINI_API_KEY")
        XCTAssertEqual(store.service, "io.geocod.tap.desktop.tests")
        XCTAssertEqual(store.account, "GEMINI_API_KEY")
        XCTAssertEqual(KeychainGeminiKeyStore().service, "io.geocod.tap.desktop")
    }

    func testTheMemoryStoreRecordsItsWrites() throws {
        let store = MemoryGeminiKeyStore(key: "placeholder-not-a-secret")
        XCTAssertEqual(try store.read(), "placeholder-not-a-secret")
        try store.write(nil)
        XCTAssertNil(try store.read())
        XCTAssertEqual(store.writes, [nil])
    }

    /// The SecItem calls themselves, run only where TAP_KEYCHAIN_TESTS=1, so
    /// a person's Mac never sees a Keychain prompt from a test. The item lives under a service name of its own and is
    /// deleted after, whatever the assertions did.
    func testTheKeychainRoundTripsOnCI() throws {
        try XCTSkipUnless(ProcessInfo.processInfo.environment["TAP_KEYCHAIN_TESTS"] == "1", "the real Keychain is exercised on CI alone")
        let store = KeychainGeminiKeyStore(service: "io.geocod.tap.desktop.tests.\(UUID().uuidString)", account: "GEMINI_API_KEY")
        addTeardownBlock { try? store.write(nil) }
        XCTAssertNil(try store.read(), "nothing under a fresh service")
        try store.write("placeholder-not-a-secret")
        XCTAssertEqual(try store.read(), "placeholder-not-a-secret")
        try store.write("second-placeholder")
        XCTAssertEqual(try store.read(), "second-placeholder", "a write replaces")
        try store.write(nil)
        XCTAssertNil(try store.read(), "nil removes")
    }
}
