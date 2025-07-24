# Makefile for BMW CarData Dashboard Go Implementation

# Variables
BINARY_NAME=bmwtools-server
GO_FILES=$(shell find . -name "*.go")
GO_MODULE=github.com/awlx/bmwtools

# Go build flags
LDFLAGS=-ldflags "-s -w"

.PHONY: all build clean run docker-build docker-run docker-stop test lint vet fmt help setup-db

# Default target
all: clean setup-db build

# Build the application
build:
	@echo "Building $(BINARY_NAME)..."
	go build $(LDFLAGS) -o $(BINARY_NAME) ./cmd/server/main.go

# Clean build files
clean:
	@echo "Cleaning..."
	go clean
	rm -f $(BINARY_NAME)

# Run the application
run: build
	@echo "Running $(BINARY_NAME)..."
	./$(BINARY_NAME)

# Run tests
test:
	@echo "Running tests..."
	go test ./...

# Run linter
lint:
	@echo "Running linter..."
	golangci-lint run

# Run go vet
vet:
	@echo "Running go vet..."
	go vet ./...

# Format code
fmt:
	@echo "Formatting code..."
	go fmt ./...

# Build Docker image
docker-build:
	@echo "Building Docker image..."
	docker build -t bmwtools-server .

# Run Docker container
docker-run: docker-build
	@echo "Running Docker container..."
	docker run -p 8050:8050 --name bmwtools-server -d bmwtools-server

# Stop Docker container
docker-stop:
	@echo "Stopping Docker container..."
	docker stop bmwtools-server || true
	docker rm bmwtools-server || true

# Run with Docker Compose
compose-up:
	@echo "Starting with Docker Compose..."
	docker-compose up -d

# Stop Docker Compose services
compose-down:
	@echo "Stopping Docker Compose services..."
	docker-compose down

# Setup database dependencies
setup-db:
	@echo "Setting up database dependencies..."
	go get github.com/mattn/go-sqlite3

# Help
help:
	@echo "BMW CarData Dashboard Go Implementation"
	@echo ""
	@echo "Usage:"
	@echo "  make build          - Build the application"
	@echo "  make clean          - Remove build artifacts"
	@echo "  make run            - Build and run the application"
	@echo "  make test           - Run tests"
	@echo "  make lint           - Run linter"
	@echo "  make vet            - Run go vet"
	@echo "  make fmt            - Format code"
	@echo "  make setup-db       - Set up database dependencies"
	@echo "  make docker-build   - Build Docker image"
	@echo "  make docker-run     - Run in Docker container"
	@echo "  make docker-stop    - Stop Docker container"
	@echo "  make compose-up     - Start with Docker Compose"
	@echo "  make compose-down   - Stop Docker Compose services"
	@echo "  make help           - Show this help message"
