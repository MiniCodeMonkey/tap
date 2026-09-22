import AppKit

@MainActor
final class AppDelegate: NSObject, NSApplicationDelegate {
    // main.swift's top-level code runs on the main thread but is not itself
    // main-actor isolated, so it cannot call the implicit isolated init.
    // Constructing an NSObject subclass touches no main-actor state, so an
    // explicit nonisolated init is safe and lets main.swift call it directly.
    override nonisolated init() {
        super.init()
    }

    func applicationDidFinishLaunching(_ notification: Notification) {
        AppEnvironment.shared.warmUp()
    }
}
