import AppKit
import Sparkle

/// Sparkle's standard user interface, with a note in `interface` of what it
/// has up: every call that shows a window marks one, the permission
/// prompt's answer and the end of a session clear them. Calls pass through
/// unchanged; progress on a window already up changes nothing.
@MainActor
final class WatchedUserDriver: NSObject, SPUUserDriver {
    let inner: SPUUserDriver
    let interface: SparkleInterface

    init(inner: SPUUserDriver, interface: SparkleInterface) {
        self.inner = inner
        self.interface = interface
        super.init()
    }

    func show(_ request: SPUUpdatePermissionRequest, reply: @escaping (SUUpdatePermissionResponse) -> Void) {
        interface.permissionPromptShown()
        inner.show(request) { [weak self] response in
            self?.interface.permissionPromptAnswered()
            reply(response)
        }
    }

    func showUserInitiatedUpdateCheck(cancellation: @escaping () -> Void) {
        interface.updateWindowShown()
        inner.showUserInitiatedUpdateCheck(cancellation: cancellation)
    }

    func showUpdateFound(with appcastItem: SUAppcastItem, state: SPUUserUpdateState, reply: @escaping (SPUUserUpdateChoice) -> Void) {
        interface.updateWindowShown()
        inner.showUpdateFound(with: appcastItem, state: state, reply: reply)
    }

    func showUpdateReleaseNotes(with downloadData: SPUDownloadData) {
        inner.showUpdateReleaseNotes(with: downloadData)
    }

    func showUpdateReleaseNotesFailedToDownloadWithError(_ error: any Error) {
        inner.showUpdateReleaseNotesFailedToDownloadWithError(error)
    }

    func showUpdateNotFoundWithError(_ error: any Error, acknowledgement: @escaping () -> Void) {
        interface.updateWindowShown()
        inner.showUpdateNotFoundWithError(error, acknowledgement: acknowledgement)
    }

    func showUpdaterError(_ error: any Error, acknowledgement: @escaping () -> Void) {
        interface.updateWindowShown()
        inner.showUpdaterError(error, acknowledgement: acknowledgement)
    }

    func showDownloadInitiated(cancellation: @escaping () -> Void) {
        interface.updateWindowShown()
        inner.showDownloadInitiated(cancellation: cancellation)
    }

    func showDownloadDidReceiveExpectedContentLength(_ expectedContentLength: UInt64) {
        inner.showDownloadDidReceiveExpectedContentLength(expectedContentLength)
    }

    func showDownloadDidReceiveData(ofLength length: UInt64) {
        inner.showDownloadDidReceiveData(ofLength: length)
    }

    func showDownloadDidStartExtractingUpdate() {
        interface.updateWindowShown()
        inner.showDownloadDidStartExtractingUpdate()
    }

    func showExtractionReceivedProgress(_ progress: Double) {
        inner.showExtractionReceivedProgress(progress)
    }

    func showReady(toInstallAndRelaunch reply: @escaping (SPUUserUpdateChoice) -> Void) {
        interface.updateWindowShown()
        inner.showReady(toInstallAndRelaunch: reply)
    }

    func showInstallingUpdate(withApplicationTerminated applicationTerminated: Bool, retryTerminatingApplication: @escaping () -> Void) {
        interface.updateWindowShown()
        inner.showInstallingUpdate(withApplicationTerminated: applicationTerminated, retryTerminatingApplication: retryTerminatingApplication)
    }

    func showUpdateInstalledAndRelaunched(_ relaunched: Bool, acknowledgement: @escaping () -> Void) {
        interface.updateWindowShown()
        inner.showUpdateInstalledAndRelaunched(relaunched, acknowledgement: acknowledgement)
    }

    func showUpdateInFocus() {
        inner.showUpdateInFocus?()
    }

    func dismissUpdateInstallation() {
        interface.sessionFinished()
        inner.dismissUpdateInstallation()
    }
}
