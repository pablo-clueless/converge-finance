.PHONY: all build run test clean docker-build docker-up docker-down migrate-up migrate-down lint proto air

# Load .env file if it exists
ifneq (,$(wildcard .env))
    include .env
    export
endif

# Variables
BINARY_NAME=converge
DOCKER_COMPOSE=docker compose -f deployments/docker-compose.yaml

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
GOFMT=gofmt

# Build the application
all: build

build:
	$(GOBUILD) -o $(BINARY_NAME) ./cmd/converge

# Run the application locally
run: build
	./$(BINARY_NAME) serve

# Run tests
test:
	$(GOTEST) -v -race -cover ./...

# Run tests with coverage report
test-coverage:
	$(GOTEST) -v -race -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html

# Clean build artifacts
clean:
	rm -f $(BINARY_NAME)
	rm -f coverage.out coverage.html

# Format code
fmt:
	$(GOFMT) -s -w .

# Lint code
lint:
	golangci-lint run ./...

# Download dependencies
deps:
	$(GOMOD) download
	$(GOMOD) tidy

# Docker commands
docker-build:
	docker build -t converge-finance:latest -f build/docker/Dockerfile .

docker-up:
	$(DOCKER_COMPOSE) up -d

docker-down:
	$(DOCKER_COMPOSE) down

docker-logs:
	$(DOCKER_COMPOSE) logs -f

docker-ps:
	$(DOCKER_COMPOSE) ps

# Database commands
db-up:
	$(DOCKER_COMPOSE) up -d postgres

db-down:
	$(DOCKER_COMPOSE) stop postgres

# Migration commands
migrate-up:
	$(GOBUILD) -o $(BINARY_NAME) ./cmd/converge
	./$(BINARY_NAME) migrate up

migrate-down:
	$(GOBUILD) -o $(BINARY_NAME) ./cmd/converge
	./$(BINARY_NAME) migrate down

migrate-reset:
	$(GOBUILD) -o $(BINARY_NAME) ./cmd/converge
	./$(BINARY_NAME) migrate reset

# Generate protobuf files
proto:
	protoc --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		api/proto/**/*.proto

# Install development tools
install-tools:
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	go install github.com/golang-migrate/migrate/v4/cmd/migrate@latest

# Development shortcuts
dev: db-up migrate-up run

# Run with Air (hot reload) - requires: go install github.com/air-verse/air@latest
air: db-up migrate-up
	air

token:
	@echo "Generating token..."
	go run ./cmd/gentoken/main.go -roles=admin -expiry=720h

# Help
help:
	@echo "Available targets:"
	@echo "  build         		- Build the application"
	@echo "  run           		- Build and run the application"
	@echo "  test          		- Run tests"
	@echo "  test-coverage 		- Run tests with coverage report"
	@echo "  clean         		- Clean build artifacts"
	@echo "  fmt           		- Format code"
	@echo "  lint          		- Lint code"
	@echo "  deps          		- Download dependencies"
	@echo "  docker-build  		- Build Docker image"
	@echo "  docker-up     		- Start Docker containers"
	@echo "  docker-down   		- Stop Docker containers"
	@echo "  docker-logs   		- View Docker logs"
	@echo "  db-up         		- Start PostgreSQL container"
	@echo "  migrate-up    		- Run database migrations"
	@echo "  migrate-down  		- Rollback last migration"
	@echo "  migrate-reset 		- Reset database (drop and recreate)"
	@echo "  proto         		- Generate protobuf files"
	@echo "  install-tools 		- Install development tools"
	@echo "  dev           		- Start development environment"
	@echo "  air           		- Start dev server with hot reload (Air)"
	@echo "  token 						- Generate a JWT token"
