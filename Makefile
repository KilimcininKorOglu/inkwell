.PHONY: build build-linux build-windows build-darwin build-all clean run fmt vet lint check-zig help

BUILD_DIR=bin
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE=$(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS=-s -w -X 'main.version=$(VERSION)' -X 'main.commit=$(COMMIT)' -X 'main.buildDate=$(BUILD_DATE)'

build:
	@mkdir -p $(BUILD_DIR)
	go build -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/inkwell .

# Cross-platform builds (requires zig: brew install zig)
check-zig:
	@which zig > /dev/null 2>&1 || (echo "ERROR: zig is required for cross-compilation. Install with: brew install zig" && exit 1)

build-linux: check-zig
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=1 GOOS=linux GOARCH=amd64 CC="zig cc -target x86_64-linux" CXX="zig c++ -target x86_64-linux" go build -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/inkwell_linux_amd64 .
	CGO_ENABLED=1 GOOS=linux GOARCH=arm64 CC="zig cc -target aarch64-linux" CXX="zig c++ -target aarch64-linux" go build -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/inkwell_linux_arm64 .

build-windows: check-zig
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=1 GOOS=windows GOARCH=amd64 CC="zig cc -target x86_64-windows" CXX="zig c++ -target x86_64-windows" go build -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/inkwell_windows_amd64.exe .

build-darwin: check-zig
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=1 GOOS=darwin GOARCH=amd64 CC="zig cc -target x86_64-macos" CXX="zig c++ -target x86_64-macos" go build -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/inkwell_darwin_amd64 .
	CGO_ENABLED=1 GOOS=darwin GOARCH=arm64 CC="zig cc -target aarch64-macos" CXX="zig c++ -target aarch64-macos" go build -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/inkwell_darwin_arm64 .

build-all: build-linux build-windows build-darwin

clean:
	rm -rf $(BUILD_DIR)
	go clean

run: build
	./$(BUILD_DIR)/inkwell

fmt:
	go fmt ./...

vet:
	go vet ./...

lint: fmt vet

help:
	@echo "Available targets:"
	@echo "  build          - Build binary (current OS/arch)"
	@echo "  build-linux    - Cross-compile for Linux (amd64 + arm64)"
	@echo "  build-windows  - Cross-compile for Windows (amd64)"
	@echo "  build-darwin   - Cross-compile for macOS (amd64 + arm64)"
	@echo "  build-all      - Cross-compile for all platforms"
	@echo "  clean          - Remove build artifacts"
	@echo "  run            - Build and run"
	@echo "  fmt            - Format code"
	@echo "  vet            - Run go vet"
	@echo "  lint           - Run fmt and vet"
	@echo ""
	@echo "Cross-compilation requires zig: brew install zig"
	@echo "CI releases use goreleaser-cross (no zig needed)."
