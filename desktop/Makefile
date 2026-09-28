# Tap Desktop. Run from the repo root with `make -C desktop <target>`.
DERIVED_DATA = build/DerivedData
XCODEBUILD = xcodebuild -project Tap.xcodeproj -derivedDataPath $(DERIVED_DATA) CODE_SIGN_IDENTITY=- CODE_SIGNING_REQUIRED=NO
ONLY ?= TapTests
# Every hosted test, UI test and benchmark has five minutes, so a hang
# fails that one test with a timeout instead of the whole job. A test
# that needs longer sets its own executionTimeAllowance, up to ten.
TEST_TIMEOUTS = -test-timeouts-enabled YES -default-test-execution-time-allowance 300 -maximum-test-execution-time-allowance 600

# The version a release build carries, in the app, its plist and the bundled
# tap. 0.0.0-dev is a local dry run; the release job passes the real one.
VERSION ?= 0.0.0-dev
BUILD_NUMBER = $(shell ./scripts/build-number.sh $(VERSION))
RELEASE_APP = $(DERIVED_DATA)/Build/Products/Release/Tap.app
RELEASE_BINARY = $(RELEASE_APP)/Contents/MacOS/Tap
RELEASE_DIR = build/release

.PHONY: project core-test build test test-build uitest bench bench-build app-path check-scenarios check-release-hooks release-build check-release-app-hooks release-tests release
project:
	xcodegen generate --spec project.yml --quiet

core-test:
	cd TapDesktopCore && swift test

build: project
	$(XCODEBUILD) -scheme Tap -configuration Debug -destination 'platform=macOS' build

# make test ONLY=TapTests/VersionTests/testVersionMatch runs one test.
test: project
	$(XCODEBUILD) -scheme Tap -destination 'platform=macOS' -only-testing:$(ONLY) $(TEST_TIMEOUTS) test

uitest: project
	$(XCODEBUILD) -scheme Tap -destination 'platform=macOS' -only-testing:TapUITests $(TEST_TIMEOUTS) test

bench: project
	$(XCODEBUILD) -scheme TapBenchmarks -destination 'platform=macOS' $(TEST_TIMEOUTS) test

# Compiles the hosted and UI tests without running them; uitest, which runs them, is CI's job.
test-build: project
	$(XCODEBUILD) -scheme Tap -destination 'platform=macOS' build-for-testing

bench-build: project
	$(XCODEBUILD) -scheme TapBenchmarks -destination 'platform=macOS' build-for-testing

app-path:
	@echo $(DERIVED_DATA)/Build/Products/Debug/Tap.app

check-scenarios:
	./scripts/check-scenarios-test.sh
	./scripts/check-scenarios.sh

# The test-only hooks are compiled into Debug alone: the Benchmark build, a
# release build, holds no symbol of approvalAnswerForTests, while it does
# hold presentExecutableURL, so a stripped or renamed binary cannot pass.
RELEASE_TAP = $(DERIVED_DATA)/Build/Products/Benchmark/Tap.app/Contents/MacOS/Tap
check-release-hooks: bench-build
	@nm $(RELEASE_TAP) | grep -q presentExecutableURL || { echo "no symbols to check in $(RELEASE_TAP)"; exit 1; }
	@! nm $(RELEASE_TAP) | grep -q approvalAnswerForTests || { echo "approvalAnswerForTests is in a release build"; exit 1; }
	@echo "no test-only hook in the release build"

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
