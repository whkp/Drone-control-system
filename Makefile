# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
GOFMT=gofmt

# Binary names
MVC_SERVER_BINARY=mvc-server
DB_TOOL_BINARY=db-tool

# Build directory
BUILD_DIR=build

.PHONY: all build clean test coverage deps fmt vet run docker-build docker-up docker-down kafka-demo help

# Default target
all: clean deps fmt vet test build

# Build all binaries
build:
	@echo "Building MVC Server and DB Tool..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) -o $(BUILD_DIR)/$(MVC_SERVER_BINARY) ./cmd/mvc-server
	$(GOBUILD) -o $(BUILD_DIR)/$(DB_TOOL_BINARY) ./cmd/db-tool

# Build MVC server
build-mvc:
	@echo "Building MVC Server..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) -o $(BUILD_DIR)/$(MVC_SERVER_BINARY) ./cmd/mvc-server

# Build DB tool
build-db-tool:
	@echo "Building DB Tool..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) -o $(BUILD_DIR)/$(DB_TOOL_BINARY) ./cmd/db-tool

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

# Run MVC server
run: build-mvc
	@echo "Starting database dependencies..."
	docker-compose up -d mysql redis
	@sleep 5
	@echo "Starting MVC Server on http://localhost:8080..."
	$(BUILD_DIR)/$(MVC_SERVER_BINARY)

# Run database tool
run-db-tool: build-db-tool
	@echo "Running database tool..."
	$(BUILD_DIR)/$(DB_TOOL_BINARY)

# Development environment setup
dev: run

# Docker commands
docker-build:
	@echo "Building Docker images..."
	docker-compose build

docker-up:
	@echo "Starting database services with Docker Compose..."
	docker-compose up -d mysql redis

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

# Database migration using db-tool
migrate:
	@echo "Running database migrations..."
	$(MAKE) run-db-tool

# Health check
health:
	@echo "Checking MVC server health..."
	@curl -f http://localhost:8080/health || echo "MVC Server not responding"

# Install
install: build
	@echo "Installing binaries..."
	@mkdir -p $$GOPATH/bin
	cp $(BUILD_DIR)/* $$GOPATH/bin/

# Cross compilation
build-linux:
	@echo "Building for Linux..."
	@mkdir -p $(BUILD_DIR)/linux
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOBUILD) -o $(BUILD_DIR)/linux/$(MVC_SERVER_BINARY) ./cmd/mvc-server

build-windows:
	@echo "Building for Windows..."
	@mkdir -p $(BUILD_DIR)/windows
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 $(GOBUILD) -o $(BUILD_DIR)/windows/$(MVC_SERVER_BINARY).exe ./cmd/mvc-server

build-mac:
	@echo "Building for macOS..."
	@mkdir -p $(BUILD_DIR)/darwin
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 $(GOBUILD) -o $(BUILD_DIR)/darwin/$(MVC_SERVER_BINARY) ./cmd/mvc-server

# 🚀 Kafka演示
kafka-demo:
	@echo "🚀 启动Kafka集成演示..."
	@chmod +x scripts/kafka-demo.sh
	@./scripts/kafka-demo.sh

# 🌐 启动完整系统（包含Kafka）
run-with-kafka: docker-up build
	@echo "🌐 启动完整系统（API + Kafka）..."
	@echo "📡 启动基础设施..."
	@sleep 5
	@echo "🚀 启动API服务器..."
	@./$(BUILD_DIR)/$(MVC_SERVER_BINARY)

# Help
help:
	@echo "Available commands:"
	@echo "  build          - Build MVC server and db-tool"
	@echo "  build-mvc      - Build MVC server only"
	@echo "  build-db-tool  - Build database tool"
	@echo "  clean          - Clean build files"
	@echo "  test           - Run tests"
	@echo "  coverage       - Run tests with coverage"
	@echo "  deps           - Download dependencies"
	@echo "  fmt            - Format code"
	@echo "  vet            - Vet code"
	@echo "  lint           - Run linter"
	@echo "  run            - Start MVC server with database"
	@echo "  run-db-tool    - Run database migration tool"
	@echo "  kafka-demo     - 🚀 启动Kafka集成演示环境"
	@echo "  run-with-kafka - 🌐 启动完整系统（API + Kafka）"
	@echo "  docker-build   - Build Docker images"
	@echo "  docker-up      - Start database services"
	@echo "  docker-down    - Stop Docker services"
	@echo "  dev-setup      - Setup development environment"
	@echo "  migrate        - Run database migrations"
	@echo "  health         - Check MVC server health"
	@echo "  help           - Show this help"
