# Tap Desktop release (D7): implementation plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** One `make -C desktop release VERSION=x.y.z` builds the Apple silicon Release app with its version and a build number, signs it with its entitlements (Developer ID when the certificate is present, ad-hoc otherwise), notarizes and staples the app and the DMG when the notary key is present, writes the DMG, its checksum, the release notes, a Sparkle appcast (the DMG, the notes and the feed itself EdDSA-signed when the Sparkle key is present) and the `tap-desktop` cask; the release job runs it on a macOS runner after the CLI release and publishes only what a person may safely receive: the DMG under its release name, the appcast and the cask only when the DMG was notarized and the feed is signed, an un-notarized DMG only under a name that says so, and never an unsigned feed; every signing, notarization, Sparkle-signing and publishing step is skipped cleanly, by name, when its secret or its precondition is absent, so the whole pipeline runs today with no secrets at all; and the app checks for updates through Sparkle's standard UI without ever interrupting a talk.

**Architecture:** The app gains Sparkle 2.10.0 as a Swift package pinned by exact version, four Info.plist keys (`SUFeedURL`, `SUPublicEDKey`, `SURequireSignedFeed`, `SUVerifyUpdateBeforeExtraction`), the microphone usage string and an entitlements file (the hardened runtime needs `com.apple.security.device.audio-input` for the talks D4 records), and one class of its own, `UpdateController`, which holds the `SPUStandardUpdaterController`, gives Tap > Check for Updates… its action, and is Sparkle's delegate: it refuses a check while a talk runs, drops an update found during a talk, postpones a relaunch until the talk's windows are down, and refuses Play while an update session is in progress, all through `AppEnvironment.updatesMayInterrupt` (D4). The rule itself lives in `TapDesktopCore` as `UpdateGate`, pure and tested without Sparkle. The updater never starts under tests or in a `0.0.0` build. Release builds are `arm64` only (the person's decision, 2026-09-27): the app, the bundled tap, the cask's `depends_on arch` and the appcast's `sparkle:hardwareRequirements` all say so, and the verifier checks it. Everything else is shell under `desktop/scripts/`, one script per step with a test script beside it, in the style of `check-scenarios.sh`: `build-number.sh` (a monotonic `CFBundleVersion` from the semantic version, `alpha` below `beta` below `rc` below the final), `sign-app.sh` (inside out, hardened runtime, the app's entitlements, `--timestamp` only with a real identity), `signing-identity.sh` (a temporary keychain from the certificate secret, removed on every exit), `notarize.sh` (`notarytool submit --wait`, then `stapler`), `make-dmg.sh` (`hdiutil`, the app and an Applications link, no Finder), `sparkle-sign.sh` (`sign_update --ed-key-file -` from the pinned Sparkle tools, for the DMG, the notes and the feed), `write-appcast.sh` (the one-item feed from the app's own Info.plist), `render-cask.sh` and `publish-cask.sh` (a template in this repository, pushed to the tap the CLI formula already uses, only for a notarized final), `verify-release.sh` (architecture, version, entitlements, plist keys, no test code, signatures, Gatekeeper when notarized), `mark-latest.sh` (the release becomes GitHub's `latest` only once its feed is up, so the feed URL never returns 404), and `release.sh`, the orchestrator that runs them in order, records what it did and skipped in `release-summary.md` and `release-state.env`, and never prints a secret. The Makefile gains `release-build`, `check-release-app-hooks`, `release-tests` and `release`. `.github/workflows/release.yml` gains a `desktop` job on `macos-15` that builds without secrets, then runs `release.sh` with the secrets in that one step's environment, then publishes by the state file; `ci.yml` gains a `release-dry-run` job that runs the same target with no secrets on every pull request and keeps the DMG as an artifact, which is the proof the pipeline works before any secret exists.

**Tech Stack:** Swift 6.3 compiler in Swift 5 language mode, AppKit, Sparkle 2.10.0 (`SPUStandardUpdaterController`, `SPUUpdaterDelegate`, `SPUUpdater.sessionInProgress`, `sparkle:hardwareRequirements`, signed feeds), XcodeGen (`packages` with `exactVersion`, `info.properties`, per-config `ENABLE_HARDENED_RUNTIME` and `ARCHS`, `CODE_SIGN_ENTITLEMENTS`), `codesign`, `lipo`, `xcrun notarytool`, `xcrun stapler`, `spctl`, `hdiutil`, `ditto`, `PlistBuddy`, `xmllint`, `shasum`, `security` (a temporary keychain), Sparkle's `sign_update` from the `Sparkle-2.10.0.tar.xz` release archive (sha256 `c2bf58aa8387266ac179357b1415d6f2635f044da8be41042af32425dae6da0c`), Homebrew casks (`brew style`), GitHub Actions on `macos-15` with Xcode 26, `gh release upload`, `gh release edit --latest`, POSIX `sh` for every script, XCTest for the hosted tests, `swift test` for the Core tests.

**Spec:** `docs/superpowers/specs/2026-09-22-tap-desktop-design.md` (milestone 7; the sections "Platform and repo", "Processes", "Presenting", "Settings", "Menus and accessibility", "Security summary", "Distribution" and "Testing"), the D7 outline and the person's decision of 2026-09-22 in `docs/superpowers/plans/2026-09-22-tap-desktop-roadmap.md` ("build all of it so it runs once the secrets are added, and use ad-hoc signing locally ... The release job skips signing steps when their secrets are absent"), the person's decisions of 2026-09-27 (Apple silicon only; nothing un-notarized or unsigned is published as a release), and `docs/superpowers/specs/tap-desktop-features/05-presenting.feature`'s "Nothing interrupts the talk" ("Sparkle shows no update prompt and never restarts the app"), already claimed by D4 (`FocusHintTests.testNothingInterruptsTheTalk` asserts `updatesMayInterrupt`); this plan reads that flag in every update decision. No feature file has a release scenario, so `desktop/scenarios.txt` gains no row. The existing release tooling is the pattern: `.github/workflows/release.yml` (the version regex, `prepare-changelog.sh`, `softprops/action-gh-release`, the Homebrew formula step with `HOMEBREW_TAP_TOKEN` and `HOMEBREW_TAP_REPO`), `.github/workflows/ci.yml`'s desktop jobs (Xcode 26, xcodegen, `make frontend` first), `desktop/scripts/build-tap.sh` (how the bundled tap gets `TAP_VERSION`), `desktop/scripts/check-scenarios.sh` and `check-scenarios-test.sh` (a POSIX script with a test script beside it). The plan's review (`d7-plan-review.md`, 2026-09-27) ran every script and typechecked the Swift; its findings are folded in.

