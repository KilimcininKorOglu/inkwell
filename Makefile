.PHONY: build build-linux build-windows build-darwin build-all clean run fmt vet lint help

BUILD_DIR=bin
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE=$(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS=-s -w -X 'main.version=$(VERSION)' -X 'main.commit=$(COMMIT)' -X 'main.buildDate=$(BUILD_DATE)'

build:
	@mkdir -p $(BUILD_DIR)
	go build -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/inkwell .

build-linux:
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/inkwell_linux_amd64 .
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/inkwell_linux_arm64 .

build-windows:
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/inkwell_windows_amd64.exe .

build-darwin:
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/inkwell_darwin_amd64 .
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/inkwell_darwin_arm64 .

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
