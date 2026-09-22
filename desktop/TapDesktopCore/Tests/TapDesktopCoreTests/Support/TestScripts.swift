import Foundation
import XCTest

/// Writes shell scripts that stand in for tap or a login shell.
enum TestScripts {
    static func temporaryFolder() throws -> URL {
        let folder = FileManager.default.temporaryDirectory.appendingPathComponent("tap-desktop-tests-\(UUID().uuidString)")
        try FileManager.default.createDirectory(at: folder, withIntermediateDirectories: true)
        return folder
    }

    /// An executable `/bin/sh` script with `body` as its text.
    static func make(_ body: String) throws -> URL {
        let url = try temporaryFolder().appendingPathComponent("script")
        try ("#!/bin/sh\n" + body + "\n").write(to: url, atomically: true, encoding: .utf8)
        try FileManager.default.setAttributes([.posixPermissions: 0o755], ofItemAtPath: url.path)
        return url
    }
}

struct WaitTimedOut: Error, CustomStringConvertible {
    let description: String
}

/// Polls `condition` on the main actor until it is true.
@MainActor
func waitUntil(timeout: TimeInterval = 5, _ message: String = "condition", _ condition: @MainActor () -> Bool) async throws {
    let deadline = Date().addingTimeInterval(timeout)
    while !condition() {
        if Date() > deadline { throw WaitTimedOut(description: "timed out waiting for \(message)") }
        try await Task.sleep(nanoseconds: 20_000_000)
    }
}
