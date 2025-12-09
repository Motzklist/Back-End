# Makefile for api-gateway-avner

BINARY_NAME=motzklist-api-gateway
BUILD_DIR=./build
SRC_DIR=./

# Default command: runs the server
.PHONY: run
run:
	go run $(SRC_DIR)main.go

# Builds the binary
.PHONY: build
build:
	go build -o $(BUILD_DIR)/$(BINARY_NAME) $(SRC_DIR)main.go

# Cleans up the build directory
.PHONY: clean
clean:
	rm -rf $(BUILD_DIR)

# Installs dependencies
.PHONY: deps
deps:
	go mod download