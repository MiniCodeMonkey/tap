# Tap Desktop. Run from the repo root with `make -C desktop <target>`.
DERIVED_DATA = build/DerivedData
XCODEBUILD = xcodebuild -project Tap.xcodeproj -derivedDataPath $(DERIVED_DATA) CODE_SIGN_IDENTITY=- CODE_SIGNING_REQUIRED=NO
ONLY ?= TapTests
# Every hosted test, UI test and benchmark has five minutes, so a hang
# fails that one test with a timeout instead of the whole job. A test
# that needs longer sets its own executionTimeAllowance, up to ten.
TEST_TIMEOUTS = -test-timeouts-enabled YES -default-test-execution-time-allowance 300 -maximum-test-execution-time-allowance 600

.PHONY: project core-test build test test-build uitest bench bench-build app-path check-scenarios check-release-hooks
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
