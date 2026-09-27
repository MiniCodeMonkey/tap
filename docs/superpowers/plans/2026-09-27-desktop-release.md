# Tap Desktop release (D7): implementation plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** One `make -C desktop release VERSION=x.y.z` builds the Release app with its version and a build number, signs it (Developer ID when the certificate is present, ad-hoc otherwise), notarizes and staples the app and the DMG when the notary key is present, writes the DMG, its checksum, a Sparkle appcast (EdDSA-signed when the Sparkle key is present) and the `tap-desktop` cask; the release job runs it on a macOS runner after the CLI release, uploads the DMG and the appcast to the same GitHub release and pushes the cask to the Homebrew tap; every signing, notarization, Sparkle-signing and cask-push step is skipped cleanly, by name, when its secret is absent, so the whole pipeline runs today with no secrets at all, and the app checks for updates through Sparkle's standard UI without ever interrupting a talk.

**Architecture:** The app gains Sparkle 2.10.0 as a Swift package pinned by exact version, two Info.plist keys (`SUFeedURL`, `SUPublicEDKey`), and one class of its own, `UpdateController`, which holds the `SPUStandardUpdaterController`, gives Tap > Check for Updates… its action, and is Sparkle's delegate: it refuses a check while a talk runs, drops an update found during a talk, and postpones a relaunch until the talk ends, all through `AppEnvironment.updatesMayInterrupt` (D4). The rule itself lives in `TapDesktopCore` as `UpdateGate`, pure and tested without Sparkle. The updater never starts under tests or in a `0.0.0` build. Everything else is shell under `desktop/scripts/`, one script per step with a test script beside it, in the style of `check-scenarios.sh`: `build-number.sh` (a monotonic `CFBundleVersion` from the semantic version), `sign-app.sh` (inside out, hardened runtime, `--timestamp` only with a real identity), `signing-identity.sh` (a temporary keychain from the certificate secret, removed at exit), `notarize.sh` (`notarytool submit --wait`, then `stapler`), `make-dmg.sh` (`hdiutil`, the app and an Applications link, no Finder), `sparkle-sign.sh` (`sign_update --ed-key-file -` from the pinned Sparkle tools), `write-appcast.sh` (the one-item feed from the app's own Info.plist), `render-cask.sh` and `publish-cask.sh` (a template in this repository, pushed to the tap the CLI formula already uses), and `release.sh`, the orchestrator that runs them in order, records what it skipped in `release-summary.md`, and never prints a secret. The Makefile gains `release-build`, `check-release-app-hooks`, `release-tests` and `release`. `.github/workflows/release.yml` gains a `desktop` job on `macos-15` that runs `make -C desktop release` with the secrets in its environment and uploads the results; `ci.yml` gains a `release-dry-run` job that runs the same target with no secrets on every pull request and keeps the DMG as an artifact, which is the proof the pipeline works before any secret exists.

**Tech Stack:** Swift 6.3 compiler in Swift 5 language mode, AppKit, Sparkle 2.10.0 (`SPUStandardUpdaterController`, `SPUUpdaterDelegate`), XcodeGen (`packages` with `exactVersion`, `info.properties`, per-config `ENABLE_HARDENED_RUNTIME`), `codesign`, `xcrun notarytool`, `xcrun stapler`, `hdiutil`, `ditto`, `PlistBuddy`, `xmllint`, `shasum`, `security` (a temporary keychain), Sparkle's `sign_update` from the `Sparkle-2.10.0.tar.xz` release archive (sha256 `c2bf58aa8387266ac179357b1415d6f2635f044da8be41042af32425dae6da0c`), Homebrew casks (`brew style`), GitHub Actions on `macos-15` with Xcode 26, `gh release upload`, POSIX `sh` for every script, XCTest for the hosted tests, `swift test` for the Core tests.

**Spec:** `docs/superpowers/specs/2026-09-22-tap-desktop-design.md` (milestone 7; the sections "Platform and repo", "Processes", "Settings", "Menus and accessibility", "Security summary", "Distribution" and "Testing"), the D7 outline and the person's decision of 2026-09-22 in `docs/superpowers/plans/2026-09-22-tap-desktop-roadmap.md` ("build all of it so it runs once the secrets are added, and use ad-hoc signing locally ... The release job skips signing steps when their secrets are absent"), and `docs/superpowers/specs/tap-desktop-features/05-presenting.feature`'s "Nothing interrupts the talk" ("Sparkle shows no update prompt and never restarts the app"), already claimed by D4 (`FocusHintTests.testNothingInterruptsTheTalk` asserts `updatesMayInterrupt`); this plan gives that assertion its consumer. No feature file has a release scenario, so `desktop/scenarios.txt` gains no row. The existing release tooling is the pattern: `.github/workflows/release.yml` (the version regex, `prepare-changelog.sh`, `softprops/action-gh-release`, the Homebrew formula step with `HOMEBREW_TAP_TOKEN` and `HOMEBREW_TAP_REPO`), `.github/workflows/ci.yml`'s desktop jobs (Xcode 26, xcodegen, `make frontend` first), `desktop/scripts/build-tap.sh` (how the bundled tap gets `TAP_VERSION`), `desktop/scripts/check-scenarios.sh` and `check-scenarios-test.sh` (a POSIX script with a test script beside it).

