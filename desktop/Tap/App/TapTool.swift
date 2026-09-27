import Foundation

/// Runs one tap subcommand with the app's tap and environment: `tap new`,
/// `tap theme show`, `tap export pdf` and the rest. The command line goes
/// to the deck's log when there is one; the environment never does. The
/// Gemini key is added to the environment of a run that asks for it (the
/// two image runs), read from the store at that moment: never to a tap
/// dev or tap present session, whose shell driver would hand it to any
/// block on a slide.
@MainActor
enum TapTool {
    static let defaultTimeout: TimeInterval = 120

    /// A run ready to start, for a caller that cancels or streams (the export sheet, the preview server).
    static func makeRun(_ arguments: [String], in directory: URL? = nil, timeout: TimeInterval? = defaultTimeout, log: TapLog? = nil,
                        includeGeminiKey: Bool = false, keepsStandardInputOpen: Bool = false) async -> ToolRun {
        let environment = AppEnvironment.shared
        var variables = await environment.tapEnvironment()
        if includeGeminiKey {
            try? GeminiKeySource.apply(store: environment.geminiKeyStore, to: &variables)
        }
        let configuration = ToolRun.Configuration(executableURL: environment.toolExecutableURL ?? environment.tapExecutableURL,
                                                  arguments: arguments, environment: variables, currentDirectoryURL: directory,
                                                  timeout: timeout, keepsStandardInputOpen: keepsStandardInputOpen)
        let run = ToolRun(configuration: configuration)
        log?.append("tap " + arguments.joined(separator: " "), source: .app)
        if let log {
            run.onStandardErrorLine = { [weak log] line in log?.append(line, source: .standardError) }
        }
        return run
    }

    /// Starts and waits.
    static func run(_ arguments: [String], in directory: URL? = nil, timeout: TimeInterval? = defaultTimeout, log: TapLog? = nil,
                    includeGeminiKey: Bool = false, onProgress: ((ProgressLine) -> Void)? = nil) async -> ToolRun.Exit {
        let run = await makeRun(arguments, in: directory, timeout: timeout, log: log, includeGeminiKey: includeGeminiKey)
        run.onProgress = onProgress
        let exit = await run.run()
        if let log, let outcome = exit.outcome, case .failed(let code, let message) = outcome {
            log.append("tap \(arguments.first ?? "") failed (\(code)): \(message)", source: .app)
        }
        return exit
    }
}
