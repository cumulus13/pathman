.PHONY: build test clean install

APP_NAME := pathman
BUILD_DIR := build
GO := go
GOFLAGS := -trimpath -ldflags="-s -w"

build:
	@echo "🔨 Building $(APP_NAME)..."
	@$(GO) build $(GOFLAGS) -o $(BUILD_DIR)/$(APP_NAME).exe ./cmd/$(APP_NAME)
	@echo "✅ Build complete: $(BUILD_DIR)/$(APP_NAME).exe"

install: build
	@echo "📦 Installing $(APP_NAME)..."
	@copy $(BUILD_DIR)\$(APP_NAME).exe %USERPROFILE%\go\bin\$(APP_NAME).exe
	@echo "✅ Installed successfully"

test:
	@echo "🧪 Running tests..."
	@$(GO) test -v -race -coverprofile=coverage.out ./...
	@$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "✅ Tests complete"

clean:
	@echo "🧹 Cleaning..."
	@rm -rf $(BUILD_DIR) coverage.out coverage.html
	@echo "✅ Clean complete"

lint:
	@echo "🔍 Running linter..."
	@golangci-lint run ./...
	@echo "✅ Lint complete"

run:
	@$(GO) run ./cmd/$(APP_NAME)

build-all:
	@echo "🚀 Building for all platforms..."
	@GOOS=windows GOARCH=amd64 $(GO) build $(GOFLAGS) -o $(BUILD_DIR)/$(APP_NAME)-win64.exe ./cmd/$(APP_NAME)
	@GOOS=windows GOARCH=386 $(GO) build $(GOFLAGS) -o $(BUILD_DIR)/$(APP_NAME)-win32.exe ./cmd/$(APP_NAME)
	@echo "✅ All builds complete"