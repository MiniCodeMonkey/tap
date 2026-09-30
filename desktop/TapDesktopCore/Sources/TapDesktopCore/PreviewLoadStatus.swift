import Foundation

/// The step the preview is waiting on before its first paint, as far as the
/// app can tell without running a script in the page.
public enum PreviewLoadStep: Equatable, Sendable {
    /// tap is starting.
    case startingTap
    /// tap runs and the page is loading.
    case loadingPage
    /// The page loaded and is fetching fonts and images before it reports its first slide ready.
    case preparingSlide

    public var message: String {
        switch self {
        case .startingTap: return "Starting the preview…"
        case .loadingPage: return "Loading the slides…"
        case .preparingSlide: return "Loading fonts and images…"
        }
    }
}

/// When the preview names what it is waiting on: only once the wait has
/// lasted `threshold`, so a quick start shows no words at all.
public enum PreviewLoadStatus {
    public static let threshold: TimeInterval = 1

    public static func message(for step: PreviewLoadStep, waited: TimeInterval) -> String? {
        waited >= threshold ? step.message : nil
    }
}
