# Tap Desktop. Run from the repo root with `make -C desktop <target>`.
DERIVED_DATA = build/DerivedData
XCODEBUILD = xcodebuild -project Tap.xcodeproj -derivedDataPath $(DERIVED_DATA) CODE_SIGN_IDENTITY=- CODE_SIGNING_REQUIRED=NO
ONLY ?= TapTests

.PHONY: project core-test build test app-path
project:
	xcodegen generate --spec project.yml --quiet

core-test:
	cd TapDesktopCore && swift test

build: project
	$(XCODEBUILD) -scheme Tap -configuration Debug -destination 'platform=macOS' build

# make test ONLY=TapTests/VersionTests/testVersionMatch runs one test.
test: project
	$(XCODEBUILD) -scheme Tap -destination 'platform=macOS' -only-testing:$(ONLY) test

app-path:
	@echo $(DERIVED_DATA)/Build/Products/Debug/Tap.app
