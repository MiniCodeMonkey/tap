# Tap Desktop

The native macOS app for writing and presenting tap decks. The design is in
`docs/superpowers/specs/2026-09-22-tap-desktop-design.md`.

## Build

You need Xcode 26, Go, Node and XcodeGen (`brew install xcodegen`).

```sh
make frontend             # once, from the repo root: the app bundles tap
make -C desktop build     # generates Tap.xcodeproj and builds Tap.app
open "desktop/$(make -s -C desktop app-path)"
```

`desktop/project.yml` is the project. `Tap.xcodeproj` is generated and not
committed; run `make -C desktop project` after changing `project.yml` or
adding files.

## Test

```sh
make -C desktop core-test   # the TapDesktopCore package, no Xcode project
make -C desktop test        # tests hosted in Tap.app, with the bundled tap
make -C desktop uitest      # UI tests: takes over the screen, so it runs on CI, not here
make -C desktop test-build  # compiles the hosted and UI tests without running them
make -C desktop bench       # the 200-slide typing and preview benchmarks
make -C desktop check-scenarios
```

The slide operation tests round-trip every result through the bundled
`tap slide list --json` (`TapTests/Support/TapSlideList.swift`), so the
slide order and the separators are checked by tap, never by Swift. The
thumbnail tests need the deck window on screen: the hidden renderer only
paints while the window is visible. Thumbnails are cached under
`~/Library/Application Support/Tap/Thumbnails`; the tests use a temporary
folder instead.

A header drag in the editor leaves a text selection elsewhere untouched:
`SlideDragUITests.testDraggingABoxHeaderLeavesTheSelectedTextInPlace`
checks it with a real drag. `NSTextView`'s end-of-move handling deletes
only the ranges of a text drag it started itself, and a header drag
starts none, so no override is needed.

The presenting tests run a real `tap present --app` beside the deck's
`tap dev --app` and put the audience window into system full screen for a
few seconds each, with AppKit's own animation, where the host can: a probe
(`FullScreenProbe`, once per process) tries it first, and on a host that
cannot the full screen assertions skip with the probe's reason while the
talk runs as plain windows and everything else is checked. The
`FullScreenSpikeTests` record what this host does, for the ledger. The
tests answer tap's recording question ahead of time in the test's own
settings folder, so nothing is ever recorded, and every tunnel test
drives a scripted tap, so no tunnel is ever started. Two displays are
stood in for by the two halves of the one screen
(`PresentingTestCase.halfScreens()`); on one screen the two windows
become two full screen Spaces of that screen, and the tests check the
frame each window was asked for and its full screen state, never its
frame. `make -C desktop test-build` compiles the UI tests without running
them.

The live code tests run the real bundled tap on fixtures with live code
(`TapTests/Fixtures/live-code.md` and its neighbours). tap asks its
approval question at every open of an unapproved deck, so
`HostedTestCase.openDeck` writes tap's own approval record for the
fixture's declared drivers into the test's settings folder first, and
`UITestCase.launch` does the same for a UI test; only the approval tests
(`approvesLiveCodeOnOpen = false`) see the sheet. Nothing a test runs
comes from anywhere but the fixture: a shell block echoes a line, the
sqlite block queries the in-memory default, and the custom driver is
`/bin/cat`. The fix-it and the Deck tab edit the frontmatter through the
editor's `replaceText`, so every change is one undo step and the hidden
range stays clamped.

The D6 tests run the bundled tap's own commands where they need no
network: `tap theme list`, `tap theme set`, `tap image add`, `tap component
new`, `tap build` and `tap serve --json` are real; `tap export pdf` is real
in `testExportAPDF` (CI caches the export engine's download, the same cache
the Go tests use), and scripted in the tests of the download state, the
warnings and Cancel; `tap theme show --image` and `tap image generate` are
scripted (`TapTests/Support/FakeToolScripts.swift`), since the first needs
the engine per theme and the second the Gemini API. A scripted tap stands
in for the one-shot commands only (`AppEnvironment.toolExecutableURL`);
the deck's `tap dev --app` stays real. No test touches the person's
Keychain (`MemoryGeminiKeyStore`, which the app itself uses under
`-TapDefaultsSuite`; the one test of the real Keychain runs on CI alone,
where `TAP_KEYCHAIN_TESTS=1`), `~/.local/bin`
(`CommandLineInstaller(linkDirectory:)`), clipboard
(`EditorTextView.pasteboardForPaste`) or defaults. The Gemini key reaches
the `tap image generate` and `regenerate` runs alone, never a `tap dev` or
`tap present` session; `GenerateImageTests` proves it.

