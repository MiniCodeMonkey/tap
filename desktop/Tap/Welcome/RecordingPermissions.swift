import AppKit
import AVFoundation
import CoreGraphics
import TapDesktopCore

/// What the recording setup screen asks of macOS. The live one talks to
/// AVFoundation, CoreGraphics and System Settings; a test installs a fake,
/// so no test shows a real permission prompt.
@MainActor
protocol RecordingPermissions: AnyObject {
    var microphone: MicrophoneAccess { get }
    var screenRecordingAllowed: Bool { get }
    /// Shows the macOS microphone prompt.
    func requestMicrophone()
    func openMicrophoneSettings()
    /// Adds Tap to the Screen Recording list, which may show the system prompt, and opens that pane.
    func requestScreenRecordingAndOpenSettings()
}

@MainActor
final class LiveRecordingPermissions: RecordingPermissions {
    var microphone: MicrophoneAccess {
        switch AVCaptureDevice.authorizationStatus(for: .audio) {
        case .authorized: .allowed
        case .notDetermined: .notDetermined
        default: .denied
        }
    }

    var screenRecordingAllowed: Bool { CGPreflightScreenCaptureAccess() }

    func requestMicrophone() {
        AVCaptureDevice.requestAccess(for: .audio) { _ in }
    }

    func openMicrophoneSettings() { Self.open("Privacy_Microphone") }

    func requestScreenRecordingAndOpenSettings() {
        _ = CGRequestScreenCaptureAccess()
        Self.open("Privacy_ScreenCapture")
    }

    private static func open(_ pane: String) {
        NSWorkspace.shared.open(URL(string: "x-apple.systempreferences:com.apple.preference.security?\(pane)")!)
    }
}
