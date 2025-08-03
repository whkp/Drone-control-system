# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
GOFMT=gofmt

# Binary names
API_GATEWAY_BINARY=api-gateway
USER_SERVICE_BINARY=user-service
TASK_SERVICE_BINARY=task-service
MONITOR_SERVICE_BINARY=monitor-service
DRONE_CONTROL_BINARY=drone-control

# Build directory
BUILD_DIR=build

.PHONY: all build clean test coverage deps fmt vet run-all run-api run-user run-task run-monitor run-drone docker-build docker-up docker-down help

# Default target
all: clean deps fmt vet test build

# Build all services
build:
	@echo "Building all services..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) -o $(BUILD_DIR)/$(API_GATEWAY_BINARY) ./cmd/api-gateway
	$(GOBUILD) -o $(BUILD_DIR)/$(USER_SERVICE_BINARY) ./cmd/user-service
	$(GOBUILD) -o $(BUILD_DIR)/$(TASK_SERVICE_BINARY) ./cmd/task-service
	$(GOBUILD) -o $(BUILD_DIR)/$(MONITOR_SERVICE_BINARY) ./cmd/monitor-service
	$(GOBUILD) -o $(BUILD_DIR)/$(DRONE_CONTROL_BINARY) ./cmd/drone-control

# Build individual services
build-api:
	@echo "Building API Gateway..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) -o $(BUILD_DIR)/$(API_GATEWAY_BINARY) ./cmd/api-gateway

build-user:
	@echo "Building User Service..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) -o $(BUILD_DIR)/$(USER_SERVICE_BINARY) ./cmd/user-service

build-task:
	@echo "Building Task Service..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) -o $(BUILD_DIR)/$(TASK_SERVICE_BINARY) ./cmd/task-service

build-monitor:
	@echo "Building Monitor Service..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) -o $(BUILD_DIR)/$(MONITOR_SERVICE_BINARY) ./cmd/monitor-service

build-drone:
	@echo "Building Drone Control Service..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) -o $(BUILD_DIR)/$(DRONE_CONTROL_BINARY) ./cmd/drone-control

# Clean
clean:
	@echo "Cleaning..."
	$(GOCLEAN)
	rm -rf $(BUILD_DIR)

# Test
test:
	@echo "Running tests..."
	$(GOTEST) -v ./...

# Test with coverage
coverage:
	@echo "Running tests with coverage..."
	$(GOTEST) -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html

# Download dependencies
deps:
	@echo "Downloading dependencies..."
	$(GOMOD) download
	$(GOMOD) tidy

# Format code
fmt:
	@echo "Formatting code..."
	$(GOFMT) -s -w .

# Vet code
vet:
	@echo "Vetting code..."
	$(GOCMD) vet ./...

# Run all services locally
run-all: build
	@echo "Starting all services..."
	@echo "Starting Redis and MySQL (make sure Docker is running)..."
	docker-compose up -d mysql redis
	@sleep 5
	@echo "Starting API Gateway..."
	$(BUILD_DIR)/$(API_GATEWAY_BINARY) &
	@echo "Starting User Service..."
	$(BUILD_DIR)/$(USER_SERVICE_BINARY) &
	@echo "Starting Task Service..."
	$(BUILD_DIR)/$(TASK_SERVICE_BINARY) &
	@echo "Starting Monitor Service..."
	$(BUILD_DIR)/$(MONITOR_SERVICE_BINARY) &
	@echo "Starting Drone Control Service..."
	$(BUILD_DIR)/$(DRONE_CONTROL_BINARY) &
	@echo "All services started!"

# Run individual services
run-api: build-api
	$(BUILD_DIR)/$(API_GATEWAY_BINARY)

run-user: build-user
	$(BUILD_DIR)/$(USER_SERVICE_BINARY)

run-task: build-task
	$(BUILD_DIR)/$(TASK_SERVICE_BINARY)

run-monitor: build-monitor
	$(BUILD_DIR)/$(MONITOR_SERVICE_BINARY)

run-drone: build-drone
	$(BUILD_DIR)/$(DRONE_CONTROL_BINARY)

# Docker commands
docker-build:
	@echo "Building Docker images..."
	docker-compose build

docker-up:
	@echo "Starting services with Docker Compose..."
	docker-compose up -d

docker-down:
	@echo "Stopping services..."
	docker-compose down

docker-logs:
	@echo "Showing logs..."
	docker-compose logs -f

# Development commands
dev-setup:
	@echo "Setting up development environment..."
	@echo "Installing Go tools..."
	$(GOGET) golang.org/x/tools/cmd/goimports
	$(GOGET) github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@echo "Creating necessary directories..."
	@mkdir -p logs
	@mkdir -p data
	@echo "Development setup complete!"

# Linting
lint:
	@echo "Running linter..."
	golangci-lint run

# Generate protocol buffers (if needed)
protoc:
	@echo "Generating protobuf files..."
	protoc --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		api/proto/*.proto

# Database migration (placeholder)
migrate-up:
	@echo "Running database migrations..."
	# Add migration command here

migrate-down:
	@echo "Rolling back database migrations..."
	# Add rollback command here

# Health check
health:
	@echo "Checking service health..."
	@curl -f http://localhost:8080/health || echo "API Gateway not responding"
	@curl -f http://localhost:50050/health || echo "Drone Control not responding"

# Install
install: build
	@echo "Installing binaries..."
	@mkdir -p $$GOPATH/bin
	cp $(BUILD_DIR)/* $$GOPATH/bin/

# Cross compilation
build-linux:
	@echo "Building for Linux..."
	@mkdir -p $(BUILD_DIR)/linux
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOBUILD) -o $(BUILD_DIR)/linux/$(API_GATEWAY_BINARY) ./cmd/api-gateway
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOBUILD) -o $(BUILD_DIR)/linux/$(DRONE_CONTROL_BINARY) ./cmd/drone-control

build-windows:
	@echo "Building for Windows..."
	@mkdir -p $(BUILD_DIR)/windows
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 $(GOBUILD) -o $(BUILD_DIR)/windows/$(API_GATEWAY_BINARY).exe ./cmd/api-gateway
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 $(GOBUILD) -o $(BUILD_DIR)/windows/$(DRONE_CONTROL_BINARY).exe ./cmd/drone-control

build-mac:
	@echo "Building for macOS..."
	@mkdir -p $(BUILD_DIR)/darwin
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 $(GOBUILD) -o $(BUILD_DIR)/darwin/$(API_GATEWAY_BINARY) ./cmd/api-gateway
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 $(GOBUILD) -o $(BUILD_DIR)/darwin/$(DRONE_CONTROL_BINARY) ./cmd/drone-control

# Help
help:
	@echo "Available commands:"
	@echo "  build          - Build all services"
	@echo "  build-api      - Build API Gateway"
	@echo "  build-drone    - Build Drone Control Service"
	@echo "  clean          - Clean build files"
	@echo "  test           - Run tests"
	@echo "  coverage       - Run tests with coverage"
	@echo "  deps           - Download dependencies"
	@echo "  fmt            - Format code"
	@echo "  vet            - Vet code"
	@echo "  lint           - Run linter"
	@echo "  run-all        - Run all services locally"
	@echo "  run-api        - Run API Gateway"
	@echo "  run-drone      - Run Drone Control Service"
	@echo "  docker-build   - Build Docker images"
	@echo "  docker-up      - Start with Docker Compose"
	@echo "  docker-down    - Stop Docker services"
	@echo "  dev-setup      - Setup development environment"
	@echo "  health         - Check service health"
	@echo "  help           - Show this help"
