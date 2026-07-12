BINARY  := gem-query
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-X main.version=$(VERSION)"
DIST_DIR := dist

# Container runtime (podman preferred, docker as fallback)
CONTAINER := $(shell command -v podman 2>/dev/null || command -v docker 2>/dev/null)
GO_IMAGE  := golang:1.26.2

# macOS Developer ID signing / notarization (see nlink-jp/.github
# CONVENTIONS.md §Code Signing). Defaults match any Developer ID
# Application cert in the keychain and the org-standard notary
# profile. Builds without these fall back to ad-hoc / un-notarized
# with a one-line warning — see scripts/codesign-darwin.sh. The
# codesign step runs on the macOS host only; non-Mach-O binaries
# (produced inside the linux / windows build containers) are
# auto-skipped by the script.
CODESIGN_IDENTITY ?= Developer ID Application
NOTARY_PROFILE    ?= nlink-jp-notary

# darwin ships arm64 only (no amd64, no universal). linux/windows keep their matrix.
PLATFORMS := \
	linux/amd64 \
	linux/arm64 \
	darwin/arm64 \
	windows/amd64

.PHONY: build build-all build-darwin build-linux build-linux-native build-windows \
        package test vet check clean help

## build: Build for the current platform (CGO required for DuckDB)
build:
	@mkdir -p $(DIST_DIR)
	CGO_ENABLED=1 go build $(LDFLAGS) -o $(DIST_DIR)/$(BINARY) .
	@scripts/codesign-darwin.sh $(DIST_DIR)/$(BINARY) "$(CODESIGN_IDENTITY)"

## build-all: Cross-compile for all target platforms
build-all: build-darwin build-linux build-windows

## build-darwin: Compile darwin/arm64 (macOS host only; arm64-only policy, no Intel)
build-darwin:
	@mkdir -p $(DIST_DIR)
	CGO_ENABLED=1 GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o $(DIST_DIR)/$(BINARY)-darwin-arm64 .
	@scripts/codesign-darwin.sh $(DIST_DIR)/$(BINARY)-darwin-arm64 "$(CODESIGN_IDENTITY)" "$(BINARY)"

## build-linux: Compile linux/amd64 and linux/arm64 inside a container
build-linux:
	@if [ -z "$(CONTAINER)" ]; then \
		echo "Error: podman or docker is required for Linux cross-compilation."; \
		exit 1; \
	fi
	@mkdir -p $(DIST_DIR)
	$(CONTAINER) run --rm \
		-v "$(CURDIR):/workspace:z" \
		-w /workspace \
		$(GO_IMAGE) \
		bash -c "apt-get update -qq && apt-get install -y -q \
			gcc-aarch64-linux-gnu g++-aarch64-linux-gnu \
			gcc-x86-64-linux-gnu g++-x86-64-linux-gnu \
			&& make build-linux-native"

## build-linux-native: Compile linux/amd64 and linux/arm64 (Linux host only)
build-linux-native:
	@mkdir -p $(DIST_DIR)
	@echo "Building linux/amd64..."
	@if [ "$$(uname -m)" = "aarch64" ]; then \
		GOOS=linux GOARCH=amd64 CGO_ENABLED=1 CC=x86_64-linux-gnu-gcc \
			go build $(LDFLAGS) -o $(DIST_DIR)/$(BINARY)-linux-amd64 .; \
	else \
		GOOS=linux GOARCH=amd64 CGO_ENABLED=1 \
			go build $(LDFLAGS) -o $(DIST_DIR)/$(BINARY)-linux-amd64 .; \
	fi
	@echo "Building linux/arm64..."
	@if [ "$$(uname -m)" = "x86_64" ]; then \
		GOOS=linux GOARCH=arm64 CGO_ENABLED=1 CC=aarch64-linux-gnu-gcc \
			go build $(LDFLAGS) -o $(DIST_DIR)/$(BINARY)-linux-arm64 .; \
	else \
		GOOS=linux GOARCH=arm64 CGO_ENABLED=1 \
			go build $(LDFLAGS) -o $(DIST_DIR)/$(BINARY)-linux-arm64 .; \
	fi

## build-windows: Compile windows/amd64 inside a container
build-windows:
	@if [ -z "$(CONTAINER)" ]; then \
		echo "Error: podman or docker is required for Windows cross-compilation."; \
		exit 1; \
	fi
	@mkdir -p $(DIST_DIR)
	$(CONTAINER) run --rm \
		-v "$(CURDIR):/workspace:z" \
		-w /workspace \
		$(GO_IMAGE) \
		bash -c 'apt-get update -qq && apt-get install -y -q gcc-mingw-w64-ucrt64 g++-mingw-w64-ucrt64 \
			&& find /usr/lib/gcc/x86_64-w64-mingw32ucrt /usr/x86_64-w64-mingw32ucrt -name "*.a" -exec x86_64-w64-mingw32ucrt-ranlib {} + \
			&& GOOS=windows GOARCH=amd64 CGO_ENABLED=1 \
			CC=x86_64-w64-mingw32ucrt-gcc CXX=x86_64-w64-mingw32ucrt-g++ \
			go build -ldflags "-X main.version=$(VERSION)" -o $(DIST_DIR)/$(BINARY)-windows-amd64.exe .'

## package: Build all platforms, archive with version suffix (zip for
## darwin/windows, tar.gz for linux), bundle the canonical binary +
## README.md + LICENSE, and notarize the darwin build. Asset naming
## follows the org Release Archive Standard
## (gem-query-vX.Y.Z-<os>-<arch>.<ext>).
package: build-all
	@cd $(DIST_DIR) && for p in $(PLATFORMS); do os=$${p%/*}; arch=$${p#*/}; \
		ext=""; [ "$$os" = windows ] && ext=".exe"; \
		stage=_pkg; rm -rf $$stage; mkdir -p $$stage; \
		cp "$(BINARY)-$$os-$$arch$$ext" "$$stage/$(BINARY)$$ext"; \
		cp ../README.md ../LICENSE $$stage/; \
		base="$(BINARY)-$(VERSION)-$$os-$$arch"; \
		if [ "$$os" = linux ]; then ( cd $$stage && tar -czf "../$$base.tar.gz" * ); \
		else ( cd $$stage && zip -q "../$$base.zip" * ); fi; \
		rm -rf $$stage; \
	done
	@scripts/notarize-darwin.sh $(DIST_DIR)/$(BINARY)-$(VERSION)-darwin-arm64.zip "$(NOTARY_PROFILE)"

## test: Run the full test suite
test:
	go test -race -cover ./...

## vet: Run go vet
vet:
	go vet ./...

## check: Run vet + test + build
check: vet test build

## clean: Remove build artifacts
clean:
	rm -rf $(DIST_DIR)

## help: Show this help
help:
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## //'
