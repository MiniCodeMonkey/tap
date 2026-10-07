import Foundation

/// Where the microphone permission stands. macOS shows its prompt only
/// from `notDetermined`; `denied` covers restricted too, and only System
/// Settings changes it.
public enum MicrophoneAccess: Equatable, Sendable {
    case notDetermined, allowed, denied
}

public enum RecordingStep: Equatable, Sendable {
    case microphone, screenRecording
}

/// What a step's button does, if the step has one.
public enum RecordingStepAction: Equatable, Sendable {
    /// Asks macOS for the permission, which shows its prompt.
    case allow
    case openSettings
    /// The person is in System Settings: no button, a waiting status.
    case waiting
    case none
}

/// The recording setup screen's whole decision: which step is current, what
/// each step's button is, and whether the screen opens by itself at launch.
/// Recording needs the microphone and Screen Recording, in that order.
/// `screenRecordingSettingsOpened` is a press of Open Settings for Screen
/// Recording in this launch: it turns the button into the waiting status,
/// and a relaunch without the permission brings the button back.
public struct RecordingSetup: Equatable, Sendable {
    public var microphone: MicrophoneAccess
    public var screenRecordingAllowed: Bool
    public var dismissed: Bool
    public var screenRecordingSettingsOpened: Bool

    public init(microphone: MicrophoneAccess, screenRecordingAllowed: Bool, dismissed: Bool, screenRecordingSettingsOpened: Bool = false) {
        self.microphone = microphone
        self.screenRecordingAllowed = screenRecordingAllowed
        self.dismissed = dismissed
        self.screenRecordingSettingsOpened = screenRecordingSettingsOpened
    }

    public var isComplete: Bool { microphone == .allowed && screenRecordingAllowed }

    /// The step that has the button; nil when both are allowed.
    public var currentStep: RecordingStep? {
        if microphone != .allowed { return .microphone }
        return screenRecordingAllowed ? nil : .screenRecording
    }

    public func isDone(_ step: RecordingStep) -> Bool {
        switch step {
        case .microphone: microphone == .allowed
        case .screenRecording: screenRecordingAllowed
        }
    }

    public func action(for step: RecordingStep) -> RecordingStepAction {
        guard step == currentStep else { return .none }
        switch step {
        case .microphone: return microphone == .notDetermined ? .allow : .openSettings
        case .screenRecording: return screenRecordingSettingsOpened ? .waiting : .openSettings
        }
    }

    /// Whether the screen replaces the Welcome window at launch: only while
    /// a permission is missing and the person has not put it off.
    public var showsAtLaunch: Bool {
        !isComplete && !dismissed
    }
}

/// The one fact about the setup screen that outlives a launch.
public struct RecordingSetupStore: @unchecked Sendable {
    private let defaults: UserDefaults
    static let dismissedKey = "RecordingSetupDismissed"

    public init(defaults: UserDefaults = .standard) {
        self.defaults = defaults
    }

    /// The person pressed Set Up Later.
    public var dismissed: Bool {
        get { defaults.bool(forKey: Self.dismissedKey) }
        nonmutating set { defaults.set(newValue, forKey: Self.dismissedKey) }
    }

}