What only a person can check: the theme grid's first open on a Mac that
has never exported (the engine download under the grid, then every render
landing), a real Gemini key in Settings > Image Generation and Generate
Image on a slide of theirs (with Match theme and each aspect), Regenerate
on the result, a PDF and a website export of a real deck of theirs with
Preview in their browser, Install Command Line Tool followed by `tap
--version` in a new Terminal window, and a Homebrew tap on their PATH
shown in Settings > Command Line and left alone by Install.

What only a person can check: the approval sheet's look with the code
of a real deck of theirs, `tap approval revoke` from a terminal while the
deck is open (the next open asks again), a `git pull` that adds a driver
or changes a custom driver's command while the deck is open (tap asks
again, about that alone, with no restart), the Deck tab against their own
frontmatter (Other keys shows what tap does not know), and a `${NAME}` in
a driver's connection read from their `~/.zshrc`.

What only a person can check, with a projector plugged in as a second
display (two displays are otherwise covered only by seam tests): the
audience Space on the projector and the presenter Space on the laptop,
Swap Displays moving them across displays, the projector unplugged
mid-talk (the presenter view comes over the audience view) and plugged
back in (the audience goes back to the projector, the presenter view gets
its Space again), Cmd-Tab to a demo app and back during a talk, the
page's own F-key full screen in either page, the toolbar sliding up from
the bottom edge and the menu bar dropping over the top edge without
covering it, the "Displays have separate Spaces" setting turned off (the
popover's note, plain windows instead of Spaces), a real recording made
with Screen Recording permission (REC in the toolbar, the keep-recording
sheet at Stop, Delete still deleting after a 20 s pause, the run in
Finder), a second talk on the same deck keeping the presenter layout and
notes size (the deck's port), and the phone remote with cloudflared
installed (the QR code from tap, a phone driving the deck). The UI tests
and benchmarks run on CI (the Desktop UI Tests and Desktop Benchmarks
jobs), never on the person's own machine, since they take over the
screen.

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
once the last talk's windows are down, and Play waits, with the
talk-not-started bar, only while Sparkle has an update window or its
permission prompt up; a silent check or download never holds it back. The updater never starts under tests or in a `0.0.0` build.

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

The certificate and the notary key start as files you exported or
downloaded: once their secrets are set, delete `certificate.p12`
(`rm certificate.p12`) and, after keeping a copy somewhere safe such as
1Password, the `.p8`. `security export` cannot pipe the certificate
instead: it has no way to pick one identity, so it would export every
identity in the keychain.

`HOMEBREW_TAP_TOKEN` exists already, and the optional variable
`HOMEBREW_TAP_REPO` names the tap when it is not `MiniCodeMonkey/homebrew-tap`.
A pre-release never reaches the feed or the cask.

The release scripts in `scripts/` read their secrets from the environment,
write a secret that must be a file (the `.p12`, the `.p8`) with `umask 077`
into a private temporary folder that a trap removes, and print none of
them. A few values still travel as command-line arguments, where another
process of the same user could read them while the command runs. Each is
accepted, on a single-tenant runner, and named here:

- the `.p12` password (`APPLE_DEVELOPER_ID_APPLICATION_PASSWORD`), an
  argument of `security import -P` in `signing-identity.sh`;
- the temporary keychain's password, a random value made by
  `signing-identity.sh` for one job, an argument of `security
  create-keychain`, `unlock-keychain` and `set-key-partition-list`;
- `APPLE_NOTARY_KEY_ID` and `APPLE_NOTARY_ISSUER_ID`, arguments of
  `notarytool submit` and `notarytool log` in `notarize.sh`: identifiers,
  useless without the `.p8`, which reaches notarytool only as a file.

`HOMEBREW_TAP_TOKEN` never is: `publish-cask.sh` hands it to git as an
Authorization header through git's environment configuration
(`GIT_CONFIG_COUNT`, `GIT_CONFIG_KEY_0`, `GIT_CONFIG_VALUE_0`) for one
clone and push, never in the URL, an argument or `.git/config`. The
Sparkle key reaches `sign_update` on its standard input alone.

What only a person can check, from the first notarized DMG: drag the app
to Applications and launch it (Gatekeeper accepts it with no dialog);
play a deck with recording on and speak, then confirm the recording has
sound (the hardened runtime's microphone entitlement; CI's recorder is a
fake); Tap > Check for Updates… against the feed; `brew install --cask
MiniCodeMonkey/tap/tap-desktop` on another Mac; `brew audit --cask
--online tap-desktop` with the tap installed, as a manual check (CI runs
`brew style` alone).
