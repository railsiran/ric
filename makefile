PROJECT := ric
BUILD_DIR := dist

# Default values (used when no arguments given)
VERSION ?= 0.0.0-dev
CODENAME ?= mystery-kebab

# Build metadata
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

# Ldflags
LDFLAGS := -X ric/version.Version=$(VERSION) -X ric/version.Codename=$(CODENAME) -X ric/version.GitCommit=$(COMMIT) -X ric/version.BuildDate=$(DATE)

# Targets
.PHONY: all clean build build-all install test dev

all: build-all

build:
	@mkdir -p $(BUILD_DIR)
	go build -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(PROJECT) .
	@echo "✅ Built: $(BUILD_DIR)/$(PROJECT)  ($(VERSION) - $(CODENAME))"

build-all: clean
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(PROJECT)-linux-amd64 .
	GOOS=linux GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(PROJECT)-linux-arm64 .
	GOOS=darwin GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(PROJECT)-darwin-arm64 .
	GOOS=windows GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(PROJECT)-windows-amd64.exe .
	@echo "✅ All binaries built in $(BUILD_DIR)/ ($(VERSION) - $(CODENAME))"

clean:
	rm -rf $(BUILD_DIR)

test:
	go test ./...

dev: build
	./$(BUILD_DIR)/$(PROJECT) version

release: build-all
	cp install.sh $(BUILD_DIR)/
	@echo "📦 Release $(VERSION) ($(CODENAME)) ready in $(BUILD_DIR)/"
	@ls -la $(BUILD_DIR)/