**Depends on:** D6 merged (`feat/desktop-creating-export-settings`): its Tap menu holds `Check for Updates…` with no action, and its Settings window has the General pane Task 10 extends. If D7 starts before D6 has merged, Task 3 adds the menu item itself (the same line D6's Task 12 writes) and Task 10 waits.

**Branch:** `feat/desktop-release`, branched from `main` after D6's pull request has merged, in a worktree at `/Users/codemonkey/projects/tap-d7`. One pull request. Mutation patches go under `.superpowers/sdd/d7/mutations-<batch>/` and `survivors-<batch>/`, as D5 did.

**Known facts (the person, 2026-09-27):** the Sparkle EdDSA public key is `Pc0PbtYL9fkyK3XnqULlpKLlH94TE4OWx4Q3n74EftU=`; its private key is in the person's login keychain under the account `tap-desktop` and in 1Password, and reaches CI only as the secret `SPARKLE_PRIVATE_KEY`. `HOMEBREW_TAP_TOKEN` exists as a repository secret today, so the cask push must be gated on more than the token; the tap `MiniCodeMonkey/homebrew-tap` has `Formula/tap.rb` and no `Casks/` folder yet. The Apple Developer credentials come after enrolment. The repository's tags run `v2.0.0-beta.1` to `v2.0.0-beta.7` then `v2.0.0-rc.1`, so a version uses more than one pre-release label. Release builds are Apple silicon only. No agent runs anything locally that opens a window.

## Global Constraints

- Everything in D2's to D6's Global Constraints still holds: macOS 14 or later, AppKit core, the bundled `tap` from `build-tap.sh`, spelled-out identifiers, present-tense comments with no ticket references, no em dashes anywhere (`--`, a comma or a new sentence instead), `make frontend` before the first Xcode build, every Xcode build through `make`.
- **THE PERSON'S RULE (2026-09-25): nothing runs locally that opens windows on their screen.** Allowed locally: `make -C desktop project`, `build`, `test-build`, `bench-build`, `core-test`, `check-scenarios`, `check-release-hooks`, `go test`, every `desktop/scripts/*-test.sh`, `make -C desktop release-tests`, `make -C desktop release-build`, `make -C desktop check-release-app-hooks` and `make -C desktop release VERSION=0.0.0-dev` (builds, signs ad-hoc, writes the DMG and the appcast, launches nothing; `hdiutil attach -nobrowse` mounts without a Finder window; the bundled `tap --version` is a command line run). Never: `make -C desktop test`, `ONLY=...`, `uitest`, `bench`, `open Tap.app`, `open Tap.dmg`, `spctl --assess` on a launch, or `stapler` outside the release script. Hosted tests (`UpdaterTests`, `SettingsUpdatesTests`) run on CI: every "Run" step that names one says what the controller's CI run confirms.
- **THE SECRETS RULE.** No secret is written into the repository, a log, a workflow summary, a test, a fixture or a release artifact. Every script that takes a secret reads it from the environment (`APPLE_DEVELOPER_ID_APPLICATION_P12`, `APPLE_DEVELOPER_ID_APPLICATION_PASSWORD`, `APPLE_NOTARY_KEY`, `APPLE_NOTARY_KEY_ID`, `APPLE_NOTARY_ISSUER_ID`, `SPARKLE_PRIVATE_KEY`, `HOMEBREW_TAP_TOKEN`) or through a pipe into standard input; a secret that must be a file (the `.p8` key, the `.p12`) is written with `umask 077` into a `mktemp -d` folder that an `EXIT` trap removes; the temporary keychain is deleted by a trap set before the import starts, and again by a workflow step that runs `if: always()`. No script uses `set -x`; no script echoes a variable that holds a secret; `security`, `notarytool` and `git` are run with their output filtered to what is not secret (the identity's name, the submission id and status, the commit hash). The workflow passes secrets only through `env:`, only to the one step that runs `release.sh`, and never in `run:` text; the build step before it has none. A step that finds its secret empty prints exactly one line, `skipped: <what> (<SECRET_NAME> is not set)`, appends it to `release-summary.md`, and exits 0; nothing else about that step happens. Two exposures are accepted and named: the `.p12` password is an argument of `security import -P` and the Homebrew token is a git config value for the length of one clone, on a single-tenant runner.
- **NOTHING UN-NOTARIZED OR UNSIGNED IS OFFERED TO A PERSON (the person, 2026-09-27).** `release.sh` records `notarized=yes|no` and `feed_signed=yes|no` in `release-state.env`. A DMG that was not notarized is named `Tap-<version>-unnotarized.dmg`, never `Tap-<version>.dmg`, and never becomes a release asset: the job keeps it as a workflow artifact (the same seven days as the dry run's) and the summary says where it is (the controller's ruling on open question 9, 2026-09-27). The appcast is written as `appcast.xml` only when the DMG, the notes and the feed itself carry EdDSA signatures; otherwise it is `appcast-unsigned.xml` and stays out of the release. The cask is pushed only for a notarized final. A release becomes GitHub's `latest` only once its signed feed is uploaded (or when no earlier release ever carried a feed), so `SUFeedURL` never answers 404 between the CLI and the desktop jobs. The app requires a signed feed (`SURequireSignedFeed`, `SUVerifyUpdateBeforeExtraction`), so a feed that slips through unsigned is refused before anything is shown.
- **Ad-hoc is the default everywhere.** `codesign --sign -` when no identity is found; `sign-app.sh` is the one place that signs, and a dry run and a real run take the same path through it with a different identity. The release build must still pass the hook check: `make -C desktop check-release-app-hooks` proves `approvalAnswerForTests` is in neither the symbol table nor the strings of the Release binary and a known string (`TapExecutablePath`) is, so a stripped or renamed binary cannot pass by being empty. (`TapDefaultsSuite`, `TapConfigHome`, `TapOpenOnLaunch` and `TapExecutablePath` are launch arguments the UI tests pass and are in every build by design; they read nothing a person did not put on the command line.) `verify-release.sh` fails when a Release product carries `Contents/PlugIns`, an `XCTest*` framework or `libXCTest*`.
- **Apple silicon only.** `ARCHS: arm64` in the Release configuration; `build-tap.sh` builds tap for `GOARCH=arm64` when `ARCHS` names one architecture; the cask says `depends_on arch: :arm64`; the appcast item carries `<sparkle:hardwareRequirements>arm64</sparkle:hardwareRequirements>` (Sparkle 2.9 or later refuses to offer it on an Intel Mac); `verify-release.sh` checks `lipo -archs` gives exactly `arm64` for `Contents/MacOS/Tap` and `Contents/Resources/tap`, and that `Contents/Resources/tap --version` prints `tap version <version>`.
- **The hardened runtime needs the microphone entitlement.** `desktop/Tap/Tap.entitlements` holds `com.apple.security.device.audio-input`; `project.yml` sets `CODE_SIGN_ENTITLEMENTS` and `NSMicrophoneUsageDescription`; `sign-app.sh` signs the app with the entitlements (nested code gets none of its own); `verify-release.sh` reads them back. CI cannot prove a recording under the hardened runtime (the hosted tests use a fake recorder and Debug has no hardened runtime), so the README's manual pass gains one real recording with sound from the first notarized DMG.
- **A talk is never interrupted.** Every Sparkle decision that could show a window or restart the app passes through `UpdateGate` with `AppEnvironment.shared.updatesMayInterrupt`: a scheduled or user check is refused (`updater(_:mayPerform:)`), a found update is dropped (`updater(_:shouldProceedWithUpdate:updateCheck:)`), a relaunch is postponed (`updater(_:shouldPostponeRelaunchForUpdate:untilInvokingBlock:)`) and resumed once, when the last talk has ended and its windows are down. Play is refused, with the talk-not-started bar D4 already shows, while an update session Sparkle started before Play is in progress (`SPUUpdater.sessionInProgress`), so a download or an "Install and Relaunch" window cannot land in the middle of a talk. The updater never starts in a test process (`XCTestConfigurationFilePath` set, or `-TapDefaultsSuite` given) or in a `0.0.0` build, so no hosted test, UI test or benchmark ever reaches the network or Sparkle's windows, and a Debug app run from DerivedData never offers to replace itself.
- **Sparkle's standard UI only.** No update sheet, no custom update window, no custom release notes view. The one UI addition in this plan, a checkbox in Settings > General (Task 10), waits for the person's mockup sign-off and is ordered last; Sparkle's own permission prompt, update alert, release notes view (which renders the release's Markdown notes), progress window and relaunch alert are Sparkle's, not new UI; the Play refusal reuses D4's `talkNotStarted` bar with one more sentence. The DMG is plain (the app and an Applications link, no background art): art would be new UI needing a mockup and Finder scripting on the runner.
- **Sparkle is pinned.** `exactVersion: 2.10.0` in `project.yml`; the tools come from `Sparkle-2.10.0.tar.xz` with its sha256 checked before extraction into a folder named after the version. A bump changes both places in one commit.
- **Every script is POSIX `sh`** (`#!/bin/sh`, `set -eu`, no arrays, no `[[`), runs from any directory (paths from `$(dirname "$0")`), and has a `<name>-test.sh` beside it that builds its own fixtures under `mktemp -d` and removes them in a trap. `make -C desktop release-tests` runs every test script. A test never touches the person's keychain search list for longer than its own run, their defaults, `~/Applications` or `/Applications`, and never downloads anything.
- **XCTest rules.** No `await` inside an `XCTAssert` autoclosure. Every wait goes through `waitUntil(timeout:)`. No test depends on a key window. Hosted tests reach Core through `@testable import Tap`. A test that needs a talk uses `PresentingTestCase.openDeckForPresenting`, `startPresenting` and `stopPresenting` (which waits for the talk's windows to close).
- `weak self` in every closure that outlives a call, no `unowned`. No modal alerts of the app's own. No production code steals focus.

## Review Focus

Five conditions the spec implies that no scenario names, most likely to bite first. Each has its test pinned to the task that owns the code.

1. **A release with some secrets but not others.** The certificate without the notary key, the notary key without the Sparkle key, the token without any of them. Each missing secret must skip its own step and nothing else, the un-notarized DMG must be named so and stay off the release, no feed may go out unsigned, no cask may point at an un-notarized DMG, and a rejected notarization must stop everything. Task 7b, `release-test.sh` (the no-secrets run: five files, the `-unnotarized` name, `appcast-unsigned.xml`, seven `skipped:` lines; the certificate-only run: the skip line names `APPLE_NOTARY_KEY`, the state sources under `bash -e`; the all-secrets run; the rejected run stops with no state) and `verify-release-test.sh` (signed but not notarized passes; "notarized" without a ticket fails); Task 6, `publish-cask-test.sh` (the token alone does not push); Task 8's `release-dry-run` CI job against the real app.
2. **An update found before a talk that would prompt during it, and an install begun before Play.** A background check that starts a minute before Play finishes during the talk; a download the person started finishes during it. Task 3, `UpdaterTests.testAnUpdateFoundDuringATalkIsDropped` (the delegate's `shouldProceedWithUpdate` throws while presenting), `testPlayWaitsForAnUpdateInProgress` (Play shows the bar and starts nothing while a session is in progress) and `UpdateGateTests.testAFoundUpdateIsDroppedWhilePresenting`.
3. **A relaunch postponed by a talk.** It must run when the talk has ended and its windows are down, exactly once, never mid-transition, and a second talk before it ran must not run it twice or lose it. Task 3, `UpdateGateTests.testAPostponedRelaunchRunsOnceWhenTheTalkEnds` and `UpdaterTests.testAPostponedRelaunchRunsWhenTheTalkWindowsAreDown`.
4. **A pre-release build number.** `2.0.0-beta.7` must sort below `2.0.0-rc.1`, which must sort below `2.0.0` (the repository's own tag history), and above `1.9.9`; a version the regex refuses must fail the build, not produce `0`. Task 1, `build-number-test.sh`.
5. **The Release app's binary, plist and entitlements.** The two architectures, the bundled tap's version, the four Sparkle keys, the microphone string and entitlement, the hardened runtime flag, the version and the build number must be in the built app, and no test hook or test framework may be; a dry run that passes with a wrong plist ships a feed nobody can reach or a recording without sound. Task 1, `check-release-app-hooks`; Task 7b, `verify-release.sh`.

## The release, end to end

| Step | Local dry run (`make -C desktop release VERSION=0.0.0-dev`) | CI with every secret (`release.yml`, job `desktop`) | Script |
|---|---|---|---|
| Build | `xcodebuild -configuration Release ARCHS=arm64 TAP_VERSION=0.0.0-dev CURRENT_PROJECT_VERSION=1` (build number of `0.0.0-dev` is 1) | the same with the release version, in a step with no secret in its environment | Makefile `release-build`, `build-number.sh`, `build-tap.sh` |
| Hook check | `nm` and `strings` of the Release binary | the same | Makefile `check-release-app-hooks` |
| Identity | `-` (a `skipped:` line) | a temporary keychain from `APPLE_DEVELOPER_ID_APPLICATION_P12`, the identity's name | `signing-identity.sh import` |
| Sign | tap, Sparkle's pieces, the framework, the app with `Tap.entitlements`, ad-hoc, `--options runtime` | the same with `--timestamp` and the identity | `sign-app.sh` |
| Notarize the app | skipped | `ditto -c -k` the app, `notarytool submit --wait`, `stapler staple Tap.app` | `notarize.sh` |
| DMG | `Tap-0.0.0-dev-unnotarized.dmg` with the app and an Applications link | `Tap-<v>.dmg`, then `codesign` the DMG | `make-dmg.sh` |
| Notarize the DMG | skipped | `notarytool submit --wait`, `stapler staple Tap-<v>.dmg` | `notarize.sh` |
| Checksum | `Tap-0.0.0-dev-unnotarized.dmg.sha256` | `Tap-<v>.dmg.sha256` | `release.sh` |
| Release notes | none (no changelog section for a dry-run version) | `Tap-<v>.md` from the tag's `CHANGELOG.md` through `scripts/prepare-changelog.sh`, EdDSA-signed | `release.sh`, `sparkle-sign.sh notes` |
| Sparkle signatures | skipped (the DMG, the notes, the feed) | `sign_update --ed-key-file -` from the pinned tools, for the DMG (`-p`), the notes (attributes) and the feed (embedded) | `sparkle-sign.sh`, `fetch-sparkle-tools.sh` |
| Appcast | `appcast-unsigned.xml`, one item from the app's Info.plist with `hardwareRequirements arm64` | `appcast.xml` with `sparkle:edSignature` on the enclosure and the notes link, and the feed's own `sparkle-signatures` block | `write-appcast.sh`, `sparkle-sign.sh feed` |
| Cask | `Casks/tap-desktop.rb` rendered into the output folder | the same, pushed to `Casks/tap-desktop.rb` in the tap only for a notarized final | `render-cask.sh`, `publish-cask.sh` |
| Verify | archs, `tap --version`, entitlements, plist keys, no test code, `codesign --verify --deep --strict`, `hdiutil verify`, `xmllint`, the feed names the same build | the same plus, when notarized, `spctl --assess` on the DMG and `stapler validate` on both, and the feed's signature block | `verify-release.sh` |
| State | `release-state.env`: `notarized=no`, `feed_signed=no`, the file names | `notarized=yes`, `feed_signed=yes` | `release.sh` |
| Publish | nothing | notarized: `gh release upload` the DMG and its checksum, then the notes, then the feed (signed only); not notarized: nothing reaches the release, the `-unnotarized` DMG is a workflow artifact | `release.yml` |
| Latest | nothing | a final becomes `latest` once its feed is up, or when no earlier release ever had one | `mark-latest.sh` |
| Summary | `release-summary.md` lists every step as done or skipped | the same, into `$GITHUB_STEP_SUMMARY`, on success and on failure | `release.sh` |

The feed URL is `https://github.com/MiniCodeMonkey/tap/releases/latest/download/appcast.xml`: GitHub redirects it to the `latest` release's asset, so no hosting is added, and a pre-release never reaches the feed (open question 1). The DMG's URL inside the appcast and the cask is the release's own, `https://github.com/MiniCodeMonkey/tap/releases/download/v<version>/Tap-<version>.dmg`; the notes are `.../Tap-<version>.md` beside it.

## File structure

| Path | Responsibility |
|---|---|
| `desktop/scripts/build-number.sh`, `build-number-test.sh` | The integer `CFBundleVersion` for a semantic version, labels ranked |
| `desktop/scripts/build-tap.sh` | Modify: `GOARCH` follows a single-architecture `ARCHS` |
| `desktop/Tap/Tap.entitlements` | `com.apple.security.device.audio-input` |
| `desktop/scripts/sign-app.sh`, `sign-app-test.sh` | Signs an app bundle inside out with the hardened runtime and the app's entitlements, ad-hoc or with an identity |
| `desktop/scripts/signing-identity.sh`, `signing-identity-test.sh` | `import`: a temporary keychain from the certificate secret, prints the identity name or `-`; `remove`: deletes it |
| `desktop/scripts/notarize.sh`, `notarize-test.sh` | Submits a file to the notary service and staples the target, or skips |
| `desktop/scripts/make-dmg.sh`, `make-dmg-test.sh` | The DMG from an app, with a retry for the runner's transient `hdiutil` failures |
| `desktop/scripts/fetch-sparkle-tools.sh` | Downloads and checks `Sparkle-2.10.0.tar.xz`, extracts `bin/` into a folder named after the version |
| `desktop/scripts/sparkle-sign.sh`, `sparkle-sign-test.sh` | `archive`: the EdDSA signature of the DMG; `notes`: the signature and length of the notes file; `feed`: signs the appcast in place; or skips |
| `desktop/scripts/write-appcast.sh`, `write-appcast-test.sh` | The one-item appcast from the app's Info.plist, the DMG, the notes and their signatures |
| `desktop/release/tap-desktop.rb.template` | The cask, with `__VERSION__` and `__SHA256__`, `depends_on arch: :arm64` |
| `desktop/scripts/render-cask.sh`, `render-cask-test.sh` | Renders the template for a DMG |
| `desktop/scripts/publish-cask.sh`, `publish-cask-test.sh` | Pushes the cask to the tap for a notarized final, or skips |
| `desktop/scripts/verify-release.sh`, `verify-release-test.sh` | The checks on the app, the DMG and the appcast, by the release's state |
| `desktop/scripts/release.sh`, `release-test.sh` | Runs the steps in order, writes `release-summary.md` and `release-state.env` |
| `desktop/scripts/mark-latest.sh`, `mark-latest-test.sh` | Makes a final release `latest` once its feed is up |
| `desktop/Makefile` | Modify: `release-build`, `check-release-app-hooks`, `release-tests`, `release`, `RELEASE_DIR` |
| `desktop/project.yml` | Modify: the Sparkle package, `ENABLE_HARDENED_RUNTIME` and `ARCHS` for Release, `CODE_SIGN_ENTITLEMENTS`, `SUFeedURL`, `SUPublicEDKey`, `SURequireSignedFeed`, `SUVerifyUpdateBeforeExtraction`, `NSMicrophoneUsageDescription` |
| `desktop/TapDesktopCore/Sources/TapDesktopCore/UpdateGate.swift` | The never-interrupt-a-talk rule and the may-start rule, pure |
| `desktop/TapDesktopCore/Tests/TapDesktopCoreTests/UpdateGateTests.swift` | Its tests |
| `desktop/Tap/App/UpdateController.swift` | Sparkle's controller and delegate, Check for Updates…, the session-in-progress seam |
| `desktop/Tap/App/AppDelegate.swift` | Modify: owns `updateController`, `checkForUpdates(_:)`, its validation |
| `desktop/Tap/App/MainMenu.swift` | Modify: Check for Updates… gets its action |
| `desktop/Tap/Windows/DeckWindowController.swift` | Modify: Play refused while an update session is in progress, through the existing bar |
| `desktop/Tap/Settings/GeneralSettingsViewController.swift` | Modify (Task 10, after sign-off): the Updates card |
| `desktop/TapTests/UpdaterTests.swift`, `SettingsUpdatesTests.swift` | The hosted tests |
| `.github/workflows/release.yml` | Modify: `make_latest: false`, the `desktop` job |
| `.github/workflows/ci.yml` | Modify: the `release-dry-run` job |
| `desktop/README.md`, `README.md`, `docs/getting-started.md`, `CONTRIBUTING.md`, `CHANGELOG.md`, `docs/superpowers/plans/2026-09-22-tap-desktop-roadmap.md` | Modify: install, release and secrets documentation; the manual recording check; the changelog entry; D7's row |

## Batches

| Batch | Tasks | What it proves when it lands |
|---|---|---|
| A | 1, 2, 3 | An arm64, versioned, hardened, entitled, ad-hoc-signed Release app with Sparkle in it that never interrupts a talk |
| B | 4, 5, 6, 7a | The DMG, the signed appcast, the cask, the identity and the notary steps, each testable alone |
| C | 7b, 8, 9, 10 | The verifier and the orchestrator, the two workflows, the docs; and, last, the Settings checkbox once its mockup is signed off |

---

### Task 1: The build number, the arm64 Release build and the hook check on it

**Files:**
- Create: `desktop/scripts/build-number.sh`, `desktop/scripts/build-number-test.sh`
- Modify: `desktop/project.yml` (`configs.Release.ENABLE_HARDENED_RUNTIME`, `configs.Release.ARCHS`)
- Modify: `desktop/scripts/build-tap.sh` (`GOARCH`)
- Modify: `desktop/Makefile` (`VERSION`, `BUILD_NUMBER`, `RELEASE_APP`, `RELEASE_BINARY`, `release-build`, `check-release-app-hooks`, `.PHONY`)

**Interfaces:**
- Consumes: `TAP_VERSION`, `MARKETING_VERSION`, `CURRENT_PROJECT_VERSION` as `project.yml` wires them into `CFBundleShortVersionString` and `CFBundleVersion`; `build-tap.sh`, which gives the bundled tap `TAP_VERSION`; Xcode's `ARCHS` in a build phase's environment.
- Produces: `desktop/scripts/build-number.sh <version>` printing an integer (exit 1 with a message on a version the release regex refuses or a pre-release number out of its label's range); `make -C desktop release-build VERSION=<v>` building `build/DerivedData/Build/Products/Release/Tap.app` (`RELEASE_APP`) for `arm64` alone; `make -C desktop check-release-app-hooks` (depends on `release-build`). Task 7b's `release` target and Task 8's jobs call both.

- [ ] **Step 1: Write the failing test script**

`desktop/scripts/build-number-test.sh`:

```sh
#!/bin/sh
# Checks build-number.sh: alpha sorts below beta below rc below the final,
# every pre-release of a version sorts below that version's final and above
# the previous patch, a number outside its label's range fails, and a bad
# version fails. The repository's own tags (2.0.0-beta.1 to beta.7, then
# 2.0.0-rc.1) are the case that matters most.
set -eu

script="$(cd "$(dirname "$0")" && pwd)/build-number.sh"

expect() {
	got=$("$script" "$1") || { echo "$1: exit $?"; exit 1; }
	[ "$got" = "$2" ] || { echo "$1: got $got, want $2"; exit 1; }
}

expect 0.0.0 99
expect 0.0.0-dev 1
expect 0.0.0-ci 1
expect 2.0.0 2000099
expect 2.0.0-alpha.1 2000001
expect 2.0.0-alpha.19 2000019
expect 2.0.0-beta.1 2000020
expect 2.0.0-beta.7 2000026
expect 2.0.0-beta.30 2000049
expect 2.0.0-rc.1 2000050
expect 2.0.0-rc.40 2000089
expect 2.1.0-beta.3 2010022
expect 2.1.0 2010099
expect 2.0.9 2000999
expect 1.9.9 1090999
expect 10.20.30 10203099

# The repository's history, in order.
beta7=$("$script" 2.0.0-beta.7); rc1=$("$script" 2.0.0-rc.1); final=$("$script" 2.0.0); previous=$("$script" 1.9.9)
[ "$previous" -lt "$beta7" ] && [ "$beta7" -lt "$rc1" ] && [ "$rc1" -lt "$final" ] || { echo "1.9.9 < beta.7 < rc.1 < 2.0.0 does not hold: $previous $beta7 $rc1 $final"; exit 1; }

# A number past its label's range would reach the next label's slots.
for bad in 1.0.0-alpha.20 1.0.0-beta.31 1.0.0-rc.41 1.0.0-alpha.0 1.0.0-beta 1.0.0-rc 1.0.0-nightly.3; do
	if "$script" "$bad" >/dev/null 2>&1; then echo "'$bad' should fail"; exit 1; fi
done
# Versions the release workflow's regex refuses fail here too, before a build.
for bad in 1.0 v1.0.0 1.0.0.0 1.0.0- "1.0.0 " abc ""; do
	if "$script" "$bad" >/dev/null 2>&1; then echo "'$bad' should fail"; exit 1; fi
done
# Minor and patch have two digits each.
if "$script" 1.100.0 >/dev/null 2>&1; then echo "1.100.0 should fail"; exit 1; fi

echo "build-number.sh is right"
```

- [ ] **Step 2: Run it to verify it fails**

Run: `chmod +x desktop/scripts/build-number-test.sh && desktop/scripts/build-number-test.sh`
Expected: fails at once, `build-number.sh: not found` (exit 1 from the first `expect`).

- [ ] **Step 3: Write the script**

`desktop/scripts/build-number.sh`:

```sh
#!/bin/sh
# Prints the CFBundleVersion for a semantic version: an integer that rises
# with every release, which Sparkle compares to decide what is an update.
#
#   MAJOR * 1000000 + MINOR * 10000 + PATCH * 100 + release slot
#
# The release slot ranks the label and its number: alpha.N is N (1 to 19),
# beta.N is 19 + N (20 to 49), rc.N is 49 + N (50 to 89), a final is 99,
# and a label with no number (dev, ci: the dry runs) is 1. So the
# repository's own 2.0.0-beta.7 (2000026) sorts below 2.0.0-rc.1 (2000050),
# which sorts below 2.0.0 (2000099); every 2.0.0 pre-release sorts above
# 1.9.9 (1090999). Minor and patch run to 99 each. Once a build has shipped,
# this scheme can only grow, never change.
set -eu

version="${1:-}"
case "$version" in
	*[!0-9A-Za-z.-]*|"") echo "build-number.sh: '$version' is not a version" >&2; exit 1 ;;
esac

core="${version%%-*}"
prerelease=""
case "$version" in
	*-*)
		prerelease="${version#*-}"
		[ -n "$prerelease" ] || { echo "build-number.sh: '$version' ends in a hyphen" >&2; exit 1; }
		;;
esac

case "$core" in
	*.*.*) ;;
	*) echo "build-number.sh: '$version' needs MAJOR.MINOR.PATCH" >&2; exit 1 ;;
esac
major="${core%%.*}"; rest="${core#*.}"
minor="${rest%%.*}"; patch="${rest#*.}"
for part in "$major" "$minor" "$patch"; do
	case "$part" in
		""|*[!0-9]*|*.*) echo "build-number.sh: '$version' is not MAJOR.MINOR.PATCH" >&2; exit 1 ;;
	esac
done
if [ "$minor" -gt 99 ] || [ "$patch" -gt 99 ]; then
	echo "build-number.sh: minor and patch run to 99 ($version)" >&2
	exit 1
fi

slot=99
if [ -n "$prerelease" ]; then
	case "$prerelease" in
		*[!0-9A-Za-z.]*|.*|*.|*..*) echo "build-number.sh: '$prerelease' is not a pre-release label" >&2; exit 1 ;;
	esac
	label="${prerelease%%.*}"
	number=""
	case "$prerelease" in *.*) number="${prerelease#*.}" ;; esac
	case "$number" in *[!0-9]*|*.*) echo "build-number.sh: '$prerelease' needs <label>.<number>" >&2; exit 1 ;; esac
	case "$label" in
		alpha) base=0; limit=19 ;;
		beta) base=19; limit=30 ;;
		rc) base=49; limit=40 ;;
		*)
			[ -z "$number" ] || { echo "build-number.sh: '$label' is not a release label (alpha, beta, rc)" >&2; exit 1; }
			base=0; limit=1; number=1
			;;
	esac
	[ -n "$number" ] || { echo "build-number.sh: '$label' needs a number ($version)" >&2; exit 1; }
	number=$((number))
	if [ "$number" -lt 1 ] || [ "$number" -gt "$limit" ]; then
		echo "build-number.sh: $label runs from 1 to $limit ($version)" >&2
		exit 1
	fi
	slot=$((base + number))
fi

echo $((major * 1000000 + minor * 10000 + patch * 100 + slot))
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `chmod +x desktop/scripts/build-number.sh && desktop/scripts/build-number-test.sh`
Expected: `build-number.sh is right`.

- [ ] **Step 5: The Release configuration, the bundled tap's architecture and the Makefile targets**

In `desktop/project.yml`, under `settings.configs`, beside `Benchmark`:

```yaml
    Release:
      # Notarization needs the hardened runtime. Debug and Benchmark keep it
      # off: a hardened test host refuses the injected test bundles.
      ENABLE_HARDENED_RUNTIME: YES
      # Apple silicon only (the person's decision, 2026-09-27): the bundled
      # tap follows ARCHS in build-tap.sh, the cask and the appcast say so.
      ARCHS: arm64
      ONLY_ACTIVE_ARCH: NO
```

In `desktop/scripts/build-tap.sh`, before the `go build` line:

```sh
# The bundled tap is built for the app's architectures. A Release build
# names one (arm64); a Debug build names the standard pair and builds the
# active one, for which the host's own GOARCH is right.
case "${ARCHS:-}" in
	arm64) export GOARCH=arm64 ;;
	x86_64) export GOARCH=amd64 ;;
esac
```

In `desktop/Makefile`, after `TEST_TIMEOUTS`:

```make
# The version a release build carries, in the app, its plist and the bundled
# tap. 0.0.0-dev is a local dry run; the release job passes the real one.
VERSION ?= 0.0.0-dev
BUILD_NUMBER = $(shell ./scripts/build-number.sh $(VERSION))
RELEASE_APP = $(DERIVED_DATA)/Build/Products/Release/Tap.app
RELEASE_BINARY = $(RELEASE_APP)/Contents/MacOS/Tap
```

Add `release-build check-release-app-hooks` to `.PHONY`, and after `check-release-hooks`:

```make
# The Release app with its version and build number, signed ad-hoc by
# Xcode; release.sh signs it again with the real identity when there is one.
release-build: project
	@test -n "$(BUILD_NUMBER)" || { echo "no build number for VERSION=$(VERSION)"; exit 1; }
	$(XCODEBUILD) -scheme Tap -configuration Release -destination 'platform=macOS' TAP_VERSION=$(VERSION) CURRENT_PROJECT_VERSION=$(BUILD_NUMBER) build

# A Release build may inline the accessors the Benchmark check reads through
# nm, so this one reads both the symbols and the strings: the test-only hook
# is in neither, and a defaults key every build carries is in the strings,
# so an empty or renamed binary cannot pass. The UI tests' launch arguments
# (TapDefaultsSuite, TapConfigHome, TapOpenOnLaunch, TapExecutablePath) are
# in every build by design and are not hooks: they read the command line.
check-release-app-hooks: release-build
	@strings -a $(RELEASE_BINARY) | grep -q TapExecutablePath || { echo "no strings to check in $(RELEASE_BINARY)"; exit 1; }
	@! { nm $(RELEASE_BINARY); strings -a $(RELEASE_BINARY); } | grep -q approvalAnswerForTests || { echo "approvalAnswerForTests is in the Release build"; exit 1; }
	@echo "no test-only hook in the Release build"
```

`$(shell ./scripts/build-number.sh ...)` runs in `desktop/`, where `make -C desktop` puts it. A bad `VERSION` prints the script's message and leaves `BUILD_NUMBER` empty; the `@test -n` line then fails the target with the version named.

- [ ] **Step 6: Build and check**

Run: `make frontend && make -C desktop release-build VERSION=1.2.3-beta.4 && make -C desktop check-release-app-hooks VERSION=1.2.3-beta.4`
Expected: the build succeeds; `no test-only hook in the Release build`. Then:

```sh
app=desktop/build/DerivedData/Build/Products/Release/Tap.app
/usr/libexec/PlistBuddy -c 'Print :CFBundleShortVersionString' "$app/Contents/Info.plist"   # 1.2.3-beta.4
/usr/libexec/PlistBuddy -c 'Print :CFBundleVersion' "$app/Contents/Info.plist"              # 1020323
"$app/Contents/Resources/tap" --version                                                     # tap version 1.2.3-beta.4
lipo -archs "$app/Contents/MacOS/Tap"; lipo -archs "$app/Contents/Resources/tap"            # arm64, twice
codesign -dv "$app" 2>&1 | grep -E 'flags=.*runtime|Signature=adhoc'                       # both lines
make -C desktop release-build VERSION=1.100.0 ; echo "exit $?"                              # fails: "minor and patch run to 99"
```

Also run `make -C desktop check-release-hooks` (the Benchmark check) and `make -C desktop test-build` to be sure the Release configuration change reached neither: `no test-only hook in the release build`, and the Debug build still compiles for the host.

- [ ] **Step 7: Commit**

```bash
git add desktop/scripts/build-number.sh desktop/scripts/build-number-test.sh desktop/scripts/build-tap.sh desktop/project.yml desktop/Makefile
git commit -m "build(desktop): an arm64 Release build with a monotonic, label-ranked build number and the hardened runtime"
```

Mutations, applied and run locally then reverted exactly: in `build-number.sh`, give `rc` the base 19 (`build-number-test.sh` fails on `beta.7 < rc.1`); drop the `-gt "$limit"` check (fails on `beta.31`); make the final slot 0 (fails on `2.0.0`); in `build-tap.sh`, drop the `case` (Step 6's `lipo -archs` of the tap still says `arm64` on an Apple silicon Mac: a survivor here, killed on an Intel runner only; the verifier's check is the guard); in the Makefile, point `RELEASE_BINARY` at `/usr/bin/true` (`check-release-app-hooks` fails on "no strings to check").

---

### Task 2: The entitlements, and signing an app inside out

**Files:**
- Create: `desktop/Tap/Tap.entitlements`
- Modify: `desktop/project.yml` (the `Tap` target's `CODE_SIGN_ENTITLEMENTS`, `NSMicrophoneUsageDescription`)
- Create: `desktop/scripts/sign-app.sh`, `desktop/scripts/sign-app-test.sh`

**Interfaces:**
- Consumes: an app bundle as `release-build` leaves it; Sparkle's layout inside `Contents/Frameworks/Sparkle.framework/Versions/B` (`XPCServices/Installer.xpc`, `XPCServices/Downloader.xpc`, `Autoupdate`, `Updater.app`), present from Task 3 on.
- Produces: `desktop/Tap/Tap.entitlements`; `desktop/scripts/sign-app.sh <app> [identity] [entitlements]`; identity defaults to `-`, entitlements to `desktop/Tap/Tap.entitlements`. Exit 0 with the app verified; exit 1 with `codesign`'s message otherwise. Task 7b's `release.sh` calls it once; `verify-release.sh` reads the entitlements back.

- [ ] **Step 1: Write the failing test script**

`desktop/scripts/sign-app-test.sh`:

```sh
#!/bin/sh
# Checks sign-app.sh on a bundle of its own: real executables (copies of
# /usr/bin/true), a Sparkle framework laid out as the real one is, signed
# ad-hoc with the hardened runtime in Sparkle's documented order, the app
# with its entitlements, verified, and signed again without complaint. A
# codesign shim on PATH records the order the real codesign was called in.
set -eu

script="$(cd "$(dirname "$0")" && pwd)/sign-app.sh"
root=$(mktemp -d)
trap 'rm -rf "$root"' EXIT

plist() {
	cat > "$1" <<PLIST
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
<key>CFBundleExecutable</key><string>$2</string>
<key>CFBundleIdentifier</key><string>io.geocod.tap.signtest.$2</string>
<key>CFBundlePackageType</key><string>$3</string>
<key>CFBundleShortVersionString</key><string>0.0.0</string>
<key>CFBundleVersion</key><string>1</string>
</dict></plist>
PLIST
}

app="$root/Fake.app"
mkdir -p "$app/Contents/MacOS" "$app/Contents/Resources"
cp /usr/bin/true "$app/Contents/MacOS/Fake"
cp /usr/bin/true "$app/Contents/Resources/tap"
plist "$app/Contents/Info.plist" Fake APPL
framework="$app/Contents/Frameworks/Sparkle.framework"
versions="$framework/Versions/B"
mkdir -p "$versions/Resources" "$versions/XPCServices/Installer.xpc/Contents/MacOS" "$versions/XPCServices/Downloader.xpc/Contents/MacOS" "$versions/Updater.app/Contents/MacOS"
for executable in "$versions/Sparkle" "$versions/Autoupdate" "$versions/XPCServices/Installer.xpc/Contents/MacOS/Installer" "$versions/XPCServices/Downloader.xpc/Contents/MacOS/Downloader" "$versions/Updater.app/Contents/MacOS/Updater"; do
	cp /usr/bin/true "$executable"
done
plist "$versions/Resources/Info.plist" Sparkle FMWK
plist "$versions/XPCServices/Installer.xpc/Contents/Info.plist" Installer XPC!
plist "$versions/XPCServices/Downloader.xpc/Contents/Info.plist" Downloader XPC!
plist "$versions/Updater.app/Contents/Info.plist" Updater APPL
ln -s B "$framework/Versions/Current"
ln -s Versions/Current/Sparkle "$framework/Sparkle"
ln -s Versions/Current/Resources "$framework/Resources"

entitlements="$root/Test.entitlements"
cat > "$entitlements" <<'PLIST'
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
<key>com.apple.security.device.audio-input</key><true/>
</dict></plist>
PLIST

# The shim: every codesign call's arguments, one per line, then the real tool.
mkdir -p "$root/bin"
cat > "$root/bin/codesign" <<SHIM
#!/bin/sh
printf '%s\n' "\$*" >> "$root/codesign.log"
exec /usr/bin/codesign "\$@"
SHIM
chmod +x "$root/bin/codesign"

PATH="$root/bin:$PATH" "$script" "$app" - "$entitlements" >/dev/null || { echo "ad-hoc signing should succeed"; exit 1; }
for binary in "$app" "$app/Contents/Resources/tap" "$framework" "$versions/Autoupdate" "$versions/Updater.app" "$versions/XPCServices/Installer.xpc" "$versions/XPCServices/Downloader.xpc"; do
	info=$(codesign -dv "$binary" 2>&1)
	echo "$info" | grep -q 'Signature=adhoc' || { echo "$binary: not ad-hoc: $info"; exit 1; }
	echo "$info" | grep -q 'runtime' || { echo "$binary: no hardened runtime: $info"; exit 1; }
done
codesign --verify --deep --strict "$app" || { echo "the bundle should verify"; exit 1; }
codesign -d --entitlements - "$app" 2>&1 | grep -q 'com.apple.security.device.audio-input' || { echo "the app should carry the microphone entitlement"; exit 1; }
if codesign -d --entitlements - "$versions/Autoupdate" 2>&1 | grep -q 'audio-input'; then echo "nested code must carry no entitlement of the app's"; exit 1; fi

# Sparkle's documented order: tap, Installer.xpc, Downloader.xpc (keeping its
# entitlements), Autoupdate, Updater.app, the framework, then the app, then
# one verify.
order=$(grep -v -- '--verify' "$root/codesign.log" | sed -e 's/.*--timestamp=none //' -e 's/.*--entitlements [^ ]* //')
expected="$app/Contents/Resources/tap
$versions/XPCServices/Installer.xpc
--preserve-metadata=entitlements $versions/XPCServices/Downloader.xpc
$versions/Autoupdate
$versions/Updater.app
$framework
$app"
[ "$order" = "$expected" ] || { echo "the signing order is wrong:"; echo "$order"; exit 1; }
grep -q -- "--entitlements $entitlements $app\$" "$root/codesign.log" || { echo "the app should be signed with the entitlements"; exit 1; }
[ "$(grep -c -- '--verify --deep --strict' "$root/codesign.log")" = "1" ] || { echo "one verify at the end"; exit 1; }
if grep -q -- '--deep --strict.*--sign\|--sign.*--deep' "$root/codesign.log"; then echo "never --deep when signing"; exit 1; fi

# Signing an already signed bundle replaces the signature.
"$script" "$app" - "$entitlements" >/dev/null || { echo "a second run should succeed"; exit 1; }

# An identity that does not exist fails with codesign's own message.
if "$script" "$app" "Developer ID Application: Nobody (NOTEAM00)" "$entitlements" >/dev/null 2>&1; then
	echo "an unknown identity should fail"; exit 1
fi

# A path that is not an app, or a missing entitlements file, fails before anything is signed.
if "$script" "$root/missing.app" - "$entitlements" >/dev/null 2>&1; then echo "a missing app should fail"; exit 1; fi
if "$script" "$app" - "$root/missing.entitlements" >/dev/null 2>&1; then echo "missing entitlements should fail"; exit 1; fi

echo "sign-app.sh is right"
```

- [ ] **Step 2: Run it to verify it fails**

Run: `chmod +x desktop/scripts/sign-app-test.sh && desktop/scripts/sign-app-test.sh`
Expected: `ad-hoc signing should succeed` (the script does not exist).

- [ ] **Step 3: The entitlements, the plist string, and the script**

`desktop/Tap/Tap.entitlements`:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<!-- A recorded talk (tap present, through screencapture) takes the microphone; under the hardened runtime that needs this. -->
	<key>com.apple.security.device.audio-input</key>
	<true/>
</dict>
</plist>
```

In `desktop/project.yml`, under the `Tap` target's `settings.base`, after `PRODUCT_NAME`: `CODE_SIGN_ENTITLEMENTS: Tap/Tap.entitlements`. Under `info.properties`, after `NSAppTransportSecurity`:

```yaml
        NSMicrophoneUsageDescription: Tap records your voice with the screen when you record a talk.
```

`desktop/scripts/sign-app.sh`:

```sh
#!/bin/sh
# Signs an app bundle inside out with the hardened runtime: the bundled tap,
# then Sparkle's own executables and the framework when the app carries
# them, then the app with its entitlements (the microphone, for recorded
# talks); nested code gets none. Ad-hoc ("-") unless an identity is given;
# a real identity also gets a secure timestamp, which notarization
# requires. Never --deep: Apple and Sparkle both say so, since it signs
# nested code with the outer code's entitlements and in the wrong order.
set -eu

app="${1:-}"
identity="${2:--}"
here="$(cd "$(dirname "$0")" && pwd)"
entitlements="${3:-$here/../Tap/Tap.entitlements}"
[ -d "$app" ] && [ -f "$app/Contents/Info.plist" ] || { echo "sign-app.sh: $app is not an app bundle" >&2; exit 1; }
[ -f "$entitlements" ] || { echo "sign-app.sh: $entitlements is missing" >&2; exit 1; }

sign() {
	if [ "$identity" = "-" ]; then
		codesign --force --sign - --options runtime --timestamp=none "$@"
	else
		codesign --force --sign "$identity" --options runtime --timestamp "$@"
	fi
}

[ -f "$app/Contents/Resources/tap" ] && sign "$app/Contents/Resources/tap"

sparkle="$app/Contents/Frameworks/Sparkle.framework"
if [ -d "$sparkle" ]; then
	versions="$sparkle/Versions/B"
	sign "$versions/XPCServices/Installer.xpc"
	sign --preserve-metadata=entitlements "$versions/XPCServices/Downloader.xpc"
	sign "$versions/Autoupdate"
	sign "$versions/Updater.app"
	sign "$sparkle"
fi

for framework in "$app"/Contents/Frameworks/*.framework; do
	[ -d "$framework" ] || continue
	[ "$framework" = "$sparkle" ] && continue
	sign "$framework"
done

sign --entitlements "$entitlements" "$app"
codesign --verify --deep --strict --verbose=1 "$app"
if [ "$identity" = "-" ]; then
	echo "signed $app ad-hoc"
else
	echo "signed $app with $identity"
fi
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `chmod +x desktop/scripts/sign-app.sh && desktop/scripts/sign-app-test.sh`
Expected: `sign-app.sh is right`.

- [ ] **Step 5: Run it against the real Release app**

Run: `make -C desktop release-build VERSION=0.0.0-dev && desktop/scripts/sign-app.sh desktop/build/DerivedData/Build/Products/Release/Tap.app && codesign -d --entitlements - desktop/build/DerivedData/Build/Products/Release/Tap.app 2>&1 | grep audio-input && /usr/libexec/PlistBuddy -c 'Print :NSMicrophoneUsageDescription' desktop/build/DerivedData/Build/Products/Release/Tap.app/Contents/Info.plist`
Expected: `signed ... ad-hoc` after `codesign --verify` printed `valid on disk` and `satisfies its Designated Requirement`; the entitlement key; the usage string. (Sparkle is not in the app until Task 3; Task 3's Step 8 repeats this run with the framework present.)

- [ ] **Step 6: Commit**

```bash
git add desktop/Tap/Tap.entitlements desktop/project.yml desktop/scripts/sign-app.sh desktop/scripts/sign-app-test.sh
git commit -m "build(desktop): the microphone entitlement, and signing the app inside out with the hardened runtime"
```

Mutations, applied locally and reverted: drop `--options runtime` from the ad-hoc branch (`sign-app-test.sh` fails on "no hardened runtime"); sign the framework before its XPC services (fails on "the signing order is wrong"); sign the app before tap (the same); drop `--entitlements` from the app's line (fails on "the app should carry the microphone entitlement"); sign `Autoupdate` with the entitlements too (fails on "nested code must carry no entitlement"); drop the `[ -d "$app" ]` guard (fails on "a missing app should fail").

---

### Task 3: Sparkle in the app, and the rule that a talk is never interrupted

**Files:**
- Modify: `desktop/project.yml` (the `Sparkle` package, the `Tap` target's dependency, `SUFeedURL`, `SUPublicEDKey`, `SURequireSignedFeed`, `SUVerifyUpdateBeforeExtraction`)
- Create: `desktop/TapDesktopCore/Sources/TapDesktopCore/UpdateGate.swift`
- Create: `desktop/TapDesktopCore/Tests/TapDesktopCoreTests/UpdateGateTests.swift`
- Create: `desktop/Tap/App/UpdateController.swift`
- Modify: `desktop/Tap/App/AppDelegate.swift` (`updateController`, `applicationWillFinishLaunching`, `applicationDidFinishLaunching`, `checkForUpdates(_:)`, `validateMenuItem`)
- Modify: `desktop/Tap/App/MainMenu.swift` (`tapMenu`: Check for Updates… gets its action)
- Modify: `desktop/Tap/Windows/DeckWindowController.swift` (`startPresenting` and `startAfterTheHint`: refused while an update session is in progress; `showTalkNotStarted(reason:)` factored out of `startAfterTheHint`)
- Modify: `desktop/Tap/Documents/DocumentBar.swift` (`DocumentBarView.detail` is kept, as `message` is, so a test can read the reason)
- Create: `desktop/TapTests/UpdaterTests.swift`

**Interfaces:**
- Consumes: `AppEnvironment.shared.updatesMayInterrupt`, `isPresenting`, `presentingDidChangeNotification` (posted by `noteTalkStarted`, `noteTalkEnded` and `noteTalkWindowsWentDown`), `endingTalks` (D4); `PresentationController.isEnding`, `windowsGoingDown`, `PresentationWindow.anyIsBusyWithFullScreen` (D4); `DeckDocument.sessionController?.presentationIfCreated` (D4); `DocumentBarView(kind: .talkNotStarted, ...)`, `editorViewController.showBar`, `bar(_:)` (D4); `PresentingTestCase.openDeckForPresenting`, `startPresenting`, `stopPresenting` (D4); D6's `menu.addItem(item("Check for Updates…", action: nil))` in `MainMenu.tapMenu`; Sparkle 2.10.0's `SPUStandardUpdaterController(startingUpdater:updaterDelegate:userDriverDelegate:)`, `startUpdater()`, `checkForUpdates(_:)`, `updater`, `SPUUpdater.canCheckForUpdates`, `sessionInProgress`, `automaticallyChecksForUpdates`, `SPUUpdaterDelegate.updater(_:mayPerform:) throws`, `updater(_:shouldProceedWithUpdate:updateCheck:) throws`, `updater(_:shouldPostponeRelaunchForUpdate:untilInvokingBlock:) -> Bool`, `SUAppcastItem.empty()`.
- Produces: `UpdateGate` (`mayCheck(mayInterrupt:) -> Bool`, `mayProceed(mayInterrupt:) -> Bool`, `shouldPostponeRelaunch(mayInterrupt:resume:) -> Bool`, `resumePostponedRelaunch()`, `postponedRelaunch: (() -> Void)?`, `static updaterMayStart(bundleVersion:isHostedByTests:hasTestDefaultsSuite:) -> Bool`, `static let presentingMessage`, `static let updateInProgressMessage`, `UpdateGate.PresentingError`); `UpdateController` (`gate`, `controller`, `updater`, `isStarted`, `startIfAllowed()`, `checkForUpdates(_:)`, `canCheckForUpdates`, `isUpdateSessionInProgress: () -> Bool` (a seam), `talkWindowsAreDown`, `static isHostedByTests`, `static hasTestDefaultsSuite`, `static runsUnderTests`, `static bundleVersion`); `AppDelegate.updateController`, `AppDelegate.checkForUpdates(_:)`; `DeckWindowController.showTalkNotStarted(reason:)`; `DocumentBarView.detail: String`. Task 10's checkbox reads `updateController.updater.automaticallyChecksForUpdates`.

- [ ] **Step 1: Write the failing Core tests**

`desktop/TapDesktopCore/Tests/TapDesktopCoreTests/UpdateGateTests.swift`:

```swift
import XCTest
@testable import TapDesktopCore

final class UpdateGateTests: XCTestCase {
    func testAChecksWaitsForTheTalk() {
        let gate = UpdateGate()
        XCTAssertTrue(gate.mayCheck(mayInterrupt: true))
        XCTAssertFalse(gate.mayCheck(mayInterrupt: false), "no check during a talk, scheduled or not")
    }

    func testAFoundUpdateIsDroppedWhilePresenting() {
        let gate = UpdateGate()
        XCTAssertTrue(gate.mayProceed(mayInterrupt: true))
        XCTAssertFalse(gate.mayProceed(mayInterrupt: false), "a check that started before Play must not prompt during the talk")
        XCTAssertEqual(UpdateGate.PresentingError().localizedDescription, UpdateGate.presentingMessage)
    }

    func testAPostponedRelaunchRunsOnceWhenTheTalkEnds() {
        let gate = UpdateGate()
        var ran = 0
        XCTAssertFalse(gate.shouldPostponeRelaunch(mayInterrupt: true) { ran += 1 }, "no talk, no postponement")
        XCTAssertNil(gate.postponedRelaunch)
        XCTAssertEqual(ran, 0, "Sparkle relaunches itself when nothing is postponed")

        XCTAssertTrue(gate.shouldPostponeRelaunch(mayInterrupt: false) { ran += 1 })
        XCTAssertNotNil(gate.postponedRelaunch)
        gate.resumePostponedRelaunch()
        XCTAssertEqual(ran, 1)
        XCTAssertNil(gate.postponedRelaunch, "run once, then forgotten")
        gate.resumePostponedRelaunch()
        XCTAssertEqual(ran, 1, "a second talk ending runs nothing")
    }

    func testASecondPostponementKeepsTheNewestBlock() {
        let gate = UpdateGate()
        var order: [String] = []
        _ = gate.shouldPostponeRelaunch(mayInterrupt: false) { order.append("first") }
        _ = gate.shouldPostponeRelaunch(mayInterrupt: false) { order.append("second") }
        gate.resumePostponedRelaunch()
        XCTAssertEqual(order, ["second"], "Sparkle asks again for the same update; the newest handler is the live one")
    }

    func testTheUpdaterStartsOnlyInAReleaseOutsideTests() {
        XCTAssertTrue(UpdateGate.updaterMayStart(bundleVersion: "2.1.0", isHostedByTests: false, hasTestDefaultsSuite: false))
        XCTAssertFalse(UpdateGate.updaterMayStart(bundleVersion: "2.1.0", isHostedByTests: true, hasTestDefaultsSuite: false), "a hosted test process")
        XCTAssertFalse(UpdateGate.updaterMayStart(bundleVersion: "2.1.0", isHostedByTests: false, hasTestDefaultsSuite: true), "a UI test launch")
        XCTAssertFalse(UpdateGate.updaterMayStart(bundleVersion: "0.0.0", isHostedByTests: false, hasTestDefaultsSuite: false), "a Debug build from DerivedData")
        XCTAssertFalse(UpdateGate.updaterMayStart(bundleVersion: "0.0.0-dev", isHostedByTests: false, hasTestDefaultsSuite: false), "a local dry run")
        XCTAssertFalse(UpdateGate.updaterMayStart(bundleVersion: nil, isHostedByTests: false, hasTestDefaultsSuite: false))
        XCTAssertTrue(UpdateGate.updaterMayStart(bundleVersion: "2.1.0-beta.3", isHostedByTests: false, hasTestDefaultsSuite: false), "a pre-release checks the feed like a final")
    }
}
```

- [ ] **Step 2: Run them to verify they fail**

Run: `make -C desktop core-test`
Expected: compile errors on `UpdateGate`.

- [ ] **Step 3: Write `UpdateGate`**

`desktop/TapDesktopCore/Sources/TapDesktopCore/UpdateGate.swift`:

```swift
import Foundation

/// The rule between Sparkle and a talk: while a talk runs, no check starts,
/// no found update is shown, and a relaunch waits for the talk's windows to
/// come down. The app's updater asks this object at each of Sparkle's
/// decision points with `AppEnvironment.updatesMayInterrupt`, and tells it
/// when the last talk is down.
public final class UpdateGate {
    public static let presentingMessage = "Tap does not check for updates during a talk."
    public static let updateInProgressMessage = "Sparkle is checking for or installing an update. Let it finish, or close its window, then press Play again."

    /// What Sparkle's delegate throws to refuse a check or a found update
    /// while a talk runs. Sparkle shows the message in its own alert for a
    /// check the person asked for and logs it for a scheduled one.
    public struct PresentingError: LocalizedError {
        public init() {}
        public var errorDescription: String? { UpdateGate.presentingMessage }
    }

    /// The relaunch Sparkle was told to wait with, until the talk is down.
    public private(set) var postponedRelaunch: (() -> Void)?

    public init() {}

    /// Whether a check may start now.
    public func mayCheck(mayInterrupt: Bool) -> Bool {
        mayInterrupt
    }

    /// Whether an update a check found may be shown now.
    public func mayProceed(mayInterrupt: Bool) -> Bool {
        mayInterrupt
    }

    /// Returns true when the relaunch is postponed, keeping `resume` for
    /// `resumePostponedRelaunch`; false lets Sparkle relaunch now. Sparkle
    /// asks again for the same update, so a later call replaces the block.
    public func shouldPostponeRelaunch(mayInterrupt: Bool, resume: @escaping () -> Void) -> Bool {
        guard !mayInterrupt else { return false }
        postponedRelaunch = resume
        return true
    }

    /// Runs a postponed relaunch, once.
    public func resumePostponedRelaunch() {
        let resume = postponedRelaunch
        postponedRelaunch = nil
        resume?()
    }

    /// Whether the updater starts at all: only in a build with a release
    /// version and never in a process that hosts tests or was launched by a
    /// UI test. A 0.0.0 build is a Debug build or a local dry run, which
    /// must never offer to replace itself from the feed.
    public static func updaterMayStart(bundleVersion: String?, isHostedByTests: Bool, hasTestDefaultsSuite: Bool) -> Bool {
        guard !isHostedByTests, !hasTestDefaultsSuite, let bundleVersion else { return false }
        return !bundleVersion.hasPrefix("0.0.0")
    }
}
```

- [ ] **Step 4: Run the Core tests to verify they pass**

Run: `make -C desktop core-test`
Expected: the five `UpdateGateTests` pass with the rest.

- [ ] **Step 5: Write the failing hosted tests**

`desktop/TapTests/UpdaterTests.swift`:

```swift
import XCTest
import Sparkle
@testable import Tap

/// Sparkle's controller in the app: never started under tests, wired to
/// Tap > Check for Updates…, and held back by every talk.
final class UpdaterTests: PresentingTestCase {
    var updates: UpdateController { (NSApp.delegate as! AppDelegate).updateController }

    override func tearDown() async throws {
        updates.isUpdateSessionInProgress = { false }
        try await super.tearDown()
    }

    func testTheUpdaterNeverStartsUnderTests() {
        XCTAssertTrue(UpdateController.runsUnderTests)
        XCTAssertFalse(updates.isStarted, "a hosted test process never reaches the feed")
        updates.startIfAllowed()
        XCTAssertFalse(updates.isStarted, "not even when asked")
        XCTAssertFalse(updates.canCheckForUpdates)
    }

    func testCheckForUpdatesIsInTheTapMenu() throws {
        let tapMenu = try XCTUnwrap(NSApp.mainMenu?.items.first { $0.title == "Tap" }?.submenu)
        let item = try XCTUnwrap(tapMenu.items.first { $0.title == "Check for Updates…" })
        XCTAssertEqual(item.action, #selector(AppDelegate.checkForUpdates(_:)))
        XCTAssertNil(item.target, "nil-targeted, so the responder chain reaches the app delegate")
        let delegate = try XCTUnwrap(NSApp.delegate as? AppDelegate)
        XCTAssertFalse(delegate.validateMenuItem(item), "disabled while the updater is not started, as under tests")
    }

    func testTheFeedAndTheKeysAreInThePlist() {
        let info = Bundle.main.infoDictionary ?? [:]
        XCTAssertEqual(info["SUFeedURL"] as? String, "https://github.com/MiniCodeMonkey/tap/releases/latest/download/appcast.xml")
        XCTAssertEqual(info["SUPublicEDKey"] as? String, "Pc0PbtYL9fkyK3XnqULlpKLlH94TE4OWx4Q3n74EftU=")
        XCTAssertEqual(info["SURequireSignedFeed"] as? Bool, true, "an unsigned feed is refused before anything is shown")
        XCTAssertEqual(info["SUVerifyUpdateBeforeExtraction"] as? Bool, true, "which a signed feed requires")
        XCTAssertNil(info["SUEnableAutomaticChecks"], "Sparkle asks the person on the second launch; the app does not decide for them")
        XCTAssertNotNil(info["NSMicrophoneUsageDescription"], "a recorded talk takes the microphone")
    }

    func testNoCheckDuringATalk() async throws {
        let (_, controller) = try await openDeckForPresenting()
        try await startPresenting(controller, PresentationOptions(mode: .rehearse, startSlide: 1))
        XCTAssertFalse(AppEnvironment.shared.updatesMayInterrupt)
        XCTAssertThrowsError(try updates.updater(updates.updater, mayPerform: .updatesInBackground)) { error in
            XCTAssertEqual(error.localizedDescription, UpdateGate.presentingMessage)
        }
        XCTAssertThrowsError(try updates.updater(updates.updater, mayPerform: .updates), "the person's own check waits too")
        XCTAssertFalse(updates.canCheckForUpdates)
        try await stopPresenting(controller)
        XCTAssertNoThrow(try updates.updater(updates.updater, mayPerform: .updatesInBackground))
    }

    func testAnUpdateFoundDuringATalkIsDropped() async throws {
        let (_, controller) = try await openDeckForPresenting()
        XCTAssertNoThrow(try updates.updater(updates.updater, shouldProceedWithUpdate: SUAppcastItem.empty(), updateCheck: .updatesInBackground))
        try await startPresenting(controller, PresentationOptions(mode: .rehearse, startSlide: 1))
        XCTAssertThrowsError(try updates.updater(updates.updater, shouldProceedWithUpdate: SUAppcastItem.empty(), updateCheck: .updatesInBackground)) { error in
            XCTAssertEqual(error.localizedDescription, UpdateGate.presentingMessage)
        }
        try await stopPresenting(controller)
    }

    func testAPostponedRelaunchRunsWhenTheTalkWindowsAreDown() async throws {
        let (_, controller) = try await openDeckForPresenting()
        var relaunched = 0
        XCTAssertFalse(updates.updater(updates.updater, shouldPostponeRelaunchForUpdate: SUAppcastItem.empty()) { relaunched += 1 }, "no talk: Sparkle relaunches now")
        XCTAssertEqual(relaunched, 0)

        try await startPresenting(controller, PresentationOptions(mode: .rehearse, startSlide: 1))
        XCTAssertTrue(updates.updater(updates.updater, shouldPostponeRelaunchForUpdate: SUAppcastItem.empty()) { relaunched += 1 })
        XCTAssertEqual(relaunched, 0, "the talk runs; the app stays")

        // The talk ends (updatesMayInterrupt turns true) before its windows
        // are down; the relaunch waits for the windows, never mid-transition.
        controller.presentation.stop()
        try await waitUntil(timeout: 30, "the talk to end") { controller.presentation.state == .idle }
        if !controller.presentation.windowsGoingDown.isEmpty {
            XCTAssertEqual(relaunched, 0, "windows still going down: no relaunch yet")
        }
        try await waitUntil(timeout: 20, "the postponed relaunch, once the windows are down") { relaunched == 1 }
        XCTAssertTrue(updates.talkWindowsAreDown)
        XCTAssertNil(updates.gate.postponedRelaunch)
    }

    func testPlayWaitsForAnUpdateInProgress() async throws {
        let (_, controller) = try await openDeckForPresenting()
        let deckWindow = try XCTUnwrap(controller.editor.window?.windowController as? DeckWindowController)
        updates.isUpdateSessionInProgress = { true }
        deckWindow.startPresenting(PresentationOptions(mode: .rehearse, startSlide: 1))
        XCTAssertEqual(controller.presentation.state, .idle, "nothing started")
        let bar = try XCTUnwrap(controller.editorViewController.bar(.talkNotStarted), "the bar says why")
        XCTAssertEqual(bar.detail, UpdateGate.updateInProgressMessage)
        XCTAssertFalse(AppEnvironment.shared.isPresenting)

        updates.isUpdateSessionInProgress = { false }
        deckWindow.startPresenting(PresentationOptions(mode: .rehearse, startSlide: 1))
        try await waitUntil(timeout: 40, "the talk") { controller.presentation.state == .presenting }
        try await stopPresenting(controller)
    }
}
```

Note: `SUAppcastItem.empty()` is Sparkle's `+emptyAppcastItem`, which the review typechecked against 2.10.0. `DocumentBarView` on `main` keeps `kind` and `message` and takes `detail` only as an `init` parameter; Step 7 stores it. TapTests finds the `Sparkle` module through `BUILT_PRODUCTS_DIR` (the review's typecheck used an explicit `-F`); if CI's build of `TapTests` cannot find it, add `- package: Sparkle` with `link: false` to the `TapTests` target's dependencies in `project.yml` and never embed a second copy.

- [ ] **Step 6: Build to verify they fail**

Run: `make -C desktop test-build`
Expected: compile errors: no module `Sparkle`, no `UpdateController`, no `AppDelegate.checkForUpdates`, no `showTalkNotStarted`.

- [ ] **Step 7: The package, the plist keys, the controller, the delegate, the menu, the Play guard**

In `desktop/project.yml`, under `packages`:

```yaml
  Sparkle:
    url: https://github.com/sparkle-project/Sparkle
    exactVersion: 2.10.0
```

Under the `Tap` target's `dependencies`, after `- package: TapDesktopCore`: `- package: Sparkle`. Under `info.properties`, after `NSMicrophoneUsageDescription`:

```yaml
        # Sparkle: the feed is the latest GitHub release's appcast, the key
        # checks every update's, the notes' and the feed's own EdDSA
        # signature, and the feed must carry one (which needs the update
        # verified before extraction). The private key never leaves the
        # person's keychain and CI's secret.
        SUFeedURL: https://github.com/MiniCodeMonkey/tap/releases/latest/download/appcast.xml
        SUPublicEDKey: Pc0PbtYL9fkyK3XnqULlpKLlH94TE4OWx4Q3n74EftU=
        SURequireSignedFeed: true
        SUVerifyUpdateBeforeExtraction: true
```

`desktop/Tap/App/UpdateController.swift`:

```swift
import AppKit
import Sparkle

/// Sparkle's standard controller and its delegate, with the app's one rule:
/// a talk is never interrupted. Every decision reads `updatesMayInterrupt`
/// through `UpdateGate`, a relaunch a talk postponed runs once the last
/// talk's windows are down, and Play is refused while an update session is
/// in progress.
@MainActor
final class UpdateController: NSObject, SPUUpdaterDelegate {
    let gate = UpdateGate()
    private(set) var controller: SPUStandardUpdaterController!
    /// Whether `startUpdater` has run. Tests and 0.0.0 builds never start it.
    private(set) var isStarted = false
    private var presentingObserver: NSObjectProtocol?
    /// Whether Sparkle is between a check and its end (an alert up, a
    /// download, an install waiting). Read at Play. A test replaces it.
    lazy var isUpdateSessionInProgress: () -> Bool = { [weak self] in self?.isStarted == true && self?.updater.sessionInProgress == true }

    override init() {
        super.init()
        controller = SPUStandardUpdaterController(startingUpdater: false, updaterDelegate: self, userDriverDelegate: nil)
        presentingObserver = NotificationCenter.default.addObserver(forName: AppEnvironment.presentingDidChangeNotification, object: nil, queue: .main) { [weak self] _ in
            MainActor.assumeIsolated { self?.presentingChanged() }
        }
    }

    deinit {
        if let presentingObserver { NotificationCenter.default.removeObserver(presentingObserver) }
    }

    var updater: SPUUpdater { controller.updater }

    /// A process that hosts tests: xcodebuild sets the configuration path.
    static var isHostedByTests: Bool { ProcessInfo.processInfo.environment["XCTestConfigurationFilePath"] != nil }
    /// A process a UI test launched: it passes its own defaults suite.
    static var hasTestDefaultsSuite: Bool { UserDefaults.standard.string(forKey: "TapDefaultsSuite") != nil }
    static var runsUnderTests: Bool { isHostedByTests || hasTestDefaultsSuite }

    static var bundleVersion: String? {
        Bundle.main.object(forInfoDictionaryKey: "CFBundleShortVersionString") as? String
    }

    /// Starts Sparkle where a release runs for a person; nowhere else.
    func startIfAllowed() {
        guard !isStarted, UpdateGate.updaterMayStart(bundleVersion: Self.bundleVersion, isHostedByTests: Self.isHostedByTests, hasTestDefaultsSuite: Self.hasTestDefaultsSuite) else { return }
        controller.startUpdater()
        isStarted = true
    }

    /// Tap > Check for Updates…: Sparkle's own check, with its own windows.
    func checkForUpdates(_ sender: Any?) {
        controller.checkForUpdates(sender)
    }

    /// The menu item's state: a started updater that Sparkle allows to check
    /// and no talk running.
    var canCheckForUpdates: Bool {
        isStarted && updater.canCheckForUpdates && gate.mayCheck(mayInterrupt: AppEnvironment.shared.updatesMayInterrupt)
    }

    /// Whether every talk is over and its windows are gone: no deck's talk
    /// is ending, no talk that outlived its deck is still ending, and no
    /// talk window is mid full screen. A relaunch runs only then.
    var talkWindowsAreDown: Bool {
        guard AppEnvironment.shared.updatesMayInterrupt, AppEnvironment.shared.endingTalks.isEmpty, !PresentationWindow.anyIsBusyWithFullScreen else { return false }
        return !NSDocumentController.shared.documents.contains { document in
            (document as? DeckDocument)?.sessionController?.presentationIfCreated?.isEnding == true
        }
    }

    /// Talks change in three steps (started, ended, windows down), each
    /// posting the same notification; the postponed relaunch waits for the
    /// last one. A talk whose deck closed during its ending leaves
    /// `endingTalks` one run-loop turn after its windows-down notification,
    /// so the check runs again on the next turn.
    private func presentingChanged() {
        if talkWindowsAreDown { gate.resumePostponedRelaunch(); return }
        DispatchQueue.main.async { [weak self] in
            MainActor.assumeIsolated {
                guard let self, self.talkWindowsAreDown else { return }
                self.gate.resumePostponedRelaunch()
            }
        }
    }

    // MARK: SPUUpdaterDelegate

    func updater(_ updater: SPUUpdater, mayPerform updateCheck: SPUUpdateCheck) throws {
        guard gate.mayCheck(mayInterrupt: AppEnvironment.shared.updatesMayInterrupt) else { throw UpdateGate.PresentingError() }
    }

    func updater(_ updater: SPUUpdater, shouldProceedWithUpdate updateItem: SUAppcastItem, updateCheck: SPUUpdateCheck) throws {
        guard gate.mayProceed(mayInterrupt: AppEnvironment.shared.updatesMayInterrupt) else { throw UpdateGate.PresentingError() }
    }

    func updater(_ updater: SPUUpdater, shouldPostponeRelaunchForUpdate item: SUAppcastItem, untilInvokingBlock installHandler: @escaping () -> Void) -> Bool {
        gate.shouldPostponeRelaunch(mayInterrupt: AppEnvironment.shared.updatesMayInterrupt, resume: installHandler)
    }
}
```

`PresentationController.isEnding` (`isActive || !windowsGoingDown.isEmpty`) and `windowsGoingDown` are `private(set)` on `main`, readable here. If `endingTalks` is not readable from outside `AppEnvironment` on `main`, it is `private(set)` there already; read it as is.

In `AppDelegate.swift`: add `private(set) var updateController: UpdateController!` after the `init`; in `applicationWillFinishLaunching`, before `NSApp.mainMenu = MainMenu.build()`: `updateController = UpdateController()`; in `applicationDidFinishLaunching`, after `AppEnvironment.shared.warmUp()`: `updateController.startIfAllowed()`; add

```swift
    /// Tap > Check for Updates…, Sparkle's standard check.
    @objc func checkForUpdates(_ sender: Any?) {
        updateController.checkForUpdates(sender)
    }
```

and in `validateMenuItem`, before the final `return true`:

```swift
        if menuItem.action == #selector(checkForUpdates(_:)) {
            return updateController.canCheckForUpdates
        }
```

In `MainMenu.tapMenu`, D6's `menu.addItem(item("Check for Updates…", action: nil))` becomes `menu.addItem(item("Check for Updates…", action: #selector(AppDelegate.checkForUpdates(_:))))`. If D6 has not merged, add the item after `About Tap` and a separator, as D6's Task 12 does.

In `desktop/Tap/Documents/DocumentBar.swift`, `DocumentBarView` gains `let detail: String`, assigned from the `init` parameter beside `message` (the label it feeds is unchanged), so a test can read the reason a bar gives.

In `DeckWindowController.swift`, factor the bar out of `startAfterTheHint` into a method the guard at Play uses too:

```swift
    /// The talk did not start; the bar says why, since nothing else would.
    func showTalkNotStarted(reason: String) {
        let bar = DocumentBarView(kind: .talkNotStarted, message: "The talk did not start.", detail: reason,
                                  buttons: [("Dismiss", { [weak self] in
                                      self?.sessionController.editorViewController.hideBar(.talkNotStarted)
                                  })])
        sessionController.editorViewController.showBar(bar)
        refreshPresentingControls()
    }
```

`startAfterTheHint`'s `guard presentation.canStart else { ... }` body becomes `showTalkNotStarted(reason: reason); return` after computing `reason` as today. One guard, in a method both entry points call first: at the top of `startPresenting(_:savingSettings:)` and at the top of `startAfterTheHint(_:)` (the Focus hint's Not Now path reaches `presentation.start` through it, and an update session can begin while the hint is up):

```swift
    /// An update Sparkle is checking for, downloading or ready to install
    /// would put its windows over the talk; the person lets it finish or
    /// quits it first. True when the talk was refused and the bar shown.
    private func refusedForAnUpdateSession() -> Bool {
        guard (NSApp.delegate as? AppDelegate)?.updateController.isUpdateSessionInProgress() == true else { return false }
        showTalkNotStarted(reason: UpdateGate.updateInProgressMessage)
        return true
    }
```

with `if refusedForAnUpdateSession() { return }` as the first line of each. `sessionInProgress` is also true for the second or two a scheduled check fetches the feed and while an unanswered update alert is open, so the message names all three states; a downloaded update deferred to quit is not a session, so Play is never blocked for good.

- [ ] **Step 8: Build, sign with Sparkle present, and hand the hosted tests to CI**

Run: `make -C desktop core-test && make -C desktop test-build && make -C desktop check-release-hooks && make -C desktop release-build VERSION=0.0.0-dev && desktop/scripts/sign-app.sh desktop/build/DerivedData/Build/Products/Release/Tap.app && make -C desktop check-release-app-hooks`
Expected: everything compiles; `no test-only hook` twice; `signed ... ad-hoc` with `Contents/Frameworks/Sparkle.framework` in the bundle (`ls desktop/build/DerivedData/Build/Products/Release/Tap.app/Contents/Frameworks`), and `codesign -dv .../Sparkle.framework/Versions/B/Autoupdate 2>&1 | grep -q adhoc` true. The first Xcode build resolves the Sparkle package from GitHub's release assets (about 10 MB); if it stalls with no bytes arriving (the review saw this once, a network or sandbox matter, not a plan defect), stop it, run `cd desktop && xcodebuild -project Tap.xcodeproj -resolvePackageDependencies` on its own, and run the make target again. Then commit and report: CI's Desktop Tests job runs `UpdaterTests` (eight tests, four of them start a rehearsal); the Desktop UI Tests and Benchmarks jobs prove the updater stays off there (no Sparkle window in their recordings; `UpdateController.runsUnderTests` is true for both); D4's `FocusHintTests` still pass (the Play guard reads a seam that is false when the updater is not started).

- [ ] **Step 9: Commit**

```bash
git add desktop/project.yml desktop/TapDesktopCore/Sources/TapDesktopCore/UpdateGate.swift desktop/TapDesktopCore/Tests/TapDesktopCoreTests/UpdateGateTests.swift desktop/Tap/App/UpdateController.swift desktop/Tap/App/AppDelegate.swift desktop/Tap/App/MainMenu.swift desktop/Tap/Windows/DeckWindowController.swift desktop/Tap/Documents/DocumentBar.swift desktop/TapTests/UpdaterTests.swift
git commit -m "feat(desktop): Sparkle updates through the standard UI, held back by every talk"
```

Mutations, each a patch in `mutations-a/`, the ones that could interrupt a talk first: in `updater(_:mayPerform:)`, drop the guard (`Test: TapTests/UpdaterTests/testNoCheckDuringATalk`; expected: fails on `XCTAssertThrowsError`); in `updater(_:shouldProceedWithUpdate:updateCheck:)`, drop the guard (`Test: .../testAnUpdateFoundDuringATalkIsDropped`; expected: fails on the throw); in `updater(_:shouldPostponeRelaunchForUpdate:untilInvokingBlock:)`, return `false` always (`Test: .../testAPostponedRelaunchRunsWhenTheTalkWindowsAreDown`; expected: fails on `XCTAssertTrue`); in `presentingChanged`, drop both calls (expected: the same test times out on "the postponed relaunch"); in `presentingChanged`, drop the deferred check (into `survivors-a/`: only a deck closed during its talk's ending reaches it, which no hosted test stages); in `talkWindowsAreDown`, drop the `isEnding` scan (into `survivors-a/` when the windows are already down by the time the state turns `.idle` on the runner; the test's conditional assertion says which); in `DeckWindowController.startPresenting`, drop the update-session guard (`Test: .../testPlayWaitsForAnUpdateInProgress`; expected: fails on `.idle`); in `startAfterTheHint`, drop it (into `survivors-a/`: the hosted tests open with the Focus hint marked shown, so Not Now is not on their path; D4's `FocusHintTests.testNotNowStartsTheTalk` keeps the path itself working); in `startIfAllowed`, drop the `updaterMayStart` guard (`Test: .../testTheUpdaterNeverStartsUnderTests`; expected: fails on `isStarted`; on the runner Sparkle may also show its permission alert, which the test does not need); in `AppDelegate.validateMenuItem`, return `true` for the item (`Test: .../testCheckForUpdatesIsInTheTapMenu`; expected: fails). Core mutations, run locally and reverted: in `UpdateGate.resumePostponedRelaunch`, keep `postponedRelaunch` (`testAPostponedRelaunchRunsOnceWhenTheTalkEnds` fails on "run once"); in `updaterMayStart`, drop the `0.0.0` check (`testTheUpdaterStartsOnlyInAReleaseOutsideTests` fails).

---

### Task 4: The DMG

**Files:**
- Create: `desktop/scripts/make-dmg.sh`, `desktop/scripts/make-dmg-test.sh`

**Interfaces:**
- Consumes: a signed app bundle.
- Produces: `desktop/scripts/make-dmg.sh <app> <output.dmg> [volume-name]` (volume name defaults to `Tap`); the image holds `<AppName>.app` and an `Applications` symlink, compressed (`UDZO`), verified; `hdiutil create` is retried up to three times, since hosted runners sometimes answer "Resource busy". Task 7b's `release.sh` and Task 5's `write-appcast.sh` (its length) and Task 6's `render-cask.sh` (its sha256) read the file.

- [ ] **Step 1: Write the failing test script**

`desktop/scripts/make-dmg-test.sh`:

```sh
#!/bin/sh
# Checks make-dmg.sh on a bundle of its own: the image mounts without a
# Finder window, holds the app and an Applications link, verifies, and a
# transient hdiutil failure is retried.
set -eu

script="$(cd "$(dirname "$0")" && pwd)/make-dmg.sh"
root=$(mktemp -d)
mount="$root/mount"
cleanup() {
	[ -d "$mount" ] && hdiutil detach "$mount" -quiet -force >/dev/null 2>&1 || true
	rm -rf "$root"
}
trap cleanup EXIT

app="$root/Fake.app"
mkdir -p "$app/Contents/MacOS"
cp /usr/bin/true "$app/Contents/MacOS/Fake"
printf 'hello' > "$app/Contents/marker.txt"

"$script" "$app" "$root/Fake-1.0.0.dmg" "Fake" >/dev/null || { echo "the DMG should be made"; exit 1; }
[ -f "$root/Fake-1.0.0.dmg" ] || { echo "no DMG"; exit 1; }
hdiutil verify "$root/Fake-1.0.0.dmg" -quiet || { echo "the DMG should verify"; exit 1; }

mkdir -p "$mount"
hdiutil attach "$root/Fake-1.0.0.dmg" -nobrowse -readonly -noverify -quiet -mountpoint "$mount"
[ -f "$mount/Fake.app/Contents/MacOS/Fake" ] || { echo "the app is not in the image"; exit 1; }
[ "$(cat "$mount/Fake.app/Contents/marker.txt")" = "hello" ] || { echo "the app's files did not copy"; exit 1; }
[ -L "$mount/Applications" ] && [ "$(readlink "$mount/Applications")" = "/Applications" ] || { echo "no Applications link"; exit 1; }
entries=0; for entry in "$mount"/*; do [ -e "$entry" ] && entries=$((entries + 1)); done
[ "$entries" = "2" ] || { echo "the image holds more than the app and the link: $(ls -A "$mount")"; exit 1; }
hdiutil detach "$mount" -quiet

# The output is replaced, not appended to.
"$script" "$app" "$root/Fake-1.0.0.dmg" "Fake" >/dev/null || { echo "a second run should replace the DMG"; exit 1; }

# A hdiutil that fails twice with "Resource busy" and then works is retried:
# a shim on PATH fails its first two create calls and hands the rest to the
# real tool.
mkdir -p "$root/bin"
cat > "$root/bin/hdiutil" <<SHIM
#!/bin/sh
if [ "\$1" = create ]; then
	count=\$(cat "$root/attempts" 2>/dev/null || echo 0)
	count=\$((count + 1))
	echo "\$count" > "$root/attempts"
	if [ "\$count" -le 2 ]; then echo "hdiutil: create failed - Resource busy" >&2; exit 1; fi
fi
exec /usr/bin/hdiutil "\$@"
SHIM
chmod +x "$root/bin/hdiutil"
PATH="$root/bin:$PATH" MAKE_DMG_RETRY_DELAY=0 "$script" "$app" "$root/Retry.dmg" "Fake" >/dev/null || { echo "two busy failures should be retried"; exit 1; }
[ "$(cat "$root/attempts")" = "3" ] || { echo "expected three create attempts, got $(cat "$root/attempts")"; exit 1; }
rm -f "$root/attempts"
cat > "$root/bin/hdiutil" <<'SHIM'
#!/bin/sh
if [ "$1" = create ]; then echo "hdiutil: create failed - Resource busy" >&2; exit 1; fi
exec /usr/bin/hdiutil "$@"
SHIM
if PATH="$root/bin:$PATH" MAKE_DMG_RETRY_DELAY=0 "$script" "$app" "$root/Never.dmg" "Fake" >/dev/null 2>&1; then echo "a hdiutil that always fails should fail the script"; exit 1; fi

if "$script" "$root/missing.app" "$root/x.dmg" >/dev/null 2>&1; then echo "a missing app should fail"; exit 1; fi

echo "make-dmg.sh is right"
```

- [ ] **Step 2: Run it to verify it fails**

Run: `chmod +x desktop/scripts/make-dmg-test.sh && desktop/scripts/make-dmg-test.sh`
Expected: `the DMG should be made`.

- [ ] **Step 3: Write the script**

`desktop/scripts/make-dmg.sh`:

```sh
#!/bin/sh
# Writes a compressed disk image holding the app and a link to
# /Applications, from a staging folder of its own. ditto keeps the app's
# signature, resource forks and extended attributes; cp -R does not always.
# Plain by design: no background art, no Finder scripting, so it runs on a
# runner and on a Mac nobody is watching. hdiutil create sometimes fails
# with "Resource busy" on a hosted runner, so it gets three tries.
set -eu

app="${1:-}"
output="${2:-}"
volume="${3:-Tap}"
[ -d "$app" ] || { echo "make-dmg.sh: $app is not a folder" >&2; exit 1; }
[ -n "$output" ] || { echo "make-dmg.sh: no output path" >&2; exit 1; }
delay="${MAKE_DMG_RETRY_DELAY:-5}"

staging=$(mktemp -d)
trap 'rm -rf "$staging"' EXIT

ditto "$app" "$staging/$(basename "$app")"
ln -s /Applications "$staging/Applications"
rm -f "$output"
attempt=1
until hdiutil create -volname "$volume" -srcfolder "$staging" -fs HFS+ -format UDZO -imagekey zlib-level=9 -ov -quiet "$output"; do
	if [ "$attempt" -ge 3 ]; then
		echo "make-dmg.sh: hdiutil create failed three times" >&2
		exit 1
	fi
	attempt=$((attempt + 1))
	echo "make-dmg.sh: hdiutil create failed; attempt $attempt of 3 in ${delay}s" >&2
	sleep "$delay"
	rm -f "$output"
done
hdiutil verify -quiet "$output"
echo "wrote $output"
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `chmod +x desktop/scripts/make-dmg.sh && desktop/scripts/make-dmg-test.sh`
Expected: `make-dmg.sh is right`. No Finder window opened (`-nobrowse`).

- [ ] **Step 5: Run it against the real app**

Run: `desktop/scripts/make-dmg.sh desktop/build/DerivedData/Build/Products/Release/Tap.app desktop/build/Tap-0.0.0-dev.dmg && ls -la desktop/build/Tap-0.0.0-dev.dmg`
Expected: `wrote ...`; a file of tens of megabytes (tap, the frontend, Sparkle). Do not open it.

- [ ] **Step 6: Commit**

```bash
git add desktop/scripts/make-dmg.sh desktop/scripts/make-dmg-test.sh
git commit -m "build(desktop): the DMG, the app and an Applications link, from hdiutil alone, with a retry"
```

Mutations, applied locally and reverted: drop the `ln -s` (`make-dmg-test.sh` fails on "no Applications link"); use `cp -R` and add a `README.txt` to staging (fails on "holds more than"); drop `rm -f "$output"` and `-ov` (the second run fails); make the loop give up after one try (fails on "two busy failures should be retried").

---

### Task 5: The appcast and its signatures

**Files:**
- Create: `desktop/scripts/fetch-sparkle-tools.sh`, `desktop/scripts/sparkle-sign.sh`, `desktop/scripts/sparkle-sign-test.sh`, `desktop/scripts/write-appcast.sh`, `desktop/scripts/write-appcast-test.sh`

**Interfaces:**
- Consumes: the app's `Contents/Info.plist` (`CFBundleShortVersionString`, `CFBundleVersion`, `LSMinimumSystemVersion`), the DMG, the release notes file, Sparkle's `bin/sign_update` (`--ed-key-file -` reads the key from standard input; `-p` prints an archive's signature alone; on a notes file it prepends a signing warning and prints `sparkle:edSignature="..." sparkle:length="..."`; on an `.xml` feed it embeds a `sparkle-signatures` block; `--verify <file> <signature>` checks one).
- Produces: `fetch-sparkle-tools.sh <folder>` (leaves `<folder>/bin/sign_update`, checks the archive's sha256 first; the caller names the folder after the version); `sparkle-sign.sh archive <dmg>` (prints the base64 signature), `sparkle-sign.sh notes <file>` (signs the file in place, prints `<signature> <length>`), `sparkle-sign.sh feed <appcast.xml>` (signs the feed in place, prints nothing); each prints nothing to stdout with `skipped: Sparkle signature of <name> (SPARKLE_PRIVATE_KEY is not set)` on stderr and exit 0 when the key is absent, and exits 1 when the key is set and signing fails; `write-appcast.sh <app> <dmg> <download-url> <release-url> <output.xml> [dmg-signature] [notes-url] [notes-signature] [notes-length]`. Task 7b's `release.sh` calls them in order: notes, appcast, feed.

- [ ] **Step 1: Write the failing appcast test**

`desktop/scripts/write-appcast-test.sh`:

```sh
#!/bin/sh
# Checks write-appcast.sh: one item from the app's own plist, the DMG's
# real length, the arm64 requirement, the notes link with its signature,
# the DMG's signature when there is one and a plain word when there is
# none, and XML that parses.
set -eu

script="$(cd "$(dirname "$0")" && pwd)/write-appcast.sh"
root=$(mktemp -d)
trap 'rm -rf "$root"' EXIT

app="$root/Tap.app"
mkdir -p "$app/Contents"
cat > "$app/Contents/Info.plist" <<'PLIST'
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
<key>CFBundleShortVersionString</key><string>2.1.0-beta.3</string>
<key>CFBundleVersion</key><string>2010022</string>
<key>LSMinimumSystemVersion</key><string>14.0</string>
</dict></plist>
PLIST
dmg="$root/Tap-2.1.0-beta.3.dmg"
head -c 12345 /dev/zero > "$dmg"
download="https://github.com/MiniCodeMonkey/tap/releases/download/v2.1.0-beta.3/Tap-2.1.0-beta.3.dmg"
release="https://github.com/MiniCodeMonkey/tap/releases/tag/v2.1.0-beta.3"
notes="https://github.com/MiniCodeMonkey/tap/releases/download/v2.1.0-beta.3/Tap-2.1.0-beta.3.md"

"$script" "$app" "$dmg" "$download" "$release" "$root/appcast.xml" "c2lnbmF0dXJl" "$notes" "bm90ZXM=" 255 >/dev/null || { echo "the appcast should be written"; exit 1; }
xmllint --noout "$root/appcast.xml" || { echo "the appcast should be XML"; exit 1; }
for expected in \
	'<sparkle:version>2010022</sparkle:version>' \
	'<sparkle:shortVersionString>2.1.0-beta.3</sparkle:shortVersionString>' \
	'<sparkle:minimumSystemVersion>14.0</sparkle:minimumSystemVersion>' \
	'<sparkle:hardwareRequirements>arm64</sparkle:hardwareRequirements>' \
	"<sparkle:releaseNotesLink sparkle:edSignature=\"bm90ZXM=\" sparkle:length=\"255\">$notes</sparkle:releaseNotesLink>" \
	"<link>$release</link>" \
	"url=\"$download\"" \
	'length="12345"' \
	'sparkle:edSignature="c2lnbmF0dXJl"' \
	'type="application/octet-stream"' \
	'<title>Tap 2.1.0-beta.3</title>' \
	'xmlns:sparkle="http://www.andymatuschak.org/xml-namespaces/sparkle"'; do
	grep -Fq "$expected" "$root/appcast.xml" || { echo "missing: $expected"; cat "$root/appcast.xml"; exit 1; }
done
[ "$(grep -c '<item>' "$root/appcast.xml")" = "1" ] || { echo "one item"; exit 1; }
grep -Eq '<pubDate>[A-Z][a-z]{2}, [0-9]{2} [A-Z][a-z]{2} [0-9]{4} [0-9]{2}:[0-9]{2}:[0-9]{2} \+0000</pubDate>' "$root/appcast.xml" || { echo "no RFC 822 pubDate"; exit 1; }
if grep -q 'unsigned' "$root/appcast.xml"; then echo "a signed appcast carries no unsigned note"; exit 1; fi

# Without a DMG signature the item is marked unsigned, and the attribute is absent.
"$script" "$app" "$dmg" "$download" "$release" "$root/unsigned.xml" >/dev/null
xmllint --noout "$root/unsigned.xml"
if grep -q 'edSignature' "$root/unsigned.xml"; then echo "an unsigned appcast must carry no signature attribute"; exit 1; fi
grep -Fq 'unsigned: SPARKLE_PRIVATE_KEY was not set' "$root/unsigned.xml" || { echo "the unsigned appcast should say so"; exit 1; }
if grep -q 'releaseNotesLink' "$root/unsigned.xml"; then echo "no notes url, no notes link"; exit 1; fi

# Notes without a signature are a link without attributes (a dry run with notes).
"$script" "$app" "$dmg" "$download" "$release" "$root/plain-notes.xml" "" "$notes" >/dev/null
grep -Fq "<sparkle:releaseNotesLink>$notes</sparkle:releaseNotesLink>" "$root/plain-notes.xml" || { echo "unsigned notes are a plain link"; exit 1; }

# A character XML must escape never reaches the feed; the writer refuses rather than corrupting it.
if "$script" "$app" "$dmg" 'https://example.com/a"b.dmg' "$release" "$root/bad.xml" >/dev/null 2>&1; then echo "a quote in a URL should fail"; exit 1; fi
if "$script" "$root/none.app" "$dmg" "$download" "$release" "$root/x.xml" >/dev/null 2>&1; then echo "a missing app should fail"; exit 1; fi
if "$script" "$app" "$dmg" "$download" "$release" "$root/x.xml" "" "$notes" "bm90ZXM=" >/dev/null 2>&1; then echo "a notes signature without a length should fail"; exit 1; fi

echo "write-appcast.sh is right"
```

- [ ] **Step 2: Run it to verify it fails**

Run: `chmod +x desktop/scripts/write-appcast-test.sh && desktop/scripts/write-appcast-test.sh`
Expected: `the appcast should be written`.

- [ ] **Step 3: Write the appcast writer**

`desktop/scripts/write-appcast.sh`:

```sh
#!/bin/sh
# Writes the Sparkle appcast for one release: a feed with one item, read
# from the built app's own Info.plist and the DMG on disk, so the feed can
# never name a version the app does not carry. The item requires arm64
# (the release is Apple silicon only) so an Intel Mac is never offered it.
# The signatures come from sparkle-sign.sh: the DMG's on the enclosure, the
# notes' on their link. Without a DMG signature the item says it is
# unsigned; release.sh then names the file appcast-unsigned.xml and the job
# never uploads it, and the app would refuse it anyway (SURequireSignedFeed).
set -eu

app="${1:-}"; dmg="${2:-}"; download_url="${3:-}"; release_url="${4:-}"; output="${5:-}"
dmg_signature="${6:-}"; notes_url="${7:-}"; notes_signature="${8:-}"; notes_length="${9:-}"
plist="$app/Contents/Info.plist"
[ -f "$plist" ] || { echo "write-appcast.sh: $app has no Info.plist" >&2; exit 1; }
[ -f "$dmg" ] || { echo "write-appcast.sh: $dmg is missing" >&2; exit 1; }
[ -n "$output" ] || { echo "write-appcast.sh: no output path" >&2; exit 1; }
if [ -n "$notes_signature" ] && [ -z "$notes_length" ]; then echo "write-appcast.sh: a notes signature needs the notes length" >&2; exit 1; fi
for value in "$download_url" "$release_url" "$dmg_signature" "$notes_url" "$notes_signature" "$notes_length"; do
	case "$value" in
		*[\"\<\>\&]*) echo "write-appcast.sh: '$value' holds a character XML would need escaped" >&2; exit 1 ;;
	esac
done

read_plist() { /usr/libexec/PlistBuddy -c "Print :$1" "$plist"; }
short_version=$(read_plist CFBundleShortVersionString)
build_number=$(read_plist CFBundleVersion)
minimum_system=$(read_plist LSMinimumSystemVersion)
length=$(stat -f%z "$dmg")
published=$(date -u '+%a, %d %b %Y %H:%M:%S +0000')

if [ -n "$dmg_signature" ]; then
	signature_attribute=" sparkle:edSignature=\"$dmg_signature\""
	unsigned_note=""
else
	signature_attribute=""
	unsigned_note="
    <!-- unsigned: SPARKLE_PRIVATE_KEY was not set when this feed was written. It is never uploaded; the app refuses an unsigned feed. -->"
fi
notes_element=""
if [ -n "$notes_url" ] && [ -n "$notes_signature" ]; then
	notes_element="
      <sparkle:releaseNotesLink sparkle:edSignature=\"$notes_signature\" sparkle:length=\"$notes_length\">$notes_url</sparkle:releaseNotesLink>"
elif [ -n "$notes_url" ]; then
	notes_element="
      <sparkle:releaseNotesLink>$notes_url</sparkle:releaseNotesLink>"
fi

cat > "$output" <<XML
<?xml version="1.0" encoding="utf-8"?>
<rss version="2.0" xmlns:sparkle="http://www.andymatuschak.org/xml-namespaces/sparkle">
  <channel>
    <title>Tap Desktop</title>
    <link>https://github.com/MiniCodeMonkey/tap</link>
    <description>Updates for Tap Desktop</description>
    <language>en</language>$unsigned_note
    <item>
      <title>Tap $short_version</title>
      <link>$release_url</link>
      <pubDate>$published</pubDate>
      <sparkle:version>$build_number</sparkle:version>
      <sparkle:shortVersionString>$short_version</sparkle:shortVersionString>
      <sparkle:minimumSystemVersion>$minimum_system</sparkle:minimumSystemVersion>
      <sparkle:hardwareRequirements>arm64</sparkle:hardwareRequirements>$notes_element
      <enclosure url="$download_url" length="$length" type="application/octet-stream"$signature_attribute/>
    </item>
  </channel>
</rss>
XML
xmllint --noout "$output"
echo "wrote $output"
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `chmod +x desktop/scripts/write-appcast.sh && desktop/scripts/write-appcast-test.sh`
Expected: `write-appcast.sh is right`.

- [ ] **Step 5: Write the failing signing test**

`desktop/scripts/sparkle-sign-test.sh`:

```sh
#!/bin/sh
# Checks sparkle-sign.sh's three modes against a stand-in sign_update that
# records what it was given, and its skip path (no key: nothing on stdout,
# one line on stderr, exit 0, no network). The key reaches sign_update on
# standard input and nowhere else. The real tool runs on CI with the secret.
set -eu

script="$(cd "$(dirname "$0")" && pwd)/sparkle-sign.sh"
root=$(mktemp -d)
trap 'rm -rf "$root"' EXIT
printf 'dmg' > "$root/Tap.dmg"
printf '# Notes\n' > "$root/Tap.md"
printf '<?xml version="1.0"?><rss/>\n' > "$root/appcast.xml"

for mode in archive notes feed; do
	out=$(env -u SPARKLE_PRIVATE_KEY "$script" "$mode" "$root/Tap.dmg" 2>"$root/err") || { echo "$mode: no key should exit 0"; exit 1; }
	[ -z "$out" ] || { echo "$mode: no key should print nothing, got '$out'"; exit 1; }
	grep -Fxq 'skipped: Sparkle signature of Tap.dmg (SPARKLE_PRIVATE_KEY is not set)' "$root/err" || { echo "$mode: the skip line is wrong: $(cat "$root/err")"; exit 1; }
done
out=$(SPARKLE_PRIVATE_KEY="" "$script" archive "$root/Tap.dmg" 2>/dev/null) || { echo "an empty key is no key"; exit 1; }
[ -z "$out" ] || { echo "an empty key should print nothing"; exit 1; }
if "$script" archive "$root/missing.dmg" >/dev/null 2>&1; then echo "a missing file should fail"; exit 1; fi
if "$script" sign "$root/Tap.dmg" >/dev/null 2>&1; then echo "an unknown mode should fail"; exit 1; fi

# The stand-in: records its arguments and its stdin, answers as sign_update
# does for each kind of file, and never verifies anything but "ok".
# The tools folder carries the version, as fetch-sparkle-tools.sh requires.
mkdir -p "$root/tools-2.10.0/bin"
cat > "$root/tools-2.10.0/bin/sign_update" <<'FAKE'
#!/bin/sh
here="$(dirname "$0")"
printf '%s\n' "$*" >> "$here/args"
case "$*" in
	*--verify*) exit 0 ;;
esac
cat > "$here/stdin"
# The file is the last argument.
for file; do :; done
case "$*" in
	*" -p "*) echo "QVJDSElWRQ==" ;;
	*.xml) printf '<!-- sparkle-signatures:\nedSignature: RkVFRA==\nlength: 1\n-->' >> "$file"; exit 0 ;;
	*) printf 'sparkle:edSignature="Tk9URVM=" sparkle:length="42"\n' ;;
esac
FAKE
chmod +x "$root/tools-2.10.0/bin/sign_update"
export SPARKLE_TOOLS="$root/tools-2.10.0"
export SPARKLE_PRIVATE_KEY="bm90LWEta2V5"

out=$("$script" archive "$root/Tap.dmg" 2>"$root/err") || { echo "archive should succeed: $(cat "$root/err")"; exit 1; }
[ "$out" = "QVJDSElWRQ==" ] || { echo "archive should print the signature alone, got '$out'"; exit 1; }
grep -q -- '--ed-key-file - -p' "$root/tools-2.10.0/bin/args" || { echo "archive should read the key from stdin and print the signature alone"; exit 1; }
grep -q -- '--verify' "$root/tools-2.10.0/bin/args" || { echo "archive should verify what it signed"; exit 1; }
[ "$(cat "$root/tools-2.10.0/bin/stdin")" = "bm90LWEta2V5" ] || { echo "the key should reach sign_update on stdin"; exit 1; }
if grep -q 'bm90LWEta2V5' "$root/err" "$root/tools-2.10.0/bin/args"; then echo "the key reached stderr or the command line"; exit 1; fi

out=$("$script" notes "$root/Tap.md" 2>"$root/err") || { echo "notes should succeed: $(cat "$root/err")"; exit 1; }
[ "$out" = "Tk9URVM= 42" ] || { echo "notes should print the signature and the length, got '$out'"; exit 1; }

"$script" feed "$root/appcast.xml" >"$root/out" 2>"$root/err" || { echo "feed should succeed: $(cat "$root/err")"; exit 1; }
[ ! -s "$root/out" ] || { echo "feed should print nothing, got '$(cat "$root/out")'"; exit 1; }
grep -q 'sparkle-signatures:' "$root/appcast.xml" || { echo "feed should be signed in place"; exit 1; }

# A sign_update that says it signed the feed but left no signature block
# fails the feed mode.
cat > "$root/tools-2.10.0/bin/sign_update" <<'FAKE'
#!/bin/sh
cat > /dev/null
exit 0
FAKE
printf '<?xml version="1.0"?><rss/>\n' > "$root/silent.xml"
if "$script" feed "$root/silent.xml" >/dev/null 2>"$root/err"; then echo "a feed left without its block should fail"; exit 1; fi
grep -q 'carries no signature block' "$root/err" || { echo "the feed failure should say why: $(cat "$root/err")"; exit 1; }

# A sign_update that fails fails the script, without the key in the output.
cat > "$root/tools-2.10.0/bin/sign_update" <<'FAKE'
#!/bin/sh
cat > /dev/null
echo "sign_update: bad key" >&2
exit 1
FAKE
if out=$("$script" archive "$root/Tap.dmg" 2>"$root/err"); then echo "a failing sign_update should fail the script"; exit 1; fi
if grep -q 'bm90LWEta2V5' "$root/err"; then echo "the key reached stderr"; exit 1; fi

echo "sparkle-sign.sh is right"
```

- [ ] **Step 6: Write the tools fetcher and the signer**

`desktop/scripts/fetch-sparkle-tools.sh`:

```sh
#!/bin/sh
# Downloads Sparkle's release archive, checks it against the pinned
# checksum, and leaves bin/sign_update in the folder given. The caller
# names the folder after the version (build/sparkle-tools-2.10.0), so a
# bump never trusts an older tool left in place. The version here and
# exactVersion in project.yml move together.
set -eu

folder="${1:-}"
[ -n "$folder" ] || { echo "fetch-sparkle-tools.sh: no folder" >&2; exit 1; }
version="2.10.0"
sha256="c2bf58aa8387266ac179357b1415d6f2635f044da8be41042af32425dae6da0c"
archive="$folder/Sparkle-$version.tar.xz"

case "$folder" in
	*"$version"*) ;;
	*) echo "fetch-sparkle-tools.sh: the folder must carry the version ($version): $folder" >&2; exit 1 ;;
esac
if [ -x "$folder/bin/sign_update" ]; then
	echo "Sparkle $version tools are in $folder"
	exit 0
fi
mkdir -p "$folder"
curl -sSL --fail -o "$archive" "https://github.com/sparkle-project/Sparkle/releases/download/$version/Sparkle-$version.tar.xz"
actual=$(shasum -a 256 "$archive" | cut -d ' ' -f 1)
if [ "$actual" != "$sha256" ]; then
	rm -f "$archive"
	echo "fetch-sparkle-tools.sh: Sparkle-$version.tar.xz has sha256 $actual, not the pinned $sha256" >&2
	exit 1
fi
tar -xJf "$archive" -C "$folder" bin/sign_update bin/generate_keys
rm -f "$archive"
echo "Sparkle $version tools are in $folder"
```

`desktop/scripts/sparkle-sign.sh`:

```sh
#!/bin/sh
# EdDSA signatures for the appcast, from the private key in
# SPARKLE_PRIVATE_KEY, which goes to sign_update on its standard input and
# nowhere else. Three modes:
#   archive <dmg>   prints the DMG's signature (verified before printing)
#   notes <file>    signs the release notes in place (sign_update prepends
#                   its warning) and prints "<signature> <length>"
#   feed <xml>      signs the appcast in place (a sparkle-signatures block)
# Without the key: one skip line on stderr, nothing on stdout, exit 0, and
# the file untouched. SPARKLE_TOOLS names the tools folder; it defaults to
# build/sparkle-tools-2.10.0 beside the scripts' parent.
set -eu

mode="${1:-}"; file="${2:-}"
case "$mode" in archive|notes|feed) ;; *) echo "sparkle-sign.sh: usage: sparkle-sign.sh archive|notes|feed <file>" >&2; exit 1 ;; esac
[ -f "$file" ] || { echo "sparkle-sign.sh: $file is missing" >&2; exit 1; }
here="$(cd "$(dirname "$0")" && pwd)"
tools="${SPARKLE_TOOLS:-$here/../build/sparkle-tools-2.10.0}"
name=$(basename "$file")

if [ -z "${SPARKLE_PRIVATE_KEY:-}" ]; then
	echo "skipped: Sparkle signature of $name (SPARKLE_PRIVATE_KEY is not set)" >&2
	exit 0
fi

"$here/fetch-sparkle-tools.sh" "$tools" >&2
sign_update="$tools/bin/sign_update"
with_key() { printf '%s\n' "$SPARKLE_PRIVATE_KEY" | "$sign_update" "$@"; }

case "$mode" in
	archive)
		signature=$(with_key --ed-key-file - -p "$file")
		[ -n "$signature" ] || { echo "sparkle-sign.sh: sign_update printed no signature for $name" >&2; exit 1; }
		with_key --ed-key-file - --verify "$file" "$signature" >&2
		printf '%s\n' "$signature"
		;;
	notes)
		attributes=$(with_key --ed-key-file - "$file" | grep 'sparkle:edSignature=')
		signature=$(printf '%s' "$attributes" | sed -n 's/.*sparkle:edSignature="\([^"]*\)".*/\1/p')
		length=$(printf '%s' "$attributes" | sed -n 's/.*sparkle:length="\([^"]*\)".*/\1/p')
		[ -n "$signature" ] && [ -n "$length" ] || { echo "sparkle-sign.sh: sign_update printed no signature and length for $name" >&2; exit 1; }
		printf '%s %s\n' "$signature" "$length"
		;;
	feed)
		with_key --ed-key-file - "$file" >&2
		grep -q 'sparkle-signatures:' "$file" || { echo "sparkle-sign.sh: $name carries no signature block after signing" >&2; exit 1; }
		;;
esac
```

- [ ] **Step 7: Run the test to verify it passes**

Run: `chmod +x desktop/scripts/fetch-sparkle-tools.sh desktop/scripts/sparkle-sign.sh desktop/scripts/sparkle-sign-test.sh && desktop/scripts/sparkle-sign-test.sh`
Expected: `sparkle-sign.sh is right`, with no network use (the stand-in `sign_update` is found in place; the folder carries the version). Then, once, to prove the fetcher and the pin: `desktop/scripts/fetch-sparkle-tools.sh desktop/build/sparkle-tools-2.10.0 && desktop/build/sparkle-tools-2.10.0/bin/sign_update --help | head -1` prints `OVERVIEW: Sign or verify an update file using your signing keys.` Never run `generate_keys` without `-p`: it would write a key into the person's keychain.

- [ ] **Step 8: Commit**

```bash
git add desktop/scripts/fetch-sparkle-tools.sh desktop/scripts/sparkle-sign.sh desktop/scripts/sparkle-sign-test.sh desktop/scripts/write-appcast.sh desktop/scripts/write-appcast-test.sh
git commit -m "build(desktop): the Sparkle appcast from the app's own plist, with the DMG, the notes and the feed signed when the key is present"
```

Mutations, applied locally and reverted: in `write-appcast.sh`, always write the signature attribute (`write-appcast-test.sh` fails on "must carry no signature attribute"); drop the `hardwareRequirements` line (fails on the arm64 line); read the version from `$2`'s name instead of the plist (fails on `shortVersionString`); in `sparkle-sign.sh`, print the skip line to stdout (`sparkle-sign-test.sh` fails on "should print nothing"); pass the key with `-s` instead of stdin (fails on "the key reached ... the command line"); in `feed`, drop the `sparkle-signatures` check (fails on "a feed left without its block should fail").

---

### Task 6: The Homebrew cask

**Files:**
- Create: `desktop/release/tap-desktop.rb.template`, `desktop/scripts/render-cask.sh`, `desktop/scripts/render-cask-test.sh`, `desktop/scripts/publish-cask.sh`, `desktop/scripts/publish-cask-test.sh`

**Interfaces:**
- Consumes: the DMG; `HOMEBREW_TAP_TOKEN` and `HOMEBREW_TAP_REPO` as `release.yml`'s formula step uses them; `TAP_RELEASE_NOTARIZED` (`yes` or `no`, from `release-state.env`).
- Produces: `render-cask.sh <version> <dmg> <output.rb>`; `publish-cask.sh <version> <cask.rb>` (skips without the token, skips a pre-release, skips unless `TAP_RELEASE_NOTARIZED=yes`, otherwise commits `Casks/tap-desktop.rb` to the tap and prints the commit hash). Task 7b's `release.sh` renders; Task 8's workflow publishes.

- [ ] **Step 1: Write the failing render test**

`desktop/scripts/render-cask-test.sh`:

```sh
#!/bin/sh
# Checks render-cask.sh: the version and the DMG's sha256 land in the
# template, nothing else changes, Ruby parses it, and brew style passes
# where brew is installed.
set -eu

here="$(cd "$(dirname "$0")" && pwd)"
script="$here/render-cask.sh"
root=$(mktemp -d)
trap 'rm -rf "$root"' EXIT

printf 'not really a dmg' > "$root/Tap-2.1.0.dmg"
sha=$(shasum -a 256 "$root/Tap-2.1.0.dmg" | cut -d ' ' -f 1)
# Under a Casks folder, as in the tap: brew style then applies its cask
# rules and not the generic Ruby ones (Sorbet sigils, frozen strings).
cask="$root/Casks/tap-desktop.rb"

"$script" 2.1.0 "$root/Tap-2.1.0.dmg" "$cask" >/dev/null || { echo "the cask should render"; exit 1; }
grep -Fq 'version "2.1.0"' "$cask" || { echo "no version"; exit 1; }
grep -Fq "sha256 \"$sha\"" "$cask" || { echo "no sha256"; exit 1; }
grep -Fq 'cask "tap-desktop" do' "$cask" || { echo "no cask header"; exit 1; }
grep -Fq 'releases/download/v#{version}/Tap-#{version}.dmg' "$cask" || { echo "no download url"; exit 1; }
grep -Fq 'app "Tap.app"' "$cask" || { echo "no app stanza"; exit 1; }
grep -Fq 'auto_updates true' "$cask" || { echo "Sparkle updates the app; the cask must say so"; exit 1; }
grep -Fq 'depends_on macos: :sonoma' "$cask" || { echo "no macOS floor"; exit 1; }
grep -Fq 'depends_on arch: :arm64' "$cask" || { echo "Apple silicon only; the cask must say so"; exit 1; }
if grep -q '__' "$cask"; then echo "a placeholder is left: $(grep '__' "$cask")"; exit 1; fi
ruby -c "$cask" >/dev/null || { echo "Ruby should parse the cask"; exit 1; }
if command -v brew >/dev/null 2>&1; then
	brew style "$cask" >/dev/null 2>&1 || { echo "brew style should pass"; brew style "$cask" || true; exit 1; }
else
	echo "brew is not installed here; brew style was not run (CI runs it)"
fi

if "$script" 2.1.0 "$root/missing.dmg" "$root/x.rb" >/dev/null 2>&1; then echo "a missing DMG should fail"; exit 1; fi
if "$script" "" "$root/Tap-2.1.0.dmg" "$root/x.rb" >/dev/null 2>&1; then echo "an empty version should fail"; exit 1; fi

echo "render-cask.sh is right"
```

- [ ] **Step 2: Run it to verify it fails**

Run: `chmod +x desktop/scripts/render-cask-test.sh && desktop/scripts/render-cask-test.sh`
Expected: `the cask should render`.

- [ ] **Step 3: Write the template and the renderer**

`desktop/release/tap-desktop.rb.template`:

```ruby
cask "tap-desktop" do
  version "__VERSION__"
  sha256 "__SHA256__"

  url "https://github.com/MiniCodeMonkey/tap/releases/download/v#{version}/Tap-#{version}.dmg"
  name "Tap Desktop"
  desc "Write and present tap decks, with the tap command-line tool built in"
  homepage "https://github.com/MiniCodeMonkey/tap"

  livecheck do
    url :url
    strategy :github_latest
  end

  auto_updates true
  depends_on arch: :arm64
  depends_on macos: :sonoma

  app "Tap.app"

  zap trash: [
    "~/Library/Application Support/Tap",
    "~/Library/Caches/io.geocod.tap.desktop",
    "~/Library/HTTPStorages/io.geocod.tap.desktop",
    "~/Library/Preferences/io.geocod.tap.desktop.plist",
    "~/Library/Saved Application State/io.geocod.tap.desktop.savedState",
    "~/Library/WebKit/io.geocod.tap.desktop",
  ]
end
```

`desktop/scripts/render-cask.sh`:

```sh
#!/bin/sh
# Renders the tap-desktop cask for one release from the template beside
# the scripts: the version and the DMG's sha256 are the only two values,
# so the cask in the tap is always this template at a version.
set -eu

version="${1:-}"; dmg="${2:-}"; output="${3:-}"
here="$(cd "$(dirname "$0")" && pwd)"
template="$here/../release/tap-desktop.rb.template"
[ -n "$version" ] || { echo "render-cask.sh: no version" >&2; exit 1; }
[ -f "$dmg" ] || { echo "render-cask.sh: $dmg is missing" >&2; exit 1; }
[ -n "$output" ] || { echo "render-cask.sh: no output path" >&2; exit 1; }
case "$version" in *[!0-9A-Za-z.-]*) echo "render-cask.sh: '$version' is not a version" >&2; exit 1 ;; esac

sha256=$(shasum -a 256 "$dmg" | cut -d ' ' -f 1)
mkdir -p "$(dirname "$output")"
sed -e "s/__VERSION__/$version/" -e "s/__SHA256__/$sha256/" "$template" > "$output"
if grep -q '__' "$output"; then echo "render-cask.sh: a placeholder was left in $output" >&2; exit 1; fi
echo "wrote $output"
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `chmod +x desktop/scripts/render-cask.sh && desktop/scripts/render-cask-test.sh`
Expected: `render-cask.sh is right` (with `brew style` run, since the Mac has Homebrew).

- [ ] **Step 5: Write the failing publish test and the publisher**

`desktop/scripts/publish-cask-test.sh`:

```sh
#!/bin/sh
# Checks publish-cask.sh: no token skips by name, a DMG that was not
# notarized skips by name even with the token (the token exists today, the
# Apple credentials do not), a pre-release skips by name, and with all
# three the cask lands as Casks/tap-desktop.rb in the tap, which here is a
# bare repository on disk reached through HOMEBREW_TAP_URL. The token
# never appears on stdout.
set -eu

script="$(cd "$(dirname "$0")" && pwd)/publish-cask.sh"
root=$(mktemp -d)
trap 'rm -rf "$root"' EXIT
printf 'cask "tap-desktop" do\nend\n' > "$root/tap-desktop.rb"

out=$(env -u HOMEBREW_TAP_TOKEN TAP_RELEASE_NOTARIZED=yes "$script" 2.1.0 "$root/tap-desktop.rb") || { echo "no token should exit 0"; exit 1; }
[ "$out" = "skipped: cask push (HOMEBREW_TAP_TOKEN is not set)" ] || { echo "wrong skip line: $out"; exit 1; }

out=$(HOMEBREW_TAP_TOKEN=token TAP_RELEASE_NOTARIZED=no "$script" 2.1.0 "$root/tap-desktop.rb") || { echo "not notarized should exit 0"; exit 1; }
[ "$out" = "skipped: cask push (the DMG is not notarized)" ] || { echo "wrong not-notarized line: $out"; exit 1; }
out=$(HOMEBREW_TAP_TOKEN=token env -u TAP_RELEASE_NOTARIZED "$script" 2.1.0 "$root/tap-desktop.rb") || { echo "unknown state should exit 0"; exit 1; }
[ "$out" = "skipped: cask push (the DMG is not notarized)" ] || { echo "an unset state is not notarized: $out"; exit 1; }

out=$(HOMEBREW_TAP_TOKEN=token TAP_RELEASE_NOTARIZED=yes "$script" 2.1.0-beta.1 "$root/tap-desktop.rb") || { echo "a pre-release should exit 0"; exit 1; }
[ "$out" = "skipped: cask push (2.1.0-beta.1 is a pre-release)" ] || { echo "wrong pre-release line: $out"; exit 1; }

# A tap of the test's own: a bare repository with one commit, so the clone has a branch.
git init -q --bare "$root/tap.git"
git clone -q "$root/tap.git" "$root/seed" 2>/dev/null
( cd "$root/seed" && mkdir Formula && printf 'class Tap < Formula\nend\n' > Formula/tap.rb && git add . && git -c user.name=t -c user.email=t@t commit -q -m seed && git push -q origin HEAD 2>/dev/null )
out=$(HOMEBREW_TAP_TOKEN=token TAP_RELEASE_NOTARIZED=yes HOMEBREW_TAP_URL="$root/tap.git" "$script" 2.1.0 "$root/tap-desktop.rb") || { echo "the push should succeed: $out"; exit 1; }
echo "$out" | grep -q '^pushed Casks/tap-desktop.rb for 2.1.0 (' || { echo "no pushed line: $out"; exit 1; }
git clone -q "$root/tap.git" "$root/check" 2>/dev/null
[ "$(cat "$root/check/Casks/tap-desktop.rb")" = "$(cat "$root/tap-desktop.rb")" ] || { echo "the cask in the tap differs"; exit 1; }
[ "$(cd "$root/check" && git log -1 --format=%s)" = "Update tap-desktop to 2.1.0" ] || { echo "wrong commit message"; exit 1; }
if echo "$out" | grep -q token; then echo "the token reached stdout"; exit 1; fi

# The same version again changes nothing and says so.
out=$(HOMEBREW_TAP_TOKEN=token TAP_RELEASE_NOTARIZED=yes HOMEBREW_TAP_URL="$root/tap.git" "$script" 2.1.0 "$root/tap-desktop.rb") || { echo "a repeat should exit 0"; exit 1; }
[ "$out" = "the tap already has this cask for 2.1.0" ] || { echo "wrong repeat line: $out"; exit 1; }

echo "publish-cask.sh is right"
```

`desktop/scripts/publish-cask.sh`:

```sh
#!/bin/sh
# Pushes a rendered cask to the Homebrew tap as Casks/tap-desktop.rb, the
# way release.yml's formula step updates Formula/tap.rb. Skips, by name,
# without HOMEBREW_TAP_TOKEN, unless TAP_RELEASE_NOTARIZED is yes (a cask
# must never point at a DMG Gatekeeper refuses; the token exists before
# the Apple credentials do), and for a pre-release (Homebrew's users get
# finals; Sparkle's feed does the same). HOMEBREW_TAP_REPO names the tap
# (default MiniCodeMonkey/homebrew-tap); HOMEBREW_TAP_URL replaces the
# whole clone URL, which the test uses for a repository on disk. The token
# reaches git as an Authorization header through git's environment
# configuration, never in the URL, argv or .git/config, and never in
# anything printed.
set -eu

version="${1:-}"; cask="${2:-}"
[ -n "$version" ] && [ -f "$cask" ] || { echo "publish-cask.sh: usage: publish-cask.sh <version> <cask.rb>" >&2; exit 1; }

if [ -z "${HOMEBREW_TAP_TOKEN:-}" ]; then
	echo "skipped: cask push (HOMEBREW_TAP_TOKEN is not set)"
	exit 0
fi
if [ "${TAP_RELEASE_NOTARIZED:-no}" != "yes" ]; then
	echo "skipped: cask push (the DMG is not notarized)"
	exit 0
fi
case "$version" in
	*-*) echo "skipped: cask push ($version is a pre-release)"; exit 0 ;;
esac

repository="${HOMEBREW_TAP_REPO:-MiniCodeMonkey/homebrew-tap}"
url="${HOMEBREW_TAP_URL:-https://github.com/${repository}.git}"
if [ -z "${HOMEBREW_TAP_URL:-}" ]; then
	export GIT_CONFIG_COUNT=1
	export GIT_CONFIG_KEY_0="http.https://github.com/.extraheader"
	GIT_CONFIG_VALUE_0="AUTHORIZATION: basic $(printf 'x-access-token:%s' "$HOMEBREW_TAP_TOKEN" | base64 | tr -d '\n')"
	export GIT_CONFIG_VALUE_0
fi
clone=$(mktemp -d)
trap 'rm -rf "$clone"' EXIT

# git's own messages go to stderr; stdout carries only this script's
# lines, which the release job's summary reads.
git clone --quiet "$url" "$clone/tap" 2>"$clone/git.log" || true
cat "$clone/git.log" >&2 || true
[ -d "$clone/tap/.git" ] || { echo "publish-cask.sh: could not clone the tap" >&2; exit 1; }
mkdir -p "$clone/tap/Casks"
cp "$cask" "$clone/tap/Casks/tap-desktop.rb"
cd "$clone/tap"
git add Casks/tap-desktop.rb
if git diff --cached --quiet; then
	echo "the tap already has this cask for $version"
	exit 0
fi
git -c user.name="github-actions[bot]" -c user.email="github-actions[bot]@users.noreply.github.com" commit --quiet -m "Update tap-desktop to $version"
git push --quiet origin HEAD 2>"$clone/git.log" || { cat "$clone/git.log" >&2; echo "publish-cask.sh: the push failed" >&2; exit 1; }
cat "$clone/git.log" >&2 || true
hash=$(git rev-parse --short HEAD)
echo "pushed Casks/tap-desktop.rb for $version ($hash)"
```

- [ ] **Step 6: Run the test to verify it passes**

Run: `chmod +x desktop/scripts/publish-cask.sh desktop/scripts/publish-cask-test.sh && desktop/scripts/publish-cask-test.sh`
Expected: `publish-cask.sh is right`.

- [ ] **Step 7: Commit**

```bash
git add desktop/release/tap-desktop.rb.template desktop/scripts/render-cask.sh desktop/scripts/render-cask-test.sh desktop/scripts/publish-cask.sh desktop/scripts/publish-cask-test.sh
git commit -m "build(desktop): the tap-desktop cask, rendered from a template and pushed to the tap for a notarized final"
```

Mutations, applied locally and reverted: in `render-cask.sh`, substitute `__VERSION__` only (`render-cask-test.sh` fails on "a placeholder is left"); in the template, drop `depends_on arch` (fails on "Apple silicon only"); in `publish-cask.sh`, drop the notarized gate (`publish-cask-test.sh` fails on "wrong not-notarized line"); default `TAP_RELEASE_NOTARIZED` to `yes` (fails on "an unset state is not notarized"); drop the pre-release case (fails on "wrong pre-release line"); print the header value after the push (fails on "the token reached stdout").

---

### Task 7a: The signing identity and notarization

**Files:**
- Create: `desktop/scripts/signing-identity.sh`, `desktop/scripts/signing-identity-test.sh`, `desktop/scripts/notarize.sh`, `desktop/scripts/notarize-test.sh`

**Interfaces:**
- Consumes: `APPLE_DEVELOPER_ID_APPLICATION_P12` (base64 of the `.p12`), `APPLE_DEVELOPER_ID_APPLICATION_PASSWORD`, `APPLE_NOTARY_KEY` (the `.p8` file's text), `APPLE_NOTARY_KEY_ID`, `APPLE_NOTARY_ISSUER_ID`; `xcrun notarytool submit <file> --key <p8> --key-id <id> --issuer <uuid> --wait --timeout 30m --output-format json`, `notarytool log <id> --key ...`, `xcrun stapler staple <target>`.
- Produces: `signing-identity.sh import` (prints the identity's name, or `-` with a skip line on stderr; the keychain path is `$TAP_SIGNING_KEYCHAIN_FILE` when set, else `build/tap-release.keychain-db`; a failed import leaves no keychain), `signing-identity.sh remove` (quiet when there is none); `notarize.sh <file> <staple-target>` (skips by name, or notarizes and staples, printing `notarized <name> (submission <id>) and stapled <target>`). Task 7b's `release.sh` calls both; Task 8's workflow calls `remove` once more, `if: always()`.

- [ ] **Step 1: Write the failing skip-path tests**

`desktop/scripts/signing-identity-test.sh`:

```sh
#!/bin/sh
# Checks signing-identity.sh's paths that need no certificate: import with
# no secret prints "-" and a skip line; remove with no keychain is quiet;
# a secret that is not a p12 fails and leaves no keychain behind and no
# entry in the search list.
set -eu

script="$(cd "$(dirname "$0")" && pwd)/signing-identity.sh"
root=$(mktemp -d)
trap 'rm -rf "$root"' EXIT
export TAP_SIGNING_KEYCHAIN_FILE="$root/test.keychain-db"
before=$(security list-keychains -d user)

out=$(env -u APPLE_DEVELOPER_ID_APPLICATION_P12 "$script" import 2>"$root/err") || { echo "no certificate should exit 0"; exit 1; }
[ "$out" = "-" ] || { echo "no certificate should print -, got '$out'"; exit 1; }
grep -Fxq 'skipped: Developer ID signing (APPLE_DEVELOPER_ID_APPLICATION_P12 is not set)' "$root/err" || { echo "wrong skip line: $(cat "$root/err")"; exit 1; }
[ ! -f "$TAP_SIGNING_KEYCHAIN_FILE" ] || { echo "no keychain without a certificate"; exit 1; }

"$script" remove || { echo "remove with no keychain should exit 0"; exit 1; }

if APPLE_DEVELOPER_ID_APPLICATION_P12="$(printf 'not a certificate' | base64)" APPLE_DEVELOPER_ID_APPLICATION_PASSWORD=pw "$script" import >"$root/out" 2>"$root/err"; then
	echo "a broken p12 should fail"; exit 1
fi
if grep -q 'not a certificate' "$root/err" "$root/out"; then echo "the secret reached the output"; exit 1; fi
[ ! -f "$TAP_SIGNING_KEYCHAIN_FILE" ] || { echo "a failed import should remove its keychain"; exit 1; }
[ "$(security list-keychains -d user)" = "$before" ] || { echo "a failed import changed the search list"; exit 1; }

echo "signing-identity.sh is right"
```

`desktop/scripts/notarize-test.sh`:

```sh
#!/bin/sh
# Checks notarize.sh's skip path: without the three notary secrets it prints
# one line naming the first missing one and exits 0 without touching the
# target. The real path runs on CI with the secrets.
set -eu

script="$(cd "$(dirname "$0")" && pwd)/notarize.sh"
root=$(mktemp -d)
trap 'rm -rf "$root"' EXIT
printf 'zip' > "$root/Tap.zip"
mkdir -p "$root/Tap.app/Contents"

out=$(env -u APPLE_NOTARY_KEY -u APPLE_NOTARY_KEY_ID -u APPLE_NOTARY_ISSUER_ID "$script" "$root/Tap.zip" "$root/Tap.app") || { echo "no secrets should exit 0"; exit 1; }
[ "$out" = "skipped: notarization of Tap.zip (APPLE_NOTARY_KEY is not set)" ] || { echo "wrong skip line: $out"; exit 1; }

out=$(APPLE_NOTARY_KEY=key env -u APPLE_NOTARY_KEY_ID -u APPLE_NOTARY_ISSUER_ID "$script" "$root/Tap.zip" "$root/Tap.app")
[ "$out" = "skipped: notarization of Tap.zip (APPLE_NOTARY_KEY_ID is not set)" ] || { echo "wrong second skip line: $out"; exit 1; }

out=$(APPLE_NOTARY_KEY=key APPLE_NOTARY_KEY_ID=id env -u APPLE_NOTARY_ISSUER_ID "$script" "$root/Tap.zip" "$root/Tap.app")
[ "$out" = "skipped: notarization of Tap.zip (APPLE_NOTARY_ISSUER_ID is not set)" ] || { echo "wrong third skip line: $out"; exit 1; }

if "$script" "$root/missing.zip" "$root/Tap.app" >/dev/null 2>&1; then echo "a missing file should fail"; exit 1; fi
[ -z "$(ls -A "$root/Tap.app/Contents")" ] || { echo "the skip path touched the target"; exit 1; }

echo "notarize.sh is right"
```

- [ ] **Step 2: Run them to verify they fail**

Run: `chmod +x desktop/scripts/signing-identity-test.sh desktop/scripts/notarize-test.sh && desktop/scripts/signing-identity-test.sh; desktop/scripts/notarize-test.sh`
Expected: both fail on their first line (`not found`).

- [ ] **Step 3: Write the identity and notarization scripts**

`desktop/scripts/signing-identity.sh`:

```sh
#!/bin/sh
# import: makes a keychain of its own from the Developer ID Application
# certificate in APPLE_DEVELOPER_ID_APPLICATION_P12 (base64 of the .p12)
# and APPLE_DEVELOPER_ID_APPLICATION_PASSWORD, adds it to the search list,
# and prints the identity's name for codesign. Without the certificate it
# prints "-" (ad-hoc) and a skip line on stderr. An import that fails at
# any step removes the keychain again before exiting.
# remove: deletes that keychain and takes it out of the search list.
# The .p12 exists on disk only inside a private temporary folder for the
# length of the import; its password is an argument of security import
# (an accepted exposure on a single-tenant runner). TAP_SIGNING_KEYCHAIN_FILE
# names the keychain (default build/tap-release.keychain-db next to the
# scripts' parent); the path is made canonical, since security prints
# canonical paths (/private/var for /var) in the search list.
set -eu

here="$(cd "$(dirname "$0")" && pwd)"
keychain="${TAP_SIGNING_KEYCHAIN_FILE:-$here/../build/tap-release.keychain-db}"
mkdir -p "$(dirname "$keychain")"
keychain="$(cd "$(dirname "$keychain")" && pwd -P)/$(basename "$keychain")"
command="${1:-}"

remove_keychain() {
	# The search list without ours; untouched when ours is not in it.
	if security list-keychains -d user | grep -Fq "$keychain"; then
		remaining=$(security list-keychains -d user | tr -d '" ' | grep -Fv "$keychain" || true)
		# shellcheck disable=SC2086
		security list-keychains -d user -s $remaining
	fi
	if [ -f "$keychain" ]; then
		security delete-keychain "$keychain" >/dev/null 2>&1 || rm -f "$keychain"
	fi
}

case "$command" in
	import)
		if [ -z "${APPLE_DEVELOPER_ID_APPLICATION_P12:-}" ]; then
			echo "skipped: Developer ID signing (APPLE_DEVELOPER_ID_APPLICATION_P12 is not set)" >&2
			echo "-"
			exit 0
		fi
		[ -n "${APPLE_DEVELOPER_ID_APPLICATION_PASSWORD:-}" ] || { echo "signing-identity.sh: APPLE_DEVELOPER_ID_APPLICATION_PASSWORD is not set" >&2; exit 1; }
		private=$(mktemp -d)
		chmod 700 "$private"
		imported=no
		# Whatever fails below, the private folder goes and so does a
		# half-made keychain; only a finished import keeps it.
		trap 'rm -rf "$private"; [ "$imported" = yes ] || remove_keychain' EXIT
		umask 077
		printf '%s' "$APPLE_DEVELOPER_ID_APPLICATION_P12" | base64 -d > "$private/certificate.p12" 2>/dev/null || { echo "signing-identity.sh: the certificate secret is not base64" >&2; exit 1; }
		keychain_password=$(head -c 24 /dev/urandom | base64)
		remove_keychain
		security create-keychain -p "$keychain_password" "$keychain"
		security set-keychain-settings -lut 21600 "$keychain"
		security unlock-keychain -p "$keychain_password" "$keychain"
		if ! security import "$private/certificate.p12" -k "$keychain" -P "$APPLE_DEVELOPER_ID_APPLICATION_PASSWORD" -T /usr/bin/codesign -T /usr/bin/security >/dev/null 2>&1; then
			echo "signing-identity.sh: the certificate did not import (wrong password, or not a .p12)" >&2
			exit 1
		fi
		security set-key-partition-list -S apple-tool:,apple:,codesign: -s -k "$keychain_password" "$keychain" >/dev/null
		existing=$(security list-keychains -d user | tr -d '" ')
		# shellcheck disable=SC2086
		security list-keychains -d user -s "$keychain" $existing
		identity=$(security find-identity -v -p codesigning "$keychain" | sed -n 's/.*"\(Developer ID Application: [^"]*\)".*/\1/p' | head -n 1)
		if [ -z "$identity" ]; then
			echo "signing-identity.sh: no Developer ID Application identity in the certificate" >&2
			exit 1
		fi
		imported=yes
		echo "$identity"
		;;
	remove)
		remove_keychain
		;;
	*)
		echo "signing-identity.sh: usage: signing-identity.sh import|remove" >&2
		exit 1
		;;
esac
```

`desktop/scripts/notarize.sh`:

```sh
#!/bin/sh
# Submits a file (a zip of the app, or the DMG) to Apple's notary service
# with an App Store Connect API key, waits for the verdict, and staples the
# ticket to the target (the app the zip holds, or the DMG itself). The key
# exists on disk only in a private temporary folder for the length of the
# run. Without the three secrets: one skip line, exit 0, nothing touched.
set -eu

file="${1:-}"; target="${2:-}"
[ -f "$file" ] || { echo "notarize.sh: $file is missing" >&2; exit 1; }
[ -e "$target" ] || { echo "notarize.sh: $target is missing" >&2; exit 1; }
name=$(basename "$file")

for secret in APPLE_NOTARY_KEY APPLE_NOTARY_KEY_ID APPLE_NOTARY_ISSUER_ID; do
	eval "value=\${$secret:-}"
	if [ -z "$value" ]; then
		echo "skipped: notarization of $name ($secret is not set)"
		exit 0
	fi
done

private=$(mktemp -d)
chmod 700 "$private"
trap 'rm -rf "$private"' EXIT
umask 077
printf '%s\n' "$APPLE_NOTARY_KEY" > "$private/AuthKey.p8"

xcrun notarytool submit "$file" --key "$private/AuthKey.p8" --key-id "$APPLE_NOTARY_KEY_ID" --issuer "$APPLE_NOTARY_ISSUER_ID" --wait --timeout 30m --output-format json > "$private/result.json" || true
status=$(/usr/bin/plutil -extract status raw -o - "$private/result.json" 2>/dev/null || true)
submission=$(/usr/bin/plutil -extract id raw -o - "$private/result.json" 2>/dev/null || true)
if [ "$status" != "Accepted" ]; then
	echo "notarize.sh: $name was not accepted (status: ${status:-none}, submission: ${submission:-none})" >&2
	if [ -n "$submission" ]; then
		xcrun notarytool log "$submission" --key "$private/AuthKey.p8" --key-id "$APPLE_NOTARY_KEY_ID" --issuer "$APPLE_NOTARY_ISSUER_ID" >&2 || true
	fi
	exit 1
fi
xcrun stapler staple "$target" >/dev/null
echo "notarized $name (submission $submission) and stapled $(basename "$target")"
```

`plutil -extract ... raw` reads a JSON file too (`notarytool --output-format json` writes `{"status":"Accepted","id":"..."}`). The notary log holds file paths and issue text, never the key.

- [ ] **Step 4: Run the two tests to verify they pass**

Run: `chmod +x desktop/scripts/signing-identity.sh desktop/scripts/notarize.sh && desktop/scripts/signing-identity-test.sh && desktop/scripts/notarize-test.sh`
Expected: `signing-identity.sh is right`, `notarize.sh is right`. Then `security list-keychains -d user` shows the person's keychains as before (the test compares them itself).

- [ ] **Step 5: Commit**

```bash
git add desktop/scripts/signing-identity.sh desktop/scripts/signing-identity-test.sh desktop/scripts/notarize.sh desktop/scripts/notarize-test.sh
git commit -m "build(desktop): a temporary keychain for the Developer ID identity, and notarization, each skipped by name without its secret"
```

Mutations, applied locally and reverted: in `signing-identity.sh`, set `imported=yes` before `security import` (`signing-identity-test.sh` fails on "a failed import should remove its keychain"); drop the `trap` (the same); in `notarize.sh`, print `$APPLE_NOTARY_KEY` in the skip line (`notarize-test.sh` fails on "wrong skip line"); skip the `for secret` loop's first name (fails on the first skip line).

---

### Task 7b: The verifier, the orchestrator and the Makefile targets

**Files:**
- Create: `desktop/scripts/verify-release.sh`, `desktop/scripts/verify-release-test.sh`, `desktop/scripts/release.sh`, `desktop/scripts/release-test.sh`
- Modify: `desktop/Makefile` (`RELEASE_DIR`, `release-tests`, `release`, `.PHONY`)

**Interfaces:**
- Consumes: every script of Tasks 1 to 7a; `scripts/prepare-changelog.sh <version> <changelog> <notes>` (the repository's; reuses a version's existing section, fails when there is none); `TAP_ENTITLEMENTS` (the entitlements `sign-app.sh` gets; default `desktop/Tap/Tap.entitlements`; the tests set their own); `TAP_RELEASE_IDENTITY` (an identity already in a keychain, used instead of importing one; the tests give a stand-in name with `codesign`, `xcrun` and `spctl` shims on `PATH`, and a person may sign with their own keychain's identity locally).
- Produces: `verify-release.sh <app> <dmg> <appcast> <identity> <version> <notarized yes|no> <feed_signed yes|no>`; `release.sh <version> <app> <output-dir>` writing `Tap-<version>.dmg` or `Tap-<version>-unnotarized.dmg` with its `.sha256`, `Tap-<version>.md` when the changelog has the version's section, `appcast.xml` (signed) or `appcast-unsigned.xml`, `Casks/tap-desktop.rb`, `release-summary.md` and `release-state.env` (`version`, `notarized`, `feed_signed`, `dmg`, `checksum`, `notes`, `appcast`, `cask`: plain words only, no identity, so `bash -eo pipefail -c 'source release-state.env'` always works); a failed notarization stops `release.sh` with exit 1 and `notarized` never says `yes` before the DMG's own submission was accepted; `make -C desktop release VERSION=<v>` (output in `build/release`), `make -C desktop release-tests`. Task 8's workflows call `release`, `release-tests` and read the state file.

- [ ] **Step 1: Write the failing verifier test**

`desktop/scripts/verify-release-test.sh`:

```sh
#!/bin/sh
# Checks verify-release.sh against a release built from a bundle of its
# own: arm64 binaries built with cc, a tap that prints its version, the plist keys, the
# entitlements, no test code. Ad-hoc and not notarized passes with the
# Gatekeeper checks skipped; a real identity that is signed but not
# notarized passes too (a missing notary key must not fail the run); a
# release that claims notarization without a ticket fails; a universal
# binary, a wrong tap version, a missing key, a stray XCTest framework and
# an unsigned "signed" feed each fail by name.
set -eu

here="$(cd "$(dirname "$0")" && pwd)"
script="$here/verify-release.sh"
root=$(mktemp -d)
trap 'rm -rf "$root"' EXIT

make_app() {
	app="$1"; tap_version="$2"
	rm -rf "$app"
	mkdir -p "$app/Contents/MacOS" "$app/Contents/Resources"
	printf 'int main(void) { return 0; }\n' | cc -arch arm64 -x c -o "$app/Contents/MacOS/Tap" -
	printf '#include <stdio.h>\nint main(void) { puts("tap version %s"); return 0; }\n' "$tap_version" | cc -arch arm64 -x c -o "$app/Contents/Resources/tap" -
	cat > "$app/Contents/Info.plist" <<PLIST
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
<key>CFBundleExecutable</key><string>Tap</string>
<key>CFBundleIdentifier</key><string>io.geocod.tap.verifytest</string>
<key>CFBundlePackageType</key><string>APPL</string>
<key>CFBundleShortVersionString</key><string>0.0.0-test</string>
<key>CFBundleVersion</key><string>1</string>
<key>LSMinimumSystemVersion</key><string>14.0</string>
<key>NSMicrophoneUsageDescription</key><string>Tap records your voice with the screen when you record a talk.</string>
<key>SUFeedURL</key><string>https://github.com/MiniCodeMonkey/tap/releases/latest/download/appcast.xml</string>
<key>SUPublicEDKey</key><string>Pc0PbtYL9fkyK3XnqULlpKLlH94TE4OWx4Q3n74EftU=</string>
<key>SURequireSignedFeed</key><true/>
<key>SUVerifyUpdateBeforeExtraction</key><true/>
</dict></plist>
PLIST
}
entitlements="$root/Test.entitlements"
cat > "$entitlements" <<'PLIST'
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
<key>com.apple.security.device.audio-input</key><true/>
</dict></plist>
PLIST
download="https://github.com/MiniCodeMonkey/tap/releases/download/v0.0.0-test/Tap-0.0.0-test.dmg"
release="https://github.com/MiniCodeMonkey/tap/releases/tag/v0.0.0-test"

app="$root/Tap.app"
make_app "$app" 0.0.0-test
"$here/sign-app.sh" "$app" - "$entitlements" >/dev/null
dmg="$root/Tap-0.0.0-test.dmg"
"$here/make-dmg.sh" "$app" "$dmg" Tap >/dev/null
# The DMG is signed before the appcast reads its length, as release.sh does.
codesign --force --sign - "$dmg"
"$here/write-appcast.sh" "$app" "$dmg" "$download" "$release" "$root/appcast.xml" >/dev/null

# Ad-hoc, not notarized, unsigned feed: the dry run.
"$script" "$app" "$dmg" "$root/appcast.xml" - 0.0.0-test no no >/dev/null || { echo "the dry run should verify"; exit 1; }
# A real identity without notarization: the DMG is signed, no ticket is asked for.
"$script" "$app" "$dmg" "$root/appcast.xml" "Developer ID Application: Someone (TEAM)" 0.0.0-test no no >/dev/null || { echo "signed but not notarized should verify"; exit 1; }
# A claim of notarization without a ticket fails.
if "$script" "$app" "$dmg" "$root/appcast.xml" "Developer ID Application: Someone (TEAM)" 0.0.0-test yes no >/dev/null 2>&1; then echo "notarized without a ticket should fail"; exit 1; fi
# A claim of a signed feed without the signature block fails.
if "$script" "$app" "$dmg" "$root/appcast.xml" - 0.0.0-test no yes >/dev/null 2>&1; then echo "a feed claimed signed without its block should fail"; exit 1; fi
# The version must be the app's.
if "$script" "$app" "$dmg" "$root/appcast.xml" - 9.9.9 no no >/dev/null 2>&1; then echo "another version should fail"; exit 1; fi

fails_with() {
	message="$1"; shift
	if "$script" "$@" >"$root/out" 2>&1; then echo "should fail: $message"; exit 1; fi
	grep -Fq "$message" "$root/out" || { echo "wrong failure for '$message': $(cat "$root/out")"; exit 1; }
}
# A tap of another version.
make_app "$root/Wrong.app" 0.0.0-other; "$here/sign-app.sh" "$root/Wrong.app" - "$entitlements" >/dev/null
fails_with "tap version 0.0.0-other" "$root/Wrong.app" "$dmg" "$root/appcast.xml" - 0.0.0-test no no
# An app binary with more than the arm64 slice (/usr/bin/true is fat).
make_app "$root/Universal.app" 0.0.0-test; cp /usr/bin/true "$root/Universal.app/Contents/MacOS/Tap"; "$here/sign-app.sh" "$root/Universal.app" - "$entitlements" >/dev/null
fails_with "is not arm64 alone" "$root/Universal.app" "$dmg" "$root/appcast.xml" - 0.0.0-test no no
# A missing plist key.
make_app "$root/NoKey.app" 0.0.0-test; /usr/libexec/PlistBuddy -c 'Delete :SURequireSignedFeed' "$root/NoKey.app/Contents/Info.plist"; "$here/sign-app.sh" "$root/NoKey.app" - "$entitlements" >/dev/null
fails_with "SURequireSignedFeed" "$root/NoKey.app" "$dmg" "$root/appcast.xml" - 0.0.0-test no no
# No entitlement.
make_app "$root/NoEntitlement.app" 0.0.0-test; printf '<?xml version="1.0" encoding="UTF-8"?>\n<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">\n<plist version="1.0"><dict/></plist>\n' > "$root/Empty.entitlements"; "$here/sign-app.sh" "$root/NoEntitlement.app" - "$root/Empty.entitlements" >/dev/null
fails_with "audio-input" "$root/NoEntitlement.app" "$dmg" "$root/appcast.xml" - 0.0.0-test no no
# Test code in the product.
make_app "$root/Tested.app" 0.0.0-test; mkdir -p "$root/Tested.app/Contents/Frameworks/XCTest.framework" "$root/Tested.app/Contents/PlugIns/TapTests.xctest"; "$here/sign-app.sh" "$root/Tested.app" - "$entitlements" >/dev/null 2>&1 || true
fails_with "test code" "$root/Tested.app" "$dmg" "$root/appcast.xml" - 0.0.0-test no no

echo "verify-release.sh is right"
```

- [ ] **Step 2: Run it to verify it fails**

Run: `chmod +x desktop/scripts/verify-release-test.sh && desktop/scripts/verify-release-test.sh`
Expected: `the dry run should verify` (the script does not exist).

- [ ] **Step 3: Write the verifier**

`desktop/scripts/verify-release.sh`:

```sh
#!/bin/sh
# The checks on a finished release, by its state: the app is arm64 alone
# and so is its tap, which prints the release's version; the plist carries
# the Sparkle keys and the microphone string; the app carries the
# microphone entitlement and the hardened runtime and verifies; no test
# framework or bundle rode along; the DMG verifies; the appcast parses and
# names the same build and length; a notarized release also passes
# Gatekeeper and has its tickets; a signed feed has its signature block.
set -eu

app="${1:-}"; dmg="${2:-}"; appcast="${3:-}"; identity="${4:--}"; version="${5:-}"; notarized="${6:-no}"; feed_signed="${7:-no}"
plist="$app/Contents/Info.plist"
fail() { echo "verify-release.sh: $1" >&2; exit 1; }
[ -f "$plist" ] && [ -f "$dmg" ] && [ -f "$appcast" ] && [ -n "$version" ] || fail "usage: verify-release.sh <app> <dmg> <appcast> <identity> <version> <notarized> <feed_signed>"
read_plist() { /usr/libexec/PlistBuddy -c "Print :$1" "$plist" 2>/dev/null || fail "$1 is missing from the plist"; }

for binary in "$app/Contents/MacOS/Tap" "$app/Contents/Resources/tap"; do
	[ -f "$binary" ] || fail "$binary is missing"
	archs=$(lipo -archs "$binary")
	[ "$archs" = "arm64" ] || fail "$binary is not arm64 alone: $archs"
done
printed=$("$app/Contents/Resources/tap" --version)
[ "$printed" = "tap version $version" ] || fail "the bundled tap prints '$printed', not 'tap version $version'"

[ "$(read_plist CFBundleShortVersionString)" = "$version" ] || fail "the app is $(read_plist CFBundleShortVersionString), not $version"
build_number=$(read_plist CFBundleVersion)
[ -n "$build_number" ] || fail "no CFBundleVersion"
[ "$(read_plist LSMinimumSystemVersion)" = "14.0" ] || fail "LSMinimumSystemVersion is not 14.0"
[ "$(read_plist SUFeedURL)" = "https://github.com/MiniCodeMonkey/tap/releases/latest/download/appcast.xml" ] || fail "SUFeedURL is wrong"
[ "$(read_plist SUPublicEDKey)" = "Pc0PbtYL9fkyK3XnqULlpKLlH94TE4OWx4Q3n74EftU=" ] || fail "SUPublicEDKey is wrong"
[ "$(read_plist SURequireSignedFeed)" = "true" ] || fail "SURequireSignedFeed is not true"
[ "$(read_plist SUVerifyUpdateBeforeExtraction)" = "true" ] || fail "SUVerifyUpdateBeforeExtraction is not true"
[ -n "$(read_plist NSMicrophoneUsageDescription)" ] || fail "NSMicrophoneUsageDescription is empty"

for stray in "$app/Contents/PlugIns" "$app"/Contents/Frameworks/XCTest*.framework "$app"/Contents/Frameworks/libXCTest*; do
	[ -e "$stray" ] && fail "test code in the product: $stray"
done
codesign --verify --deep --strict "$app" || fail "$app does not verify"
codesign -dv "$app" 2>&1 | grep -q 'runtime' || fail "$app has no hardened runtime"
codesign -d --entitlements - "$app" 2>&1 | grep -q 'com.apple.security.device.audio-input' || fail "$app lacks the audio-input entitlement"

hdiutil verify -quiet "$dmg" || fail "$dmg does not verify"
xmllint --noout "$appcast" || fail "$appcast is not XML"
grep -Fq "<sparkle:version>$build_number</sparkle:version>" "$appcast" || fail "the appcast names another build than $build_number"
grep -Fq "length=\"$(stat -f%z "$dmg")\"" "$appcast" || fail "the appcast's length is not the DMG's"
grep -Fq "<sparkle:hardwareRequirements>arm64</sparkle:hardwareRequirements>" "$appcast" || fail "the appcast does not require arm64"

if [ "$identity" != "-" ]; then
	codesign --verify --strict "$dmg" || fail "$dmg is not signed"
fi
if [ "$notarized" = "yes" ]; then
	[ "$identity" != "-" ] || fail "notarized without an identity"
	spctl --assess --type open --context context:primary-signature -v "$dmg" || fail "Gatekeeper refuses $dmg"
	xcrun stapler validate "$app" >/dev/null || fail "$app has no stapled ticket"
	xcrun stapler validate "$dmg" >/dev/null || fail "$dmg has no stapled ticket"
fi
if [ "$feed_signed" = "yes" ]; then
	grep -q 'sparkle-signatures:' "$appcast" || fail "the feed is claimed signed but has no signature block"
	grep -q 'sparkle:edSignature=' "$appcast" || fail "the feed is claimed signed but its enclosure has no signature"
fi
echo "verified $app, $dmg and $appcast"
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `chmod +x desktop/scripts/verify-release.sh && desktop/scripts/verify-release-test.sh`
Expected: `verify-release.sh is right`.

- [ ] **Step 5: Write the failing orchestrator test**

`desktop/scripts/release-test.sh`:

```sh
#!/bin/sh
# Checks release.sh against a bundle of its own. With no secret at all: the
# DMG under its -unnotarized name, its checksum, the unsigned appcast under
# its own name, the cask, the summary and the state file are written; every
# secret-bearing step is skipped by name; nothing was notarized, signed for
# Sparkle or pushed; the state says so (the dry run, the same path CI takes
# without secrets). Then, with a stand-in identity and shims: the
# certificate alone, every secret, and a rejected notarization.
set -eu

here="$(cd "$(dirname "$0")" && pwd)"
script="$here/release.sh"
root=$(mktemp -d)
trap 'rm -rf "$root"' EXIT
export TAP_SIGNING_KEYCHAIN_FILE="$root/test.keychain-db"
export TAP_ENTITLEMENTS="$root/Test.entitlements"
cat > "$TAP_ENTITLEMENTS" <<'PLIST'
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
<key>com.apple.security.device.audio-input</key><true/>
</dict></plist>
PLIST

app="$root/Tap.app"
mkdir -p "$app/Contents/MacOS" "$app/Contents/Resources"
printf 'int main(void) { return 0; }\n' | cc -arch arm64 -x c -o "$app/Contents/MacOS/Tap" -
printf '#include <stdio.h>\nint main(void) { puts("tap version 0.0.0-test"); return 0; }\n' | cc -arch arm64 -x c -o "$app/Contents/Resources/tap" -
cat > "$app/Contents/Info.plist" <<'PLIST'
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
<key>CFBundleExecutable</key><string>Tap</string>
<key>CFBundleIdentifier</key><string>io.geocod.tap.releasetest</string>
<key>CFBundlePackageType</key><string>APPL</string>
<key>CFBundleShortVersionString</key><string>0.0.0-test</string>
<key>CFBundleVersion</key><string>1</string>
<key>LSMinimumSystemVersion</key><string>14.0</string>
<key>NSMicrophoneUsageDescription</key><string>Tap records your voice with the screen when you record a talk.</string>
<key>SUFeedURL</key><string>https://github.com/MiniCodeMonkey/tap/releases/latest/download/appcast.xml</string>
<key>SUPublicEDKey</key><string>Pc0PbtYL9fkyK3XnqULlpKLlH94TE4OWx4Q3n74EftU=</string>
<key>SURequireSignedFeed</key><true/>
<key>SUVerifyUpdateBeforeExtraction</key><true/>
</dict></plist>
PLIST

env -u APPLE_DEVELOPER_ID_APPLICATION_P12 -u APPLE_NOTARY_KEY -u SPARKLE_PRIVATE_KEY -u HOMEBREW_TAP_TOKEN \
	"$script" 0.0.0-test "$app" "$root/out" > "$root/log" 2>&1 || { echo "the dry run should succeed"; cat "$root/log"; exit 1; }

for file in Tap-0.0.0-test-unnotarized.dmg Tap-0.0.0-test-unnotarized.dmg.sha256 appcast-unsigned.xml Casks/tap-desktop.rb release-summary.md release-state.env; do
	[ -f "$root/out/$file" ] || { echo "missing $file"; cat "$root/log"; exit 1; }
done
for absent in Tap-0.0.0-test.dmg appcast.xml Tap-0.0.0-test.md; do
	[ ! -e "$root/out/$absent" ] || { echo "$absent must not exist in a dry run"; exit 1; }
done
grep -Fq "Tap-0.0.0-test-unnotarized.dmg" "$root/out/Tap-0.0.0-test-unnotarized.dmg.sha256" || { echo "the checksum names the DMG"; exit 1; }
[ "$(cut -d ' ' -f 1 "$root/out/Tap-0.0.0-test-unnotarized.dmg.sha256")" = "$(shasum -a 256 "$root/out/Tap-0.0.0-test-unnotarized.dmg" | cut -d ' ' -f 1)" ] || { echo "the checksum is wrong"; exit 1; }
grep -Fq 'unsigned: SPARKLE_PRIVATE_KEY was not set' "$root/out/appcast-unsigned.xml" || { echo "the appcast should say it is unsigned"; exit 1; }
grep -Fq 'version "0.0.0-test"' "$root/out/Casks/tap-desktop.rb" || { echo "the cask was not rendered"; exit 1; }
codesign -dv "$app" 2>&1 | grep -q 'Signature=adhoc' || { echo "the app should be signed ad-hoc"; exit 1; }

for line in \
	'skipped: Developer ID signing (APPLE_DEVELOPER_ID_APPLICATION_P12 is not set)' \
	'skipped: notarization of Tap-0.0.0-test.zip (no Developer ID identity)' \
	'skipped: DMG signature (no Developer ID identity)' \
	'skipped: notarization of Tap-0.0.0-test-unnotarized.dmg (no Developer ID identity)' \
	'skipped: release notes (CHANGELOG.md has no section for 0.0.0-test)' \
	'skipped: Sparkle signature of Tap-0.0.0-test-unnotarized.dmg (SPARKLE_PRIVATE_KEY is not set)' \
	'skipped: Gatekeeper assessment (not notarized)'; do
	grep -Fq "$line" "$root/out/release-summary.md" || { echo "the summary lacks: $line"; cat "$root/out/release-summary.md"; exit 1; }
done
[ "$(grep -c '^- skipped:' "$root/out/release-summary.md")" = "7" ] || { echo "seven skipped lines, got $(grep -c '^- skipped:' "$root/out/release-summary.md")"; exit 1; }
for line in \
	'done: signed Tap.app ad-hoc' \
	'done: wrote Tap-0.0.0-test-unnotarized.dmg' \
	'done: wrote appcast-unsigned.xml (unsigned; never uploaded)' \
	'done: rendered Casks/tap-desktop.rb (not pushed by this script)' \
	'done: verified the app, the DMG and the appcast'; do
	grep -Fq "$line" "$root/out/release-summary.md" || { echo "the summary lacks: $line"; cat "$root/out/release-summary.md"; exit 1; }
done
for pair in 'version=0.0.0-test' 'notarized=no' 'feed_signed=no' 'dmg=Tap-0.0.0-test-unnotarized.dmg' 'checksum=Tap-0.0.0-test-unnotarized.dmg.sha256' 'notes=' 'appcast=appcast-unsigned.xml' 'cask=Casks/tap-desktop.rb'; do
	grep -Fxq "$pair" "$root/out/release-state.env" || { echo "the state lacks: $pair"; cat "$root/out/release-state.env"; exit 1; }
done
[ ! -f "$TAP_SIGNING_KEYCHAIN_FILE" ] || { echo "the keychain should be gone"; exit 1; }

# A second run replaces the output folder's files rather than failing on them.
env -u APPLE_DEVELOPER_ID_APPLICATION_P12 -u APPLE_NOTARY_KEY -u SPARKLE_PRIVATE_KEY -u HOMEBREW_TAP_TOKEN \
	"$script" 0.0.0-test "$app" "$root/out" >/dev/null 2>&1 || { echo "a second run should succeed"; exit 1; }

# The version must match the app's own.
if "$script" 9.9.9 "$app" "$root/out2" >/dev/null 2>&1; then echo "a version the app does not carry should fail"; exit 1; fi

# With a stand-in identity: codesign turns the named identity into ad-hoc
# and drops the timestamp, xcrun answers notarytool and stapler, spctl
# accepts, and a stand-in sign_update signs. Three runs: the certificate
# alone (the state right after enrolment), every secret, and every secret
# with the notary service rejecting the submission.
mkdir -p "$root/shims" "$root/tools-2.10.0/bin"
cat > "$root/shims/codesign" <<'SHIM'
#!/bin/sh
next_is_identity=no
for argument; do
	if [ "$next_is_identity" = yes ]; then set -- "$@" -; next_is_identity=no; continue; fi
	case "$argument" in
		--sign) set -- "$@" --sign; next_is_identity=yes ;;
		--timestamp) set -- "$@" --timestamp=none ;;
		*) set -- "$@" "$argument" ;;
	esac
	shift
done
exec /usr/bin/codesign "$@"
SHIM
cat > "$root/shims/xcrun" <<SHIM
#!/bin/sh
case "\$1 \$2" in
	"notarytool submit") printf '{"status":"%s","id":"sub-1"}\n' "\$(cat "$root/notary-status")" ;;
	"notarytool log") echo "log for sub-1" ;;
	"stapler staple"|"stapler validate") echo "\$3: stapled (stand-in)" ;;
	*) exec /usr/bin/xcrun "\$@" ;;
esac
SHIM
printf '#!/bin/sh\necho "accepted (stand-in)"\n' > "$root/shims/spctl"
cat > "$root/tools-2.10.0/bin/sign_update" <<'FAKE'
#!/bin/sh
cat > /dev/null
for file; do :; done
case "$*" in
	*--verify*) exit 0 ;;
	*" -p "*) echo "QVJDSElWRQ==" ;;
	*.xml) printf '<!-- sparkle-signatures:\nedSignature: RkVFRA==\nlength: 1\n-->' >> "$file" ;;
	*) printf 'sparkle:edSignature="Tk9URVM=" sparkle:length="42"\n' ;;
esac
FAKE
chmod +x "$root/shims"/* "$root/tools-2.10.0/bin/sign_update"
export TAP_RELEASE_IDENTITY="Developer ID Application: Test Person (TEAM123456)"
export SPARKLE_TOOLS="$root/tools-2.10.0"
sources() { bash -eo pipefail -c "source '$1/release-state.env' && printf '%s %s %s\n' \"\$notarized\" \"\$feed_signed\" \"\$dmg\""; }

# A: the certificate alone.
PATH="$root/shims:$PATH" env -u APPLE_NOTARY_KEY -u APPLE_NOTARY_KEY_ID -u APPLE_NOTARY_ISSUER_ID -u SPARKLE_PRIVATE_KEY \
	"$script" 0.0.0-test "$app" "$root/a" > "$root/log" 2>&1 || { echo "the certificate-only run should succeed"; cat "$root/log"; exit 1; }
grep -Fq 'done: using the identity Developer ID Application: Test Person (TEAM123456)' "$root/a/release-summary.md" || { echo "A: the identity line"; exit 1; }
grep -Fq 'skipped: notarization of Tap-0.0.0-test.zip (APPLE_NOTARY_KEY is not set)' "$root/a/release-summary.md" || { echo "A: the app's notarization skip line names the secret"; cat "$root/a/release-summary.md"; exit 1; }
grep -Fq 'skipped: notarization of Tap-0.0.0-test-unnotarized.dmg (APPLE_NOTARY_KEY is not set)' "$root/a/release-summary.md" || { echo "A: the DMG's notarization skip line names the secret"; exit 1; }
grep -Fq 'done: signed Tap-0.0.0-test-unnotarized.dmg' "$root/a/release-summary.md" || { echo "A: the DMG is signed"; exit 1; }
if grep -q '^- $' "$root/a/release-summary.md"; then echo "A: an empty summary line"; exit 1; fi
[ "$(sources "$root/a")" = "no no Tap-0.0.0-test-unnotarized.dmg" ] || { echo "A: the state does not source: $(sources "$root/a" 2>&1)"; exit 1; }
if grep -q 'identity=' "$root/a/release-state.env"; then echo "the identity stays out of the state file"; exit 1; fi

# B: every secret, the notary service accepting.
echo Accepted > "$root/notary-status"
PATH="$root/shims:$PATH" APPLE_NOTARY_KEY=key APPLE_NOTARY_KEY_ID=id APPLE_NOTARY_ISSUER_ID=issuer SPARKLE_PRIVATE_KEY=bm90LWEta2V5 \
	"$script" 0.0.0-test "$app" "$root/b" > "$root/log" 2>&1 || { echo "the all-secrets run should succeed"; cat "$root/log"; exit 1; }
for file in Tap-0.0.0-test.dmg Tap-0.0.0-test.dmg.sha256 appcast.xml Casks/tap-desktop.rb; do
	[ -f "$root/b/$file" ] || { echo "B: missing $file"; cat "$root/log"; exit 1; }
done
[ ! -e "$root/b/appcast-unsigned.xml" ] || { echo "B: no unsigned appcast"; exit 1; }
grep -Fq 'notarized Tap-0.0.0-test.zip (submission sub-1) and stapled Tap.app' "$root/b/release-summary.md" || { echo "B: the app's notarization line"; exit 1; }
grep -Fq 'notarized Tap-0.0.0-test.dmg (submission sub-1) and stapled Tap-0.0.0-test.dmg' "$root/b/release-summary.md" || { echo "B: the DMG's notarization line"; exit 1; }
grep -Fq 'done: wrote appcast.xml (signed)' "$root/b/release-summary.md" || { echo "B: the signed appcast line"; exit 1; }
grep -q 'sparkle-signatures:' "$root/b/appcast.xml" || { echo "B: the feed is signed"; exit 1; }
[ "$(sources "$root/b")" = "yes yes Tap-0.0.0-test.dmg" ] || { echo "B: the state does not source: $(sources "$root/b" 2>&1)"; exit 1; }

# C: every secret, the notary service rejecting: the release stops, and
# nothing says notarized.
echo Invalid > "$root/notary-status"
if PATH="$root/shims:$PATH" APPLE_NOTARY_KEY=key APPLE_NOTARY_KEY_ID=id APPLE_NOTARY_ISSUER_ID=issuer SPARKLE_PRIVATE_KEY=bm90LWEta2V5 \
	"$script" 0.0.0-test "$app" "$root/c" > "$root/log" 2>&1; then echo "a rejected notarization should fail the release"; exit 1; fi
grep -q "was not accepted (status: Invalid" "$root/log" || { echo "C: the rejection is reported: $(cat "$root/log")"; exit 1; }
grep -q "nothing is published" "$root/log" || { echo "C: release.sh says it stopped"; exit 1; }
[ ! -e "$root/c/release-state.env" ] || { echo "C: no state file after a failure"; exit 1; }
[ ! -e "$root/c/appcast.xml" ] || { echo "C: no appcast after a failure"; exit 1; }
if grep -q 'notarized' "$root/c/release-summary.md" 2>/dev/null; then echo "C: the summary must not claim notarization"; exit 1; fi
unset TAP_RELEASE_IDENTITY SPARKLE_TOOLS

echo "release.sh is right"
```

- [ ] **Step 6: Write the orchestrator and the Makefile targets**

`desktop/scripts/release.sh`:

```sh
#!/bin/sh
# Turns a built Release app into a release: signs it, notarizes the app and
# the DMG, writes the DMG with its checksum, the release notes, the
# Sparkle appcast and the cask, verifies everything, and records every
# step in release-summary.md as done or skipped and the outcome in
# release-state.env, which the release job reads to decide what may be
# published. Every step that needs a secret skips itself, by name, when
# the secret is absent, so this runs with none (a dry run) and with all
# of them (the release job) along the same path. A DMG that was not
# notarized is named -unnotarized; an appcast that is not signed is
# appcast-unsigned.xml; neither is ever offered to a person. Nothing here
# prints a secret; the scripts it calls own that rule.
set -eu

version="${1:-}"; app="${2:-}"; out="${3:-}"
[ -n "$version" ] && [ -d "$app" ] && [ -n "$out" ] || { echo "release.sh: usage: release.sh <version> <Tap.app> <output-dir>" >&2; exit 1; }
here="$(cd "$(dirname "$0")" && pwd)"
entitlements="${TAP_ENTITLEMENTS:-$here/../Tap/Tap.entitlements}"
changelog="$here/../../CHANGELOG.md"
prepare_changelog="$here/../../scripts/prepare-changelog.sh"
built=$(/usr/libexec/PlistBuddy -c 'Print :CFBundleShortVersionString' "$app/Contents/Info.plist")
[ "$built" = "$version" ] || { echo "release.sh: the app is $built, not $version; build it with VERSION=$version" >&2; exit 1; }

mkdir -p "$out"
summary="$out/release-summary.md"
state="$out/release-state.env"
: > "$summary"
note() { echo "$1"; echo "- $1" >> "$summary"; }

# The keychain goes whatever happens from here on, even before the import
# has returned. TAP_RELEASE_IDENTITY names an identity already in a
# keychain (a person's own, or a test's stand-in) and skips the import.
trap '"$here/signing-identity.sh" remove; rm -f "$out/identity.log" "$out/sparkle.log"' EXIT
if [ -n "${TAP_RELEASE_IDENTITY:-}" ]; then
	identity="$TAP_RELEASE_IDENTITY"
	note "done: using the identity $identity"
else
	identity=$("$here/signing-identity.sh" import 2>"$out/identity.log") || { cat "$out/identity.log" >&2; exit 1; }
	if [ "$identity" = "-" ]; then
		note "$(cat "$out/identity.log")"
	else
		note "done: found the identity $identity"
	fi
fi

"$here/sign-app.sh" "$app" "$identity" "$entitlements" >/dev/null
if [ "$identity" = "-" ]; then note "done: signed $(basename "$app") ad-hoc"; else note "done: signed $(basename "$app") with $identity"; fi

# Notarization needs an identity and the three notary secrets; a DMG
# without it carries the fact in its name. A submission the notary service
# rejects stops the release here: nothing after this point may pretend.
notarized=no
missing_notary_secret=""
for secret in APPLE_NOTARY_KEY APPLE_NOTARY_KEY_ID APPLE_NOTARY_ISSUER_ID; do
	eval "value=\${$secret:-}"
	if [ -z "$value" ] && [ -z "$missing_notary_secret" ]; then missing_notary_secret="$secret"; fi
done
will_notarize=no
if [ "$identity" != "-" ] && [ -z "$missing_notary_secret" ]; then will_notarize=yes; fi
if [ "$will_notarize" = yes ]; then dmg_name="Tap-$version.dmg"; else dmg_name="Tap-$version-unnotarized.dmg"; fi
dmg="$out/$dmg_name"
zip="$out/Tap-$version.zip"
if [ "$identity" = "-" ]; then
	note "skipped: notarization of $(basename "$zip") (no Developer ID identity)"
elif [ "$will_notarize" = no ]; then
	note "skipped: notarization of $(basename "$zip") ($missing_notary_secret is not set)"
else
	rm -f "$zip"
	ditto -c -k --keepParent "$app" "$zip"
	result=$("$here/notarize.sh" "$zip" "$app") || { echo "release.sh: the app's notarization failed; nothing is published" >&2; exit 1; }
	note "$result"
	rm -f "$zip"
fi

rm -f "$out"/Tap-"$version"*.dmg "$out"/Tap-"$version"*.dmg.sha256
"$here/make-dmg.sh" "$app" "$dmg" Tap >/dev/null
note "done: wrote $dmg_name"
if [ "$identity" = "-" ]; then
	note "skipped: DMG signature (no Developer ID identity)"
	note "skipped: notarization of $dmg_name (no Developer ID identity)"
else
	codesign --force --sign "$identity" --timestamp "$dmg"
	note "done: signed $dmg_name"
	if [ "$will_notarize" = yes ]; then
		result=$("$here/notarize.sh" "$dmg" "$dmg") || { echo "release.sh: the DMG's notarization failed; nothing is published" >&2; exit 1; }
		note "$result"
		notarized=yes
	else
		note "skipped: notarization of $dmg_name ($missing_notary_secret is not set)"
	fi
fi
( cd "$out" && shasum -a 256 "$dmg_name" > "$dmg_name.sha256" )
note "done: wrote $dmg_name.sha256"

# The release notes: the version's own section of the changelog, which
# the CLI job wrote before tagging. A dry-run version has none.
notes=""
notes_url=""
notes_signature=""
notes_length=""
rm -f "$out/Tap-$version.md"
if [ -f "$changelog" ] && [ -x "$prepare_changelog" ] && grep -q "^## \[$(printf '%s' "$version" | sed 's/[.]/\\./g')\]" "$changelog"; then
	cp "$changelog" "$out/CHANGELOG.copy.md"
	"$prepare_changelog" "$version" "$out/CHANGELOG.copy.md" "$out/Tap-$version.md" >/dev/null
	rm -f "$out/CHANGELOG.copy.md"
	notes="Tap-$version.md"
	notes_url="https://github.com/MiniCodeMonkey/tap/releases/download/v$version/Tap-$version.md"
	note "done: wrote $notes"
	signed_notes=$("$here/sparkle-sign.sh" notes "$out/$notes" 2>"$out/sparkle.log") || { cat "$out/sparkle.log" >&2; exit 1; }
	if [ -n "$signed_notes" ]; then
		notes_signature="${signed_notes%% *}"
		notes_length="${signed_notes##* }"
		note "done: signed $notes for Sparkle"
	else
		note "$(grep '^skipped:' "$out/sparkle.log")"
	fi
else
	note "skipped: release notes (CHANGELOG.md has no section for $version)"
fi

# The appcast: signed only when the DMG, the notes (if any) and the feed
# itself carry signatures; otherwise named so nothing uploads it.
signature=$("$here/sparkle-sign.sh" archive "$dmg" 2>"$out/sparkle.log") || { cat "$out/sparkle.log" >&2; exit 1; }
if [ -n "$signature" ]; then
	note "done: signed $dmg_name for Sparkle"
else
	note "$(grep '^skipped:' "$out/sparkle.log")"
fi
download_url="https://github.com/MiniCodeMonkey/tap/releases/download/v$version/$dmg_name"
release_url="https://github.com/MiniCodeMonkey/tap/releases/tag/v$version"
rm -f "$out/appcast.xml" "$out/appcast-unsigned.xml"
feed_signed=no
if [ -n "$signature" ] && [ "$notarized" = yes ] && { [ -z "$notes" ] || [ -n "$notes_signature" ]; }; then
	appcast="appcast.xml"
	"$here/write-appcast.sh" "$app" "$dmg" "$download_url" "$release_url" "$out/$appcast" "$signature" "$notes_url" "$notes_signature" "$notes_length" >/dev/null
	"$here/sparkle-sign.sh" feed "$out/$appcast" 2>"$out/sparkle.log" || { cat "$out/sparkle.log" >&2; exit 1; }
	feed_signed=yes
	note "done: wrote $appcast (signed)"
else
	appcast="appcast-unsigned.xml"
	"$here/write-appcast.sh" "$app" "$dmg" "$download_url" "$release_url" "$out/$appcast" "$signature" "$notes_url" "$notes_signature" "$notes_length" >/dev/null
	note "done: wrote $appcast (unsigned; never uploaded)"
fi

"$here/render-cask.sh" "$version" "$dmg" "$out/Casks/tap-desktop.rb" >/dev/null
note "done: rendered Casks/tap-desktop.rb (not pushed by this script)"

"$here/verify-release.sh" "$app" "$dmg" "$out/$appcast" "$identity" "$version" "$notarized" "$feed_signed" >/dev/null
if [ "$notarized" = yes ]; then note "done: Gatekeeper accepts $dmg_name"; else note "skipped: Gatekeeper assessment (not notarized)"; fi
note "done: verified the app, the DMG and the appcast"

# Plain words only, so the release job can source this under bash -e: the
# identity's name (with its spaces and parentheses) stays out of it.
cat > "$state" <<STATE
version=$version
notarized=$notarized
feed_signed=$feed_signed
dmg=$dmg_name
checksum=$dmg_name.sha256
notes=$notes
appcast=$appcast
cask=Casks/tap-desktop.rb
STATE

echo
echo "release $version in $out:"
cat "$summary"
```

The "identity but no notary key" branch writes the skip line itself, naming the first missing secret as `notarize.sh` would, since `notarize.sh` only takes a file. A `notarize.sh` failure is read from its exit status into `result=... || exit 1`, never inside a `note "$(...)"` argument, where `set -e` would not see it. The `identity` variable holds a certificate's common name, never a secret, and stays out of the state file only because its spaces and parentheses would break `source`.

In `desktop/Makefile`, add `RELEASE_DIR = build/release` after `RELEASE_BINARY`, `release-tests release` to `.PHONY`, and after `check-release-app-hooks`:

```make
# Every release script's own test; none needs a build, a secret, a window
# or the network.
release-tests:
	@for test in scripts/build-number-test.sh scripts/sign-app-test.sh scripts/make-dmg-test.sh scripts/write-appcast-test.sh scripts/sparkle-sign-test.sh scripts/render-cask-test.sh scripts/publish-cask-test.sh scripts/signing-identity-test.sh scripts/notarize-test.sh scripts/verify-release-test.sh scripts/release-test.sh scripts/mark-latest-test.sh; do \
		echo "== $$test"; sh "$$test" || exit 1; \
	done

# The release: the Release build, the hook check, then release.sh, which
# signs, notarizes, writes the DMG, the notes, the appcast and the cask
# into build/release, skipping what has no secret. The same target is the
# local dry run (VERSION=0.0.0-dev, no secrets) and the release job's step.
release: check-release-app-hooks
	./scripts/release.sh $(VERSION) $(RELEASE_APP) $(RELEASE_DIR)
```

`mark-latest-test.sh` is Task 8's; until it exists the loop stops there, so Task 8 lands before `release-tests` is green on CI (both are in batch C).

- [ ] **Step 7: Run the tests and the local dry run**

Run: `chmod +x desktop/scripts/release.sh desktop/scripts/release-test.sh && desktop/scripts/release-test.sh && for test in desktop/scripts/*-test.sh; do sh "$test" || exit 1; done`
Expected: `release.sh is right` (four runs of `release.sh` inside: no secrets, the certificate alone, every secret, a rejected notarization), and every other test script's `... is right` line. Then the real dry run:

Run: `make -C desktop release VERSION=0.0.0-dev`
Expected: the build, `no test-only hook in the Release build`, then `release 0.0.0-dev in build/release:` with the summary: the identity skipped, `signed Tap.app ad-hoc`, the app's notarization skipped, `wrote Tap-0.0.0-dev-unnotarized.dmg`, DMG signature and notarization skipped, the checksum, the notes skipped (no changelog section for `0.0.0-dev`), the Sparkle signature skipped, `wrote appcast-unsigned.xml (unsigned; never uploaded)`, `rendered Casks/tap-desktop.rb (not pushed by this script)`, Gatekeeper skipped, verified. `ls desktop/build/release` shows the DMG, its checksum, `appcast-unsigned.xml`, `Casks/`, `release-summary.md` and `release-state.env` with `notarized=no`. Nothing was launched or mounted with a window.

- [ ] **Step 8: Commit**

```bash
git add desktop/scripts/verify-release.sh desktop/scripts/verify-release-test.sh desktop/scripts/release.sh desktop/scripts/release-test.sh desktop/Makefile
git commit -m "build(desktop): release.sh runs the release end to end, names what is not notarized, and never writes an unsigned appcast.xml"
```

Mutations, applied locally and reverted: in `release.sh`, name the DMG `Tap-$version.dmg` regardless (`release-test.sh` fails on the missing `-unnotarized` file); write `appcast.xml` when the signature is empty (fails on "appcast.xml must not exist"); wrap the DMG's `notarize.sh` call back into `note "$(...)"` (fails on "a rejected notarization should fail the release"); set `notarized=yes` before the DMG's submission (the same); write `identity=$identity` into the state file (fails on "the state does not source"); call `notarize.sh "$app" "$app"` in the no-notary-key branch (fails on "the app's notarization skip line names the secret"); drop the `built = version` guard (fails on "a version the app does not carry"); set the trap after the import (`release-test.sh` still passes: into the survivor notes, since no test can make the import fail after creating the keychain without a certificate); in `verify-release.sh`, gate `spctl` on the identity instead of `notarized` (`verify-release-test.sh` fails on "signed but not notarized should verify"); drop the `lipo` check (fails on "is not arm64 alone"); drop the `tap --version` check (fails on "tap version 0.0.0-other"); drop the stray check (fails on "test code").

---

### Task 8: The release job, the dry-run job, and marking a release latest

**Files:**
- Create: `desktop/scripts/mark-latest.sh`, `desktop/scripts/mark-latest-test.sh`
- Modify: `.github/workflows/release.yml` (`make_latest: false` on the CLI release; the `desktop` job)
- Modify: `.github/workflows/ci.yml` (the `release-dry-run` job)

**Interfaces:**
- Consumes: `make -C desktop check-release-app-hooks`, `desktop/scripts/release.sh`, `release-state.env`, `desktop/scripts/publish-cask.sh`, `desktop/scripts/signing-identity.sh remove`, `make -C desktop release-tests` (Tasks 1 to 7b); the `release` job's tag `v${version}` and its GitHub release; the secrets by name; `vars.HOMEBREW_TAP_REPO`; `gh release upload`, `gh api repos/{owner}/{repo}/releases/latest` (HTTP 404 when no release exists), `gh release edit --latest`, `actions/upload-artifact`.
- Produces: `mark-latest.sh <version> <feed-uploaded yes|no>` (`GH` names the `gh` binary, for the test's stand-in; exit 1 on any `gh` failure but 404); on a release: for a notarized DMG, the DMG with its checksum, then the notes and the signed feed when the state allows, the cask pushed for a notarized final, the release marked `latest` when its feed is up or no earlier feed exists (otherwise the summary names the command for later); for a DMG that was not notarized, nothing on the release and the workflow artifact `desktop-release-unnotarized` (seven days); the summary in the job's step summary whether the job passed or failed; on every pull request: a `desktop-release-dry-run` artifact with the DMG, the unsigned appcast, the cask, the summary and the state.

- [ ] **Step 1: Write the failing test for marking latest**

`desktop/scripts/mark-latest-test.sh`:

```sh
#!/bin/sh
# Checks mark-latest.sh against a stand-in gh that records its calls and
# answers the latest-release API from a file the test writes: a pre-release
# is never marked; a final with its feed uploaded is; a final without one is
# marked only when the current latest release carries no appcast.xml (so
# there is no working feed to take away) or there is no release at all; any
# other gh failure marks nothing and fails the step.
set -eu

script="$(cd "$(dirname "$0")" && pwd)/mark-latest.sh"
root=$(mktemp -d)
trap 'rm -rf "$root"' EXIT
cat > "$root/gh" <<FAKE
#!/bin/sh
printf '%s\n' "\$*" >> "$root/calls"
case "\$*" in
	"api repos/{owner}/{repo}/releases/latest")
		# The file holds the JSON gh api would print, or a word for a failure.
		case "\$(cat "$root/latest.json")" in
			404) echo "gh: Not Found (HTTP 404)" >&2; exit 1 ;;
			ERROR) echo "gh: error connecting to api.github.com" >&2; exit 1 ;;
			*) cat "$root/latest.json" ;;
		esac
		;;
esac
FAKE
chmod +x "$root/gh"
export GH="$root/gh"
calls() { cat "$root/calls" 2>/dev/null || true; rm -f "$root/calls"; }

out=$("$script" 2.1.0-beta.1 yes)
[ "$out" = "skipped: marking v2.1.0-beta.1 latest (a pre-release)" ] || { echo "wrong pre-release line: $out"; exit 1; }
[ -z "$(calls)" ] || { echo "a pre-release should call gh for nothing"; exit 1; }

out=$("$script" 2.1.0 yes)
[ "$out" = "marked v2.1.0 latest (its feed is up)" ] || { echo "wrong marked line: $out"; exit 1; }
[ "$(calls)" = "release edit v2.1.0 --latest" ] || { echo "the feed-up case should edit the release alone"; exit 1; }

printf '{"tag_name":"v2.0.0","assets":[{"name":"tap-darwin-arm64"},{"name":"appcast.xml"}]}\n' > "$root/latest.json"
out=$("$script" 2.1.0 no)
[ "$out" = "skipped: marking v2.1.0 latest (v2.0.0 carries the working feed; this release has none). Once v2.1.0 has a signed feed, run: gh release edit v2.1.0 --latest" ] || { echo "wrong protected line: $out"; exit 1; }
if calls | grep -q 'release edit'; then echo "a protected feed must not be replaced"; exit 1; fi

printf '{"tag_name":"v2.0.0","assets":[{"name":"tap-darwin-arm64"}]}\n' > "$root/latest.json"
out=$("$script" 2.1.0 no)
[ "$out" = "marked v2.1.0 latest (no earlier release carries a feed)" ] || { echo "wrong no-feed line: $out"; exit 1; }
calls | grep -q '^release edit v2.1.0 --latest$' || { echo "no earlier feed: the release should be marked"; exit 1; }

# No release at all: gh api answers 404, and there is nothing to protect.
echo 404 > "$root/latest.json"
out=$("$script" 2.1.0 no)
[ "$out" = "marked v2.1.0 latest (no earlier release carries a feed)" ] || { echo "no releases at all: $out"; exit 1; }
calls | grep -q '^release edit v2.1.0 --latest$' || { echo "no releases at all: the release should be marked"; exit 1; }

# Any other gh failure (auth, network, a rate limit) fails closed: nothing is marked.
echo ERROR > "$root/latest.json"
if out=$("$script" 2.1.0 no 2>"$root/err"); then echo "a gh error must not mark anything: $out"; exit 1; fi
grep -q 'could not read the latest release' "$root/err" || { echo "the gh error should be named: $(cat "$root/err")"; exit 1; }
if calls | grep -q 'release edit'; then echo "a gh error must not lead to an edit"; exit 1; fi

if "$script" 2.1.0 >/dev/null 2>&1; then echo "the feed state is required"; exit 1; fi

echo "mark-latest.sh is right"
```

- [ ] **Step 2: Write the script**

`desktop/scripts/mark-latest.sh`:

```sh
#!/bin/sh
# Makes a final release GitHub's "latest", which is where SUFeedURL points,
# only once that would not break the feed: when this release's signed
# appcast is uploaded, or when no earlier release carries one (nothing to
# take away). The CLI job publishes with make_latest false, so the tag
# exists and the desktop job decides. A pre-release is never latest. When
# an earlier feed is protected, the line says which command marks this
# release later. GH names the gh binary; the test gives a stand-in.
set -eu

version="${1:-}"; feed_uploaded="${2:-}"
[ -n "$version" ] && [ -n "$feed_uploaded" ] || { echo "mark-latest.sh: usage: mark-latest.sh <version> <feed-uploaded yes|no>" >&2; exit 1; }
gh="${GH:-gh}"
tag="v$version"

case "$version" in
	*-*) echo "skipped: marking $tag latest (a pre-release)"; exit 0 ;;
esac

if [ "$feed_uploaded" = yes ]; then
	"$gh" release edit "$tag" --latest >/dev/null
	echo "marked $tag latest (its feed is up)"
	exit 0
fi

# Fail closed: only "no release exists" (HTTP 404) means there is nothing
# to protect; any other failure (auth, network, a rate limit) marks nothing.
if latest=$("$gh" api 'repos/{owner}/{repo}/releases/latest' 2>"${TMPDIR:-/tmp}/mark-latest.$$"); then
	rm -f "${TMPDIR:-/tmp}/mark-latest.$$"
elif grep -q 'HTTP 404' "${TMPDIR:-/tmp}/mark-latest.$$"; then
	rm -f "${TMPDIR:-/tmp}/mark-latest.$$"
	latest=""
else
	cat "${TMPDIR:-/tmp}/mark-latest.$$" >&2
	rm -f "${TMPDIR:-/tmp}/mark-latest.$$"
	echo "mark-latest.sh: could not read the latest release; $tag is left as it is" >&2
	exit 1
fi
latest_tag=$(printf '%s' "$latest" | sed -n 's/.*"tag_name":"\([^"]*\)".*/\1/p')
case "$latest" in
	*'"name":"appcast.xml"'*)
		echo "skipped: marking $tag latest ($latest_tag carries the working feed; this release has none). Once $tag has a signed feed, run: gh release edit $tag --latest"
		exit 0
		;;
esac
"$gh" release edit "$tag" --latest >/dev/null
echo "marked $tag latest (no earlier release carries a feed)"
```

Run: `chmod +x desktop/scripts/mark-latest.sh desktop/scripts/mark-latest-test.sh && desktop/scripts/mark-latest-test.sh && make -C desktop release-tests`
Expected: `mark-latest.sh is right`; every test in the Makefile's list passes.

- [ ] **Step 3: The `desktop` job in `release.yml`**

In the `release` job's `Validate version format` step, after the regex check and before the `echo "VERSION=..."` lines, add:

```yaml
          # The desktop build refuses versions the regex accepts (an unknown
          # label, a label with no number, beta.08), and a tag pushed before
          # that failure blocks a re-run; so the same check runs here first.
          desktop/scripts/build-number.sh "$VERSION" >/dev/null
```

In the `release` job's `Create GitHub Release` step, add `make_latest: false` under `with:` (after `prerelease:`), with the comment:

```yaml
          # The desktop job marks a final latest once its Sparkle feed is
          # uploaded (desktop/scripts/mark-latest.sh): "latest" is where the
          # app's feed URL points, and it must never point at a release with
          # no feed. The formula uses versioned URLs and is unaffected.
          make_latest: false
```

After the `release` job (same indentation as `release:`), add:

```yaml
  # Tap Desktop: built on a macOS runner from the tag the release job
  # pushed, after the GitHub release exists so the DMG, the notes and the
  # appcast can join it. The build runs with no secret in its environment;
  # only the release.sh step has them. Every step that needs a secret is
  # skipped, by name, when the secret is absent (see
  # desktop/scripts/release.sh), and what gets published follows the
  # state file: a DMG that was not notarized goes up under its
  # -unnotarized name alone, an appcast only when signed, the cask only for
  # a notarized final, and the release becomes "latest" only once its feed
  # is up.
  desktop:
    name: Desktop release
    needs: release
    runs-on: macos-15
    timeout-minutes: 90
    env:
      VERSION: ${{ github.event.inputs.version }}
    steps:
      - name: Checkout the tag
        uses: actions/checkout@v7
        with:
          ref: v${{ github.event.inputs.version }}
          persist-credentials: false

      - name: Select Xcode
        uses: maxim-lobanov/setup-xcode@v1
        with:
          xcode-version: '26'

      - name: Set up Go
        uses: actions/setup-go@v7
        with:
          go-version-file: go.mod

      - name: Set up Node.js
        uses: actions/setup-node@v7
        with:
          node-version: '20'
          cache: 'npm'
          cache-dependency-path: frontend/package-lock.json

      - name: Install xcodegen
        run: brew install xcodegen

      - name: Install frontend dependencies
        run: cd frontend && npm ci

      # build-tap.sh refuses a stale embedded/dist, so the frontend is built
      # before Xcode runs, as every desktop job does.
      - name: Build frontend
        run: make frontend

      - name: Check the release scripts
        run: make -C desktop release-tests

      # No secret is in this step's environment: xcodegen, SwiftPM, xcodebuild
      # and go build need none.
      - name: Build the Release app
        run: make -C desktop check-release-app-hooks VERSION="$VERSION"

      # The secrets reach release.sh through this step's environment alone.
      # An unset secret is an empty string here, which each script treats as
      # absent and reports as skipped.
      - name: Sign, notarize and package
        env:
          APPLE_DEVELOPER_ID_APPLICATION_P12: ${{ secrets.APPLE_DEVELOPER_ID_APPLICATION_P12 }}
          APPLE_DEVELOPER_ID_APPLICATION_PASSWORD: ${{ secrets.APPLE_DEVELOPER_ID_APPLICATION_PASSWORD }}
          APPLE_NOTARY_KEY: ${{ secrets.APPLE_NOTARY_KEY }}
          APPLE_NOTARY_KEY_ID: ${{ secrets.APPLE_NOTARY_KEY_ID }}
          APPLE_NOTARY_ISSUER_ID: ${{ secrets.APPLE_NOTARY_ISSUER_ID }}
          SPARKLE_PRIVATE_KEY: ${{ secrets.SPARKLE_PRIVATE_KEY }}
        run: desktop/scripts/release.sh "$VERSION" desktop/build/DerivedData/Build/Products/Release/Tap.app desktop/build/release

      - name: Remove the signing keychain
        if: always()
        run: desktop/scripts/signing-identity.sh remove

      - name: Summarize
        if: always()
        run: |
          echo "## Tap Desktop $VERSION" >> "$GITHUB_STEP_SUMMARY"
          if [ -f desktop/build/release/release-summary.md ]; then
            cat desktop/build/release/release-summary.md >> "$GITHUB_STEP_SUMMARY"
          else
            echo "release.sh wrote no summary; the build or an earlier step failed." >> "$GITHUB_STEP_SUMMARY"
          fi

      # What the state allows, in an order where a feed never names a file
      # that is not up yet: the DMG and its checksum, then the notes, then
      # the feed. A DMG that was not notarized never reaches the release; the
      # next step keeps it as a workflow artifact.
      - name: Upload to the release
        shell: bash
        env:
          GH_TOKEN: ${{ secrets.GITHUB_TOKEN }}
        run: |
          source desktop/build/release/release-state.env
          echo "notarized=$notarized" >> "$GITHUB_ENV"
          feed_uploaded=no
          if [ "$notarized" != yes ]; then
            echo "- the DMG is not notarized: nothing was added to the release; $dmg is the workflow artifact desktop-release-unnotarized (seven days)" | tee -a "$GITHUB_STEP_SUMMARY"
          else
            cd desktop/build/release
            gh release upload "v$VERSION" "$dmg" "$checksum" --clobber
            echo "uploaded $dmg and $checksum" | tee -a "$GITHUB_STEP_SUMMARY"
            if [ -n "$notes" ] && [ "$feed_signed" = yes ]; then
              gh release upload "v$VERSION" "$notes" --clobber
              echo "uploaded $notes" | tee -a "$GITHUB_STEP_SUMMARY"
            fi
            if [ "$feed_signed" = yes ]; then
              gh release upload "v$VERSION" appcast.xml --clobber
              feed_uploaded=yes
              echo "uploaded appcast.xml (signed)" | tee -a "$GITHUB_STEP_SUMMARY"
            else
              echo "- skipped: appcast upload (the feed is not signed)" | tee -a "$GITHUB_STEP_SUMMARY"
            fi
          fi
          echo "feed_uploaded=$feed_uploaded" >> "$GITHUB_ENV"

      - name: Keep the un-notarized DMG as an artifact
        if: env.notarized != 'yes'
        uses: actions/upload-artifact@v7
        with:
          name: desktop-release-unnotarized
          path: |
            desktop/build/release/*-unnotarized.dmg
            desktop/build/release/*-unnotarized.dmg.sha256
            desktop/build/release/release-summary.md
            desktop/build/release/release-state.env
          retention-days: 7

      - name: Mark the release latest
        shell: bash
        env:
          GH_TOKEN: ${{ secrets.GITHUB_TOKEN }}
        run: desktop/scripts/mark-latest.sh "$VERSION" "$feed_uploaded" | tee -a "$GITHUB_STEP_SUMMARY"

      # A notarized final only, as the formula step above and the state say.
      - name: Update the Homebrew cask
        shell: bash
        env:
          HOMEBREW_TAP_TOKEN: ${{ secrets.HOMEBREW_TAP_TOKEN }}
          HOMEBREW_TAP_REPO: ${{ vars.HOMEBREW_TAP_REPO || 'MiniCodeMonkey/homebrew-tap' }}
        run: |
          source desktop/build/release/release-state.env
          TAP_RELEASE_NOTARIZED="$notarized" desktop/scripts/publish-cask.sh "$VERSION" "desktop/build/release/$cask" > cask-result.txt
          cat cask-result.txt | tee -a "$GITHUB_STEP_SUMMARY"
```

`shell: bash` on GitHub Actions is `bash --noprofile --norc -eo pipefail {0}`, so a failed `publish-cask.sh` or `mark-latest.sh` fails its step even through `tee`, and `source` of the state file runs under `-e` (its values are plain words; the identity is not in it). `permissions: contents: write` at the top of the file already covers `gh release upload` and `gh release edit`. `needs: release` means the tag and the release exist before the checkout. `${{ github.event.inputs.version }}` appears in `with:` values only; every `run:` reads `$VERSION`.

- [ ] **Step 4: The `release-dry-run` job in `ci.yml`**

After `bench-desktop`:

```yaml
  # The release pipeline with no secrets, on every pull request: the arm64
  # Release build with the hardened runtime and the entitlements, the hook
  # check, ad-hoc signing, the -unnotarized DMG, the unsigned appcast under
  # its own name and the rendered cask, then brew style on the cask.
  # release.sh skips notarization, the Sparkle signatures and everything
  # that depends on them by name; the summary lists them. The DMG is kept
  # for a week so a reviewer can inspect it. This is how the pipeline is
  # proven before the Developer ID certificate, the notary key and the
  # Sparkle key exist. It references no secret, so a fork's pull request
  # reaches none.
  release-dry-run:
    name: Desktop Release Dry Run
    runs-on: macos-15
    timeout-minutes: 60
    steps:
      - name: Checkout
        uses: actions/checkout@v7
        with:
          persist-credentials: false

      - name: Select Xcode
        uses: maxim-lobanov/setup-xcode@v1
        with:
          xcode-version: '26'

      - name: Set up Go
        uses: actions/setup-go@v7
        with:
          go-version-file: go.mod

      - name: Set up Node.js
        uses: actions/setup-node@v7
        with:
          node-version: '20'
          cache: 'npm'
          cache-dependency-path: frontend/package-lock.json

      - name: Install xcodegen
        run: brew install xcodegen

      - name: Install frontend dependencies
        run: cd frontend && npm ci

      - name: Build frontend
        run: make frontend

      - name: Check the release scripts
        run: make -C desktop release-tests

      - name: Dry run the release
        run: make -C desktop release VERSION=0.0.0-ci

      - name: Check the cask's style
        run: brew style desktop/build/release/Casks/tap-desktop.rb

      - name: Summarize
        if: always()
        run: |
          echo "## Desktop release dry run" >> "$GITHUB_STEP_SUMMARY"
          if [ -f desktop/build/release/release-summary.md ]; then
            cat desktop/build/release/release-summary.md >> "$GITHUB_STEP_SUMMARY"
            grep -c '^- skipped:' desktop/build/release/release-summary.md | xargs -I{} echo "{} steps skipped, as expected with no secrets" >> "$GITHUB_STEP_SUMMARY"
            grep -q '^notarized=no$' desktop/build/release/release-state.env && echo "state: not notarized, feed unsigned, nothing publishable" >> "$GITHUB_STEP_SUMMARY"
          fi

      - name: Keep the DMG
        if: always()
        uses: actions/upload-artifact@v7
        with:
          name: desktop-release-dry-run
          path: |
            desktop/build/release/Tap-0.0.0-ci-unnotarized.dmg
            desktop/build/release/Tap-0.0.0-ci-unnotarized.dmg.sha256
            desktop/build/release/appcast-unsigned.xml
            desktop/build/release/Casks/tap-desktop.rb
            desktop/build/release/release-summary.md
            desktop/build/release/release-state.env
          retention-days: 7
```

- [ ] **Step 5: Check the workflows parse, and commit**

Run: `for f in .github/workflows/release.yml .github/workflows/ci.yml; do ruby -ryaml -e 'YAML.load_file(ARGV[0]); puts "#{ARGV[0]} parses"' "$f"; done && grep -n 'secrets\.' .github/workflows/release.yml && grep -c 'github.event.inputs.version' .github/workflows/release.yml`
Expected: both parse; every `secrets.` line the grep prints is an `env:` value (`NAME: ${{ secrets.NAME }}`), the CLI job's checkout `token:` or a `GH_TOKEN`; none is inside a `run:` block; the version expression appears in the CLI job (as before), the `desktop` job's `env:` and its checkout `ref:`, and in no `run:` line. Then:

```bash
git add desktop/scripts/mark-latest.sh desktop/scripts/mark-latest-test.sh .github/workflows/release.yml .github/workflows/ci.yml
git commit -m "ci: a desktop release job that publishes by the release's state, and a no-secrets dry run on every pull request"
```

The controller pushes and reads CI: the Desktop Release Dry Run job is green, its summary lists seven `skipped:` lines and the `done:` lines, `brew style` passes, and the artifact holds the six files. The release job itself is exercised by the person's first release after merge, or by a `workflow_dispatch` of a pre-release such as `2.0.0-rc.2` from `main`, which (before the secrets exist) adds nothing to the release, keeps `Tap-2.0.0-rc.2-unnotarized.dmg` as the `desktop-release-unnotarized` artifact, pushes no cask, and leaves `latest` alone.

Mutations, applied locally and reverted: in `mark-latest.sh`, drop the `appcast.xml` case (`mark-latest-test.sh` fails on "a protected feed must not be replaced"); treat every `gh` failure as 404 (fails on "a gh error must not mark anything"); mark a pre-release (fails on the pre-release line).

---

### Task 9: Documentation, the changelog and the roadmap

**Files:**
- Modify: `desktop/README.md` (a "Release" section and the manual pass), `README.md` (install Tap Desktop), `docs/getting-started.md` (the same), `CONTRIBUTING.md` (the Release Process section), `CHANGELOG.md` (`[Unreleased]`), `docs/superpowers/plans/2026-09-22-tap-desktop-roadmap.md` (D7's row)

- [ ] **Step 1: The desktop README**

After the "Test" section of `desktop/README.md`, add:

````markdown
## Release

```sh
make -C desktop release-tests                # every release script's own test, no build, no network
make -C desktop release VERSION=0.0.0-dev    # the dry run: builds, signs ad-hoc, writes the DMG
```

Release builds are Apple silicon only. `make -C desktop release` builds the
Release configuration for `arm64` with the version and a build number
(`scripts/build-number.sh`: `alpha` below `beta` below `rc` below the
final, so `2.0.0-beta.7 < 2.0.0-rc.1 < 2.0.0`), checks that no test hook is
in the binary, then runs `scripts/release.sh`, which signs the app inside
out with the hardened runtime and `Tap/Tap.entitlements` (the microphone,
for recorded talks), notarizes the app and the DMG, writes the DMG, its
`.sha256`, the release notes (`Tap-<version>.md`, from the changelog's
section for the version), the Sparkle `appcast.xml` and the
`Casks/tap-desktop.rb` cask into `build/release`, verifies them
(architectures, the bundled `tap --version`, the plist keys, the
entitlements, no test code, the signatures) and records every step in
`build/release/release-summary.md` and the outcome in
`build/release/release-state.env`. Each step that needs a secret is
skipped, by name, when the secret is absent, so the dry run needs none
and launches nothing. What is not notarized is named so
(`Tap-<version>-unnotarized.dmg`) and never becomes a release asset (the
job keeps it as a workflow artifact for seven days); an appcast that is
not signed is `appcast-unsigned.xml`; neither is ever offered to a person.
A rejected notarization stops the release.

The release job (`.github/workflows/release.yml`, job `desktop`) runs the
same target on a macOS runner after the CLI release, then publishes by the
state file: for a notarized DMG, the DMG and its checksum, then the notes
and the signed feed when the feed is signed; the cask to the Homebrew tap
only for a notarized final; and the release becomes GitHub's "latest"
(where the app's feed URL points) only once its feed is up, or when no
earlier release ever carried one (otherwise the summary names the
`gh release edit v<version> --latest` to run once the feed is fixed). A DMG
that was not notarized reaches the release page never; it is the workflow
artifact `desktop-release-unnotarized`. CI's Desktop
Release Dry Run job runs the dry run on every pull request and keeps the
DMG as an artifact.

Updates come through Sparkle's standard UI. The feed is the latest
release's `appcast.xml`
(`https://github.com/MiniCodeMonkey/tap/releases/latest/download/appcast.xml`);
`SUPublicEDKey` in `project.yml` checks the signature of every update, of
the notes and of the feed itself, which the app requires
(`SURequireSignedFeed`). A talk is never interrupted: `UpdateController`
refuses a check, drops a found update and postpones a relaunch while a talk
runs (`AppEnvironment.updatesMayInterrupt`), the postponed relaunch runs
once the last talk's windows are down, and Play is refused, with the
talk-not-started bar, while an update Sparkle already started is in
progress. The updater never starts under tests or in a `0.0.0` build.

The secrets the release job reads, all optional, each skipping its step
when absent, set with `gh secret set` so no secret lands on disk or in the
clipboard:

```sh
base64 -i certificate.p12 | gh secret set APPLE_DEVELOPER_ID_APPLICATION_P12   # the Developer ID Application certificate with its key
gh secret set APPLE_DEVELOPER_ID_APPLICATION_PASSWORD                          # prompts; the .p12's password
gh secret set APPLE_NOTARY_KEY < AuthKey_XXXXXXXXXX.p8                         # an App Store Connect API key, Developer role or higher
gh secret set APPLE_NOTARY_KEY_ID                                              # prompts; the XXXXXXXXXX of the file name
gh secret set APPLE_NOTARY_ISSUER_ID                                           # prompts; the issuer UUID
op read "op://<vault>/<item>/<field>" | gh secret set SPARKLE_PRIVATE_KEY      # the EdDSA key, from its 1Password item (the only route: nothing on disk)
```

`HOMEBREW_TAP_TOKEN` exists already. A pre-release never reaches the feed
or the cask.

What only a person can check, from the first notarized DMG: drag the app
to Applications and launch it (Gatekeeper accepts it with no dialog); Play
a deck with recording on and speak, then confirm the recording has sound
(the hardened runtime's microphone entitlement; CI's recorder is a fake);
Tap > Check for Updates… against the feed; `brew install --cask
MiniCodeMonkey/tap/tap-desktop` on another Mac; `brew audit --cask --online
tap-desktop` with the tap installed (CI runs `brew style` alone).
````

- [ ] **Step 2: Install docs, contributing, changelog, roadmap**

In `README.md`, after the "Using Homebrew (macOS/Linux)" block:

````markdown
### Tap Desktop (Apple silicon, macOS 14 or later)

The native app, with tap built in:

```bash
brew install --cask MiniCodeMonkey/tap/tap-desktop
```

Or download `Tap-<version>.dmg` from the [releases page](https://github.com/MiniCodeMonkey/tap/releases). The app updates itself through Tap > Check for Updates…
````

In `docs/getting-started.md`, the same block after the Homebrew section, headed `### Tap Desktop (macOS)`.

In `CONTRIBUTING.md`, at the end of "Release Process" (after the list of what the workflow does), add:

```markdown
The workflow's second job, `desktop`, builds Tap Desktop for Apple silicon
on a macOS runner from the tag, signs and notarizes it, and adds
`Tap-<version>.dmg`, its checksum, the release notes and Sparkle's
`appcast.xml` to the same release; for a notarized final it also updates
the `tap-desktop` cask in the Homebrew tap and marks the release "latest"
(the CLI job publishes with `make_latest: false`, since the app's feed URL
points at "latest"). Each signing step is skipped, by name, when its secret
is absent, a DMG that was not notarized never reaches the release (it is
kept as a workflow artifact for seven days), and the job's summary says
which steps ran. `desktop/README.md` lists the secrets, how to set them, and how
to run the same pipeline locally with none of them.
```

In `CHANGELOG.md`, under `## [Unreleased]` / `### Added`, a bullet at the end:

```markdown
- **Tap Desktop ships as a macOS app for Apple silicon** - A signed and notarized `Tap-<version>.dmg` on every release, `brew install --cask MiniCodeMonkey/tap/tap-desktop`, and updates through Sparkle from Tap > Check for Updates… An update never interrupts a talk: no check runs and no restart happens while you present, a postponed restart waits for the talk's windows to close, and Play waits for an update that is already installing.
```

In the roadmap, D7's row becomes `| D7 | \`2026-09-27-desktop-release.md\` | Desktop milestone 7 | D6 | written, revised after review; runs with no secrets, publishes only what is notarized and signed once they exist |`.

- [ ] **Step 3: Commit**

```bash
git add desktop/README.md README.md docs/getting-started.md CONTRIBUTING.md CHANGELOG.md
git add -f docs/superpowers/plans/2026-09-22-tap-desktop-roadmap.md
git commit -m "docs: how Tap Desktop is released, installed and updated"
```

---

### Task 10: Settings > General: automatic update checks (waits for the mockup)

**This task builds nothing until the controller records the person's sign-off on the "SettingsUpdates" board in the ledger.** The board: in the General pane, under the Saving card, an "Updates" card with one row, a checkbox "Check for updates automatically" (no hint; Sparkle asks the same question on the second launch, this is where the answer is changed later). It follows the SettingsGeneral board's card style exactly. If the person declines, this task is dropped and Sparkle's permission prompt stays the only control.

**Files:**
- Modify: `desktop/Tap/Settings/GeneralSettingsViewController.swift` (D6; the Updates card)
- Modify: `desktop/Tap/App/UpdateController.swift` (`automaticChecks` with a test seam)
- Create: `desktop/TapTests/SettingsUpdatesTests.swift`

**Interfaces:**
- Consumes: `SettingsCard(title:rows:)`, `GeneralSettingsViewController.loadView`, `refresh()`, `changed(_:)` (D6 Task 12); `SettingsWindowController.shared.show(pane:)`, `.general`; `SPUUpdater.automaticallyChecksForUpdates`.
- Produces: `UpdateController.automaticChecks: Bool` (get and set), `UpdateController.automaticChecksStore: (read: () -> Bool, write: (Bool) -> Void)` (Sparkle's property by default; a test replaces it so no test writes the app's real defaults), `GeneralSettingsViewController.automaticUpdatesCheckbox`.

- [ ] **Step 1: Write the failing hosted test**

`desktop/TapTests/SettingsUpdatesTests.swift`:

```swift
import XCTest
@testable import Tap

final class SettingsUpdatesTests: HostedTestCase {
    var updates: UpdateController { (NSApp.delegate as! AppDelegate).updateController }

    func testTheUpdatesCardMirrorsSparklesSetting() throws {
        // A store of the test's own: Sparkle's real one is the app's defaults.
        var stored = true
        let previous = updates.automaticChecksStore
        updates.automaticChecksStore = (read: { stored }, write: { stored = $0 })
        defer { updates.automaticChecksStore = previous }

        SettingsWindowController.shared.show(pane: .general)
        let general = SettingsWindowController.shared.general
        let checkbox = try XCTUnwrap(general.automaticUpdatesCheckbox)
        XCTAssertEqual(checkbox.title, "Check for updates automatically")
        XCTAssertEqual(checkbox.state, .on)

        checkbox.performClick(nil)
        XCTAssertFalse(stored, "the click reaches the store")
        stored = true
        general.refresh()
        XCTAssertEqual(checkbox.state, .on, "refresh reads the store")
        stored = false
        general.refresh()
        XCTAssertEqual(checkbox.state, .off, "and follows it both ways")
        SettingsWindowController.shared.window?.orderOut(nil)
    }
}
```

- [ ] **Step 2: Build to verify it fails**

Run: `make -C desktop test-build`
Expected: no `automaticChecksStore`, no `automaticUpdatesCheckbox`.

- [ ] **Step 3: The seam and the card**

In `UpdateController`:

```swift
    /// Sparkle's "check automatically" answer, which the Settings checkbox
    /// shows and changes. A test replaces the store so it never writes the
    /// app's defaults, where Sparkle keeps the real one.
    lazy var automaticChecksStore: (read: () -> Bool, write: (Bool) -> Void) = (
        read: { [weak self] in self?.updater.automaticallyChecksForUpdates ?? false },
        write: { [weak self] value in self?.updater.automaticallyChecksForUpdates = value }
    )

    var automaticChecks: Bool {
        get { automaticChecksStore.read() }
        set { automaticChecksStore.write(newValue) }
    }
```

In `GeneralSettingsViewController`: a property `let automaticUpdatesCheckbox = NSButton(checkboxWithTitle: "Check for updates automatically", target: nil, action: nil)`; in `loadView`, before the cards: `automaticUpdatesCheckbox.target = self; automaticUpdatesCheckbox.action = #selector(changed(_:)); automaticUpdatesCheckbox.setAccessibilityIdentifier("settings-automatic-updates")`; `let updatesCard = SettingsCard(title: "Updates", rows: [("Updates", nil, automaticUpdatesCheckbox)])` (the board decides whether the row label is shown; follow it), added to the stack after `savingCard` and to the width loop; in `refresh()`: `automaticUpdatesCheckbox.state = (NSApp.delegate as? AppDelegate)?.updateController.automaticChecks == true ? .on : .off`; in `changed(_:)`, a case `case let button as NSButton where button === automaticUpdatesCheckbox: (NSApp.delegate as? AppDelegate)?.updateController.automaticChecks = button.state == .on`.

- [ ] **Step 4: Build, and hand the test to CI**

Run: `make -C desktop test-build && make -C desktop core-test`
Expected: compiles. CI's Desktop Tests job runs `SettingsUpdatesTests.testTheUpdatesCardMirrorsSparklesSetting`; D6's `testSettingsWindow` and `testGeneralSettings` still pass (they list tabs and D6's controls, not the cards).

- [ ] **Step 5: Commit**

```bash
git add desktop/Tap/App/UpdateController.swift desktop/Tap/Settings/GeneralSettingsViewController.swift desktop/TapTests/SettingsUpdatesTests.swift
git commit -m "feat(desktop): Settings > General shows and changes Sparkle's automatic check"
```

Mutations, each a patch in `mutations-c/`: in `changed(_:)`, drop the checkbox case (`Test: TapTests/SettingsUpdatesTests/testTheUpdatesCardMirrorsSparklesSetting`; expected: fails on "the click reaches the store"); in `refresh()`, set `.on` always (expected: fails on "and follows it both ways"); in `automaticChecksStore`'s default `write`, drop the assignment (into `survivors-c/`: the test replaces the store; Sparkle's own property is not the app's to test).

---

## Final check

- [ ] `make -C desktop release-tests` green (twelve scripts); `make -C desktop core-test` green; `make -C desktop test-build` compiles; `make -C desktop check-release-hooks` and `make -C desktop check-release-app-hooks` both print their "no test-only hook" line; `make -C desktop release VERSION=0.0.0-dev` writes `Tap-0.0.0-dev-unnotarized.dmg`, its checksum, `appcast-unsigned.xml`, `Casks/tap-desktop.rb`, the summary with seven `skipped:` lines and the state with `notarized=no`; `make -C desktop check-scenarios` unchanged and green (no new rows). `shellcheck -S warning desktop/scripts/*.sh` is clean where shellcheck is installed.
- [ ] `grep -rn 'set -x' desktop/scripts` finds nothing. `grep -rn 'SPARKLE_PRIVATE_KEY\|APPLE_NOTARY_KEY\b\|APPLE_DEVELOPER_ID_APPLICATION_P12\|HOMEBREW_TAP_TOKEN' desktop/scripts .github` finds only reads into `sign_update`'s stdin, the `.p8` and `.p12` temporary files, the `for secret` loop, the git config header value, the skip lines and the workflow's `env:` values; nothing echoes one. `grep -n 'secrets\.' .github/workflows/*.yml` shows `env:` values, `token:` and `GH_TOKEN` lines only.
- [ ] `grep -rn 'evaluateJavaScript\|callAsyncJavaScript' desktop/Tap` is unchanged from D6; `grep -rn 'NSAlert\|runModal' desktop/Tap/App/UpdateController.swift` finds nothing (Sparkle's alerts are Sparkle's).
- [ ] `grep -n 'isPresenting' desktop/Tap/App/UpdateController.swift` finds nothing: every decision reads `updatesMayInterrupt`; `FocusHintTests.testNothingInterruptsTheTalk` is unchanged.
- [ ] `grep -n 'exactVersion: 2.10.0' desktop/project.yml`, `grep -n 'version="2.10.0"' desktop/scripts/fetch-sparkle-tools.sh` and `grep -n 'sparkle-tools-2.10.0' desktop/scripts/sparkle-sign.sh` agree. `grep -n 'ARCHS: arm64' desktop/project.yml`, `grep -n 'arch: :arm64' desktop/release/tap-desktop.rb.template` and `grep -n 'hardwareRequirements>arm64' desktop/scripts/write-appcast.sh` all hit.
- [ ] No em dash in any file this plan touched: `git diff main --name-only | xargs grep -ln "$(printf '\342\200\224')"` prints no file.
- [ ] The controller pushes and reads CI: Go Tests, Frontend Tests, E2E, Theme Checks, Desktop Tests (`UpdaterTests` eight green, `SettingsUpdatesTests` one green, D4's `FocusHintTests` green), Desktop UI Tests, Desktop Benchmarks (no Sparkle window in a recording), Desktop Release Dry Run (artifact present with six files, `brew style` green). `grep -n 'identity=' desktop/scripts/release.sh` shows the state file's heredoc has no such line. The mutation branches `mutations/d7-batch-a` and `d7-batch-c` hold the patches; every one with a named killing test is killed.
- [ ] The final review reads `release-summary.md` and `release-state.env` from the dry-run artifact and the `desktop` job's design against "The release, end to end" above.

## Steps that wait for a mockup

| Step | Board | What it builds |
|---|---|---|
| Task 10, whole | SettingsUpdates | The Updates card in Settings > General with the "Check for updates automatically" checkbox |

Everything else uses Sparkle's standard UI (its permission prompt, update alert, release notes view, progress and relaunch windows), the menu item D6's MenusFile board already draws (Tap > Check for Updates…), D4's talk-not-started bar with one more sentence, and a plain DMG. Two more screens exist only if the person wants them, each needing a mockup first and none built by this plan: a DMG window with background art and icon positions (needs Finder scripting on the runner), and a custom release notes view in the update window (Sparkle renders the release's Markdown notes today).

## Open questions

Product decisions this plan makes that the spec leaves open. Each line is the default the plan implements, the alternative, and what it costs if the person wants it otherwise. Decisions the person made on 2026-09-27 (Apple silicon only; nothing un-notarized or unsigned offered; the microphone entitlement; a signed feed; label-ranked build numbers) are rulings above, not questions.

1. **Where the feed lives.** Default: `https://github.com/MiniCodeMonkey/tap/releases/latest/download/appcast.xml`, the `latest` release's asset, no hosting added; `mark-latest.sh` moves `latest` only when the feed is up, so pre-release users get no Sparkle updates until the next final. Alternative: host `appcast.xml` (and a `beta` channel feed) on tap.sh through the publish project. Cost if wrong: one plist value and a copy step in the job.
2. **Automatic checks.** Default: Sparkle's standard permission prompt on the second launch (`SUEnableAutomaticChecks` unset), and Task 10's checkbox to change the answer later. Alternative: `SUEnableAutomaticChecks: true`, no prompt. Cost if wrong: one plist key.
3. **Release notes in the update window.** Default: the version's changelog section as `Tap-<version>.md` on the release, signed, linked from the feed (Sparkle 2.9 renders Markdown on macOS 12 and later). Alternative: a link to the GitHub release page (HTML, would need signing too under `SURequireSignedFeed`). Cost if wrong: one script branch.
4. **Two notarizations.** Default: the app (zipped, then stapled) and then the DMG, so a dragged app launches offline with its ticket. Alternative: the DMG only, one submission, an online check at the app's first launch. Cost if wrong: one branch in `release.sh`.
5. **The build-number ranges.** `alpha` 1 to 19, `beta` 20 to 49 (thirty betas), `rc` 50 to 89 (forty candidates), final 99, `dev`/`ci` 1. Alternative: other widths, while nothing has shipped. Cost if wrong: three numbers and the test, before the first release only.
6. **A plain DMG.** Default: the app and an Applications link, no art. Alternative: a background image and icon layout after a mockup. Cost if wrong: a mockup and Finder scripting on the runner.
7. **Check for Updates… during a talk.** Default: the item is disabled by validation, and the delegate refuses anyway with "Tap does not check for updates during a talk." in Sparkle's own alert if a check slips through. Alternative: disabled only. Cost if wrong: one guard.
8. **Play during an update session.** Default: refused, with D4's talk-not-started bar saying "Sparkle is checking for or installing an update. Let it finish, or close its window, then press Play again." (the same sentence covers a check in flight, an open update alert and an install). Alternative: let Play start and accept that Sparkle's download or "Install and Relaunch" window can appear over the talk (no delegate hook stops them). Cost if wrong: one guard and one sentence.
9. **Decided (the controller, 2026-09-27):** an un-notarized DMG is never a release asset. It stays a workflow artifact (`desktop-release-unnotarized`, seven days) and the summary says so. Closed.
10. **Decided (the controller, 2026-09-27): `make_latest: false` for the CLI job stays.** A final whose desktop job fails, or runs before the Sparkle key exists while an earlier release already has a feed, is not marked `latest`; the summary then prints the one command to run once the feed is fixed (`gh release edit vX --latest`); the formula's versioned URLs are unaffected. Closed.
11. **The dry run on every pull request.** Default: `release-dry-run` runs with the other desktop jobs (about ten minutes of a macOS runner). Alternative: only on `main` and `workflow_dispatch`. Cost if wrong: a `paths` filter or an `if`.
12. **Hardened runtime in Release only.** Default: Debug and Benchmark stay off, since the hosted tests inject into them. Cost if wrong: two lines in `project.yml`.
13. **Sparkle pinned at 2.10.0** in the package and the tools, bumped by hand. Cost if wrong: two lines.
14. **The cask in the existing tap** (`MiniCodeMonkey/homebrew-tap`, `Casks/tap-desktop.rb`, the same token). Alternative: a separate cask repository. Cost if wrong: `HOMEBREW_TAP_REPO`'s value.
15. **The person's runs.** The local dry run (`make -C desktop release VERSION=0.0.0-dev`) is the one release step an agent runs on their Mac; hosted tests, UI tests, benchmarks and the first real release run on CI. The first release with the secrets present is the person's, from the Actions tab; the README's manual pass (a launch from the DMG, a recording with sound, Check for Updates…, `brew install --cask`, `brew audit --cask`) is theirs too.

## What the person adds later

The job runs today with none of these; each one turns on its step. Every command pipes straight into `gh secret set`, so no secret lands on disk or in the clipboard.

| Secret or setting | Where | What it is | How to set it |
|---|---|---|---|
| `APPLE_DEVELOPER_ID_APPLICATION_P12` | repository secret | The Developer ID Application certificate with its private key, base64 | Keychain Access: export the certificate and key as `.p12`; `base64 -i certificate.p12 \| gh secret set APPLE_DEVELOPER_ID_APPLICATION_P12`; delete the `.p12` |
| `APPLE_DEVELOPER_ID_APPLICATION_PASSWORD` | repository secret | The `.p12`'s password | `gh secret set APPLE_DEVELOPER_ID_APPLICATION_PASSWORD` (prompts) |
| `APPLE_NOTARY_KEY` | repository secret | An App Store Connect API key (`.p8`), as text, with the Developer role or higher | App Store Connect > Users and Access > Integrations > App Store Connect API > Team Keys; `gh secret set APPLE_NOTARY_KEY < AuthKey_XXXXXXXXXX.p8` |
| `APPLE_NOTARY_KEY_ID` | repository secret | The key's ID (the `XXXXXXXXXX` in the file name) | `gh secret set APPLE_NOTARY_KEY_ID` (prompts) |
| `APPLE_NOTARY_ISSUER_ID` | repository secret | The issuer UUID | `gh secret set APPLE_NOTARY_ISSUER_ID` (prompts) |
| `SPARKLE_PRIVATE_KEY` | repository secret | The EdDSA private key whose public half is `Pc0PbtYL9fkyK3XnqULlpKLlH94TE4OWx4Q3n74EftU=` | `op read "op://<vault>/<item>/<field>" \| gh secret set SPARKLE_PRIVATE_KEY`, from the 1Password item that holds the key (the login keychain holds the same key under the account `tap-desktop`; `generate_keys -x` refuses an existing path such as `/dev/stdout` and would write a file, so it is not the route) |
| `HOMEBREW_TAP_TOKEN` | repository secret | exists | already set; the cask push waits for a notarized final regardless |
| `HOMEBREW_TAP_REPO` | repository variable, optional | The tap, default `MiniCodeMonkey/homebrew-tap` | only if the cask should live elsewhere |
| The cask repository | `MiniCodeMonkey/homebrew-tap` | `Casks/tap-desktop.rb`, created by the first notarized final's job | nothing to do; the token has write access already |
| Apple Developer Program enrolment | developer.apple.com | Needed for the certificate and the notary key | the person, after enrolment |

## What this plan found missing in the spec

- The spec says "one release job" for the app and the CLI; the CLI job runs on Ubuntu and cannot build the app, so the app is a second job of the same workflow, after the first, on the same tag and release, and the first no longer marks the release `latest`.
- The spec names no feed URL and no hosting for the appcast; the plan uses the GitHub `latest` release asset (open question 1) and moves `latest` itself.
- The spec says "Developer ID signing and notarization" without saying what happens before the credentials exist; the person's decisions (2026-09-22 and 2026-09-27) do, and this plan's skip-by-name and publish-by-state rules are those decisions made testable.
- The spec says macOS 14 or later and nothing about architectures; the bundled tap was arm64-only inside a universal app until the person chose Apple silicon only.
- The spec's recorded talks take the microphone through tap; nothing named the entitlement the hardened runtime needs for that. Task 2 adds it; only a person can prove it (the README's manual pass).
- No feature file has a release, update or install scenario, so `scenarios.txt` gains nothing; the one Sparkle line in `05-presenting.feature` is D4's and stays D4's.
- `CFBundleVersion` was the constant `1` since D2; Sparkle needs it to rise, and the repository's own tags use `beta` and `rc` for one version, so Task 1 ranks the labels.
- D6's Settings boards have no Updates card; Task 10 waits for one.

## Execution handoff

Plan complete and saved to `docs/superpowers/plans/2026-09-27-desktop-release.md`, revised after its review (`d7-plan-review.md`) and the person's decisions of 2026-09-27. Execute with superpowers:subagent-driven-development, one fresh subagent per task, as the roadmap requires, on `feat/desktop-release` cut from `main` after D6 has merged. Batch A (Tasks 1 to 3) is where the app changes and the one hosted test class lives; Batches B and C are shell and YAML that every implementer proves locally with `make -C desktop release-tests` and the dry run, and CI proves once more on the pull request. Task 10 waits for the SettingsUpdates board's sign-off and can trail the pull request as its own if the sign-off is late.
