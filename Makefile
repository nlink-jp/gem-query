BINARY  := gem-query
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-X main.version=$(VERSION)"
DIST_DIR := dist

# Container runtime (podman preferred, docker as fallback)
CONTAINER := $(shell command -v podman 2>/dev/null || command -v docker 2>/dev/null)
GO_IMAGE  := golang:1.26

PLATFORMS := \
	linux/amd64 \
	linux/arm64 \
	darwin/amd64 \
	darwin/arm64 \
	windows/amd64

.PHONY: build build-all build-darwin build-linux build-linux-native build-windows \
        test vet check clean help

## build: Build for the current platform (CGO required for DuckDB)
build:
	@mkdir -p $(DIST_DIR)
	CGO_ENABLED=1 go build $(LDFLAGS) -o $(DIST_DIR)/$(BINARY) .

## build-all: Cross-compile for all target platforms
build-all: build-darwin build-linux build-windows

## build-darwin: Compile darwin/amd64 and darwin/arm64 (macOS host only)
build-darwin:
	@mkdir -p $(DIST_DIR)
	CGO_ENABLED=1 GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o $(DIST_DIR)/$(BINARY)-darwin-amd64 .
	CGO_ENABLED=1 GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o $(DIST_DIR)/$(BINARY)-darwin-arm64 .

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
		bash -c "apt-get update -qq && apt-get install -y -q gcc-mingw-w64-x86-64 \
			&& GOOS=windows GOARCH=amd64 CGO_ENABLED=1 CC=x86_64-w64-mingw32-gcc \
			go build $(LDFLAGS) -o $(DIST_DIR)/$(BINARY)-windows-amd64.exe ."

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
