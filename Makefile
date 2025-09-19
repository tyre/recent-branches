.PHONY: build clean run test install

# Binary name
BINARY_NAME=recent-branches
BUILD_DIR=build

# Build the application
build:
	@mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/$(BINARY_NAME)

# Build and run the application
run: build
	./$(BUILD_DIR)/$(BINARY_NAME)

# Run with specific flags
run-remote: build
	./$(BUILD_DIR)/$(BINARY_NAME) -remote -n 10

# Clean build artifacts
clean:
	@rm -rf $(BUILD_DIR)
	@rm -f $(BINARY_NAME)

# Run tests
test:
	go test -v ./...

# Install dependencies
install:
	go mod download
	go mod tidy

# Format code
fmt:
	go fmt ./...

# Run linter
lint:
	golangci-lint run

# Build for multiple platforms
build-all: clean
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=amd64 go build -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64
	GOOS=darwin GOARCH=amd64 go build -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64
	GOOS=darwin GOARCH=arm64 go build -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64
	GOOS=windows GOARCH=amd64 go build -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe

# Development build (builds to current directory for quick testing)
dev:
	go build -o $(BINARY_NAME)