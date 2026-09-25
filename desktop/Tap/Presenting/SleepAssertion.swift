import Foundation
import IOKit.pwr_mgt

/// Keeps the display awake for exactly as long as a talk runs: the same
/// kind of assertion Keynote holds during a slideshow. Every way a talk
/// ends releases it (see `PresentationController.takeDownWindows`), and a
/// process that dies takes its assertions with it.
final class SleepAssertion {
    static let reason = "Tap is presenting"
    /// The assertion type, spelled as IOPMLib.h spells
    /// kIOPMAssertionTypePreventUserIdleDisplaySleep, which is a macro of a
    /// macro that Swift does not import.
    static let type = "PreventUserIdleDisplaySleep"

    private var identifier: IOPMAssertionID = 0
    private(set) var isHeld = false

    func acquire() {
        guard !isHeld else { return }
        let result = IOPMAssertionCreateWithName(Self.type as CFString, IOPMAssertionLevel(kIOPMAssertionLevelOn),
                                                 Self.reason as CFString, &identifier)
        // kIOReturnSuccess is KERN_SUCCESS, which is 0.
        isHeld = result == 0
    }

    func release() {
        guard isHeld else { return }
        IOPMAssertionRelease(identifier)
        identifier = 0
        isHeld = false
    }

    deinit {
        release()
    }
}
