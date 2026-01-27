.PHONY: build build-api build-worker build-simulator test test-unit test-integration lint clean \
        dev-up dev-down test-up test-down docker-build docker-build-api docker-build-worker

# Docker compose paths
DOCKER_DIR := infrastructure/docker
COMPOSE := docker compose -f $(DOCKER_DIR)/docker-compose.yml
COMPOSE_TEST := docker compose -f $(DOCKER_DIR)/docker-compose.test.yml

# =============================================================================
# Build targets
# =============================================================================

# Build all services
build: build-api build-worker

# Build the combined binary (legacy, for backwards compatibility)
build-combined:
	go build -o payment-processing .

# Build payment-api service
build-api:
	go build -o bin/payment-api ./services/payment-api/cmd

# Build payment-worker service
build-worker:
	go build -o bin/payment-worker ./services/payment-worker/cmd

# Build provider-simulator (separate module)
build-simulator:
	cd services/provider-simulator && go build -o ../../bin/provider-simulator .

# =============================================================================
# Docker targets
# =============================================================================

# Build all Docker images
docker-build: docker-build-api docker-build-worker docker-build-simulator

# Build payment-api Docker image
docker-build-api:
	docker build -t payment-api:latest -f services/payment-api/Dockerfile .

# Build payment-worker Docker image
docker-build-worker:
	docker build -t payment-worker:latest -f services/payment-worker/Dockerfile .

# Build provider-simulator Docker image
docker-build-simulator:
	docker build -t provider-simulator:latest -f services/provider-simulator/Dockerfile services/provider-simulator

# =============================================================================
# Development environment
# =============================================================================

# Start development stack (postgres, temporal, temporal-ui)
dev-up:
	$(COMPOSE) up -d postgres temporal temporal-ui
	@echo "Development services starting..."
	@echo "  PostgreSQL: localhost:5432"
	@echo "  Temporal: localhost:7233"
	@echo "  Temporal UI: http://localhost:8088"

# Start full development stack including services
dev-up-all:
	$(COMPOSE) up -d
	@echo "All services starting..."
	@echo "  Payment API: http://localhost:8080"
	@echo "  Temporal UI: http://localhost:8088"

# Stop development stack
dev-down:
	$(COMPOSE) down

# Stop and remove volumes
dev-down-v:
	$(COMPOSE) down -v

# View logs
dev-logs:
	$(COMPOSE) logs -f

# =============================================================================
# Testing environment
# =============================================================================

# Start test environment with provider simulator
test-up:
	$(COMPOSE_TEST) up -d
	@echo "Test environment starting..."
	@echo "  PostgreSQL: localhost:5433"
	@echo "  Temporal: localhost:7234"
	@echo "  Payment API: http://localhost:8081"
	@echo "  Provider Simulator: http://localhost:9000"

# Stop test environment
test-down:
	$(COMPOSE_TEST) down -v

# =============================================================================
# Test targets
# =============================================================================

# Run all tests
test: test-unit

# Run unit tests (no database required)
test-unit:
	go test -short ./...

# Run integration tests (requires database)
test-integration: test-up
	@echo "Waiting for services to be ready..."
	@sleep 5
	DATABASE_URL="postgres://payment_test:payment_test_secret@localhost:5433/payment_processing_test?sslmode=disable" \
		go test -v ./shared/repository/... -run TestIntegration
	$(MAKE) test-down

# =============================================================================
# Code quality
# =============================================================================

# Run linter
lint:
	golangci-lint run ./...

# Generate test coverage report
coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

# =============================================================================
# Maintenance
# =============================================================================

# Clean build artifacts
clean:
	rm -rf bin/
	rm -f payment-processing
	rm -f coverage.out coverage.html
	go clean -testcache

# Run migrations against development database
migrate-up:
	go run ./cmd/migrate up

# Rollback migrations
migrate-down:
	go run ./cmd/migrate down

# Show migration status
migrate-status:
	go run ./cmd/migrate status

# =============================================================================
# Help
# =============================================================================

help:
	@echo "Build targets:"
	@echo "  build             - Build all services"
	@echo "  build-api         - Build payment-api service"
	@echo "  build-worker      - Build payment-worker service"
	@echo "  build-simulator   - Build provider-simulator service"
	@echo "  build-combined    - Build combined binary (legacy)"
	@echo ""
	@echo "Docker targets:"
	@echo "  docker-build      - Build all Docker images"
	@echo "  docker-build-api  - Build payment-api Docker image"
	@echo "  docker-build-worker - Build payment-worker Docker image"
	@echo ""
	@echo "Development environment:"
	@echo "  dev-up            - Start dev dependencies (postgres, temporal)"
	@echo "  dev-up-all        - Start full dev stack including services"
	@echo "  dev-down          - Stop development stack"
	@echo "  dev-logs          - View development logs"
	@echo ""
	@echo "Testing environment:"
	@echo "  test-up           - Start test env with provider simulator"
	@echo "  test-down         - Stop test environment"
	@echo ""
	@echo "Test targets:"
	@echo "  test              - Run unit tests"
	@echo "  test-unit         - Run unit tests (no database required)"
	@echo "  test-integration  - Run integration tests"
	@echo ""
	@echo "Code quality:"
	@echo "  lint              - Run linter"
	@echo "  coverage          - Generate test coverage report"
	@echo ""
	@echo "Maintenance:"
	@echo "  clean             - Clean build artifacts"