**Depends on:** D6 merged (`feat/desktop-creating-export-settings`): its Tap menu holds `Check for Updates…` with no action, and its Settings window has the General pane Task 10 extends. If D7 starts before D6 has merged, Task 3 adds the menu item itself (the same line D6's Task 12 writes) and Task 10 waits.

**Branch:** `feat/desktop-release`, branched from `main` after D6's pull request has merged, in a worktree at `/Users/codemonkey/projects/tap-d7`. One pull request. Mutation patches go under `.superpowers/sdd/d7/mutations-<batch>/` and `survivors-<batch>/`, as D5 did.

**Known facts (the person, 2026-09-27):** the Sparkle EdDSA public key is `Pc0PbtYL9fkyK3XnqULlpKLlH94TE4OWx4Q3n74EftU=`; its private key is in the person's login keychain under the account `tap-desktop` and in 1Password, and reaches CI only as the secret `SPARKLE_PRIVATE_KEY`. `HOMEBREW_TAP_TOKEN` exists as a repository secret; the tap `MiniCodeMonkey/homebrew-tap` has `Formula/tap.rb` and no `Casks/` folder yet. The Apple Developer credentials come after enrolment. No agent runs anything locally that opens a window.

## Global Constraints

- Everything in D2's to D6's Global Constraints still holds: macOS 14 or later, AppKit core, the bundled `tap` from `build-tap.sh`, spelled-out identifiers, present-tense comments with no ticket references, no em dashes anywhere (`--`, a comma or a new sentence instead), `make frontend` before the first Xcode build, every Xcode build through `make`.
- **THE PERSON'S RULE (2026-09-25): nothing runs locally that opens windows on their screen.** Allowed locally: `make -C desktop project`, `build`, `test-build`, `bench-build`, `core-test`, `check-scenarios`, `check-release-hooks`, `go test`, every `desktop/scripts/*-test.sh`, `make -C desktop release-tests`, `make -C desktop release-build`, `make -C desktop check-release-app-hooks` and `make -C desktop release VERSION=0.0.0-dev` (builds, signs ad-hoc, writes the DMG and the appcast, launches nothing; `hdiutil attach -nobrowse` mounts without a Finder window). Never: `make -C desktop test`, `ONLY=...`, `uitest`, `bench`, `open Tap.app`, `open Tap.dmg`, `spctl --assess` on a launch, or `stapler` outside the release script. Hosted tests (`UpdaterTests`, `SettingsUpdatesTests`) run on CI: every "Run" step that names one says what the controller's CI run confirms.
- **THE SECRETS RULE.** No secret is written into the repository, a log, a workflow summary, a test, a fixture or a release artifact. Every script that takes a secret reads it from the environment (`APPLE_DEVELOPER_ID_APPLICATION_P12`, `APPLE_DEVELOPER_ID_APPLICATION_PASSWORD`, `APPLE_NOTARY_KEY`, `APPLE_NOTARY_KEY_ID`, `APPLE_NOTARY_ISSUER_ID`, `SPARKLE_PRIVATE_KEY`, `HOMEBREW_TAP_TOKEN`) or through a pipe into standard input; a secret that must be a file (the `.p8` key, the `.p12`) is written with `umask 077` into a `mktemp -d` folder that an `EXIT` trap removes; the temporary keychain is deleted by the same trap. No script uses `set -x`; no script echoes a variable that holds a secret; `security`, `notarytool` and `git` are run with their output filtered to what is not secret (the identity's name, the submission id and status, the commit hash). The workflow passes secrets only through `env:` and never in `run:` text. A step that finds its secret empty prints exactly one line, `skipped: <what> (<SECRET_NAME> is not set)`, appends it to `release-summary.md`, and exits 0; nothing else about that step happens. `release.sh` ends by printing the summary, so the job's log and step summary say what was skipped.
- **Ad-hoc is the default everywhere.** `codesign --sign -` when no identity is found; `sign-app.sh` is the one place that signs, and a dry run and a real run take the same path through it with a different identity. The release build must still pass the hook check: `make -C desktop check-release-app-hooks` proves `approvalAnswerForTests` is in neither the symbol table nor the strings of the Release binary and a known string (`TapExecutablePath`) is, so a stripped or renamed binary cannot pass by being empty.
- **A talk is never interrupted.** Every Sparkle decision that could show a window or restart the app passes through `UpdateGate` with `AppEnvironment.shared.isPresenting` (`updatesMayInterrupt` is its negation, kept for D4's test): a scheduled or user check is refused (`updater(_:mayPerform:)`), a found update is dropped (`updater(_:shouldProceedWithUpdate:updateCheck:)`), a relaunch is postponed (`updater(_:shouldPostponeRelaunchForUpdate:untilInvokingBlock:)`) and resumed once when the last talk ends. The updater never starts in a test process (`XCTestConfigurationFilePath` set, or `-TapDefaultsSuite` given) or in a `0.0.0` build, so no hosted test, UI test or benchmark ever reaches the network or Sparkle's windows, and a Debug app run from DerivedData never offers to replace itself.
- **Sparkle's standard UI only.** No update sheet, no custom update window, no custom release notes view. The one UI addition in this plan, a checkbox in Settings > General (Task 10), waits for the person's mockup sign-off and is ordered last; Sparkle's own permission prompt, update alert and progress window are Sparkle's, not new UI. The DMG is plain (the app and an Applications link, no background art): art would be new UI needing a mockup and Finder scripting on the runner.
- **Sparkle is pinned.** `exactVersion: 2.10.0` in `project.yml`; the tools come from `Sparkle-2.10.0.tar.xz` with its sha256 checked before extraction. A bump changes both places in one commit.
- **Every script is POSIX `sh`** (`#!/bin/sh`, `set -eu`, no arrays, no `[[`), runs from any directory (paths from `$(dirname "$0")`), and has a `<name>-test.sh` beside it that builds its own fixtures under `mktemp -d` and removes them in a trap. `make -C desktop release-tests` runs every test script. A test never touches the person's keychain, defaults, `~/Applications` or `/Applications`.
- **XCTest rules.** No `await` inside an `XCTAssert` autoclosure. Every wait goes through `waitUntil(timeout:)`. No test depends on a key window. Hosted tests reach Core through `@testable import Tap`. A test that needs a talk uses `PresentingTestCase.openDeckForPresenting`, `startPresenting` and `stopPresenting`.
- `weak self` in every closure that outlives a call, no `unowned`. No modal alerts of the app's own. No production code steals focus.

## Review Focus

Five conditions the spec implies that no scenario names, most likely to bite first. Each has its test pinned to the task that owns the code.

1. **A release with no secrets at all.** The person's decision: everything runs today. A missing secret must skip its own step and nothing else, name itself, and still leave a DMG, a checksum, an appcast and a rendered cask in the output folder. Task 7, `release-test.sh` (runs `release.sh` against a fake app with an empty environment and checks the five files and the six `skipped:` lines), and Task 8's `release-dry-run` CI job against the real app.
2. **An update found before a talk that would prompt during it.** A background check that starts a minute before Play finishes during the talk; `mayPerform` alone does not stop its alert. Task 3, `UpdaterTests.testAnUpdateFoundDuringATalkIsDropped` (the delegate's `shouldProceedWithUpdate` throws while presenting) and `UpdateGateTests.testAFoundUpdateIsDroppedWhilePresenting`.
3. **A relaunch postponed by a talk.** It must run when the talk ends, exactly once, and a second talk before it ran must not run it twice or lose it. Task 3, `UpdateGateTests.testAPostponedRelaunchRunsOnceWhenTheTalkEnds` and `UpdaterTests.testAPostponedRelaunchRunsWhenTheTalkEnds`.
4. **A pre-release build number.** `2.1.0-beta.3` must sort below `2.1.0` and above `2.0.9`, or Sparkle offers a beta as an upgrade from its own final; a version the regex refuses must fail the build, not produce `0`. Task 1, `build-number-test.sh`.
5. **The Release app's plist and binary.** The two Sparkle keys, the hardened runtime flag, the version and the build number must be in the built app, and no test hook may be; a dry run that passes with a wrong plist ships a feed nobody can reach. Task 1, `check-release-app-hooks`; Task 7, `verify-release.sh` (PlistBuddy reads of `SUFeedURL`, `SUPublicEDKey`, `CFBundleShortVersionString`, `CFBundleVersion`, `LSMinimumSystemVersion`, `codesign -d` shows `runtime`).

## The release, end to end

| Step | Local dry run (`make -C desktop release VERSION=0.0.0-dev`) | CI with every secret (`release.yml`, job `desktop`) | Script |
|---|---|---|---|
| Build | `xcodebuild -configuration Release TAP_VERSION=0.0.0-dev CURRENT_PROJECT_VERSION=1` (build number of `0.0.0-dev` is 1) | the same with the release version | Makefile `release-build`, `build-number.sh` |
| Hook check | `nm` and `strings` of the Release binary | the same | Makefile `check-release-app-hooks` |
| Identity | `-` (a `skipped:` line) | a temporary keychain from `APPLE_DEVELOPER_ID_APPLICATION_P12`, the identity's name | `signing-identity.sh import` |
| Sign | tap, Sparkle's pieces, the framework, the app, ad-hoc, `--options runtime` | the same with `--timestamp` and the identity | `sign-app.sh` |
| Notarize the app | skipped | `ditto -c -k` the app, `notarytool submit --wait`, `stapler staple Tap.app` | `notarize.sh` |
| DMG | `Tap-0.0.0-dev.dmg` with the app and an Applications link | the same, then `codesign` the DMG | `make-dmg.sh` |
| Notarize the DMG | skipped | `notarytool submit --wait`, `stapler staple Tap-x.dmg` | `notarize.sh` |
| Checksum | `Tap-0.0.0-dev.dmg.sha256` | the same | `release.sh` |
| Sparkle signature | skipped (the appcast says it is unsigned) | `sign_update --ed-key-file - -p` from the pinned tools | `sparkle-sign.sh`, `fetch-sparkle-tools.sh` |
| Appcast | `appcast.xml` with one item from the app's Info.plist | the same, with `sparkle:edSignature` | `write-appcast.sh` |
| Cask | `tap-desktop.rb` rendered into the output folder | the same, then pushed to `Casks/tap-desktop.rb` in the tap (never for a pre-release) | `render-cask.sh`, `publish-cask.sh` |
| Verify | `codesign --verify --deep --strict`, `hdiutil verify`, `xmllint`, the plist reads | the same plus `spctl --assess --type open --context context:primary-signature` on the DMG | `verify-release.sh` |
| Publish | nothing | `gh release upload` the DMG, the checksum and `appcast.xml` | `release.yml` |
| Summary | `release-summary.md` lists every step as done or skipped | the same, into `$GITHUB_STEP_SUMMARY` | `release.sh` |

The feed URL is `https://github.com/MiniCodeMonkey/tap/releases/latest/download/appcast.xml`: GitHub redirects it to the newest non-pre-release release's asset, so no hosting is added and a pre-release never reaches the feed (open question 1). The DMG's URL inside the appcast and the cask is the release's own, `https://github.com/MiniCodeMonkey/tap/releases/download/v<version>/Tap-<version>.dmg`.

## File structure

| Path | Responsibility |
|---|---|
| `desktop/scripts/build-number.sh`, `build-number-test.sh` | The integer `CFBundleVersion` for a semantic version |
| `desktop/scripts/sign-app.sh`, `sign-app-test.sh` | Signs an app bundle inside out with the hardened runtime, ad-hoc or with an identity |
| `desktop/scripts/signing-identity.sh`, `signing-identity-test.sh` | `import`: a temporary keychain from the certificate secret, prints the identity name or `-`; `remove`: deletes it |
| `desktop/scripts/notarize.sh`, `notarize-test.sh` | Submits a file to the notary service and staples the target, or skips |
| `desktop/scripts/make-dmg.sh`, `make-dmg-test.sh` | The DMG from an app |
| `desktop/scripts/fetch-sparkle-tools.sh` | Downloads and checks `Sparkle-2.10.0.tar.xz`, extracts `bin/` |
| `desktop/scripts/sparkle-sign.sh`, `sparkle-sign-test.sh` | The EdDSA signature of the DMG, or skips |
| `desktop/scripts/write-appcast.sh`, `write-appcast-test.sh` | The one-item appcast from the app's Info.plist and the DMG |
| `desktop/release/tap-desktop.rb.template` | The cask, with `__VERSION__` and `__SHA256__` |
| `desktop/scripts/render-cask.sh`, `render-cask-test.sh` | Renders the template for a DMG |
| `desktop/scripts/publish-cask.sh`, `publish-cask-test.sh` | Pushes the cask to the tap, or skips |
| `desktop/scripts/verify-release.sh` | The checks on the app, the DMG and the appcast |
| `desktop/scripts/release.sh`, `release-test.sh` | Runs the steps in order, writes `release-summary.md` |
| `desktop/Makefile` | Modify: `release-build`, `check-release-app-hooks`, `release-tests`, `release`, `RELEASE_DIR` |
| `desktop/project.yml` | Modify: the Sparkle package, `ENABLE_HARDENED_RUNTIME` for Release, `SUFeedURL`, `SUPublicEDKey` |
| `desktop/TapDesktopCore/Sources/TapDesktopCore/UpdateGate.swift` | The never-interrupt-a-talk rule and the may-start rule, pure |
| `desktop/TapDesktopCore/Tests/TapDesktopCoreTests/UpdateGateTests.swift` | Its tests |
| `desktop/Tap/App/UpdateController.swift` | Sparkle's controller and delegate, Check for Updates… |
| `desktop/Tap/App/AppDelegate.swift` | Modify: owns `updateController`, `checkForUpdates(_:)`, its validation |
| `desktop/Tap/App/MainMenu.swift` | Modify: Check for Updates… gets its action |
| `desktop/Tap/Settings/GeneralSettingsViewController.swift` | Modify (Task 10, after sign-off): the Updates card |
| `desktop/TapTests/UpdaterTests.swift`, `SettingsUpdatesTests.swift` | The hosted tests |
| `.github/workflows/release.yml` | Modify: the `desktop` job |
| `.github/workflows/ci.yml` | Modify: the `release-dry-run` job |
| `desktop/README.md`, `README.md`, `docs/getting-started.md`, `CONTRIBUTING.md`, `CHANGELOG.md`, `docs/superpowers/plans/2026-09-22-tap-desktop-roadmap.md` | Modify: install, release and secrets documentation; the changelog entry; D7's row |

## Batches

| Batch | Tasks | What it proves when it lands |
|---|---|---|
| A | 1, 2, 3 | A versioned, hardened, ad-hoc-signed Release app with Sparkle in it that never interrupts a talk |
| B | 4, 5, 6 | The DMG, the appcast and the cask from that app, each testable alone |
| C | 7, 8, 9, 10 | The orchestrator, the two workflows, the docs; and, last, the Settings checkbox once its mockup is signed off |

---

### Task 1: The build number, the Release build and the hook check on it

**Files:**
- Create: `desktop/scripts/build-number.sh`, `desktop/scripts/build-number-test.sh`
- Modify: `desktop/project.yml` (`configs.Release.ENABLE_HARDENED_RUNTIME`)
- Modify: `desktop/Makefile` (`VERSION`, `BUILD_NUMBER`, `RELEASE_APP`, `release-build`, `check-release-app-hooks`, `.PHONY`)

**Interfaces:**
- Consumes: `TAP_VERSION`, `MARKETING_VERSION`, `CURRENT_PROJECT_VERSION` as `project.yml` wires them into `CFBundleShortVersionString` and `CFBundleVersion`; `build-tap.sh`, which gives the bundled tap `TAP_VERSION`.
- Produces: `desktop/scripts/build-number.sh <version>` printing an integer (exit 1 with a message on a version the release regex refuses); `make -C desktop release-build VERSION=<v>` building `build/DerivedData/Build/Products/Release/Tap.app` (`RELEASE_APP`); `make -C desktop check-release-app-hooks` (depends on `release-build`). Task 7's `release` target and Task 8's jobs call both.

- [ ] **Step 1: Write the failing test script**

`desktop/scripts/build-number-test.sh`:

```sh
#!/bin/sh
# Checks build-number.sh: a final release sorts above every pre-release of
# its own version and below the next patch, and a bad version fails.
set -eu

script="$(cd "$(dirname "$0")" && pwd)/build-number.sh"

expect() {
	got=$("$script" "$1") || { echo "$1: exit $?"; exit 1; }
	[ "$got" = "$2" ] || { echo "$1: got $got, want $2"; exit 1; }
}

expect 0.0.0 99
expect 0.0.0-dev 1
expect 2.0.0 2000099
expect 2.0.0-beta.5 2000005
expect 2.0.0-rc.1 2000001
expect 2.1.0-beta.3 2010003
expect 2.1.0 2010099
expect 2.0.9 2000999
expect 10.20.30 10203099
expect 1.0.0-beta.98 1000098

# A pre-release number above 98 would reach the final's slot.
if "$script" 1.0.0-beta.99 >/dev/null 2>&1; then echo "beta.99 should fail"; exit 1; fi
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
# The release slot is 99 for a final version and the pre-release's own
# number (1 to 98) otherwise, so 2.1.0-beta.3 (2010003) sorts below 2.1.0
# (2010099) and above 2.0.9 (2000999). A pre-release with no number
# (0.0.0-dev) takes slot 1. The pre-release label itself does not order:
# one label per version. Minor and patch run to 99 each.
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
	number="${prerelease##*.}"
	case "$number" in
		*[!0-9]*) slot=1 ;;
		"") slot=1 ;;
		*) slot=$((number)) ;;
	esac
	if [ "$slot" -lt 1 ] || [ "$slot" -gt 98 ]; then
		echo "build-number.sh: a pre-release number runs from 1 to 98 ($version)" >&2
		exit 1
	fi
fi

echo $((major * 1000000 + minor * 10000 + patch * 100 + slot))
```

Note: `0.0.0-dev` has `number="dev"` (no dot), which is not numeric, so slot 1; `2.0.0-beta.5` has `number=5`; `1.0.0-` fails on its empty label.

- [ ] **Step 4: Run the test to verify it passes**

Run: `chmod +x desktop/scripts/build-number.sh && desktop/scripts/build-number-test.sh`
Expected: `build-number.sh is right`.

- [ ] **Step 5: The Release configuration and the Makefile targets**

In `desktop/project.yml`, under `settings.configs`, beside `Benchmark`:

```yaml
    Release:
      # Notarization needs the hardened runtime. Debug and Benchmark keep it
      # off: a hardened test host refuses the injected test bundles.
      ENABLE_HARDENED_RUNTIME: YES
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
# so an empty or renamed binary cannot pass.
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
/usr/libexec/PlistBuddy -c 'Print :CFBundleVersion' "$app/Contents/Info.plist"              # 1020304
"$app/Contents/Resources/tap" --version                                                     # tap version 1.2.3-beta.4
codesign -dv "$app" 2>&1 | grep -E 'flags=.*runtime|Signature=adhoc'                       # both lines
make -C desktop release-build VERSION=1.100.0 ; echo "exit $?"                              # fails: "minor and patch run to 99"
```

Also run `make -C desktop check-release-hooks` (the Benchmark check) to be sure the Release configuration change did not reach it: `no test-only hook in the release build`.

- [ ] **Step 7: Commit**

```bash
git add desktop/scripts/build-number.sh desktop/scripts/build-number-test.sh desktop/project.yml desktop/Makefile
git commit -m "build(desktop): a versioned Release build with a monotonic build number and the hardened runtime"
```

Mutations, applied and run locally then reverted exactly: in `build-number.sh`, make the final slot 0 (`build-number-test.sh` fails on `2.0.0`); drop the `-gt 98` check (fails on `beta.99`); in the Makefile, point `RELEASE_BINARY` at `/usr/bin/true` (`check-release-app-hooks` fails on "no strings to check").

---

### Task 2: Signing an app inside out

**Files:**
- Create: `desktop/scripts/sign-app.sh`, `desktop/scripts/sign-app-test.sh`

**Interfaces:**
- Consumes: an app bundle as `release-build` leaves it; Sparkle's layout inside `Contents/Frameworks/Sparkle.framework/Versions/B` (`XPCServices/Installer.xpc`, `XPCServices/Downloader.xpc`, `Autoupdate`, `Updater.app`), present from Task 3 on.
- Produces: `desktop/scripts/sign-app.sh <app> [identity]`; identity defaults to `-`. Exit 0 with the app verified; exit 1 with `codesign`'s message otherwise. Task 7's `release.sh` calls it once.

- [ ] **Step 1: Write the failing test script**

`desktop/scripts/sign-app-test.sh`:

```sh
#!/bin/sh
# Checks sign-app.sh on a bundle of its own: two real executables (copies
# of /usr/bin/true), signed ad-hoc with the hardened runtime, verified, and
# signed again without complaint.
set -eu

script="$(cd "$(dirname "$0")" && pwd)/sign-app.sh"
root=$(mktemp -d)
trap 'rm -rf "$root"' EXIT

app="$root/Fake.app"
mkdir -p "$app/Contents/MacOS" "$app/Contents/Resources"
cp /usr/bin/true "$app/Contents/MacOS/Fake"
cp /usr/bin/true "$app/Contents/Resources/tap"
cat > "$app/Contents/Info.plist" <<'PLIST'
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
<key>CFBundleExecutable</key><string>Fake</string>
<key>CFBundleIdentifier</key><string>io.geocod.tap.signtest</string>
<key>CFBundlePackageType</key><string>APPL</string>
<key>CFBundleShortVersionString</key><string>0.0.0</string>
<key>CFBundleVersion</key><string>1</string>
</dict></plist>
PLIST

"$script" "$app" >/dev/null || { echo "ad-hoc signing should succeed"; exit 1; }
for binary in "$app" "$app/Contents/Resources/tap"; do
	info=$(codesign -dv "$binary" 2>&1)
	echo "$info" | grep -q 'Signature=adhoc' || { echo "$binary: not ad-hoc: $info"; exit 1; }
	echo "$info" | grep -q 'runtime' || { echo "$binary: no hardened runtime: $info"; exit 1; }
done
codesign --verify --deep --strict "$app" || { echo "the bundle should verify"; exit 1; }

# Signing an already signed bundle replaces the signature.
"$script" "$app" >/dev/null || { echo "a second run should succeed"; exit 1; }

# An identity that does not exist fails with codesign's own message.
if "$script" "$app" "Developer ID Application: Nobody (NOTEAM00)" >/dev/null 2>&1; then
	echo "an unknown identity should fail"; exit 1
fi

# A path that is not an app fails before anything is signed.
if "$script" "$root/missing.app" >/dev/null 2>&1; then echo "a missing app should fail"; exit 1; fi

echo "sign-app.sh is right"
```

- [ ] **Step 2: Run it to verify it fails**

Run: `chmod +x desktop/scripts/sign-app-test.sh && desktop/scripts/sign-app-test.sh`
Expected: `ad-hoc signing should succeed` (the script does not exist).

- [ ] **Step 3: Write the script**

`desktop/scripts/sign-app.sh`:

```sh
#!/bin/sh
# Signs an app bundle inside out with the hardened runtime: the bundled tap,
# then Sparkle's own executables and the framework when the app carries
# them, then the app. Ad-hoc ("-") unless an identity is given; a real
# identity also gets a secure timestamp, which notarization requires.
# Never --deep: Apple and Sparkle both say so, since it signs nested code
# with the outer code's entitlements and in the wrong order.
set -eu

app="${1:-}"
identity="${2:--}"
[ -d "$app" ] && [ -f "$app/Contents/Info.plist" ] || { echo "sign-app.sh: $app is not an app bundle" >&2; exit 1; }

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

sign "$app"
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

Run: `make -C desktop release-build VERSION=0.0.0-dev && desktop/scripts/sign-app.sh desktop/build/DerivedData/Build/Products/Release/Tap.app`
Expected: `signed ... ad-hoc` after `codesign --verify` printed `valid on disk` and `satisfies its Designated Requirement`. (Sparkle is not in the app until Task 3; Task 3's Step 7 repeats this run with the framework present.)

- [ ] **Step 6: Commit**

```bash
git add desktop/scripts/sign-app.sh desktop/scripts/sign-app-test.sh
git commit -m "build(desktop): sign the app inside out with the hardened runtime, ad-hoc by default"
```

Mutations, applied locally and reverted: drop `--options runtime` from the ad-hoc branch (`sign-app-test.sh` fails on "no hardened runtime"); sign the app before tap (the test's `--verify --deep --strict` fails, since the outer seal predates the inner signature); drop the `[ -d "$app" ]` guard (fails on "a missing app should fail").

---

### Task 3: Sparkle in the app, and the rule that a talk is never interrupted

**Files:**
- Modify: `desktop/project.yml` (the `Sparkle` package, the `Tap` target's dependency, `SUFeedURL` and `SUPublicEDKey`)
- Create: `desktop/TapDesktopCore/Sources/TapDesktopCore/UpdateGate.swift`
- Create: `desktop/TapDesktopCore/Tests/TapDesktopCoreTests/UpdateGateTests.swift`
- Create: `desktop/Tap/App/UpdateController.swift`
- Modify: `desktop/Tap/App/AppDelegate.swift` (`updateController`, `applicationWillFinishLaunching`, `applicationDidFinishLaunching`, `checkForUpdates(_:)`, `validateMenuItem`)
- Modify: `desktop/Tap/App/MainMenu.swift` (`tapMenu`: Check for Updates… gets its action)
- Create: `desktop/TapTests/UpdaterTests.swift`

**Interfaces:**
- Consumes: `AppEnvironment.shared.isPresenting`, `updatesMayInterrupt`, `presentingDidChangeNotification` (D4); `PresentingTestCase.openDeckForPresenting`, `startPresenting`, `stopPresenting` (D4); D6's `menu.addItem(item("Check for Updates…", action: nil))` in `MainMenu.tapMenu`; Sparkle 2.10.0's `SPUStandardUpdaterController(startingUpdater:updaterDelegate:userDriverDelegate:)`, `startUpdater()`, `checkForUpdates(_:)`, `updater`, `SPUUpdater.canCheckForUpdates`, `automaticallyChecksForUpdates`, `SPUUpdaterDelegate.updater(_:mayPerform:) throws`, `updater(_:shouldProceedWithUpdate:updateCheck:) throws`, `updater(_:shouldPostponeRelaunchForUpdate:untilInvokingBlock:) -> Bool`, `SUAppcastItem.empty()`.
- Produces: `UpdateGate` (`mayCheck(isPresenting:) -> Bool`, `mayProceed(isPresenting:) -> Bool`, `shouldPostponeRelaunch(isPresenting:resume:) -> Bool`, `talkEnded()`, `postponedRelaunch: (() -> Void)?`, `static updaterMayStart(bundleVersion:isHostedByTests:hasTestDefaultsSuite:) -> Bool`, `static let presentingMessage`, `UpdateGate.PresentingError`); `UpdateController` (`gate`, `controller`, `updater`, `isStarted`, `startIfAllowed()`, `checkForUpdates(_:)`, `canCheckForUpdates`, `static isHostedByTests`, `static hasTestDefaultsSuite`, `static runsUnderTests`, `static bundleVersion`); `AppDelegate.updateController`, `AppDelegate.checkForUpdates(_:)`. Task 10's checkbox reads `updateController.updater.automaticallyChecksForUpdates`.

- [ ] **Step 1: Write the failing Core tests**

`desktop/TapDesktopCore/Tests/TapDesktopCoreTests/UpdateGateTests.swift`:

```swift
import XCTest
@testable import TapDesktopCore

final class UpdateGateTests: XCTestCase {
    func testAChecksWaitsForTheTalk() {
        let gate = UpdateGate()
        XCTAssertTrue(gate.mayCheck(isPresenting: false))
        XCTAssertFalse(gate.mayCheck(isPresenting: true), "no check during a talk, scheduled or not")
    }

    func testAFoundUpdateIsDroppedWhilePresenting() {
        let gate = UpdateGate()
        XCTAssertTrue(gate.mayProceed(isPresenting: false))
        XCTAssertFalse(gate.mayProceed(isPresenting: true), "a check that started before Play must not prompt during the talk")
        XCTAssertEqual(UpdateGate.PresentingError().localizedDescription, UpdateGate.presentingMessage)
    }

    func testAPostponedRelaunchRunsOnceWhenTheTalkEnds() {
        let gate = UpdateGate()
        var ran = 0
        XCTAssertFalse(gate.shouldPostponeRelaunch(isPresenting: false) { ran += 1 }, "no talk, no postponement")
        XCTAssertNil(gate.postponedRelaunch)
        XCTAssertEqual(ran, 0, "Sparkle relaunches itself when nothing is postponed")

        XCTAssertTrue(gate.shouldPostponeRelaunch(isPresenting: true) { ran += 1 })
        XCTAssertNotNil(gate.postponedRelaunch)
        gate.talkEnded()
        XCTAssertEqual(ran, 1)
        XCTAssertNil(gate.postponedRelaunch, "run once, then forgotten")
        gate.talkEnded()
        XCTAssertEqual(ran, 1, "a second talk ending runs nothing")
    }

    func testASecondPostponementKeepsTheNewestBlock() {
        let gate = UpdateGate()
        var order: [String] = []
        _ = gate.shouldPostponeRelaunch(isPresenting: true) { order.append("first") }
        _ = gate.shouldPostponeRelaunch(isPresenting: true) { order.append("second") }
        gate.talkEnded()
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
/// no found update is shown, and a relaunch waits for the talk to end. The
/// app's updater asks this object at each of Sparkle's decision points and
/// tells it when the last talk ends.
public final class UpdateGate {
    public static let presentingMessage = "Tap does not check for updates during a talk."

    /// What Sparkle's delegate throws to refuse a check or a found update
    /// while a talk runs. Sparkle shows the message in its own alert for a
    /// check the person asked for and logs it for a scheduled one.
    public struct PresentingError: LocalizedError {
        public init() {}
        public var errorDescription: String? { UpdateGate.presentingMessage }
    }

    /// The relaunch Sparkle was told to wait with, until the talk ends.
    public private(set) var postponedRelaunch: (() -> Void)?

    public init() {}

    /// Whether a check may start now.
    public func mayCheck(isPresenting: Bool) -> Bool {
        !isPresenting
    }

    /// Whether an update a check found may be shown now.
    public func mayProceed(isPresenting: Bool) -> Bool {
        !isPresenting
    }

    /// Returns true when the relaunch is postponed, keeping `resume` for
    /// `talkEnded`; false lets Sparkle relaunch now. Sparkle asks again for
    /// the same update, so a later call replaces the block.
    public func shouldPostponeRelaunch(isPresenting: Bool, resume: @escaping () -> Void) -> Bool {
        guard isPresenting else { return false }
        postponedRelaunch = resume
        return true
    }

    /// Runs a postponed relaunch, once.
    public func talkEnded() {
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

    func testTheFeedAndTheKeyAreInThePlist() {
        let info = Bundle.main.infoDictionary ?? [:]
        XCTAssertEqual(info["SUFeedURL"] as? String, "https://github.com/MiniCodeMonkey/tap/releases/latest/download/appcast.xml")
        XCTAssertEqual(info["SUPublicEDKey"] as? String, "Pc0PbtYL9fkyK3XnqULlpKLlH94TE4OWx4Q3n74EftU=")
        XCTAssertNil(info["SUEnableAutomaticChecks"], "Sparkle asks the person on the second launch; the app does not decide for them")
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

    func testAPostponedRelaunchRunsWhenTheTalkEnds() async throws {
        let (_, controller) = try await openDeckForPresenting()
        var relaunched = 0
        XCTAssertFalse(updates.updater(updates.updater, shouldPostponeRelaunchForUpdate: SUAppcastItem.empty()) { relaunched += 1 }, "no talk: Sparkle relaunches now")
        XCTAssertEqual(relaunched, 0)

        try await startPresenting(controller, PresentationOptions(mode: .rehearse, startSlide: 1))
        XCTAssertTrue(updates.updater(updates.updater, shouldPostponeRelaunchForUpdate: SUAppcastItem.empty()) { relaunched += 1 })
        XCTAssertEqual(relaunched, 0, "the talk runs; the app stays")
        try await stopPresenting(controller)
        try await waitUntil(timeout: 10, "the postponed relaunch") { relaunched == 1 }
        XCTAssertNil(updates.gate.postponedRelaunch)
    }
}
```

Note: `SUAppcastItem.empty()` is Sparkle's `+emptyAppcastItem`. If the Swift name differs in 2.10.0's module interface (check `SUAppcastItem.h` under the resolved package's `Headers`), use the name found there; the item's content is never read.

- [ ] **Step 6: Build to verify they fail**

Run: `make -C desktop test-build`
Expected: compile errors: no module `Sparkle`, no `UpdateController`, no `AppDelegate.checkForUpdates`.

- [ ] **Step 7: The package, the plist keys, the controller, the delegate, the menu**

In `desktop/project.yml`, under `packages`:

```yaml
  Sparkle:
    url: https://github.com/sparkle-project/Sparkle
    exactVersion: 2.10.0
```

Under the `Tap` target's `dependencies`, after `- package: TapDesktopCore`: `- package: Sparkle`. Under `info.properties`, after `NSAppTransportSecurity`:

```yaml
        # Sparkle: the feed is the newest non-pre-release GitHub release's
        # appcast, and the key checks every update's EdDSA signature. The
        # private key never leaves the person's keychain and CI's secret.
        SUFeedURL: https://github.com/MiniCodeMonkey/tap/releases/latest/download/appcast.xml
        SUPublicEDKey: Pc0PbtYL9fkyK3XnqULlpKLlH94TE4OWx4Q3n74EftU=
```

`desktop/Tap/App/UpdateController.swift`:

```swift
import AppKit
import Sparkle

/// Sparkle's standard controller and its delegate, with the app's one rule:
/// a talk is never interrupted. Every decision goes through `UpdateGate`
/// with the app's presenting state, and a relaunch a talk postponed runs
/// when the last talk ends.
@MainActor
final class UpdateController: NSObject, SPUUpdaterDelegate {
    let gate = UpdateGate()
    private(set) var controller: SPUStandardUpdaterController!
    /// Whether `startUpdater` has run. Tests and 0.0.0 builds never start it.
    private(set) var isStarted = false
    private var presentingObserver: NSObjectProtocol?

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
        isStarted && updater.canCheckForUpdates && gate.mayCheck(isPresenting: AppEnvironment.shared.isPresenting)
    }

    private func presentingChanged() {
        if !AppEnvironment.shared.isPresenting { gate.talkEnded() }
    }

    // MARK: SPUUpdaterDelegate

    func updater(_ updater: SPUUpdater, mayPerform updateCheck: SPUUpdateCheck) throws {
        guard gate.mayCheck(isPresenting: AppEnvironment.shared.isPresenting) else { throw UpdateGate.PresentingError() }
    }

    func updater(_ updater: SPUUpdater, shouldProceedWithUpdate updateItem: SUAppcastItem, updateCheck: SPUUpdateCheck) throws {
        guard gate.mayProceed(isPresenting: AppEnvironment.shared.isPresenting) else { throw UpdateGate.PresentingError() }
    }

    func updater(_ updater: SPUUpdater, shouldPostponeRelaunchForUpdate item: SUAppcastItem, untilInvokingBlock installHandler: @escaping () -> Void) -> Bool {
        gate.shouldPostponeRelaunch(isPresenting: AppEnvironment.shared.isPresenting, resume: installHandler)
    }
}
```

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

- [ ] **Step 8: Build, sign with Sparkle present, and hand the hosted tests to CI**

Run: `make -C desktop core-test && make -C desktop test-build && make -C desktop check-release-hooks && make -C desktop release-build VERSION=0.0.0-dev && desktop/scripts/sign-app.sh desktop/build/DerivedData/Build/Products/Release/Tap.app && make -C desktop check-release-app-hooks`
Expected: everything compiles (the first `xcodebuild` resolves the Sparkle package from the network); `no test-only hook` twice; `signed ... ad-hoc` with `Contents/Frameworks/Sparkle.framework` in the bundle (`ls desktop/build/DerivedData/Build/Products/Release/Tap.app/Contents/Frameworks`), and `codesign -dv .../Sparkle.framework/Versions/B/Autoupdate 2>&1 | grep -q adhoc` true. Then commit and report: CI's Desktop Tests job runs `UpdaterTests` (six tests, two of them start a rehearsal); the Desktop UI Tests and Benchmarks jobs prove the updater stays off there (no Sparkle window in their recordings; `UpdateController.runsUnderTests` is true for both).

- [ ] **Step 9: Commit**

```bash
git add desktop/project.yml desktop/TapDesktopCore/Sources/TapDesktopCore/UpdateGate.swift desktop/TapDesktopCore/Tests/TapDesktopCoreTests/UpdateGateTests.swift desktop/Tap/App/UpdateController.swift desktop/Tap/App/AppDelegate.swift desktop/Tap/App/MainMenu.swift desktop/TapTests/UpdaterTests.swift
git commit -m "feat(desktop): Sparkle updates through the standard UI, held back by every talk"
```

Mutations, each a patch in `mutations-a/`, the ones that could interrupt a talk first: in `updater(_:mayPerform:)`, drop the guard (`Test: TapTests/UpdaterTests/testNoCheckDuringATalk`; expected: fails on `XCTAssertThrowsError`); in `updater(_:shouldProceedWithUpdate:updateCheck:)`, drop the guard (`Test: .../testAnUpdateFoundDuringATalkIsDropped`; expected: fails on the throw); in `updater(_:shouldPostponeRelaunchForUpdate:untilInvokingBlock:)`, return `false` always (`Test: .../testAPostponedRelaunchRunsWhenTheTalkEnds`; expected: fails on `XCTAssertTrue`); in `presentingChanged`, drop the `talkEnded` call (expected: the same test times out on "the postponed relaunch"); in `startIfAllowed`, drop the `updaterMayStart` guard (`Test: .../testTheUpdaterNeverStartsUnderTests`; expected: fails on `isStarted`; on the runner Sparkle may also show its permission alert, which the test does not need); in `AppDelegate.validateMenuItem`, return `true` for the item (`Test: .../testCheckForUpdatesIsInTheTapMenu`; expected: fails). Core mutations, run locally and reverted: in `UpdateGate.talkEnded`, keep `postponedRelaunch` (`testAPostponedRelaunchRunsOnceWhenTheTalkEnds` fails on "run once"); in `updaterMayStart`, drop the `0.0.0` check (`testTheUpdaterStartsOnlyInAReleaseOutsideTests` fails).

---

### Task 4: The DMG

**Files:**
- Create: `desktop/scripts/make-dmg.sh`, `desktop/scripts/make-dmg-test.sh`

**Interfaces:**
- Consumes: a signed app bundle.
- Produces: `desktop/scripts/make-dmg.sh <app> <output.dmg> [volume-name]` (volume name defaults to `Tap`); the image holds `<AppName>.app` and an `Applications` symlink, compressed (`UDZO`), verified. Task 7's `release.sh` and Task 5's `write-appcast.sh` (its length) and Task 6's `render-cask.sh` (its sha256) read the file.

- [ ] **Step 1: Write the failing test script**

`desktop/scripts/make-dmg-test.sh`:

```sh
#!/bin/sh
# Checks make-dmg.sh on a bundle of its own: the image mounts without a
# Finder window, holds the app and an Applications link, and verifies.
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
[ "$(ls -A "$mount" | grep -v '^\.' | wc -l | tr -d ' ')" = "2" ] || { echo "the image holds more than the app and the link: $(ls -A "$mount")"; exit 1; }
hdiutil detach "$mount" -quiet

# The output is replaced, not appended to.
"$script" "$app" "$root/Fake-1.0.0.dmg" "Fake" >/dev/null || { echo "a second run should replace the DMG"; exit 1; }

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
# runner and on a Mac nobody is watching.
set -eu

app="${1:-}"
output="${2:-}"
volume="${3:-Tap}"
[ -d "$app" ] || { echo "make-dmg.sh: $app is not a folder" >&2; exit 1; }
[ -n "$output" ] || { echo "make-dmg.sh: no output path" >&2; exit 1; }

staging=$(mktemp -d)
trap 'rm -rf "$staging"' EXIT

ditto "$app" "$staging/$(basename "$app")"
ln -s /Applications "$staging/Applications"
rm -f "$output"
hdiutil create -volname "$volume" -srcfolder "$staging" -fs HFS+ -format UDZO -imagekey zlib-level=9 -ov -quiet "$output"
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
git commit -m "build(desktop): the DMG, the app and an Applications link, from hdiutil alone"
```

Mutations, applied locally and reverted: drop the `ln -s` (`make-dmg-test.sh` fails on "no Applications link"); use `cp -R` and add a `README.txt` to staging (fails on "holds more than"); drop `rm -f "$output"` and `-ov` (the second run fails).

---

### Task 5: The appcast and its signature

**Files:**
- Create: `desktop/scripts/fetch-sparkle-tools.sh`, `desktop/scripts/sparkle-sign.sh`, `desktop/scripts/sparkle-sign-test.sh`, `desktop/scripts/write-appcast.sh`, `desktop/scripts/write-appcast-test.sh`

**Interfaces:**
- Consumes: the app's `Contents/Info.plist` (`CFBundleShortVersionString`, `CFBundleVersion`, `LSMinimumSystemVersion`), the DMG, Sparkle's `bin/sign_update` (`--ed-key-file -` reads the key from standard input; `-p` prints the signature alone; `--verify <file> <signature>` checks one).
- Produces: `fetch-sparkle-tools.sh <folder>` (leaves `<folder>/bin/sign_update`, checks the archive's sha256 first); `sparkle-sign.sh <dmg>` (prints the base64 signature on stdout, or nothing with `skipped: Sparkle signature (SPARKLE_PRIVATE_KEY is not set)` on stderr, exit 0 either way; exit 1 when the key is set and signing fails); `write-appcast.sh <app> <dmg> <download-url> <release-url> <output.xml> [signature]`. Task 7's `release.sh` calls all three.

- [ ] **Step 1: Write the failing appcast test**

`desktop/scripts/write-appcast-test.sh`:

```sh
#!/bin/sh
# Checks write-appcast.sh: one item from the app's own plist, the DMG's
# real length, the signature when there is one and a plain word when
# there is none, and XML that parses.
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
<key>CFBundleVersion</key><string>2010003</string>
<key>LSMinimumSystemVersion</key><string>14.0</string>
</dict></plist>
PLIST
dmg="$root/Tap-2.1.0-beta.3.dmg"
head -c 12345 /dev/zero > "$dmg"
download="https://github.com/MiniCodeMonkey/tap/releases/download/v2.1.0-beta.3/Tap-2.1.0-beta.3.dmg"
release="https://github.com/MiniCodeMonkey/tap/releases/tag/v2.1.0-beta.3"

"$script" "$app" "$dmg" "$download" "$release" "$root/appcast.xml" "c2lnbmF0dXJl" >/dev/null || { echo "the appcast should be written"; exit 1; }
xmllint --noout "$root/appcast.xml" || { echo "the appcast should be XML"; exit 1; }
for expected in \
	'<sparkle:version>2010003</sparkle:version>' \
	'<sparkle:shortVersionString>2.1.0-beta.3</sparkle:shortVersionString>' \
	'<sparkle:minimumSystemVersion>14.0</sparkle:minimumSystemVersion>' \
	"<sparkle:releaseNotesLink>$release</sparkle:releaseNotesLink>" \
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

# Without a signature the item is marked unsigned, and the attribute is absent.
"$script" "$app" "$dmg" "$download" "$release" "$root/unsigned.xml" >/dev/null
xmllint --noout "$root/unsigned.xml"
if grep -q 'edSignature' "$root/unsigned.xml"; then echo "an unsigned appcast must carry no signature attribute"; exit 1; fi
grep -Fq 'unsigned: SPARKLE_PRIVATE_KEY was not set' "$root/unsigned.xml" || { echo "the unsigned appcast should say so"; exit 1; }

# A version with a character XML must escape never reaches the feed, but the writer refuses rather than corrupting it.
if "$script" "$app" "$dmg" 'https://example.com/a"b.dmg' "$release" "$root/bad.xml" >/dev/null 2>&1; then echo "a quote in a URL should fail"; exit 1; fi
if "$script" "$root/none.app" "$dmg" "$download" "$release" "$root/x.xml" >/dev/null 2>&1; then echo "a missing app should fail"; exit 1; fi

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
# never name a version the app does not carry. The signature is the DMG's
# EdDSA signature from sparkle-sign.sh; without one the item says it is
# unsigned, which Sparkle refuses, and which only a dry run produces.
set -eu

app="${1:-}"; dmg="${2:-}"; download_url="${3:-}"; release_url="${4:-}"; output="${5:-}"; signature="${6:-}"
plist="$app/Contents/Info.plist"
[ -f "$plist" ] || { echo "write-appcast.sh: $app has no Info.plist" >&2; exit 1; }
[ -f "$dmg" ] || { echo "write-appcast.sh: $dmg is missing" >&2; exit 1; }
[ -n "$output" ] || { echo "write-appcast.sh: no output path" >&2; exit 1; }
for value in "$download_url" "$release_url" "$signature"; do
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

if [ -n "$signature" ]; then
	signature_attribute=" sparkle:edSignature=\"$signature\""
	unsigned_note=""
else
	signature_attribute=""
	unsigned_note="
    <!-- unsigned: SPARKLE_PRIVATE_KEY was not set when this feed was written. Sparkle refuses this item. -->"
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
      <pubDate>$published</pubDate>
      <sparkle:version>$build_number</sparkle:version>
      <sparkle:shortVersionString>$short_version</sparkle:shortVersionString>
      <sparkle:minimumSystemVersion>$minimum_system</sparkle:minimumSystemVersion>
      <sparkle:releaseNotesLink>$release_url</sparkle:releaseNotesLink>
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
# Checks sparkle-sign.sh's skip path (no key: nothing on stdout, one line
# on stderr, exit 0, no network) and that a key it cannot use fails rather
# than printing garbage. The real path runs on CI with the secret.
set -eu

script="$(cd "$(dirname "$0")" && pwd)/sparkle-sign.sh"
root=$(mktemp -d)
trap 'rm -rf "$root"' EXIT
printf 'dmg' > "$root/Tap.dmg"

out=$(env -u SPARKLE_PRIVATE_KEY "$script" "$root/Tap.dmg" 2>"$root/err") || { echo "no key should exit 0"; exit 1; }
[ -z "$out" ] || { echo "no key should print no signature, got '$out'"; exit 1; }
grep -Fxq 'skipped: Sparkle signature (SPARKLE_PRIVATE_KEY is not set)' "$root/err" || { echo "the skip line is wrong: $(cat "$root/err")"; exit 1; }

out=$(SPARKLE_PRIVATE_KEY="" "$script" "$root/Tap.dmg" 2>/dev/null) || { echo "an empty key is no key"; exit 1; }
[ -z "$out" ] || { echo "an empty key should print nothing"; exit 1; }

if "$script" "$root/missing.dmg" >/dev/null 2>&1; then echo "a missing file should fail"; exit 1; fi

# With a key, the key goes to sign_update's standard input and nowhere
# else, and a sign_update that fails fails the script without echoing it.
# A stand-in sign_update records what it was given; no download happens
# since fetch-sparkle-tools.sh finds it in place.
mkdir -p "$root/tools/bin"
cat > "$root/tools/bin/sign_update" <<'FAKE'
#!/bin/sh
printf '%s\n' "$*" > "$(dirname "$0")/args"
cat > "$(dirname "$0")/stdin"
exit 1
FAKE
chmod +x "$root/tools/bin/sign_update"
if out=$(SPARKLE_PRIVATE_KEY="bm90LWEta2V5" SPARKLE_TOOLS="$root/tools" "$script" "$root/Tap.dmg" 2>"$root/err"); then
	echo "a failing sign_update should fail the script"; exit 1
fi
if grep -q 'bm90LWEta2V5' "$root/err" "$root/tools/bin/args"; then echo "the key reached stderr or the command line"; exit 1; fi
[ "$(cat "$root/tools/bin/stdin")" = "bm90LWEta2V5" ] || { echo "the key should reach sign_update on stdin"; exit 1; }
grep -q -- '--ed-key-file - -p' "$root/tools/bin/args" || { echo "sign_update should read the key from stdin and print the signature alone: $(cat "$root/tools/bin/args")"; exit 1; }

echo "sparkle-sign.sh is right"
```

- [ ] **Step 6: Write the tools fetcher and the signer**

`desktop/scripts/fetch-sparkle-tools.sh`:

```sh
#!/bin/sh
# Downloads Sparkle's release archive, checks it against the pinned
# checksum, and leaves bin/sign_update in the folder given. The version
# here and exactVersion in project.yml move together.
set -eu

folder="${1:-}"
[ -n "$folder" ] || { echo "fetch-sparkle-tools.sh: no folder" >&2; exit 1; }
version="2.10.0"
sha256="c2bf58aa8387266ac179357b1415d6f2635f044da8be41042af32425dae6da0c"
archive="$folder/Sparkle-$version.tar.xz"

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
# Prints the EdDSA signature of a release file for the appcast, from the
# private key in SPARKLE_PRIVATE_KEY, which goes to sign_update on its
# standard input and nowhere else. Without the key: one skip line on
# stderr, nothing on stdout, exit 0. SPARKLE_TOOLS names the folder the
# tools are (or get fetched) in; it defaults to build/sparkle-tools.
set -eu

file="${1:-}"
[ -f "$file" ] || { echo "sparkle-sign.sh: $file is missing" >&2; exit 1; }
here="$(cd "$(dirname "$0")" && pwd)"
tools="${SPARKLE_TOOLS:-$here/../build/sparkle-tools}"

if [ -z "${SPARKLE_PRIVATE_KEY:-}" ]; then
	echo "skipped: Sparkle signature (SPARKLE_PRIVATE_KEY is not set)" >&2
	exit 0
fi

"$here/fetch-sparkle-tools.sh" "$tools" >&2
signature=$(printf '%s\n' "$SPARKLE_PRIVATE_KEY" | "$tools/bin/sign_update" --ed-key-file - -p "$file")
[ -n "$signature" ] || { echo "sparkle-sign.sh: sign_update printed no signature" >&2; exit 1; }
printf '%s\n' "$SPARKLE_PRIVATE_KEY" | "$tools/bin/sign_update" --ed-key-file - --verify "$file" "$signature" >&2
printf '%s\n' "$signature"
```

- [ ] **Step 7: Run the test to verify it passes**

Run: `chmod +x desktop/scripts/fetch-sparkle-tools.sh desktop/scripts/sparkle-sign.sh desktop/scripts/sparkle-sign-test.sh && desktop/scripts/sparkle-sign-test.sh`
Expected: `sparkle-sign.sh is right`, with no network use (the stand-in `sign_update` is found in place). Then, once, to prove the fetcher and the pin: `desktop/scripts/fetch-sparkle-tools.sh desktop/build/sparkle-tools && desktop/build/sparkle-tools/bin/sign_update --help | head -1` prints `OVERVIEW: Sign or verify an update file using your signing keys.` Never run `generate_keys` without `-p`: it would write a key into the person's keychain.

- [ ] **Step 8: Commit**

```bash
git add desktop/scripts/fetch-sparkle-tools.sh desktop/scripts/sparkle-sign.sh desktop/scripts/sparkle-sign-test.sh desktop/scripts/write-appcast.sh desktop/scripts/write-appcast-test.sh
git commit -m "build(desktop): the Sparkle appcast from the app's own plist, signed when the key is present"
```

Mutations, applied locally and reverted: in `write-appcast.sh`, always write the signature attribute (`write-appcast-test.sh` fails on "must carry no signature attribute"); read the version from `$2`'s name instead of the plist (fails on `shortVersionString`); in `sparkle-sign.sh`, print the skip line to stdout (`sparkle-sign-test.sh` fails on "should print no signature"); echo `$SPARKLE_PRIVATE_KEY` in the failure message (fails on "the key reached stderr").

---

### Task 6: The Homebrew cask

**Files:**
- Create: `desktop/release/tap-desktop.rb.template`, `desktop/scripts/render-cask.sh`, `desktop/scripts/render-cask-test.sh`, `desktop/scripts/publish-cask.sh`, `desktop/scripts/publish-cask-test.sh`

**Interfaces:**
- Consumes: the DMG; `HOMEBREW_TAP_TOKEN` and `HOMEBREW_TAP_REPO` as `release.yml`'s formula step uses them.
- Produces: `render-cask.sh <version> <dmg> <output.rb>`; `publish-cask.sh <version> <cask.rb>` (skips without the token, skips a pre-release, otherwise commits `Casks/tap-desktop.rb` to the tap and prints the commit hash). Task 7's `release.sh` renders; Task 8's workflow publishes.

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
# Checks publish-cask.sh: no token skips by name, a pre-release skips by
# name, and with a token the cask lands as Casks/tap-desktop.rb in the tap,
# which here is a bare repository on disk reached through HOMEBREW_TAP_URL.
set -eu

script="$(cd "$(dirname "$0")" && pwd)/publish-cask.sh"
root=$(mktemp -d)
trap 'rm -rf "$root"' EXIT
printf 'cask "tap-desktop" do\nend\n' > "$root/tap-desktop.rb"

out=$(env -u HOMEBREW_TAP_TOKEN "$script" 2.1.0 "$root/tap-desktop.rb") || { echo "no token should exit 0"; exit 1; }
[ "$out" = "skipped: cask push (HOMEBREW_TAP_TOKEN is not set)" ] || { echo "wrong skip line: $out"; exit 1; }

out=$(HOMEBREW_TAP_TOKEN=token "$script" 2.1.0-beta.1 "$root/tap-desktop.rb") || { echo "a pre-release should exit 0"; exit 1; }
[ "$out" = "skipped: cask push (2.1.0-beta.1 is a pre-release)" ] || { echo "wrong pre-release line: $out"; exit 1; }

# A tap of the test's own: a bare repository with one commit, so the clone has a branch.
git init -q --bare "$root/tap.git"
git clone -q "$root/tap.git" "$root/seed"
( cd "$root/seed" && mkdir Formula && printf 'class Tap < Formula\nend\n' > Formula/tap.rb && git add . && git -c user.name=t -c user.email=t@t commit -q -m seed && git push -q origin HEAD )
out=$(HOMEBREW_TAP_TOKEN=token HOMEBREW_TAP_URL="$root/tap.git" "$script" 2.1.0 "$root/tap-desktop.rb") || { echo "the push should succeed: $out"; exit 1; }
echo "$out" | grep -q '^pushed Casks/tap-desktop.rb for 2.1.0 (' || { echo "no pushed line: $out"; exit 1; }
git clone -q "$root/tap.git" "$root/check"
[ "$(cat "$root/check/Casks/tap-desktop.rb")" = "$(cat "$root/tap-desktop.rb")" ] || { echo "the cask in the tap differs"; exit 1; }
[ "$(cd "$root/check" && git log -1 --format=%s)" = "Update tap-desktop to 2.1.0" ] || { echo "wrong commit message"; exit 1; }
if echo "$out" | grep -q token; then echo "the token reached stdout"; exit 1; fi

# The same version again changes nothing and says so.
out=$(HOMEBREW_TAP_TOKEN=token HOMEBREW_TAP_URL="$root/tap.git" "$script" 2.1.0 "$root/tap-desktop.rb") || { echo "a repeat should exit 0"; exit 1; }
[ "$out" = "the tap already has this cask for 2.1.0" ] || { echo "wrong repeat line: $out"; exit 1; }

echo "publish-cask.sh is right"
```

`desktop/scripts/publish-cask.sh`:

```sh
#!/bin/sh
# Pushes a rendered cask to the Homebrew tap as Casks/tap-desktop.rb, the
# way release.yml's formula step updates Formula/tap.rb. Skips, by name,
# without HOMEBREW_TAP_TOKEN and for a pre-release (Homebrew's users get
# finals; Sparkle's feed does the same). HOMEBREW_TAP_REPO names the tap
# (default MiniCodeMonkey/homebrew-tap); HOMEBREW_TAP_URL replaces the
# whole clone URL, which the test uses for a repository on disk. The token
# is in the clone URL only, never in anything printed.
set -eu

version="${1:-}"; cask="${2:-}"
[ -n "$version" ] && [ -f "$cask" ] || { echo "publish-cask.sh: usage: publish-cask.sh <version> <cask.rb>" >&2; exit 1; }

if [ -z "${HOMEBREW_TAP_TOKEN:-}" ]; then
	echo "skipped: cask push (HOMEBREW_TAP_TOKEN is not set)"
	exit 0
fi
case "$version" in
	*-*) echo "skipped: cask push ($version is a pre-release)"; exit 0 ;;
esac

repository="${HOMEBREW_TAP_REPO:-MiniCodeMonkey/homebrew-tap}"
url="${HOMEBREW_TAP_URL:-https://x-access-token:${HOMEBREW_TAP_TOKEN}@github.com/${repository}.git}"
clone=$(mktemp -d)
trap 'rm -rf "$clone"' EXIT

# git's own messages go to stderr with the token filtered out; stdout
# carries only this script's lines, which the release job's summary reads.
git clone --quiet "$url" "$clone/tap" 2>"$clone/git.log" || true
grep -v x-access-token "$clone/git.log" >&2 || true
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
git push --quiet origin HEAD 2>"$clone/git.log" || { grep -v x-access-token "$clone/git.log" >&2 || true; echo "publish-cask.sh: the push failed" >&2; exit 1; }
grep -v x-access-token "$clone/git.log" >&2 || true
hash=$(git rev-parse --short HEAD)
echo "pushed Casks/tap-desktop.rb for $version ($hash)"
```

- [ ] **Step 6: Run the test to verify it passes**

Run: `chmod +x desktop/scripts/publish-cask.sh desktop/scripts/publish-cask-test.sh && desktop/scripts/publish-cask-test.sh`
Expected: `publish-cask.sh is right`. If `git push` refuses because the seed repository's branch is `master` while the clone tracks it, the test still passes: `push origin HEAD` pushes the current branch by name.

- [ ] **Step 7: Commit**

```bash
git add desktop/release/tap-desktop.rb.template desktop/scripts/render-cask.sh desktop/scripts/render-cask-test.sh desktop/scripts/publish-cask.sh desktop/scripts/publish-cask-test.sh
git commit -m "build(desktop): the tap-desktop cask, rendered from a template and pushed to the tap for finals"
```

Mutations, applied locally and reverted: in `render-cask.sh`, substitute `__VERSION__` only (`render-cask-test.sh` fails on "a placeholder is left"); in `publish-cask.sh`, drop the pre-release case (`publish-cask-test.sh` fails on "wrong pre-release line"); print the URL after the push (fails on "the token reached stdout").

---

### Task 7: The identity, notarization, verification and the orchestrator

**Files:**
- Create: `desktop/scripts/signing-identity.sh`, `desktop/scripts/signing-identity-test.sh`, `desktop/scripts/notarize.sh`, `desktop/scripts/notarize-test.sh`, `desktop/scripts/verify-release.sh`, `desktop/scripts/release.sh`, `desktop/scripts/release-test.sh`
- Modify: `desktop/Makefile` (`RELEASE_DIR`, `release-tests`, `release`, `.PHONY`)

**Interfaces:**
- Consumes: every script of Tasks 1 to 6; `APPLE_DEVELOPER_ID_APPLICATION_P12` (base64 of the `.p12`), `APPLE_DEVELOPER_ID_APPLICATION_PASSWORD`, `APPLE_NOTARY_KEY` (the `.p8` file's text), `APPLE_NOTARY_KEY_ID`, `APPLE_NOTARY_ISSUER_ID`, `SPARKLE_PRIVATE_KEY`; `xcrun notarytool submit <file> --key <p8> --key-id <id> --issuer <uuid> --wait --timeout 30m --output-format json`, `notarytool log <id> --key ...`, `xcrun stapler staple <target>`.
- Produces: `signing-identity.sh import` (prints the identity's name, or `-` with a skip line on stderr; the keychain path goes to `$TAP_SIGNING_KEYCHAIN_FILE` when set, else `build/tap-release.keychain-db`), `signing-identity.sh remove`; `notarize.sh <file> <staple-target>` (skips by name or notarizes and staples); `verify-release.sh <app> <dmg> <appcast> <identity>`; `release.sh <version> <app> <output-dir>` writing `Tap-<version>.dmg`, `Tap-<version>.dmg.sha256`, `appcast.xml`, `Casks/tap-desktop.rb` (under `Casks/`, where `brew style` reads it as a cask), `release-summary.md`; `make -C desktop release VERSION=<v>` (output in `build/release`), `make -C desktop release-tests`. Task 8's workflows call `release` and `release-tests`.

- [ ] **Step 1: Write the failing skip-path tests**

`desktop/scripts/signing-identity-test.sh`:

```sh
#!/bin/sh
# Checks signing-identity.sh's paths that need no certificate: import with
# no secret prints "-" and a skip line; remove with no keychain is quiet;
# a secret that is not a p12 fails and leaves no keychain behind.
set -eu

script="$(cd "$(dirname "$0")" && pwd)/signing-identity.sh"
root=$(mktemp -d)
trap 'rm -rf "$root"' EXIT
export TAP_SIGNING_KEYCHAIN_FILE="$root/test.keychain-db"

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
security list-keychains | grep -q "$TAP_SIGNING_KEYCHAIN_FILE" && { echo "a failed import left the keychain in the search list"; exit 1; }

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
# prints "-" (ad-hoc) and a skip line on stderr.
# remove: deletes that keychain and takes it out of the search list.
# The .p12 exists on disk only inside a private temporary folder for the
# length of the import. TAP_SIGNING_KEYCHAIN_FILE names the keychain
# (default build/tap-release.keychain-db next to the scripts' parent).
set -eu

here="$(cd "$(dirname "$0")" && pwd)"
keychain="${TAP_SIGNING_KEYCHAIN_FILE:-$here/../build/tap-release.keychain-db}"
command="${1:-}"

remove_keychain() {
	# The search list without ours; untouched when ours is not in it.
	if security list-keychains -d user | grep -q "$keychain"; then
		remaining=$(security list-keychains -d user | tr -d '" ' | grep -v "$keychain" || true)
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
		trap 'rm -rf "$private"' EXIT
		umask 077
		printf '%s' "$APPLE_DEVELOPER_ID_APPLICATION_P12" | base64 -d > "$private/certificate.p12" 2>/dev/null || { echo "signing-identity.sh: the certificate secret is not base64" >&2; exit 1; }
		keychain_password=$(head -c 24 /dev/urandom | base64)
		mkdir -p "$(dirname "$keychain")"
		remove_keychain
		security create-keychain -p "$keychain_password" "$keychain"
		security set-keychain-settings -lut 21600 "$keychain"
		security unlock-keychain -p "$keychain_password" "$keychain"
		if ! security import "$private/certificate.p12" -k "$keychain" -P "$APPLE_DEVELOPER_ID_APPLICATION_PASSWORD" -T /usr/bin/codesign -T /usr/bin/security >/dev/null 2>&1; then
			remove_keychain
			echo "signing-identity.sh: the certificate did not import (wrong password, or not a .p12)" >&2
			exit 1
		fi
		security set-key-partition-list -S apple-tool:,apple:,codesign: -s -k "$keychain_password" "$keychain" >/dev/null
		existing=$(security list-keychains -d user | tr -d '" ')
		# shellcheck disable=SC2086
		security list-keychains -d user -s "$keychain" $existing
		identity=$(security find-identity -v -p codesigning "$keychain" | sed -n 's/.*"\(Developer ID Application: [^"]*\)".*/\1/p' | head -n 1)
		if [ -z "$identity" ]; then
			remove_keychain
			echo "signing-identity.sh: no Developer ID Application identity in the certificate" >&2
			exit 1
		fi
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
Expected: `signing-identity.sh is right`, `notarize.sh is right`. Then `security list-keychains -d user` shows the person's keychains as before (the test's keychain was removed).

- [ ] **Step 5: Write the failing orchestrator test**

`desktop/scripts/release-test.sh`:

```sh
#!/bin/sh
# Checks release.sh with no secret at all against a bundle of its own: the
# DMG, its checksum, the appcast, the cask and the summary are written,
# every secret-bearing step is skipped by name, and nothing was notarized
# or pushed. This is the dry run, the same path CI takes without secrets.
set -eu

here="$(cd "$(dirname "$0")" && pwd)"
script="$here/release.sh"
root=$(mktemp -d)
trap 'rm -rf "$root"' EXIT
export TAP_SIGNING_KEYCHAIN_FILE="$root/test.keychain-db"

app="$root/Tap.app"
mkdir -p "$app/Contents/MacOS" "$app/Contents/Resources"
cp /usr/bin/true "$app/Contents/MacOS/Tap"
cp /usr/bin/true "$app/Contents/Resources/tap"
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
<key>SUFeedURL</key><string>https://github.com/MiniCodeMonkey/tap/releases/latest/download/appcast.xml</string>
<key>SUPublicEDKey</key><string>Pc0PbtYL9fkyK3XnqULlpKLlH94TE4OWx4Q3n74EftU=</string>
</dict></plist>
PLIST

env -u APPLE_DEVELOPER_ID_APPLICATION_P12 -u APPLE_NOTARY_KEY -u SPARKLE_PRIVATE_KEY -u HOMEBREW_TAP_TOKEN \
	"$script" 0.0.0-test "$app" "$root/out" > "$root/log" 2>&1 || { echo "the dry run should succeed"; cat "$root/log"; exit 1; }

for file in Tap-0.0.0-test.dmg Tap-0.0.0-test.dmg.sha256 appcast.xml Casks/tap-desktop.rb release-summary.md; do
	[ -f "$root/out/$file" ] || { echo "missing $file"; cat "$root/log"; exit 1; }
done
grep -Fq "Tap-0.0.0-test.dmg" "$root/out/Tap-0.0.0-test.dmg.sha256" || { echo "the checksum names the DMG"; exit 1; }
[ "$(cut -d ' ' -f 1 "$root/out/Tap-0.0.0-test.dmg.sha256")" = "$(shasum -a 256 "$root/out/Tap-0.0.0-test.dmg" | cut -d ' ' -f 1)" ] || { echo "the checksum is wrong"; exit 1; }
grep -Fq 'unsigned: SPARKLE_PRIVATE_KEY was not set' "$root/out/appcast.xml" || { echo "the appcast should say it is unsigned"; exit 1; }
grep -Fq 'version "0.0.0-test"' "$root/out/Casks/tap-desktop.rb" || { echo "the cask was not rendered"; exit 1; }
codesign -dv "$app" 2>&1 | grep -q 'Signature=adhoc' || { echo "the app should be signed ad-hoc"; exit 1; }

for line in \
	'skipped: Developer ID signing (APPLE_DEVELOPER_ID_APPLICATION_P12 is not set)' \
	'skipped: notarization of Tap-0.0.0-test.zip (no Developer ID identity)' \
	'skipped: DMG signature (no Developer ID identity)' \
	'skipped: notarization of Tap-0.0.0-test.dmg (no Developer ID identity)' \
	'skipped: Sparkle signature (SPARKLE_PRIVATE_KEY is not set)' \
	'skipped: Gatekeeper assessment (no Developer ID identity)'; do
	grep -Fq "$line" "$root/out/release-summary.md" || { echo "the summary lacks: $line"; cat "$root/out/release-summary.md"; exit 1; }
done
grep -Fq 'done: signed Tap.app ad-hoc' "$root/out/release-summary.md" || { echo "the summary lacks the signing line"; exit 1; }
grep -Fq 'done: wrote Tap-0.0.0-test.dmg' "$root/out/release-summary.md" || { echo "the summary lacks the DMG line"; exit 1; }
grep -Fq 'done: wrote appcast.xml (unsigned)' "$root/out/release-summary.md" || { echo "the summary lacks the appcast line"; exit 1; }
grep -Fq 'done: rendered tap-desktop.rb (not pushed by this script)' "$root/out/release-summary.md" || { echo "the summary lacks the cask line"; exit 1; }
[ ! -f "$TAP_SIGNING_KEYCHAIN_FILE" ] || { echo "the keychain should be gone"; exit 1; }

# A second run replaces the output folder's files rather than failing on them.
env -u APPLE_DEVELOPER_ID_APPLICATION_P12 -u APPLE_NOTARY_KEY -u SPARKLE_PRIVATE_KEY -u HOMEBREW_TAP_TOKEN \
	"$script" 0.0.0-test "$app" "$root/out" >/dev/null 2>&1 || { echo "a second run should succeed"; exit 1; }

# The version must match the app's own.
if "$script" 9.9.9 "$app" "$root/out2" >/dev/null 2>&1; then echo "a version the app does not carry should fail"; exit 1; fi

echo "release.sh is right"
```

- [ ] **Step 6: Write the verifier and the orchestrator**

`desktop/scripts/verify-release.sh`:

```sh
#!/bin/sh
# The checks on a finished release: the app verifies with the hardened
# runtime and the two Sparkle keys and carries the version it was built
# with, the DMG verifies, the appcast parses and names the same build, and
# with a real identity Gatekeeper accepts the DMG.
set -eu

app="${1:-}"; dmg="${2:-}"; appcast="${3:-}"; identity="${4:--}"; version="${5:-}"
plist="$app/Contents/Info.plist"
fail() { echo "verify-release.sh: $1" >&2; exit 1; }

codesign --verify --deep --strict "$app" || fail "$app does not verify"
codesign -dv "$app" 2>&1 | grep -q 'runtime' || fail "$app has no hardened runtime"
read_plist() { /usr/libexec/PlistBuddy -c "Print :$1" "$plist"; }
[ "$(read_plist SUFeedURL)" = "https://github.com/MiniCodeMonkey/tap/releases/latest/download/appcast.xml" ] || fail "SUFeedURL is wrong"
[ "$(read_plist SUPublicEDKey)" = "Pc0PbtYL9fkyK3XnqULlpKLlH94TE4OWx4Q3n74EftU=" ] || fail "SUPublicEDKey is wrong"
[ -z "$version" ] || [ "$(read_plist CFBundleShortVersionString)" = "$version" ] || fail "the app is $(read_plist CFBundleShortVersionString), not $version"
build_number=$(read_plist CFBundleVersion)
[ -n "$build_number" ] || fail "no CFBundleVersion"
[ "$(read_plist LSMinimumSystemVersion)" = "14.0" ] || fail "LSMinimumSystemVersion is not 14.0"

hdiutil verify -quiet "$dmg" || fail "$dmg does not verify"
xmllint --noout "$appcast" || fail "$appcast is not XML"
grep -Fq "<sparkle:version>$build_number</sparkle:version>" "$appcast" || fail "the appcast names another build than $build_number"
grep -Fq "length=\"$(stat -f%z "$dmg")\"" "$appcast" || fail "the appcast's length is not the DMG's"

if [ "$identity" != "-" ]; then
	codesign --verify --strict "$dmg" || fail "$dmg is not signed"
	spctl --assess --type open --context context:primary-signature -v "$dmg" || fail "Gatekeeper refuses $dmg"
	xcrun stapler validate "$app" >/dev/null || fail "$app has no stapled ticket"
	xcrun stapler validate "$dmg" >/dev/null || fail "$dmg has no stapled ticket"
fi
echo "verified $app, $dmg and $appcast"
```

`desktop/scripts/release.sh`:

```sh
#!/bin/sh
# Turns a built Release app into a release: signs it, notarizes the app and
# the DMG, writes the DMG with its checksum, the Sparkle appcast and the
# cask, verifies everything, and records every step in
# release-summary.md as done or skipped. Every step that needs a secret
# skips itself, by name, when the secret is absent, so this runs with none
# (a dry run) and with all of them (the release job) along the same path.
# Nothing here prints a secret; the scripts it calls own that rule.
set -eu

version="${1:-}"; app="${2:-}"; out="${3:-}"
[ -n "$version" ] && [ -d "$app" ] && [ -n "$out" ] || { echo "release.sh: usage: release.sh <version> <Tap.app> <output-dir>" >&2; exit 1; }
here="$(cd "$(dirname "$0")" && pwd)"
built=$(/usr/libexec/PlistBuddy -c 'Print :CFBundleShortVersionString' "$app/Contents/Info.plist")
[ "$built" = "$version" ] || { echo "release.sh: the app is $built, not $version; build it with VERSION=$version" >&2; exit 1; }

mkdir -p "$out"
summary="$out/release-summary.md"
: > "$summary"
note() { echo "$1"; echo "- $1" >> "$summary"; }

identity=$("$here/signing-identity.sh" import 2>"$out/identity.log")
trap '"$here/signing-identity.sh" remove; rm -f "$out/identity.log"' EXIT
if [ "$identity" = "-" ]; then
	note "$(cat "$out/identity.log")"
else
	note "done: found the identity $identity"
fi

"$here/sign-app.sh" "$app" "$identity" >/dev/null
if [ "$identity" = "-" ]; then note "done: signed $(basename "$app") ad-hoc"; else note "done: signed $(basename "$app") with $identity"; fi

dmg="$out/Tap-$version.dmg"
zip="$out/Tap-$version.zip"
if [ "$identity" = "-" ]; then
	note "skipped: notarization of $(basename "$zip") (no Developer ID identity)"
else
	rm -f "$zip"
	ditto -c -k --keepParent "$app" "$zip"
	note "$("$here/notarize.sh" "$zip" "$app")"
	rm -f "$zip"
fi

"$here/make-dmg.sh" "$app" "$dmg" Tap >/dev/null
note "done: wrote $(basename "$dmg")"
if [ "$identity" = "-" ]; then
	note "skipped: DMG signature (no Developer ID identity)"
	note "skipped: notarization of $(basename "$dmg") (no Developer ID identity)"
else
	codesign --force --sign "$identity" --timestamp "$dmg"
	note "done: signed $(basename "$dmg")"
	note "$("$here/notarize.sh" "$dmg" "$dmg")"
fi
( cd "$out" && shasum -a 256 "$(basename "$dmg")" > "$(basename "$dmg").sha256" )
note "done: wrote $(basename "$dmg").sha256"

signature=$("$here/sparkle-sign.sh" "$dmg" 2>"$out/sparkle.log") || { cat "$out/sparkle.log" >&2; exit 1; }
if [ -z "$signature" ]; then
	note "$(grep '^skipped:' "$out/sparkle.log")"
else
	note "done: signed $(basename "$dmg") for Sparkle"
fi
rm -f "$out/sparkle.log"
download_url="https://github.com/MiniCodeMonkey/tap/releases/download/v$version/Tap-$version.dmg"
release_url="https://github.com/MiniCodeMonkey/tap/releases/tag/v$version"
"$here/write-appcast.sh" "$app" "$dmg" "$download_url" "$release_url" "$out/appcast.xml" "$signature" >/dev/null
if [ -z "$signature" ]; then note "done: wrote appcast.xml (unsigned)"; else note "done: wrote appcast.xml (signed)"; fi

"$here/render-cask.sh" "$version" "$dmg" "$out/Casks/tap-desktop.rb" >/dev/null
note "done: rendered tap-desktop.rb (not pushed by this script)"

"$here/verify-release.sh" "$app" "$dmg" "$out/appcast.xml" "$identity" "$version" >/dev/null
if [ "$identity" = "-" ]; then note "skipped: Gatekeeper assessment (no Developer ID identity)"; else note "done: Gatekeeper accepts $(basename "$dmg")"; fi
note "done: verified the app, the DMG and the appcast"

echo
echo "release $version in $out:"
cat "$summary"
```

In `desktop/Makefile`, add `RELEASE_DIR = build/release` after `RELEASE_BINARY`, `release-tests release` to `.PHONY`, and after `check-release-app-hooks`:

```make
# Every release script's own test; none needs a build, a secret or a window.
release-tests:
	@for test in scripts/build-number-test.sh scripts/sign-app-test.sh scripts/make-dmg-test.sh scripts/write-appcast-test.sh scripts/sparkle-sign-test.sh scripts/render-cask-test.sh scripts/publish-cask-test.sh scripts/signing-identity-test.sh scripts/notarize-test.sh scripts/release-test.sh; do \
		echo "== $$test"; sh "$$test" || exit 1; \
	done

# The release: the Release build, the hook check, then release.sh, which
# signs, notarizes, writes the DMG, the appcast and the cask into
# build/release, skipping what has no secret. The same target is the local
# dry run (VERSION=0.0.0-dev, no secrets) and the release job's step.
release: check-release-app-hooks
	./scripts/release.sh $(VERSION) $(RELEASE_APP) $(RELEASE_DIR)
```

- [ ] **Step 7: Run the tests and the local dry run**

Run: `chmod +x desktop/scripts/verify-release.sh desktop/scripts/release.sh desktop/scripts/release-test.sh && make -C desktop release-tests`
Expected: ten `==` headers, each followed by its `... is right` line. Then the real dry run:

Run: `make -C desktop release VERSION=0.0.0-dev`
Expected: the build, `no test-only hook in the Release build`, then `release 0.0.0-dev in build/release:` with the summary: the identity skipped, `signed Tap.app ad-hoc`, the app's notarization skipped, `wrote Tap-0.0.0-dev.dmg`, DMG signature and notarization skipped, the checksum, the Sparkle signature skipped, `wrote appcast.xml (unsigned)`, `rendered tap-desktop.rb (not pushed by this script)`, Gatekeeper skipped, verified. `ls desktop/build/release` shows the five files. Nothing was launched or mounted with a window.

- [ ] **Step 8: Commit**

```bash
git add desktop/scripts/signing-identity.sh desktop/scripts/signing-identity-test.sh desktop/scripts/notarize.sh desktop/scripts/notarize-test.sh desktop/scripts/verify-release.sh desktop/scripts/release.sh desktop/scripts/release-test.sh desktop/Makefile
git commit -m "build(desktop): release.sh runs the release end to end, skipping by name every step whose secret is absent"
```

Mutations, applied locally and reverted: in `release.sh`, skip `verify-release.sh` (`release-test.sh` still passes: file a survivor note, the verifier's own mutations are next); in `verify-release.sh`, drop the `SUPublicEDKey` check and run the dry run against an app whose plist lacks it (a hand-edited copy; expected: passes, which is the mutation caught only by the real key check: revert); in `release.sh`, drop the `built = version` guard (`release-test.sh` fails on "a version the app does not carry"); in `notarize.sh`, print `$APPLE_NOTARY_KEY` in the skip line (`notarize-test.sh` fails on "wrong skip line"); in `signing-identity.sh`, leave the keychain after a failed import (`signing-identity-test.sh` fails on "a failed import should remove its keychain").

---

### Task 8: The release job and the dry-run job

**Files:**
- Modify: `.github/workflows/release.yml` (the `desktop` job)
- Modify: `.github/workflows/ci.yml` (the `release-dry-run` job)

**Interfaces:**
- Consumes: `make -C desktop release`, `make -C desktop release-tests`, `desktop/scripts/publish-cask.sh` (Task 7, 6); the `release` job's tag `v${version}` and its GitHub release; the secrets by name; `vars.HOMEBREW_TAP_REPO`.
- Produces: on a release, `Tap-<version>.dmg`, `Tap-<version>.dmg.sha256` and `appcast.xml` as assets of the release, the cask pushed for a final, and the summary in the job's step summary; on every pull request, a `desktop-release-dry-run` artifact with the DMG and the summary.

- [ ] **Step 1: The `desktop` job in `release.yml`**

After the `release` job (same indentation as `release:`), add:

```yaml
  # Tap Desktop: built on a macOS runner from the tag the release job
  # pushed, after the GitHub release exists so the DMG and the appcast can
  # join it. Every step that needs a secret is skipped, by name, when the
  # secret is absent (see desktop/scripts/release.sh), so this job runs
  # before the Apple and Sparkle credentials exist and produces an ad-hoc
  # signed DMG then; with the secrets it produces the notarized release.
  desktop:
    name: Desktop release
    needs: release
    runs-on: macos-15
    timeout-minutes: 90
    steps:
      - name: Checkout the tag
        uses: actions/checkout@v7
        with:
          ref: v${{ github.event.inputs.version }}

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

      # The secrets reach the scripts through the environment alone. An
      # unset secret is an empty string here, which each script treats as
      # absent and reports as skipped.
      - name: Build, sign, notarize and package
        env:
          APPLE_DEVELOPER_ID_APPLICATION_P12: ${{ secrets.APPLE_DEVELOPER_ID_APPLICATION_P12 }}
          APPLE_DEVELOPER_ID_APPLICATION_PASSWORD: ${{ secrets.APPLE_DEVELOPER_ID_APPLICATION_PASSWORD }}
          APPLE_NOTARY_KEY: ${{ secrets.APPLE_NOTARY_KEY }}
          APPLE_NOTARY_KEY_ID: ${{ secrets.APPLE_NOTARY_KEY_ID }}
          APPLE_NOTARY_ISSUER_ID: ${{ secrets.APPLE_NOTARY_ISSUER_ID }}
          SPARKLE_PRIVATE_KEY: ${{ secrets.SPARKLE_PRIVATE_KEY }}
        run: make -C desktop release VERSION="${{ github.event.inputs.version }}"

      - name: Summarize
        run: |
          echo "## Tap Desktop ${{ github.event.inputs.version }}" >> "$GITHUB_STEP_SUMMARY"
          cat desktop/build/release/release-summary.md >> "$GITHUB_STEP_SUMMARY"

      - name: Upload the DMG and the appcast to the release
        env:
          GH_TOKEN: ${{ secrets.GITHUB_TOKEN }}
        run: |
          cd desktop/build/release
          gh release upload "v${{ github.event.inputs.version }}" "Tap-${{ github.event.inputs.version }}.dmg" "Tap-${{ github.event.inputs.version }}.dmg.sha256" appcast.xml --clobber

      # Finals only, as the formula step above: Homebrew's users get finals,
      # and the appcast's "latest" URL does the same for Sparkle.
      - name: Update the Homebrew cask
        env:
          HOMEBREW_TAP_TOKEN: ${{ secrets.HOMEBREW_TAP_TOKEN }}
          HOMEBREW_TAP_REPO: ${{ vars.HOMEBREW_TAP_REPO || 'MiniCodeMonkey/homebrew-tap' }}
        run: |
          desktop/scripts/publish-cask.sh "${{ github.event.inputs.version }}" desktop/build/release/Casks/tap-desktop.rb | tee -a "$GITHUB_STEP_SUMMARY"
```

`permissions: contents: write` at the top of the file already covers `gh release upload`. `needs: release` means the tag and the release exist before the checkout.

- [ ] **Step 2: The `release-dry-run` job in `ci.yml`**

After `bench-desktop`:

```yaml
  # The release pipeline with no secrets, on every pull request: the Release
  # build with the hardened runtime, the hook check, ad-hoc signing, the
  # DMG, the unsigned appcast and the rendered cask, then brew style on the
  # cask. release.sh skips notarization, the Sparkle signature and the cask
  # push by name; the summary lists them. The DMG is kept for a week so a
  # reviewer can inspect it. This is how the pipeline is proven before the
  # Developer ID certificate, the notary key and the Sparkle key exist.
  release-dry-run:
    name: Desktop Release Dry Run
    runs-on: macos-15
    timeout-minutes: 60
    steps:
      - name: Checkout
        uses: actions/checkout@v7

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
        run: |
          echo "## Desktop release dry run" >> "$GITHUB_STEP_SUMMARY"
          cat desktop/build/release/release-summary.md >> "$GITHUB_STEP_SUMMARY"
          grep -c '^- skipped:' desktop/build/release/release-summary.md | xargs -I{} echo "{} steps skipped, as expected with no secrets" >> "$GITHUB_STEP_SUMMARY"

      - name: Keep the DMG
        uses: actions/upload-artifact@v7
        with:
          name: desktop-release-dry-run
          path: |
            desktop/build/release/Tap-0.0.0-ci.dmg
            desktop/build/release/Tap-0.0.0-ci.dmg.sha256
            desktop/build/release/appcast.xml
            desktop/build/release/Casks/tap-desktop.rb
            desktop/build/release/release-summary.md
          retention-days: 7
```

- [ ] **Step 3: Check the workflows parse, and commit**

Run: `for f in .github/workflows/release.yml .github/workflows/ci.yml; do ruby -ryaml -e 'YAML.load_file(ARGV[0]); puts "#{ARGV[0]} parses"' "$f"; done && grep -n 'secrets\.' .github/workflows/release.yml`
Expected: both parse; every `secrets.` line the grep prints is an `env:` value (`NAME: ${{ secrets.NAME }}`), the checkout's `token:` or `GH_TOKEN`; none is inside a `run:` block. Then:

```bash
git add .github/workflows/release.yml .github/workflows/ci.yml
git commit -m "ci: a desktop release job after the CLI release, and a no-secrets dry run on every pull request"
```

The controller pushes and reads CI: the Desktop Release Dry Run job is green, its summary lists six `skipped:` lines and the `done:` lines, `brew style` passes, and the artifact holds the five files. The release job itself is exercised by the person's first release after merge (or by a `workflow_dispatch` of a pre-release such as `2.0.0-beta.6` from `main`, which uploads an ad-hoc DMG and skips the cask).

---

### Task 9: Documentation, the changelog and the roadmap

**Files:**
- Modify: `desktop/README.md` (a "Release" section), `README.md` (install Tap Desktop), `docs/getting-started.md` (the same), `CONTRIBUTING.md` (the Release Process section), `CHANGELOG.md` (`[Unreleased]`), `docs/superpowers/plans/2026-09-22-tap-desktop-roadmap.md` (D7's row)

- [ ] **Step 1: The desktop README**

After the "Test" section of `desktop/README.md`, add:

````markdown
## Release

```sh
make -C desktop release-tests                # every release script's own test, no build
make -C desktop release VERSION=0.0.0-dev    # the dry run: builds, signs ad-hoc, writes the DMG
```

`make -C desktop release` builds the Release configuration with the version
and a build number (`scripts/build-number.sh`), checks that no test hook is
in the binary, then runs `scripts/release.sh`, which signs the app inside
out with the hardened runtime, notarizes the app and the DMG, writes
`build/release/Tap-<version>.dmg`, its `.sha256`, the Sparkle `appcast.xml`
and the `Casks/tap-desktop.rb` cask, verifies them, and lists every step in
`build/release/release-summary.md`. Each step that needs a secret is skipped,
by name, when the secret is absent, so the dry run needs none and launches
nothing. The release job (`.github/workflows/release.yml`, job `desktop`)
runs the same target with the secrets and uploads the DMG and the appcast
to the GitHub release; the cask goes to the Homebrew tap for a final
version. CI's Desktop Release Dry Run job runs the dry run on every pull
request and keeps the DMG as an artifact.

Updates come through Sparkle's standard UI. The feed is the newest
non-pre-release release's `appcast.xml`
(`https://github.com/MiniCodeMonkey/tap/releases/latest/download/appcast.xml`);
`SUPublicEDKey` in `project.yml` checks each update's signature. A talk is
never interrupted: `UpdateController` refuses a check, drops a found update
and postpones a relaunch while `AppEnvironment.isPresenting`, and the
postponed relaunch runs when the last talk ends. The updater never starts
under tests or in a `0.0.0` build.

The secrets the release job reads, all optional, each skipping its step
when absent: `APPLE_DEVELOPER_ID_APPLICATION_P12` (the Developer ID
Application certificate with its key, `base64 -i certificate.p12`),
`APPLE_DEVELOPER_ID_APPLICATION_PASSWORD`, `APPLE_NOTARY_KEY` (an App Store
Connect API key's `.p8` file, as text), `APPLE_NOTARY_KEY_ID`,
`APPLE_NOTARY_ISSUER_ID`, `SPARKLE_PRIVATE_KEY` (`generate_keys --account
tap-desktop -x key.txt`, the file's text) and `HOMEBREW_TAP_TOKEN`.
````

- [ ] **Step 2: Install docs, contributing, changelog, roadmap**

In `README.md`, after the "Using Homebrew (macOS/Linux)" block:

````markdown
### Tap Desktop (macOS 14 or later)

The native app, with tap built in:

```bash
brew install --cask MiniCodeMonkey/tap/tap-desktop
```

Or download `Tap-<version>.dmg` from the [releases page](https://github.com/MiniCodeMonkey/tap/releases). The app updates itself through Tap > Check for Updates…
````

In `docs/getting-started.md`, the same block after the Homebrew section, headed `### Tap Desktop (macOS)`.

In `CONTRIBUTING.md`, at the end of "Release Process" (after the list of what the workflow does), add:

```markdown
The workflow's second job, `desktop`, builds Tap Desktop on a macOS runner
from the tag, signs and notarizes it, and adds `Tap-<version>.dmg`, its
checksum and Sparkle's `appcast.xml` to the same release; for a final
version it also updates the `tap-desktop` cask in the Homebrew tap. Each
signing step is skipped, by name, when its secret is absent, and the job's
summary says which. `desktop/README.md` lists the secrets and how to run
the same pipeline locally with none of them.
```

In `CHANGELOG.md`, under `## [Unreleased]` / `### Added`, a bullet at the end:

```markdown
- **Tap Desktop ships as a macOS app** - A signed and notarized `Tap-<version>.dmg` on every release, `brew install --cask MiniCodeMonkey/tap/tap-desktop`, and updates through Sparkle from Tap > Check for Updates… An update never interrupts a talk: no check runs and no restart happens while you present, and a postponed restart waits for the talk to end.
```

In the roadmap, D7's row becomes `| D7 | \`2026-09-27-desktop-release.md\` | Desktop milestone 7 | D6 | written; runs with no secrets, signs and notarizes once they exist |`.

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
- Produces: `UpdateController.automaticChecks: Bool` (get and set), `UpdateController.automaticChecksStore: (read: () -> Bool, write: (Bool) -> Void)` (Sparkle's property by default; a test replaces it so no test writes the app's real defaults); `GeneralSettingsViewController.automaticUpdatesCheckbox`.

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

Mutations, each a patch in `mutations-c/`: in `changed(_:)`, drop the checkbox case (`Test: TapTests/SettingsUpdatesTests/testTheUpdatesCardMirrorsSparklesSetting`; expected: fails on "the click reaches the store"); in `refresh()`, set `.on` always (expected: passes; into `survivors-c/` with the note that the test starts from `true`; the reviewer may add a `stored = false` refresh assertion); in `automaticChecksStore`'s default `write`, drop the assignment (into `survivors-c/`: the test replaces the store; Sparkle's own property is not the app's to test).

---

## Final check

- [ ] `make -C desktop release-tests` green; `make -C desktop core-test` green; `make -C desktop test-build` compiles; `make -C desktop check-release-hooks` and `make -C desktop check-release-app-hooks` both print their "no test-only hook" line; `make -C desktop release VERSION=0.0.0-dev` writes the five files with six `skipped:` lines; `make -C desktop check-scenarios` unchanged and green (no new rows).
- [ ] `grep -rn 'set -x' desktop/scripts` finds nothing. `grep -rn 'SPARKLE_PRIVATE_KEY\|APPLE_NOTARY_KEY\b\|APPLE_DEVELOPER_ID_APPLICATION_P12' desktop/scripts .github` finds only reads into `sign_update`'s stdin, the `.p8` and `.p12` temporary files, the `for secret` loop, the skip lines and the workflow's `env:` values; nothing echoes one.
- [ ] `grep -rn 'evaluateJavaScript\|callAsyncJavaScript' desktop/Tap` is unchanged from D6; `grep -rn 'NSAlert\|runModal' desktop/Tap/App/UpdateController.swift` finds nothing (Sparkle's alerts are Sparkle's).
- [ ] `grep -rn 'updatesMayInterrupt\|isPresenting' desktop/Tap/App/UpdateController.swift` shows every delegate method reads the presenting state; `FocusHintTests.testNothingInterruptsTheTalk` is unchanged.
- [ ] `grep -n 'exactVersion: 2.10.0' desktop/project.yml` and `grep -n 'version="2.10.0"' desktop/scripts/fetch-sparkle-tools.sh` agree.
- [ ] No em dash in any file this plan touched: `git diff main --name-only | xargs grep -ln "$(printf '\342\200\224')"` prints no file.
- [ ] The controller pushes and reads CI: Go Tests, Frontend Tests, E2E, Theme Checks, Desktop Tests (`UpdaterTests` six green, `SettingsUpdatesTests` one green), Desktop UI Tests, Desktop Benchmarks (no Sparkle window in a recording), Desktop Release Dry Run (artifact present, `brew style` green). The mutation branches `mutations/d7-batch-a` and `d7-batch-c` hold the patches; every one with a named killing test is killed.
- [ ] The final review reads `release-summary.md` from the dry-run artifact and the `desktop` job's design against "The release, end to end" above.

## Steps that wait for a mockup

| Step | Board | What it builds |
|---|---|---|
| Task 10, whole | SettingsUpdates | The Updates card in Settings > General with the "Check for updates automatically" checkbox |

Everything else uses Sparkle's standard UI (its permission prompt, update alert, release notes view, progress and relaunch windows), the menu item D6's MenusFile board already draws (Tap > Check for Updates…), and a plain DMG. Two more screens exist only if the person wants them, each needing a mockup first and none built by this plan: a DMG window with background art and icon positions (needs Finder scripting on the runner), and a custom release notes view in the update window (Sparkle shows the GitHub release page today).

## Open questions

Product decisions this plan makes that the spec leaves open. Each line is the default the plan implements, the alternative, and what it costs if the person wants it otherwise.

1. **Where the feed lives.** Default: `https://github.com/MiniCodeMonkey/tap/releases/latest/download/appcast.xml`, the newest non-pre-release release's asset, no hosting added, so pre-release users get no Sparkle updates until the next final. Alternative: host `appcast.xml` (and a `beta` channel feed) on tap.sh through the publish project. Cost if wrong: one plist value and a copy step in the job.
2. **Automatic checks.** Default: Sparkle's standard permission prompt on the second launch (`SUEnableAutomaticChecks` unset), and Task 10's checkbox to change the answer later. Alternative: `SUEnableAutomaticChecks: true`, no prompt. Cost if wrong: one plist key.
3. **Release notes in the update window.** Default: `releaseNotesLink` to the GitHub release page. Alternative: embed `release_notes.md` as HTML (needs a markdown converter on the runner). Cost if wrong: one script step.
4. **Two notarizations.** Default: the app (zipped, then stapled) and then the DMG, so a dragged app launches offline with its ticket. Alternative: the DMG only, one submission, an online check at the app's first launch. Cost if wrong: one branch in `release.sh`.
5. **The build number scheme.** Default: `MAJOR*1000000 + MINOR*10000 + PATCH*100 + slot`, the slot being the pre-release number (1 to 98) or 99 for a final; the label (`beta`, `rc`) does not order, so one label per version. Alternative: `CFBundleVersion` equal to the version string and Sparkle's comparator. Cost if wrong: one script and its test.
6. **A plain DMG.** Default: the app and an Applications link, no art. Alternative: a background image and icon layout after a mockup. Cost if wrong: a mockup and Finder scripting on the runner.
7. **Check for Updates… during a talk.** Default: the item is disabled by validation, and the delegate refuses anyway with "Tap does not check for updates during a talk." in Sparkle's own alert if a check slips through. Alternative: disabled only. Cost if wrong: one guard.
8. **The dry run on every pull request.** Default: `release-dry-run` runs with the other desktop jobs (about ten minutes of a macOS runner). Alternative: only on `main` and `workflow_dispatch`. Cost if wrong: a `paths` filter or an `if`.
9. **Hardened runtime in Release only.** Default: Debug and Benchmark stay off, since the hosted tests inject into them. Cost if wrong: two lines in `project.yml`.
10. **Sparkle pinned at 2.10.0** in both the package and the tools, bumped by hand. Cost if wrong: two lines.
11. **The cask in the existing tap** (`MiniCodeMonkey/homebrew-tap`, `Casks/tap-desktop.rb`, the same token). Alternative: a separate cask repository. Cost if wrong: `HOMEBREW_TAP_REPO`'s value.
12. **The pre-release regex allows any label**, and the build number takes the last dotted number. The person uses one label per version (`2.0.0-beta.6`, then `2.0.0`).
13. **The person's runs.** The local dry run (`make -C desktop release VERSION=0.0.0-dev`) is the one release step an agent runs on their Mac; hosted tests, UI tests, benchmarks and the first real release run on CI. The first release with the secrets present is the person's, from the Actions tab.

## What the person adds later

The job runs today with none of these; each one turns on its step.

| Secret or setting | Where | What it is | How to make it |
|---|---|---|---|
| `APPLE_DEVELOPER_ID_APPLICATION_P12` | repository secret | The Developer ID Application certificate with its private key, base64 | Keychain Access: export the certificate and key as `.p12`; `base64 -i certificate.p12 \| pbcopy` |
| `APPLE_DEVELOPER_ID_APPLICATION_PASSWORD` | repository secret | The `.p12`'s password | chosen at export |
| `APPLE_NOTARY_KEY` | repository secret | An App Store Connect API key (`.p8`), as text, with the Developer role or higher | App Store Connect > Users and Access > Integrations > App Store Connect API > Team Keys; `cat AuthKey_XXXXXXXXXX.p8 \| pbcopy` |
| `APPLE_NOTARY_KEY_ID` | repository secret | The key's ID (the `XXXXXXXXXX` in the file name) | the same page |
| `APPLE_NOTARY_ISSUER_ID` | repository secret | The issuer UUID | the same page, top |
| `SPARKLE_PRIVATE_KEY` | repository secret | The EdDSA private key whose public half is `Pc0PbtYL9fkyK3XnqULlpKLlH94TE4OWx4Q3n74EftU=` | `desktop/build/sparkle-tools/bin/generate_keys --account tap-desktop -x key.txt` after `desktop/scripts/fetch-sparkle-tools.sh desktop/build/sparkle-tools`; `cat key.txt \| pbcopy`; delete `key.txt`; or copy it from 1Password |
| `HOMEBREW_TAP_TOKEN` | repository secret | exists | already set |
| `HOMEBREW_TAP_REPO` | repository variable, optional | The tap, default `MiniCodeMonkey/homebrew-tap` | only if the cask should live elsewhere |
| The cask repository | `MiniCodeMonkey/homebrew-tap` | `Casks/tap-desktop.rb`, created by the first final release's job | nothing to do; the token has write access already |
| Apple Developer Program enrolment | developer.apple.com | Needed for the certificate and the notary key | the person, after enrolment |

## What this plan found missing in the spec

- The spec says "one release job" for the app and the CLI; the CLI job runs on Ubuntu and cannot build the app, so the app is a second job of the same workflow, after the first, on the same tag and release.
- The spec names no feed URL and no hosting for the appcast; the plan uses the GitHub release asset (open question 1).
- The spec says "Developer ID signing and notarization" without saying what happens before the credentials exist; the person's decision of 2026-09-22 does, and this plan's skip-by-name rule is that decision made testable.
- No feature file has a release, update or install scenario, so `scenarios.txt` gains nothing; the one Sparkle line in `05-presenting.feature` is D4's and stays D4's.
- `CFBundleVersion` was the constant `1` since D2; Sparkle needs it to rise, so Task 1 derives it from the version.
- D6's Settings boards have no Updates card; Task 10 waits for one.

## Execution handoff

Plan complete and saved to `docs/superpowers/plans/2026-09-27-desktop-release.md`. Execute with superpowers:subagent-driven-development, one fresh subagent per task, as the roadmap requires, on `feat/desktop-release` cut from `main` after D6 has merged. Batch A (Tasks 1 to 3) is where the app changes and the one hosted test class lives; Batches B and C are shell and YAML that every implementer proves locally with `make -C desktop release-tests` and the dry run, and CI proves once more on the pull request. Task 10 waits for the SettingsUpdates board's sign-off and can trail the pull request as its own if the sign-off is late.
