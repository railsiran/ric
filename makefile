PROJECT := ric
BUILD_DIR := dist/$(TARGET)

VERSION ?= 0.0.0-dev
CODENAME ?= mystery-kebab
TARGET ?= develop

ifeq ($(TARGET),railsiran)
  BASE_URL := https://railsiran.org
else ifeq ($(TARGET),github)
  BASE_URL := https://github.com/railsiran/ric/releases/latest/download
else
  BASE_URL := http://localhost:3000
endif

COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
BRANCH := $(shell git rev-parse --abbrev-ref HEAD 2>/dev/null || echo "unknown")

LDFLAGS := -X ric/version.Version=$(VERSION) \
           -X ric/version.Codename=$(CODENAME) \
           -X ric/version.GitCommit=$(COMMIT) \
           -X ric/version.BuildDate=$(DATE) \
           -X ric/version.Branch=$(BRANCH) \
           -X ric/commands.BaseURL=$(BASE_URL)

.PHONY: all clean build build-all test dev

all: build-all

build:
	@mkdir -p $(BUILD_DIR)
	go build -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(PROJECT) .
	@echo "✅ Built for target: $(TARGET)  ($(VERSION) - $(CODENAME))"
	@echo "   Output: $(BUILD_DIR)/$(PROJECT)"

build-all: clean
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(PROJECT)-linux-amd64 .
	GOOS=linux GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(PROJECT)-linux-arm64 .
	GOOS=darwin GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(PROJECT)-darwin-arm64 .
	GOOS=windows GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(PROJECT)-windows-amd64.exe .
	@echo "✅ All binaries built for target: $(TARGET)  ($(VERSION) - $(CODENAME))"
	@echo "   Output: $(BUILD_DIR)/"

clean:
	rm -rf $(BUILD_DIR)

test:
	go test ./...

dev: build
	./$(BUILD_DIR)/$(PROJECT) version
	@echo "Target: $(TARGET)  Base URL: $(BASE_URL)"
