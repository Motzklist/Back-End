BINARY_NAME=motzklist-api-gateway
BUILD_DIR=./build

# Default command: runs the server (using '.' to include all files in package)
.PHONY: run
run:
	go run .

# Builds the binary (using '.' to include all files in package)
.PHONY: build
build:
	go build -o $(BUILD_DIR)/$(BINARY_NAME) .

# Cleans up the build directory
.PHONY: clean
clean:
	rm -rf $(BUILD_DIR)

# Installs dependencies
.PHONY: deps
deps:
	go mod download

# Runs tests
.PHONY: test
test:
	@if command -v gotestsum > /dev/null; then \
		gotestsum --format testname ./...; \
	else \
		go test -v ./...; \
	fi