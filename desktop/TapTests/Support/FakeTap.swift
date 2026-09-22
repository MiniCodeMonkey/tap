import Foundation

/// Stands in for `tap dev --app` in the hosted tests. `dev --app` is not
/// implemented by the CLI on this branch yet (it lands with Task 13, on a
/// parallel branch), so `HostedTestCase` points every session at this
/// instead of the bundled tap. It prints a ready line at once and keeps
/// running until its standard input closes, same as `FakeTap` in
/// TapDesktopCoreTests.
enum FakeTap {
    static func ready(port: Int = 4242) throws -> URL {
        let folder = FileManager.default.temporaryDirectory.appendingPathComponent("tap-desktop-app-tests-\(UUID().uuidString)")
        try FileManager.default.createDirectory(at: folder, withIntermediateDirectories: true)
        let url = folder.appendingPathComponent("tap")
        let body = """
        #!/bin/sh
        echo '{"type":"ready","port":\(port),"token":"token","launch":"launch"}'
        while IFS= read -r line; do :; done
        exit 0
        """
        try body.write(to: url, atomically: true, encoding: .utf8)
        try FileManager.default.setAttributes([.posixPermissions: 0o755], ofItemAtPath: url.path)
        return url
    }
}
