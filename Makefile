.PHONY: build clean run docker-up docker-down help

BINARY_NAME=migrator
BUILD_DIR=bin

help:
	@echo "Available targets:"
	@echo "  build       - Build the binary"
	@echo "  clean       - Remove build artifacts"
	@echo "  run         - Run the application in demo mode"
	@echo "  docker-up   - Start Docker containers"
	@echo "  docker-down - Stop Docker containers"

build:
	@echo "Building..."
	@go build -o $(BUILD_DIR)/$(BINARY_NAME) cmd/migrator/main.go
	@echo "Build complete: $(BUILD_DIR)/$(BINARY_NAME)"

clean:
	@echo "Cleaning..."
	@rm -rf $(BUILD_DIR)
	@echo "Clean complete"

run: build
	@./$(BUILD_DIR)/$(BINARY_NAME) --demo --workers 8

docker-up:
	@echo "Starting Docker containers..."
	@docker-compose up -d
	@docker-compose ps

docker-down:
	@echo "Stopping Docker containers..."
	@docker-compose down

.DEFAULT_GOAL := help
