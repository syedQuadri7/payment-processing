.PHONY: build test test-unit test-integration test-db-up test-db-down lint clean

# Build the application
build:
	go build -o payment-processing .

# Run all tests
test: test-unit

# Run unit tests (no database required)
test-unit:
	go test -short ./...

# Run integration tests (requires database)
test-integration: test-db-up
	@echo "Waiting for database to be ready..."
	@sleep 2
	DATABASE_URL="postgres://payment:payment_secret@localhost:5433/payment_processing_test?sslmode=disable" \
		go test -v ./internal/database/... -run TestMigrations
	$(MAKE) test-db-down

# Start test database
test-db-up:
	docker compose -f docker-compose.test.yml up -d
	@echo "Test database starting on port 5433..."

# Stop test database
test-db-down:
	docker compose -f docker-compose.test.yml down -v

# Run development database
dev-db-up:
	docker compose up -d postgres

# Run full development stack (postgres + temporal)
dev-up:
	docker compose up -d

# Stop development stack
dev-down:
	docker compose down

# Run linter
lint:
	golangci-lint run ./...

# Clean build artifacts
clean:
	rm -f payment-processing
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

# Generate test coverage report
coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

# Help
help:
	@echo "Available targets:"
	@echo "  build           - Build the application"
	@echo "  test            - Run unit tests"
	@echo "  test-unit       - Run unit tests (no database required)"
	@echo "  test-integration- Run integration tests (starts test database)"
	@echo "  test-db-up      - Start test database (port 5433)"
	@echo "  test-db-down    - Stop test database"
	@echo "  dev-up          - Start development stack"
	@echo "  dev-down        - Stop development stack"
	@echo "  lint            - Run linter"
	@echo "  coverage        - Generate test coverage report"
	@echo "  clean           - Clean build artifacts"
